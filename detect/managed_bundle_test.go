package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

func TestManagedBundleDetector_ReportsOutermostBundleAsKeep(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "Pictures", "Photos Library.photoslibrary")
	inner := filepath.Join(lib, "inner.migratedphotolibrary")
	for _, d := range []string{filepath.Join(lib, "originals"), inner} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(lib, "originals", "a.jpeg"), 400_000)
	writeFile(t, filepath.Join(inner, "b.jpeg"), 100_000)
	writeFile(t, filepath.Join(root, "Pictures", "plain.bin"), 1000)
	db := ingestFixture(t, root)

	findings, err := ManagedBundleDetector{}.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (the nested bundle is part of the outer one)", len(findings))
	}
	f := findings[0]
	if f.Path != lib || f.Entity.Kind != entity.KindManagedBundle {
		t.Errorf("finding = %s / %s, want %s / managed_bundle", f.Path, f.Entity.Kind, lib)
	}
	if f.ActionClass != policy.Keep {
		t.Errorf("ActionClass = %s, want keep", f.ActionClass)
	}
	if f.AllocatedSize < 500_000 {
		t.Errorf("AllocatedSize = %d, want the whole bundle (>= 500000)", f.AllocatedSize)
	}
}

func TestPolicy_ManagedBundleCannotBePromoted(t *testing.T) {
	if got := policy.Evaluate(policy.SafeDelete, entity.KindManagedBundle, 1); got == policy.SafeDelete || got == policy.LikelySafe {
		t.Errorf("a managed bundle must never be promoted to %s", got)
	}
}

// Archives and large-directory detection must treat a bundle's insides
// as claimed, or the same bytes would be reported twice.
func TestDetectors_SkipInsideManagedBundle(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "Photos Library.photoslibrary", "resources")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(lib, "export.zip"), 3_000_000)
	db := ingestFixture(t, root)
	ctx := context.Background()

	arch, err := ArchiveInstallerDetector{ApplicationsDir: t.TempDir()}.Detect(ctx, db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(arch) != 0 {
		t.Errorf("archive detector reported %d finding(s) inside a managed bundle", len(arch))
	}
	big, err := LargeUnexplainedDetector{MinSize: 1_000_000}.Detect(ctx, db, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range big {
		if f.Path == lib {
			t.Errorf("unexplained detector reported %s inside a managed bundle", f.Path)
		}
	}
}

func TestManagedBundleDetector_DefersToKnownLocations(t *testing.T) {
	root := t.TempDir()
	claimedDir := filepath.Join(root, "Simulator")
	lib := filepath.Join(claimedDir, "Syndication.photoslibrary")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(lib, "a.bin"), 100_000)
	db := ingestFixture(t, root)

	registry := knowledge.Registry{
		{ID: "sim", Description: "Simulator", DataPaths: []string{claimedDir}, DefaultActionClass: policy.Review},
	}
	findings, err := ManagedBundleDetector{Registry: registry}.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("bundle inside a known location reported separately: %+v", findings)
	}
}
