package report

import (
	"sort"
	"strings"
	"testing"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
)

// sortedRows orders rows as Rows does (tier, then size) without
// needing service.Opportunity fixtures.
func sortedRows(rows []Row) []Row {
	out := append([]Row(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := tierRank(out[i].Tier), tierRank(out[j].Tier)
		if ri != rj {
			return ri < rj
		}
		return out[i].AllocatedSize > out[j].AllocatedSize
	})
	return out
}

func TestWriteMarkdown_GroupsByTierWithCheckboxes(t *testing.T) {
	rows := []Row{
		{Tier: policy.Review, Kind: entity.KindModel, Path: "/m/models", AllocatedSize: 3 << 30, Reason: "local models"},
		{Tier: policy.SafeDelete, Kind: entity.KindCache, Path: "/c/go-build", AllocatedSize: 2 << 30, Reason: "regeneratable cache"},
		{Tier: policy.Keep, Kind: entity.KindManagedBundle, Path: "/p/Photos Library.photoslibrary", AllocatedSize: 9 << 30},
		{Tier: policy.Unknown, Kind: entity.KindUnknown, Path: "/big", AllocatedSize: 100 << 30},
		{Tier: policy.LikelySafe, Kind: entity.KindArtifactFamily, Path: "/d", AllocatedSize: 1 << 30,
			Scenarios: []detect.Scenario{{Name: "keep-newest", Description: "Keep v2", ReclaimableBytes: 512 << 20}}},
	}
	var buf strings.Builder
	if err := WriteMarkdown(&buf, "/root", sortedRows(rows), nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Savings total counts safe + likely + review only (2+1+3 GiB), not keep/unknown.
	if !strings.Contains(out, "**6.0 GiB**") {
		t.Errorf("savings total wrong:\n%s", out)
	}
	for _, want := range []string{
		"## Safe to delete: 2.0 GiB",
		"- [ ] **2.0 GiB** cache `/c/go-build`",
		"  - regeneratable cache",
		"  - _keep-newest_ (512.0 MiB): Keep v2",
		"## Keep (managed or protected): 9.0 GiB",
		"## Unexplained (not a recommendation): 100.0 GiB",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Index(out, "## Safe to delete") > strings.Index(out, "## Review") {
		t.Error("tiers must be ordered most-actionable first")
	}
}

// Every list must be preceded by a blank line (repo Markdown rule).
func TestWriteMarkdown_BlankLineBeforeLists(t *testing.T) {
	rows := []Row{{Tier: policy.SafeDelete, Kind: entity.KindCache, Path: "/c", AllocatedSize: 1 << 20, Reason: "r"}}
	pairs := []detect.ArchivePair{{Archive: "/a.tar", Dir: "/a", Verdict: detect.PairSame, Compared: detect.CompareAllFiles}}
	var buf strings.Builder
	if err := WriteMarkdown(&buf, "/root", rows, pairs); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(buf.String(), "\n")
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "- ") && lines[i-1] != "" && !strings.HasPrefix(lines[i-1], "- ") && !strings.HasPrefix(lines[i-1], "  ") {
			t.Errorf("list at line %d has no blank line before it: %q", i+1, lines[i-1])
		}
	}
}

func TestWriteMarkdown_PairsSectionAndDeterminism(t *testing.T) {
	pairs := []detect.ArchivePair{
		{Archive: "/x/P.tar", ArchiveAlloc: 5 << 30, Dir: "/x/P", DirAlloc: 5 << 30, Verdict: detect.PairArchiveHasMore,
			Compared: detect.CompareLibraryOriginals, ArchiveFiles: 10, ArchiveBytes: 100, DirFiles: 8, DirBytes: 90, ArchiveExtra: 2},
		{Archive: "/x/Q.zip", Dir: "/x/Q", Verdict: detect.PairUnreadable, Detail: "boom"},
	}
	render := func() string {
		var buf strings.Builder
		if err := WriteMarkdown(&buf, "/root", nil, pairs); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	out := render()
	for _, want := range []string{
		"## Archives beside extracted copies",
		"- [ ] **5.0 GiB** `/x/P.tar` is **archive_has_more**",
		"the archive lists 2 more file(s)",
		"  - boom",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if out != render() {
		t.Error("output must be deterministic")
	}
}
