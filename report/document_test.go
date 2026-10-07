package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/grokify/diskwise/reportdoc"
)

func sampleDoc() *reportdoc.Document {
	return &reportdoc.Document{
		SchemaVersion: reportdoc.SchemaVersion, Root: "/Users/example",
		MeasuredAt: time.Date(2026, 3, 4, 5, 6, 0, 0, time.UTC), ScanStatus: "partial", Stale: true,
		Tiers:            map[string]int64{"safe_delete": 4 << 30, "review": 9 << 30, "keep": 98 << 30, "unknown": 700 << 30},
		ReclaimableBytes: 13 << 30,
		Missing:          reportdoc.Missing{Count: 1, Bytes: 3 << 30},
		Filter:           reportdoc.Filter{MinSizeBytes: 1 << 20, OmittedCount: 40, OmittedBytes: 5 << 20},
		Findings: []reportdoc.Finding{
			{Tier: "review", Kind: "model", Name: "AI models", Detector: "known-location:ai-models", Path: "/Users/example/.ollama/models",
				AllocatedBytes: 9 << 30, LogicalBytes: 9 << 30, Confidence: 1, Reason: "Local AI models", ActionablePaths: []string{"/Users/example/.ollama/models"}},
			{Tier: "safe_delete", Kind: "cache", Name: "Go caches", Detector: "known-location:go-caches", Path: "/Users/example/go/pkg/mod",
				AllocatedBytes: 3 << 30, LogicalBytes: 3 << 30, Confidence: 1, Reason: "regeneratable", Missing: true},
			{Tier: "safe_delete", Kind: "archive", Name: "tool.dmg", Detector: "archive-installer", Path: "/Users/example/Downloads/tool.dmg",
				AllocatedBytes: 1 << 30, LogicalBytes: 1 << 30, Confidence: 0.9, Reason: "already installed"},
			{Tier: "keep", Kind: "managed_bundle", Name: "Photos Library.photoslibrary", Detector: "managed-bundle", Path: "/Users/example/Pictures/Photos Library.photoslibrary",
				AllocatedBytes: 98 << 30, LogicalBytes: 98 << 30, Confidence: 1, Reason: "managed bundle"},
			{Tier: "unknown", Kind: "unknown", Name: "Data", Detector: "large-unexplained", Path: "/Users/example/Data",
				AllocatedBytes: 300 << 30, LogicalBytes: 300 << 30, Reason: "unexplained"},
		},
		Pairs: []reportdoc.Pair{{
			Archive: "/Users/example/Backup/P.tar", ArchiveAlloc: 5 << 30, Dir: "/Users/example/Backup/P", DirAlloc: 5 << 30,
			Compared: "photos library originals only", ArchiveFiles: 10, ArchiveBytes: 100, DirFiles: 8, DirBytes: 90,
			Verdict: "archive_has_more", ArchiveExtra: 2,
		}},
	}
}

func TestWriteHTMLDocument_ShowsEverythingTheDocumentSays(t *testing.T) {
	var buf strings.Builder
	if err := WriteHTMLDocument(&buf, sampleDoc()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"measured 2026-03-04 05:06 UTC",
		"13.0 GiB reclaimable",
		"more than a week old",                           // stale banner
		"Partial scan",                                   // partial banner
		"1 finding(s) (3.0 GiB) no longer exist on disk", // missing banner
		"40 smaller finding(s) totalling 5.0 MiB",        // filter banner
		"(no longer exists)",                             // marker on the finding
		"<h2>Known heavy locations</h2>",
		"<h2>Large unexplained directories</h2>",
		"<h2>Archives beside extracted copies</h2>",
		"the archive lists 2 more file(s)",
		"archive_has_more",
		"700.0 GiB", // the unknown tier's pill: totals cover findings not in the table
		"98.0 GiB",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing %q", want)
		}
	}

	// Self-contained: no external script, stylesheet or font.
	for _, forbidden := range []string{"http://", "https://", "cdn.", "<script src", "<link "} {
		if strings.Contains(out, forbidden) {
			t.Errorf("HTML references an external resource: %q", forbidden)
		}
	}

	// Hotspot sections: the registry location and the bundle are "known";
	// the archive is not; the unknown directory is "unexplained".
	known := between(out, "<h2>Known heavy locations</h2>", "<h2>")
	for _, in := range []string{".ollama/models", "Photos Library.photoslibrary", "go/pkg/mod"} {
		if !strings.Contains(known, in) {
			t.Errorf("known locations missing %q", in)
		}
	}
	if strings.Contains(known, "tool.dmg") {
		t.Error("an archive finding is not a known location")
	}
	unexplained := between(out, "<h2>Large unexplained directories</h2>", "<h2>")
	if !strings.Contains(unexplained, "/Users/example/Data") {
		t.Error("the unknown-tier directory should be listed as unexplained")
	}
}

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	if j := strings.Index(rest, end); j >= 0 {
		return rest[:j]
	}
	return rest
}

// Rendering is a pure function of the document: the same input renders
// byte-identical output, with no clock involved.
func TestWriteHTMLDocument_IsDeterministic(t *testing.T) {
	render := func() string {
		var buf strings.Builder
		if err := WriteHTMLDocument(&buf, sampleDoc()); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if a, b := render(), render(); a != b {
		t.Error("two renders of the same document differ")
	}
}

func TestWriteHTMLDocument_QuietWhenNothingToWarnAbout(t *testing.T) {
	d := sampleDoc()
	d.Stale, d.ScanStatus = false, "complete"
	d.Missing, d.Filter, d.Pairs = reportdoc.Missing{}, reportdoc.Filter{}, nil
	for i := range d.Findings {
		d.Findings[i].Missing = false
	}
	var buf strings.Builder
	if err := WriteHTMLDocument(&buf, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, absent := range []string{`class="banner`, "<h2>Archives beside", "(no longer exists)"} {
		if strings.Contains(out, absent) {
			t.Errorf("a clean document should not render %q", absent)
		}
	}
}

func TestDocumentRenderers_MarkdownAndXLSX(t *testing.T) {
	d := sampleDoc()

	var md strings.Builder
	if err := WriteMarkdownDocument(&md, d); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# DiskWise review: /Users/example", "Measured 2026-03-04 05:06 UTC (partial scan",
		"more than a week old", "1 finding(s) no longer exist", "40 smaller finding(s)", "## Archives beside extracted copies",
		"`/Users/example/go/pkg/mod` _(no longer exists)_"} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("markdown missing %q", want)
		}
	}

	var x bytes.Buffer
	if err := WriteXLSXDocument(&x, d); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(x.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := f.GetRows(xlsxSheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+len(d.Findings) {
		t.Errorf("xlsx rows = %d, want header + %d findings", len(rows), len(d.Findings))
	}
}

func TestRowsFromDocument_OrderedAndComplete(t *testing.T) {
	rows := RowsFromDocument(sampleDoc())
	if len(rows) != 5 {
		t.Fatalf("rows = %d", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		a, b := rows[i-1], rows[i]
		if tierRank(a.Tier) > tierRank(b.Tier) || (a.Tier == b.Tier && a.AllocatedSize < b.AllocatedSize) {
			t.Errorf("rows %d/%d out of order: %s %d then %s %d", i-1, i, a.Tier, a.AllocatedSize, b.Tier, b.AllocatedSize)
		}
	}
	var missing, detector bool
	for _, r := range rows {
		missing = missing || r.Missing
		detector = detector || r.Detector == "known-location:go-caches"
	}
	if !missing || !detector {
		t.Error("Missing and Detector must carry through to rows")
	}
}
