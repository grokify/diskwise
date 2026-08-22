package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grokify/diskwise/policy"
)

// TestArtifactFamilyDetector_ElasticsearchScenario reproduces the
// exact motivating example from the product ideation: five downloaded
// Elasticsearch distribution archives should be grouped into one
// finding with keep-newest and remove-all scenarios, not reported as
// five unrelated files.
func TestArtifactFamilyDetector_ElasticsearchScenario(t *testing.T) {
	root := t.TempDir()
	versions := []struct {
		name string
		age  time.Duration
	}{
		{"elasticsearch-9.1.3-darwin-aarch64.tar.gz", 1 * time.Hour},
		{"elasticsearch-9.1.2-darwin-aarch64.tar.gz", 24 * time.Hour},
		{"elasticsearch-9.0.4-darwin-aarch64.tar.gz", 48 * time.Hour},
		{"elasticsearch-8.18.3-darwin-aarch64.tar.gz", 72 * time.Hour},
		{"elasticsearch-8.17.6-darwin-aarch64.tar.gz", 96 * time.Hour},
	}
	now := time.Now()
	for _, v := range versions {
		path := filepath.Join(root, v.name)
		writeFile(t, path, 600_000)
		modTime := now.Add(-v.age)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	db := ingestFixture(t, root)
	d := ArtifactFamilyDetector{}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (one family)", len(findings))
	}
	f := findings[0]

	if len(f.Paths) != 5 {
		t.Errorf("Paths has %d entries, want 5", len(f.Paths))
	}
	// LogicalSize is exact regardless of filesystem block rounding;
	// AllocatedSize is only guaranteed to be >= LogicalSize.
	if f.LogicalSize != 5*600_000 {
		t.Errorf("LogicalSize = %d, want %d", f.LogicalSize, 5*600_000)
	}
	if f.AllocatedSize < f.LogicalSize {
		t.Errorf("AllocatedSize = %d, want >= LogicalSize (%d)", f.AllocatedSize, f.LogicalSize)
	}
	if f.ActionClass != policy.LikelySafe {
		t.Errorf("ActionClass = %s, want likely_safe", f.ActionClass)
	}

	if len(f.Scenarios) != 2 {
		t.Fatalf("got %d scenarios, want 2", len(f.Scenarios))
	}
	var keepNewest, removeAll *Scenario
	for i := range f.Scenarios {
		switch f.Scenarios[i].Name {
		case "keep-newest":
			keepNewest = &f.Scenarios[i]
		case "remove-all":
			removeAll = &f.Scenarios[i]
		}
	}
	if keepNewest == nil || removeAll == nil {
		t.Fatalf("missing a scenario: %+v", f.Scenarios)
	}
	if keepNewest.ReclaimableBytes <= 0 || keepNewest.ReclaimableBytes >= f.AllocatedSize {
		t.Errorf("keep-newest ReclaimableBytes = %d, want strictly between 0 and total AllocatedSize (%d)", keepNewest.ReclaimableBytes, f.AllocatedSize)
	}
	if len(keepNewest.Paths) != 4 {
		t.Errorf("keep-newest Paths has %d entries, want 4", len(keepNewest.Paths))
	}
	newestPath := filepath.Join(root, "elasticsearch-9.1.3-darwin-aarch64.tar.gz")
	for _, p := range keepNewest.Paths {
		if p == newestPath {
			t.Error("keep-newest scenario should not include the newest version among paths to remove")
		}
	}
	if removeAll.ReclaimableBytes != f.AllocatedSize {
		t.Errorf("remove-all ReclaimableBytes = %d, want %d (equal to the finding's total)", removeAll.ReclaimableBytes, f.AllocatedSize)
	}
	if len(removeAll.Paths) != 5 {
		t.Errorf("remove-all Paths has %d entries, want 5", len(removeAll.Paths))
	}
}

func TestArtifactFamilyDetector_SingleFileIsNotAFamily(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "elasticsearch-9.1.3-darwin-aarch64.tar.gz"), 600_000)

	db := ingestFixture(t, root)
	d := ArtifactFamilyDetector{}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 (a single file is not a family)", len(findings))
	}
}

func TestArtifactFamilyDetector_DifferentDirectoriesNotGrouped(t *testing.T) {
	root := t.TempDir()
	subA := filepath.Join(root, "a")
	subB := filepath.Join(root, "b")
	if err := os.MkdirAll(subA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subB, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subA, "tool-1.0.0.zip"), 100_000)
	writeFile(t, filepath.Join(subB, "tool-2.0.0.zip"), 100_000)

	db := ingestFixture(t, root)
	d := ArtifactFamilyDetector{}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 (same product name but different directories should not group)", len(findings))
	}
}

func TestArtifactFamilyDetector_BackupNameNeverPromoted(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "financial-export-1.0.zip"), 100_000)
	writeFile(t, filepath.Join(root, "financial-export-2.0.zip"), 100_000)

	db := ingestFixture(t, root)
	d := ArtifactFamilyDetector{}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].ActionClass == policy.SafeDelete || findings[0].ActionClass == policy.LikelySafe {
		t.Errorf("ActionClass = %s, want review despite matching the version-family pattern", findings[0].ActionClass)
	}
}

func TestArtifactFamilyDetector_ExcludesClaimedPaths(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cacheDir, "tool-1.0.0.zip"), 100_000)
	writeFile(t, filepath.Join(cacheDir, "tool-2.0.0.zip"), 100_000)

	db := ingestFixture(t, root)
	d := ArtifactFamilyDetector{Registry: fixtureRegistryFor(cacheDir)}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 (family is inside a claimed known location)", len(findings))
	}
}
