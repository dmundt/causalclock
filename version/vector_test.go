package vvector

import (
	"fmt"
	"testing"

	"github.com/dmundt/causalclock/clock"
)

// TestNewVersionVector verifies that NewVersionVector creates properly initialized vectors.
func TestNewVersionVector(t *testing.T) {
	tests := []struct {
		name     string
		replicas []ReplicaID
		want     int
	}{
		{
			name:     "empty vector",
			replicas: nil,
			want:     0,
		},
		{
			name:     "single replica",
			replicas: []ReplicaID{"replica1"},
			want:     1,
		},
		{
			name:     "multiple replicas",
			replicas: []ReplicaID{"replica1", "replica2", "replica3"},
			want:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vv := NewVersionVector(tt.replicas...)
			if vv == nil {
				t.Fatal("NewVersionVector returned nil")
			}
			if got := vv.Len(); got != tt.want {
				t.Errorf("Len() = %d, want %d", got, tt.want)
			}

			// Verify all replicas are initialized to zero
			for _, replica := range tt.replicas {
				if got := vv.Get(replica); got != 0 {
					t.Errorf("Get(%s) = %d, want 0", replica, got)
				}
			}
		})
	}
}

// TestVersionVectorIncrement verifies increment operations.
func TestVersionVectorIncrement(t *testing.T) {
	vv := NewVersionVector()

	// First increment should set to 1
	if got := vv.Increment("replica1"); got != 1 {
		t.Errorf("First Increment() = %d, want 1", got)
	}

	// Second increment should set to 2
	if got := vv.Increment("replica1"); got != 2 {
		t.Errorf("Second Increment() = %d, want 2", got)
	}

	// Different replica should start at 1
	if got := vv.Increment("replica2"); got != 1 {
		t.Errorf("Increment(replica2) = %d, want 1", got)
	}

	// Verify state
	if vv.Get("replica1") != 2 {
		t.Errorf("Get(replica1) = %d, want 2", vv.Get("replica1"))
	}
	if vv.Get("replica2") != 1 {
		t.Errorf("Get(replica2) = %d, want 1", vv.Get("replica2"))
	}
}

// TestVersionVectorGetSet verifies get and set operations.
func TestVersionVectorGetSet(t *testing.T) {
	vv := NewVersionVector()

	// Get on empty vector should return 0
	if got := vv.Get("nonexistent"); got != 0 {
		t.Errorf("Get(nonexistent) = %d, want 0", got)
	}

	// Set and retrieve
	vv.Set("replica1", 42)
	if got := vv.Get("replica1"); got != 42 {
		t.Errorf("Get(replica1) = %d, want 42", got)
	}

	// Set to zero (should keep the entry)
	vv.Set("replica1", 0)
	if got := vv.Get("replica1"); got != 0 {
		t.Errorf("Get(replica1) after Set(0) = %d, want 0", got)
	}
	if vv.Len() != 1 {
		t.Errorf("Len() after Set(0) = %d, want 1 (entry should remain)", vv.Len())
	}
}

// TestVersionVectorCopy verifies deep copying.
func TestVersionVectorCopy(t *testing.T) {
	original := NewVersionVector()
	original.Set("replica1", 10)
	original.Set("replica2", 20)

	copy := original.Copy()

	// Verify copy has same values
	if copy.Get("replica1") != 10 {
		t.Errorf("Copy Get(replica1) = %d, want 10", copy.Get("replica1"))
	}
	if copy.Get("replica2") != 20 {
		t.Errorf("Copy Get(replica2) = %d, want 20", copy.Get("replica2"))
	}

	// Mutate original
	original.Set("replica1", 999)
	original.Set("replica3", 30)

	// Verify copy is unaffected
	if copy.Get("replica1") != 10 {
		t.Errorf("Copy Get(replica1) after mutation = %d, want 10", copy.Get("replica1"))
	}
	if copy.Get("replica3") != 0 {
		t.Errorf("Copy Get(replica3) after mutation = %d, want 0", copy.Get("replica3"))
	}

	// Test copying nil vector
	var nilVector *VersionVector
	copyNil := nilVector.Copy()
	if copyNil == nil {
		t.Error("Copy of nil vector returned nil, want empty vector")
	}
	if copyNil.Len() != 0 {
		t.Errorf("Copy of nil vector Len() = %d, want 0", copyNil.Len())
	}
}

