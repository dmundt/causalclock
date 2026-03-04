package vclock

import (
	"fmt"
)

// Example demonstrates basic vector clock operations.
func Example() {
	// Create a clock for node A
	clockA := NewClock()

	// Node A does some work
	clockA.Increment("A")
	clockA.Increment("A")

	fmt.Println("Node A:", clockA)

	// Create a clock for node B
	clockB := NewClock()
	clockB.Increment("B")

	fmt.Println("Node B:", clockB)

	// Compare clocks
	fmt.Println("Relationship:", clockA.Compare(clockB))

	// Output:
	// Node A: {A:2}
	// Node B: {B:1}
	// Relationship: Concurrent
}

// ExampleClock_Increment demonstrates incrementing a node's clock value.
func ExampleClock_Increment() {
	c := NewClock()

	// Each increment returns the new value
	v1 := c.Increment("node1")
	v2 := c.Increment("node1")
	v3 := c.Increment("node1")

	fmt.Printf("Values: %d, %d, %d\n", v1, v2, v3)
	fmt.Println("Clock:", c)

	// Output:
	// Values: 1, 2, 3
	// Clock: {node1:3}
}

// ExampleClock_Merge demonstrates merging vector clocks when receiving a message.
func ExampleClock_Merge() {
	// Sender's clock
	sender := NewClock()
	sender.Set("alice", 5)
	sender.Set("bob", 3)

	// Receiver's clock
	receiver := NewClock()
	receiver.Set("alice", 2)
	receiver.Set("bob", 7)
	receiver.Set("charlie", 4)

	fmt.Println("Before merge:", receiver)

	// Receiver merges sender's clock (element-wise maximum)
	receiver.Merge(sender)

	fmt.Println("After merge:", receiver)

	// Output:
	// Before merge: {alice:2, bob:7, charlie:4}
	// After merge: {alice:5, bob:7, charlie:4}
}

// ExampleClock_Compare demonstrates comparing vector clocks for causality.
func ExampleClock_Compare() {
	c1 := NewClock()
	c1.Set("alice", 2)
	c1.Set("bob", 3)

	c2 := NewClock()
	c2.Set("alice", 5)
	c2.Set("bob", 3)

	c3 := NewClock()
	c3.Set("alice", 5)
	c3.Set("bob", 2)

	// c1 happened-before c2 (c1 <= c2 and c1 < c2 for alice)
	fmt.Println("c1 vs c2:", c1.Compare(c2))

	// c1 concurrent with c3 (alice: c1 < c3, bob: c1 > c3)
	fmt.Println("c1 vs c3:", c1.Compare(c3))

	// c1 equal to itself
	fmt.Println("c1 vs c1:", c1.Compare(c1))

	// Output:
	// c1 vs c2: Before
	// c1 vs c3: Concurrent
	// c1 vs c1: Equal
}

// ExampleClock_Copy demonstrates creating immutable snapshots.
func ExampleClock_Copy() {
	original := NewClock()
	original.Set("node1", 10)

	// Create a snapshot before mutation
	snapshot := original.Copy()

	// Mutate the original
	original.Set("node1", 999)
	original.Set("node2", 20)

	fmt.Println("Original:", original)
	fmt.Println("Snapshot:", snapshot)

	// Output:
	// Original: {node1:999, node2:20}
	// Snapshot: {node1:10}
}

// Example_messagePassingProtocol demonstrates a typical message-passing scenario.
func Example_messagePassingProtocol() {
	// Initialize clocks for three nodes
	alice := NewClock("alice", "bob", "charlie")
	bob := NewClock("alice", "bob", "charlie")
	charlie := NewClock("alice", "bob", "charlie")

	// Alice does local work
	alice.Increment("alice")
	alice.Increment("alice")
	fmt.Println("Alice after work:", alice)

	// Alice sends message to Bob (Bob receives Alice's clock)
	bob.Merge(alice)
	bob.Increment("bob") // Bob processes the message
	fmt.Println("Bob after receiving from Alice:", bob)

	// Charlie does independent work
	charlie.Increment("charlie")
	fmt.Println("Charlie after work:", charlie)

	// Check causality
	fmt.Println("Bob happened-after Alice?", bob.HappenedAfter(alice))
	fmt.Println("Charlie concurrent with Bob?", charlie.Concurrent(bob))

	// Output:
	// Alice after work: {alice:2, bob:0, charlie:0}
	// Bob after receiving from Alice: {alice:2, bob:1, charlie:0}
	// Charlie after work: {alice:0, bob:0, charlie:1}
	// Bob happened-after Alice? true
	// Charlie concurrent with Bob? true
}

