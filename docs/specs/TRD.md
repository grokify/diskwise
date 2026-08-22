# DiskWise — Technical Requirements Document (TRD)

**Status:** Draft
**Date:** 2026-08-19
**Repo:** github.com/grokify/diskwise

## 1. Architecture overview

One Go core, thin adapters, shared SQLite index. Library-first: all logic lives in importable packages; `cmd/` binaries are adapters over one service layer.

```text
                      diskwise core (Go library)
              ┌───────────────────────────────────────┐
              │ scan/      progressive walker          │
              │ knowledge/ known-location registry     │
              │ detect/    detectors → findings        │
              │ entity/    semantic entity model       │
              │ policy/    confidence → action class   │
              │ rollup/    directory aggregation       │
              │ service/   commands + queries (API)    │
              │ index/     SQLite persistence          │
              │ platform/  macOS-specific syscalls     │
              └───────────────┬───────────────────────┘
                              │
                           SQLite
                              │
              ┌───────────────┼───────────────────────┐
              │               │                       │
        cmd/diskwise    cmd/diskwise-mcp      (future SwiftUI app,
         Cobra CLI       MCP server            via same service)
```

Design rules:

- **SQLite belongs to the core.** Adapters never issue SQL; they call `service` queries.
- **The scanner collects facts. Detectors decide what things are. Policy decides what to recommend.** No layer skips ahead.
- **V1 is read-only.** No code path deletes, moves, or modifies user files. The only writes are to the DiskWise SQLite database and logs.
- **macOS specifics live in `platform/`** (allocated-size stat, volume capacity, standard user directories) so a Linux port changes one package.

## 2. Module and dependencies

- **Module:** `github.com/grokify/diskwise`
- **Go:** 1.25+
- **CLI:** `github.com/spf13/cobra`
- **MCP:** `github.com/modelcontextprotocol/go-sdk` (official Go SDK)
- **SQLite driver:** `modernc.org/sqlite` (pure Go, no cgo). **Resolved in Phase 1:** a single prepared statement reused across a batch ingest reached 89.7k nodes/sec, comfortably past the ≥50k/sec target — no benchmark-driven switch to `mattn/go-sqlite3` was needed.

Per global convention, **verify latest versions at implementation time** (GitHub releases / pkg.go.dev / `go list -m -versions`); do not assume versions written here or in training data are current.

## 3. Package layout

```text
diskwise/
├── cmd/
│   ├── diskwise/            # Cobra CLI (thin)
│   └── diskwise-mcp/        # MCP server (thin)
├── scan/                    # walker, progressive sizing, scan scheduling
├── knowledge/               # known-location registry + app adapters
├── detect/                  # Detector interface, archive/installer/artifact-family detectors
├── entity/                  # StorageEntity types, ArtifactFamily
├── policy/                  # action classes, confidence model, fail-closed rules
├── rollup/                  # directory rollup: highest fully-reclaimable dir
├── service/                 # Service interface: commands + queries (the only adapter API)
├── index/                   # SQLite schema, migrations, bulk ingest, queries
├── platform/                # darwin: allocated size, volume stats, home dirs
└── docs/specs/              # this spec set
```

## 4. Scanning model (two tracks)

### 4.1 Track 1 — knowledge registry (`knowledge/`)

A curated catalog of known heavy macOS locations. Each entry has:

```go
type KnownLocation struct {
    ID        string   // "docker-desktop", "xcode-derived-data", ...
    Probes    []Probe  // app bundle exists, process running, path exists
    DataPaths []string // where the heavy bytes live (~-expanded)
    Entity    entity.Kind
    Semantics Semantics // e.g. SparseVMDisk, RegeneratableCache
    DefaultActionClass policy.ActionClass
}
```

Initial registry (data as declarative Go structures; externalizable to versioned YAML later):

| ID | Data paths | Semantics |
|----|-----------|-----------|
| docker-desktop | `~/Library/Containers/com.docker.docker` | `Docker.raw` sparse VM disk; report allocated size |
| xcode | `~/Library/Developer/Xcode/DerivedData`, `Archives`, `~/Library/Developer/CoreSimulator` | DerivedData regeneratable |
| homebrew | `~/Library/Caches/Homebrew`, `/opt/homebrew/Cellar` | cache safe; Cellar = installed software |
| go | `~/go/pkg/mod`, `~/Library/Caches/go-build` | regeneratable |
| node | `~/.npm`, `~/Library/pnpm`, `~/Library/Caches/Yarn` | regeneratable |
| python | `~/Library/Caches/pip`, `~/.cache/uv` | regeneratable |
| ai-models | `~/.cache/huggingface`, `~/.ollama/models`, `~/.lmstudio` | re-downloadable but large; `review` |
| ios-backups | `~/Library/Application Support/MobileSync/Backup` | user data; `review` |
| browsers | `~/Library/Caches/<browser bundle ids>` | cache |
| trash | `~/.Trash` | already-deleted; informational |

