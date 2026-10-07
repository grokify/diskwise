package service

import (
	"context"
	"fmt"
	"os"
	"time"
)

// StaleAfter is how old a scan may be before results are flagged as
// possibly out of date.
const StaleAfter = 7 * 24 * time.Hour

// Freshness says when the data behind a result was measured. The index
// is a snapshot of the disk at scan time, so anything that changed
// since (a cache cleaned, a download removed) is invisible until the
// path is rescanned; carrying this in every result keeps that visible.
type Freshness struct {
	// ScannedAt is when the scan covering the queried path finished.
	ScannedAt time.Time
	// ScanStatus is "complete" or "partial" (some directories were
	// denied, skipped, or beyond the depth cap).
	ScanStatus string
	// Stale is true when ScannedAt is older than StaleAfter.
	Stale bool
}

// freshness reports when the scan covering path was measured.
func (s *Service) freshness(ctx context.Context, path string) (Freshness, error) {
	row, ok, err := s.db.NodeByPath(ctx, path)
	if err != nil {
		return Freshness{}, fmt.Errorf("service: freshness %s: %w", path, err)
	}
	if !ok {
		return Freshness{}, errNotIndexed(path)
	}
	run, err := s.db.ScanRunByID(ctx, row.ScanRunID)
	if err != nil {
		return Freshness{}, fmt.Errorf("service: freshness %s: %w", path, err)
	}
	at := run.StartedAt
	if run.FinishedAt != nil {
		at = *run.FinishedAt
	}
	return Freshness{
		ScannedAt:  at.UTC(),
		ScanStatus: run.Status,
		Stale:      s.now().Sub(at) > StaleAfter,
	}, nil
}

// anyMissing reports whether any of paths no longer exists on disk.
func (s *Service) anyMissing(paths []string) bool {
	for _, p := range paths {
		if !s.exists(p) {
			return true
		}
	}
	return false
}

// pathExists reports whether path is present (without following a
// final symlink).
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
