package vclock

import (
	"fmt"
	"testing"
)

// TestNew verifies that New creates properly initialized clocks.
func TestNew(t *testing.T) {
	tests := []struct {
		name  string
		nodes []NodeID
		want  int
	}{
		{
			name:  "empty clock",
			nodes: nil,
			want:  0,
		},
		{
			name:  "single node",
			nodes: []NodeID{"node1"},
			want:  1,
		},
		{
			name:  "multiple nodes",
			nodes: []NodeID{"node1", "node2", "node3"},
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClock(tt.nodes...)
			if c == nil {
			t.Fatal("NewClock returned nil")
			}
			if got := c.Len(); got != tt.want {
				t.Errorf("Len() = %d, want %d", got, tt.want)
			}

			// Verify all nodes are initialized to zero
			for _, node := range tt.nodes {
				if got := c.Get(node); got != 0 {
					t.Errorf("Get(%s) = %d, want 0", node, got)
				}
			}
		})
	}
}

// TestIncrement verifies increment operations.
func TestIncrement(t *testing.T) {
	c := NewClock()

	// First increment should set to 1
	if got := c.Increment("node1"); got != 1 {
		t.Errorf("First Increment() = %d, want 1", got)
	}

	// Second increment should set to 2
	if got := c.Increment("node1"); got != 2 {
		t.Errorf("Second Increment() = %d, want 2", got)
	}

	// Different node should start at 1
	if got := c.Increment("node2"); got != 1 {
		t.Errorf("Increment(node2) = %d, want 1", got)
	}

	// Verify state
	if c.Get("node1") != 2 {
		t.Errorf("Get(node1) = %d, want 2", c.Get("node1"))
	}
	if c.Get("node2") != 1 {
		t.Errorf("Get(node2) = %d, want 1", c.Get("node2"))
	}
}

// TestGetSet verifies get and set operations.
func TestGetSet(t *testing.T) {
	c := NewClock()

	// Get on empty clock should return 0
	if got := c.Get("nonexistent"); got != 0 {
		t.Errorf("Get(nonexistent) = %d, want 0", got)
	}

	// Set and retrieve
	c.Set("node1", 42)
	if got := c.Get("node1"); got != 42 {
		t.Errorf("Get(node1) = %d, want 42", got)
	}

	// Set to zero (should keep the entry)
	c.Set("node1", 0)
	if got := c.Get("node1"); got != 0 {
		t.Errorf("Get(node1) after Set(0) = %d, want 0", got)
	}
	if c.Len() != 1 {
		t.Errorf("Len() after Set(0) = %d, want 1 (entry should remain)", c.Len())
	}

	// Negative values (edge case)
	c.Set("node2", -5)
	if got := c.Get("node2"); got != -5 {
		t.Errorf("Get(node2) with negative = %d, want -5", got)
	}
}

// TestCopy verifies deep copying.
func TestCopy(t *testing.T) {
	original := NewClock()
	original.Set("node1", 10)
	original.Set("node2", 20)

	copy := original.Copy()

	// Verify copy has same values
	if copy.Get("node1") != 10 {
		t.Errorf("Copy Get(node1) = %d, want 10", copy.Get("node1"))
	}
	if copy.Get("node2") != 20 {
		t.Errorf("Copy Get(node2) = %d, want 20", copy.Get("node2"))
	}

	// Mutate original
	original.Set("node1", 999)
	original.Set("node3", 30)

	// Verify copy is unaffected
	if copy.Get("node1") != 10 {
		t.Errorf("Copy Get(node1) after original mutation = %d, want 10", copy.Get("node1"))
	}
	if copy.Get("node3") != 0 {
		t.Errorf("Copy Get(node3) after original mutation = %d, want 0", copy.Get("node3"))
	}

	// Test copying nil clock
	var nilClock *Clock
	copyNil := nilClock.Copy()
	if copyNil == nil {
		t.Error("Copy of nil clock returned nil, want empty clock")
	}
	if copyNil.Len() != 0 {
		t.Errorf("Copy of nil clock Len() = %d, want 0", copyNil.Len())
	}
}

