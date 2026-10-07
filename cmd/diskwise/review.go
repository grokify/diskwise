package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/report"
)

func newReviewCmd() *cobra.Command {
	var format, out string
	var src docSource

	cmd := &cobra.Command{
		Use:   "review [path]",
		Short: "Write a checkbox worklist of findings, grouped by tier, for human review",
		Long: `Write a Markdown worklist of findings grouped by tier, each with an
unticked checkbox, so you can tick what to remove and act on it yourself.
DiskWise never deletes anything. With --pairs, archives that sit beside an
extracted copy are included with their comparison (this reads archive
member lists and can take a while on very large archives).

With --from, a saved report document is rendered instead of reading the
index. The only time shown is when the data was measured (taken from the
index or the document, not the clock), so regenerating after a rescan
produces a clean diff. Findings smaller than --min-size are left out, and
the document says how many.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "md" {
				return fmt.Errorf("unknown --format %q (want md)", format)
			}
			doc, err := loadDocument(cmd, args, src)
			if err != nil {
				return err
			}

			var w io.Writer = cmd.OutOrStdout()
			if out != "" {
				f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // G304: the user chose this output path
				if err != nil {
					return fmt.Errorf("create %s: %w", out, err)
				}
				defer func() { _ = f.Close() }()
				w = f
			}
			if err := report.WriteMarkdownDocument(w, doc); err != nil {
				return err
			}
			if out != "" {
				fprintf(cmd.ErrOrStderr(), "wrote %d findings to %s\n", len(doc.Findings), out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "output format: md")
	cmd.Flags().StringVar(&out, "out", "", "write to this file instead of stdout")
	addDocFlags(cmd, &src, "1mb")
	addRedactFlags(cmd)
	return cmd
}
