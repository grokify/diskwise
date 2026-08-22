package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// NodeRow is one persisted node, as read back from the index.
type NodeRow struct {
	ID                   int64
	ScanRunID            int64
	ParentID             sql.NullInt64
	Path                 string
	Name                 string
	Ext                  string
	Kind                 Kind
	LogicalSize          int64
	AllocatedSize        int64
	SubtreeLogicalSize   int64
	SubtreeAllocatedSize int64
	State                SubtreeState
	Duplicate            bool
	ModTime              time.Time
}

const nodeRowColumns = `id, scan_run_id, parent_id, path, name, ext, kind, logical_size, allocated_size, subtree_logical, subtree_allocated, subtree_state, duplicate, mtime`

func scanNodeRow(scanner interface{ Scan(...any) error }) (NodeRow, error) {
	var n NodeRow
	var kind, state string
	var mtime int64
	if err := scanner.Scan(&n.ID, &n.ScanRunID, &n.ParentID, &n.Path, &n.Name, &n.Ext, &kind,
		&n.LogicalSize, &n.AllocatedSize, &n.SubtreeLogicalSize, &n.SubtreeAllocatedSize,
		&state, &n.Duplicate, &mtime); err != nil {
		return NodeRow{}, err
	}
	n.Kind = Kind(kind)
	n.State = SubtreeState(state)
	n.ModTime = time.Unix(mtime, 0).UTC()
	return n, nil
}

// LogicalSizeFor returns the row's logical size measure: subtree total
// for directories, its own size otherwise.
func (n NodeRow) LogicalSizeFor() int64 {
	if n.Kind == KindDir {
		return n.SubtreeLogicalSize
	}
	return n.LogicalSize
}

// AllocatedSizeFor returns the row's allocated size measure: subtree
// total for directories, its own size otherwise.
func (n NodeRow) AllocatedSizeFor() int64 {
	if n.Kind == KindDir {
		return n.SubtreeAllocatedSize
	}
	return n.AllocatedSize
}

// NodeByPath returns the row for the given path, or (NodeRow{}, false, nil) if not indexed.
func (db *DB) NodeByPath(ctx context.Context, path string) (NodeRow, bool, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT `+nodeRowColumns+` FROM nodes WHERE path = ?`, path)
	n, err := scanNodeRow(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return NodeRow{}, false, nil
	case err != nil:
		return NodeRow{}, false, fmt.Errorf("index: node by path %s: %w", path, err)
	default:
		return n, true, nil
	}
}

// Children returns the direct children of the node at parentPath,
// ordered by allocated subtree size descending (files use their own
// allocated size).
func (db *DB) Children(ctx context.Context, parentPath string) ([]NodeRow, error) {
	parent, ok, err := db.NodeByPath(ctx, parentPath)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("index: children of %s: not indexed", parentPath)
	}

	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+nodeRowColumns+` FROM nodes WHERE parent_id = ?
		 ORDER BY MAX(subtree_allocated, allocated_size) DESC`, parent.ID)
	if err != nil {
		return nil, fmt.Errorf("index: children of %s: %w", parentPath, err)
	}
	defer func() { _ = rows.Close() }()

	var out []NodeRow
	for rows.Next() {
		n, err := scanNodeRow(rows)
		if err != nil {
			return nil, fmt.Errorf("index: children of %s: %w", parentPath, err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CountNodes returns the total number of indexed nodes, primarily for
// tests and diagnostics.
func (db *DB) CountNodes(ctx context.Context) (int64, error) {
	var n int64
	row := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM nodes`)
	if err := row.Scan(&n); err != nil {
		return 0, fmt.Errorf("index: count nodes: %w", err)
	}
	return n, nil
}

// Largest returns the top limit nodes under rootPath (rootPath itself
// excluded) by their size measure — subtree_allocated for directories,
// allocated_size for files — optionally restricted to one kind.
func (db *DB) Largest(ctx context.Context, rootPath string, limit int, kind Kind) ([]NodeRow, error) {
	query := `SELECT ` + nodeRowColumns + ` FROM nodes WHERE (path = ? OR path LIKE ?) AND path != ?`
	args := []any{rootPath, rootPath + "/%", rootPath}
	if kind != "" {
		query += ` AND kind = ?`
		args = append(args, string(kind))
	}
	query += ` ORDER BY MAX(subtree_allocated, allocated_size) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("index: largest under %s: %w", rootPath, err)
	}
	defer func() { _ = rows.Close() }()

	var out []NodeRow
	for rows.Next() {
		n, err := scanNodeRow(rows)
		if err != nil {
			return nil, fmt.Errorf("index: largest under %s: %w", rootPath, err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// FilesByExtensions returns file nodes under rootPath whose stored
// extension matches one of exts (case-insensitive), sorted by
// allocated size descending. The stored extension is only the final
// path component (filepath.Ext), so compound extensions like
// ".tar.gz" are matched via their trailing ".gz" — callers needing
// the true compound suffix should re-check row.Name themselves.
func (db *DB) FilesByExtensions(ctx context.Context, rootPath string, exts []string) ([]NodeRow, error) {
	if len(exts) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(exts))
	args := make([]any, 0, len(exts)+2)
	args = append(args, rootPath, rootPath+"/%")
	for i, e := range exts {
		placeholders[i] = "?"
		args = append(args, strings.ToLower(e))
	}

	// The concatenated parts are nodeRowColumns (a package constant)
	// and a fixed "?" placeholder string sized to len(exts) — every
	// actual value flows through args via QueryContext's
	// parameterization, never through this string.
	//nolint:gosec // G202: no untrusted input reaches the query text
	query := `SELECT ` + nodeRowColumns + ` FROM nodes
		WHERE (path = ? OR path LIKE ?) AND kind = 'file' AND lower(ext) IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY allocated_size DESC`

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("index: files by extension under %s: %w", rootPath, err)
	}
	defer func() { _ = rows.Close() }()

	var out []NodeRow
	for rows.Next() {
		n, err := scanNodeRow(rows)
		if err != nil {
			return nil, fmt.Errorf("index: files by extension under %s: %w", rootPath, err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ScanRunByID returns the scan_runs row for id.
func (db *DB) ScanRunByID(ctx context.Context, id int64) (ScanRun, error) {
	var run ScanRun
	var startedAt int64
	var finishedAt sql.NullInt64
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, root, started_at, finished_at, status FROM scan_runs WHERE id = ?`, id)
	if err := row.Scan(&run.ID, &run.Root, &startedAt, &finishedAt, &run.Status); err != nil {
		return ScanRun{}, fmt.Errorf("index: scan run %d: %w", id, err)
	}
	run.StartedAt = time.Unix(startedAt, 0).UTC()
	if finishedAt.Valid {
		t := time.Unix(finishedAt.Int64, 0).UTC()
		run.FinishedAt = &t
	}
	return run, nil
}
