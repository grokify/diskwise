package scan

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/grokify/diskwise/platform"
)

// Options configures a Walk.
type Options struct {
	// Workers bounds concurrency; 0 defaults to GOMAXPROCS.
	Workers int
	// Depth caps how many levels below root are expanded; 0 means
	// unlimited (a full scan). Directories beyond the cap are recorded
	// with State StateDeferred and contribute zero to subtree totals —
	// useful for a fast bounded preview, not for an authoritative scan.
	Depth int
	// CrossDevice, if true, descends into directories on a different
	// volume than root. Default false, matching `du -x` behavior, so a
	// home-directory scan doesn't wander into other mounted volumes.
	CrossDevice bool
	// OnComplete, if set, is called for each directory node once its
	// subtree measurement is finalized, in bottom-up order.
	OnComplete VisitFunc
	// Stats, if non-nil, is used to accumulate counters instead of a
	// freshly allocated one. Pass a pointer in and read it with
	// sync/atomic from another goroutine to poll live progress while
	// Walk is still running.
	Stats *Stats
	// PriorityPaths are absolute paths (typically known heavy-storage
	// locations from a knowledge registry) that jump to the front of
	// the descent queue as soon as they're discovered, ahead of the
	// normal size-based priority. A path can only be prioritized once
	// its parent has been expanded — Walk can't skip ahead of its own
	// causal discovery order — but this still means DiskWise doesn't
	// have to stumble onto a known-important location by chance.
	PriorityPaths []string
}

// Walk scans the directory tree rooted at root, expanding directories
// concurrently in order of already-counted direct byte size (largest
// first) so the biggest subtrees resolve early. It returns the fully
// populated node tree and aggregate stats. A cancelled ctx stops
// further expansion; already-discovered nodes are returned with State
// StatePartial where their subtree was not fully measured.
func Walk(ctx context.Context, root string, opts Options) (*Node, *Stats, error) {
	rootStat, err := platform.Stat(root)
	if err != nil {
		return nil, nil, fmt.Errorf("scan: stat root %s: %w", root, err)
	}
	if !rootStat.IsDir {
		return nil, nil, fmt.Errorf("scan: root %s is not a directory", root)
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}

	rootNode := &Node{
		Path:          root,
		Name:          filepath.Base(root),
		Kind:          KindDir,
		LogicalSize:   rootStat.LogicalSize,
		AllocatedSize: rootStat.AllocatedSize,
		Device:        rootStat.Device,
		Inode:         rootStat.Inode,
		Nlink:         rootStat.Nlink,
		ModTime:       rootStat.ModTime,
	}

	stats := opts.Stats
	if stats == nil {
		stats = &Stats{}
	}
	dedup := newDedupSet()
	pq := newPriorityQueue()

	var pending sync.WaitGroup
	pending.Add(1)
	atomic.AddInt64(&stats.DirsQueued, 1)
	pq.Push(rootNode, 0, 0)

	var workerWG sync.WaitGroup
	for range workers {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for {
				item, ok := pq.Pop()
				if !ok {
					return
				}
				expandDir(ctx, item, rootStat.Device, opts, pq, &pending, stats, dedup)
				pending.Done()
			}
		}()
	}

	go func() {
		pending.Wait()
		pq.Close()
	}()

	workerWG.Wait()

	rollup(rootNode, opts.OnComplete)

	return rootNode, stats, ctx.Err()
}