// TestVersionVectorMerge verifies merge operations with Dynamo/Riak semantics.
func TestVersionVectorMerge(t *testing.T) {
	tests := []struct {
		name     string
		vector1  map[ReplicaID]uint64
		vector2  map[ReplicaID]uint64
		expected map[ReplicaID]uint64
	}{
		{
			name:     "empty vectors",
			vector1:  map[ReplicaID]uint64{},
			vector2:  map[ReplicaID]uint64{},
			expected: map[ReplicaID]uint64{},
		},
		{
			name:     "merge with empty",
			vector1:  map[ReplicaID]uint64{"replica1": 5},
			vector2:  map[ReplicaID]uint64{},
			expected: map[ReplicaID]uint64{"replica1": 5},
		},
		{
			name:     "merge into empty",
			vector1:  map[ReplicaID]uint64{},
			vector2:  map[ReplicaID]uint64{"replica1": 5},
			expected: map[ReplicaID]uint64{"replica1": 5},
		},
		{
			name:     "element-wise maximum (Dynamo semantics)",
			vector1:  map[ReplicaID]uint64{"replica1": 5, "replica2": 2, "replica3": 8},
			vector2:  map[ReplicaID]uint64{"replica1": 3, "replica2": 7, "replica4": 1},
			expected: map[ReplicaID]uint64{"replica1": 5, "replica2": 7, "replica3": 8, "replica4": 1},
		},
		{
			name:     "identical vectors",
			vector1:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			vector2:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			expected: map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
		},
		{
			name:     "disjoint replicas",
			vector1:  map[ReplicaID]uint64{"replicaA": 10},
			vector2:  map[ReplicaID]uint64{"replicaB": 20},
			expected: map[ReplicaID]uint64{"replicaA": 10, "replicaB": 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vv1 := NewVersionVector()
			for k, v := range tt.vector1 {
				vv1.Set(k, v)
			}

			vv2 := NewVersionVector()
			for k, v := range tt.vector2 {
				vv2.Set(k, v)
			}

			vv1.Merge(vv2)

			// Verify merge result
			for replica, expected := range tt.expected {
				if got := vv1.Get(replica); got != expected {
					t.Errorf("After merge, Get(%s) = %d, want %d", replica, got, expected)
				}
			}

			// Verify no extra replicas
			if vv1.Len() != len(tt.expected) {
				t.Errorf("After merge, Len() = %d, want %d", vv1.Len(), len(tt.expected))
			}
		})
	}

	// Test merging with nil
	vv := NewVersionVector()
	vv.Set("replica1", 5)
	vv.Merge(nil)
	if vv.Get("replica1") != 5 {
		t.Errorf("Merge(nil) affected vector: Get(replica1) = %d, want 5", vv.Get("replica1"))
	}
}

