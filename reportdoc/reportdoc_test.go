package reportdoc

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

func sampleService() (*service.OpportunitiesReport, *service.SavingsByTier, []detect.ArchivePair) {
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("x", 3600))
	fresh := service.Freshness{ScannedAt: at, ScanStatus: "partial", Stale: true}
	cache := detect.Finding{
		Entity: entity.Entity{ID: "/r/cache", Kind: entity.KindCache, Name: "Go caches", Detector: "known-location:go-caches"},
		Path:   "/r/cache", Paths: []string{"/r/cache"},
		AllocatedSize: 3000, LogicalSize: 3100, Confidence: 1, ActionClass: policy.SafeDelete, Reason: "regeneratable",
		Scenarios: []detect.Scenario{{Name: "keep-newest", Description: "keep v2", ReclaimableBytes: 10, Paths: []string{"/r/old"}}},
	}
	rep := &service.OpportunitiesReport{
		Root: "/r", Freshness: fresh, MinSize: 500, OmittedCount: 4, OmittedBytes: 800, MissingCount: 1,
		Opportunities: []service.Opportunity{{Finding: cache, Paths: []string{"/r/cache"}, Missing: true}},
	}
	sav := &service.SavingsByTier{
		Path: "/r", Freshness: fresh, MissingCount: 1, MissingBytes: 3000,
		Tiers: map[policy.ActionClass]int64{policy.SafeDelete: 3800, policy.Keep: 9000, policy.Unknown: 70000},
	}
	pairs := []detect.ArchivePair{{
		Archive: "/r/a.tar", ArchiveAlloc: 10, Dir: "/r/a", DirAlloc: 9, Compared: detect.CompareAllFiles,
		ArchiveFiles: 3, ArchiveBytes: 30, DirFiles: 2, DirBytes: 20, Verdict: detect.PairArchiveHasMore, ArchiveExtra: 1,
	}}
	return rep, sav, pairs
}

func TestFromService_MapsEverythingOnce(t *testing.T) {
	rep, sav, pairs := sampleService()
	d := FromService(rep, sav, pairs)

	if err := d.Validate(); err != nil {
		t.Fatalf("converted document invalid: %v", err)
	}
	if d.Root != "/r" || d.ScanStatus != "partial" || !d.Stale {
		t.Errorf("header = %q %q stale=%v", d.Root, d.ScanStatus, d.Stale)
	}
	if d.MeasuredAt.Location() != time.UTC || d.MeasuredAt.Hour() != 4 {
		t.Errorf("MeasuredAt = %v, want the instant converted to UTC (04:06)", d.MeasuredAt)
	}
	// Totals cover everything (including tiers not reclaimable) and the
	// reclaimable figure excludes keep and unknown.
	if d.Tiers["safe_delete"] != 3800 || d.Tiers["keep"] != 9000 || d.Tiers["unknown"] != 70000 {
		t.Errorf("Tiers = %v", d.Tiers)
	}
	if d.ReclaimableBytes != 3800 {
		t.Errorf("ReclaimableBytes = %d, want 3800 (keep and unknown excluded)", d.ReclaimableBytes)
	}
	if d.Missing != (Missing{Count: 1, Bytes: 3000}) {
		t.Errorf("Missing = %+v", d.Missing)
	}
	if d.Filter != (Filter{MinSizeBytes: 500, OmittedCount: 4, OmittedBytes: 800}) {
		t.Errorf("Filter = %+v", d.Filter)
	}

	if len(d.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(d.Findings))
	}
	f := d.Findings[0]
	if f.Tier != "safe_delete" || f.Kind != "cache" || f.Name != "Go caches" || f.Detector != "known-location:go-caches" ||
		f.Path != "/r/cache" || f.AllocatedBytes != 3000 || f.LogicalBytes != 3100 || !f.Missing ||
		!reflect.DeepEqual(f.ActionablePaths, []string{"/r/cache"}) || len(f.Scenarios) != 1 || f.Scenarios[0].ReclaimableBytes != 10 {
		t.Errorf("finding mapped wrong: %+v", f)
	}
	if len(d.Pairs) != 1 || d.Pairs[0].Verdict != "archive_has_more" || d.Pairs[0].ArchiveExtra != 1 || d.Pairs[0].Compared != "all files" {
		t.Errorf("pair mapped wrong: %+v", d.Pairs)
	}
}

