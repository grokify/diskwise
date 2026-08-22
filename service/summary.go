package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/platform"
)

// Summary reports what is known about a previously scanned path: its
// current rollup totals, the scan run that produced them, known
// heavy-storage locations it contains, and (if available) live volume
// capacity.
type Summary struct {
	Path           string
	Kind           index.Kind
	LogicalSize    int64
	AllocatedSize  int64
	State          index.SubtreeState
	ScanRun        index.ScanRun
	KnownLocations []detect.Finding `json:",omitempty"`
	Volume         *platform.VolumeStats
}

// Summary returns the current index state for path.
func (s *Service) Summary(ctx context.Context, path string) (*Summary, error) {
	row, ok, err := s.db.NodeByPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("service: summary %s: %w", path, err)
	}
	if !ok {
		return nil, errNotIndexed(path)
	}

	run, err := s.db.ScanRunByID(ctx, row.ScanRunID)
	if err != nil {
		return nil, fmt.Errorf("service: summary %s: %w", path, err)
	}

	known, err := (detect.KnownLocationDetector{Registry: s.registry}).Detect(ctx, s.db, path)
	if err != nil {
		return nil, fmt.Errorf("service: summary %s: %w", path, err)
	}
	sort.Slice(known, func(i, j int) bool { return known[i].AllocatedSize > known[j].AllocatedSize })

	summary := &Summary{
		Path:           path,
		Kind:           row.Kind,
		LogicalSize:    row.LogicalSizeFor(),
		AllocatedSize:  row.AllocatedSizeFor(),
		State:          row.State,
		ScanRun:        run,
		KnownLocations: known,
	}
	if vs, verr := platform.VolumeStatsForPath(path); verr == nil {
		summary.Volume = &vs
	}
	return summary, nil
}
