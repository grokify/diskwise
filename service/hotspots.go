package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/grokify/diskwise/detect"
)

// HotspotsQuery configures a Hotspots report.
type HotspotsQuery struct {
	Path string
	// MinSize is the smallest allocated size worth reporting as
	// unexplained; 0 uses LargeUnexplainedDetector's 1 GiB default.
	MinSize int64
	// Limit caps the number of unexplained findings; 0 uses
	// LargeUnexplainedDetector's default of 20.
	Limit int
}

// HotspotsReport merges known-location hits (with semantics) and
// large subtrees no known location explains, per TRD §4.1 — the
// "worth exploring" list.
type HotspotsReport struct {
	Root        string
	Known       []detect.Finding
	Unexplained []detect.Finding
}

// Hotspots reports known heavy storage locations and large
// unexplained directories under q.Path.
func (s *Service) Hotspots(ctx context.Context, q HotspotsQuery) (*HotspotsReport, error) {
	if _, ok, err := s.db.NodeByPath(ctx, q.Path); err != nil {
		return nil, fmt.Errorf("service: hotspots %s: %w", q.Path, err)
	} else if !ok {
		return nil, errNotIndexed(q.Path)
	}

	known, err := (detect.KnownLocationDetector{Registry: s.registry}).Detect(ctx, s.db, q.Path)
	if err != nil {
		return nil, fmt.Errorf("service: hotspots %s: %w", q.Path, err)
	}
	sort.Slice(known, func(i, j int) bool { return known[i].AllocatedSize > known[j].AllocatedSize })

	unexplainedDetector := detect.LargeUnexplainedDetector{Registry: s.registry, MinSize: q.MinSize, Limit: q.Limit}
	unexplained, err := unexplainedDetector.Detect(ctx, s.db, q.Path)
	if err != nil {
		return nil, fmt.Errorf("service: hotspots %s: %w", q.Path, err)
	}

	return &HotspotsReport{Root: q.Path, Known: known, Unexplained: unexplained}, nil
}
