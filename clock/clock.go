// Package clock implements vector clocks for distributed systems.
//
// A vector clock is a data structure used to determine the partial ordering
// of events in a distributed system and detect causality violations.
//
// This implementation is:
//   - Pure logic with no I/O or networking
//   - Deterministic in all operations
//   - Concurrency-safe when used with external locking
//   - Zero external dependencies
//
// Design Decisions:
//
// 1. NodeID is a string for flexibility (supports UUIDs, hostnames, etc.)
// 2. Internal map is not exported; prevents external mutation
// 3. Copy-on-write semantics for immutability where practical
// 4. Comparison returns explicit enum for clarity
// 5. Stable iteration order via sorted keys for determinism
package clock

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// NodeID uniquely identifies a node in the distributed system.
type NodeID string

// Clock represents a vector clock mapping node IDs to logical timestamps.
// The zero value is a valid empty clock.
//
// Concurrency: Clock is NOT internally synchronized. Callers must provide
// external locking if concurrent access is required.
type Clock struct {
	// clock maps node IDs to their logical clock values.
	// We use int64 to avoid overflow in practical scenarios.
	clock map[NodeID]int64
}

// Comparison represents the relationship between two clocks.
type Comparison int

const (
	// ConcurrentCmp indicates the clocks are concurrent (neither happened-before).
	ConcurrentCmp Comparison = iota
	// BeforeCmp indicates the first clock happened-before the second.
	BeforeCmp
	// AfterCmp indicates the first clock happened-after the second.
	AfterCmp
	// EqualCmp indicates the clocks are identical.
	EqualCmp
)

// String returns a human-readable representation of the comparison.
func (c Comparison) String() string {
	switch c {
	case ConcurrentCmp:
		return "Concurrent"
	case BeforeCmp:
		return "Before"
	case AfterCmp:
		return "After"
	case EqualCmp:
		return "Equal"
	default:
		return fmt.Sprintf("Comparison(%d)", c)
	}
}

// NewClock creates a new vector clock, optionally initialized with the given node IDs
// set to zero. If no nodes are provided, an empty clock is created.
func NewClock(nodes ...NodeID) *Clock {
	c := &Clock{
		clock: make(map[NodeID]int64, len(nodes)),
	}
	for _, node := range nodes {
		c.clock[node] = 0
	}
	return c
}

// Copy creates a deep copy of the clock.
// This is useful for creating snapshots or ensuring immutability.
func (c *Clock) Copy() *Clock {
	if c == nil {
		return NewClock()
	}

	copy := &Clock{
		clock: make(map[NodeID]int64, len(c.clock)),
	}
	for k, v := range c.clock {
		copy.clock[k] = v
	}
	return copy
}

// Increment increments the clock value for the given node and returns the new value.
// If the node doesn't exist in the clock, it is initialized to 1.
//
// This operation MUTATES the clock. Use Copy() first if immutability is required.
func (c *Clock) Increment(node NodeID) int64 {
	if c.clock == nil {
		c.clock = make(map[NodeID]int64)
	}
	c.clock[node]++
	return c.clock[node]
}

// Get returns the clock value for the given node.
// Returns 0 if the node is not present in the clock.
func (c *Clock) Get(node NodeID) int64 {
	if c == nil || c.clock == nil {
		return 0
	}
	return c.clock[node]
}

// Set sets the clock value for the given node.
// This operation MUTATES the clock.
//
// Edge case: Setting a negative value is allowed but not recommended.
// Edge case: Setting a value to 0 keeps the entry (doesn't remove it).
func (c *Clock) Set(node NodeID, value int64) {
	if c.clock == nil {
		c.clock = make(map[NodeID]int64)
	}
	c.clock[node] = value
}

// Merge updates this clock to the element-wise maximum of itself and other.
// This implements the merge operation for vector clocks when receiving a message.
//
// This operation MUTATES the clock. Use Copy() first if immutability is required.
//
// Nil safety: If other is nil, this is a no-op.
func (c *Clock) Merge(other *Clock) {
	if other == nil || other.clock == nil {
		return
	}

	if c.clock == nil {
		c.clock = make(map[NodeID]int64, len(other.clock))
	}

	for node, otherVal := range other.clock {
		if currentVal := c.clock[node]; otherVal > currentVal {
			c.clock[node] = otherVal
		} else if currentVal == 0 && otherVal == 0 {
			// Ensure the node is present even if both are zero
			c.clock[node] = 0
		}
	}
}

