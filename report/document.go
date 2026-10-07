package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
	"github.com/grokify/diskwise/reportdoc"
)

// hotspotLimit caps each hotspot table in the HTML report. The full
// list is always in the findings table below it.
const hotspotLimit = 15

// RowsFromDocument flattens a report document's findings into rows,
// ordered the way the other renderers present them.
func RowsFromDocument(d *reportdoc.Document) []Row {
	rows := make([]Row, len(d.Findings))
	for i, f := range d.Findings {
		r := Row{
			Tier:          policy.ActionClass(f.Tier),
			Kind:          entity.Kind(f.Kind),
			Path:          f.Path,
			AllocatedSize: f.AllocatedBytes,
			LogicalSize:   f.LogicalBytes,
			Confidence:    f.Confidence,
			Reason:        f.Reason,
			Detector:      f.Detector,
			PathCount:     len(f.ActionablePaths),
			Missing:       f.Missing,
		}
		for _, sc := range f.Scenarios {
			r.Scenarios = append(r.Scenarios, detect.Scenario{
				Name: sc.Name, Description: sc.Description, ReclaimableBytes: sc.ReclaimableBytes, Paths: sc.Paths,
			})
		}
		rows[i] = r
	}
	sortRows(rows)
	return rows
}

// MetaFromDocument extracts the worklist Meta (freshness, size cut,
// vanished paths) from a document.
func MetaFromDocument(d *reportdoc.Document) Meta {
	return Meta{
		ScannedAt: d.MeasuredAt, ScanStatus: d.ScanStatus, Stale: d.Stale,
		MinSize: d.Filter.MinSizeBytes, OmittedCount: d.Filter.OmittedCount, OmittedBytes: d.Filter.OmittedBytes,
		MissingCount: d.Missing.Count,
	}
}

// PairsFromDocument converts a document's pairs back to detect form.
func PairsFromDocument(d *reportdoc.Document) []detect.ArchivePair {
	if len(d.Pairs) == 0 {
		return nil
	}
	out := make([]detect.ArchivePair, len(d.Pairs))
	for i, p := range d.Pairs {
		out[i] = detect.ArchivePair{
			Archive: p.Archive, ArchiveAlloc: p.ArchiveAlloc, Dir: p.Dir, DirAlloc: p.DirAlloc,
			Compared: p.Compared, ArchiveFiles: p.ArchiveFiles, ArchiveBytes: p.ArchiveBytes,
			DirFiles: p.DirFiles, DirBytes: p.DirBytes, Verdict: detect.PairVerdict(p.Verdict),
			Detail: p.Detail, ArchiveExtra: p.ArchiveExtra,
		}
	}
	return out
}

// WriteMarkdownDocument renders d as the review worklist.
func WriteMarkdownDocument(w io.Writer, d *reportdoc.Document) error {
	return WriteMarkdown(w, d.Root, RowsFromDocument(d), PairsFromDocument(d), MetaFromDocument(d))
}

// WriteXLSXDocument renders d's findings as the spreadsheet.
func WriteXLSXDocument(w io.Writer, d *reportdoc.Document) error {
	return WriteXLSX(w, RowsFromDocument(d))
}

