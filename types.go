package causalclock

import (
	"github.com/dmundt/causalclock/clock"
	vvector "github.com/dmundt/causalclock/version"
)

// Re-export clock types for convenient access
type (
	// Clock represents a vector clock
	Clock = clock.Clock

	// NodeID uniquely identifies a node
	NodeID = clock.NodeID

	// VersionVector represents a version vector
	VersionVector = vvector.VersionVector

	// ReplicaID uniquely identifies a replica
	ReplicaID = vvector.ReplicaID

	// Comparison represents ordering relationships
	Comparison = clock.Comparison
)

// Re-export comparison constants
const (
	ConcurrentCmp = clock.ConcurrentCmp
	BeforeCmp     = clock.BeforeCmp
	AfterCmp      = clock.AfterCmp
	EqualCmp      = clock.EqualCmp
)

// Re-export constructor functions
var (
	NewClock         = clock.NewClock
	NewVersionVector = vvector.NewVersionVector
	ParseClock       = clock.ParseClock
)
