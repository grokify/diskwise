package detect

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/knowledge"
	"github.com/grokify/diskwise/policy"
)

// ArtifactFamilyDetector groups repeated downloads of the same
// product into one finding — e.g. five elasticsearch-*.tar.gz
// versions in Downloads — with keep-newest and remove-all scenarios,
// rather than reporting each archive as an unrelated file.
type ArtifactFamilyDetector struct {
	Registry knowledge.Registry
}

func (d ArtifactFamilyDetector) ID() string { return "artifact-family" }

func (d ArtifactFamilyDetector) Detect(ctx context.Context, db *index.DB, root string) ([]Finding, error) {
	claimed, err := claimedPaths(d.Registry)
	if err != nil {
		return nil, fmt.Errorf("detect: artifact-family: %w", err)
	}

	rows, err := db.FilesByExtensions(ctx, root, archiveExtCandidates)
	if err != nil {
		return nil, fmt.Errorf("detect: artifact-family: %w", err)
	}

	var findings []Finding
	for groupKey, members := range groupArchiveFamilies(rows) {
		if len(members) < minFamilyMembers {
			continue
		}
		if isClaimed(members[0].row.Path, claimed) {
			continue
		}

		// Newest download first: the "keep" candidate. Recency is a
		// more reliable ordering signal than trying to semver-compare
		// arbitrary, loosely-parsed version strings.
		sort.Slice(members, func(i, j int) bool {
			return members[i].row.ModTime.After(members[j].row.ModTime)
		})

		var totalLogical, totalAllocated, oldAllocated int64
		var allPaths, oldPaths []string
		anyBackupLike := false
		for i, m := range members {
			totalLogical += m.row.LogicalSizeFor()
			totalAllocated += m.row.AllocatedSizeFor()
			allPaths = append(allPaths, m.row.Path)
			if i > 0 {
				oldAllocated += m.row.AllocatedSizeFor()
				oldPaths = append(oldPaths, m.row.Path)
			}
			if looksLikeBackup(m.row.Name) {
				anyBackupLike = true
			}
		}

		productName := strings.SplitN(groupKey, "\x00", 2)[1]
		familyDir := filepath.Dir(members[0].row.Path)
		proposed, confidence, reason := classifyFamily(anyBackupLike, len(members), members[0].row.Name)

		findings = append(findings, Finding{
			Entity: entity.Entity{
				ID:         familyDir + "/" + productName + "-*",
				Kind:       entity.KindArtifactFamily,
				Name:       productName,
				Detector:   "artifact-family",
				Confidence: confidence,
			},
			Path:          familyDir,
			Paths:         allPaths,
			LogicalSize:   totalLogical,
			AllocatedSize: totalAllocated,
			Confidence:    confidence,
			ActionClass:   policy.Evaluate(proposed, entity.KindArtifactFamily, confidence),
			Reason:        reason,
			Scenarios: []Scenario{
				{
					Name:             "keep-newest",
					Description:      "Keep " + members[0].row.Name + ", remove the other " + strconv.Itoa(len(members)-1),
					ReclaimableBytes: oldAllocated,
					Paths:            oldPaths,
				},
				{
					Name:             "remove-all",
					Description:      "Remove all " + strconv.Itoa(len(members)) + " downloaded versions",
					ReclaimableBytes: totalAllocated,
					Paths:            allPaths,
				},
			},
		})
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].AllocatedSize > findings[j].AllocatedSize })
	return findings, nil
}

func classifyFamily(anyBackupLike bool, memberCount int, newestName string) (proposed policy.ActionClass, confidence float64, reason string) {
	if anyBackupLike {
		return policy.Review, 0.2, "filenames suggest a deliberate backup or export; not treated as disposable regardless of the version pattern match"
	}
	return policy.LikelySafe, 0.75, fmt.Sprintf("%d downloaded versions of the same artifact; %s is newest", memberCount, newestName)
}
