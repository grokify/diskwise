// Package report renders a set of opportunities into a
// human-shareable file — HTML for interactive triage in a browser,
// XLSX for spreadsheet sorting/filtering — without adding a UI
// framework or persisting a separate report format. Both writers
// consume the same Row slice so the two outputs never drift.
package report

import (
	"sort"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

// Row is one finding, flattened for display. It carries the same
// data as service.Opportunity plus its rolled-up path count, so a
// report writer never needs to reach back into detect/service types.
type Row struct {
	Tier          policy.ActionClass
	Kind          entity.Kind
	Path          string
	AllocatedSize int64
	LogicalSize   int64
	Confidence    float64
	Reason        string
	PathCount     int
	Scenarios     []detect.Scenario
}

// tierOrder is most-actionable first, matching how a user triages:
// what's free to reclaim right now, down to what needs the most
// caution, with unknown (unclassified) last since it's not a
// recommendation at all.
var tierOrder = map[policy.ActionClass]int{
	policy.SafeDelete:       0,
	policy.LikelySafe:       1,
	policy.BackupThenDelete: 2,
	policy.Review:           3,
	policy.Keep:             4,
	policy.Unknown:          5,
}

func tierRank(tier policy.ActionClass) int {
	if r, ok := tierOrder[tier]; ok {
		return r
	}
	return len(tierOrder) // unrecognized tiers sort last, not first
}

// Rows flattens opportunities into report rows, sorted by tier then
// by allocated size descending within each tier — the same priority
// order a user would triage in.
func Rows(opps []service.Opportunity) []Row {
	rows := make([]Row, len(opps))
	for i, o := range opps {
		rows[i] = Row{
			Tier:          o.Finding.ActionClass,
			Kind:          o.Finding.Entity.Kind,
			Path:          o.Finding.Path,
			AllocatedSize: o.Finding.AllocatedSize,
			LogicalSize:   o.Finding.LogicalSize,
			Confidence:    o.Finding.Confidence,
			Reason:        o.Finding.Reason,
			PathCount:     len(o.Paths),
			Scenarios:     o.Finding.Scenarios,
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := tierRank(rows[i].Tier), tierRank(rows[j].Tier)
		if ri != rj {
			return ri < rj
		}
		return rows[i].AllocatedSize > rows[j].AllocatedSize
	})
	return rows
}
