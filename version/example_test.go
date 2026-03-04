package vvector

import (
	"fmt"
)

// ExampleVersionVector demonstrates basic version vector operations.
func ExampleVersionVector() {
	// Replica A creates a new version
	versionA := NewVersionVector()
	versionA.Increment("A")
	
	fmt.Println("Version A:", versionA)
	
	// Replica B receives and updates
	versionB := versionA.Copy()
	versionB.Increment("B")
	
	fmt.Println("Version B:", versionB)
	
	// Compare versions
	fmt.Println("A -> B:", versionA.HappenedBefore(versionB))
	
	// Output:
	// Version A: {A:1}
	// Version B: {A:1, B:1}
	// A -> B: true
}

// ExampleVersionVector_Increment demonstrates incrementing version counters.
func ExampleVersionVector_Increment() {
	vv := NewVersionVector()
	
	// Replica increments its own counter on each update
	v1 := vv.Increment("replica1")
	v2 := vv.Increment("replica1")
	v3 := vv.Increment("replica1")
	
	fmt.Printf("Values: %d, %d, %d\n", v1, v2, v3)
	fmt.Println("Vector:", vv)
	
	// Output:
	// Values: 1, 2, 3
	// Vector: {replica1:3}
}

// ExampleVersionVector_Merge demonstrates Dynamo/Riak merge semantics.
func ExampleVersionVector_Merge() {
	// Version from replica A
	versionA := NewVersionVector()
	versionA.Set("A", 5)
	versionA.Set("B", 3)
	
	// Version from replica B
	versionB := NewVersionVector()
	versionB.Set("A", 2)
	versionB.Set("B", 7)
	versionB.Set("C", 4)
	
	fmt.Println("Before merge:", versionB)
	
	// Merge takes element-wise maximum
	versionB.Merge(versionA)
	
	fmt.Println("After merge:", versionB)
	
	// Output:
	// Before merge: {A:2, B:7, C:4}
	// After merge: {A:5, B:7, C:4}
}

// ExampleVersionVector_Compare demonstrates comparing version vectors.
func ExampleVersionVector_Compare() {
	vv1 := NewVersionVector()
	vv1.Set("A", 2)
	vv1.Set("B", 3)
	
	vv2 := NewVersionVector()
	vv2.Set("A", 5)
	vv2.Set("B", 3)
	
	vv3 := NewVersionVector()
	vv3.Set("A", 5)
	vv3.Set("B", 2)
	
	// vv1 happened-before vv2 (vv1 <= vv2 and vv1 < vv2 for A)
	fmt.Println("vv1 vs vv2:", vv1.Compare(vv2))
	
	// vv1 concurrent with vv3 (A: vv1 < vv3, B: vv1 > vv3)
	fmt.Println("vv1 vs vv3:", vv1.Compare(vv3))
	
	// vv1 equal to itself
	fmt.Println("vv1 vs vv1:", vv1.Compare(vv1))
	
	// Output:
	// vv1 vs vv2: Before
	// vv1 vs vv3: Concurrent
	// vv1 vs vv1: Equal
}

// ExampleVersionVector_Copy demonstrates creating snapshots.
func ExampleVersionVector_Copy() {
	original := NewVersionVector()
	original.Set("replica1", 10)
	
	// Create a snapshot before mutation
	snapshot := original.Copy()
	
	// Mutate the original
	original.Set("replica1", 999)
	original.Set("replica2", 20)
	
	fmt.Println("Original:", original)
	fmt.Println("Snapshot:", snapshot)
	
	// Output:
	// Original: {replica1:999, replica2:20}
	// Snapshot: {replica1:10}
}

// Example_dynamoReplication demonstrates a Dynamo-style replication scenario.
func Example_dynamoReplication() {
	// Initial write to replica A
	versionA := NewVersionVector()
	versionA.Increment("A")
	fmt.Println("Initial write at A:", versionA)
	
	// Replicate to B and C
	versionB := versionA.Copy()
	versionC := versionA.Copy()
	
	// Client writes to B
	versionB.Increment("B")
	fmt.Println("Write at B:", versionB)
	
	// Client writes to C
	versionC.Increment("C")
	fmt.Println("Write at C:", versionC)
	
	// Check for conflicts
	if versionB.Concurrent(versionC) {
		fmt.Println("Conflict detected!")
		
		// Resolve by merging
		merged := versionB.Copy()
		merged.Merge(versionC)
		fmt.Println("Merged version:", merged)
	}
	
	// Output:
	// Initial write at A: {A:1}
	// Write at B: {A:1, B:1}
	// Write at C: {A:1, C:1}
	// Conflict detected!
	// Merged version: {A:1, B:1, C:1}
}

