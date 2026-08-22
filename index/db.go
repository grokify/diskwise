package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/grokify/diskwise/platform"
	"github.com/grokify/diskwise/scan"
)

// DB is a handle to a DiskWise SQLite index. The index is disposable
// and rebuildable from a fresh scan; the schema is applied on Open.
type DB struct {
	conn *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path and
// applies the schema. Use ":memory:" for an ephemeral in-process
// database, primarily for tests.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("index: open %s: %w", path, err)
	}
	// A single SQLite connection is a straightforward way to guarantee
	// writer serialization without pool-level lock contention; the
	// index is written by one scan at a time.
	conn.SetMaxOpenConns(1)

	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON"} {
		if _, err := conn.Exec(pragma); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("index: %s: %w", pragma, err)
		}
	}
	if _, err := conn.Exec(schemaSQL); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("index: apply schema: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// IngestScan persists a freshly walked tree as a new scan run, along
// with the volume it was measured on if known. It is the batched-write
// path a full `diskwise scan` uses: one transaction, one row per node.
func (db *DB) IngestScan(ctx context.Context, tree *scan.Node, vol *platform.VolumeStats) (*ScanRun, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("index: begin ingest: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	startedAt := time.Now().UTC()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO scan_runs (root, started_at, status) VALUES (?, ?, 'running')`,
		tree.Path, startedAt.Unix())
	if err != nil {
		return nil, fmt.Errorf("index: insert scan_runs: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("index: scan_runs last insert id: %w", err)
	}

	var volID sql.NullInt64
	if vol != nil {
		id, err := upsertVolume(ctx, tx, *vol)
		if err != nil {
			return nil, err
		}
		volID = sql.NullInt64{Int64: id, Valid: true}
	}

	if err := insertTree(ctx, tx, runID, volID, sql.NullInt64{}, tree); err != nil {
		return nil, err
	}

	status := statusFor(tree.State)
	finishedAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx,
		`UPDATE scan_runs SET finished_at = ?, status = ? WHERE id = ?`,
		finishedAt.Unix(), status, runID); err != nil {
		return nil, fmt.Errorf("index: finish scan_runs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("index: commit ingest: %w", err)
	}

	return &ScanRun{ID: runID, Root: tree.Path, StartedAt: startedAt, FinishedAt: &finishedAt, Status: status}, nil
}

// Rescan re-walks path and replaces its subtree in the index,
// recomputing the byte-delta up through its recorded ancestors. If
// path was not previously indexed, it is ingested as a new,
// unparented scan run instead.
func (db *DB) Rescan(ctx context.Context, path string, opts scan.Options) (*scan.Node, *scan.Stats, *ScanRun, error) {
	tree, stats, err := scan.Walk(ctx, path, opts)
	if err != nil {
		return nil, nil, nil, err
	}

	parentID, oldLogical, oldAllocated, existed, err := db.lookupNode(ctx, path)
	if err != nil {
		return nil, nil, nil, err
	}

	if !existed {
		run, err := db.IngestScan(ctx, tree, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		return tree, stats, run, nil
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("index: begin rescan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	startedAt := time.Now().UTC()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO scan_runs (root, started_at, status) VALUES (?, ?, 'running')`,
		tree.Path, startedAt.Unix())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("index: insert rescan run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return nil, nil, nil, err
	}

	if err := deleteSubtree(ctx, tx, path); err != nil {
		return nil, nil, nil, err
	}

	if err := insertTree(ctx, tx, runID, sql.NullInt64{}, parentID, tree); err != nil {
		return nil, nil, nil, err
	}

	if parentID.Valid {
		deltaLogical := tree.SubtreeLogicalSize - oldLogical
		deltaAllocated := tree.SubtreeAllocatedSize - oldAllocated
		if err := propagateDelta(ctx, tx, parentID.Int64, deltaLogical, deltaAllocated); err != nil {
			return nil, nil, nil, err
		}
	}

	status := statusFor(tree.State)
	finishedAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx,
		`UPDATE scan_runs SET finished_at = ?, status = ? WHERE id = ?`,
		finishedAt.Unix(), status, runID); err != nil {
		return nil, nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, nil, fmt.Errorf("index: commit rescan: %w", err)
	}

	run := &ScanRun{ID: runID, Root: tree.Path, StartedAt: startedAt, FinishedAt: &finishedAt, Status: status}
	return tree, stats, run, nil
}

// lookupNode returns the existing row for path, if any.
func (db *DB) lookupNode(ctx context.Context, path string) (parentID sql.NullInt64, subtreeLogical, subtreeAllocated int64, existed bool, err error) {
	var id int64
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, parent_id, subtree_logical, subtree_allocated FROM nodes WHERE path = ?`, path)
	err = row.Scan(&id, &parentID, &subtreeLogical, &subtreeAllocated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return sql.NullInt64{}, 0, 0, false, nil
	case err != nil:
		return sql.NullInt64{}, 0, 0, false, fmt.Errorf("index: lookup %s: %w", path, err)
	default:
		return parentID, subtreeLogical, subtreeAllocated, true, nil
	}
}

func deleteSubtree(ctx context.Context, tx *sql.Tx, path string) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM nodes WHERE path = ? OR path LIKE ?`, path, path+"/%"); err != nil {
		return fmt.Errorf("index: delete subtree %s: %w", path, err)
	}
	return nil
}

// propagateDelta applies a byte delta to nodeID and every ancestor
// above it, following parent_id up to the indexed root.
func propagateDelta(ctx context.Context, tx *sql.Tx, nodeID int64, deltaLogical, deltaAllocated int64) error {
	for {
		if _, err := tx.ExecContext(ctx,
			`UPDATE nodes SET subtree_logical = subtree_logical + ?, subtree_allocated = subtree_allocated + ? WHERE id = ?`,
			deltaLogical, deltaAllocated, nodeID); err != nil {
			return fmt.Errorf("index: propagate delta to node %d: %w", nodeID, err)
		}
		var parentID sql.NullInt64
		row := tx.QueryRowContext(ctx, `SELECT parent_id FROM nodes WHERE id = ?`, nodeID)
		if err := row.Scan(&parentID); err != nil {
			return fmt.Errorf("index: lookup parent of node %d: %w", nodeID, err)
		}
		if !parentID.Valid {
			return nil
		}
		nodeID = parentID.Int64
	}
}

func upsertVolume(ctx context.Context, tx *sql.Tx, vs platform.VolumeStats) (int64, error) {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO volumes (mount_point, fs_type, capacity_bytes, available_bytes)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(mount_point) DO UPDATE SET
			fs_type = excluded.fs_type,
			capacity_bytes = excluded.capacity_bytes,
			available_bytes = excluded.available_bytes`,
		vs.MountPoint, vs.FSType, vs.CapacityBytes, vs.AvailableBytes)
	if err != nil {
		return 0, fmt.Errorf("index: upsert volume %s: %w", vs.MountPoint, err)
	}
	var id int64
	row := tx.QueryRowContext(ctx, `SELECT id FROM volumes WHERE mount_point = ?`, vs.MountPoint)
	if err := row.Scan(&id); err != nil {
		return 0, fmt.Errorf("index: read back volume %s: %w", vs.MountPoint, err)
	}
	return id, nil
}

const insertNodeSQL = `
INSERT INTO nodes (
	scan_run_id, volume_id, parent_id, path, name, ext, kind,
	logical_size, allocated_size, subtree_logical, subtree_allocated, subtree_state,
	duplicate, mtime, inode, device, nlink
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// insertTree prepares insertNodeSQL once and recursively inserts n and
// its descendants through it, in parent-before-child order so
// parent_id references are always valid. Reusing one prepared
// statement for the whole tree is what keeps bulk ingest fast: SQLite
// only has to parse and plan the insert once, not once per node.
func insertTree(ctx context.Context, tx *sql.Tx, runID int64, volID, parentID sql.NullInt64, n *scan.Node) error {
	stmt, err := tx.PrepareContext(ctx, insertNodeSQL)
	if err != nil {
		return fmt.Errorf("index: prepare node insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	return insertNode(ctx, stmt, runID, volID, parentID, n)
}

func insertNode(ctx context.Context, stmt *sql.Stmt, runID int64, volID, parentID sql.NullInt64, n *scan.Node) error {
	res, err := stmt.ExecContext(ctx,
		runID, volID, parentID, n.Path, n.Name, filepath.Ext(n.Name), string(kindFromScan(n.Kind)),
		n.LogicalSize, n.AllocatedSize, n.SubtreeLogicalSize, n.SubtreeAllocatedSize, string(stateFromScan(n.State)),
		n.Duplicate, n.ModTime.Unix(), n.Inode, n.Device, n.Nlink,
	)
	if err != nil {
		return fmt.Errorf("index: insert node %s: %w", n.Path, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("index: node %s last insert id: %w", n.Path, err)
	}

	childParentID := sql.NullInt64{Int64: id, Valid: true}
	for _, child := range n.Children {
		if err := insertNode(ctx, stmt, runID, volID, childParentID, child); err != nil {
			return err
		}
	}
	return nil
}