// expandDir reads one directory's entries, stats each child, applies
// hard-link dedup and mount-boundary/depth rules, and enqueues any
// discovered subdirectories for further expansion.
func expandDir(ctx context.Context, item *queueItem, rootDevice int32, opts Options, pq *priorityQueue, pending *sync.WaitGroup, stats *Stats, dedup *dedupSet) {
	node := item.node

	if err := ctx.Err(); err != nil {
		return
	}
	if opts.Depth > 0 && item.depth >= opts.Depth {
		node.State = StateDeferred
		return
	}

	entries, err := os.ReadDir(node.Path)
	if err != nil {
		node.State = StateDenied
		atomic.AddInt64(&stats.DeniedPaths, 1)
		return
	}
	node.expanded = true
	atomic.AddInt64(&stats.DirsScanned, 1)

	var directBytes int64
	var subdirs []*Node

	for _, entry := range entries {
		childPath := filepath.Join(node.Path, entry.Name())
		st, err := platform.Stat(childPath)
		if err != nil {
			atomic.AddInt64(&stats.DeniedPaths, 1)
			continue
		}

		child := &Node{
			Path:          childPath,
			Name:          entry.Name(),
			LogicalSize:   st.LogicalSize,
			AllocatedSize: st.AllocatedSize,
			Device:        st.Device,
			Inode:         st.Inode,
			Nlink:         st.Nlink,
			ModTime:       st.ModTime,
		}
		node.Children = append(node.Children, child)

		switch {
		case st.IsDir:
			child.Kind = KindDir
			subdirs = append(subdirs, child)
		case st.IsSymlink:
			child.Kind = KindSymlink
			directBytes += child.AllocatedSize
			atomic.AddInt64(&stats.SymlinksScanned, 1)
		default:
			child.Kind = KindFile
			if st.Nlink > 1 && dedup.checkAndMark(st.Device, st.Inode) {
				child.Duplicate = true
				atomic.AddInt64(&stats.DuplicateFiles, 1)
				atomic.AddInt64(&stats.DuplicateBytesSaved, child.AllocatedSize)
			} else {
				directBytes += child.AllocatedSize
			}
			atomic.AddInt64(&stats.FilesScanned, 1)
		}
	}

	for _, sub := range subdirs {
		if !shouldDescend(sub.Device, rootDevice, opts.CrossDevice) {
			sub.State = StateCrossDevice
			atomic.AddInt64(&stats.CrossDeviceSkipped, 1)
			continue
		}
		priority := directBytes
		if isPriorityPath(sub.Path, opts.PriorityPaths) {
			priority = math.MaxInt64
		}
		pending.Add(1)
		atomic.AddInt64(&stats.DirsQueued, 1)
		pq.Push(sub, priority, item.depth+1)
	}
}

// isPriorityPath reports whether path is one of the caller-supplied
// priority paths.
func isPriorityPath(path string, priorityPaths []string) bool {
	return slices.Contains(priorityPaths, path)
}

// shouldDescend reports whether a discovered subdirectory should be
// expanded further, given the volume it lives on relative to the scan
// root. Matches `du -x` default behavior: don't cross mount points
// unless explicitly requested.
func shouldDescend(childDevice, rootDevice int32, crossDevice bool) bool {
	return crossDevice || childDevice == rootDevice
}

// rollup recursively computes each directory's subtree totals from its
// (already-expanded) children and finalizes its State, firing
// onComplete in bottom-up order.
func rollup(node *Node, onComplete VisitFunc) {
	defer func() {
		if onComplete != nil {
			onComplete(node)
		}
	}()

	switch node.State {
	case StateDenied, StateDeferred, StateCrossDevice:
		return
	}
	if !node.expanded {
		node.State = StatePartial
		return
	}

	var logical, allocated int64
	complete := true
	for _, child := range node.Children {
		if child.Kind == KindDir {
			rollup(child, onComplete)
			if child.State != StateComplete {
				complete = false
			}
			logical += child.SubtreeLogicalSize
			allocated += child.SubtreeAllocatedSize
			continue
		}
		if !child.Duplicate {
			logical += child.LogicalSize
			allocated += child.AllocatedSize
		}
	}
	node.SubtreeLogicalSize = logical
	node.SubtreeAllocatedSize = allocated
	if complete {
		node.State = StateComplete
	} else {
		node.State = StatePartial
	}
}