// TestMerge verifies merge operations.
func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		clock1   map[NodeID]int64
		clock2   map[NodeID]int64
		expected map[NodeID]int64
	}{
		{
			name:     "empty clocks",
			clock1:   map[NodeID]int64{},
			clock2:   map[NodeID]int64{},
			expected: map[NodeID]int64{},
		},
		{
			name:     "merge with empty",
			clock1:   map[NodeID]int64{"node1": 5},
			clock2:   map[NodeID]int64{},
			expected: map[NodeID]int64{"node1": 5},
		},
		{
			name:     "merge into empty",
			clock1:   map[NodeID]int64{},
			clock2:   map[NodeID]int64{"node1": 5},
			expected: map[NodeID]int64{"node1": 5},
		},
		{
			name:     "element-wise maximum",
			clock1:   map[NodeID]int64{"node1": 5, "node2": 2, "node3": 8},
			clock2:   map[NodeID]int64{"node1": 3, "node2": 7, "node4": 1},
			expected: map[NodeID]int64{"node1": 5, "node2": 7, "node3": 8, "node4": 1},
		},
		{
			name:     "identical clocks",
			clock1:   map[NodeID]int64{"node1": 5, "node2": 3},
			clock2:   map[NodeID]int64{"node1": 5, "node2": 3},
			expected: map[NodeID]int64{"node1": 5, "node2": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c1 := NewClock()
			for k, v := range tt.clock1 {
				c1.Set(k, v)
			}

			c2 := NewClock()
			for k, v := range tt.clock2 {
				c2.Set(k, v)
			}

			c1.Merge(c2)

			// Verify merge result
			for node, expected := range tt.expected {
				if got := c1.Get(node); got != expected {
					t.Errorf("After merge, Get(%s) = %d, want %d", node, got, expected)
				}
			}

			// Verify no extra nodes
			if c1.Len() != len(tt.expected) {
				t.Errorf("After merge, Len() = %d, want %d", c1.Len(), len(tt.expected))
			}
		})
	}

	// Test merging with nil
	c := NewClock()
	c.Set("node1", 5)
	c.Merge(nil)
	if c.Get("node1") != 5 {
		t.Errorf("Merge(nil) affected clock: Get(node1) = %d, want 5", c.Get("node1"))
	}
}

