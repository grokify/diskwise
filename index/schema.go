package index

// schemaSQL defines the Phase 1 tables: scan provenance, volume
// capacity, and the flattened filesystem node tree with its rollup
// columns. Entity/finding/evidence tables are added when the detect
// and policy packages that populate them land (see docs/specs/PLAN.md).
//
// The schema is disposable and rebuildable from a rescan, so V1
// migrations are additive `CREATE TABLE IF NOT EXISTS` rather than a
// versioned migration framework.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS scan_runs (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	root        TEXT NOT NULL,
	started_at  INTEGER NOT NULL,
	finished_at INTEGER,
	status      TEXT NOT NULL DEFAULT 'running'
);

CREATE TABLE IF NOT EXISTS volumes (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	mount_point     TEXT NOT NULL UNIQUE,
	fs_type         TEXT NOT NULL,
	capacity_bytes  INTEGER NOT NULL,
	available_bytes INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS nodes (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	scan_run_id       INTEGER NOT NULL REFERENCES scan_runs(id),
	volume_id         INTEGER REFERENCES volumes(id),
	parent_id         INTEGER REFERENCES nodes(id),
	path              TEXT NOT NULL UNIQUE,
	name              TEXT NOT NULL,
	ext               TEXT NOT NULL DEFAULT '',
	kind              TEXT NOT NULL,
	logical_size      INTEGER NOT NULL DEFAULT 0,
	allocated_size    INTEGER NOT NULL DEFAULT 0,
	subtree_logical   INTEGER NOT NULL DEFAULT 0,
	subtree_allocated INTEGER NOT NULL DEFAULT 0,
	subtree_state     TEXT NOT NULL DEFAULT '',
	duplicate         INTEGER NOT NULL DEFAULT 0,
	mtime             INTEGER,
	inode             INTEGER NOT NULL DEFAULT 0,
	device            INTEGER NOT NULL DEFAULT 0,
	nlink             INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_nodes_parent_id ON nodes(parent_id);
CREATE INDEX IF NOT EXISTS idx_nodes_subtree_allocated ON nodes(subtree_allocated DESC);
`
