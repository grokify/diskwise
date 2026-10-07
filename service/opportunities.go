package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/rollup"
)

// OpportunityQuery filters an Opportunities report.
type OpportunityQuery struct {
	Path string
	// ActionClass restricts results to one tier; "" returns all.
	ActionClass policy.ActionClass
	// MinConfidence excludes findings below this detection confidence.
	MinConfidence float64
	// Kind restricts results to one entity kind; "" returns all.
	Kind entity.Kind
	// MinUnexplainedSize is the smallest unexplained-directory remainder
	// worth reporting; 0 uses detect.DefaultUnexplainedMinSize.
	MinUnexplainedSize int64
}

// Opportunity is one reclaimable finding with its directly-actionable
// paths — the highest fully-covered directory wherever every child is
// part of the finding, individual files otherwise.
type Opportunity struct {
	Finding detect.Finding
	Paths   []string
}

// Opportunities runs every detector under q.Path and returns
// the findings matching q, each rolled up to its actionable paths.
// Each byte is reported by exactly one detector: KnownLocationDetector
// owns its registry paths; ArchiveInstallerDetector and
// ArtifactFamilyDetector each exclude the other's territory and any
// KnownLocation; LargeUnexplainedDetector excludes all claimed paths.
func (s *Service) Opportunities(ctx context.Context, q OpportunityQuery) ([]Opportunity, error) {
	if _, ok, err := s.db.NodeByPath(ctx, q.Path); err != nil {
		return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
	} else if !ok {
		return nil, errNotIndexed(q.Path)
	}

	// Claimed-territory detectors run first; the unexplained detector
	// runs last so its directory totals can exclude bytes they already
	// report.
	detectors := []detect.Detector{
		detect.KnownLocationDetector{Registry: s.registry},
		detect.ManagedBundleDetector{Registry: s.registry},
		detect.ArchiveInstallerDetector{Registry: s.registry},
		detect.ArtifactFamilyDetector{Registry: s.registry},
	}
	unexplained := detect.LargeUnexplainedDetector{Registry: s.registry, MinSize: q.MinUnexplainedSize}

	var out []Opportunity
	add := func(f detect.Finding) error {
		if q.ActionClass != "" && f.ActionClass != q.ActionClass {
			return nil
		}
		if f.Confidence < q.MinConfidence {
			return nil
		}
		if q.Kind != "" && f.Entity.Kind != q.Kind {
			return nil
		}
		paths, err := rollup.Collapse(ctx, s.db, f.Paths)
		if err != nil {
			return err
		}
		out = append(out, Opportunity{Finding: f, Paths: paths})
		return nil
	}

	// explained collects every path another detector claims, whether or
	// not q filtered the finding out: filtering is about what to show,
	// not about which bytes are already accounted for.
	var explained []string
	for _, d := range detectors {
		findings, err := d.Detect(ctx, s.db, q.Path)
		if err != nil {
			return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
		}
		for _, f := range findings {
			if err := add(f); err != nil {
				return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
			}
			explained = append(explained, f.Paths...)
		}
	}

	unknown, err := unexplained.Detect(ctx, s.db, q.Path)
	if err != nil {
		return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
	}
	unknown, err = excludeExplained(ctx, s.db, unknown, explained, unexplained.MinSize)
	if err != nil {
		return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
	}
	for _, f := range unknown {
		if err := add(f); err != nil {
			return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Finding.AllocatedSize > out[j].Finding.AllocatedSize })
	return out, nil
}

// SavingsByTier totals allocated bytes per action-class tier.
type SavingsByTier struct {
	Path  string
	Tiers map[policy.ActionClass]int64
}

// Reclaimable totals the tiers that represent space a user could
// actually recover (everything except Keep and Unknown). Unknown is
// not a savings estimate — it is the large directories no detector
// explains — and Keep is by definition not to be removed.
func (t *SavingsByTier) Reclaimable() int64 {
	var total int64
	for tier, v := range t.Tiers {
		if tier == policy.Keep || tier == policy.Unknown {
			continue
		}
		total += v
	}
	return total
}

// Savings summarizes Opportunities(path) by action-class tier —
// conservative (SafeDelete) versus what requires review or a backup
// first, without listing every individual finding.
func (s *Service) Savings(ctx context.Context, path string) (*SavingsByTier, error) {
	opps, err := s.Opportunities(ctx, OpportunityQuery{Path: path})
	if err != nil {
		return nil, err
	}
	tiers := make(map[policy.ActionClass]int64)
	for _, o := range opps {
		tiers[o.Finding.ActionClass] += o.Finding.AllocatedSize
	}
	return &SavingsByTier{Path: path, Tiers: tiers}, nil
}
