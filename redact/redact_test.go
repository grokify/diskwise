package redact

import (
	"strings"
	"testing"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
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
