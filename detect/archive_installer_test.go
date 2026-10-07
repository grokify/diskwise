package detect

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// A same-named directory is only a hint. For a readable tar the
// detector checks the archive against it, so an archive holding files
// the directory lacks is not called likely-safe on its name alone.
func TestArchiveInstallerDetector_ChecksExtractedSiblingAgainstArchive(t *testing.T) {
	root := t.TempDir()
	same := []member{{"Same/a", 1000}, {"Same/b", 2000}}
	writeTree(t, root, same)
	writeTar(t, filepath.Join(root, "Same.tar"), false, same)

	writeTree(t, root, []member{{"More/a", 1000}})
	writeTar(t, filepath.Join(root, "More.tar"), false, []member{{"More/a", 1000}, {"More/b", 1000}, {"More/c", 1000}})

	// Unreadable archive beside a directory: falls back to the name hint.
	writeTree(t, root, []member{{"Odd/a", 10}})
	writeFile(t, filepath.Join(root, "Odd.zip"), 500)

	db := ingestFixture(t, root)
	findings, err := ArchiveInstallerDetector{ApplicationsDir: t.TempDir()}.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Finding{}
	for _, f := range findings {
		byName[filepath.Base(f.Path)] = f
	}

	if f := byName["Same.tar"]; f.ActionClass != policy.LikelySafe || !strings.Contains(f.Reason, "same-named directory holds 2 file(s)") {
		t.Errorf("Same.tar = %s / %q, want likely_safe citing the matching file count", f.ActionClass, f.Reason)
	}
	if f := byName["More.tar"]; f.ActionClass != policy.Review || !strings.Contains(f.Reason, "lists 2 more file(s)") {
		t.Errorf("More.tar = %s / %q, want review explaining the 2 extra files", f.ActionClass, f.Reason)
	}
	if f := byName["Odd.zip"]; f.ActionClass != policy.LikelySafe || !strings.Contains(f.Reason, "appears already extracted") {
		t.Errorf("Odd.zip = %s / %q, want the name-only likely_safe fallback", f.ActionClass, f.Reason)
	}
}
