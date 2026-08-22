package scan

import "time"

// NodeKind classifies a scanned filesystem entry.
type NodeKind int

const (
	KindFile NodeKind = iota
	KindDir
	KindSymlink
)

// SubtreeState reports how completely a directory's contents have been
// measured. It is meaningful only for directory nodes.
type SubtreeState int

const (
	// StatePartial means the subtree was not fully measured, either
	// because the walk was cancelled before reaching it or because one
	// of its descendants is Denied/Deferred/CrossDevice.
	StatePartial SubtreeState = iota
	// StateComplete means this directory and every descendant were
	// fully measured.
	StateComplete
	// StateDeferred means the depth cap was reached before this
	// directory could be expanded.
	StateDeferred
	// StateDenied means this directory could not be read (permission).
	StateDenied
	// StateCrossDevice means this directory is on a different volume
	// than the scan root and was not descended into.
	StateCrossDevice
)

// Node is one scanned filesystem entry.
type Node struct {
	Path          string
	Name          string
	Kind          NodeKind
	LogicalSize   int64
	AllocatedSize int64
	Device        int32
	Inode         uint64
	Nlink         uint16
	ModTime       time.Time

	// Duplicate is true when this node shares a (device, inode) pair
	// with an earlier node in the same scan run — its bytes are
	// excluded from subtree rollups to avoid double-counting hard
	// links.
	Duplicate bool

	// The following fields apply only to directory nodes (Kind == KindDir).
	State                SubtreeState
	SubtreeLogicalSize   int64
	SubtreeAllocatedSize int64
	Children             []*Node

	expanded bool // internal: set once this directory's contents have been read
}

// VisitFunc is called once for each directory node as its subtree
// measurement is finalized, in bottom-up (post-order) order.
type VisitFunc func(dir *Node)

// Stats holds aggregate counters for a completed (or cancelled) walk.
// All fields are safe to poll live via sync/atomic from another
// goroutine while Walk is still running.
type Stats struct {
	DirsScanned         int64
	DirsQueued          int64 // directories discovered and enqueued so far, including DirsScanned; a live progress denominator
	FilesScanned        int64
	SymlinksScanned     int64
	DeniedPaths         int64
	CrossDeviceSkipped  int64
	DuplicateFiles      int64
	DuplicateBytesSaved int64 // allocated bytes not double-counted due to hard-link dedup
}