// TestVersionVectorCompare verifies all comparison scenarios.
func TestVersionVectorCompare(t *testing.T) {
	tests := []struct {
		name     string
		vector1  map[ReplicaID]uint64
		vector2  map[ReplicaID]uint64
		expected clock.Comparison
	}{
		{
			name:     "both empty",
			vector1:  map[ReplicaID]uint64{},
			vector2:  map[ReplicaID]uint64{},
			expected: clock.EqualCmp,
		},
		{
			name:     "identical vectors",
			vector1:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			vector2:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			expected: clock.EqualCmp,
		},
		{
			name:     "vector1 before vector2",
			vector1:  map[ReplicaID]uint64{"replica1": 2, "replica2": 3},
			vector2:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			expected: clock.BeforeCmp,
		},
		{
			name:     "vector1 after vector2",
			vector1:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			vector2:  map[ReplicaID]uint64{"replica1": 2, "replica2": 3},
			expected: clock.AfterCmp,
		},
		{
			name:     "concurrent vectors (classic conflict)",
			vector1:  map[ReplicaID]uint64{"replica1": 5, "replica2": 2},
			vector2:  map[ReplicaID]uint64{"replica1": 2, "replica2": 5},
			expected: clock.ConcurrentCmp,
		},
		{
			name:     "vector1 empty, vector2 has zero",
			vector1:  map[ReplicaID]uint64{},
			vector2:  map[ReplicaID]uint64{"replica1": 0},
			expected: clock.EqualCmp,
		},
		{
			name:     "vector1 empty, vector2 has non-zero",
			vector1:  map[ReplicaID]uint64{},
			vector2:  map[ReplicaID]uint64{"replica1": 5},
			expected: clock.BeforeCmp,
		},
		{
			name:     "vector1 has non-zero, vector2 empty",
			vector1:  map[ReplicaID]uint64{"replica1": 5},
			vector2:  map[ReplicaID]uint64{},
			expected: clock.AfterCmp,
		},
		{
			name:     "different replica sets - before",
			vector1:  map[ReplicaID]uint64{"replica1": 2},
			vector2:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			expected: clock.BeforeCmp,
		},
		{
			name:     "different replica sets - after",
			vector1:  map[ReplicaID]uint64{"replica1": 5, "replica2": 3},
			vector2:  map[ReplicaID]uint64{"replica1": 2},
			expected: clock.AfterCmp,
		},
		{
			name:     "different replica sets - concurrent",
			vector1:  map[ReplicaID]uint64{"replica1": 5},
			vector2:  map[ReplicaID]uint64{"replica2": 5},
			expected: clock.ConcurrentCmp,
		},
		{
			name:     "all zeros - equal",
			vector1:  map[ReplicaID]uint64{"replica1": 0, "replica2": 0},
			vector2:  map[ReplicaID]uint64{"replica1": 0, "replica2": 0},
			expected: clock.EqualCmp,
		},
		{
			name:     "Dynamo scenario: vector1 strictly dominates",
			vector1:  map[ReplicaID]uint64{"a": 3, "b": 2, "c": 4},
			vector2:  map[ReplicaID]uint64{"a": 2, "b": 1, "c": 4},
			expected: clock.AfterCmp,
		},
		{
			name:     "Dynamo scenario: concurrent writes from different replicas",
			vector1:  map[ReplicaID]uint64{"a": 5, "b": 1, "c": 1},
			vector2:  map[ReplicaID]uint64{"a": 1, "b": 5, "c": 1},
			expected: clock.ConcurrentCmp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vv1 := NewVersionVector()
			for k, v := range tt.vector1 {
				vv1.Set(k, v)
			}

			vv2 := NewVersionVector()
			for k, v := range tt.vector2 {
				vv2.Set(k, v)
			}

			got := vv1.Compare(vv2)
			if got != tt.expected {
				t.Errorf("Compare() = %s, want %s", got, tt.expected)
			}

			// Verify comparison is symmetric for Equal and Concurrent
			reverse := vv2.Compare(vv1)
			switch tt.expected {
			case clock.EqualCmp:
				if reverse != clock.EqualCmp {
					t.Errorf("Reverse Compare() = %s, want EqualCmp (symmetric)", reverse)
				}
			case clock.BeforeCmp:
				if reverse != clock.AfterCmp {
					t.Errorf("Reverse Compare() = %s, want AfterCmp", reverse)
				}
			case clock.AfterCmp:
				if reverse != clock.BeforeCmp {
					t.Errorf("Reverse Compare() = %s, want BeforeCmp", reverse)
				}
			case clock.ConcurrentCmp:
				if reverse != clock.ConcurrentCmp {
					t.Errorf("Reverse Compare() = %s, want ConcurrentCmp (symmetric)", reverse)
				}
			}
		})
	}
}

// TestVersionVectorCompareNil verifies nil handling in comparisons.
func TestVersionVectorCompareNil(t *testing.T) {
	var nilVector *VersionVector
	empty := NewVersionVector()
	nonEmpty := NewVersionVector()
	nonEmpty.Set("replica1", 5)

	// nil vs nil
	if got := nilVector.Compare(nilVector); got != clock.EqualCmp {
		t.Errorf("nil.Compare(nil) = %s, want EqualCmp", got)
	}

	// nil vs empty
	if got := nilVector.Compare(empty); got != clock.EqualCmp {
		t.Errorf("nil.Compare(empty) = %s, want EqualCmp", got)
	}

	// empty vs nil
	if got := empty.Compare(nilVector); got != clock.EqualCmp {
		t.Errorf("empty.Compare(nil) = %s, want EqualCmp", got)
	}

	// nil vs non-empty
	if got := nilVector.Compare(nonEmpty); got != clock.BeforeCmp {
		t.Errorf("nil.Compare(nonEmpty) = %s, want BeforeCmp", got)
	}

	// non-empty vs nil
	if got := nonEmpty.Compare(nilVector); got != clock.AfterCmp {
		t.Errorf("nonEmpty.Compare(nil) = %s, want AfterCmp", got)
	}
}