Probes make the registry cheap: entries whose app probe fails are skipped entirely. Registry hits are scanned first and their subtrees **claimed**, so the generic walker skips re-enumerating them.

### 4.2 Track 2 — progressive tree sizing (`scan/`)

- **Breadth-first, depth-limited first pass** (default depth 2–3 from scan roots): every visible subtree gets a running total immediately, refined as deeper enumeration completes. Status per subtree: `partial` → `complete`.
- **Prioritized descent queue** ordered by (a) knowledge-registry hits, (b) largest partial byte counts.
- **Sizes:** logical size and allocated size both recorded. Allocated = `st_blocks × 512` from `syscall.Stat_t` (via `platform/darwin`). Hard links deduplicated per (device, inode) within a scan run.
- **Skips:** other-volume mount points (compare `st_dev`), firmlinks/`/System/Volumes` duplicates, adapter-claimed subtrees, and permission-denied directories — which are **recorded and surfaced** ("Inaccessible: ~34 GB"), never silently dropped.
- **Concurrency:** worker pool over the descent queue; SQLite writes batched in transactions (target: ≥50k file rows/sec ingest).
- **Incremental rescan:** `rescan <path>` re-walks one subtree, replaces its rows, recomputes ancestor rollups. Full-scan freshness via directory mtime comparison is a later optimization; V1 correctness baseline is "rescan what you touched."

## 5. Data model (SQLite)

```sql
scan_runs   (id, root, started_at, finished_at, status)
volumes     (id, mount_point, fs_type, capacity_bytes, available_bytes)
nodes       (id, scan_run_id, volume_id, parent_id, path, name, ext,
             kind,                -- file | dir | symlink | package
             logical_size, allocated_size,
             subtree_logical, subtree_allocated,   -- dirs: rollup totals
             subtree_state,      -- partial | complete | claimed | denied
             mtime, inode, device, nlink)
entities    (id, kind, name, detector_id, confidence)
entity_nodes(entity_id, node_id, role)
findings    (id, entity_id, kind, bytes, reclaimable_bytes,
             confidence, action_class, reason)
evidence    (id, finding_id, type, detail)
```

Indexes on `nodes(parent_id)`, `nodes(subtree_allocated DESC)`, `nodes(path)`. The database is disposable/rebuildable; schema migrations may drop-and-rescan in V1. Default location: `~/Library/Application Support/DiskWise/index.db` (override via `--db`).

## 6. Detection and policy

### 6.1 Detector interface (`detect/`)

```go
type Detector interface {
    ID() string
    Detect(ctx context.Context, in Input) ([]Finding, error)
}

type Finding struct {
    Kind             Kind
    Entity           entity.Entity
    Nodes            []NodeRef
    Bytes            int64
    ReclaimableBytes int64
    Confidence       float64      // detection confidence, 0..1
    ActionClass      policy.ActionClass
    Evidence         []Evidence
    Reason           string       // human-readable, one sentence
    Scenarios        []Scenario   // e.g. keep-newest vs remove-all
}
```

V1 detectors (path/file evidence only — no service APIs, no process interrogation beyond existence probes):

- **KnownLocationDetector** — wraps the knowledge registry into findings.
- **ArchiveInstallerDetector** — `.dmg .pkg .xip .iso .zip .tar .tar.{gz,bz2,xz,zst} .tgz .tbz2 .txz .7z .gz .ipsw` in user directories; extraction-sibling evidence (archive next to same-named directory); installed-app evidence for DMGs (`/Applications/<App>.app` newer than DMG).
- **ArtifactFamilyDetector** — parses `<product>-<version>-<platform>-<arch>.<ext>` (and common variants: `go1.x.darwin-arm64`, `node-vX`, `terraform_X`), groups instances, computes keep-newest and remove-all scenarios. Backup-suggestive names (`backup`, `export`, tax/financial terms) are excluded from `likely_safe` regardless of pattern match.
- **LargeUnexplainedDetector** — large subtrees claimed by no entity → `hotspots` "worth exploring" list.

### 6.2 Policy engine (`policy/`)

Action classes: `safe_delete`, `likely_safe`, `backup_then_delete`, `review`, `keep`, `unknown`.

Rules:

