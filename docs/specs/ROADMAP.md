# DiskWise — Roadmap (ROADMAP)

**Status:** v0.1.0 released 2026-08-22 (Phases 1–3 and 6 done; Phases 4–5 planned)
**Date:** 2026-08-19 (statuses last verified against code: 2026-08-22)
**Repo:** github.com/grokify/diskwise
**RMI slug:** `DISKWISE`

Phase status is always derived from member RMI statuses, never set directly. Review and execution happen by phase. Commits implementing an RMI carry the git trailer `Refs: RMI-DISKWISE-NNN`.

RMI statuses: `planned` | `in_progress` | `done` | `dropped`.

## Phase 1 — Scanner & Index Foundation

Fast, accurate filesystem measurement: progressive tree sizing with logical + allocated bytes, persisted to SQLite, exposed via the first CLI commands.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-001 | Module scaffold: go.mod, package skeletons, CI (build/test/lint), README stub | done |
| RMI-DISKWISE-002 | `platform/darwin`: allocated size, device/inode, volume capacity & mount detection | done |
| RMI-DISKWISE-003 | `scan/`: breadth-first depth-limited walker, prioritized descent, hard-link dedup, denied/mount handling | done |
| RMI-DISKWISE-004 | `index/`: SQLite schema, batched ingest, subtree rollups, `rescan <path>` subtree replacement | done |
| RMI-DISKWISE-005 | CLI v0 via service layer: `scan`, `rescan`, `summary`, `tree`, `largest` with `--json` + progress streaming | done |

## Phase 2 — Knowledge Registry & Hotspots

Knowledge-aware targeting: probe known heavy locations (app adapters), claim their subtrees, and surface both known hits and unexplained large trees.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-006 | `knowledge/` registry types (probes, data paths, semantics, default action class) | done |
| RMI-DISKWISE-007 | Initial registry entries: docker-desktop, xcode, homebrew, go, node, python, ai-models, ios-backups, browsers, trash | done |
| RMI-DISKWISE-008 | Scanner integration: registry-first priority, subtree claiming, sparse-file semantics (Docker.raw) | done |
| RMI-DISKWISE-009 | `entity/` model + KnownLocationDetector + LargeUnexplainedDetector | done |
| RMI-DISKWISE-010 | `hotspots` command + entity attribution in `summary` | done |

## Phase 3 — Archives, Artifact Families & Opportunities

The reclamation worklist: policy engine, archive/installer detection, version-family grouping, directory rollups, and the `opportunities`/`savings` commands.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-011 | `policy/`: action classes, fail-closed rules, managed-data guard, confidence/class separation | done |
| RMI-DISKWISE-012 | ArchiveInstallerDetector: extension catalog, extraction-sibling evidence, DMG installed-app evidence | done |
| RMI-DISKWISE-013 | ArtifactFamilyDetector: filename parsing, family grouping, keep-newest/remove-all scenarios, backup-name exclusions | done |
| RMI-DISKWISE-014 | `rollup/`: highest fully-reclaimable directory, mixed-directory decomposition | done |
| RMI-DISKWISE-015 | `opportunities` (`--action`, `--min-confidence`, `--type`, `--paths`) and `savings` commands | done |

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

## Phase 6 — Reporting & Narrative Analysis

Human- and agent-facing output formats over `service.Opportunities()`: shareable HTML/XLSX reports, and a JSON IR an LLM agent can fill out with ranked, evidence-backed narrative analysis (duplicate migration backups, redundant archive/extracted-folder pairs) that Go renders deterministically to Markdown/HTML.

Not part of the original PRD/TRD phased plan — added during v0.1.0 development in response to direct requests, and released alongside Phases 1–3. The two RMI numbers below were assigned retroactively when this table was reconciled with actual code state; the implementing commits do not carry `Refs:` trailers for them.

| RMI | Title | Status |
|-----|-------|--------|
| RMI-DISKWISE-026 | `report/`: self-contained HTML (sortable/filterable table, `file://` links) and XLSX (frozen header, autofilter) exports of opportunities | done |
| RMI-DISKWISE-027 | `insights/`: JSON IR (`insights.Report`) for narrative savings write-ups — ranked opportunities, evidence, kept/redundant path pairs, per-tool reclaim actions (filesystem/CLI/app-UI) — with generated/embedded JSON Schema and deterministic Markdown/HTML renderers | done |

## Post-V1 backlog (unphased)

Not yet broken into RMIs; promote into phases when scheduled:

- SwiftUI macOS app; visual screenshot review grid (resizable thumbnails, near-duplicate groups)
- Cleanup execution: Trash-first deletion, verified backup-then-delete workflows
- FSEvents incremental index; growth snapshots ("why did my disk grow?")
- Deep service adapters: Docker API, PostgreSQL/MySQL native queries
- Duplicate and near-duplicate file detection (Phase 6's `insights/` IR lets an LLM agent surface these manually today — see `docs/releases/v0.1.0.md` Known Limitations — but there's no built-in detector)
- Linux support; versioned external rule corpus
