package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var outDir string
	var src docSource

	cmd := &cobra.Command{
		Use:   "export [path]",
		Short: "Write the report document and its renderings to a directory",
		Long: `Build one report document for a scanned path and write it, with its
renderings, into --out:

  report.json   the report document: everything below is rendered from it
  report.html   self-contained HTML (hotspots, archive pairs, all findings)
  review.md     checkbox worklist grouped by tier

report.json records the root, when it was measured, the tier totals, what
was filtered out or has vanished from disk, the findings, and (with --pairs)
archive comparisons. Render it again any time, on any machine, with
"diskwise report --from report.json --format html|xlsx|md".

--redact and --redact-prefix apply to all three files. Existing files are
overwritten.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return fmt.Errorf("--out is required")
			}
			src.from = "" // export always builds from the index
			doc, err := loadDocument(cmd, args, src)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(outDir, 0o750); err != nil {
				return fmt.Errorf("create %s: %w", outDir, err)
			}
			// Render everything before writing anything, so a failure
			// cannot leave a half-updated set of files.
			outputs := []struct{ name, format string }{
				{"report.json", "json"}, {"report.html", "html"}, {"review.md", "md"},
			}
			rendered := make([][]byte, len(outputs))
			for i, o := range outputs {
				var buf bytes.Buffer
				if err := reportFormats[o.format].render(&buf, doc); err != nil {
					return fmt.Errorf("render %s: %w", o.name, err)
				}
				rendered[i] = buf.Bytes()
			}
			for i, o := range outputs {
				dest := filepath.Join(outDir, o.name)
				if err := os.WriteFile(dest, rendered[i], 0o600); err != nil {
					return fmt.Errorf("write %s: %w", dest, err)
				}
				fprintf(cmd.OutOrStdout(), "wrote %s\n", dest)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "directory to write into (created if missing; required)")
	addDocFlags(cmd, &src, "1mb")
	_ = cmd.Flags().MarkHidden("from") // export builds from the index; use `report --from` to re-render
	addRedactFlags(cmd)
	return cmd
}