// TestCompare verifies all comparison scenarios.
func TestCompare(t *testing.T) {
	tests := []struct {
		name     string
		clock1   map[NodeID]int64
		clock2   map[NodeID]int64
		expected Comparison
	}{
		{
			name:     "both empty",
			clock1:   map[NodeID]int64{},
			clock2:   map[NodeID]int64{},
			expected: EqualCmp,
		},
		{
			name:     "identical clocks",
			clock1:   map[NodeID]int64{"node1": 5, "node2": 3},
			clock2:   map[NodeID]int64{"node1": 5, "node2": 3},
			expected: EqualCmp,
		},
		{
			name:     "clock1 before clock2",
			clock1:   map[NodeID]int64{"node1": 2, "node2": 3},
			clock2:   map[NodeID]int64{"node1": 5, "node2": 3},
			expected: BeforeCmp,
		},
		{
			name:     "clock1 after clock2",
			clock1:   map[NodeID]int64{"node1": 5, "node2": 3},
			clock2:   map[NodeID]int64{"node1": 2, "node2": 3},
			expected: AfterCmp,
		},
		{
			name:     "concurrent clocks",
			clock1:   map[NodeID]int64{"node1": 5, "node2": 2},
			clock2:   map[NodeID]int64{"node1": 2, "node2": 5},
			expected: ConcurrentCmp,
		},
		{
			name:     "clock1 empty, clock2 has zero",
			clock1:   map[NodeID]int64{},
			clock2:   map[NodeID]int64{"node1": 0},
			expected: EqualCmp,
		},
		{
			name:     "clock1 empty, clock2 has non-zero",
			clock1:   map[NodeID]int64{},
			clock2:   map[NodeID]int64{"node1": 5},
			expected: BeforeCmp,
		},
		{
			name:     "clock1 has non-zero, clock2 empty",
			clock1:   map[NodeID]int64{"node1": 5},
			clock2:   map[NodeID]int64{},
			expected: AfterCmp,
		},
		{
			name:     "different node sets - before",
			clock1:   map[NodeID]int64{"node1": 2},
			clock2:   map[NodeID]int64{"node1": 5, "node2": 3},
			expected: BeforeCmp,
		},
		{
			name:     "different node sets - after",
			clock1:   map[NodeID]int64{"node1": 5, "node2": 3},
			clock2:   map[NodeID]int64{"node1": 2},
			expected: AfterCmp,
		},
		{
			name:     "different node sets - concurrent",
			clock1:   map[NodeID]int64{"node1": 5},
			clock2:   map[NodeID]int64{"node2": 5},
			expected: ConcurrentCmp,
		},
		{
			name:     "all zeros - equal",
			clock1:   map[NodeID]int64{"node1": 0, "node2": 0},
			clock2:   map[NodeID]int64{"node1": 0, "node2": 0},
			expected: EqualCmp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c1 := NewClock()
			for k, v := range tt.clock1 {
				c1.Set(k, v)
			}

			c2 := NewClock()
			for k, v := range tt.clock2 {
				c2.Set(k, v)
			}

			got := c1.Compare(c2)
			if got != tt.expected {
				t.Errorf("Compare() = %s, want %s", got, tt.expected)
			}

			// Verify comparison is symmetric for Equal and Concurrent
			reverse := c2.Compare(c1)
			switch tt.expected {
			case EqualCmp:
				if reverse != EqualCmp {
					t.Errorf("Reverse Compare() = %s, want EqualCmp (symmetric)", reverse)
				}
			case BeforeCmp:
				if reverse != AfterCmp {
					t.Errorf("Reverse Compare() = %s, want AfterCmp", reverse)
				}
			case AfterCmp:
				if reverse != BeforeCmp {
					t.Errorf("Reverse Compare() = %s, want BeforeCmp", reverse)
				}
			case ConcurrentCmp:
				if reverse != ConcurrentCmp {
					t.Errorf("Reverse Compare() = %s, want ConcurrentCmp (symmetric)", reverse)
				}
			}
		})
	}
}

// TestCompareNil verifies nil handling in comparisons.
func TestCompareNil(t *testing.T) {
	var nilClock *Clock
	empty := NewClock()
	nonEmpty := NewClock()
	nonEmpty.Set("node1", 5)

	// nil vs nil
	if got := nilClock.Compare(nilClock); got != EqualCmp {
		t.Errorf("nil.Compare(nil) = %s, want EqualCmp", got)
	}

	// nil vs empty
	if got := nilClock.Compare(empty); got != EqualCmp {
		t.Errorf("nil.Compare(empty) = %s, want EqualCmp", got)
	}

	// empty vs nil
	if got := empty.Compare(nilClock); got != EqualCmp {
		t.Errorf("empty.Compare(nil) = %s, want EqualCmp", got)
	}

	// nil vs non-empty
	if got := nilClock.Compare(nonEmpty); got != BeforeCmp {
		t.Errorf("nil.Compare(nonEmpty) = %s, want BeforeCmp", got)
	}

	// non-empty vs nil
	if got := nonEmpty.Compare(nilClock); got != AfterCmp {
		t.Errorf("nonEmpty.Compare(nil) = %s, want AfterCmp", got)
	}
}