// Compare determines the ordering relationship between two clocks.
//
// Returns:
//   - EqualCmp: clocks are identical
//   - BeforeCmp: c happened-before other (c <= other for all nodes, and c < other for at least one)
//   - AfterCmp: c happened-after other (c >= other for all nodes, and c > other for at least one)
//   - ConcurrentCmp: clocks are concurrent (neither happened-before relationship holds)
//
// Nil handling:
//   - Compare(nil, nil) -> EqualCmp
//   - Compare(non-empty, nil) -> AfterCmp
//   - Compare(nil, non-empty) -> BeforeCmp
//   - Compare(empty, empty) -> EqualCmp
func (c *Clock) Compare(other *Clock) Comparison {
	// Normalize nil clocks to empty clocks for comparison
	cEmpty := c == nil || c.clock == nil || len(c.clock) == 0
	otherEmpty := other == nil || other.clock == nil || len(other.clock) == 0

	if cEmpty && otherEmpty {
		return EqualCmp
	}

	if cEmpty {
		// c is empty, other is not
		// Check if other has any non-zero values
		for _, v := range other.clock {
			if v > 0 {
				return BeforeCmp
			}
		}
		return EqualCmp
	}

	if otherEmpty {
		// c is not empty, other is
		for _, v := range c.clock {
			if v > 0 {
				return AfterCmp
			}
		}
		return EqualCmp
	}

	// Both clocks have entries
	// Collect all nodes present in either clock
	allNodes := make(map[NodeID]struct{})
	for node := range c.clock {
		allNodes[node] = struct{}{}
	}
	for node := range other.clock {
		allNodes[node] = struct{}{}
	}

	// Track relationship
	hasGreater := false
	hasLess := false

	for node := range allNodes {
		cVal := c.clock[node]
		otherVal := other.clock[node]

		if cVal > otherVal {
			hasGreater = true
		} else if cVal < otherVal {
			hasLess = true
		}

		// Early exit if we know it's concurrent
		if hasGreater && hasLess {
			return ConcurrentCmp
		}
	}

	if hasGreater && !hasLess {
		return AfterCmp
	}
	if hasLess && !hasGreater {
		return BeforeCmp
	}
	return EqualCmp
}

// Nodes returns a sorted list of all node IDs present in the clock.
// The returned slice is a new allocation to prevent external mutation.
//
// The sort order is deterministic (lexicographic) for stable iteration.
func (c *Clock) Nodes() []NodeID {
	if c == nil || c.clock == nil {
		return []NodeID{}
	}

	nodes := make([]NodeID, 0, len(c.clock))
	for node := range c.clock {
		nodes = append(nodes, node)
	}

	// Sort for deterministic ordering
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})

	return nodes
}

// IsEmpty returns true if the clock has no entries or all entries are zero.
func (c *Clock) IsEmpty() bool {
	if c == nil || c.clock == nil || len(c.clock) == 0 {
		return true
	}

	for _, v := range c.clock {
		if v != 0 {
			return false
		}
	}
	return true
}

// Len returns the number of nodes tracked by this clock.
func (c *Clock) Len() int {
	if c == nil || c.clock == nil {
		return 0
	}
	return len(c.clock)
}

// String returns a human-readable representation of the clock.
// Format: {node1:value1, node2:value2, ...} with nodes in sorted order.
func (c *Clock) String() string {
	if c == nil || c.clock == nil || len(c.clock) == 0 {
		return "{}"
	}

	nodes := c.Nodes()
	var buf bytes.Buffer
	buf.WriteString("{")

	for i, node := range nodes {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%s:%d", node, c.clock[node])
	}

	buf.WriteString("}")
	return buf.String()
}

// Equal returns true if two clocks are identical (all node values match).
// This is a convenience wrapper around Compare.
func (c *Clock) Equal(other *Clock) bool {
	return c.Compare(other) == EqualCmp
}

// HappenedBefore returns true if c happened-before other.
// This is a convenience wrapper around Compare.
func (c *Clock) HappenedBefore(other *Clock) bool {
	return c.Compare(other) == BeforeCmp
}

// HappenedAfter returns true if c happened-after other.
// This is a convenience wrapper around Compare.
func (c *Clock) HappenedAfter(other *Clock) bool {
	return c.Compare(other) == AfterCmp
}

// Concurrent returns true if c and other are concurrent.
// This is a convenience wrapper around Compare.
func (c *Clock) Concurrent(other *Clock) bool {
	return c.Compare(other) == ConcurrentCmp
}

// Descendant returns true if c is a descendant of other (happened-after or equal).
func (c *Clock) Descendant(other *Clock) bool {
	cmp := c.Compare(other)
	return cmp == AfterCmp || cmp == EqualCmp
}

// Ancestor returns true if c is an ancestor of other (happened-before or equal).
func (c *Clock) Ancestor(other *Clock) bool {
	cmp := c.Compare(other)
	return cmp == BeforeCmp || cmp == EqualCmp
}

// ParseClock parses a string representation back into a Clock.
// Format: {node1:value1, node2:value2} or {}
// This is primarily for testing and debugging; production code should not
// serialize clocks as strings.
//
// Returns an error if the format is invalid or values are not valid integers.
func ParseClock(s string) (*Clock, error) {
	s = strings.TrimSpace(s)
	if s == "{}" {
		return NewClock(), nil
	}

	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("clock string must be wrapped in braces: %s", s)
	}

	s = s[1 : len(s)-1] // Remove braces
	if s == "" {
		return NewClock(), nil
	}

	c := NewClock()
	pairs := strings.Split(s, ",")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.Split(pair, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid node:value pair: %s", pair)
		}

		node := NodeID(strings.TrimSpace(parts[0]))
		var value int64
		_, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &value)
		if err != nil {
			return nil, fmt.Errorf("invalid value for node %s: %w", node, err)
		}

		c.Set(node, value)
	}

	return c, nil
}
