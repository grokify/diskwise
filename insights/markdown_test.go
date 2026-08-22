package insights

import (
	"strings"
	"testing"
)

func TestWriteMarkdown_IncludesKeyContent(t *testing.T) {
	var buf strings.Builder
	if err := WriteMarkdown(&buf, sampleReport()); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"# DiskWise savings report — /Users/j",
		"1.7 TiB of 1.8 TiB used",
		"1. Duplicate 2022 migration backup in ~/Data",
		"stickers16",
		"stickers_jgo_grokify.tar and stickers_jgo_grokify_copy-2.tar are byte-identical",
		"chrome://settings/clearBrowserData",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output is missing %q", want)
		}
	}
}

func TestWriteMarkdown_RendersPairsAsTable(t *testing.T) {
	var buf strings.Builder
	if err := WriteMarkdown(&buf, sampleReport()); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"| Kept | Redundant | Notes |",
		"/Users/j/Data/stickers16/grokify",
		"/Users/j/Data/stickers16/stickers_jgo_grokify.tar",
		"byte-identical size to stickers_jgo_grokify_copy-2.tar",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output is missing pairs-table content %q", want)
		}
	}
}

func TestWriteMarkdown_NumbersVerificationStepsSequentially(t *testing.T) {
	r := sampleReport()
	r.Opportunities[0].VerificationSteps = []string{"first step", "second step", "third step"}
	var buf strings.Builder
	if err := WriteMarkdown(&buf, r); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"1. first step", "2. second step", "3. third step"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output is missing sequentially numbered step %q; got:\n%s", want, out)
		}
	}
}

func TestWriteMarkdown_EmptyOpportunities(t *testing.T) {
	r := sampleReport()
	r.Opportunities = nil
	var buf strings.Builder
	if err := WriteMarkdown(&buf, r); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if !strings.Contains(buf.String(), r.Summary) {
		t.Error("markdown output is missing the summary even with no opportunities")
	}
}
