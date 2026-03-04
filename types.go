package causalclock

import (
	vc "github.com/dmundt/causalclock/clock"
	"github.com/dmundt/causalclock/version"
)

// Re-export clock types for convenient access
type (
	// Clock represents a vector clock
	Clock = vc.Clock

	// NodeID uniquely identifies a node
	NodeID = vc.NodeID

	// VersionVector represents a version vector
	VersionVector = version.VersionVector

	// ReplicaID uniquely identifies a replica
	ReplicaID = version.ReplicaID

	// Comparison represents ordering relationships
	Comparison = vc.Comparison
)

// Re-export comparison constants
const (
	ConcurrentCmp = vc.ConcurrentCmp
	BeforeCmp     = vc.BeforeCmp
	AfterCmp      = vc.AfterCmp
	EqualCmp      = vc.EqualCmp
)

// Re-export constructor functions
var (
	NewClock         = vc.NewClock
	NewVersionVector = version.NewVersionVector
	ParseClock       = vc.ParseClock
)
