package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/report"
	"github.com/grokify/diskwise/service"
)

// pairsExport is the export envelope for archive pairs.
type pairsExport struct {
	Root  string
	Pairs []detect.ArchivePair
}

// exportFile is one output of `export`: a name and a lazy renderer.
type exportFile struct {
	name string
	data func() ([]byte, error)
}

func newExportCmd() *cobra.Command {
	var outDir, minSizeFlag string
	var withPairs bool

	cmd := &cobra.Command{
		Use:   "export [path]",
		Short: "Write savings, opportunities and hotspots JSON (and a review worklist) to a directory",
		Long: `Write a consistent set of files for one scanned path into --out:

  savings.json        savings by tier
  opportunities.json  findings, as the same report that "opportunities --json" prints
  hotspots.json       known locations and large unexplained directories
  review.md           checkbox worklist grouped by tier
  pairs.json          archive/directory comparisons (with --pairs)

Every file records the root it was computed for, so output from the wrong
directory is recognizable. --redact and --redact-prefix apply to all files.
Existing files are overwritten.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return fmt.Errorf("--out is required")
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

			ctx := cmd.Context()
			svc := service.New(db)
			minSize, err := parseSize(minSizeFlag)
			if err != nil {
				return err
			}
			// Run the detectors once; the report applies the size cut and
			// the savings totals use the full list.
			all, err := svc.Opportunities(ctx, service.OpportunityQuery{Path: path})
			if err != nil {
				return err
			}
			rep, err := svc.OpportunitiesReportFrom(ctx, service.OpportunityQuery{Path: path, MinSize: minSize}, all)
			if err != nil {
				return err
			}
			warnStale(cmd, rep.Freshness, rep.MissingCount, 0, rep.Root)
			savings, err := svc.SavingsFrom(ctx, path, all)
			if err != nil {
				return err
			}
			hot, err := svc.Hotspots(ctx, service.HotspotsQuery{Path: path})
			if err != nil {
				return err
			}
			var pairs []detect.ArchivePair
			if withPairs {
				if pairs, err = svc.ArchivePairs(ctx, service.ArchivePairQuery{Path: path, MinSize: 10 << 20, Progress: archiveProgress(cmd)}); err != nil {
					return err
				}
			}

			root := path
			rd, err := redactorFor(cmd)
			if err != nil {
				return err
			}
			if rd != nil {
				rep, savings, hot, pairs, root = rd.OpportunitiesReport(rep), rd.Savings(savings), rd.Hotspots(hot), rd.Pairs(pairs), rd.Path(path)
			}

			var review bytes.Buffer
			if err := report.WriteMarkdown(&review, root, report.Rows(rep.Opportunities), pairs, report.MetaFrom(rep)); err != nil {
				return err
			}

			if err := os.MkdirAll(outDir, 0o750); err != nil {
				return fmt.Errorf("create %s: %w", outDir, err)
			}
			files := []exportFile{
				{"savings.json", func() ([]byte, error) { return jsonBytes(savings) }},
				{"opportunities.json", func() ([]byte, error) { return jsonBytes(rep) }},
				{"hotspots.json", func() ([]byte, error) { return jsonBytes(hot) }},
				{"review.md", func() ([]byte, error) { return review.Bytes(), nil }},
			}
			if withPairs {
				files = append(files, exportFile{"pairs.json", func() ([]byte, error) { return jsonBytes(pairsExport{Root: root, Pairs: pairs}) }})
			}
			for _, f := range files {
				data, err := f.data()
				if err != nil {
					return fmt.Errorf("render %s: %w", f.name, err)
				}
				dest := filepath.Join(outDir, f.name)
				if err := os.WriteFile(dest, data, 0o600); err != nil {
					return fmt.Errorf("write %s: %w", dest, err)
				}
				fprintf(cmd.OutOrStdout(), "wrote %s\n", dest)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "directory to write into (created if missing; required)")
	cmd.Flags().StringVar(&minSizeFlag, "min-size", "1mb", "omit findings smaller than this from opportunities.json and review.md (0 keeps everything); the omitted count is recorded in both")
	cmd.Flags().BoolVar(&withPairs, "pairs", false, "also compare archives with extracted directories (reads archive member lists)")
	addRedactFlags(cmd)
	return cmd
}
