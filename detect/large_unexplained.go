package detect

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

// LargeUnexplainedDetector surfaces large directories that no
// KnownLocation claims — the "worth exploring" half of hotspots.
// It makes no claim about what these directories are: Confidence 0,
// ActionClass Unknown. That's the point: DiskWise should never guess
// at reclaimability, only report where it has no explanation.
type LargeUnexplainedDetector struct {
	Registry knowledge.Registry
	// MinSize is the smallest allocated size worth reporting; 0 uses
	// a 1 GiB default (below that, "large" isn't a meaningful signal).
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

	limit := d.Limit
	if limit <= 0 {
		limit = 20
	}
	minSize := d.MinSize
	if minSize <= 0 {
		minSize = 1 << 30 // 1 GiB
	}

	// Over-fetch: some of the largest directories will be filtered
	// out as claimed, so ask for more than we need.
	rows, err := db.Largest(ctx, root, limit*4, index.KindDir)
	if err != nil {
		return nil, fmt.Errorf("detect: large-unexplained: %w", err)
	}

	var findings []Finding
	for _, row := range rows {
		if len(findings) >= limit {
			break
		}
		if row.AllocatedSizeFor() < minSize {
			continue
		}
		if isClaimed(row.Path, claimed) {
			continue
		}
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
