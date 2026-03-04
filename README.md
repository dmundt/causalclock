# Vector Clocks and Version Vectors for Go

[![CI](https://github.com/dmundt/causalclock/actions/workflows/ci.yml/badge.svg)](https://github.com/dmundt/causalclock/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dmundt/causalclock)](https://github.com/dmundt/causalclock)
[![Go Doc](https://pkg.go.dev/badge/github.com/dmundt/causalclock)](https://pkg.go.dev/github.com/dmundt/causalclock)

Production-ready implementations for distributed systems causal tracking:
- **Vector Clocks**: Event ordering and happens-before relationships
- **Version Vectors**: Per-object version tracking with Dynamo/Riak semantics

## Features

- **Pure Logic**: No I/O, networking, or external dependencies
- **Deterministic**: Stable iteration order and reproducible behavior
- **Concurrency-Safe**: Thread-safe when used with external locking (no hidden global state)
- **Zero Dependencies**: Standard library only
- **Fully Tested**: Comprehensive test coverage with edge cases
- **Well-Documented**: Examples for all major use cases

## Installation

```bash
go get github.com/dmundt/causalclock
```

## Quick Start

### Vector Clocks (Event Ordering)

```go
package main

import (
    "fmt"
    "github.com/dmundt/causalclock"
)

func main() {
    // Create clocks for two nodes
    alice := vclock.NewClock()
    bob := vclock.NewClock()
    
    // Alice does work
    alice.Increment("alice")
    alice.Increment("alice")
    
    // Bob receives Alice's clock and does work
    bob.Merge(alice)
    bob.Increment("bob")
    
    // Check causality
    fmt.Println(bob.HappenedAfter(alice)) // true
}
```

### Version Vectors (Object Versioning)

```go
package main

import (
    "fmt"
    "github.com/dmundt/causalclock"
)

func main() {
    // Track versions for an object across replicas
    version := vclock.NewVersionVector()
    
    // Replica A updates
    version.Increment("A")
    
    // Replica B receives and updates
    versionB := version.Copy()
    versionB.Increment("B")
    
    // Detect causality
    fmt.Println(version.HappenedBefore(versionB)) // true
}
```

## API Overview

### Vector Clocks

**Core Operations**
- **`NewClock(nodes ...NodeID) *Clock`** - Create a new vector clock
- **`Increment(node NodeID) int64`** - Increment and return new value
- **`Get(node NodeID) int64`** - Read a node's clock value
- **`Set(node NodeID, value int64)`** - Set a node's clock value
- **`Merge(other *Clock)`** - Merge two clocks (element-wise max)
- **`Copy() *Clock`** - Create a deep copy

**Comparison Operations**
- **`Compare(other *Clock) Comparison`** - Full comparison (returns Equal, Before, After, Concurrent)
- **`Equal(other *Clock) bool`** - Test for equality
- **`HappenedBefore(other *Clock) bool`** - Test for causality
- **`HappenedAfter(other *Clock) bool`** - Test for causality
- **`Concurrent(other *Clock) bool`** - Test for concurrency
- **`Ancestor(other *Clock) bool`** - Happened-before or equal
- **`Descendant(other *Clock) bool`** - Happened-after or equal

**Utility Operations**
- **`Nodes() []NodeID`** - Get sorted list of nodes
- **`Len() int`** - Number of tracked nodes
- **`IsEmpty() bool`** - Check if all values are zero
- **`String() string`** - Human-readable representation
- **`ParseClock(s string) (*Clock, error)`** - Parse from string

### Version Vectors

**Core Operations**
- **`NewVersionVector(replicas ...ReplicaID) *VersionVector`** - Create a new version vector
- **`Increment(replica ReplicaID) uint64`** - Increment replica counter
- **`Get(replica ReplicaID) uint64`** - Read a replica's version
- **`Set(replica ReplicaID, version uint64)`** - Set a replica's version
- **`Merge(other *VersionVector)`** - Merge with Dynamo/Riak semantics (element-wise max)
- **`Copy() *VersionVector`** - Create a deep copy

**Comparison Operations**
- **`Compare(other *VersionVector) Comparison`** - Full comparison
- **`Equal(other *VersionVector) bool`** - Test equality
- **`HappenedBefore(other *VersionVector) bool`** - Ancestor check
- **`HappenedAfter(other *VersionVector) bool`** - Descendant check
- **`Concurrent(other *VersionVector) bool`** - Conflict detection
- **`Descends(other *VersionVector) bool`** - Descendant or equal
- **`Dominates(other *VersionVector) bool`** - Strictly newer
- **`IsDominatedBy(other *VersionVector) bool`** - Strictly older

**Utility Operations**
- **`Replicas() []ReplicaID`** - Get sorted list of replicas
- **`Len() int`** - Number of tracked replicas
- **`IsEmpty() bool`** - Check if all versions are zero
- **`String() string`** - Human-readable representation

## Design Decisions

### Vector Clocks vs Version Vectors

**When to use Vector Clocks:**
- Tracking causality between events in a distributed system
- Message ordering in distributed protocols
- Detecting race conditions
- General happens-before relationship tracking

**When to use Version Vectors:**
- Per-object version tracking (Dynamo, Riak, Cassandra style)
- Conflict detection in eventually consistent systems
- Multi-master replication
- Shopping cart merging, collaborative editing

**Key Differences:**
- Vector Clocks use `int64` (signed, for flexibility)
- Version Vectors use `uint64` (unsigned, versions are always positive)
- Version Vectors follow Dynamo/Riak semantics explicitly
- Terminology: NodeID vs ReplicaID reflects different use cases

### 1. NodeID/ReplicaID as String

**Decision**: Use `string` for identifiers.

**Rationale**:
- Flexibility: Supports UUIDs, hostnames, IP addresses, etc.
- No artificial constraints on ID format
- Minimal memory overhead in practice
- Go's string type is immutable and efficient

**Tradeoff**: Slightly less type-safe than custom types, but gains flexibility.

### 2. Map-Based Storage

**Decision**: Internal `map[ID]counter` for both implementations.

**Rationale**:
- O(1) lookups and updates
- Sparse representation (only tracks seen nodes/replicas)
- Natural fit for distributed systems (dynamic membership)
- Memory-efficient for typical cluster sizes

**Tradeoff**: Slightly more overhead than arrays for small fixed sets, but more flexible.

### 3. Counter Types

**Decision**: 
- Vector Clocks use `int64` (signed)
- Version Vectors use `uint64` (unsigned)

**Rationale**:
- **int64 for Vector Clocks**: 
  - Extremely large range: 9.2 quintillion increments
  - Signed allows negative values (useful for debugging/testing)
  - Fixed size = predictable performance
  - Overflow practically impossible (1M increments/sec = 292K years)
  
- **uint64 for Version Vectors**:
  - Version numbers are conceptually unsigned (never negative)
  - Follows Dynamo/Riak conventions
  - Slightly larger positive range (18.4 quintillion)
  - Makes semantic intent clear (versions start at 0 or 1)

**Tradeoff**: Cannot detect overflow, but overflow is practically impossible.

### 4. Internal Map Not Exported

**Decision**: The internal map is private.

**Rationale**:
- Prevents direct mutation bypassing API contracts
- Allows implementation changes without breaking API
- Enforces Copy-on-Write semantics where needed
- Enables invariant validation if needed later

**Tradeoff**: Requires copying for snapshots, but Copy() is explicit and efficient.

### 5. Element-Wise Maximum in Merge

**Decision**: Both implementations use element-wise maximum merge.

**Rationale**:
- Standard algorithm for both vector clocks and version vectors
- Preserves causality (happens-before relationships)
- Commutative and associative (order doesn't matter)
- Idempotent (safe to merge multiple times)
- Matches Dynamo/Riak semantics for version vectors

**Alternative Considered**: Copy-on-write merge returning new structure. Rejected for performance and mutation clarity.

### 6. Comparison Semantics

**Decision**: Four explicit comparison results: Equal, Before, After, Concurrent.

**Rationale**:
- Matches distributed systems literature
- Concurrent (conflict) detection is critical for eventual consistency
- Single comparison operation more efficient than multiple calls
- Impossible to have contradictory states
- Clear API semantics

**Implementation**:
- Before: v1[i] ≤ v2[i] for all i, and v1[j] < v2[j] for at least one j
- After: v1[i] ≥ v2[i] for all i, and v1[j] > v2[j] for at least one j  
- Equal: v1[i] = v2[i] for all i
- Concurrent: Neither Before nor After (indicates conflict)

### 7. Deterministic Iteration Order

**Decision**: Always sort IDs before iteration/serialization.

**Rationale**:
- Reproducible output for testing
- Consistent serialization
- Debuggability (same state = same string)
- Prevents Go map iteration non-determinism from leaking

**Tradeoff**: O(n log n) sorting cost, negligible for typical sizes (<1000 nodes/replicas).

### 8. No Internal Locking

**Decision**: Neither implementation is internally synchronized.

**Rationale**:
- Composability: Users choose locking granularity
- Performance: No lock overhead when not needed
- Flexibility: Supports lock-free algorithms with external coordination
- Testability: Deterministic without lock ordering issues
- Separation of concerns: Data structure vs synchronization

**Tradeoff**: Requires external locking for concurrent access, but this is more flexible.

### 10. Zero Value is Valid

**Decision**: `var c Clock` and `var vv VersionVector` are valid empty structures.

**Rationale**:
- Idiomatic Go (zero values should be useful)
- Simplifies initialization
- No "constructed vs. unconstructed" state to track

**Tradeoff**: None significant.

### 11. Immutability via Copy

**Decision**: Provide `Copy()` instead of making structures immutable.

**Rationale**:
- Performance: Avoids unnecessary copies for every operation
- Flexibility: Users choose when to snapshot
- Clarity: Mutations are explicit (`Increment`, `Set`, `Merge`)
- Go idiom: Value semantics via explicit copy

**Alternative Considered**: Persistent data structures. Rejected for complexity and performance.

## Version Vector Design Notes

### Dynamo/Riak Semantics

Version vectors in this implementation follow the semantics used by Amazon Dynamo and Riak:

1. **Per-Object Versioning**: Each object has its own version vector
2. **Element-Wise Maximum Merge**: When merging, take max(v1[i], v2[i]) for each replica
3. **Conflict Detection**: Concurrent versions indicate conflicts requiring resolution
4. **Sibling Resolution**: Application handles conflicting versions (last-write-wins, merge, manual)

### Comparison Relations

Version vectors support all four causal relations:

- **Equal**: Identical versions (same object state)
- **Before**: v1 is an ancestor of v2 (v1 happened-before v2)
- **After**: v1 is a descendant of v2 (v1 happened-after v2)
- **Concurrent**: Neither happened-before (conflict/divergent versions)

These match the theoretical foundations from:
- Lamport (1978): "Time, Clocks, and the Ordering of Events"
- Fidge (1988): "Timestamps in Message-Passing Systems"
- Mattern (1988): "Virtual Time and Global States"

### Use Cases

**Version Vectors are ideal for:**
- Shopping carts (Amazon, Riak)
- Session management
- Document versioning (collaborative editing)
- Multi-master databases (Cassandra, Riak, Voldemort)
- Conflict-free replicated data types (CRDTs)
- Eventually consistent key-value stores

**Not suitable for:**
- Single-master replication (use simple version numbers)
- Systems requiring total ordering (use logical clocks or timestamps)
- Real-time constraints (version vectors don't capture wall-clock time)

## Edge Cases and Handling

### 1. Empty Structures

**Vector Clocks**:
```go
c1 := vclock.NewClock()
c2 := vclock.NewClock()
fmt.Println(c1.Equal(c2)) // true
```

**Version Vectors**:
```go
vv1 := vclock.NewVersionVector()
vv2 := vclock.NewVersionVector()
fmt.Println(vv1.Equal(vv2)) // true
```

**Behavior**:
- Empty compares `Equal` to another empty
- Empty `Before` any non-empty with positive values
- Merging with empty is a no-op

### 2. Nil Safety

**Behavior**:
- Nil methods don't panic
- Nil treated as empty clock for comparison
- `nil.Copy()` returns new empty clock

**Example**:
```go
var c *vclock.Clock
fmt.Println(c.Get("node1"))    // 0
fmt.Println(c.Nodes())         // []
fmt.Println(c.Compare(nil))    // Equal
```

### 3. Non-Existent Nodes

**Behavior**:
- `Get(missing)` returns 0 (no entry created)
- `Increment(missing)` initializes to 1
- `Set(missing, value)` creates entry

**Example**:
```go
c := vclock.NewClock()
fmt.Println(c.Get("missing")) // 0, no entry
fmt.Println(c.Len())          // 0
c.Increment("missing")
fmt.Println(c.Len())          // 1
```

### 4. Zero Values

**Behavior**:
- `Set(node, 0)` keeps the entry (doesn't remove it)
- Clock with all zeros is considered empty (`IsEmpty() == true`)
- Zero entries participate in comparisons

**Rationale**: Explicit presence tracking. A node set to 0 is different from a node never seen.

**Example**:
```go
c := vclock.NewClock()
c.Set("node1", 0)
fmt.Println(c.Len())      // 1 (entry exists)
fmt.Println(c.IsEmpty())  // true (all values zero)
```

### 5. Negative Values

**Behavior**:
- `Set(node, -5)` is allowed (not recommended)
- Negative values participate in comparisons normally
- No special handling or validation

**Rationale**: Keep implementation simple; clients can validate if needed.

**Example**:
```go
c := vclock.NewClock()
c.Set("node1", -5)
fmt.Println(c.Get("node1")) // -5
```

### 6. Disjoint Node Sets

**Behavior**:
- Clocks with different nodes compare correctly
- Missing nodes treated as 0 in comparisons
- Merge combines node sets

**Example**:
```go
c1 := vclock.NewClock()
c1.Set("alice", 5)

c2 := vclock.NewClock()
c2.Set("bob", 5)

fmt.Println(c1.Concurrent(c2)) // true (disjoint sets)
```

### 7. Large Clock Values

**Behavior**:
- int64 max = 9,223,372,036,854,775,807
- No overflow detection
- Wrapping to negative on overflow (standard Go behavior)

**Mitigation**: Practically impossible to overflow in real systems.

### 8. Concurrent Updates Detection

**Behavior**:
- Two clocks where neither is <= the other are concurrent
- Indicates conflict in distributed system
- Application decides resolution strategy

**Example**:
```go
c1 := vclock.NewClock()
c1.Set("node1", 5)
c1.Set("node2", 2)

c2 := vclock.NewClock()
c2.Set("node1", 2)
c2.Set("node2", 5)

if c1.Concurrent(c2) {
    // Handle conflict (e.g., merge, last-write-wins, prompt user)
}
```

### 9. String Parsing

**Behavior**:
- `ParseClock` is inverse of `String()`
- Whitespace tolerant
- Returns error on invalid format
- **Not for production serialization** (use protocol buffers, JSON, etc.)

**Example**:
```go
c, err := vclock.ParseClock("{alice:5, bob:3}")
// Round-trip: ParseClock(c.String()) == c
```

### 10. Comparison Symmetry

**Behavior**:
- `c1.Compare(c2) == Before` ⟺ `c2.Compare(c1) == After`
- `Equal` and `Concurrent` are symmetric

**Verified**: All comparison tests check reverse comparison.

## Usage Patterns

### Message Passing Protocol

```go
// Sender
sender := vclock.NewClock()
sender.Increment("sender")
message := Message{
    Data:  payload,
    Clock: sender.Copy(), // Attach clock to message
}

// Receiver
receiver := vclock.NewClock()
func handleMessage(msg Message) {
    receiver.Merge(msg.Clock)   // Merge sender's clock
    receiver.Increment("receiver") // Increment own clock
    // Process message...
}
```

### Conflict Detection

```go
func detectConflict(c1, c2 *vclock.Clock) bool {
    return c1.Concurrent(c2)
}

func resolveConflict(c1, c2 *vclock.Clock) *vclock.Clock {
    merged := c1.Copy()
    merged.Merge(c2)
    // Apply application-specific resolution
    return merged
}
```

### External Locking for Concurrency

```go
type SafeClock struct {
    mu    sync.RWMutex
    clock *vclock.Clock
}

func (s *SafeClock) Increment(node vclock.NodeID) int64 {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.clock.Increment(node)
}

func (s *SafeClock) Snapshot() *vclock.Clock {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.clock.Copy()
}
```

### Event Ordering

```go
type Event struct {
    ID    string
    Data  interface{}
    Clock *vclock.Clock
}

func orderEvents(e1, e2 Event) string {
    switch e1.Clock.Compare(e2.Clock) {
    case vclock.BeforeCmp:
        return "e1 -> e2"
    case vclock.AfterCmp:
        return "e2 -> e1"
    case vclock.ConcurrentCmp:
        return "e1 || e2"
    case vclock.EqualCmp:
        return "e1 == e2"
    }
    return "unknown"
}
```

## Performance Characteristics

| Operation | Time Complexity | Space Complexity | Notes |
|-----------|----------------|------------------|-------|
| `NewClock()` | O(n) | O(n) | n = number of initial nodes |
| `Increment()` | O(1) | O(1) | Amortized for map growth |
| `Get()` | O(1) | O(1) | Map lookup |
| `Set()` | O(1) | O(1) | Amortized for map growth |
| `Merge()` | O(m) | O(k) | m = other.Len(), k = new nodes |
| `Compare()` | O(n+m) | O(n+m) | n,m = Len() of clocks |
| `Copy()` | O(n) | O(n) | n = Len() |
| `Nodes()` | O(n log n) | O(n) | Sorting for determinism |
| `String()` | O(n log n) | O(n) | Sorting + string building |

**Typical Node Counts**: 3-100 nodes  
**Memory**: ~40 bytes + 16 bytes per node (map overhead)

## Testing

Run all tests:
```bash
go test -v
```

Run with coverage:
```bash
go test -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Run benchmarks:
```bash
go test -bench=. -benchmem
```

Run examples:
```bash
go test -run=Example
```

## Thread Safety

**This package does NOT provide internal synchronization.** Clocks are safe for concurrent reads with no concurrent writes, but concurrent writes require external locking.

### Correct Concurrent Usage

```go
var mu sync.Mutex
clock := vclock.NewClock()

// Concurrent reads (no writes) - SAFE without lock
value := clock.Get("node1")

// Concurrent writes - REQUIRES lock
mu.Lock()
clock.Increment("node1")
mu.Unlock()

// Atomic read-modify-write - REQUIRES lock
mu.Lock()
snapshot := clock.Copy()
snapshot.Merge(otherClock)
clock = snapshot
mu.Unlock()
```

### Why No Internal Locks?

1. **Performance**: Avoids lock overhead when not needed
2. **Composability**: Lets callers batch operations atomically
3. **Flexibility**: Supports custom synchronization strategies
4. **Testability**: Deterministic without lock contention

## Alternatives Considered

### 1. Hybrid Logical Clocks (HLC)

**Tradeoff**: HLCs provide clock-like timestamps but require wall-clock synchronization. Vector clocks are pure logical time.

**Use Vector Clocks When**: You need causality without clock sync, or have many concurrent writes.

**Use HLC When**: You need human-readable timestamps and bounded clock size.

### 2. Dotted Version Vectors (DVV)

**Tradeoff**: DVVs handle concurrent writes more precisely but are more complex.

**Use Version Vectors When**: Standard Dynamo/Riak semantics are sufficient.

**Use DVV When**: You need precise per-key causality in key-value stores with complex write patterns.

### 3. Logical Timestamps

**Tradeoff**: Simpler but only provide total ordering, not causality.

**Use Version Vectors When**: You need to detect conflicts and causality.

**Use Logical Timestamps When**: Simple ordering is sufficient (single writer, append-only logs).

## Version Vector Examples

### Shopping Cart (Dynamo/Riak)

```go
type CartItem struct {
    ProductID string
    Quantity  int
}

type ShoppingCart struct {
    UserID  string
    Items   []CartItem
    Version *vclock.VersionVector
}

// Add item to cart at replica A
cart := &ShoppingCart{
    UserID:  "user123",
    Version: vclock.NewVersionVector(),
}
cart.Version.Increment("replicaA")

// Concurrent update at replica B
cartB := cart // (received via replication)
cartB.Version.Increment("replicaB")

// Detect conflict
if cart.Version.Concurrent(cartB.Version) {
    // Merge carts and versions
    merged := mergeCartItems(cart.Items, cartB.Items)
    mergedVersion := cart.Version.Copy()
    mergedVersion.Merge(cartB.Version)
    mergedVersion.Increment("coordinator") // Resolution marker
}
```

### Multi-Master Replication

```go
type Document struct {
    Key     string
    Content string
    Version *vclock.VersionVector
}

// Write at master A
docA := &Document{
    Key:     "doc1",
    Content: "Version A",
    Version: vclock.NewVersionVector(),
}
docA.Version.Increment("masterA")

// Replicate and write at master B
docB := docA // (received via replication)
docB.Content = "Version B"
docB.Version.Increment("masterB")

// On read, detect conflict
if docA.Version.Concurrent(docB.Version) {
    // Application-specific resolution
    // Option 1: Last-write-wins (merge + timestamp)
    // Option 2: Keep both as siblings
    // Option 3: Semantic merge of content
}
```

## License

MIT License - See LICENSE file

## Contributing

Contributions welcome! Please ensure:
- All tests pass (`go test`)
- Code is formatted (`go fmt`)
- New features include tests and examples
- Design decisions are documented

## References

- Lamport, L. (1978). "Time, Clocks, and the Ordering of Events in a Distributed System"
- Fidge, C. (1988). "Timestamps in Message-Passing Systems That Preserve the Partial Ordering"
- Mattern, F. (1988). "Virtual Time and Global States of Distributed Systems"

## FAQ

**Q: When should I use vector clocks vs. timestamps?**  
A: Use vector clocks when:
- You need to detect causality violations
- Nodes don't have synchronized clocks  
- You handle concurrent writes

Use timestamps when:
- You only need total ordering
- Clocks are synchronized (NTP)
- You can tolerate small skew

**Q: How do I serialize clocks for network transmission?**  
A: Use a proper serialization format:
```go
// Option 1: JSON
data, _ := json.Marshal(clock)

// Option 2: Protocol Buffers (recommended for performance)
// Define .proto message with map<string, int64> field

// Option 3: Custom binary format
// See implementation of String() for inspiration
```

**Q: Can I use this with databases?**  
A: Yes. Store clocks as JSON, binary blobs, or decomposed into rows. Comparisons can detect write conflicts in optimistic locking schemes.

**Q: What's the maximum number of nodes?**  
A: Practically unlimited, but performance degrades linearly. Typical systems have 3-100 nodes. For 1000+ nodes, consider Dotted Version Vectors or other compressed variants.

**Q: How do I handle node failures?**  
A: Vector clocks themselves don't detect failures. Combine with failure detectors or heartbeats. Offline nodes simply stop incrementing their clock value.

**Q: Should I garbage collect old nodes?**  
A: Depends on your application. If nodes are ephemeral, you may want to prune nodes not seen for N operations. This requires application-level logic.