// TestVersionVectorConvenienceMethods verifies the convenience comparison methods.
func TestVersionVectorConvenienceMethods(t *testing.T) {
	vv1 := NewVersionVector()
	vv1.Set("replica1", 2)

	vv2 := NewVersionVector()
	vv2.Set("replica1", 5)

	vv3 := NewVersionVector()
	vv3.Set("replica1", 2)

	vv4 := NewVersionVector()
	vv4.Set("replica2", 2)

	// Equal
	if !vv1.Equal(vv3) {
		t.Error("Equal() returned false for identical vectors")
	}
	if vv1.Equal(vv2) {
		t.Error("Equal() returned true for different vectors")
	}

	// HappenedBefore
	if !vv1.HappenedBefore(vv2) {
		t.Error("HappenedBefore() returned false when it should be true")
	}
	if vv2.HappenedBefore(vv1) {
		t.Error("HappenedBefore() returned true when it should be false")
	}

	// HappenedAfter
	if !vv2.HappenedAfter(vv1) {
		t.Error("HappenedAfter() returned false when it should be true")
	}
	if vv1.HappenedAfter(vv2) {
		t.Error("HappenedAfter() returned true when it should be false")
	}

	// Concurrent
	if !vv1.Concurrent(vv4) {
		t.Error("Concurrent() returned false for concurrent vectors")
	}
	if vv1.Concurrent(vv2) {
		t.Error("Concurrent() returned true for non-concurrent vectors")
	}

	// Descends
	if !vv2.Descends(vv1) {
		t.Error("Descends() returned false when vv2 > vv1")
	}
	if !vv1.Descends(vv3) {
		t.Error("Descends() returned false for equal vectors")
	}
	if vv1.Descends(vv2) {
		t.Error("Descends() returned true when vv1 < vv2")
	}

	// Dominates
	if !vv2.Dominates(vv1) {
		t.Error("Dominates() returned false when vv2 > vv1")
	}
	if vv1.Dominates(vv2) {
		t.Error("Dominates() returned true when vv1 < vv2")
	}

	// IsDominatedBy
	if !vv1.IsDominatedBy(vv2) {
		t.Error("IsDominatedBy() returned false when vv1 < vv2")
	}
	if vv2.IsDominatedBy(vv1) {
		t.Error("IsDominatedBy() returned true when vv2 > vv1")
	}
}

// TestVersionVectorReplicas verifies the Replicas method.
func TestVersionVectorReplicas(t *testing.T) {
	vv := NewVersionVector()
	vv.Set("charlie", 3)
	vv.Set("alice", 1)
	vv.Set("bob", 2)

	replicas := vv.Replicas()

	// Verify length
	if len(replicas) != 3 {
		t.Errorf("Replicas() length = %d, want 3", len(replicas))
	}

	// Verify sorted order
	expected := []ReplicaID{"alice", "bob", "charlie"}
	for i, replica := range replicas {
		if replica != expected[i] {
			t.Errorf("Replicas()[%d] = %s, want %s", i, replica, expected[i])
		}
	}

	// Verify mutation doesn't affect vector
	replicas[0] = "mutated"
	if vv.Replicas()[0] != "alice" {
		t.Error("Mutating Replicas() result affected the vector")
	}

	// Empty vector
	empty := NewVersionVector()
	if len(empty.Replicas()) != 0 {
		t.Errorf("Empty vector Replicas() length = %d, want 0", len(empty.Replicas()))
	}

	// Nil vector
	var nilVector *VersionVector
	if len(nilVector.Replicas()) != 0 {
		t.Errorf("Nil vector Replicas() length = %d, want 0", len(nilVector.Replicas()))
	}
}

