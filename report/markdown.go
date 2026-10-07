package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/service"
)

// tierHeading is the review-worklist heading for each tier, and
// tierBlurb says what ticking an item means there.
var tierHeading = map[policy.ActionClass]string{
	policy.SafeDelete:       "Safe to delete",
	policy.LikelySafe:       "Likely safe",
	policy.BackupThenDelete: "Back up, then delete",
	policy.Review:           "Review",
	policy.Keep:             "Keep (managed or protected)",
	policy.Unknown:          "Unexplained (not a recommendation)",
}

var tierBlurb = map[policy.ActionClass]string{
	policy.SafeDelete:       "Regenerates on demand; removing it costs only rebuild time.",
	policy.LikelySafe:       "Evidence suggests it is no longer needed; skim before removing.",
	policy.BackupThenDelete: "Live data. Back it up first.",
	policy.Review:           "No evidence either way. Decide per item.",
	policy.Keep:             "Listed so the bytes are explained; not for removal here.",
	policy.Unknown:          "Large directories no detector explains. Drill in with `diskwise tree`.",
}

// reclaimable reports whether a tier counts toward the savings total.
func reclaimable(t policy.ActionClass) bool { return t != policy.Keep && t != policy.Unknown }

// Meta carries what a reader needs to judge a worklist: when the data
// was measured, and what was left out or has since disappeared. The
// zero value omits all of it.
type Meta struct {
	ScannedAt    time.Time
	ScanStatus   string
	Stale        bool
	MinSize      int64
	OmittedCount int
	OmittedBytes int64
	MissingCount int
}

// MetaFrom extracts a worklist's Meta from an opportunities report.
func MetaFrom(rep *service.OpportunitiesReport) Meta {
	return Meta{
		ScannedAt: rep.ScannedAt, ScanStatus: rep.ScanStatus, Stale: rep.Stale,
		MinSize: rep.MinSize, OmittedCount: rep.OmittedCount, OmittedBytes: rep.OmittedBytes,
		MissingCount: rep.MissingCount,
	}
}

// WriteMarkdown renders rows (and optionally archive/directory pairs)
// as a checkbox worklist grouped by tier, for human review. Tick an
// item to mark it for removal; DiskWise itself never acts on the file.
// Rendering is a pure function of its inputs. The only time shown is when
// the data was measured (from the index, not the clock), so the same
// findings always produce the same document and diffs stay quiet.
// Every list is preceded by a blank line, per the repo's Markdown rules.
func WriteMarkdown(w io.Writer, root string, rows []Row, pairs []detect.ArchivePair, meta Meta) error {
	var b strings.Builder
	var total int64
	for _, r := range rows {
		if reclaimable(r.Tier) {
			total += r.AllocatedSize
		}
	}

	fmt.Fprintf(&b, "# DiskWise review: %s\n\n", root)
	fmt.Fprintf(&b, "Potential savings listed: **%s** (excludes keep and unexplained). Nothing here has been changed.\n\n", humanBytes(total))

	if !meta.ScannedAt.IsZero() {
		fmt.Fprintf(&b, "Measured %s", meta.ScannedAt.UTC().Format("2006-01-02 15:04 UTC"))
		if meta.ScanStatus == "partial" {
			b.WriteString(" (partial scan: some directories were not readable)")
		}
		b.WriteString(".\n\n")
	}
	if meta.Stale {
		b.WriteString("> **This data is more than a week old.** Run `diskwise scan` to refresh it before acting.\n\n")
	}
	if meta.MissingCount > 0 {
		fmt.Fprintf(&b, "> **%d finding(s) no longer exist on disk** (marked below). The index predates their removal; `diskwise rescan` their parent paths.\n\n", meta.MissingCount)
	}
	if meta.OmittedCount > 0 {
		fmt.Fprintf(&b, "%d smaller finding(s) totalling %s are omitted (under %s). Re-run with `--min-size 0` to list them.\n\n",
			meta.OmittedCount, humanBytes(meta.OmittedBytes), humanBytes(meta.MinSize))
	}

	byTier := make(map[policy.ActionClass][]Row)
	var order []policy.ActionClass
	for _, r := range rows { // Rows() already sorts by tier, then size
		if _, ok := byTier[r.Tier]; !ok {
			order = append(order, r.Tier)
		}
		byTier[r.Tier] = append(byTier[r.Tier], r)
	}

	for _, tier := range order {
		var tierBytes int64
		for _, r := range byTier[tier] {
			tierBytes += r.AllocatedSize
		}
		heading, ok := tierHeading[tier]
		if !ok {
			heading = string(tier)
		}
		fmt.Fprintf(&b, "## %s: %s\n\n", heading, humanBytes(tierBytes))
		if blurb, ok := tierBlurb[tier]; ok {
			fmt.Fprintf(&b, "%s\n\n", blurb)
		}
		for _, r := range byTier[tier] {
			gone := ""
			if r.Missing {
				gone = " _(no longer exists)_"
			}
			fmt.Fprintf(&b, "- [ ] **%s** %s `%s`%s\n", humanBytes(r.AllocatedSize), r.Kind, mdCode(r.Path), gone)
			if r.Reason != "" {
				fmt.Fprintf(&b, "  - %s\n", r.Reason)
			}
			for _, sc := range r.Scenarios {
				fmt.Fprintf(&b, "  - _%s_ (%s): %s\n", sc.Name, humanBytes(sc.ReclaimableBytes), sc.Description)
			}
		}
		b.WriteString("\n")
	}

	if len(pairs) > 0 {
		b.WriteString("## Archives beside extracted copies\n\n")
		b.WriteString("Counts and bytes only; contents are not hashed. Verify before removing either side.\n\n")
		for _, p := range pairs {
			fmt.Fprintf(&b, "- [ ] **%s** `%s` is **%s**\n", humanBytes(p.ArchiveAlloc), mdCode(p.Archive), p.Verdict)
			fmt.Fprintf(&b, "  - beside `%s` (%s)\n", mdCode(p.Dir), humanBytes(p.DirAlloc))
			if p.Verdict == detect.PairSkipped || p.Verdict == detect.PairUnreadable {
				fmt.Fprintf(&b, "  - %s\n", p.Detail)
				continue
			}
			fmt.Fprintf(&b, "  - compared %s: archive %d files (%s), directory %d files (%s)\n",
				p.Compared, p.ArchiveFiles, humanBytes(p.ArchiveBytes), p.DirFiles, humanBytes(p.DirBytes))
			if p.ArchiveExtra > 0 {
				fmt.Fprintf(&b, "  - the archive lists %d more file(s) than the directory holds\n", p.ArchiveExtra)
			}
		}
		b.WriteString("\n")
	}

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("report: write markdown: %w", err)
	}
	return nil
}

// mdCode makes s safe inside a single-backtick code span.
func mdCode(s string) string { return strings.ReplaceAll(s, "`", "'") }
