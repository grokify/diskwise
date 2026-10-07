package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/reportdoc"
	"github.com/grokify/diskwise/service"
)

// docSource says where a report document comes from: built from the
// index for a path, or loaded from a saved document with --from.
type docSource struct {
	from        string
	minSizeFlag string
	pairs       bool
}

// addDocFlags registers the flags every document-producing command
// shares. defaultMinSize is the --min-size default.
func addDocFlags(cmd *cobra.Command, src *docSource, defaultMinSize string) {
	cmd.Flags().StringVar(&src.from, "from", "", "render a saved report document (report.json) instead of reading the index; no path or scan is needed")
	cmd.Flags().StringVar(&src.minSizeFlag, "min-size", defaultMinSize, "omit findings smaller than this (0 lists everything); the document records what was omitted. Ignored with --from")
	cmd.Flags().BoolVar(&src.pairs, "pairs", false, "include archive/extracted-directory comparisons (reads archive member lists; can take minutes). Ignored with --from")
}

// loadDocument returns the report document the command should render:
// loaded from --from, or built from the index for the path argument. It
// warns on stderr when the data is old or points at vanished paths, then
// applies --redact / --redact-prefix, so every renderer gets the same
// already-redacted document.
func loadDocument(cmd *cobra.Command, args []string, src docSource) (*reportdoc.Document, error) {
	var doc *reportdoc.Document
	if src.from != "" {
		if len(args) > 0 {
			return nil, fmt.Errorf("give either a path or --from, not both")
		}
		data, err := os.ReadFile(src.from)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", src.from, err)
		}
		d, err := reportdoc.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", src.from, err)
		}
		doc = &d
	} else {
		path, err := pathArg(cmd, args)
		if err != nil {
			return nil, err
		}
		minSize, err := parseSize(src.minSizeFlag)
		if err != nil {
			return nil, err
		}
		dbFlag, _ := cmd.Flags().GetString("db")
		db, err := openIndexDB(dbFlag)
		if err != nil {
			return nil, err
		}
		defer func() { _ = db.Close() }()

		opts := reportdoc.BuildOptions{Path: path, MinSize: minSize, Pairs: src.pairs, PairsMinSize: 10 << 20}
		if src.pairs {
			opts.Progress = archiveProgress(cmd)
		}
		if doc, err = reportdoc.Build(cmd.Context(), service.New(db), opts); err != nil {
			return nil, err
		}
	}

	warnDocument(cmd, doc)

	rd, err := redactorFor(cmd)
	if err != nil {
		return nil, err
	}
	if rd != nil {
		doc = rd.Document(doc)
	}
	return doc, nil
}

// warnDocument writes the staleness and vanished-path warnings for a
// document. Age is judged against now, not against the Stale flag the
// document was saved with, so an old saved document still warns.
func warnDocument(cmd *cobra.Command, doc *reportdoc.Document) {
	warnStale(cmd, service.Freshness{
		ScannedAt: doc.MeasuredAt, ScanStatus: doc.ScanStatus,
		Stale: time.Since(doc.MeasuredAt) > service.StaleAfter,
	}, doc.Missing.Count, doc.Missing.Bytes, doc.Root)
}
