package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/scan"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ingestFixture scans root and returns a fresh index containing it.
func ingestFixture(t *testing.T, root string) *index.DB {
	t.Helper()
	tree, _, err := scan.Walk(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db, err := index.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.IngestScan(context.Background(), tree, nil); err != nil {
		t.Fatal(err)
	}
	return db
}

// fixtureRegistryFor returns a registry claiming a single hermetic
// fixture path, for tests that need to verify claimed-path exclusion
// without depending on knowledge.Default's real machine locations.
func fixtureRegistryFor(claimedPath string) knowledge.Registry {
	return knowledge.Registry{
		{
			ID:                 "fixture-claimed",
			Description:        "Fixture claimed location",
			DataPaths:          []string{claimedPath},
			Entity:             entity.KindCache,
			Semantics:          knowledge.SemanticsRegeneratableCache,
			DefaultActionClass: policy.SafeDelete,
		},
	}
}
