package report

import (
	"strings"
	"testing"
	"time"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
)

func TestWriteHTML_SelfContainedNoExternalReferences(t *testing.T) {
	rows := []Row{
		{Tier: policy.SafeDelete, Kind: entity.KindCache, Path: "/Users/j/Library/Caches/go-build", AllocatedSize: 1 << 30, LogicalSize: 1 << 30, Confidence: 0.95, Reason: "regeneratable cache"},
	}
	var buf strings.Builder
	if err := WriteHTML(&buf, "/Users/j", rows, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()

	for _, forbidden := range []string{"http://", "https://", "cdn."} {
		if strings.Contains(out, forbidden) {
			t.Errorf("report references external resource %q; must be fully self-contained", forbidden)
		}
	}
	if !strings.Contains(out, "go-build") {
		t.Error("report is missing the row's path")
	}
	if !strings.Contains(out, "file:///Users/j/Library/Caches/go-build") {
		t.Error("report is missing a file:// link for the row's path")
	}
	if !strings.Contains(out, "1.0 GiB") {
		t.Error("report is missing the human-readable allocated size")
	}
	if !strings.Contains(out, "tier-safe_delete") {
		t.Error("report is missing the tier CSS class for styling/filtering")
	}
}

func TestWriteHTML_EscapesPathContent(t *testing.T) {
	rows := []Row{
		{Tier: policy.Review, Kind: entity.KindUnknown, Path: `/Users/j/<script>alert(1)</script>`, Reason: "test"},
	}
	var buf strings.Builder
	if err := WriteHTML(&buf, "/Users/j", rows, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Error("path was not HTML-escaped; a malicious path could inject script content")
	}
}

func TestWriteHTML_IncludesScenarios(t *testing.T) {
	rows := []Row{
		{
			Tier: policy.LikelySafe, Kind: entity.KindArtifactFamily, Path: "/dl/tool-*",
			AllocatedSize: 500, PathCount: 3,
			Scenarios: []detect.Scenario{
				{Name: "keep-newest", Description: "keep the latest, remove the rest", ReclaimableBytes: 300},
			},
		},
	}
	var buf strings.Builder
	if err := WriteHTML(&buf, "/dl", rows, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "keep-newest") || !strings.Contains(out, "keep the latest, remove the rest") {
		t.Error("report is missing scenario detail")
	}
}
