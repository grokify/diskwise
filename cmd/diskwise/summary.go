package main

import (
	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/service"
)

func newSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary [path]",
		Short: "Show what's known about a previously scanned path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := "."
			if len(args) == 1 {
				raw = args[0]
			}
			path, err := resolvePath(raw)
			if err != nil {
				return err
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			summary, err := service.New(db).Summary(cmd.Context(), path)
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), summary)
			}

			w := cmd.OutOrStdout()
			fprintf(w, "%s\n", summary.Path)
			fprintf(w, "  logical:   %s\n", humanBytes(summary.LogicalSize))
			fprintf(w, "  allocated: %s\n", humanBytes(summary.AllocatedSize))
			fprintf(w, "  state:     %s\n", summary.State)
			fprintf(w, "  last scan: %s (%s)\n", summary.ScanRun.StartedAt.Local().Format("2006-01-02 15:04:05"), summary.ScanRun.Status)
			if len(summary.KnownLocations) > 0 {
				fprintf(w, "\nKnown locations\n")
				for _, f := range summary.KnownLocations {
					fprintf(w, "  %-10s [%s] %s\n", humanBytes(f.AllocatedSize), f.ActionClass, f.Reason)
				}
			}
			if summary.Volume != nil {
				fprintf(w, "\n%s (%s)\n", summary.Volume.MountPoint, summary.Volume.FSType)
				fprintf(w, "  capacity:  %s\n", humanBytes(summary.Volume.CapacityBytes))
				fprintf(w, "  used:      %s\n", humanBytes(summary.Volume.UsedBytes))
				fprintf(w, "  available: %s\n", humanBytes(summary.Volume.AvailableBytes))
			}
			return nil
		},
	}
	return cmd
}
