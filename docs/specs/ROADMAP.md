# DiskWise — Roadmap (ROADMAP)

**Status:** Draft
**Date:** 2026-08-19
**Repo:** github.com/grokify/diskwise
**RMI slug:** `DISKWISE`

Phase status is always derived from member RMI statuses, never set directly. Review and execution happen by phase. Commits implementing an RMI carry the git trailer `Refs: RMI-DISKWISE-NNN`.

RMI statuses: `planned` | `in_progress` | `done` | `dropped`.

## Phase 1 — Scanner & Index Foundation

Fast, accurate filesystem measurement: progressive tree sizing with logical + allocated bytes, persisted to SQLite, exposed via the first CLI commands.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-001 | Module scaffold: go.mod, package skeletons, CI (build/test/lint), README stub | planned |
| RMI-DISKWISE-002 | `platform/darwin`: allocated size, device/inode, volume capacity & mount detection | planned |
| RMI-DISKWISE-003 | `scan/`: breadth-first depth-limited walker, prioritized descent, hard-link dedup, denied/mount handling | planned |
| RMI-DISKWISE-004 | `index/`: SQLite schema, batched ingest, subtree rollups, `rescan <path>` subtree replacement | planned |
| RMI-DISKWISE-005 | CLI v0 via service layer: `scan`, `rescan`, `summary`, `tree`, `largest` with `--json` + progress streaming | planned |

## Phase 2 — Knowledge Registry & Hotspots

Knowledge-aware targeting: probe known heavy locations (app adapters), claim their subtrees, and surface both known hits and unexplained large trees.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-006 | `knowledge/` registry types (probes, data paths, semantics, default action class) | planned |
| RMI-DISKWISE-007 | Initial registry entries: docker-desktop, xcode, homebrew, go, node, python, ai-models, ios-backups, browsers, trash | planned |
| RMI-DISKWISE-008 | Scanner integration: registry-first priority, subtree claiming, sparse-file semantics (Docker.raw) | planned |
| RMI-DISKWISE-009 | `entity/` model + KnownLocationDetector + LargeUnexplainedDetector | planned |
| RMI-DISKWISE-010 | `hotspots` command + entity attribution in `summary` | planned |

## Phase 3 — Archives, Artifact Families & Opportunities

The reclamation worklist: policy engine, archive/installer detection, version-family grouping, directory rollups, and the `opportunities`/`savings` commands.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-011 | `policy/`: action classes, fail-closed rules, managed-data guard, confidence/class separation | planned |
| RMI-DISKWISE-012 | ArchiveInstallerDetector: extension catalog, extraction-sibling evidence, DMG installed-app evidence | planned |
| RMI-DISKWISE-013 | ArtifactFamilyDetector: filename parsing, family grouping, keep-newest/remove-all scenarios, backup-name exclusions | planned |
| RMI-DISKWISE-014 | `rollup/`: highest fully-reclaimable directory, mixed-directory decomposition | planned |
| RMI-DISKWISE-015 | `opportunities` (`--action`, `--min-confidence`, `--type`, `--paths`) and `savings` commands | planned |

## Phase 4 — Developer Detectors & Explainability

Deeper attribution of developer storage and evidence-chain explanations for every recommendation.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-016 | Xcode detector depth: DerivedData vs Archives vs simulators, `.xip` installers | planned |
| RMI-DISKWISE-017 | Docker path-structure breakdown (images/volumes/build-cache, no Docker API) | planned |
| RMI-DISKWISE-018 | ProjectDetector: attribute source, node_modules, target/, venvs, .git to one project entity | planned |
| RMI-DISKWISE-019 | `inspect <path|entity>` and `explain <finding-id>` with evidence rendering | planned |
| RMI-DISKWISE-020 | Findings persistence & staleness: tie findings to scan runs, rescan invalidation | planned |

## Phase 5 — MCP Server & Contract Hardening

Agent surface over the same service layer, with contract parity enforced in CI, packaging, and dogfooding.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-021 | `cmd/diskwise-mcp`: official Go SDK, stdio transport, read-only tools mirroring service 1:1 | planned |
| RMI-DISKWISE-022 | Contract tests: service structs ↔ CLI `--json` ↔ MCP results structural equality in CI | planned |
| RMI-DISKWISE-023 | Long-scan ergonomics over MCP: run id + progress polling | planned |
| RMI-DISKWISE-024 | Packaging & docs: `go install` paths, README with CLI + MCP (Claude Code) setup | planned |
| RMI-DISKWISE-025 | Dogfood pass: agent answers "why is my disk full?" / "free 30 GB safely" from tool results only | planned |

## Post-V1 backlog (unphased)

Not yet broken into RMIs; promote into phases when scheduled:

- SwiftUI macOS app; visual screenshot review grid (resizable thumbnails, near-duplicate groups)
- Cleanup execution: Trash-first deletion, verified backup-then-delete workflows
- FSEvents incremental index; growth snapshots ("why did my disk grow?")
- Deep service adapters: Docker API, PostgreSQL/MySQL native queries
- Duplicate and near-duplicate file detection
- Linux support; versioned external rule corpus
