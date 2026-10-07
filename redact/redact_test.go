package redact

import (
	"strings"
	"testing"
	"time"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/reportdoc"
	"github.com/grokify/diskwise/service"
)

const home = "/Users/example"

func TestPath(t *testing.T) {
	r := New(home, []string{"~/work/acme", "/Volumes/Secret"})
	tests := []struct{ in, want string }{
		{"/Users/example/Downloads/a.zip", "~/Downloads/a.zip"},
		{"/Users/example", "~"},
		{home + "xyz/Downloads", home + "xyz/Downloads"}, // shares home as a string prefix, but not a path segment
		{"/opt/tool", "/opt/tool"},
	}
	for _, tt := range tests {
		if got := r.Path(tt.in); got != tt.want {
			t.Errorf("Path(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	denied := r.Path("/Users/example/work/acme/project/sensitivePath.zip")
	if !strings.HasPrefix(denied, "<redacted:") || strings.Contains(denied, "acme") {
		t.Errorf("deny-prefix path = %q, want an opaque token", denied)
	}
	if denied != r.Path("/Users/example/work/acme/project/sensitivePath.zip") {
		t.Error("token must be stable for the same path")
	}
	if denied == r.Path("/Users/example/work/acme/project/other.zip") {
		t.Error("different paths must yield different tokens")
	}
	if got := r.Path("/Users/example/work/acmetwo/x"); strings.HasPrefix(got, "<redacted:") {
		t.Errorf("prefix must match on segment boundaries, got %q", got)
	}
	if got := r.Path("/Volumes/Secret/x"); !strings.HasPrefix(got, "<redacted:") {
		t.Errorf("absolute deny prefix not applied: %q", got)
	}
}

func TestFinding_ScrubsNamesAndText(t *testing.T) {
	r := New(home, []string{"~/work/acme"})
	sensitivePath := "/Users/example/work/acme/payroll-export-2026.zip"
	f := detect.Finding{
		Entity: entity.Entity{ID: sensitivePath, Name: "payroll-export-2026.zip", Kind: entity.KindArchive},
		Path:   sensitivePath,
		Paths:  []string{sensitivePath},
		Reason: "downloaded archive payroll-export-2026.zip under /Users/example/work/acme",
		Scenarios: []detect.Scenario{
			{Name: "keep-newest", Description: "Keep payroll-export-2026.zip", Paths: []string{sensitivePath}},
		},
	}
	got := r.Finding(f)

	blob := strings.Join([]string{got.Entity.ID, got.Entity.Name, got.Path, strings.Join(got.Paths, ","), got.Reason,
		got.Scenarios[0].Description, strings.Join(got.Scenarios[0].Paths, ",")}, "\n")
	for _, leak := range []string{"payroll", "acme", "example"} {
		if strings.Contains(blob, leak) {
			t.Errorf("redacted finding still contains %q:\n%s", leak, blob)
		}
	}
	if f.Path != sensitivePath || f.Scenarios[0].Description != "Keep payroll-export-2026.zip" {
		t.Error("Finding must not mutate its input")
	}
}

func TestFinding_HomeOnlyKeepsNames(t *testing.T) {
	r := New(home, nil)
	f := detect.Finding{
		Entity: entity.Entity{ID: "/Users/example/Downloads/tool.dmg", Name: "tool.dmg"},
		Path:   "/Users/example/Downloads/tool.dmg",
		Paths:  []string{"/Users/example/Downloads/tool.dmg"},
		Reason: "installer in /Users/example/Downloads",
	}
	got := r.Finding(f)
	if got.Path != "~/Downloads/tool.dmg" || got.Entity.Name != "tool.dmg" || got.Reason != "installer in ~/Downloads" {
		t.Errorf("home-only redaction wrong: %+v", got)
	}
}

func TestHotspotsAndOpportunities(t *testing.T) {
	r := New(home, []string{"~/work"})
	f := detect.Finding{Path: "/Users/example/work/x", Paths: []string{"/Users/example/work/x"}}
	opps := r.Opportunities([]service.Opportunity{{Finding: f, Paths: f.Paths}})
	if !strings.HasPrefix(opps[0].Paths[0], "<redacted:") || !strings.HasPrefix(opps[0].Finding.Path, "<redacted:") {
		t.Errorf("opportunity not redacted: %+v", opps[0])
	}
	rep := r.Hotspots(&service.HotspotsReport{Root: home, Known: []detect.Finding{f}, Unexplained: []detect.Finding{f}})
	if rep.Root != "~" || !strings.HasPrefix(rep.Known[0].Path, "<redacted:") || !strings.HasPrefix(rep.Unexplained[0].Path, "<redacted:") {
		t.Errorf("hotspots not redacted: %+v", rep)
	}
}

func TestPairs(t *testing.T) {
	r := New(home, []string{"~/work"})
	in := []detect.ArchivePair{{
		Archive: "/Users/example/work/payroll.tar", Dir: "/Users/example/work/payroll",
		Detail: "read /Users/example/work/payroll.tar: unexpected EOF",
	}, {
		Archive: "/Users/example/Downloads/x.tar", Dir: "/Users/example/Downloads/x",
	}}
	got := r.Pairs(in)
	blob := got[0].Archive + got[0].Dir + got[0].Detail
	for _, leak := range []string{"payroll", "work", "example"} {
		if strings.Contains(blob, leak) {
			t.Errorf("redacted pair still contains %q: %s", leak, blob)
		}
	}
	if got[1].Archive != "~/Downloads/x.tar" || got[1].Dir != "~/Downloads/x" {
		t.Errorf("home-only pair = %+v", got[1])
	}
	if in[0].Archive != "/Users/example/work/payroll.tar" {
		t.Error("Pairs must not mutate its input")
	}
}

// The newer result fields must survive redaction: dropping them would
// quietly turn a stale, filtered report into a complete-looking one.
func TestRedaction_PreservesFreshnessAndFilters(t *testing.T) {
	r := New(home, []string{"~/work"})
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fresh := service.Freshness{ScannedAt: at, ScanStatus: "partial", Stale: true}
	f := detect.Finding{Path: "/Users/example/work/x", Paths: []string{"/Users/example/work/x"}}

	rep := r.OpportunitiesReport(&service.OpportunitiesReport{
		Root: home, Freshness: fresh, MinSize: 5, OmittedCount: 2, OmittedBytes: 9, MissingCount: 1,
		Opportunities: []service.Opportunity{{Finding: f, Paths: f.Paths, Missing: true}},
	})
	if rep.Root != "~" || rep.Freshness != fresh || rep.MinSize != 5 || rep.OmittedCount != 2 || rep.OmittedBytes != 9 || rep.MissingCount != 1 {
		t.Errorf("report fields lost: %+v", rep)
	}
	if !rep.Opportunities[0].Missing || !strings.HasPrefix(rep.Opportunities[0].Finding.Path, "<redacted:") {
		t.Errorf("opportunity not redacted or lost Missing: %+v", rep.Opportunities[0])
	}

	sav := r.Savings(&service.SavingsByTier{Path: home, Freshness: fresh, MissingCount: 3, MissingBytes: 7})
	if sav.Path != "~" || sav.Freshness != fresh || sav.MissingCount != 3 || sav.MissingBytes != 7 {
		t.Errorf("savings fields lost: %+v", sav)
	}

	hot := r.Hotspots(&service.HotspotsReport{Root: home, Freshness: fresh, MissingKnown: []string{"/Users/example/work/x", "/Users/example/Downloads/y"}})
	if hot.Freshness != fresh || !strings.HasPrefix(hot.MissingKnown[0], "<redacted:") || hot.MissingKnown[1] != "~/Downloads/y" {
		t.Errorf("hotspots fields lost or unredacted: %+v", hot)
	}
}

func TestDocument_RedactsEverythingAndKeepsTheRest(t *testing.T) {
	r := New(home, []string{"~/work"})
	sensitivePath := "/Users/example/work/acme/payroll-export.zip"
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	in := &reportdoc.Document{
		SchemaVersion: reportdoc.SchemaVersion, Root: home, MeasuredAt: at, ScanStatus: "partial", Stale: true,
		Tiers: map[string]int64{"review": 5}, ReclaimableBytes: 5,
		Missing: reportdoc.Missing{Count: 1, Bytes: 2}, Filter: reportdoc.Filter{MinSizeBytes: 9, OmittedCount: 3, OmittedBytes: 4},
		Findings: []reportdoc.Finding{
			{
				Tier: "review", Kind: "archive", Name: "payroll-export.zip", Path: sensitivePath, Paths: []string{sensitivePath},
				ActionablePaths: []string{sensitivePath}, AllocatedBytes: 5, Confidence: 0.3, Missing: true,
				Reason:    "downloaded archive payroll-export.zip under /Users/example/work/acme",
				Scenarios: []reportdoc.Scenario{{Name: "keep-newest", Description: "Keep payroll-export.zip", ReclaimableBytes: 1, Paths: []string{sensitivePath}}},
			},
			{Tier: "safe_delete", Kind: "cache", Name: "Go caches", Path: "/Users/example/go/pkg/mod", AllocatedBytes: 7, Confidence: 1},
		},
		Pairs: []reportdoc.Pair{
			{Archive: sensitivePath, Dir: "/Users/example/work/acme/payroll-export", Verdict: "same", Detail: "read " + sensitivePath + ": boom"},
			{Archive: "/Users/example/Downloads/x.tar", Dir: "/Users/example/Downloads/x", Verdict: "same"},
		},
	}
	before, _ := reportdoc.Marshal(*in)
	got := r.Document(in)
	after, _ := reportdoc.Marshal(*in)
	if string(before) != string(after) {
		t.Fatal("Document must not mutate its input")
	}

	blob, err := reportdoc.Marshal(*got)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"payroll", "acme", "alice", "/Users/example/work", "Users/example"} {
		if strings.Contains(string(blob), leak) {
			t.Errorf("redacted document still contains %q:\n%s", leak, blob)
		}
	}

	// Everything that is not a path survives, so a redacted report is
	// still accurate about size, age and what was left out.
	if got.Root != "~" || !got.MeasuredAt.Equal(at) || got.ScanStatus != "partial" || !got.Stale ||
		got.ReclaimableBytes != 5 || got.Tiers["review"] != 5 || got.Missing.Count != 1 || got.Filter.OmittedCount != 3 {
		t.Errorf("document-level fields lost: %+v", got)
	}
	f := got.Findings[0]
	if !f.Missing || f.AllocatedBytes != 5 || f.Confidence != 0.3 || f.Tier != "review" || f.Scenarios[0].ReclaimableBytes != 1 {
		t.Errorf("finding fields lost: %+v", f)
	}
	if got.Findings[1].Path != "~/go/pkg/mod" || got.Findings[1].Name != "Go caches" {
		t.Errorf("a finding outside the deny prefix should only get the home abbreviation: %+v", got.Findings[1])
	}
	if got.Pairs[1].Archive != "~/Downloads/x.tar" || got.Pairs[0].Verdict != "same" {
		t.Errorf("pairs wrong: %+v", got.Pairs)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("a redacted document must still be valid: %v", err)
	}
}

// Free text can name a directory ABOVE the redacted file. Scrubbing only
// the deny prefix would leave the rest of that path ("/acme") behind.
func TestFinding_ScrubsWholeDirectoryPathsMentionedInText(t *testing.T) {
	r := New(home, []string{"~/work"})
	f := detect.Finding{
		Path: "/Users/example/work/acme/project/file.zip", Paths: []string{"/Users/example/work/acme/project/file.zip"},
		Reason: "found in /Users/example/work/acme/project (and /Users/example/work/other-client/x), kept",
	}
	got := r.Finding(f).Reason
	for _, leak := range []string{"acme", "other-client", "project", "Users"} {
		if strings.Contains(got, leak) {
			t.Errorf("reason still contains %q: %q", leak, got)
		}
	}
	if !strings.Contains(got, "kept") || !strings.Contains(got, "found in") {
		t.Errorf("the surrounding words should survive: %q", got)
	}
}