// TestConvenienceMethods verifies the convenience comparison methods.
func TestConvenienceMethods(t *testing.T) {
	c1 := NewClock()
	c1.Set("node1", 2)

	c2 := NewClock()
	c2.Set("node1", 5)

	c3 := NewClock()
	c3.Set("node1", 2)

	c4 := NewClock()
	c4.Set("node2", 2)

	// Equal
	if !c1.Equal(c3) {
		t.Error("Equal() returned false for identical clocks")
	}
	if c1.Equal(c2) {
		t.Error("Equal() returned true for different clocks")
	}

	// HappenedBefore
	if !c1.HappenedBefore(c2) {
		t.Error("HappenedBefore() returned false when it should be true")
	}
	if c2.HappenedBefore(c1) {
		t.Error("HappenedBefore() returned true when it should be false")
	}

	// HappenedAfter
	if !c2.HappenedAfter(c1) {
		t.Error("HappenedAfter() returned false when it should be true")
	}
	if c1.HappenedAfter(c2) {
		t.Error("HappenedAfter() returned true when it should be false")
	}

	// Concurrent
	if !c1.Concurrent(c4) {
		t.Error("Concurrent() returned false for concurrent clocks")
	}
	if c1.Concurrent(c2) {
		t.Error("Concurrent() returned true for non-concurrent clocks")
	}

	// Descendant
	if !c2.Descendant(c1) {
		t.Error("Descendant() returned false when clock2 > clock1")
	}
	if !c1.Descendant(c3) {
		t.Error("Descendant() returned false for equal clocks")
	}
	if c1.Descendant(c2) {
		t.Error("Descendant() returned true when clock1 < clock2")
	}

	// Ancestor
	if !c1.Ancestor(c2) {
		t.Error("Ancestor() returned false when clock1 < clock2")
	}
	if !c1.Ancestor(c3) {
		t.Error("Ancestor() returned false for equal clocks")
	}
	if c2.Ancestor(c1) {
		t.Error("Ancestor() returned true when clock2 > clock1")
	}
}

// TestNodes verifies the Nodes method.
func TestNodes(t *testing.T) {
	c := NewClock()
	c.Set("charlie", 3)
	c.Set("alice", 1)
	c.Set("bob", 2)

	nodes := c.Nodes()

	// Verify length
	if len(nodes) != 3 {
		t.Errorf("Nodes() length = %d, want 3", len(nodes))
	}

	// Verify sorted order
	expected := []NodeID{"alice", "bob", "charlie"}
	for i, node := range nodes {
		if node != expected[i] {
			t.Errorf("Nodes()[%d] = %s, want %s", i, node, expected[i])
		}
	}

	// Verify mutation doesn't affect clock
	nodes[0] = "mutated"
	if c.Nodes()[0] != "alice" {
		t.Error("Mutating Nodes() result affected the clock")
	}

	// Empty clock
	empty := NewClock()
	if len(empty.Nodes()) != 0 {
		t.Errorf("Empty clock Nodes() length = %d, want 0", len(empty.Nodes()))
	}

	// Nil clock
	var nilClock *Clock
	if len(nilClock.Nodes()) != 0 {
		t.Errorf("Nil clock Nodes() length = %d, want 0", len(nilClock.Nodes()))
	}
}

// TestIsEmpty verifies the IsEmpty method.
func TestIsEmpty(t *testing.T) {
	// Nil clock
	var nilClock *Clock
	if !nilClock.IsEmpty() {
		t.Error("Nil clock IsEmpty() = false, want true")
	}

	// Empty clock
	empty := NewClock()
	if !empty.IsEmpty() {
		t.Error("Empty clock IsEmpty() = false, want true")
	}

	// Clock with zeros
	zeros := NewClock()
	zeros.Set("node1", 0)
	zeros.Set("node2", 0)
	if !zeros.IsEmpty() {
		t.Error("Clock with all zeros IsEmpty() = false, want true")
	}

	// Non-empty clock
	nonEmpty := NewClock()
	nonEmpty.Set("node1", 1)
	if nonEmpty.IsEmpty() {
		t.Error("Non-empty clock IsEmpty() = true, want false")
	}

	// Mixed
	mixed := NewClock()
	mixed.Set("node1", 0)
	mixed.Set("node2", 5)
	if mixed.IsEmpty() {
		t.Error("Clock with mixed values IsEmpty() = true, want false")
	}
}

