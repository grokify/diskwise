package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/report"
	"github.com/grokify/diskwise/service"
)

func newReviewCmd() *cobra.Command {
	var format, out, minSizeFlag string
	var withPairs bool

	cmd := &cobra.Command{
		Use:   "review [path]",
		Short: "Write a checkbox worklist of findings, grouped by tier, for human review",
		Long: `Write a Markdown worklist of findings grouped by tier, each with an
unticked checkbox, so you can tick what to remove and act on it yourself.
DiskWise never deletes anything. With --pairs, archives that sit beside an
extracted copy are included with their comparison (this reads archive
member lists and can take a while on very large archives).

The only time shown is when the data was measured (taken from the index, not
the clock), so regenerating after a rescan produces a clean diff. Findings
smaller than --min-size are left out, and the document says how many.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "md" {
				return fmt.Errorf("unknown --format %q (want md)", format)
			}
			path, err := pathArg(cmd, args)
			if err != nil {
				return err
			}
			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			svc := service.New(db)
			minSize, err := parseSize(minSizeFlag)
			if err != nil {
				return err
			}
			rep, err := svc.OpportunitiesReport(cmd.Context(), service.OpportunityQuery{Path: path, MinSize: minSize})
			if err != nil {
				return err
			}
			warnStale(cmd, rep.Freshness, rep.MissingCount, 0, rep.Root)
			var pairs []detect.ArchivePair
			if withPairs {
				if pairs, err = svc.ArchivePairs(cmd.Context(), service.ArchivePairQuery{Path: path, MinSize: 10 << 20, Progress: archiveProgress(cmd)}); err != nil {
					return err
				}
			}

			rd, err := redactorFor(cmd)
			if err != nil {
				return err
			}
			if rd != nil {
				rep, pairs, path = rd.OpportunitiesReport(rep), rd.Pairs(pairs), rd.Path(path)
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
			if err := report.WriteMarkdown(w, path, report.Rows(rep.Opportunities), pairs, report.MetaFrom(rep)); err != nil {
				return err
			}
			if out != "" {
				fprintf(cmd.ErrOrStderr(), "wrote %d findings to %s\n", len(rep.Opportunities), out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "output format: md")
	cmd.Flags().StringVar(&out, "out", "", "write to this file instead of stdout")
	cmd.Flags().StringVar(&minSizeFlag, "min-size", "1mb", "omit findings smaller than this (0 lists everything); the count omitted is stated in the document")
	cmd.Flags().BoolVar(&withPairs, "pairs", false, "include archive/extracted-directory comparisons")
	addRedactFlags(cmd)
	return cmd
}