// Example_versionVectorConflictDetection shows how to detect and handle conflicts.
func Example_versionVectorConflictDetection() {
	// Two replicas start with same version
	replica1 := NewVersionVector()
	replica1.Set("A", 1)
	replica1.Set("B", 1)
	
	replica2 := replica1.Copy()
	
	// Concurrent updates
	replica1.Increment("A")
	replica2.Increment("B")
	
	// Detect conflict
	if replica1.Concurrent(replica2) {
		fmt.Println("Conflict detected!")
		fmt.Println("Replica 1:", replica1)
		fmt.Println("Replica 2:", replica2)
		
		// Application-specific resolution
		// Option 1: Merge and increment a coordinator
		resolved := replica1.Copy()
		resolved.Merge(replica2)
		resolved.Increment("coordinator")
		
		fmt.Println("Resolved:", resolved)
	}
	
	// Output:
	// Conflict detected!
	// Replica 1: {A:2, B:1}
	// Replica 2: {A:1, B:2}
	// Resolved: {A:2, B:2, coordinator:1}
}

// Example_riakReadRepair demonstrates read repair in Riak-style systems.
func Example_riakReadRepair() {
	// Three replicas with different versions
	version1 := NewVersionVector()
	version1.Set("A", 5)
	version1.Set("B", 3)
	
	version2 := NewVersionVector()
	version2.Set("A", 5)
	version2.Set("B", 4)
	version2.Set("C", 1)
	
	version3 := NewVersionVector()
	version3.Set("A", 6)
	version3.Set("B", 3)
	
	// Read from multiple replicas
	versions := []*VersionVector{version1, version2, version3}
	
	// Find all non-dominated versions (siblings)
	var siblings []*VersionVector
	for i, v1 := range versions {
		dominated := false
		for j, v2 := range versions {
			if i != j && v1.IsDominatedBy(v2) {
				dominated = true
				break
			}
		}
		if !dominated {
			siblings = append(siblings, v1)
		}
	}
	
	fmt.Printf("Found %d sibling(s)\n", len(siblings))
	for i, sibling := range siblings {
		fmt.Printf("Sibling %d: %s\n", i+1, sibling)
	}
	
	// Output:
	// Found 2 sibling(s)
	// Sibling 1: {A:5, B:4, C:1}
	// Sibling 2: {A:6, B:3}
}

// Example_causalityChain demonstrates a chain of causal dependencies.
func Example_causalityChain() {
	// Event 1: Initial write
	v1 := NewVersionVector()
	v1.Increment("A")
	
	// Event 2: Read v1, write
	v2 := v1.Copy()
	v2.Increment("B")
	
	// Event 3: Read v2, write
	v3 := v2.Copy()
	v3.Increment("C")
	
	// Verify causal chain
	fmt.Println("v1 -> v2:", v1.HappenedBefore(v2))
	fmt.Println("v2 -> v3:", v2.HappenedBefore(v3))
	fmt.Println("v1 -> v3:", v1.HappenedBefore(v3))
	
	// v3 dominates all previous versions
	fmt.Println("v3 dominates v1:", v3.Dominates(v1))
	fmt.Println("v3 dominates v2:", v3.Dominates(v2))
	
	// Output:
	// v1 -> v2: true
	// v2 -> v3: true
	// v1 -> v3: true
	// v3 dominates v1: true
	// v3 dominates v2: true
}

// Example_lastWriteWins demonstrates last-write-wins conflict resolution.
func Example_lastWriteWins() {
	// Two concurrent versions
	v1 := NewVersionVector()
	v1.Set("A", 5)
	v1.Set("B", 2)
	
	v2 := NewVersionVector()
	v2.Set("A", 3)
	v2.Set("B", 6)
	
	if v1.Concurrent(v2) {
		fmt.Println("Concurrent versions detected")
		
		// Merge to create a new version that dominates both
		lww := v1.Copy()
		lww.Merge(v2)
		
		// Increment to show this is a resolution
		lww.Increment("resolver")
		
		fmt.Println("LWW version:", lww)
		fmt.Println("Dominates v1:", lww.Dominates(v1))
		fmt.Println("Dominates v2:", lww.Dominates(v2))
	}
	
	// Output:
	// Concurrent versions detected
	// LWW version: {A:5, B:6, resolver:1}
	// Dominates v1: true
	// Dominates v2: true
}