// TestString verifies the String method.
func TestString(t *testing.T) {
	tests := []struct {
		name     string
		clock    map[NodeID]int64
		expected string
	}{
		{
			name:     "empty clock",
			clock:    map[NodeID]int64{},
			expected: "{}",
		},
		{
			name:     "single node",
			clock:    map[NodeID]int64{"node1": 5},
			expected: "{node1:5}",
		},
		{
			name:     "multiple nodes sorted",
			clock:    map[NodeID]int64{"charlie": 3, "alice": 1, "bob": 2},
			expected: "{alice:1, bob:2, charlie:3}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClock()
			for k, v := range tt.clock {
				c.Set(k, v)
			}

			got := c.String()
			if got != tt.expected {
				t.Errorf("String() = %s, want %s", got, tt.expected)
			}
		})
	}

	// Nil clock
	var nilClock *Clock
	if got := nilClock.String(); got != "{}" {
		t.Errorf("Nil clock String() = %s, want {}", got)
	}
}

// TestParseClock verifies the ParseClock function.
func TestParseClock(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantClock map[NodeID]int64
		wantErr   bool
	}{
		{
			name:      "empty clock",
			input:     "{}",
			wantClock: map[NodeID]int64{},
			wantErr:   false,
		},
		{
			name:      "single node",
			input:     "{node1:5}",
			wantClock: map[NodeID]int64{"node1": 5},
			wantErr:   false,
		},
		{
			name:      "multiple nodes",
			input:     "{node1:5, node2:3, node3:7}",
			wantClock: map[NodeID]int64{"node1": 5, "node2": 3, "node3": 7},
			wantErr:   false,
		},
		{
			name:      "with whitespace",
			input:     "{ node1 : 5 , node2 : 3 }",
			wantClock: map[NodeID]int64{"node1": 5, "node2": 3},
			wantErr:   false,
		},
		{
			name:    "missing braces",
			input:   "node1:5",
			wantErr: true,
		},
		{
			name:    "invalid pair format",
			input:   "{node1-5}",
			wantErr: true,
		},
		{
			name:    "invalid value",
			input:   "{node1:abc}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseClock(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("ParseClock() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseClock() unexpected error: %v", err)
			}

			if got == nil {
				t.Fatal("ParseClock() returned nil clock")
			}

			// Verify parsed values
			for node, expected := range tt.wantClock {
				if val := got.Get(node); val != expected {
					t.Errorf("Get(%s) = %d, want %d", node, val, expected)
				}
			}

			if got.Len() != len(tt.wantClock) {
				t.Errorf("Len() = %d, want %d", got.Len(), len(tt.wantClock))
			}
		})
	}
}

// TestParseClock_RoundTrip verifies that String and ParseClock are inverses.
func TestParseClock_RoundTrip(t *testing.T) {
	original := NewClock()
	original.Set("alice", 10)
	original.Set("bob", 20)
	original.Set("charlie", 15)

	str := original.String()
	parsed, err := ParseClock(str)
	if err != nil {
		t.Fatalf("ParseClock() error: %v", err)
	}

	if !original.Equal(parsed) {
		t.Errorf("Round trip failed: original=%s, parsed=%s", original, parsed)
	}
}

// TestComparisonString verifies the String method on Comparison.
func TestComparisonString(t *testing.T) {
	tests := []struct {
		cmp  Comparison
		want string
	}{
		{ConcurrentCmp, "Concurrent"},
		{BeforeCmp, "Before"},
		{AfterCmp, "After"},
		{EqualCmp, "Equal"},
		{Comparison(999), "Comparison(999)"},
	}

	for _, tt := range tests {
		if got := tt.cmp.String(); got != tt.want {
			t.Errorf("Comparison(%d).String() = %s, want %s", tt.cmp, got, tt.want)
		}
	}
}

