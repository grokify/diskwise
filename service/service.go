package service

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/platform"
	"github.com/grokify/diskwise/scan"
)

// Service is the single API surface the CLI and MCP adapters call. It
// owns the index and translates between the scan/index packages'
// internal representations and the request/response types below,
// which double as the JSON contract for both adapters.
type Service struct {
	db       *index.DB
	registry knowledge.Registry
}

// New returns a Service backed by db, using the built-in known-location registry.
func New(db *index.DB) *Service {
	return &Service{db: db, registry: knowledge.Default}
}

// WithRegistry overrides the knowledge registry (primarily for tests,
// which need hermetic fixture paths rather than knowledge.Default's
// real machine locations).
func (s *Service) WithRegistry(r knowledge.Registry) *Service {
	s.registry = r
	return s
}

// ScanRequest configures a Scan command.
type ScanRequest struct {
	Root        string
	Depth       int
	Workers     int
	CrossDevice bool
	// Stats, if non-nil, is populated live during the walk so a caller
	// can poll progress from another goroutine.
	Stats *scan.Stats
}

// ScanResult is the outcome of a Scan or Rescan command.
type ScanResult struct {
	Run   index.ScanRun
	Stats scan.Stats
}

// Scan walks req.Root and records the result as a new scan run. Known
// heavy-storage locations from the registry are prioritized during
// the walk so they resolve early rather than by chance.
func (s *Service) Scan(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	priorityPaths, err := relevantDataPaths(s.registry)
	if err != nil {
		return nil, fmt.Errorf("service: scan %s: %w", req.Root, err)
	}

	tree, stats, err := scan.Walk(ctx, req.Root, scan.Options{
		Depth:         req.Depth,
		Workers:       req.Workers,
		CrossDevice:   req.CrossDevice,
		Stats:         req.Stats,
		PriorityPaths: priorityPaths,
	})
	if err != nil {
		return nil, fmt.Errorf("service: scan %s: %w", req.Root, err)
	}

	var vol *platform.VolumeStats
	if vs, verr := platform.VolumeStatsForPath(req.Root); verr == nil {
		vol = &vs
	}

	run, err := s.db.IngestScan(ctx, tree, vol)
	if err != nil {
		return nil, fmt.Errorf("service: ingest scan of %s: %w", req.Root, err)
	}
	return &ScanResult{Run: *run, Stats: *stats}, nil
}

// RescanRequest configures a Rescan command.
type RescanRequest struct {
	Path string
	// Stats, if non-nil, is populated live during the walk so a caller
	// can poll progress from another goroutine.
	Stats *scan.Stats
}

// Rescan re-walks req.Path and updates the index, propagating the
// byte delta to any recorded ancestors.
func (s *Service) Rescan(ctx context.Context, req RescanRequest) (*ScanResult, error) {
	_, stats, run, err := s.db.Rescan(ctx, req.Path, scan.Options{Stats: req.Stats})
	if err != nil {
		return nil, fmt.Errorf("service: rescan %s: %w", req.Path, err)
	}
	return &ScanResult{Run: *run, Stats: *stats}, nil
}

// errNotIndexed reports that a query path has never been scanned.
func errNotIndexed(path string) error {
	return fmt.Errorf("%s is not indexed; run `diskwise scan %s` first", path, path)
}

// relevantDataPaths expands every registry entry applicable to this
// machine into its data paths, for use as scan.Options.PriorityPaths.
func relevantDataPaths(registry knowledge.Registry) ([]string, error) {
	relevant, err := registry.Relevant()
	if err != nil {
		return nil, fmt.Errorf("registry: %w", err)
	}
	var paths []string
	for _, loc := range relevant {
		expanded, err := loc.ExpandedDataPaths()
		if err != nil {
			return nil, fmt.Errorf("registry %s: %w", loc.ID, err)
		}
		paths = append(paths, expanded...)
	}
	return paths, nil
}
