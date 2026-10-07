package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/grokify/diskwise/index"
	"github.com/grokify/diskwise/redact"
	"github.com/grokify/diskwise/service"
)

// defaultDBPath returns the standard per-user location for the
// DiskWise index.
func defaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "DiskWise", "index.db"), nil
}

// openIndexDB opens the index at the --db flag's path, creating its
// parent directory and applying the schema if needed.
func openIndexDB(dbFlag string) (*index.DB, error) {
	path := dbFlag
	if path == "" {
		var err error
		path, err = defaultDBPath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create index directory: %w", err)
	}
	return index.Open(path)
}

// resolvePath makes raw absolute, matching how scan/rescan record
// paths in the index — every command taking a path argument must
// resolve it the same way for lookups to match what was scanned.
func resolvePath(raw string) (string, error) {
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", raw, err)
	}
	return abs, nil
}

// pathArg returns the absolute path a command should operate on: its
// optional path argument, or the current directory when none is given.
// The implicit default is announced on stderr, because a query that
// silently runs against the wrong directory looks identical to one
// that found nothing (stdout, and so --json output, is unaffected).
func pathArg(cmd *cobra.Command, args []string) (string, error) {
	raw := "."
	if len(args) == 1 {
		raw = args[0]
	}
	abs, err := resolvePath(raw)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		fprintf(cmd.ErrOrStderr(), "diskwise: no path given; using current directory %s\n", abs)
	}
	return abs, nil
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// jsonBytes renders v exactly as printJSON would write it.
func jsonBytes(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := printJSON(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fprintf writes to w, ignoring the error: a failed write to
// stdout/stderr (e.g. a broken pipe) leaves nothing more useful for
// the CLI's cosmetic output to do about it.
func fprintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// humanBytes formats b using 1024-based units, matching the du/ncdu
// convention DiskWise users are used to.
func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// sizeSuffixes is checked longest-suffix-first so "10mb" matches "mb"
// before the trailing "b" of any other unit is considered.
var sizeSuffixes = []struct {
	suffix string
	mult   int64
}{
	{"tib", 1 << 40}, {"gib", 1 << 30}, {"mib", 1 << 20}, {"kib", 1 << 10},
	{"tb", 1 << 40}, {"gb", 1 << 30}, {"mb", 1 << 20}, {"kb", 1 << 10},
	{"t", 1 << 40}, {"g", 1 << 30}, {"m", 1 << 20}, {"k", 1 << 10},
	{"b", 1},
}

// parseSize parses a byte count with an optional 1024-based unit
// suffix (b, k, kb, kib, m, mb, mib, g, gb, gib, t, tb, tib; case-insensitive). An empty
// string parses as 0.
func parseSize(s string) (int64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, nil
	}
	lower := strings.ToLower(trimmed)
	for _, u := range sizeSuffixes {
		numPart, ok := strings.CutSuffix(lower, u.suffix)
		if !ok {
			continue
		}
		numPart = strings.TrimSpace(numPart)
		if numPart == "" {
			continue
		}
		f, err := strconv.ParseFloat(numPart, 64)
		if err != nil {
			continue
		}
		return int64(f * float64(u.mult)), nil
	}
	n, err := strconv.ParseInt(lower, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n, nil
}

// addRedactFlags registers --redact and --redact-prefix on a command
// that prints paths.
func addRedactFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("redact", false, "abbreviate the home directory to ~ in output (for sharing)")
	cmd.Flags().StringSlice("redact-prefix", nil, "fully redact every path under this prefix (repeatable; implies --redact)")
}

// redactorFor returns the Redactor the command's flags ask for, or nil
// when output should be left as-is.
func redactorFor(cmd *cobra.Command) (*redact.Redactor, error) {
	on, _ := cmd.Flags().GetBool("redact")
	prefixes, _ := cmd.Flags().GetStringSlice("redact-prefix")
	if !on && len(prefixes) == 0 {
		return nil, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determine home directory for --redact: %w", err)
	}
	return redact.New(home, prefixes), nil
}

// archiveProgress returns a callback that announces each archive being
// read on stderr, so long comparisons are not silent. stdout (and so
// --json output) is unaffected.
func archiveProgress(cmd *cobra.Command) func(string, int64) {
	return func(archive string, size int64) {
		fprintf(cmd.ErrOrStderr(), "diskwise: reading %s (%s)\n", archive, humanBytes(size))
	}
}

// humanAge renders how long ago something was, coarsely ("3 days ago").
func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// printMeasured writes the "Measured ..." line that tells a reader how
// old the numbers below it are.
func printMeasured(w io.Writer, f service.Freshness) {
	if f.ScannedAt.IsZero() {
		return
	}
	fprintf(w, "Measured %s (%s)", f.ScannedAt.Format("2006-01-02 15:04 UTC"), humanAge(time.Since(f.ScannedAt)))
	if f.ScanStatus == "partial" {
		fprintf(w, "; partial scan")
	}
	fprintf(w, "\n")
}

// warnStale writes warnings to stderr (so stdout and --json stay clean)
// when the data is old or points at paths that have since disappeared.
func warnStale(cmd *cobra.Command, f service.Freshness, missing int, missingBytes int64, root string) {
	w := cmd.ErrOrStderr()
	if f.Stale {
		fprintf(w, "diskwise: warning: this scan is %s; run `diskwise scan %s` to refresh it\n", humanAge(time.Since(f.ScannedAt)), root)
	}
	if missing > 0 {
		detail := ""
		if missingBytes > 0 {
			detail = fmt.Sprintf(" (%s)", humanBytes(missingBytes))
		}
		fprintf(w, "diskwise: warning: %d finding(s)%s point at paths that no longer exist; the index predates their removal; run `diskwise rescan` on their parent paths\n", missing, detail)
	}
}