// Example_conflictDetection shows how to detect concurrent updates (conflicts).
func Example_conflictDetection() {
	// Two replicas start with the same state
	replica1 := NewClock()
	replica1.Set("replica1", 1)
	replica1.Set("replica2", 1)

	replica2 := replica1.Copy()

	// Both replicas make concurrent updates
	replica1.Increment("replica1")
	replica2.Increment("replica2")

	// Detect conflict
	if replica1.Concurrent(replica2) {
		fmt.Println("Conflict detected! Concurrent updates.")
		fmt.Println("Replica 1:", replica1)
		fmt.Println("Replica 2:", replica2)

		// Resolve by merging (last-write-wins or application-specific logic)
		merged := replica1.Copy()
		merged.Merge(replica2)
		fmt.Println("Merged:", merged)
	}

	// Output:
	// Conflict detected! Concurrent updates.
	// Replica 1: {replica1:2, replica2:1}
	// Replica 2: {replica1:1, replica2:2}
	// Merged: {replica1:2, replica2:2}
}

// Example_eventOrdering demonstrates using vector clocks for event ordering.
func Example_eventOrdering() {
	// Create events with their vector clocks
	event1 := NewClock()
	event1.Set("node1", 1)

	event2 := NewClock()
	event2.Set("node1", 2)
	event2.Set("node2", 1)

	event3 := NewClock()
	event3.Set("node1", 0)
	event3.Set("node2", 1)

	// Determine causality
	fmt.Println("Event1 -> Event2:", event1.HappenedBefore(event2))
	fmt.Println("Event1 || Event3:", event1.Concurrent(event3))
	fmt.Println("Event2 -> Event3:", event2.HappenedAfter(event3))

	// Output:
	// Event1 -> Event2: true
	// Event1 || Event3: true
	// Event2 -> Event3: true
}

// Example_externalLocking demonstrates the concurrency pattern with external locks.
func Example_externalLocking() {
	// In production, use sync.Mutex or sync.RWMutex
	var mu struct{ mu interface{} } // Simplified for example
	_ = mu

	clock := NewClock()

	// Read operation pattern:
	// mu.Lock()
	value := clock.Get("node1")
	// mu.Unlock()
	fmt.Printf("Read value for node1: %d\n", value)

	// Write operation pattern:
	// mu.Lock()
	clock.Increment("node1")
	// mu.Unlock()

	// Atomic read-modify-write pattern:
	// mu.Lock()
	snapshot := clock.Copy()
	otherClock := NewClock()
	snapshot.Merge(otherClock)
	clock = snapshot
	// mu.Unlock()

	fmt.Printf("Clock after merge: %v\n", clock)

	// Output:
	// Read value for node1: 0
	// Clock after merge: {node1:1}
}

// Example_stableOrdering demonstrates deterministic iteration.
func Example_stableOrdering() {
	c := NewClock()
	c.Set("zebra", 26)
	c.Set("alpha", 1)
	c.Set("gamma", 3)
	c.Set("beta", 2)

	// Nodes() always returns the same order
	nodes := c.Nodes()
	fmt.Println("Nodes (sorted):", nodes)

	// String() is also deterministic
	fmt.Println("Clock:", c)

	// Output:
	// Nodes (sorted): [alpha beta gamma zebra]
	// Clock: {alpha:1, beta:2, gamma:3, zebra:26}
}

// Example_edgeCases demonstrates edge case handling.
func Example_edgeCases() {
	c := NewClock()

	// Getting non-existent node returns 0
	fmt.Println("Non-existent node:", c.Get("missing"))

	// Incrementing non-existent node initializes to 1
	fmt.Println("First increment:", c.Increment("new"))

	// Setting to 0 keeps the entry
	c.Set("new", 0)
	fmt.Println("After Set(0), Len:", c.Len())

	// Empty clock comparisons
	empty1 := NewClock()
	empty2 := NewClock()
	fmt.Println("Empty == Empty:", empty1.Equal(empty2))

	// Nil clock handling
	var nilClock *Clock
	fmt.Println("Nil clock nodes:", len(nilClock.Nodes()))
	fmt.Println("Nil.Compare(Nil):", nilClock.Compare(nilClock))

	// Output:
	// Non-existent node: 0
	// First increment: 1
	// After Set(0), Len: 1
	// Empty == Empty: true
	// Nil clock nodes: 0
	// Nil.Compare(Nil): Equal
}