// TestDeterministicIteration verifies that iteration order is stable.
func TestDeterministicIteration(t *testing.T) {
	c := NewClock()
	c.Set("zebra", 1)
	c.Set("apple", 2)
	c.Set("mango", 3)
	c.Set("banana", 4)

	// Call Nodes() multiple times and verify same order
	nodes1 := c.Nodes()
	nodes2 := c.Nodes()
	nodes3 := c.Nodes()

	for i := range nodes1 {
		if nodes1[i] != nodes2[i] || nodes2[i] != nodes3[i] {
			t.Errorf("Nodes() order is not deterministic at index %d: %s, %s, %s",
				i, nodes1[i], nodes2[i], nodes3[i])
		}
	}

	// Verify String() is deterministic
	str1 := c.String()
	str2 := c.String()
	str3 := c.String()

	if str1 != str2 || str2 != str3 {
		t.Errorf("String() is not deterministic: %s, %s, %s", str1, str2, str3)
	}
}

// TestConcurrencySafety_WithExternalLocking demonstrates external locking pattern.
func TestConcurrencySafety_WithExternalLocking(t *testing.T) {
	// This test demonstrates the correct usage pattern with external locking.
	// The clock itself is not thread-safe, but when used with external
	// synchronization, it should work correctly.

	c := NewClock()
	c.Set("node1", 0)

	// In a real scenario, you would use sync.Mutex or sync.RWMutex
	// Here we just verify that the operations are deterministic

	for i := 0; i < 100; i++ {
		c.Increment("node1")
	}

	if c.Get("node1") != 100 {
		t.Errorf("After 100 increments, Get(node1) = %d, want 100", c.Get("node1"))
	}
}

// TestTypicalUsagePattern demonstrates a typical distributed system scenario.
func TestTypicalUsagePattern(t *testing.T) {
	// Node A's clock
	nodeA := NewClock("A", "B", "C")

	// Node A does work
	nodeA.Increment("A")
	nodeA.Increment("A")

	// Node A sends message to Node B with clock {A:2, B:0, C:0}
	messageA := nodeA.Copy()

	// Node B's clock
	nodeB := NewClock("A", "B", "C")
	nodeB.Increment("B")

	// Node B receives message from A
	nodeB.Merge(messageA) // Should be {A:2, B:1, C:0}
	nodeB.Increment("B")  // Should be {A:2, B:2, C:0}

	// Verify final state
	if nodeB.Get("A") != 2 {
		t.Errorf("NodeB Get(A) = %d, want 2", nodeB.Get("A"))
	}
	if nodeB.Get("B") != 2 {
		t.Errorf("NodeB Get(B) = %d, want 2", nodeB.Get("B"))
	}
	if nodeB.Get("C") != 0 {
		t.Errorf("NodeB Get(C) = %d, want 0", nodeB.Get("C"))
	}

	// Verify happened-before relationship
	if !messageA.HappenedBefore(nodeB) {
		t.Error("messageA should happen-before nodeB's final state")
	}
}

// BenchmarkIncrement measures increment performance.
func BenchmarkIncrement(b *testing.B) {
	c := NewClock()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Increment("node1")
	}
}

// BenchmarkMerge measures merge performance.
func BenchmarkMerge(b *testing.B) {
	c1 := NewClock()
	c2 := NewClock()
	for i := 0; i < 100; i++ {
		node := NodeID(fmt.Sprintf("node%d", i))
		c1.Set(node, int64(i))
		c2.Set(node, int64(i+50))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c1Copy := c1.Copy()
		c1Copy.Merge(c2)
	}
}

// BenchmarkCompare measures comparison performance.
func BenchmarkCompare(b *testing.B) {
	c1 := NewClock()
	c2 := NewClock()
	for i := 0; i < 100; i++ {
		node := NodeID(fmt.Sprintf("node%d", i))
		c1.Set(node, int64(i))
		c2.Set(node, int64(i+1))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c1.Compare(c2)
	}
}

// BenchmarkCopy measures copy performance.
func BenchmarkCopy(b *testing.B) {
	c := NewClock()
	for i := 0; i < 100; i++ {
		c.Set(NodeID(fmt.Sprintf("node%d", i)), int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Copy()
	}
}

// BenchmarkString measures String() performance.
func BenchmarkString(b *testing.B) {
	c := NewClock()
	for i := 0; i < 50; i++ {
		c.Set(NodeID(fmt.Sprintf("node%d", i)), int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.String()
	}
}
