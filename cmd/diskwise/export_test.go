package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/grokify/diskwise/reportdoc"
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
	out, errOut, err := runCLIErr(args...)
	if err != nil {
		t.Fatalf("diskwise %s: %v\nstderr: %s", strings.Join(args, " "), err, errOut)
	}
	return out, errOut
}

// runCLIErr runs the root command in-process, returning its error.
func runCLIErr(args ...string) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
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

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: path built from t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// scannedFixture scans a small tree and returns its root, index path
// and a fresh output directory.
func scannedFixture(t *testing.T) (root, db, outDir string) {
	t.Helper()
	root = t.TempDir()
	writeBytes(t, filepath.Join(root, "installer-1.0.dmg"), 200_000)
	writeBytes(t, filepath.Join(root, "Docs", "a.txt"), 100)
	db = filepath.Join(t.TempDir(), "index.db")
	outDir = filepath.Join(t.TempDir(), "out")
	runCLI(t, "scan", root, "--db", db)
	return root, db, outDir
}

func TestExport_WritesTheDocumentAndItsRenderings(t *testing.T) {
	root, db, outDir := scannedFixture(t)
	runCLI(t, "export", root, "--db", db, "--out", outDir, "--pairs", "--min-size", "0")

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "report.html,report.json,review.md" {
		t.Fatalf("export wrote %v, want exactly report.html, report.json, review.md (the old per-topic JSON files are gone)", names)
	}

	doc, err := reportdoc.Parse(readFile(t, filepath.Join(outDir, "report.json")))
	if err != nil {
		t.Fatalf("report.json is not a valid report document: %v", err)
	}
	if doc.Root != root || doc.SchemaVersion != reportdoc.SchemaVersion || doc.MeasuredAt.IsZero() {
		t.Errorf("document header wrong: %+v", doc)
	}
	if len(doc.Findings) == 0 || doc.Filter.MinSizeBytes != 0 {
		t.Errorf("findings = %d, filter = %+v; --min-size 0 should keep everything", len(doc.Findings), doc.Filter)
	}

	html := string(readFile(t, filepath.Join(outDir, "report.html")))
	if !strings.Contains(html, "DiskWise report") || !strings.Contains(html, "installer-1.0.dmg") {
		t.Errorf("report.html missing the title or the finding")
	}
	review := string(readFile(t, filepath.Join(outDir, "review.md")))
	if !strings.Contains(review, "# DiskWise review: "+root) || !strings.Contains(review, "- [ ]") {
		t.Errorf("review.md missing heading or checkbox:\n%s", review)
	}
}

func TestExport_RequiresOut(t *testing.T) {
	_, _, err := runCLIErr("export", t.TempDir(), "--db", filepath.Join(t.TempDir(), "i.db"))
	if err == nil {
		t.Error("export without --out should fail")
	}
}

// The point of the single document: every rendering is a pure function
// of it. Re-rendering report.json elsewhere gives byte-identical output.
func TestReportFrom_RendersTheSavedDocumentIdentically(t *testing.T) {
	root, db, outDir := scannedFixture(t)
	runCLI(t, "export", root, "--db", db, "--out", outDir, "--min-size", "0")
	saved := filepath.Join(outDir, "report.json")
	redo := t.TempDir()

	for _, c := range []struct{ format, want string }{
		{"html", "report.html"}, {"md", "review.md"}, {"json", "report.json"},
	} {
		dest := filepath.Join(redo, c.want)
		runCLI(t, "report", "--from", saved, "--format", c.format, "--out", dest)
		if !bytes.Equal(readFile(t, dest), readFile(t, filepath.Join(outDir, c.want))) {
			t.Errorf("report --from --format %s differs from what export wrote (%s)", c.format, c.want)
		}
	}

	x := filepath.Join(redo, "r.xlsx")
	runCLI(t, "report", "--from", saved, "--format", "xlsx", "--out", x)
	f, err := excelize.OpenReader(bytes.NewReader(readFile(t, x)))
	if err != nil {
		t.Fatalf("xlsx does not open: %v", err)
	}
	defer func() { _ = f.Close() }()
	if rows, _ := f.GetRows("Opportunities"); len(rows) < 2 {
		t.Errorf("xlsx has %d rows, want a header and at least one finding", len(rows))
	}

	// `review --from` is the same Markdown.
	md := runCLI(t, "review", "--from", saved)
	if md != string(readFile(t, filepath.Join(outDir, "review.md"))) {
		t.Error("review --from differs from review.md")
	}
}

func TestReportFrom_NeedsNoIndex(t *testing.T) {
	root, db, outDir := scannedFixture(t)
	runCLI(t, "export", root, "--db", db, "--out", outDir)
	// A different, empty index and a removed tree: only the document matters.
	other := filepath.Join(t.TempDir(), "empty.db")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "r.html")
	runCLI(t, "report", "--from", filepath.Join(outDir, "report.json"), "--db", other, "--out", out)
	if !strings.Contains(string(readFile(t, out)), "DiskWise report") {
		t.Error("rendering from a saved document should not need the index or the files")
	}
}

