package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/insights"
	"github.com/grokify/diskwise/insights/schema"
)

func newInsightsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Work with the insights JSON IR: a re-renderable narrative savings report",
		Long: `Insights is a JSON intermediate representation (insights.Report) for a
narrative, ranked storage-savings write-up — the kind of analysis an
LLM agent produces by reading DiskWise's raw findings (opportunities,
largest, hotspots) and recognizing patterns the deterministic
detectors can't yet, like duplicate migration backups or redundant
archive+extracted-folder pairs.

'insights schema' prints the JSON Schema contract for an agent to
fill out. 'insights render' turns a filled-in document back into
Markdown or HTML, deterministically — so producing the write-up again
later never requires redoing the analysis, only re-running render.`,
	}
	cmd.AddCommand(newInsightsSchemaCmd(), newInsightsRenderCmd())
	return cmd
}

func newInsightsSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print the insights.Report JSON Schema",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := cmd.OutOrStdout().Write(schema.JSON)
			return err
		},
	}
}

func newInsightsRenderCmd() *cobra.Command {
	var format, out string

	cmd := &cobra.Command{
		Use:   "render <insights.json>",
		Short: "Render an insights.Report JSON document as Markdown or HTML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			report, err := insights.Parse(data)
			if err != nil {
				return err
			}
			if out == "" {
				out = "diskwise-insights." + format
			}

			f, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("create %s: %w", out, err)
			}
			defer func() { _ = f.Close() }()

			switch format {
			case "md":
				err = insights.WriteMarkdown(f, report)
			case "html":
				err = insights.WriteHTML(f, report)
			default:
				return fmt.Errorf("unknown --format %q (want md or html)", format)
			}
			if err != nil {
				return fmt.Errorf("write %s: %w", format, err)
			}

			fprintf(cmd.OutOrStdout(), "wrote %d opportunities to %s\n", len(report.Opportunities), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "output format: md or html")
	cmd.Flags().StringVar(&out, "out", "", "output file path (default: diskwise-insights.<format>)")
	return cmd
}
