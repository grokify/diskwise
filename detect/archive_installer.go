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

// ArchiveInstallerDetector flags downloaded archives and installers
// with evidence-based reclaimability: a newer installed .app bundle
// with a matching name (DMGs only), or an extraction-sibling
// directory suggesting the archive has already been used. Files that
// belong to a 2+-member version family are left to
// ArtifactFamilyDetector, and files under a KnownLocation are left to
// KnownLocationDetector — each byte is reported by exactly one detector.
type ArchiveInstallerDetector struct {
	Registry knowledge.Registry
	// ApplicationsDir overrides where installed .app bundles are
	// looked up for DMG evidence; "" defaults to /Applications.
	ApplicationsDir string
}

func (d ArchiveInstallerDetector) ID() string { return "archive-installer" }

func (d ArchiveInstallerDetector) Detect(ctx context.Context, db *index.DB, root string) ([]Finding, error) {
	claimed, err := claimedPaths(d.Registry)
	if err != nil {
		return nil, fmt.Errorf("detect: archive-installer: %w", err)
	}
	if err := withBundleRoots(ctx, db, root, claimed); err != nil {
		return nil, fmt.Errorf("detect: archive-installer: %w", err)
	}

	rows, err := db.FilesByExtensions(ctx, root, archiveExtCandidates)
	if err != nil {
		return nil, fmt.Errorf("detect: archive-installer: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	inFamily := make(map[string]bool)
	for _, members := range groupArchiveFamilies(rows) {
		if len(members) >= minFamilyMembers {
			for _, m := range members {
				inFamily[m.row.Path] = true
			}
		}
	}

	appsDir := d.ApplicationsDir
	if appsDir == "" {
		appsDir = "/Applications"
	}
	installedApps, err := listInstalledApps(appsDir)
	if err != nil {
		return nil, fmt.Errorf("detect: archive-installer: %w", err)
	}

	var findings []Finding
	for _, row := range rows {
		if isClaimed(row.Path, claimed) || inFamily[row.Path] {
			continue
		}
		base, ok := stripArchiveSuffix(row.Name)
		if !ok {
			continue
		}

		dir := filepath.Dir(row.Path)
		siblingRow, siblingOK, err := db.NodeByPath(ctx, filepath.Join(dir, base))
		if err != nil {
			return nil, fmt.Errorf("detect: archive-installer: %w", err)
		}
		extractedSibling := siblingOK && siblingRow.Kind == index.KindDir

		// A same-named directory is only a hint. When the archive's
		// member list is cheap to read (plain tar or zip), check it
		// against the directory so an archive that holds files the
		// directory lacks is not called likely-safe on name alone.
		var pair *ArchivePair
		if extractedSibling {
			// maxCompressed 1 skips gzip/bzip2: decompressing them
			// during a scan-wide report would be far too slow.
			if p, supported, perr := comparePairFor(ctx, db, row, siblingRow, 1); perr != nil {
				return nil, fmt.Errorf("detect: archive-installer: %w", perr)
			} else if supported {
				pair = &p
			}
		}

		var installedNewer bool
		if strings.EqualFold(filepath.Ext(row.Name), ".dmg") {
			if appModTime, found := installedApps[normalizeAppName(row.Name)]; found {
				installedNewer = appModTime.After(row.ModTime)
			}
		}

		proposed, confidence, reason := classifyArchive(row.Name, installedNewer, extractedSibling, pair)

		findings = append(findings, Finding{
			Entity: entity.Entity{
				ID:         row.Path,
				Kind:       entity.KindArchive,
				Name:       row.Name,
				Detector:   "archive-installer",
				Confidence: confidence,
			},
			Path:          row.Path,
			Paths:         []string{row.Path},
			LogicalSize:   row.LogicalSizeFor(),
			AllocatedSize: row.AllocatedSizeFor(),
			Confidence:    confidence,
			ActionClass:   policy.Evaluate(proposed, entity.KindArchive, confidence),
			Reason:        reason,
		})
	}
	return findings, nil
}

func classifyArchive(name string, installedNewer, extractedSibling bool, pair *ArchivePair) (proposed policy.ActionClass, confidence float64, reason string) {
	switch {
	case looksLikeBackup(name):
		return policy.Review, 0.2, "filename suggests a deliberate backup or export; not treated as disposable regardless of other evidence"
	case installedNewer:
		return policy.SafeDelete, 0.9, "already installed: a newer .app bundle with a matching name exists in Applications"
	case extractedSibling && pair != nil && pair.Verdict == PairArchiveHasMore:
		return policy.Review, 0.3, fmt.Sprintf("a same-named directory exists, but the archive lists %d more file(s) than it holds (%s); run `diskwise pairs` and check before removing the archive", pair.ArchiveExtra, pair.Compared)
	case extractedSibling && pair != nil && pair.Verdict == PairSameCount:
		return policy.Review, 0.4, fmt.Sprintf("a same-named directory has the same number of files (%d) but different total bytes; run `diskwise pairs` and check before removing the archive", pair.ArchiveFiles)
	case extractedSibling && pair != nil && (pair.Verdict == PairSame || pair.Verdict == PairDirHasMore):
		return policy.LikelySafe, 0.75, fmt.Sprintf("already extracted: a same-named directory holds %d file(s) against the archive's %d (%s); contents are not hashed", pair.DirFiles, pair.ArchiveFiles, pair.Compared)
	case extractedSibling:
		return policy.LikelySafe, 0.7, "appears already extracted: a directory with the same name exists alongside it"
	default:
		return policy.Review, 0.3, "downloaded archive/installer; no evidence it has been used yet"
	}
}
