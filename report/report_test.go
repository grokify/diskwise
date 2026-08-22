package report

import (
	"testing"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

func opp(tier policy.ActionClass, kind entity.Kind, path string, allocated int64) service.Opportunity {
	return service.Opportunity{
		Finding: detect.Finding{
			Entity:        entity.Entity{Kind: kind},
			Path:          path,
			AllocatedSize: allocated,
			LogicalSize:   allocated,
			ActionClass:   tier,
			Confidence:    0.9,
		},
		Paths: []string{path},
	}
}

func TestRows_SortsByTierThenSizeDescending(t *testing.T) {
	opps := []service.Opportunity{
		opp(policy.Review, entity.KindArchive, "/small-review", 10),
		opp(policy.SafeDelete, entity.KindCache, "/small-safe", 5),
		opp(policy.SafeDelete, entity.KindCache, "/big-safe", 100),
		opp(policy.Unknown, entity.KindUnknown, "/huge-unknown", 1_000_000),
	}

	rows := Rows(opps)
	if len(rows) != 4 {
		t.Fatalf("len(rows) = %d, want 4", len(rows))
	}

	want := []string{"/big-safe", "/small-safe", "/small-review", "/huge-unknown"}
	for i, path := range want {
		if rows[i].Path != path {
			t.Errorf("rows[%d].Path = %q, want %q (tier order must win over raw size)", i, rows[i].Path, path)
		}
	}
}

func TestRows_UnrecognizedTierSortsLast(t *testing.T) {
	opps := []service.Opportunity{
		opp(policy.ActionClass("mystery_tier"), entity.KindUnknown, "/mystery", 1_000_000),
		opp(policy.Keep, entity.KindCache, "/keep", 1),
	}
	rows := Rows(opps)
	if rows[len(rows)-1].Path != "/mystery" {
		t.Errorf("unrecognized tier did not sort last: rows = %+v", rows)
	}
}
