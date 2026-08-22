package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/grokify/diskwise/index"
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

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
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
	{"tb", 1 << 40}, {"gb", 1 << 30}, {"mb", 1 << 20}, {"kb", 1 << 10},
	{"t", 1 << 40}, {"g", 1 << 30}, {"m", 1 << 20}, {"k", 1 << 10},
	{"b", 1},
}

// parseSize parses a byte count with an optional 1024-based unit
// suffix (b, k, kb, m, mb, g, gb, t, tb; case-insensitive). An empty
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
