package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grokify/diskwise/platform"
	"github.com/grokify/diskwise/scan"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIngestScan_PersistsTreeAndRollups(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.txt"), 10_000)
	writeFile(t, filepath.Join(root, "sub", "b.txt"), 20_000)

	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}

	db := openTestDB(t)
	vol := platform.VolumeStats{MountPoint: "/", FSType: "apfs", CapacityBytes: 1_000_000_000, AvailableBytes: 500_000_000}
	run, err := db.IngestScan(context.Background(), tree, &vol)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "complete" {
		t.Errorf("run.Status = %q, want complete", run.Status)
	}
	if run.FinishedAt == nil {
		t.Fatal("run.FinishedAt is nil")
	}

	count, err := db.CountNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 { // root, a.txt, sub, sub/b.txt
		t.Errorf("CountNodes = %d, want 4", count)
	}

	rootRow, ok, err := db.NodeByPath(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("root not found")
	}
	if rootRow.Kind != KindDir {
		t.Errorf("root Kind = %q, want dir", rootRow.Kind)
	}
	if rootRow.SubtreeLogicalSize != 30_000 {
		t.Errorf("root SubtreeLogicalSize = %d, want 30000", rootRow.SubtreeLogicalSize)
	}
	if rootRow.State != StateComplete {
		t.Errorf("root State = %q, want complete", rootRow.State)
	}
	if rootRow.ParentID.Valid {
		t.Error("root ParentID should be NULL")
	}

	subRow, ok, err := db.NodeByPath(context.Background(), filepath.Join(root, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("sub not found")
	}
	if subRow.SubtreeLogicalSize != 20_000 {
		t.Errorf("sub SubtreeLogicalSize = %d, want 20000", subRow.SubtreeLogicalSize)
	}
	if !subRow.ParentID.Valid || subRow.ParentID.Int64 != rootRow.ID {
		t.Errorf("sub ParentID = %v, want %d", subRow.ParentID, rootRow.ID)
	}
}

func TestChildren_OrderedBySizeDescending(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "small.txt"), 1_000)
	writeFile(t, filepath.Join(root, "big.txt"), 100_000)

	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}

	db := openTestDB(t)
	if _, err := db.IngestScan(context.Background(), tree, nil); err != nil {
		t.Fatal(err)
	}

	children, err := db.Children(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Fatalf("got %d children, want 2", len(children))
	}
	if children[0].Name != "big.txt" || children[1].Name != "small.txt" {
		t.Errorf("order = [%s, %s], want [big.txt, small.txt]", children[0].Name, children[1].Name)
	}
}

func TestLargest_ExcludesRootAndFiltersByKind(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bigdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "bigdir", "inner.txt"), 200_000)
	writeFile(t, filepath.Join(root, "small.txt"), 1_000)
	writeFile(t, filepath.Join(root, "medium.txt"), 50_000)

	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t)
	if _, err := db.IngestScan(context.Background(), tree, nil); err != nil {
		t.Fatal(err)
	}

	all, err := db.Largest(context.Background(), root, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range all {
		if n.Path == root {
			t.Error("Largest should exclude the root path itself")
		}
	}
	if len(all) != 4 { // bigdir, bigdir/inner.txt, small.txt, medium.txt
		t.Fatalf("got %d results, want 4", len(all))
	}
	if all[0].Name != "bigdir" {
		t.Errorf("largest overall = %s, want bigdir (subtree 200000 > any single file)", all[0].Name)
	}

	filesOnly, err := db.Largest(context.Background(), root, 10, KindFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(filesOnly) != 3 {
		t.Fatalf("got %d files, want 3", len(filesOnly))
	}
	if filesOnly[0].Name != "inner.txt" {
		t.Errorf("largest file = %s, want inner.txt", filesOnly[0].Name)
	}

	limited, err := db.Largest(context.Background(), root, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 {
		t.Fatalf("got %d results, want 1 (limit)", len(limited))
	}
}

func TestFilesByExtensions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "archive.tar.gz"), 10_000)
	writeFile(t, filepath.Join(root, "installer.dmg"), 5_000)
	writeFile(t, filepath.Join(root, "notes.txt"), 100)

	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t)
	if _, err := db.IngestScan(context.Background(), tree, nil); err != nil {
		t.Fatal(err)
	}

	// "archive.tar.gz" is stored with ext=".gz" (filepath.Ext only
	// captures the final component), so querying by ".gz" and ".dmg"
	// should surface both archive files, not the plain .txt file.
	rows, err := db.FilesByExtensions(context.Background(), root, []string{".gz", ".dmg"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	names := map[string]bool{rows[0].Name: true, rows[1].Name: true}
	if !names["archive.tar.gz"] || !names["installer.dmg"] {
		t.Errorf("names = %v, want archive.tar.gz and installer.dmg", names)
	}

	none, err := db.FilesByExtensions(context.Background(), root, []string{".zip"})
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("got %d rows for unmatched extension, want 0", len(none))
	}

	empty, err := db.FilesByExtensions(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("got %d rows for empty extension list, want 0", len(empty))
	}
}

