package detect

import (
	"context"
	"fmt"
	"sort"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

// DefaultUnexplainedMinSize is the smallest allocated size
// LargeUnexplainedDetector reports when MinSize is unset (1 GiB; below
// that, "large" isn't a meaningful signal).
const DefaultUnexplainedMinSize int64 = 1 << 30

// LargeUnexplainedDetector surfaces large directories that no
// KnownLocation claims — the "worth exploring" half of hotspots.
// It makes no claim about what these directories are: Confidence 0,
// ActionClass Unknown. That's the point: DiskWise should never guess
// at reclaimability, only report where it has no explanation.
//
// Findings are disjoint: when a large directory and its own
// subdirectories all qualify, only the outermost is reported, so the
// reported sizes never double-count. Drill into one with `diskwise
// tree`. A reported directory may still contain bytes another detector
// claims (e.g. a known location nested inside it); the service layer
// subtracts those (see service.excludeExplained).
type LargeUnexplainedDetector struct {
	Registry knowledge.Registry
	// MinSize is the smallest allocated size worth reporting; 0 uses
	// DefaultUnexplainedMinSize.
	MinSize int64
	// Limit caps the number of findings; 0 uses a default of 20.
	Limit int
}

func (d LargeUnexplainedDetector) ID() string { return "large-unexplained" }

func (d LargeUnexplainedDetector) Detect(ctx context.Context, db *index.DB, root string) ([]Finding, error) {
	claimed, err := claimedPaths(d.Registry)
	if err != nil {
		return nil, fmt.Errorf("detect: large-unexplained: %w", err)
	}
	if err := withBundleRoots(ctx, db, root, claimed); err != nil {
		return nil, fmt.Errorf("detect: large-unexplained: %w", err)
	}

	limit := d.Limit
	if limit <= 0 {
		limit = 20
	}
	minSize := d.MinSize
	if minSize <= 0 {
		minSize = DefaultUnexplainedMinSize
	}

	// Over-fetch: nested directories and claimed ones are filtered out
	// below, and a deep tree can have many qualifying ancestors of a
	// single hot spot.
	rows, err := db.Largest(ctx, root, max(limit*50, 1000), index.KindDir)
	if err != nil {
		return nil, fmt.Errorf("detect: large-unexplained: %w", err)
	}
	// Largest first; on a size tie the shorter (outer) path wins so an
	// ancestor is always considered before its descendants.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].AllocatedSizeFor() != rows[j].AllocatedSizeFor() {
			return rows[i].AllocatedSizeFor() > rows[j].AllocatedSizeFor()
		}
		return len(rows[i].Path) < len(rows[j].Path)
	})

	var findings []Finding
	var selected []string
	for _, row := range rows {
		if len(findings) >= limit {
			break
		}
		if row.AllocatedSizeFor() < minSize {
			continue
		}
		if isClaimed(row.Path, claimed) || isNested(row.Path, selected) {
			continue
		}
		selected = append(selected, row.Path)
		findings = append(findings, Finding{
			Entity: entity.Entity{
				ID:         row.Path,
				Kind:       entity.KindUnknown,
				Name:       row.Name,
				Detector:   "large-unexplained",
				Confidence: 0,
			},
			Path:          row.Path,
			Paths:         []string{row.Path},
			LogicalSize:   row.LogicalSizeFor(),
			AllocatedSize: row.AllocatedSizeFor(),
			Confidence:    0,
			ActionClass:   policy.Evaluate(policy.Unknown, entity.KindUnknown, 0),
			Reason:        "large directory with no known-location match; worth exploring",
		})
	}
	return findings, nil
}

// isClaimed reports whether path is, or is inside, a claimed location.
func isClaimed(path string, claimed map[string]bool) bool {
	if claimed[path] {
		return true
	}
	for c := range claimed {
		if underRoot(path, c) {
			return true
		}
	}
	return false
}

// isNested reports whether path is inside any directory in outer.
func isNested(path string, outer []string) bool {
	for _, o := range outer {
		if underRoot(path, o) {
			return true
		}
	}
	return false
}
