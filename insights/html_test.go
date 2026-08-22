package insights

import (
	"strings"
	"testing"
)

func TestWriteHTML_SelfContainedAndIncludesKeyContent(t *testing.T) {
	var buf strings.Builder
	if err := WriteHTML(&buf, sampleReport()); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, forbidden := range []string{"http://", "https://", "cdn."} {
		if strings.Contains(out, forbidden) {
			t.Errorf("insights report references external resource %q; must be self-contained", forbidden)
		}
	}
	for _, want := range []string{
		"Duplicate 2022 migration backup in ~/Data",
		"file:///Users/j/Data/stickers16",
		"cat-duplicate_backup",
		"conf-high",
		"chrome://settings/clearBrowserData",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("html output is missing %q", want)
		}
	}
}

func TestWriteHTML_RendersPairsAsTableWithFileLinks(t *testing.T) {
	var buf strings.Builder
	if err := WriteHTML(&buf, sampleReport()); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"<table><thead><tr><th>Kept</th><th>Redundant</th><th>Notes</th></tr></thead>",
		"file:///Users/j/Data/stickers16/grokify",
		"file:///Users/j/Data/stickers16/stickers_jgo_grokify.tar",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("html output is missing pairs-table content %q", want)
		}
	}
}

func TestWriteHTML_EscapesDescriptionContent(t *testing.T) {
	r := sampleReport()
	r.Opportunities[0].Description = `<script>alert(1)</script>`
	var buf strings.Builder
	if err := WriteHTML(&buf, r); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	if strings.Contains(buf.String(), "<script>alert(1)</script>") {
		t.Error("description was not HTML-escaped")
	}
}