// Example_perObjectTracking demonstrates per-object version tracking.
func Example_perObjectTracking() {
	// Each object has its own version vector
	type Object struct {
		Key     string
		Value   string
		Version *VersionVector
	}
	
	// Create object at replica A
	obj := Object{
		Key:     "user:123",
		Value:   "Alice",
		Version: NewVersionVector(),
	}
	obj.Version.Increment("A")
	
	fmt.Printf("Created: %s = %s, version: %s\n", obj.Key, obj.Value, obj.Version)
	
	// Update at replica B
	obj.Value = "Alice Smith"
	obj.Version.Increment("B")
	
	fmt.Printf("Updated: %s = %s, version: %s\n", obj.Key, obj.Value, obj.Version)
	
	// Output:
	// Created: user:123 = Alice, version: {A:1}
	// Updated: user:123 = Alice Smith, version: {A:1, B:1}
}

// Example_versionVectorRelations demonstrates all four comparison relations.
func Example_versionVectorRelations() {
	// Equal: identical versions
	v1 := NewVersionVector()
	v1.Set("A", 3)
	v1.Set("B", 2)
	
	v2 := v1.Copy()
	fmt.Println("Equal:", v1.Equal(v2))
	
	// Before: v3 is ancestor of v4
	v3 := NewVersionVector()
	v3.Set("A", 3)
	v3.Set("B", 2)
	
	v4 := v3.Copy()
	v4.Increment("B")
	fmt.Println("Before:", v3.HappenedBefore(v4))
	
	// After: v5 is descendant of v6
	v5 := v4.Copy()
	v6 := v3.Copy()
	fmt.Println("After:", v5.HappenedAfter(v6))
	
	// Concurrent: divergent versions
	v7 := v3.Copy()
	v7.Increment("A")
	
	v8 := v3.Copy()
	v8.Increment("B")
	fmt.Println("Concurrent:", v7.Concurrent(v8))
	
	// Output:
	// Equal: true
	// Before: true
	// After: true
	// Concurrent: true
}

// Example_versionVectorStableOrdering demonstrates deterministic iteration.
func Example_versionVectorStableOrdering() {
	vv := NewVersionVector()
	vv.Set("zebra", 26)
	vv.Set("alpha", 1)
	vv.Set("gamma", 3)
	vv.Set("beta", 2)
	
	// Replicas() always returns the same order
	replicas := vv.Replicas()
	fmt.Println("Replicas (sorted):", replicas)
	
	// String() is also deterministic
	fmt.Println("Vector:", vv)
	
	// Output:
	// Replicas (sorted): [alpha beta gamma zebra]
	// Vector: {alpha:1, beta:2, gamma:3, zebra:26}
}

// Example_versionVectorEdgeCases demonstrates edge case handling.
func Example_versionVectorEdgeCases() {
	vv := NewVersionVector()
	
	// Getting non-existent replica returns 0
	fmt.Println("Non-existent replica:", vv.Get("missing"))
	
	// Incrementing non-existent replica initializes to 1
	fmt.Println("First increment:", vv.Increment("new"))
	
	// Setting to 0 keeps the entry
	vv.Set("new", 0)
	fmt.Println("After Set(0), Len:", vv.Len())
	
	// Empty vector comparisons
	empty1 := NewVersionVector()
	empty2 := NewVersionVector()
	fmt.Println("Empty == Empty:", empty1.Equal(empty2))
	
	// Nil vector handling
	var nilVector *VersionVector
	fmt.Println("Nil vector replicas:", len(nilVector.Replicas()))
	fmt.Println("Nil.Compare(Nil):", nilVector.Compare(nilVector))
	
	// Output:
	// Non-existent replica: 0
	// First increment: 1
	// After Set(0), Len: 1
	// Empty == Empty: true
	// Nil vector replicas: 0
	// Nil.Compare(Nil): Equal
}
