package detect

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/grokify/diskwise/index"
)

// minFamilyMembers is the smallest group size ArtifactFamilyDetector
// treats as a "family" — a single file is just an archive, not a
// version family. ArchiveInstallerDetector uses the same threshold so
// a file is reported by exactly one of the two detectors, never both
// and never neither.
const minFamilyMembers = 2

// archiveExtCandidates is the SQL-level filter: the stored ext column
// only captures a file's final path component (filepath.Ext), so this
// list includes the trailing extension of every compound suffix too
// (".gz" catches both "foo.gz" and "foo.tar.gz").
var archiveExtCandidates = []string{
	".dmg", ".pkg", ".xip", ".iso", ".ipsw",
	".zip", ".7z", ".rar",
	".gz", ".bz2", ".xz", ".zst",
	".tgz", ".tbz2", ".txz",
}

// archiveSuffixes are checked longest-first so "foo.tar.gz" strips to
// "foo", not "foo.tar".
var archiveSuffixes = []string{
	".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst",
	".tgz", ".tbz2", ".txz",
	".zip", ".7z", ".rar", ".dmg", ".pkg", ".xip", ".iso", ".ipsw",
	".gz", ".bz2", ".xz", ".zst",
}

// stripArchiveSuffix removes a recognized archive/installer extension
// from name, reporting whether one matched.
func stripArchiveSuffix(name string) (base string, ok bool) {
	lower := strings.ToLower(name)
	for _, suf := range archiveSuffixes {
		if strings.HasSuffix(lower, suf) {
			return name[:len(name)-len(suf)], true
		}
	}
	return name, false
}

// backupSuggestiveWords flags filenames that read as a deliberate
// backup or sensitive export rather than disposable download cruft —
// per TRD §7, these are excluded from likely_safe/safe_delete
// regardless of what other evidence (extraction sibling, version
// pattern) might otherwise suggest.
var backupSuggestiveWords = []string{"backup", "export", "tax", "financial", "invoice", "statement"}

func looksLikeBackup(name string) bool {
	lower := strings.ToLower(name)
	for _, w := range backupSuggestiveWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// versionPattern matches the first version-like number sequence in a
// filename (e.g. "9.1.3", "1.26.2", "3.9.11").
var versionPattern = regexp.MustCompile(`\d+(\.\d+){1,3}`)

// familyKey derives a grouping key from an archive's base name (after
// stripArchiveSuffix): everything before the first version-like
// number, normalized. Files sharing a key in the same directory are
// treated as versions of the same downloaded artifact. This is a
// best-effort heuristic, not a strict product/platform/arch parser —
// consistency across a product's repeated downloads matters more than
// exactness.
func familyKey(base string) (key, version string, ok bool) {
	loc := versionPattern.FindStringIndex(base)
	if loc == nil {
		return "", "", false
	}
	prefix := strings.TrimRight(base[:loc[0]], "-_. ")
	if prefix == "" {
		return "", "", false
	}
	return strings.ToLower(prefix), base[loc[0]:loc[1]], true
}

// archiveMember is one archive/installer file, with its parsed family
// version.
type archiveMember struct {
	row     index.NodeRow
	version string
}

// groupArchiveFamilies groups archive/installer rows by (directory,
// product-prefix). ArtifactFamilyDetector reports groups of 2+ as
// families; ArchiveInstallerDetector defers to those groups so the
// same file is never reported by both detectors.
func groupArchiveFamilies(rows []index.NodeRow) map[string][]archiveMember {
	families := make(map[string][]archiveMember)
	for _, row := range rows {
		base, ok := stripArchiveSuffix(row.Name)
		if !ok {
			continue
		}
		key, version, ok := familyKey(base)
		if !ok {
			continue
		}
		groupKey := filepath.Dir(row.Path) + "\x00" + key
		families[groupKey] = append(families[groupKey], archiveMember{row: row, version: version})
	}
	return families
}

// normalizeAppName strips extension, version suffixes, and
// spacing/punctuation, for fuzzy matching between installer filenames
// and installed .app bundle names (e.g. "VisualStudioCode-1.2.3.dmg"
// vs "Visual Studio Code.app").
var versionSuffixPattern = regexp.MustCompile(`[-_ ]v?\d+(\.\d+)*[a-zA-Z0-9]*$`)

func normalizeAppName(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	for {
		stripped := versionSuffixPattern.ReplaceAllString(base, "")
		if stripped == base {
			break
		}
		base = stripped
	}
	base = strings.ToLower(base)
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', '_', ' ', '.':
			return -1
		}
		return r
	}, base)
}

// listInstalledApps returns normalized .app bundle names under
// appsDir mapped to their modification time, for DMG installed-app
// evidence. A missing directory is not an error — just no evidence.
func listInstalledApps(appsDir string) (map[string]time.Time, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]time.Time{}, nil
		}
		return nil, err
	}
	apps := make(map[string]time.Time)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".app") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		apps[normalizeAppName(e.Name())] = info.ModTime()
	}
	return apps, nil
}
