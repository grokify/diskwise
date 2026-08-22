package insights

import (
	"fmt"
	"io"
	"strings"
)

// WriteMarkdown renders r as a Markdown document: a summary, an
// overview table, and one detail section per opportunity. Rendering
// is a pure function of r — the same Report always produces the same
// Markdown, so regenerating the write-up later never requires
// re-running the analysis that produced r.
func WriteMarkdown(w io.Writer, r Report) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# DiskWise savings report — %s\n\n", r.Root)
	fmt.Fprintf(&b, "_generated %s", r.GeneratedAt.Format("2006-01-02 15:04:05 MST"))
	if r.UsedBytes > 0 && r.CapacityBytes > 0 {
		fmt.Fprintf(&b, " · %s used of %s", humanBytes(r.UsedBytes), humanBytes(r.CapacityBytes))
	}
	b.WriteString("_\n\n")

	b.WriteString(r.Summary)
	b.WriteString("\n\n")

	if len(r.Opportunities) == 0 {
		if err := writeString(w, b.String()); err != nil {
			return err
		}
		return nil
	}

	b.WriteString("| Rank | Title | Category | Est. Reclaim | Directories |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, o := range r.Opportunities {
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %d |\n",
			o.Rank, mdEscape(o.Title), o.Category, humanBytes(o.EstimatedBytes), len(o.Directories))
	}
	b.WriteString("\n")

	for _, o := range r.Opportunities {
		writeOpportunityMarkdown(&b, o)
	}

	return writeString(w, b.String())
}

func writeOpportunityMarkdown(b *strings.Builder, o Opportunity) {
	fmt.Fprintf(b, "## %d. %s\n\n", o.Rank, o.Title)
	fmt.Fprintf(b, "**Category:** %s · **Confidence:** %s · **Estimated reclaim:** %s\n\n",
		o.Category, o.Confidence, humanBytes(o.EstimatedBytes))

	b.WriteString("**Directories:**\n\n")
	for _, d := range o.Directories {
		fmt.Fprintf(b, "- `%s`\n", d)
	}
	b.WriteString("\n")

	b.WriteString(o.Description)
	b.WriteString("\n\n")

	if len(o.Evidence) > 0 {
		b.WriteString("**Evidence:**\n\n")
		for _, e := range o.Evidence {
			fmt.Fprintf(b, "- %s\n", e)
		}
		b.WriteString("\n")
	}

	if len(o.VerificationSteps) > 0 {
		b.WriteString("**Verify before acting:**\n\n")
		for i, v := range o.VerificationSteps {
			fmt.Fprintf(b, "%d. %s\n", i+1, v)
		}
		b.WriteString("\n")
	}

	for _, a := range o.Actions {
		writeActionMarkdown(b, a)
	}
}

func writeActionMarkdown(b *strings.Builder, a Action) {
	label := string(a.Method)
	if a.Target != "" {
		label = fmt.Sprintf("%s (%s)", a.Target, a.Method)
	}
	fmt.Fprintf(b, "**Action — %s:**\n\n", label)

	for _, d := range a.Directories {
		fmt.Fprintf(b, "- `%s`\n", d)
	}
	if len(a.Pairs) > 0 {
		b.WriteString("\n| Kept | Redundant | Notes |\n|---|---|---|\n")
		for _, p := range a.Pairs {
			fmt.Fprintf(b, "| `%s` | `%s` | %s |\n", p.Kept, p.Redundant, mdEscape(p.Notes))
		}
		b.WriteString("\n")
	}
	if a.Command != "" {
		fmt.Fprintf(b, "- `%s`\n", a.Command)
	}
	for i, s := range a.Steps {
		fmt.Fprintf(b, "%d. %s\n", i+1, s)
	}
	if a.Notes != "" {
		fmt.Fprintf(b, "\n%s\n", a.Notes)
	}
	b.WriteString("\n")
}

func mdEscape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func writeString(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