// TestVersionVectorIsEmpty verifies the IsEmpty method.
func TestVersionVectorIsEmpty(t *testing.T) {
	// Nil vector
	var nilVector *VersionVector
	if !nilVector.IsEmpty() {
		t.Error("Nil vector IsEmpty() = false, want true")
	}

	// Empty vector
	empty := NewVersionVector()
	if !empty.IsEmpty() {
		t.Error("Empty vector IsEmpty() = false, want true")
	}

	// Vector with zeros
	zeros := NewVersionVector()
	zeros.Set("replica1", 0)
	zeros.Set("replica2", 0)
	if !zeros.IsEmpty() {
		t.Error("Vector with all zeros IsEmpty() = false, want true")
	}

	// Non-empty vector
	nonEmpty := NewVersionVector()
	nonEmpty.Set("replica1", 1)
	if nonEmpty.IsEmpty() {
		t.Error("Non-empty vector IsEmpty() = true, want false")
	}

	// Mixed
	mixed := NewVersionVector()
	mixed.Set("replica1", 0)
	mixed.Set("replica2", 5)
	if mixed.IsEmpty() {
		t.Error("Vector with mixed values IsEmpty() = true, want false")
	}
}

// TestVersionVectorString verifies the String method.
func TestVersionVectorString(t *testing.T) {
	tests := []struct {
		name     string
		vector   map[ReplicaID]uint64
		expected string
	}{
		{
			name:     "empty vector",
			vector:   map[ReplicaID]uint64{},
			expected: "{}",
		},
		{
			name:     "single replica",
			vector:   map[ReplicaID]uint64{"replica1": 5},
			expected: "{replica1:5}",
		},
		{
			name:     "multiple replicas sorted",
			vector:   map[ReplicaID]uint64{"charlie": 3, "alice": 1, "bob": 2},
			expected: "{alice:1, bob:2, charlie:3}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vv := NewVersionVector()
			for k, v := range tt.vector {
				vv.Set(k, v)
			}

			got := vv.String()
			if got != tt.expected {
				t.Errorf("String() = %s, want %s", got, tt.expected)
			}
		})
	}

	// Nil vector
	var nilVector *VersionVector
	if got := nilVector.String(); got != "{}" {
		t.Errorf("Nil vector String() = %s, want {}", got)
	}
}

// TestVersionVectorDeterministicIteration verifies that iteration order is stable.
func TestVersionVectorDeterministicIteration(t *testing.T) {
	vv := NewVersionVector()
	vv.Set("zebra", 1)
	vv.Set("apple", 2)
	vv.Set("mango", 3)
	vv.Set("banana", 4)

	// Call Replicas() multiple times and verify same order
	replicas1 := vv.Replicas()
	replicas2 := vv.Replicas()
	replicas3 := vv.Replicas()

	for i := range replicas1 {
		if replicas1[i] != replicas2[i] || replicas2[i] != replicas3[i] {
			t.Errorf("Replicas() order is not deterministic at index %d: %s, %s, %s",
				i, replicas1[i], replicas2[i], replicas3[i])
		}
	}

	// Verify String() is deterministic
	str1 := vv.String()
	str2 := vv.String()
	str3 := vv.String()

	if str1 != str2 || str2 != str3 {
		t.Errorf("String() is not deterministic: %s, %s, %s", str1, str2, str3)
	}
}

// TestVersionVectorDynamoScenario demonstrates a typical Dynamo/Riak scenario.
func TestVersionVectorDynamoScenario(t *testing.T) {
	// Initial write to replica A
	versionA := NewVersionVector()
	versionA.Increment("A") // {A:1}

	// Object propagates to replica B
	versionB := versionA.Copy()
	versionB.Increment("B") // {A:1, B:1}

	// Concurrent writes happen
	// Client 1 writes to A
	version1 := versionB.Copy()
	version1.Increment("A") // {A:2, B:1}

	// Client 2 writes to B (from old version)
	version2 := versionB.Copy()
	version2.Increment("B") // {A:1, B:2}

	// Detect conflict
	if !version1.Concurrent(version2) {
		t.Error("Expected version1 and version2 to be concurrent (conflict)")
	}

	// Resolve by merging
	merged := version1.Copy()
	merged.Merge(version2)

	// Merged should dominate both
	if !merged.Dominates(version1) {
		t.Error("Merged should dominate version1")
	}
	if !merged.Dominates(version2) {
		t.Error("Merged should dominate version2")
	}

	// Verify merged state
	if merged.Get("A") != 2 {
		t.Errorf("Merged Get(A) = %d, want 2", merged.Get("A"))
	}
	if merged.Get("B") != 2 {
		t.Errorf("Merged Get(B) = %d, want 2", merged.Get("B"))
	}
}

