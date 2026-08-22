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
}

// Opportunity is one reclaimable finding with its directly-actionable
// paths — the highest fully-covered directory wherever every child is
// part of the finding, individual files otherwise.
type Opportunity struct {
	Finding detect.Finding
	Paths   []string
}

// Opportunities runs every Phase 3 detector under q.Path and returns
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

	detectors := []detect.Detector{
		detect.KnownLocationDetector{Registry: s.registry},
		detect.ArchiveInstallerDetector{Registry: s.registry},
		detect.ArtifactFamilyDetector{Registry: s.registry},
		detect.LargeUnexplainedDetector{Registry: s.registry},
	}

	var out []Opportunity
	for _, d := range detectors {
		findings, err := d.Detect(ctx, s.db, q.Path)
		if err != nil {
			return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
		}
		for _, f := range findings {
			if q.ActionClass != "" && f.ActionClass != q.ActionClass {
				continue
			}
			if f.Confidence < q.MinConfidence {
				continue
			}
			if q.Kind != "" && f.Entity.Kind != q.Kind {
				continue
			}

			paths, err := rollup.Collapse(ctx, s.db, f.Paths)
			if err != nil {
				return nil, fmt.Errorf("service: opportunities %s: %w", q.Path, err)
			}
			out = append(out, Opportunity{Finding: f, Paths: paths})
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
