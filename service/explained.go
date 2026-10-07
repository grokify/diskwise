package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/index"
)

// excludeExplained removes from each unexplained directory finding the
// bytes that another finding already reports, so every byte is
// attributed to exactly one finding. explained lists the paths other
// detectors claim. An unexplained directory that lies inside an
// explained path is dropped; one that merely contains explained paths
// keeps its Path but reports only the remainder. Findings whose
// remainder falls below minSize (0 means detect.DefaultUnexplainedMinSize)
// are dropped, since "large" is judged on what's left unexplained.
func excludeExplained(ctx context.Context, db *index.DB, unexplained []detect.Finding, explained []string, minSize int64) ([]detect.Finding, error) {
	if minSize <= 0 {
		minSize = detect.DefaultUnexplainedMinSize
	}

	var out []detect.Finding
	for _, f := range unexplained {
		if coveredBy(f.Path, explained) {
			continue
		}

		var subAlloc, subLogical int64
		var nested int
		for _, p := range explained {
			if !pathUnder(p, f.Path) {
				continue
			}
			row, ok, err := db.NodeByPath(ctx, p)
			if err != nil {
				return nil, fmt.Errorf("exclude explained %s: %w", p, err)
			}
			if !ok {
				continue
			}
			subAlloc += row.AllocatedSizeFor()
			subLogical += row.LogicalSizeFor()
			nested++
		}
		if nested == 0 {
			if f.AllocatedSize >= minSize {
				out = append(out, f)
			}
			continue
		}

		f.AllocatedSize = max(f.AllocatedSize-subAlloc, 0)
		f.LogicalSize = max(f.LogicalSize-subLogical, 0)
		if f.AllocatedSize < minSize {
			continue
		}
		f.Reason = fmt.Sprintf("%s (excludes %d path(s) reported by other findings)", f.Reason, nested)
		out = append(out, f)
	}
	// Subtraction can reorder what the detector returned largest-first.
	sort.SliceStable(out, func(i, j int) bool { return out[i].AllocatedSize > out[j].AllocatedSize })
	return out, nil
}

// coveredBy reports whether path is, or is inside, any of the covering paths.
func coveredBy(path string, covering []string) bool {
	for _, c := range covering {
		if pathUnder(path, c) {
			return true
		}
	}
	return false
}

// pathUnder reports whether path is root itself or a descendant of it.
func pathUnder(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}
