package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db)
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestService_ScanThenSummary(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.txt"), 10_000)
	writeFile(t, filepath.Join(root, "sub", "b.txt"), 5_000)

	svc := newTestService(t)
	ctx := context.Background()

	scanResult, err := svc.Scan(ctx, ScanRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if scanResult.Run.Status != "complete" {
		t.Errorf("Run.Status = %q, want complete", scanResult.Run.Status)
	}
	if scanResult.Stats.DirsScanned != 2 {
		t.Errorf("Stats.DirsScanned = %d, want 2", scanResult.Stats.DirsScanned)
	}

	summary, err := svc.Summary(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.LogicalSize != 15_000 {
		t.Errorf("LogicalSize = %d, want 15000", summary.LogicalSize)
	}
	if summary.State != index.StateComplete {
		t.Errorf("State = %q, want complete", summary.State)
	}
	if summary.ScanRun.ID != scanResult.Run.ID {
		t.Errorf("Summary ScanRun.ID = %d, want %d", summary.ScanRun.ID, scanResult.Run.ID)
	}
}

func TestService_Summary_NotIndexed(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Summary(context.Background(), "/nowhere")
	if err == nil {
		t.Fatal("expected an error for an unindexed path")
	}
}

func TestService_Tree_ExpandsToDepthAndFiltersMinSize(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "small.txt"), 100)
	writeFile(t, filepath.Join(root, "big.txt"), 100_000)
	writeFile(t, filepath.Join(root, "sub", "deep", "leaf.txt"), 1_000)

	svc := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	tree, err := svc.Tree(ctx, TreeQuery{Path: root, Depth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Children) != 3 { // small.txt, big.txt, sub
		t.Fatalf("got %d children at depth 2, want 3", len(tree.Children))
	}
	if tree.Children[0].Name != "big.txt" {
		t.Errorf("first child = %s, want big.txt (largest first)", tree.Children[0].Name)
	}

	var sub *TreeNode
	for i := range tree.Children {
		if tree.Children[i].Name == "sub" {
			sub = &tree.Children[i]
		}
	}
	if sub == nil {
		t.Fatal("sub not found")
	}
	if len(sub.Children) != 1 || sub.Children[0].Name != "deep" {
		t.Errorf("sub.Children = %v, want [deep]", sub.Children)
	}
	if len(sub.Children[0].Children) != 0 {
		t.Error("depth 2 from root should not expand grandchildren's own children (deep/leaf.txt)")
	}
}

// TestService_Tree_MinSizeFilter uses two files of drastically
// different sizes (1 byte vs 5 MB) so the assertion holds regardless
// of filesystem block-rounding or small-file inline-storage behavior:
// with the threshold set at the midpoint of their actual allocated
// sizes, the filter must keep exactly the larger one.
func TestService_Tree_MinSizeFilter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tiny.txt"), 1)
	writeFile(t, filepath.Join(root, "huge.txt"), 5_000_000)

	svc := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	unfiltered, err := svc.Tree(ctx, TreeQuery{Path: root, Depth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(unfiltered.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(unfiltered.Children))
	}
	var tiny, huge TreeNode
	for _, c := range unfiltered.Children {
		if c.Name == "tiny.txt" {
			tiny = c
		} else {
			huge = c
		}
	}
	threshold := (tiny.AllocatedSize + huge.AllocatedSize) / 2

	filtered, err := svc.Tree(ctx, TreeQuery{Path: root, Depth: 1, MinSize: threshold})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Children) != 1 || filtered.Children[0].Name != "huge.txt" {
		t.Errorf("filtered children = %v, want just huge.txt", filtered.Children)
	}
}

func TestService_Largest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bigdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "bigdir", "inner.txt"), 200_000)
	writeFile(t, filepath.Join(root, "small.txt"), 1_000)

	svc := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	items, err := svc.Largest(ctx, LargestQuery{Path: root, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Name != "bigdir" {
		t.Errorf("largest = %s, want bigdir", items[0].Name)
	}
}

func TestService_Rescan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "f.txt"), 1_000)

	svc := newTestService(t)
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(root, "g.txt"), 4_000)

	result, err := svc.Rescan(ctx, RescanRequest{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Status != "complete" {
		t.Errorf("Run.Status = %q, want complete", result.Run.Status)
	}

	summary, err := svc.Summary(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.LogicalSize != 5_000 {
		t.Errorf("LogicalSize after rescan = %d, want 5000", summary.LogicalSize)
	}
}

// fixtureRegistry returns a registry pointing at hermetic fixture
// paths, so Hotspots/Summary tests don't depend on the real machine's
// installed apps (knowledge.Default is environment-dependent).
func fixtureRegistry(cachePath string) knowledge.Registry {
	return knowledge.Registry{
		{
			ID:                 "fixture-cache",
			Description:        "Fixture cache",
			DataPaths:          []string{cachePath},
			Entity:             entity.KindCache,
			Semantics:          knowledge.SemanticsRegeneratableCache,
			DefaultActionClass: policy.SafeDelete,
		},
	}
}

func TestService_Hotspots(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "AppCache")
	mystery := filepath.Join(root, "Mystery")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mystery, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cache, "f.bin"), 5_000_000)
	writeFile(t, filepath.Join(mystery, "f.bin"), 5_000_000)

	svc := newTestService(t).WithRegistry(fixtureRegistry(cache))
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	report, err := svc.Hotspots(ctx, HotspotsQuery{Path: root, MinSize: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Known) != 1 || report.Known[0].Path != cache {
		t.Errorf("Known = %v, want one finding for %s", report.Known, cache)
	}

	var foundMystery bool
	for _, f := range report.Unexplained {
		if f.Path == mystery {
			foundMystery = true
		}
		if f.Path == cache {
			t.Error("Unexplained should not include the claimed cache directory")
		}
	}
	if !foundMystery {
		t.Errorf("Unexplained = %v, want it to include %s", report.Unexplained, mystery)
	}
}

func TestService_Hotspots_NotIndexed(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Hotspots(context.Background(), HotspotsQuery{Path: "/nowhere"})
	if err == nil {
		t.Fatal("expected an error for an unindexed path")
	}
}

func TestService_Summary_IncludesKnownLocations(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cache, "f.bin"), 1_000)

	svc := newTestService(t).WithRegistry(fixtureRegistry(cache))
	ctx := context.Background()
	if _, err := svc.Scan(ctx, ScanRequest{Root: root}); err != nil {
		t.Fatal(err)
	}

	summary, err := svc.Summary(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.KnownLocations) != 1 || summary.KnownLocations[0].Path != cache {
		t.Errorf("KnownLocations = %v, want one finding for %s", summary.KnownLocations, cache)
	}
}
