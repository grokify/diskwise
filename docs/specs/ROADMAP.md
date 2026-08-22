# DiskWise — Roadmap (ROADMAP)

**Initiative:** `INIT-DISKWISE-001`
**Repository:** `github.com/grokify/diskwise`

**Status:** v0.1.0 released 2026-08-22 (Phases 1–3 and 6 done; Phases 4–5 planned)
**RMI slug:** `DISKWISE`

Phase status is always derived from member RMI statuses, never set directly. Review and execution happen by phase. Commits implementing an RMI carry the git trailer `Refs: RMI-DISKWISE-NNN`.

## Phase 1 — Scanner & Index Foundation
**Theme:** Fast, accurate filesystem measurement — progressive tree sizing with logical + allocated bytes, persisted to SQLite, exposed via the first CLI commands.

- [x] `RMI-DISKWISE-001` Module scaffold: go.mod, package skeletons, CI (build/test/lint), README stub
- [x] `RMI-DISKWISE-002` platform/darwin: allocated size, device/inode, volume capacity & mount detection
- [x] `RMI-DISKWISE-003` scan/: breadth-first depth-limited walker, prioritized descent, hard-link dedup, denied/mount handling
- [x] `RMI-DISKWISE-004` index/: SQLite schema, batched ingest, subtree rollups, rescan subtree replacement
- [x] `RMI-DISKWISE-005` CLI v0 via service layer: scan, rescan, summary, tree, largest with --json and progress streaming

## Phase 2 — Knowledge Registry & Hotspots
**Theme:** Knowledge-aware targeting — probe known heavy locations, claim their subtrees, and surface both known hits and unexplained large trees.

- [x] `RMI-DISKWISE-006` knowledge/ registry types: probes, data paths, semantics, default action class
- [x] `RMI-DISKWISE-007` Initial registry entries: docker-desktop, xcode, homebrew, go, node, python, ai-models, ios-backups, browsers, trash
- [x] `RMI-DISKWISE-008` Scanner integration: registry-first priority, subtree claiming, sparse-file semantics for Docker.raw
- [x] `RMI-DISKWISE-009` entity/ model plus KnownLocationDetector and LargeUnexplainedDetector
- [x] `RMI-DISKWISE-010` hotspots command and entity attribution in summary

## Phase 3 — Archives, Artifact Families & Opportunities
**Theme:** The reclamation worklist — policy engine, archive/installer detection, version-family grouping, directory rollups, and the opportunities/savings commands.

- [x] `RMI-DISKWISE-011` policy/: action classes, fail-closed rules, managed-data guard, confidence/class separation
- [x] `RMI-DISKWISE-012` ArchiveInstallerDetector: extension catalog, extraction-sibling evidence, DMG installed-app evidence
- [x] `RMI-DISKWISE-013` ArtifactFamilyDetector: filename parsing, family grouping, keep-newest/remove-all scenarios, backup-name exclusions
- [x] `RMI-DISKWISE-014` rollup/: highest fully-reclaimable directory, mixed-directory decomposition
- [x] `RMI-DISKWISE-015` opportunities and savings commands with action/confidence/type/paths filters

## Phase 4 — Developer Detectors & Explainability
**Theme:** Deeper attribution of developer storage and evidence-chain explanations for every recommendation.

- [ ] `RMI-DISKWISE-016` Xcode detector depth: DerivedData vs Archives vs simulators, .xip installers
- [ ] `RMI-DISKWISE-017` Docker path-structure breakdown by images/volumes/build-cache, no Docker API
- [ ] `RMI-DISKWISE-018` ProjectDetector: attribute source, node_modules, target/, venvs, and .git to one project entity
- [ ] `RMI-DISKWISE-019` inspect and explain commands with evidence-chain rendering
- [ ] `RMI-DISKWISE-020` Findings persistence and staleness: tie findings to scan runs, rescan invalidation

## Phase 5 — MCP Server & Contract Hardening
**Theme:** Agent surface over the same service layer, with contract parity enforced in CI, packaging, and dogfooding.

- [ ] `RMI-DISKWISE-021` cmd/diskwise-mcp: official Go SDK, stdio transport, read-only tools mirroring service 1:1
- [ ] `RMI-DISKWISE-022` Contract tests: service structs, CLI --json, and MCP results held structurally equal in CI
- [ ] `RMI-DISKWISE-023` Long-scan ergonomics over MCP: run id plus progress polling
- [ ] `RMI-DISKWISE-024` Packaging and docs: go install paths, README with CLI and MCP setup
- [ ] `RMI-DISKWISE-025` Dogfood pass: agent answers disk-full and safe-reclaim questions from tool results only

## Phase 6 — Reporting & Narrative Analysis
**Theme:** Human- and agent-facing report/insights output formats over service.Opportunities, rendered deterministically to Markdown/HTML.

Not part of the original PRD/TRD phased plan; added during v0.1.0 development in response to direct requests. These two RMI numbers were assigned retroactively — the implementing commits do not carry `Refs:` trailers.

- [x] `RMI-DISKWISE-026` report/: self-contained HTML with sortable/filterable table and file links, plus XLSX with frozen header and autofilter
- [x] `RMI-DISKWISE-027` insights/: JSON IR for narrative savings write-ups with generated/embedded JSON Schema and deterministic Markdown/HTML renderers

## Post-V1 backlog (unphased)

Not yet broken into RMIs; not parsed by `vistudio roadmap import` since items here have no RMI ID yet. Promote into a phase above once scheduled.

- SwiftUI macOS app; visual screenshot review grid (resizable thumbnails, near-duplicate groups)
- Cleanup execution: Trash-first deletion, verified backup-then-delete workflows
- FSEvents incremental index; growth snapshots ("why did my disk grow?")
- Deep service adapters: Docker API, PostgreSQL/MySQL native queries
- Duplicate and near-duplicate file detection (Phase 6's insights/ IR lets an LLM agent surface these manually today — see docs/releases/v0.1.0.md Known Limitations — but there's no built-in detector)
- Linux support; versioned external rule corpus
