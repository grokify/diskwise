package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grokify/mogo/fmt/progress"

	"github.com/grokify/diskwise/scan"
)

// startProgress renders live scan counters to w every tick while a
// walk runs, so a slow scan doesn't sit silent. The walk has no known
// total directory count up front — Stats.DirsQueued grows as
// directories are discovered and converges with DirsScanned as the
// queue drains, giving a real (if approximate) completion percentage
// rather than a fake one. It only writes when w is a real terminal,
// keeping stdout/scripted output clean. The returned stop function is
// safe to call more than once.
func startProgress(w io.Writer, stats *scan.Stats) func() {
	if !isTerminal(w) {
		return func() {}
	}

	renderer := progress.NewSingleStageRenderer(w)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				updateProgress(renderer, stats)
			case <-done:
				updateProgress(renderer, stats)
				renderer.Done("")
				return
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			wg.Wait()
		})
	}
}

func updateProgress(renderer *progress.SingleStageRenderer, stats *scan.Stats) {
	scanned := int(atomic.LoadInt64(&stats.DirsScanned))
	queued := int(atomic.LoadInt64(&stats.DirsQueued))
	files := atomic.LoadInt64(&stats.FilesScanned)
	renderer.Update(scanned, queued, fmt.Sprintf("%d files", files))
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