- **Fail closed.** Missing/weak evidence → `review` or `unknown`. Only registry entries with `RegeneratableCache` semantics plus corroborating evidence may reach `safe_delete`.
- **Confidence ≠ action class.** A confirmed database directory is high-confidence detection and `keep`/`backup_then_delete`, never `safe_delete`.
- **Managed-data guard:** entities of kind Database, Container, VM, ApplicationData are structurally barred from `safe_delete` in V1 (no override flag).

### 6.3 Directory rollup (`rollup/`)

For each finding, emit the smallest set of paths covering all its nodes such that every emitted directory is *entirely* covered by the finding. Mixed directories decompose into child paths/files. This is the "act on it manually" contract: every `opportunities` row is directly actionable.

## 7. Service layer (`service/`)

The only API adapters may use:

```go
type Service interface {
    Scan(ctx context.Context, req ScanRequest) (*ScanResult, error)       // command
    Rescan(ctx context.Context, path string) (*ScanResult, error)        // command
    Summary(ctx context.Context) (*StorageSummary, error)
    Tree(ctx context.Context, q TreeQuery) ([]TreeNode, error)
    Largest(ctx context.Context, q LargestQuery) ([]Item, error)
    Hotspots(ctx context.Context) (*HotspotsReport, error)
    Opportunities(ctx context.Context, q OpportunityQuery) ([]Opportunity, error)
    Savings(ctx context.Context) (*SavingsByTier, error)
    Inspect(ctx context.Context, ref string) (*Inspection, error)
    Explain(ctx context.Context, findingID string) (*Explanation, error)
}
```

Return types are the JSON contract: CLI `--json` marshals them directly; MCP tool results marshal the same structs. **Contract tests** run one fixture through service → CLI JSON → MCP response and assert structural equality.

## 8. CLI (`cmd/diskwise`, Cobra)

```text
diskwise scan [path...]        --depth N (initial pass depth)
diskwise rescan <path>
diskwise summary               --json
diskwise tree <path>           --depth N --min-size S --json
diskwise largest               --limit N --dirs|--files --json
diskwise hotspots              --json
diskwise opportunities         --action <class> --min-confidence F --type <kind>
                               --json --paths
diskwise savings               --json
diskwise inspect <path|entity> --json
diskwise explain <finding-id>  --json
```

Human output: aligned tables grouped by action class, sizes in human units, sorted by bytes descending. `--paths` prints one actionable path per line for piping. Long scans stream progress to stderr (subtree counts refining live); stdout stays clean for data.

## 9. MCP server (`cmd/diskwise-mcp`)

Official Go SDK, stdio transport. Tools mirror the service vocabulary 1:1:

```text
scan_storage                 (root, depth?)          — may be long-running; returns run id + progress
get_storage_summary
get_storage_tree             (path, depth?, min_size?)
list_largest_items           (limit?, kind?)
list_hotspots
list_cleanup_opportunities   (action_class?, min_confidence?, type?)
get_savings_by_tier
inspect_entity               (ref)
explain_finding              (finding_id)
```

All tools are read-only against the shared index (no tool triggers deletion — none exists). Tool descriptions state explicitly that recommendations come from a deterministic policy engine and the agent must not extrapolate beyond returned action classes.

## 10. Testing strategy

- **Fixture filesystems:** testdata trees built by test helpers (temp dirs with sized files, sparse files, hard links, archive/extract pairs, elasticsearch-style version families). Detectors and rollup are tested as pure functions over fixtures.
- **Golden findings:** fixture → expected `[]Finding` JSON, asserted byte-for-byte (update via `-update` flag).
- **Contract tests:** service structs ↔ CLI `--json` ↔ MCP result parity.
- **Policy tests:** adversarial cases — live-looking database dir, `backup` in filename, mixed directories — must never yield `safe_delete`.
- **Platform tests:** allocated-size on sparse files, hard-link dedup (darwin-only, `//go:build darwin`).
- Unit tests preferred over integration tests throughout (org convention). `golangci-lint run` clean; error handling per global priority order (no discarded errors).

## 11. Performance targets (V1)

- First useful output (`tree` depth-2 totals, partial) ≤ 5 s on a 1 TB home directory.
- Full home-directory scan ≤ 5 min on Apple Silicon with warm FS cache.
- SQLite ingest ≥ 50k nodes/sec batched.
- `opportunities` / `summary` queries over an existing index ≤ 200 ms.

## 12. Security & safety

- Read-only V1: no deletion code paths exist anywhere in the module.
- Never open or report contents of files; only metadata (path, size, times). Paths may still be sensitive — no telemetry, no network calls.
- Respect permission errors: record, surface as "inaccessible," never escalate or prompt for privileges in V1 (Full Disk Access guidance documented in README instead).
