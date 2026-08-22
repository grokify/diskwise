package main

import (
	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/service"
)

func newHotspotsCmd() *cobra.Command {
	var minSizeFlag string
	var limit int

	cmd := &cobra.Command{
		Use:   "hotspots [path]",
		Short: "Show known heavy storage locations and large unexplained directories",
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
			minSize, err := parseSize(minSizeFlag)
			if err != nil {
				return err
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			report, err := service.New(db).Hotspots(cmd.Context(), service.HotspotsQuery{Path: path, MinSize: minSize, Limit: limit})
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return printJSON(cmd.OutOrStdout(), report)
			}

			w := cmd.OutOrStdout()
			fprintf(w, "Known locations under %s\n", report.Root)
			if len(report.Known) == 0 {
				fprintf(w, "  none found\n")
			}
			for _, f := range report.Known {
				fprintf(w, "  %-10s [%s] %s\n", humanBytes(f.AllocatedSize), f.ActionClass, f.Reason)
			}

			fprintf(w, "\nLarge unexplained directories\n")
			if len(report.Unexplained) == 0 {
				fprintf(w, "  none found\n")
			}
			for _, f := range report.Unexplained {
				fprintf(w, "  %-10s %s\n", humanBytes(f.AllocatedSize), f.Path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&minSizeFlag, "min-size", "", "hide unexplained entries below this size (default: 1gb)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum unexplained entries to show (default: 20)")
	return cmd
}
