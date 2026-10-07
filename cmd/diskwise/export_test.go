package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI runs the root command in-process and returns stdout.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diskwise %s: %v\nstderr: %s", strings.Join(args, " "), err, errOut.String())
	}
	return out.String()
}

func writeBytes(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExport_WritesRootedFilesAndReview(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "installer-1.0.dmg"), 200_000)
	writeBytes(t, filepath.Join(root, "Docs", "a.txt"), 100)
	db := filepath.Join(t.TempDir(), "index.db")
	outDir := filepath.Join(t.TempDir(), "out")

	runCLI(t, "scan", root, "--db", db)
	runCLI(t, "export", root, "--db", db, "--out", outDir, "--pairs")

	for _, name := range []string{"savings.json", "opportunities.json", "hotspots.json", "pairs.json", "review.md"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	// Every JSON file records the root it was computed for.
	for name, field := range map[string]string{"savings.json": "Path", "opportunities.json": "Root", "hotspots.json": "Root", "pairs.json": "Root"} {
		raw, err := os.ReadFile(filepath.Join(outDir, name)) //nolint:gosec // G304: path built from t.TempDir
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var got string
		if err := json.Unmarshal(doc[field], &got); err != nil || got != root {
			t.Errorf("%s: %s = %q (%v), want %q", name, field, got, err, root)
		}
	}

	review, err := os.ReadFile(filepath.Join(outDir, "review.md")) //nolint:gosec // G304: path built from t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(review), "# DiskWise review: "+root) || !strings.Contains(string(review), "- [ ]") {
		t.Errorf("review.md missing heading or checkbox:\n%s", review)
	}
}

func TestExport_RequiresOut(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"export", t.TempDir(), "--db", filepath.Join(t.TempDir(), "i.db")})
	if err := cmd.Execute(); err == nil {
		t.Error("export without --out should fail")
	}
}

func TestExportAndReview_Redact(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "clientname", "tool-1.0.dmg"), 200_000)
	db := filepath.Join(t.TempDir(), "index.db")
	outDir := filepath.Join(t.TempDir(), "out")
	runCLI(t, "scan", root, "--db", db)

	runCLI(t, "export", root, "--db", db, "--out", outDir, "--redact-prefix", filepath.Join(root, "clientname"))
	review := runCLI(t, "review", root, "--db", db, "--redact-prefix", filepath.Join(root, "clientname"))

	files, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	blobs := []string{review}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(outDir, f.Name())) //nolint:gosec // G304: path built from t.TempDir
		if err != nil {
			t.Fatal(err)
		}
		blobs = append(blobs, string(b))
	}
	for i, blob := range blobs {
		if strings.Contains(blob, "clientname") {
			t.Errorf("output %d still contains the redacted name", i)
		}
	}
}

func TestReview_UnknownFormat(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"review", t.TempDir(), "--format", "html", "--db", filepath.Join(t.TempDir(), "i.db")})
	if err := cmd.Execute(); err == nil {
		t.Error("review --format html should fail")
	}
}
