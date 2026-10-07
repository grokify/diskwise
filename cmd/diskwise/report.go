package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/report"
	"github.com/grokify/diskwise/reportdoc"
	"github.com/grokify/diskwise/reportdoc/schema"
)

// reportFormats maps --format to the file extension and renderer.
var reportFormats = map[string]struct {
	ext    string
	render func(io.Writer, *reportdoc.Document) error
}{
	"html": {"html", report.WriteHTMLDocument},
	"xlsx": {"xlsx", report.WriteXLSXDocument},
	"md":   {"md", report.WriteMarkdownDocument},
	"json": {"json", func(w io.Writer, d *reportdoc.Document) error {
		data, err := reportdoc.Marshal(*d)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}},
}

func newReportCmd() *cobra.Command {
	var format, out string
	var printSchema bool
	var src docSource

	cmd := &cobra.Command{
		Use:   "report [path]",
		Short: "Write a report (HTML, XLSX, Markdown or JSON) of reclaimable storage",
		Long: `Write one report document in the format you choose.

With a path, the document is built from the index. With --from, a saved
document (the report.json that "export" or "report --format json" writes) is
rendered instead, with no index and no scan, so the same findings can be
rendered on another machine or after redaction. Rendering is deterministic:
the same document always gives the same output.

Formats: html (self-contained, sortable and filterable, with hotspot and
archive-pair sections), xlsx (findings), md (review checklist), json (the
document itself; "report --schema" prints its JSON Schema).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if printSchema {
				_, err := cmd.OutOrStdout().Write(schema.JSON)
				return err
			}
			f, ok := reportFormats[format]
			if !ok {
				return fmt.Errorf("unknown --format %q (want html, xlsx, md or json)", format)
			}
			doc, err := loadDocument(cmd, args, src)
			if err != nil {
				return err
			}
			if out == "" {
				out = "diskwise-report." + f.ext
			}

			// Render fully before touching the file, so a render failure
			// never leaves a truncated report behind.
			var buf bytes.Buffer
			if err := f.render(&buf, doc); err != nil {
				return fmt.Errorf("render %s: %w", format, err)
			}
			if err := os.WriteFile(out, buf.Bytes(), 0o600); err != nil {
				return fmt.Errorf("write %s: %w", out, err)
			}
			fprintf(cmd.OutOrStdout(), "wrote %d findings to %s\n", len(doc.Findings), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "html", "output format: html, xlsx, md or json")
	cmd.Flags().StringVar(&out, "out", "", "output file (default: diskwise-report.<format>)")
	cmd.Flags().BoolVar(&printSchema, "schema", false, "print the JSON Schema of the report document and exit")
	addDocFlags(cmd, &src, "1mb")
	addRedactFlags(cmd)
	return cmd
}