// WriteHTMLDocument renders d as one self-contained HTML page: notices
// about stale, partial, vanished or filtered data; tier totals; the
// known heavy locations and large unexplained directories; any archive
// comparisons; and the full sortable, filterable findings table.
// Rendering is a pure function of d: the only time shown is when the
// data was measured, taken from the document, never from the clock.
func WriteHTMLDocument(w io.Writer, d *reportdoc.Document) error {
	rows := RowsFromDocument(d)
	data := htmlData{Root: d.Root}

	for _, r := range rows {
		data.Rows = append(data.Rows, newHTMLRow(r))
	}

	// Tier pills use the document's totals, which cover every finding
	// including any the size filter left out of the table.
	tiers := make([]string, 0, len(d.Tiers))
	for t := range d.Tiers {
		tiers = append(tiers, t)
	}
	sort.Slice(tiers, func(i, j int) bool {
		return tierRank(policy.ActionClass(tiers[i])) < tierRank(policy.ActionClass(tiers[j]))
	})
	for _, t := range tiers {
		data.TierTotals = append(data.TierTotals, htmlTierTotal{
			Tier: t, Class: tierClass(policy.ActionClass(t)), Total: humanBytes(d.Tiers[t]),
		})
	}
	data.GrandTotal = humanBytes(d.ReclaimableBytes)

	data.MetaLine = fmt.Sprintf("measured %s · %d findings listed · %s reclaimable (excludes keep and unexplained)",
		d.MeasuredAt.UTC().Format("2006-01-02 15:04 UTC"), len(rows), humanBytes(d.ReclaimableBytes))

	if d.Stale {
		data.Banners = append(data.Banners, htmlBanner{"warn", "This data is more than a week old. Run `diskwise scan` to refresh it before acting."})
	}
	if d.ScanStatus == "partial" {
		data.Banners = append(data.Banners, htmlBanner{"info", "Partial scan: some directories were not readable, so totals may be low."})
	}
	if d.Missing.Count > 0 {
		data.Banners = append(data.Banners, htmlBanner{"warn", fmt.Sprintf(
			"%d finding(s) (%s) no longer exist on disk. The scan predates their removal; rescan their parent paths. They are still counted in the totals.",
			d.Missing.Count, humanBytes(d.Missing.Bytes))})
	}
	if d.Filter.OmittedCount > 0 {
		data.Banners = append(data.Banners, htmlBanner{"info", fmt.Sprintf(
			"%d smaller finding(s) totalling %s are not listed (under %s).",
			d.Filter.OmittedCount, humanBytes(d.Filter.OmittedBytes), humanBytes(d.Filter.MinSizeBytes))})
	}

	data.Known, data.Unexplained = hotspots(rows)

	for _, p := range PairsFromDocument(d) {
		summary := fmt.Sprintf("%s: archive %d files (%s), directory %d files (%s)",
			p.Compared, p.ArchiveFiles, humanBytes(p.ArchiveBytes), p.DirFiles, humanBytes(p.DirBytes))
		switch p.Verdict {
		case detect.PairSkipped, detect.PairUnreadable:
			summary = p.Detail
		default:
			if p.ArchiveExtra > 0 {
				summary += fmt.Sprintf("; the archive lists %d more file(s)", p.ArchiveExtra)
			}
		}
		data.Pairs = append(data.Pairs, htmlPair{
			ArchiveSize: humanBytes(p.ArchiveAlloc), Verdict: string(p.Verdict),
			Archive: p.Archive, Dir: p.Dir, Summary: summary,
		})
	}

	return htmlTemplate.Execute(w, data)
}

// hotspots picks, from already-sorted rows, the recognized heavy
// locations (known locations and managed bundles) and the large
// directories nothing explains.
func hotspots(rows []Row) (known, unexplained []htmlMini) {
	var knownRows, unexRows []Row
	for _, r := range rows {
		switch {
		case r.Tier == policy.Unknown:
			unexRows = append(unexRows, r)
		case r.Kind == entity.KindManagedBundle || strings.HasPrefix(r.Detector, "known-location"):
			knownRows = append(knownRows, r)
		}
	}
	mini := func(rs []Row) []htmlMini {
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].AllocatedSize > rs[j].AllocatedSize })
		if len(rs) > hotspotLimit {
			rs = rs[:hotspotLimit]
		}
		out := make([]htmlMini, len(rs))
		for i, r := range rs {
			out[i] = htmlMini{
				Size: humanBytes(r.AllocatedSize), Tier: string(r.Tier), Class: tierClass(r.Tier),
				Path: r.Path, FileURL: fileURL(r.Path), Missing: r.Missing,
			}
		}
		return out
	}
	return mini(knownRows), mini(unexRows)
}
