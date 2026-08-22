package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grokify/diskwise/policy"
)

func TestArchiveInstallerDetector_ExtractionSibling(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "foo.tar.gz"), 10_000)
	if err := os.MkdirAll(filepath.Join(root, "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "foo", "content.txt"), 500)

	db := ingestFixture(t, root)
	d := ArchiveInstallerDetector{ApplicationsDir: filepath.Join(root, "NoApplicationsHere")}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.ActionClass != policy.LikelySafe {
		t.Errorf("ActionClass = %s, want likely_safe", f.ActionClass)
	}
	if f.Confidence <= 0 {
		t.Errorf("Confidence = %v, want > 0", f.Confidence)
	}
}

func TestArchiveInstallerDetector_NoEvidence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "mystery.zip"), 10_000)

	db := ingestFixture(t, root)
	d := ArchiveInstallerDetector{ApplicationsDir: filepath.Join(root, "NoApplicationsHere")}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].ActionClass != policy.Review {
		t.Errorf("ActionClass = %s, want review (no evidence)", findings[0].ActionClass)
	}
}

// TestArchiveInstallerDetector_InstalledApp verifies the DMG
// installed-app evidence path using an injected fake Applications
// directory (a real /Applications dependency would make this test
// environment-dependent).
func TestArchiveInstallerDetector_InstalledApp(t *testing.T) {
	root := t.TempDir()
	dmgPath := filepath.Join(root, "SuperTool-1.2.3.dmg")
	writeFile(t, dmgPath, 50_000)
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(dmgPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	appsDir := filepath.Join(root, "Applications")
	appBundle := filepath.Join(appsDir, "SuperTool.app")
	if err := os.MkdirAll(appBundle, 0o755); err != nil {
		t.Fatal(err)
	}
	// appBundle's mtime (just created) is newer than the DMG's.

	db := ingestFixture(t, root)
	d := ArchiveInstallerDetector{ApplicationsDir: appsDir}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.ActionClass != policy.SafeDelete {
		t.Errorf("ActionClass = %s, want safe_delete (newer installed app match)", f.ActionClass)
	}
	if f.Confidence < policy.MinConfidenceForSafeDelete {
		t.Errorf("Confidence = %v, want >= %v to survive policy.Evaluate", f.Confidence, policy.MinConfidenceForSafeDelete)
	}
}

func TestArchiveInstallerDetector_BackupNameNeverPromoted(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "tax-documents-backup.zip")
	writeFile(t, archivePath, 10_000)
	// Give it an extraction sibling too — strong evidence that should
	// normally promote it, but the backup-suggestive name must win.
	if err := os.MkdirAll(filepath.Join(root, "tax-documents-backup"), 0o755); err != nil {
		t.Fatal(err)
	}

	db := ingestFixture(t, root)
	d := ArchiveInstallerDetector{ApplicationsDir: filepath.Join(root, "NoApplicationsHere")}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].ActionClass == policy.SafeDelete || findings[0].ActionClass == policy.LikelySafe {
		t.Errorf("ActionClass = %s, want review or more conservative despite extraction-sibling evidence", findings[0].ActionClass)
	}
}

func TestArchiveInstallerDetector_DefersFamilyMembersToArtifactFamilyDetector(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "elasticsearch-9.1.3-darwin-aarch64.tar.gz"), 10_000)
	writeFile(t, filepath.Join(root, "elasticsearch-9.1.2-darwin-aarch64.tar.gz"), 10_000)
	writeFile(t, filepath.Join(root, "standalone.zip"), 5_000)

	db := ingestFixture(t, root)
	d := ArchiveInstallerDetector{ApplicationsDir: filepath.Join(root, "NoApplicationsHere")}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (only standalone.zip; the elasticsearch pair belongs to artifact-family)", len(findings))
	}
	if findings[0].Path != filepath.Join(root, "standalone.zip") {
		t.Errorf("Path = %s, want standalone.zip", findings[0].Path)
	}
}

func TestArchiveInstallerDetector_ExcludesClaimedPaths(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "AppCache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cacheDir, "download.zip"), 10_000)

	db := ingestFixture(t, root)
	registry := fixtureRegistryFor(cacheDir)
	d := ArchiveInstallerDetector{Registry: registry, ApplicationsDir: filepath.Join(root, "NoApplicationsHere")}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 (download.zip is inside a claimed known location)", len(findings))
	}
}
