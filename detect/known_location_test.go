package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

func TestKnownLocationDetector_Detect(t *testing.T) {
	root := t.TempDir()
	cachePath := filepath.Join(root, "SomeApp", "Cache")
	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(cachePath, "data.bin"), 50_000)
	if err := os.MkdirAll(filepath.Join(root, "Unrelated"), 0o755); err != nil {
		t.Fatal(err)
	}

	db := ingestFixture(t, root)

	registry := knowledge.Registry{
		{
			ID:                 "someapp-cache",
			Description:        "SomeApp cache",
			DataPaths:          []string{cachePath},
			Entity:             entity.KindCache,
			Semantics:          knowledge.SemanticsRegeneratableCache,
			DefaultActionClass: policy.SafeDelete,
		},
		{
			ID:                 "not-present",
			Description:        "Not present on this fixture",
			DataPaths:          []string{filepath.Join(root, "DoesNotExist")},
			DefaultActionClass: policy.Review,
		},
	}

	d := KnownLocationDetector{Registry: registry}
	findings, err := d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}

	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Path != cachePath {
		t.Errorf("Path = %s, want %s", f.Path, cachePath)
	}
	if f.AllocatedSize <= 0 {
		t.Errorf("AllocatedSize = %d, want > 0", f.AllocatedSize)
	}
	if f.ActionClass != policy.SafeDelete {
		t.Errorf("ActionClass = %s, want safe_delete", f.ActionClass)
	}
	if f.Entity.Kind != entity.KindCache {
		t.Errorf("Entity.Kind = %s, want cache", f.Entity.Kind)
	}
	if f.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", f.Confidence)
	}
}

// TestKnownLocationDetector_FiltersByRoot verifies a location outside
// the queried root is excluded, even though it's indexed — findings
// should never leak beyond the scope the caller asked about.
func TestKnownLocationDetector_FiltersByRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "Sub")
	other := filepath.Join(root, "Other")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(other, "data.bin"), 1_000)

	db := ingestFixture(t, root)

	registry := knowledge.Registry{
		{ID: "other-loc", Description: "Other location", DataPaths: []string{other}, DefaultActionClass: policy.Review},
	}
	d := KnownLocationDetector{Registry: registry}

	findings, err := d.Detect(context.Background(), db, sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings scoped to sub, want 0 (other is outside sub)", len(findings))
	}

	findings, err = d.Detect(context.Background(), db, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings scoped to root, want 1", len(findings))
	}
}

func TestIsClaimed(t *testing.T) {
	claimed := map[string]bool{"/a/b": true}
	if !isClaimed("/a/b", claimed) {
		t.Error("exact match should be claimed")
	}
	if !isClaimed("/a/b/c", claimed) {
		t.Error("descendant of a claimed path should be claimed")
	}
	if isClaimed("/a/bc", claimed) {
		t.Error("shared string prefix should not be claimed")
	}
	if isClaimed("/x", claimed) {
		t.Error("unrelated path should not be claimed")
	}
}
