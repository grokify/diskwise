package platform

import "time"

// FileStat holds filesystem metadata for a single path, including both
// the logical (apparent) size and the allocated (on-disk) size — the
// figure that reflects sparse files and APFS clones rather than the
// logical size alone.
type FileStat struct {
	Path          string
	LogicalSize   int64
	AllocatedSize int64
	Device        int32
	Inode         uint64
	Nlink         uint16
	IsDir         bool
	IsSymlink     bool
	ModTime       time.Time
}