// TestRescan_PropagatesDeltaToAncestors verifies the central rescan
// contract: growing a leaf subtree after manual changes updates every
// ancestor's rollup, not just the rescanned node itself.
func TestRescan_PropagatesDeltaToAncestors(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	leaf := filepath.Join(sub, "leaf")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(leaf, "f.txt"), 10_000)

	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t)
	if _, err := db.IngestScan(context.Background(), tree, nil); err != nil {
		t.Fatal(err)
	}

	rootBefore, _, err := db.NodeByPath(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if rootBefore.SubtreeLogicalSize != 10_000 {
		t.Fatalf("precondition failed: root SubtreeLogicalSize = %d, want 10000", rootBefore.SubtreeLogicalSize)
	}

	// Simulate the user adding a file under leaf, then rescanning just that subtree.
	writeFile(t, filepath.Join(leaf, "g.txt"), 5_000)

	rescannedTree, _, _, err := db.Rescan(context.Background(), leaf, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rescannedTree.SubtreeLogicalSize != 15_000 {
		t.Errorf("rescanned leaf SubtreeLogicalSize = %d, want 15000", rescannedTree.SubtreeLogicalSize)
	}

	subAfter, _, err := db.NodeByPath(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if subAfter.SubtreeLogicalSize != 15_000 {
		t.Errorf("sub SubtreeLogicalSize after rescan = %d, want 15000", subAfter.SubtreeLogicalSize)
	}

	rootAfter, _, err := db.NodeByPath(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if rootAfter.SubtreeLogicalSize != 15_000 {
		t.Errorf("root SubtreeLogicalSize after rescan = %d, want 15000", rootAfter.SubtreeLogicalSize)
	}

	// The leaf's own row should have been replaced (new id), and the
	// old row's children fully removed and reinserted, not duplicated.
	leafRow, ok, err := db.NodeByPath(context.Background(), leaf)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("leaf not found after rescan")
	}
	children, err := db.Children(context.Background(), leaf)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Errorf("leaf has %d children after rescan, want 2 (f.txt, g.txt)", len(children))
	}
	_ = leafRow
}

// TestRescan_ShrinkingSubtreePropagatesNegativeDelta verifies deletion
// (the product's actual use case: user deletes files, then rescans to
// confirm savings) correctly reduces ancestor totals rather than only
// ever growing them.
func TestRescan_ShrinkingSubtreePropagatesNegativeDelta(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(sub, "big.txt")
	writeFile(t, big, 50_000)
	writeFile(t, filepath.Join(root, "keep.txt"), 1_000)

	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t)
	if _, err := db.IngestScan(context.Background(), tree, nil); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(big); err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := db.Rescan(context.Background(), sub, scan.Options{}); err != nil {
		t.Fatal(err)
	}

	rootAfter, _, err := db.NodeByPath(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if rootAfter.SubtreeLogicalSize != 1_000 {
		t.Errorf("root SubtreeLogicalSize after deletion+rescan = %d, want 1000", rootAfter.SubtreeLogicalSize)
	}
}

func TestRescan_UnindexedPathIngestsFresh(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "f.txt"), 2_000)

	db := openTestDB(t)
	tree, _, _, err := db.Rescan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if tree.SubtreeLogicalSize != 2_000 {
		t.Errorf("SubtreeLogicalSize = %d, want 2000", tree.SubtreeLogicalSize)
	}

	row, ok, err := db.NodeByPath(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("root not indexed after rescan of an unindexed path")
	}
	if row.ParentID.Valid {
		t.Error("freshly ingested path via Rescan should have no parent")
	}
}

func TestScanRunByID(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "f.txt"), 500)

	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t)
	run, err := db.IngestScan(context.Background(), tree, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.ScanRunByID(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != root {
		t.Errorf("Root = %q, want %q", got.Root, root)
	}
	if got.Status != "complete" {
		t.Errorf("Status = %q, want complete", got.Status)
	}
	if got.FinishedAt == nil {
		t.Fatal("FinishedAt is nil")
	}
	if time.Since(*got.FinishedAt) > time.Minute {
		t.Errorf("FinishedAt = %v, looks stale", got.FinishedAt)
	}
}
