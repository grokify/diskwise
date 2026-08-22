package main

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/scan"
	"github.com/grokify/diskwise/service"
)

func newScanCmd() *cobra.Command {
	var depth, workers int
	var crossDevice bool

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan a directory and record its storage usage in the index",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := "."
			if len(args) == 1 {
				raw = args[0]
			}
			root, err := resolvePath(raw)
			if err != nil {
				return err
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			stats := &scan.Stats{}
			stop := startProgress(cmd.ErrOrStderr(), stats)
			result, err := service.New(db).Scan(cmd.Context(), service.ScanRequest{
				Root: root, Depth: depth, Workers: workers, CrossDevice: crossDevice, Stats: stats,
			})
			stop()
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), result)
			}
			printScanResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 0, "cap descent to this many levels (0 = unlimited, a full scan)")
	cmd.Flags().IntVar(&workers, "workers", 0, "concurrent workers (0 = GOMAXPROCS)")
	cmd.Flags().BoolVar(&crossDevice, "cross-device", false, "descend into directories on other mounted volumes")
	return cmd
}

func printScanResult(w io.Writer, result *service.ScanResult) {
	fprintf(w, "%s\n", result.Run.Root)
	fprintf(w, "  status:  %s\n", result.Run.Status)
	if result.Run.FinishedAt != nil {
		fprintf(w, "  elapsed: %s\n", result.Run.FinishedAt.Sub(result.Run.StartedAt).Round(1e6))
	}
	fprintf(w, "  dirs:    %d\n", result.Stats.DirsScanned)
	fprintf(w, "  files:   %d\n", result.Stats.FilesScanned)
	if result.Stats.SymlinksScanned > 0 {
		fprintf(w, "  symlinks: %d\n", result.Stats.SymlinksScanned)
	}
	if result.Stats.DeniedPaths > 0 {
		fprintf(w, "  denied:  %d (inaccessible; run with elevated permissions to include them)\n", result.Stats.DeniedPaths)
	}
	if result.Stats.CrossDeviceSkipped > 0 {
		fprintf(w, "  skipped: %d other-volume directories (use --cross-device to include them)\n", result.Stats.CrossDeviceSkipped)
	}
	if result.Stats.DuplicateFiles > 0 {
		fprintf(w, "  dedup:   %d hard-linked files, %s not double-counted\n", result.Stats.DuplicateFiles, humanBytes(result.Stats.DuplicateBytesSaved))
	}
}