func TestReportFrom_Errors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"schemaVersion":"1","bogus":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"missing file":   {"report", "--from", filepath.Join(dir, "nope.json"), "--out", filepath.Join(dir, "o.html")},
		"invalid doc":    {"report", "--from", bad, "--out", filepath.Join(dir, "o.html")},
		"path and from":  {"report", dir, "--from", bad, "--out", filepath.Join(dir, "o.html")},
		"unknown format": {"report", "--from", bad, "--format", "pdf", "--out", filepath.Join(dir, "o.pdf")},
	}
	for name, args := range cases {
		if _, _, err := runCLIErr(args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "o.html")); err == nil {
		t.Error("a failed render must not leave an output file behind")
	}
}

func TestReport_SchemaFlag(t *testing.T) {
	out := runCLI(t, "report", "--schema")
	var s map[string]any
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("--schema output is not JSON: %v", err)
	}
	if s["$ref"] != "#/$defs/Document" {
		t.Errorf("schema $ref = %v", s["$ref"])
	}
}

// Redaction applies to a saved document too, so a report can be
// re-rendered safely for sharing without going back to the index.
func TestRedaction_AppliesToExportAndToSavedDocuments(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, filepath.Join(root, "clientname", "tool-1.0.dmg"), 200_000)
	db := filepath.Join(t.TempDir(), "index.db")
	outDir := filepath.Join(t.TempDir(), "out")
	runCLI(t, "scan", root, "--db", db)
	prefix := filepath.Join(root, "clientname")

	runCLI(t, "export", root, "--db", db, "--out", outDir, "--min-size", "0", "--redact-prefix", prefix)
	review := runCLI(t, "review", root, "--db", db, "--min-size", "0", "--redact-prefix", prefix)
	blobs := []string{review}
	for _, n := range []string{"report.json", "report.html", "review.md"} {
		blobs = append(blobs, string(readFile(t, filepath.Join(outDir, n))))
	}

	// An unredacted document, redacted only at render time.
	plain := filepath.Join(t.TempDir(), "plain")
	runCLI(t, "export", root, "--db", db, "--out", plain, "--min-size", "0")
	if !strings.Contains(string(readFile(t, filepath.Join(plain, "report.json"))), "clientname") {
		t.Fatal("test setup: the unredacted document should contain the name")
	}
	blobs = append(blobs, runCLI(t, "review", "--from", filepath.Join(plain, "report.json"), "--redact-prefix", prefix))

	for i, blob := range blobs {
		if strings.Contains(blob, "clientname") {
			t.Errorf("output %d still contains the redacted name", i)
		}
	}
}

func TestReview_MinSizeDefaultStatesWhatItOmits(t *testing.T) {
	root, db, _ := scannedFixture(t)

	def := runCLI(t, "review", root, "--db", db)
	if !strings.Contains(def, "smaller finding(s)") || !strings.Contains(def, "--min-size 0") {
		t.Errorf("default review must say what it omitted and how to see it:\n%s", def)
	}
	if strings.Contains(def, "installer-1.0.dmg") {
		t.Error("a 200 KB finding should be under the 1mb default")
	}
	all := runCLI(t, "review", root, "--db", db, "--min-size", "0")
	if !strings.Contains(all, "installer-1.0.dmg") || strings.Contains(all, "omitted") {
		t.Errorf("--min-size 0 must list everything and omit nothing:\n%s", all)
	}
}

func TestReview_UnknownFormat(t *testing.T) {
	if _, _, err := runCLIErr("review", t.TempDir(), "--format", "html", "--db", filepath.Join(t.TempDir(), "i.db")); err == nil {
		t.Error("review --format html should fail")
	}
}

// opportunities --json is its own report envelope (it predates the
// document and stays for scripts); the document is the thing to render.
func TestOpportunitiesJSON_IsTheReportEnvelope(t *testing.T) {
	root, db, _ := scannedFixture(t)
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(runCLI(t, "opportunities", root, "--db", db, "--json")), &m); err != nil {
		t.Fatalf("opportunities --json must be an object: %v", err)
	}
	for _, k := range []string{"Root", "ScannedAt", "ScanStatus", "Stale", "Opportunities"} {
		if _, ok := m[k]; !ok {
			t.Errorf("opportunities --json is missing %q", k)
		}
	}
}

// A path removed after the scan must be called out, on stderr so JSON
// output stays parseable.
func TestCommands_WarnWhenIndexedPathsNoLongerExist(t *testing.T) {
	root, db, _ := scannedFixture(t)
	if err := os.Remove(filepath.Join(root, "installer-1.0.dmg")); err != nil {
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

	// The document carries the same fact into every rendering.
	doc := filepath.Join(t.TempDir(), "report.json")
	_, errOut = runCLIBoth(t, "report", root, "--db", db, "--format", "json", "--min-size", "0", "--out", doc)
	if !strings.Contains(errOut, "no longer exist") {
		t.Errorf("report should warn about vanished paths, got %q", errOut)
	}
	d, err := reportdoc.Parse(readFile(t, doc))
	if err != nil {
		t.Fatal(err)
	}
	if d.Missing.Count != 1 {
		t.Errorf("document Missing = %+v, want 1", d.Missing)
	}
}
