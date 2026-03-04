// Package vvector implements per-object version vectors for distributed systems.
package vvector

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/dmundt/causalclock/clock"
)

// VersionVector implements per-object causal tracking with Dynamo/Riak semantics.
//
// A version vector tracks causality for a single object across multiple replicas.
// Unlike vector clocks which track events, version vectors track object versions.
//
// Key properties:
//   - Map-based storage: replica ID -> counter
//   - Dynamo/Riak merge semantics: element-wise maximum
//   - Detects concurrent updates (conflicts)
//   - No transport logic (pure data structure)
//
// Design Decisions:
//
// 1. ReplicaID as string for flexibility (node names, UUIDs, etc.)
// 2. uint64 counters (positive version numbers only)
// 3. Element-wise maximum merge (Dynamo/Riak standard)
// 4. Nil-safe operations
// 5. Deterministic iteration order (sorted keys)
// 6. No internal locking (external synchronization required)
//
// Concurrency: VersionVector is NOT thread-safe. Use external locking.
type VersionVector struct {
	// versions maps replica IDs to version counters
	versions map[ReplicaID]uint64
}

// ReplicaID uniquely identifies a replica in the distributed system.
type ReplicaID string

// NewVersionVector creates a new version vector, optionally initialized
// with the given replicas set to zero.
func NewVersionVector(replicas ...ReplicaID) *VersionVector {
	vv := &VersionVector{
		versions: make(map[ReplicaID]uint64, len(replicas)),
	}
	for _, replica := range replicas {
		vv.versions[replica] = 0
	}
	return vv
}

// Copy creates a deep copy of the version vector.
// This is useful for creating snapshots before mutations.
func (vv *VersionVector) Copy() *VersionVector {
	if vv == nil {
		return NewVersionVector()
	}
	
	copy := &VersionVector{
		versions: make(map[ReplicaID]uint64, len(vv.versions)),
	}
	for k, v := range vv.versions {
		copy.versions[k] = v
	}
	return copy
}

// Increment increments the version counter for the given replica and returns the new value.
// If the replica doesn't exist, it is initialized to 1.
//
// This operation MUTATES the version vector.
//
// Typical usage: A replica increments its own counter when updating the object.
func (vv *VersionVector) Increment(replica ReplicaID) uint64 {
	if vv.versions == nil {
		vv.versions = make(map[ReplicaID]uint64)
	}
	vv.versions[replica]++
	return vv.versions[replica]
}

// Get returns the version counter for the given replica.
// Returns 0 if the replica is not present.
func (vv *VersionVector) Get(replica ReplicaID) uint64 {
	if vv == nil || vv.versions == nil {
		return 0
	}
	return vv.versions[replica]
}

// Set sets the version counter for the given replica.
// This operation MUTATES the version vector.
//
// Note: Setting to 0 keeps the entry (doesn't remove it).
func (vv *VersionVector) Set(replica ReplicaID, version uint64) {
	if vv.versions == nil {
		vv.versions = make(map[ReplicaID]uint64)
	}
	vv.versions[replica] = version
}

// Merge updates this version vector to the element-wise maximum of itself and other.
// This implements Dynamo/Riak merge semantics.
//
// This operation MUTATES the version vector.
//
// Typical usage: When receiving an object version, merge it with the local version
// to reconcile concurrent updates.
//
// Nil safety: If other is nil, this is a no-op.
func (vv *VersionVector) Merge(other *VersionVector) {
	if other == nil || other.versions == nil {
		return
	}
	
	if vv.versions == nil {
		vv.versions = make(map[ReplicaID]uint64, len(other.versions))
	}
	
	for replica, otherVersion := range other.versions {
		if currentVersion := vv.versions[replica]; otherVersion > currentVersion {
			vv.versions[replica] = otherVersion
		} else if currentVersion == 0 && otherVersion == 0 {
			// Ensure the replica is present even if both are zero
			vv.versions[replica] = 0
		}
	}
}

