package reportdoc

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

// BuildOptions configures Build.
type BuildOptions struct {
	// Path is the scanned path the report covers.
	Path string
	// MinSize omits findings smaller than this many allocated bytes
	// (0 keeps everything); the document records what it omitted.
	MinSize int64
	// Pairs adds archive/extracted-directory comparisons. They read
	// archive member lists from disk and can take minutes on a large tree.
	Pairs bool
	// PairsMinSize skips archives smaller than this when Pairs is set.
	PairsMinSize int64
	// Progress, if non-nil, is called with each archive before it is read.
	Progress func(archive string, size int64)
}

// Build runs the detectors once and assembles the document from the
// index: findings (with the size cut applied), tier totals over all
// findings, freshness, and optionally archive pairs.
func Build(ctx context.Context, svc *service.Service, opts BuildOptions) (*Document, error) {
	all, err := svc.Opportunities(ctx, service.OpportunityQuery{Path: opts.Path})
	if err != nil {
		return nil, err
	}
	rep, err := svc.OpportunitiesReportFrom(ctx, service.OpportunityQuery{Path: opts.Path, MinSize: opts.MinSize}, all)
	if err != nil {
		return nil, err
	}
	sav, err := svc.SavingsFrom(ctx, opts.Path, all)
	if err != nil {
		return nil, err
	}

	var pairs []detect.ArchivePair
	if opts.Pairs {
		pairs, err = svc.ArchivePairs(ctx, service.ArchivePairQuery{
			Path: opts.Path, MinSize: opts.PairsMinSize, Progress: opts.Progress,
		})
		if err != nil {
			return nil, err
		}
	}
	d := FromService(rep, sav, pairs)
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("reportdoc: built an invalid document: %w", err)
	}
	return d, nil
}

// FromService converts service results into a Document. It is pure, so
// callers that already hold the results (and tests) need no index.
func FromService(rep *service.OpportunitiesReport, sav *service.SavingsByTier, pairs []detect.ArchivePair) *Document {
	d := &Document{
		SchemaVersion:    SchemaVersion,
		Root:             rep.Root,
		MeasuredAt:       rep.ScannedAt.UTC(),
		ScanStatus:       rep.ScanStatus,
		Stale:            rep.Stale,
		Tiers:            make(map[string]int64, len(sav.Tiers)),
		ReclaimableBytes: sav.Reclaimable(),
		Missing:          Missing{Count: sav.MissingCount, Bytes: sav.MissingBytes},
		Filter:           Filter{MinSizeBytes: rep.MinSize, OmittedCount: rep.OmittedCount, OmittedBytes: rep.OmittedBytes},
		Findings:         make([]Finding, 0, len(rep.Opportunities)),
	}
	for tier, v := range sav.Tiers {
		d.Tiers[string(tier)] = v
	}
	for _, o := range rep.Opportunities {
		f := o.Finding
		out := Finding{
			Tier:            string(f.ActionClass),
			Kind:            string(f.Entity.Kind),
			Name:            f.Entity.Name,
			Detector:        f.Entity.Detector,
			Path:            f.Path,
			Paths:           f.Paths,
			ActionablePaths: o.Paths,
			AllocatedBytes:  f.AllocatedSize,
			LogicalBytes:    f.LogicalSize,
			Confidence:      f.Confidence,
			Reason:          f.Reason,
			Missing:         o.Missing,
		}
		for _, sc := range f.Scenarios {
			out.Scenarios = append(out.Scenarios, Scenario{
				Name: sc.Name, Description: sc.Description, ReclaimableBytes: sc.ReclaimableBytes, Paths: sc.Paths,
			})
		}
		d.Findings = append(d.Findings, out)
	}
	for _, p := range pairs {
		d.Pairs = append(d.Pairs, Pair{
			Archive: p.Archive, ArchiveAlloc: p.ArchiveAlloc, Dir: p.Dir, DirAlloc: p.DirAlloc,
			Compared: p.Compared, ArchiveFiles: p.ArchiveFiles, ArchiveBytes: p.ArchiveBytes,
			DirFiles: p.DirFiles, DirBytes: p.DirBytes, Verdict: string(p.Verdict),
			Detail: p.Detail, ArchiveExtra: p.ArchiveExtra,
		})
	}
	return d
}

// Reclaimable reports whether a tier counts toward ReclaimableBytes.
func Reclaimable(tier string) bool {
	return tier != string(policy.Keep) && tier != string(policy.Unknown)
}
