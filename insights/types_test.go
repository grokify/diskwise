package insights

import (
	"encoding/json"
	"testing"
	"time"
)

func sampleReport() Report {
	return Report{
		SchemaVersion: SchemaVersion,
		Root:          "/Users/j",
		GeneratedAt:   time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
		UsedBytes:     1700 << 30,
		CapacityBytes: 1800 << 30,
		Summary:       "1.7 TiB of 1.8 TiB used; the largest reclaim opportunity is a duplicated 2022 migration backup.",
		Opportunities: []Opportunity{
			{
				Rank:           1,
				Title:          "Duplicate 2022 migration backup in ~/Data",
				Category:       CategoryDuplicateBackup,
				Directories:    []string{"/Users/j/Data/stickers16"},
				EstimatedBytes: 200 << 30,
				Confidence:     ConfidenceHigh,
				Description:    "Extracted folders sit next to tar archives of the same content.",
				Evidence:       []string{"stickers_jgo_grokify.tar and stickers_jgo_grokify_copy-2.tar are byte-identical in size"},
				VerificationSteps: []string{
					"Compare tar contents against the extracted folder with `tar -tf`",
				},
				Actions: []Action{
					{
						Method: MethodFilesystem,
						Pairs: []Pair{
							{
								Kept:      "/Users/j/Data/stickers16/grokify",
								Redundant: "/Users/j/Data/stickers16/stickers_jgo_grokify.tar",
								Notes:     "byte-identical size to stickers_jgo_grokify_copy-2.tar",
							},
						},
					},
				},
			},
			{
				Rank:           2,
				Title:          "Regeneratable dev/browser caches",
				Category:       CategoryRegeneratableCache,
				Directories:    []string{"/Users/j/Library/Caches/go-build", "/Users/j/Library/Caches/Google/Chrome"},
				EstimatedBytes: 35 << 30,
				Confidence:     ConfidenceHigh,
				Description:    "Pure caches that rebuild on next use.",
				Actions: []Action{
					{Method: MethodFilesystem, Target: "Go build cache", Directories: []string{"/Users/j/Library/Caches/go-build"}},
					{
						Method: MethodAppUI, Target: "Google Chrome",
						Steps: []string{
							"Open Chrome",
							"Go to chrome://settings/clearBrowserData",
							"Select 'Cached images and files'",
							"Click 'Clear data'",
						},
					},
				},
			},
		},
	}
}

func TestReport_ValidateAccepts(t *testing.T) {
	if err := sampleReport().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestReport_ValidateRejectsBadSchemaVersion(t *testing.T) {
	r := sampleReport()
	r.SchemaVersion = "999"
	if err := r.Validate(); err == nil {
		t.Fatal("expected an error for an unrecognized schemaVersion")
	}
}

func TestReport_ValidateRejectsGapInRanks(t *testing.T) {
	r := sampleReport()
	r.Opportunities[1].Rank = 5
	if err := r.Validate(); err == nil {
		t.Fatal("expected an error for a gap in rank sequence")
	}
}

func TestReport_ValidateRejectsDuplicateRank(t *testing.T) {
	r := sampleReport()
	r.Opportunities[1].Rank = 1
	if err := r.Validate(); err == nil {
		t.Fatal("expected an error for a duplicate rank")
	}
}

func TestReport_ValidateRejectsUnknownCategory(t *testing.T) {
	r := sampleReport()
	r.Opportunities[0].Category = Category("not_a_real_category")
	if err := r.Validate(); err == nil {
		t.Fatal("expected an error for an unknown category")
	}
}

func TestReport_ValidateRejectsEmptyDirectories(t *testing.T) {
	r := sampleReport()
	r.Opportunities[0].Directories = nil
	if err := r.Validate(); err == nil {
		t.Fatal("expected an error for an opportunity with no directories")
	}
}

func TestReport_ValidateRejectsUnknownActionMethod(t *testing.T) {
	r := sampleReport()
	r.Opportunities[0].Actions[0].Method = Method("telepathy")
	if err := r.Validate(); err == nil {
		t.Fatal("expected an error for an unknown action method")
	}
}

func TestReport_ValidateRejectsIncompletePair(t *testing.T) {
	r := sampleReport()
	r.Opportunities[0].Actions[0].Pairs[0].Redundant = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected an error for a pair missing its redundant path")
	}
}

func TestParse_RoundTripsAndValidates(t *testing.T) {
	r := sampleReport()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.Root != r.Root || len(parsed.Opportunities) != len(r.Opportunities) {
		t.Errorf("Parse round-trip mismatch: got %+v", parsed)
	}
}

func TestParse_RejectsUnknownFields(t *testing.T) {
	raw := `{"schemaVersion":"1","root":"/x","generatedAt":"2026-01-01T00:00:00Z","summary":"s","opportunities":[],"typo_field":true}`
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("expected Parse to reject an unknown field")
	}
}

func TestParse_RejectsInvalidReport(t *testing.T) {
	raw := `{"schemaVersion":"1","root":"/x","generatedAt":"2026-01-01T00:00:00Z","summary":"s","opportunities":[{"rank":1,"title":"t","category":"bogus","directories":["/x"],"estimatedBytes":1,"confidence":"high","description":"d"}]}`
	if _, err := Parse([]byte(raw)); err == nil {
		t.Fatal("expected Parse to reject an invalid category via Validate")
	}
}
