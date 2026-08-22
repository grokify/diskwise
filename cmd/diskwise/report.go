package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/report"
	"github.com/grokify/diskwise/service"
)

func newReportCmd() *cobra.Command {
	var format, out string

	cmd := &cobra.Command{
		Use:   "report [path]",
		Short: "Generate an HTML or XLSX report of reclaimable storage",
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
			if out == "" {
				out = "diskwise-report." + format
			}

			dbFlag, _ := cmd.Flags().GetString("db")
			db, err := openIndexDB(dbFlag)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			opps, err := service.New(db).Opportunities(cmd.Context(), service.OpportunityQuery{Path: path})
			if err != nil {
				return err
			}
			rows := report.Rows(opps)

			f, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("create %s: %w", out, err)
			}
			defer func() { _ = f.Close() }()

			switch format {
			case "html":
				err = report.WriteHTML(f, path, rows, time.Now())
			case "xlsx":
				err = report.WriteXLSX(f, rows)
			default:
				return fmt.Errorf("unknown --format %q (want html or xlsx)", format)
			}
			if err != nil {
				return fmt.Errorf("write %s report: %w", format, err)
			}

			fprintf(cmd.OutOrStdout(), "wrote %d findings to %s\n", len(rows), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "html", "report format: html or xlsx")
	cmd.Flags().StringVar(&out, "out", "", "output file path (default: diskwise-report.<format>)")
	return cmd
}
