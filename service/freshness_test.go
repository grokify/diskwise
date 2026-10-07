package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grokify/diskwise/policy"
)

func scannedCacheFixture(t *testing.T) (svc *Service, root, cache string) {
	t.Helper()
	root = t.TempDir()
	cache = filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cache, "data.bin"), 1_000_000)
	writeFile(t, filepath.Join(root, "tool.dmg"), 50_000)

	svc = newTestService(t).WithRegistry(fixtureRegistry(cache))
	if _, err := svc.Scan(context.Background(), ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}
	return svc, root, cache
}

func TestFreshness_StaleAfterAWeek(t *testing.T) {
	svc, root, _ := scannedCacheFixture(t)
	ctx := context.Background()

	rep, err := svc.OpportunitiesReport(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Stale {
		t.Error("a scan from moments ago must not be stale")
	}
	if rep.ScanStatus == "" || rep.ScannedAt.IsZero() {
		t.Errorf("freshness not populated: %+v", rep.Freshness)
	}
	if rep.ScannedAt.Location() != time.UTC {
		t.Error("ScannedAt should be UTC so output is the same on every machine")
	}

	svc.now = func() time.Time { return rep.ScannedAt.Add(StaleAfter + time.Hour) }
	rep, err = svc.OpportunitiesReport(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Stale {
		t.Error("a scan older than StaleAfter must be flagged stale")
	}
}

// The motivating case: a cache is cleaned after the scan. The index
// still counts it, so the result must say the path is gone instead of
// presenting a phantom saving.
func TestMissing_PathRemovedAfterScan(t *testing.T) {
	svc, root, cache := scannedCacheFixture(t)
	ctx := context.Background()
	if err := os.RemoveAll(cache); err != nil {
		t.Fatal(err)
	}

	rep, err := svc.OpportunitiesReport(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	var cacheMissing, dmgMissing bool
	for _, o := range rep.Opportunities {
		switch o.Finding.Path {
		case cache:
			cacheMissing = o.Missing
		case filepath.Join(root, "tool.dmg"):
			dmgMissing = o.Missing
		}
	}
	if !cacheMissing {
		t.Error("removed cache must be marked Missing")
	}
	if dmgMissing {
		t.Error("an untouched file must not be marked Missing")
	}
	if rep.MissingCount != 1 {
		t.Errorf("MissingCount = %d, want 1", rep.MissingCount)
	}

	sav, err := svc.Savings(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if sav.MissingCount != 1 || sav.MissingBytes < 1_000_000 {
		t.Errorf("Savings missing = %d findings / %d bytes, want the 1 MB cache", sav.MissingCount, sav.MissingBytes)
	}
	if sav.Tiers[policy.SafeDelete] < 1_000_000 {
		t.Error("tier totals describe the index and still include the missing cache")
	}

	hot, err := svc.Hotspots(ctx, HotspotsQuery{Path: root, MinSize: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	if len(hot.MissingKnown) != 1 || hot.MissingKnown[0] != cache {
		t.Errorf("MissingKnown = %v, want [%s]", hot.MissingKnown, cache)
	}
}

func TestOpportunitiesReport_MinSizeAccountsForWhatItDrops(t *testing.T) {
	svc, root, cache := scannedCacheFixture(t)
	ctx := context.Background()

	all, err := svc.OpportunitiesReport(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if all.OmittedCount != 0 || all.MinSize != 0 {
		t.Errorf("no cut requested, got omitted=%d minSize=%d", all.OmittedCount, all.MinSize)
	}

	cut, err := svc.OpportunitiesReport(ctx, OpportunityQuery{Path: root, MinSize: 500_000})
	if err != nil {
		t.Fatal(err)
	}
	if cut.OmittedCount != len(all.Opportunities)-len(cut.Opportunities) || cut.OmittedCount == 0 {
		t.Errorf("OmittedCount = %d, want the %d findings dropped", cut.OmittedCount, len(all.Opportunities)-len(cut.Opportunities))
	}
	if cut.OmittedBytes <= 0 {
		t.Error("OmittedBytes should total the dropped findings")
	}
	if len(cut.Opportunities) != 1 || cut.Opportunities[0].Finding.Path != cache {
		t.Errorf("kept = %+v, want only the 1 MB cache", cut.Opportunities)
	}

	plain, err := svc.Opportunities(ctx, OpportunityQuery{Path: root, MinSize: 500_000})
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != len(cut.Opportunities) {
		t.Error("Opportunities and OpportunitiesReport must apply MinSize identically")
	}
}

func TestOpportunitiesReport_EmptyEncodesAsList(t *testing.T) {
	svc, root, _ := scannedCacheFixture(t)
	rep, err := svc.OpportunitiesReport(context.Background(), OpportunityQuery{Path: root, MinSize: 1 << 40})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Opportunities == nil {
		t.Error("an empty result must be [] not null, so consumers can iterate it")
	}
	if rep.Root != root {
		t.Errorf("Root = %q, want %q", rep.Root, root)
	}
}

// The *From variants exist so one analysis can feed several outputs;
// they must agree exactly with the standalone calls.
func TestFromVariantsMatchStandaloneCalls(t *testing.T) {
	svc, root, _ := scannedCacheFixture(t)
	ctx := context.Background()
	q := OpportunityQuery{Path: root, MinSize: 500_000}

	all, err := svc.Opportunities(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	gotRep, err := svc.OpportunitiesReportFrom(ctx, q, all)
	if err != nil {
		t.Fatal(err)
	}
	wantRep, err := svc.OpportunitiesReport(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotRep.Opportunities) != len(wantRep.Opportunities) || gotRep.OmittedCount != wantRep.OmittedCount || gotRep.OmittedBytes != wantRep.OmittedBytes {
		t.Errorf("ReportFrom = %+v, standalone = %+v", gotRep, wantRep)
	}

	gotSav, err := svc.SavingsFrom(ctx, root, all)
	if err != nil {
		t.Fatal(err)
	}
	wantSav, err := svc.Savings(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for tier, v := range wantSav.Tiers {
		if gotSav.Tiers[tier] != v {
			t.Errorf("tier %s: SavingsFrom=%d Savings=%d", tier, gotSav.Tiers[tier], v)
		}
	}
}
