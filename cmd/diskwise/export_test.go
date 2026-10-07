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
	out, _ := runCLIBoth(t, args...)
	return out
}

// runCLIBoth runs the root command in-process and returns stdout and stderr.
func runCLIBoth(t *testing.T, args ...string) (stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diskwise %s: %v\nstderr: %s", strings.Join(args, " "), err, errOut.String())
	}
	return out.String(), errOut.String()
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
	runCLI(t, "export", root, "--db", db, "--out", outDir, "--pairs", "--min-size", "0")

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

// opportunities --json and the export file are one shape: the report
// envelope, with the root, when it was measured, and the findings.
func TestOpportunitiesJSON_IsTheExportShape(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "installer-1.0.dmg"), 200_000)
	db := filepath.Join(t.TempDir(), "index.db")
	outDir := filepath.Join(t.TempDir(), "out")
	runCLI(t, "scan", root, "--db", db)

	direct := runCLI(t, "opportunities", root, "--db", db, "--json")
	runCLI(t, "export", root, "--db", db, "--out", outDir, "--min-size", "0")
	exported, err := os.ReadFile(filepath.Join(outDir, "opportunities.json")) //nolint:gosec // G304: path built from t.TempDir
	if err != nil {
		t.Fatal(err)
	}

	keys := func(raw []byte) map[string]json.RawMessage {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("not a JSON object (a bare array is the old shape): %v\n%s", err, raw)
		}
		return m
	}
	d, e := keys([]byte(direct)), keys(exported)
	for _, k := range []string{"Root", "ScannedAt", "ScanStatus", "Stale", "Opportunities"} {
		if _, ok := d[k]; !ok {
			t.Errorf("opportunities --json is missing %q", k)
		}
		if _, ok := e[k]; !ok {
			t.Errorf("export opportunities.json is missing %q", k)
		}
	}
	if string(d["Opportunities"]) != string(e["Opportunities"]) {
		t.Error("the findings differ between opportunities --json and export")
	}
}

func TestReview_MinSizeDefaultStatesWhatItOmits(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "small-1.0.dmg"), 200_000)
	db := filepath.Join(t.TempDir(), "index.db")
	runCLI(t, "scan", root, "--db", db)

	def := runCLI(t, "review", root, "--db", db)
	if !strings.Contains(def, "smaller finding(s)") || !strings.Contains(def, "--min-size 0") {
		t.Errorf("default review must say what it omitted and how to see it:\n%s", def)
	}
	if strings.Contains(def, "small-1.0.dmg") {
		t.Error("a 200 KB finding should be under the 1mb default")
	}
	all := runCLI(t, "review", root, "--db", db, "--min-size", "0")
	if !strings.Contains(all, "small-1.0.dmg") || strings.Contains(all, "omitted") {
		t.Errorf("--min-size 0 must list everything and omit nothing:\n%s", all)
	}
}

// A path removed after the scan must be called out, on stderr so JSON
// output stays parseable.
func TestCommands_WarnWhenIndexedPathsNoLongerExist(t *testing.T) {
	root := t.TempDir()
	dmg := filepath.Join(root, "installer-1.0.dmg")
	writeBytes(t, dmg, 200_000)
	db := filepath.Join(t.TempDir(), "index.db")
	runCLI(t, "scan", root, "--db", db)
	if err := os.Remove(dmg); err != nil {
		t.Fatal(err)
	}

	out, errOut := runCLIBoth(t, "opportunities", root, "--db", db, "--json")
	if !strings.Contains(errOut, "no longer exist") {
		t.Errorf("expected a missing-path warning on stderr, got %q", errOut)
	}
	var rep struct {
		MissingCount  int
		Opportunities []struct{ Missing bool }
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stdout must stay valid JSON: %v\n%s", err, out)
	}
	if rep.MissingCount != 1 || len(rep.Opportunities) != 1 || !rep.Opportunities[0].Missing {
		t.Errorf("report = %+v, want 1 finding marked Missing", rep)
	}

	text, _ := runCLIBoth(t, "opportunities", root, "--db", db)
	if !strings.Contains(text, "[missing]") || !strings.Contains(text, "Measured ") {
		t.Errorf("text output should mark the missing finding and show when it was measured:\n%s", text)
	}
	if _, errOut := runCLIBoth(t, "savings", root, "--db", db); !strings.Contains(errOut, "no longer exist") {
		t.Errorf("savings should warn too, got %q", errOut)
	}
}