// Compare determines the causal relationship between two version vectors.
//
// Returns:
//   - clock.EqualCmp: vectors are identical
//   - clock.BeforeCmp: vv happened-before other (vv is ancestor of other)
//   - clock.AfterCmp: vv happened-after other (vv is descendant of other)
//   - clock.ConcurrentCmp: vectors are concurrent (conflict/divergent versions)
//
// Dynamo/Riak semantics:
//   - Before: vv[i] <= other[i] for all i, and vv[j] < other[j] for at least one j
//   - After: vv[i] >= other[i] for all i, and vv[j] > other[j] for at least one j
//   - Equal: vv[i] == other[i] for all i
//   - Concurrent: neither Before nor After (indicates conflict)
//
// Nil handling:
//   - Compare(nil, nil) -> EqualCmp
//   - Compare(non-empty, nil) -> AfterCmp
//   - Compare(nil, non-empty) -> BeforeCmp
func (vv *VersionVector) Compare(other *VersionVector) clock.Comparison {
	// Normalize nil vectors to empty vectors
	vvEmpty := vv == nil || vv.versions == nil || len(vv.versions) == 0
	otherEmpty := other == nil || other.versions == nil || len(other.versions) == 0
	
	if vvEmpty && otherEmpty {
		return clock.EqualCmp
	}
	
	if vvEmpty {
		// vv is empty, other is not
		// Check if other has any non-zero values
		for _, v := range other.versions {
			if v > 0 {
				return clock.BeforeCmp
			}
		}
		return clock.EqualCmp
	}
	
	if otherEmpty {
		// vv is not empty, other is
		for _, v := range vv.versions {
			if v > 0 {
				return clock.AfterCmp
			}
		}
		return clock.EqualCmp
	}
	
	// Both vectors have entries
	// Collect all replicas present in either vector
	allReplicas := make(map[ReplicaID]struct{})
	for replica := range vv.versions {
		allReplicas[replica] = struct{}{}
	}
	for replica := range other.versions {
		allReplicas[replica] = struct{}{}
	}
	
	// Track relationship
	hasGreater := false
	hasLess := false
	
	for replica := range allReplicas {
		vvVal := vv.versions[replica]
		otherVal := other.versions[replica]
		
		if vvVal > otherVal {
			hasGreater = true
		} else if vvVal < otherVal {
			hasLess = true
		}
		
		// Early exit if we know it's concurrent
		if hasGreater && hasLess {
			return clock.ConcurrentCmp
		}
	}
	
	if hasGreater && !hasLess {
		return clock.AfterCmp
	}
	if hasLess && !hasGreater {
		return clock.BeforeCmp
	}
	return clock.EqualCmp
}

// Replicas returns a sorted list of all replica IDs present in the version vector.
// The returned slice is a new allocation to prevent external mutation.
//
// The sort order is deterministic (lexicographic) for stable iteration.
func (vv *VersionVector) Replicas() []ReplicaID {
	if vv == nil || vv.versions == nil {
		return []ReplicaID{}
	}
	
	replicas := make([]ReplicaID, 0, len(vv.versions))
	for replica := range vv.versions {
		replicas = append(replicas, replica)
	}
	
	// Sort for deterministic ordering
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i] < replicas[j]
	})
	
	return replicas
}

// Len returns the number of replicas tracked by this version vector.
func (vv *VersionVector) Len() int {
	if vv == nil || vv.versions == nil {
		return 0
	}
	return len(vv.versions)
}

// IsEmpty returns true if the version vector has no entries or all entries are zero.
func (vv *VersionVector) IsEmpty() bool {
	if vv == nil || vv.versions == nil || len(vv.versions) == 0 {
		return true
	}
	
	for _, v := range vv.versions {
		if v != 0 {
			return false
		}
	}
	return true
}

// String returns a human-readable representation of the version vector.
// Format: {replica1:version1, replica2:version2, ...} with replicas in sorted order.
func (vv *VersionVector) String() string {
	if vv == nil || vv.versions == nil || len(vv.versions) == 0 {
		return "{}"
	}
	
	replicas := vv.Replicas()
	var buf bytes.Buffer
	buf.WriteString("{")
	
	for i, replica := range replicas {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%s:%d", replica, vv.versions[replica])
	}
	
	buf.WriteString("}")
	return buf.String()
}

// Equal returns true if two version vectors are identical.
// This is a convenience wrapper around Compare.
func (vv *VersionVector) Equal(other *VersionVector) bool {
	return vv.Compare(other) == clock.EqualCmp
}

// HappenedBefore returns true if vv happened-before other (vv is an ancestor of other).
// This is a convenience wrapper around Compare.
func (vv *VersionVector) HappenedBefore(other *VersionVector) bool {
	return vv.Compare(other) == clock.BeforeCmp
}

// HappenedAfter returns true if vv happened-after other (vv is a descendant of other).
// This is a convenience wrapper around Compare.
func (vv *VersionVector) HappenedAfter(other *VersionVector) bool {
	return vv.Compare(other) == clock.AfterCmp
}

// Concurrent returns true if vv and other are concurrent (conflict detected).
// This is a convenience wrapper around Compare.
//
// In Dynamo/Riak, concurrent versions indicate a conflict that requires
// application-specific resolution (e.g., last-write-wins, merge, manual resolution).
func (vv *VersionVector) Concurrent(other *VersionVector) bool {
	return vv.Compare(other) == clock.ConcurrentCmp
}

// Descends returns true if vv is a descendant of other (happened-after or equal).
// This checks if vv could have been derived from other.
func (vv *VersionVector) Descends(other *VersionVector) bool {
	cmp := vv.Compare(other)
	return cmp == clock.AfterCmp || cmp == clock.EqualCmp
}

// Dominates returns true if vv dominates other (is newer in all aspects).
// This is the same as HappenedAfter - kept for API compatibility with some systems.
func (vv *VersionVector) Dominates(other *VersionVector) bool {
	return vv.HappenedAfter(other)
}

// IsDominatedBy returns true if vv is dominated by other (is older in all aspects).
// This is the same as HappenedBefore - kept for API compatibility with some systems.
func (vv *VersionVector) IsDominatedBy(other *VersionVector) bool {
	return vv.HappenedBefore(other)
}