func TestMarshalParse_RoundTrip(t *testing.T) {
	rep, sav, pairs := sampleService()
	d := *FromService(rep, sav, pairs)

	data, err := Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("document should end with a newline")
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(Marshal(d)): %v", err)
	}
	if !reflect.DeepEqual(d, got) {
		t.Errorf("round trip changed the document:\n got %+v\nwant %+v", got, d)
	}
	again, _ := Marshal(got)
	if string(again) != string(data) {
		t.Error("marshalling must be deterministic")
	}
	// camelCase on the wire, per the document-format convention.
	for _, key := range []string{`"schemaVersion"`, `"measuredAt"`, `"reclaimableBytes"`, `"allocatedBytes"`, `"actionablePaths"`, `"omittedCount"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("document missing camelCase key %s", key)
		}
	}
}

func TestEmptyFindingsEncodeAsList(t *testing.T) {
	rep, sav, _ := sampleService()
	rep.Opportunities = nil
	d := FromService(rep, sav, nil)
	data, err := Marshal(*d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"findings": []`) {
		t.Errorf("an empty findings list must encode as [], not null:\n%s", data)
	}
	if strings.Contains(string(data), `"pairs"`) {
		t.Error("pairs must be omitted when not requested")
	}
}

func TestParse_RejectsBadDocuments(t *testing.T) {
	good := `{"schemaVersion":"1","root":"/r","measuredAt":"2026-01-01T00:00:00Z","scanStatus":"complete","stale":false,
	"tiers":{"safe_delete":1},"reclaimableBytes":1,"missing":{"count":0,"bytes":0},"filter":{"minSizeBytes":0,"omittedCount":0,"omittedBytes":0},
	"findings":[{"tier":"safe_delete","kind":"cache","path":"/r/c","allocatedBytes":1,"logicalBytes":1,"confidence":1}]}`
	if _, err := Parse([]byte(good)); err != nil {
		t.Fatalf("good document rejected: %v", err)
	}

	tests := map[string]string{
		"unknown field":   strings.Replace(good, `"stale":false`, `"stale":false,"bogus":1`, 1),
		"wrong version":   strings.Replace(good, `"schemaVersion":"1"`, `"schemaVersion":"9"`, 1),
		"missing root":    strings.Replace(good, `"root":"/r"`, `"root":""`, 1),
		"unknown tier":    strings.Replace(good, `"tier":"safe_delete"`, `"tier":"delete_now"`, 1),
		"bad confidence":  strings.Replace(good, `"confidence":1`, `"confidence":2`, 1),
		"finding no path": strings.Replace(good, `"path":"/r/c"`, `"path":""`, 1),
		"tiers bad key":   strings.Replace(good, `"tiers":{"safe_delete":1}`, `"tiers":{"nope":1}`, 1),
	}
	for name, doc := range tests {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Parse([]byte(`{`)); err == nil {
		t.Error("malformed JSON should be an error")
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Build against a real scanned fixture: one detector run feeds the
// findings, the pre-cut tier totals, the size cut and the pairs.
func TestBuild_FromIndexedFixture(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "AppCache")
	writeFile(t, filepath.Join(cache, "data.bin"), 1_000_000)
	writeFile(t, filepath.Join(root, "tool-1.0.dmg"), 20_000)
	writeFile(t, filepath.Join(root, "Stuff", "a.txt"), 10)

	db, err := index.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	reg := knowledge.Registry{{
		ID: "fixture-cache", Description: "Fixture cache", DataPaths: []string{cache},
		Entity: entity.KindCache, Semantics: knowledge.SemanticsRegeneratableCache, DefaultActionClass: policy.SafeDelete,
	}}
	svc := service.New(db).WithRegistry(reg)
	ctx := context.Background()
	if _, err := svc.Scan(ctx, service.ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	all, err := Build(ctx, svc, BuildOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	cut, err := Build(ctx, svc, BuildOptions{Path: root, MinSize: 500_000})
	if err != nil {
		t.Fatal(err)
	}

	if len(cut.Findings) >= len(all.Findings) || cut.Filter.OmittedCount != len(all.Findings)-len(cut.Findings) {
		t.Errorf("cut: %d findings / omitted %d, uncut: %d", len(cut.Findings), cut.Filter.OmittedCount, len(all.Findings))
	}
	if !reflect.DeepEqual(all.Tiers, cut.Tiers) || all.ReclaimableBytes != cut.ReclaimableBytes {
		t.Errorf("tier totals must not change with the size cut:\n all %v / %d\n cut %v / %d", all.Tiers, all.ReclaimableBytes, cut.Tiers, cut.ReclaimableBytes)
	}
	if all.Tiers["safe_delete"] < 1_000_000 {
		t.Errorf("Tiers = %v, want the 1 MB cache under safe_delete", all.Tiers)
	}
	if all.MeasuredAt.IsZero() || all.ScanStatus == "" {
		t.Errorf("freshness missing: %v %q", all.MeasuredAt, all.ScanStatus)
	}
	if all.Pairs != nil {
		t.Error("pairs were not requested")
	}

	withPairs, err := Build(ctx, svc, BuildOptions{Path: root, Pairs: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := withPairs.Validate(); err != nil {
		t.Errorf("built document invalid: %v", err)
	}

	if _, err := Build(ctx, svc, BuildOptions{Path: "/nowhere"}); err == nil {
		t.Error("an unindexed path should fail")
	}
}
