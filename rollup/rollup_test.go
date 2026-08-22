package rollup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/scan"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ingestFixture(t *testing.T, root string) *index.DB {
	t.Helper()
	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db, err := index.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.IngestScan(context.Background(), tree, nil); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCollapse_TrivialCases(t *testing.T) {
	got, err := Collapse(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}

	got, err = Collapse(context.Background(), nil, []string{"/a/b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "/a/b" {
		t.Errorf("got %v, want [/a/b] (single path returned unchanged)", got)
	}
}

func TestCollapse_FullyCoveredDirectoryCollapses(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a.bin"), 1000)
	writeFile(t, filepath.Join(dir, "b.bin"), 2000)
	writeFile(t, filepath.Join(root, "sibling.txt"), 500) // uncovered, so collapse can't cascade past dir

	db := ingestFixture(t, root)

	got, err := Collapse(context.Background(), db, []string{
		filepath.Join(dir, "a.bin"),
		filepath.Join(dir, "b.bin"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != dir {
		t.Errorf("got %v, want [%s] (both children covered, should collapse to the directory)", got, dir)
	}
}

func TestCollapse_MixedDirectoryStaysDecomposed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "downloads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "old.zip"), 1000)
	writeFile(t, filepath.Join(dir, "keep.txt"), 2000) // not in the covered set

	db := ingestFixture(t, root)

	got, err := Collapse(context.Background(), db, []string{filepath.Join(dir, "old.zip")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != filepath.Join(dir, "old.zip") {
		t.Errorf("got %v, want the single file path unchanged (directory is only partially covered)", got)
	}
}

// TestCollapse_MultiLevelCollapse verifies collapsing cascades
// upward: once a subdirectory fully collapses, its parent might now
// also be fully covered and should collapse too, in the same call.
func TestCollapse_MultiLevelCollapse(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "DerivedData")
	child := filepath.Join(parent, "Build")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(child, "a.o"), 1000)
	writeFile(t, filepath.Join(child, "b.o"), 1000)
	writeFile(t, filepath.Join(parent, "Logs.txt"), 500)
	writeFile(t, filepath.Join(root, "sibling.txt"), 500) // uncovered, so collapse can't cascade past parent

	db := ingestFixture(t, root)

	got, err := Collapse(context.Background(), db, []string{
		filepath.Join(child, "a.o"),
		filepath.Join(child, "b.o"),
		filepath.Join(parent, "Logs.txt"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != parent {
		t.Errorf("got %v, want [%s] (child fully collapses, which then fully covers parent)", got, parent)
	}
}

func TestCollapse_StopsAtScannedRootBoundary(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.bin"), 1000)
	writeFile(t, filepath.Join(root, "b.bin"), 2000)

	db := ingestFixture(t, root)

	// root's own parent (filepath.Dir(root)) is not indexed, so
	// collapsing must stop at root itself.
	got, err := Collapse(context.Background(), db, []string{
		filepath.Join(root, "a.bin"),
		filepath.Join(root, "b.bin"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != root {
		t.Errorf("got %v, want [%s]", got, root)
	}
}
