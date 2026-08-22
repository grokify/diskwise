package detect

import (
	"context"
	"fmt"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

// KnownLocationDetector turns knowledge-registry hits into findings:
// for each relevant KnownLocation whose data path is indexed under
// root, it reports the already-measured size with the registry's
// default recommendation and an explanation of its semantics.
type KnownLocationDetector struct {
	Registry knowledge.Registry
}

func (d KnownLocationDetector) ID() string { return "known-location" }

func (d KnownLocationDetector) Detect(ctx context.Context, db *index.DB, root string) ([]Finding, error) {
	relevant, err := d.Registry.Relevant()
	if err != nil {
		return nil, fmt.Errorf("detect: known-location: %w", err)
	}

	var findings []Finding
	for _, loc := range relevant {
		paths, err := loc.ExpandedDataPaths()
		if err != nil {
			return nil, fmt.Errorf("detect: known-location %s: %w", loc.ID, err)
		}
		for _, path := range paths {
			if !underRoot(path, root) {
				continue
			}
			row, ok, err := db.NodeByPath(ctx, path)
			if err != nil {
				return nil, fmt.Errorf("detect: known-location %s: %w", loc.ID, err)
			}
			if !ok {
				continue // not indexed: outside the scanned tree, or genuinely absent
			}
			findings = append(findings, Finding{
				Entity: entity.Entity{
					ID:         loc.ID,
					Kind:       loc.Entity,
					Name:       loc.Description,
					Detector:   "known-location:" + loc.ID,
					Confidence: 1.0, // a registry match is definitional, not inferred
				},
				Path:          row.Path,
				Paths:         []string{row.Path},
				LogicalSize:   row.LogicalSizeFor(),
				AllocatedSize: row.AllocatedSizeFor(),
				Confidence:    1.0,
				ActionClass:   policy.Evaluate(loc.DefaultActionClass, loc.Entity, 1.0),
				Reason:        describeSemantics(loc),
			})
		}
	}
	return findings, nil
}

func describeSemantics(loc knowledge.KnownLocation) string {
	switch loc.Semantics {
	case knowledge.SemanticsSparseVMDisk:
		return loc.Description + " — sparse virtual disk; reported at allocated size, which reflects real usage, not apparent size"
	case knowledge.SemanticsRegeneratableCache:
		return loc.Description + " — regeneratable cache"
	case knowledge.SemanticsInstalledSoftware:
		return loc.Description + " — installed software, not a cache"
	case knowledge.SemanticsAlreadyDeleted:
		return loc.Description + " — already moved to Trash"
	default:
		return loc.Description
	}
}

// claimedPaths returns the set of expanded data paths for every
// relevant registry entry, regardless of whether they're indexed —
// used to keep other detectors from re-reporting territory a known
// location already explains.
func claimedPaths(registry knowledge.Registry) (map[string]bool, error) {
	relevant, err := registry.Relevant()
	if err != nil {
		return nil, fmt.Errorf("detect: claimed paths: %w", err)
	}
	claimed := make(map[string]bool)
	for _, loc := range relevant {
		paths, err := loc.ExpandedDataPaths()
		if err != nil {
			return nil, fmt.Errorf("detect: claimed paths %s: %w", loc.ID, err)
		}
		for _, p := range paths {
			claimed[p] = true
		}
	}
	return claimed, nil
}