// BenchmarkVersionVectorIncrement measures increment performance.
func BenchmarkVersionVectorIncrement(b *testing.B) {
	vv := NewVersionVector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vv.Increment("replica1")
	}
}

// BenchmarkVersionVectorMerge measures merge performance.
func BenchmarkVersionVectorMerge(b *testing.B) {
	vv1 := NewVersionVector()
	vv2 := NewVersionVector()
	for i := 0; i < 100; i++ {
		replica := ReplicaID(fmt.Sprintf("replica%d", i))
		vv1.Set(replica, uint64(i))
		vv2.Set(replica, uint64(i+50))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vv1Copy := vv1.Copy()
		vv1Copy.Merge(vv2)
	}
}

// BenchmarkVersionVectorCompare measures comparison performance.
func BenchmarkVersionVectorCompare(b *testing.B) {
	vv1 := NewVersionVector()
	vv2 := NewVersionVector()
	for i := 0; i < 100; i++ {
		replica := ReplicaID(fmt.Sprintf("replica%d", i))
		vv1.Set(replica, uint64(i))
		vv2.Set(replica, uint64(i+1))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vv1.Compare(vv2)
	}
}

// BenchmarkVersionVectorCopy measures copy performance.
func BenchmarkVersionVectorCopy(b *testing.B) {
	vv := NewVersionVector()
	for i := 0; i < 100; i++ {
		vv.Set(ReplicaID(fmt.Sprintf("replica%d", i)), uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vv.Copy()
	}
}

// BenchmarkVersionVectorString measures String() performance.
func BenchmarkVersionVectorString(b *testing.B) {
	vv := NewVersionVector()
	for i := 0; i < 50; i++ {
		vv.Set(ReplicaID(fmt.Sprintf("replica%d", i)), uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vv.String()
	}
}

// BenchmarkVersionVectorIncrement_Scaling measures increment performance with varying replica counts.
func BenchmarkVersionVectorIncrement_Scaling(b *testing.B) {
	tests := []struct {
		name     string
		replicas int
	}{
		{"10replicas", 10},
		{"50replicas", 50},
		{"100replicas", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			vv := NewVersionVector()
			for i := 0; i < tt.replicas; i++ {
				vv.Set(ReplicaID(fmt.Sprintf("replica%d", i)), uint64(i))
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				vv.Increment(ReplicaID(fmt.Sprintf("replica%d", i%tt.replicas)))
			}
		})
	}
}

// BenchmarkVersionVectorCompare_Scaling measures compare performance with varying replica counts.
func BenchmarkVersionVectorCompare_Scaling(b *testing.B) {
	tests := []struct {
		name     string
		replicas int
	}{
		{"10replicas", 10},
		{"50replicas", 50},
		{"100replicas", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			vv1 := NewVersionVector()
			vv2 := NewVersionVector()
			for i := 0; i < tt.replicas; i++ {
				vv1.Set(ReplicaID(fmt.Sprintf("replica%d", i)), uint64(i))
				vv2.Set(ReplicaID(fmt.Sprintf("replica%d", i)), uint64(i+1))
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				vv1.Compare(vv2)
			}
		})
	}
}

// BenchmarkVersionVectorMerge_Scaling measures merge performance with varying replica counts.
func BenchmarkVersionVectorMerge_Scaling(b *testing.B) {
	tests := []struct {
		name     string
		replicas int
	}{
		{"10replicas", 10},
		{"50replicas", 50},
		{"100replicas", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			vv1 := NewVersionVector()
			vv2 := NewVersionVector()
			for i := 0; i < tt.replicas; i++ {
				vv1.Set(ReplicaID(fmt.Sprintf("replica%d", i)), uint64(i))
				vv2.Set(ReplicaID(fmt.Sprintf("replica%d", i)), uint64(i+1))
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				vv1.Merge(vv2)
			}
		})
	}
}
