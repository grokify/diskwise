package detect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

func TestLargeUnexplainedDetector_ExcludesClaimedAndSmall(t *testing.T) {
	root := t.TempDir()
	claimedDir := filepath.Join(root, "Claimed")
	bigUnexplained := filepath.Join(root, "BigMystery")
	smallDir := filepath.Join(root, "Small")
	for _, d := range []string{claimedDir, bigUnexplained, smallDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(claimedDir, "f.bin"), 5_000_000)
	writeFile(t, filepath.Join(bigUnexplained, "f.bin"), 5_000_000)
	writeFile(t, filepath.Join(smallDir, "f.bin"), 100)

	db := ingestFixture(t, root)

	registry := knowledge.Registry{
		{ID: "claimed", Description: "Claimed", DataPaths: []string{claimedDir}, DefaultActionClass: policy.SafeDelete},
	}
	d := LargeUnexplainedDetector{Registry: registry, MinSize: 1_000_000, Limit: 10}

	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (bigUnexplained only)", len(findings))
	}
	if findings[0].Path != bigUnexplained {
		t.Errorf("Path = %s, want %s", findings[0].Path, bigUnexplained)
	}
	if findings[0].ActionClass != policy.Unknown {
		t.Errorf("ActionClass = %s, want unknown", findings[0].ActionClass)
	}
	if findings[0].Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", findings[0].Confidence)
	}
}

func TestLargeUnexplainedDetector_RespectsLimit(t *testing.T) {
	root := t.TempDir()
	for i := range 5 {
		d := filepath.Join(root, fmt.Sprintf("dir%d", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(d, "f.bin"), 2_000_000)
	}
	db := ingestFixture(t, root)

	det := LargeUnexplainedDetector{MinSize: 1_000_000, Limit: 2}
	findings, err := det.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2 (limit)", len(findings))
	}
}

// A directory and its own large subdirectories must not both be
// reported: their sizes overlap, so listing both double-counts.
func TestLargeUnexplainedDetector_FindingsAreDisjoint(t *testing.T) {
	root := t.TempDir()
	outer := filepath.Join(root, "Outer")
	inner := filepath.Join(outer, "Inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(inner, "f.bin"), 3_000_000)
	writeFile(t, filepath.Join(outer, "g.bin"), 1_500_000)
	db := ingestFixture(t, root)

	det := LargeUnexplainedDetector{MinSize: 1_000_000, Limit: 10}
	findings, err := det.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Path != outer {
		var got []string
		for _, f := range findings {
			got = append(got, f.Path)
		}
		t.Fatalf("findings = %v, want only the outermost directory %s", got, outer)
	}
}
