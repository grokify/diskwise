package scan

import "sync"

type dedupKey struct {
	device int32
	inode  uint64
}

// dedupSet tracks (device, inode) pairs already counted in a scan run,
// so a hard-linked file's allocated bytes are attributed once rather
// than once per path that references it.
type dedupSet struct {
	mu   sync.Mutex
	seen map[dedupKey]struct{}
}

func newDedupSet() *dedupSet {
	return &dedupSet{seen: make(map[dedupKey]struct{})}
}

// checkAndMark reports whether (device, inode) was already seen in
// this scan run, recording it if not. Files with Nlink <= 1 cannot be
// hard-linked, so callers should skip the check (and its lock
// contention) for the common case.
func (d *dedupSet) checkAndMark(device int32, inode uint64) bool {
	key := dedupKey{device, inode}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.seen[key]; ok {
		return true
	}
	d.seen[key] = struct{}{}
	return false
}
