package detect

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

// managedBundleKinds lists app-managed package directories by name
// suffix. Their contents are live user data laid out by the owning app
// (and re-laid-out across app versions), so DiskWise classifies the
// bundle as a whole and never looks inside it.
var managedBundleKinds = []struct {
	suffix string
	label  string
}{
	{".photoslibrary", "Photos library"},
	{".migratedphotolibrary", "migrated iPhoto library"},
	{".aplibrary", "Aperture library"},
	{".musiclibrary", "Music library"},
	{".tvlibrary", "TV library"},
	{".imovielibrary", "iMovie library"},
	{".fcpbundle", "Final Cut Pro library"},
	{".logicx", "Logic Pro project"},
	{".band", "GarageBand project"},
	{".sparsebundle", "sparse bundle disk image"},
}

func managedBundleSuffixes() []string {
	out := make([]string, len(managedBundleKinds))
	for i, k := range managedBundleKinds {
		out[i] = k.suffix
	}
	return out
}

// ManagedBundleDetector reports app-managed bundles (Photos libraries
// and the like) as KEEP findings — explaining the bytes instead of
// leaving them "unknown", and stating that the bundle's internals are
// not DiskWise's to edit. Only outermost bundles are reported, and the
// other detectors treat every path inside a bundle as claimed.
type ManagedBundleDetector struct {
	// Registry's known locations take precedence: a bundle inside one
	// (e.g. a simulator's photo library) belongs to that finding.
	Registry knowledge.Registry
}

func (ManagedBundleDetector) ID() string { return "managed-bundle" }

func (d ManagedBundleDetector) Detect(ctx context.Context, db *index.DB, root string) ([]Finding, error) {
	claimed, err := claimedPaths(d.Registry)
	if err != nil {
		return nil, fmt.Errorf("detect: managed-bundle: %w", err)
	}
	rows, err := db.DirsByNameSuffixes(ctx, root, managedBundleSuffixes())
	if err != nil {
		return nil, fmt.Errorf("detect: managed-bundle: %w", err)
	}

	var findings []Finding
	var outer []string
	for _, row := range rows { // largest first; an inner bundle can't outrank its container
		if isNested(row.Path, outer) || isClaimed(row.Path, claimed) {
			continue
		}
		outer = append(outer, row.Path)

		label := bundleLabel(row.Name)
		findings = append(findings, Finding{
			Entity: entity.Entity{
				ID:         row.Path,
				Kind:       entity.KindManagedBundle,
				Name:       row.Name,
				Detector:   "managed-bundle",
				Confidence: 1,
			},
			Path:          row.Path,
			Paths:         []string{row.Path},
			LogicalSize:   row.LogicalSizeFor(),
			AllocatedSize: row.AllocatedSizeFor(),
			Confidence:    1,
			ActionClass:   policy.Evaluate(policy.Keep, entity.KindManagedBundle, 1),
			Reason: label + ": app-managed bundle holding user data. Manage it from the owning app, not by editing its contents. " +
				"If another copy exists (e.g. an archive), compare the original media only: internal layout differs between app versions.",
		})
	}
	return findings, nil
}

func bundleLabel(name string) string {
	lower := strings.ToLower(name)
	for _, k := range managedBundleKinds {
		if strings.HasSuffix(lower, k.suffix) {
			return k.label
		}
	}
	return "managed bundle"
}

// withBundleRoots adds the root of every managed bundle under root to
// claimed, so a detector treats everything inside a bundle as owned by
// ManagedBundleDetector.
func withBundleRoots(ctx context.Context, db *index.DB, root string, claimed map[string]bool) error {
	rows, err := db.DirsByNameSuffixes(ctx, root, managedBundleSuffixes())
	if err != nil {
		return fmt.Errorf("managed bundles under %s: %w", root, err)
	}
	for _, row := range rows {
		claimed[filepath.Clean(row.Path)] = true
	}
	return nil
}
