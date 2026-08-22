package index

import (
	"time"

	"github.com/grokify/diskwise/scan"
)

// Kind is the persisted, human-readable form of scan.NodeKind.
type Kind string

const (
	KindFile    Kind = "file"
	KindDir     Kind = "dir"
	KindSymlink Kind = "symlink"
)

func kindFromScan(k scan.NodeKind) Kind {
	switch k {
	case scan.KindDir:
		return KindDir
	case scan.KindSymlink:
		return KindSymlink
	default:
		return KindFile
	}
}

// SubtreeState is the persisted, human-readable form of scan.SubtreeState.
type SubtreeState string

const (
	StatePartial     SubtreeState = "partial"
	StateComplete    SubtreeState = "complete"
	StateDeferred    SubtreeState = "deferred"
	StateDenied      SubtreeState = "denied"
	StateCrossDevice SubtreeState = "cross_device"
)

func stateFromScan(s scan.SubtreeState) SubtreeState {
	switch s {
	case scan.StateComplete:
		return StateComplete
	case scan.StateDeferred:
		return StateDeferred
	case scan.StateDenied:
		return StateDenied
	case scan.StateCrossDevice:
		return StateCrossDevice
	default:
		return StatePartial
	}
}

// ScanRun records one scan (or rescan) invocation.
type ScanRun struct {
	ID         int64
	Root       string
	StartedAt  time.Time
	FinishedAt *time.Time
	Status     string
}

// statusFor derives a scan_runs.status value from the final state of a
// walked tree's root node.
func statusFor(state scan.SubtreeState) string {
	if state == scan.StateComplete {
		return "complete"
	}
	return "partial"
}
