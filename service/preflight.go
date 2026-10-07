package service

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/platform"
)

// SystemProbe reports machine-level storage state. platform.System is
// the production implementation; tests supply a fake.
type SystemProbe interface {
	Volume(path string) (platform.VolumeStats, error)
	ContainerFreeBytes(ctx context.Context, path string) (int64, error)
	LocalSnapshots(ctx context.Context) ([]string, error)
}

// WithProbe overrides the system probe (for tests).
func (s *Service) WithProbe(p SystemProbe) *Service {
	s.probe = p
	return s
}

// PreflightRequest asks whether the volume holding Path has NeedBytes free.
type PreflightRequest struct {
	Path      string
	NeedBytes int64
}

// PreflightResult is the answer to a PreflightRequest. Everything is
// read-only observation; the verdict is based on AvailableBytes alone.
type PreflightResult struct {
	Path           string
	MountPoint     string
	FSType         string
	CapacityBytes  int64
	AvailableBytes int64
	// ContainerFreeBytes is the free space of the APFS container, when
	// it could be determined (0 otherwise). Volumes in one container
	// share this pool.
	ContainerFreeBytes int64 `json:",omitempty"`
	LocalSnapshots     []string
	NeedBytes          int64
	// Sufficient is true when AvailableBytes >= NeedBytes.
	Sufficient     bool
	ShortfallBytes int64
	Notes          []string
}

// Preflight reports free space for the volume holding req.Path and
// whether it meets req.NeedBytes (0 reports without a verdict
// threshold, so Sufficient is always true). It reads current system
// state rather than the index. Probing the container free space and
// local snapshots is best-effort: failures are noted, not fatal.
func (s *Service) Preflight(ctx context.Context, req PreflightRequest) (*PreflightResult, error) {
	vol, err := s.probe.Volume(req.Path)
	if err != nil {
		return nil, fmt.Errorf("service: preflight %s: %w", req.Path, err)
	}

	res := &PreflightResult{
		Path:           req.Path,
		MountPoint:     vol.MountPoint,
		FSType:         vol.FSType,
		CapacityBytes:  vol.CapacityBytes,
		AvailableBytes: vol.AvailableBytes,
		NeedBytes:      req.NeedBytes,
		Sufficient:     vol.AvailableBytes >= req.NeedBytes,
	}
	if !res.Sufficient {
		res.ShortfallBytes = req.NeedBytes - vol.AvailableBytes
	}

	if free, err := s.probe.ContainerFreeBytes(ctx, req.Path); err != nil {
		res.Notes = append(res.Notes, fmt.Sprintf("container free space unavailable: %v", err))
	} else {
		res.ContainerFreeBytes = free
	}

	snaps, err := s.probe.LocalSnapshots(ctx)
	if err != nil {
		res.Notes = append(res.Notes, fmt.Sprintf("local snapshots unavailable: %v", err))
	}
	res.LocalSnapshots = snaps

	res.Notes = append(res.Notes,
		"purgeable space (caches and snapshots macOS evicts on demand) is not measurable from the command line and is not counted; the verdict uses space available right now")
	if !res.Sufficient && len(snaps) > 0 {
		res.Notes = append(res.Notes,
			fmt.Sprintf("%d local snapshot(s) exist; macOS may reclaim them automatically when an installer needs room", len(snaps)))
	}
	return res, nil
}
