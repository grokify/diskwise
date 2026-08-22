package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/grokify/diskwise/policy"
)

// TestService_Opportunities_NoDoubleCounting is the key correctness
// property for `savings`: a fixture exercising known-location,
// artifact-family, and archive-installer detectors together (the
// three whose territory could plausibly overlap) must never report
// the same indexed path in more than one finding's Paths.
// LargeUnexplainedDetector is excluded from this fixture — it only
// ever reports directories, which can't collide with these file-level
// findings, and its claimed-path exclusion is already covered by
// dedicated detect package tests.
func TestService_Opportunities_NoDoubleCounting(t *testing.T) {
	root := t.TempDir()

	cache := filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cache, "data.bin"), 1_000_000)

	writeFile(t, filepath.Join(root, "tool-1.0.0.zip"), 100_000)
	writeFile(t, filepath.Join(root, "tool-2.0.0.zip"), 100_000)

	writeFile(t, filepath.Join(root, "standalone.dmg"), 50_000)

	svc := newTestService(t).WithRegistry(fixtureRegistry(cache))
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	opps, err := svc.Opportunities(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(opps) == 0 {
		t.Fatal("expected at least one opportunity")
	}

	seen := make(map[string]string) // path -> which finding's entity ID first claimed it
	for _, o := range opps {
		for _, p := range o.Finding.Paths {
			if owner, dup := seen[p]; dup {
				t.Errorf("path %s reported by both %q and %q", p, owner, o.Finding.Entity.ID)
			}
			seen[p] = o.Finding.Entity.ID
		}
	}

	// Sanity: the known location, the family, and the standalone
	// archive should each be represented exactly once.
	var haveCache, haveFamily, haveStandalone bool
	for _, o := range opps {
		switch o.Finding.Path {
		case cache:
			haveCache = true
		case root: // artifact family's Path is its containing directory
			haveFamily = true
		case filepath.Join(root, "standalone.dmg"):
			haveStandalone = true
		}
	}
	if !haveCache {
		t.Error("missing known-location finding for the cache directory")
	}
	if !haveFamily {
		t.Error("missing artifact-family finding for the tool-*.zip versions")
	}
	if !haveStandalone {
		t.Error("missing archive-installer finding for standalone.dmg")
	}
}

func TestService_Opportunities_FiltersByActionClassAndConfidence(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cache, "data.bin"), 1_000_000)
	writeFile(t, filepath.Join(root, "mystery.zip"), 50_000) // no evidence -> review, confidence 0.3

	svc := newTestService(t).WithRegistry(fixtureRegistry(cache))
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	safeOnly, err := svc.Opportunities(ctx, OpportunityQuery{Path: root, ActionClass: policy.SafeDelete})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range safeOnly {
		if o.Finding.ActionClass != policy.SafeDelete {
			t.Errorf("ActionClass filter leaked a %s finding", o.Finding.ActionClass)
		}
	}
	if len(safeOnly) == 0 {
		t.Error("expected at least the cache finding under ActionClass=safe_delete")
	}

	highConfidence, err := svc.Opportunities(ctx, OpportunityQuery{Path: root, MinConfidence: 0.99})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range highConfidence {
		if o.Finding.Confidence < 0.99 {
			t.Errorf("MinConfidence filter leaked a finding with confidence %v", o.Finding.Confidence)
		}
	}
}

func TestService_Opportunities_NotIndexed(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Opportunities(context.Background(), OpportunityQuery{Path: "/nowhere"})
	if err == nil {
		t.Fatal("expected an error for an unindexed path")
	}
}

func TestService_Opportunities_RollsUpToActionablePaths(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cache, "a.bin"), 500_000)
	writeFile(t, filepath.Join(cache, "b.bin"), 500_000)

	svc := newTestService(t).WithRegistry(fixtureRegistry(cache))
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	opps, err := svc.Opportunities(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	var cacheOpp *Opportunity
	for i := range opps {
		if opps[i].Finding.Path == cache {
			cacheOpp = &opps[i]
		}
	}
	if cacheOpp == nil {
		t.Fatal("no opportunity for the cache directory")
	}
	if len(cacheOpp.Paths) != 1 || cacheOpp.Paths[0] != cache {
		t.Errorf("Paths = %v, want [%s] (single-path finding, trivially the directory itself)", cacheOpp.Paths, cache)
	}
}

func TestService_Savings_SumsMatchOpportunities(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cache, "data.bin"), 1_000_000)

	svc := newTestService(t).WithRegistry(fixtureRegistry(cache))
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	savings, err := svc.Savings(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	opps, err := svc.Opportunities(ctx, OpportunityQuery{Path: root})
	if err != nil {
		t.Fatal(err)
	}

	var wantTotal int64
	for _, o := range opps {
		wantTotal += o.Finding.AllocatedSize
	}
	var gotTotal int64
	for _, v := range savings.Tiers {
		gotTotal += v
	}
	if gotTotal != wantTotal {
		t.Errorf("Savings total = %d, want %d (sum of Opportunities)", gotTotal, wantTotal)
	}
	if savings.Tiers[policy.SafeDelete] <= 0 {
		t.Error("expected a non-zero safe_delete tier for the cache finding")
	}
}
