# DiskWise — Implementation Plan (PLAN)

**Status:** Draft
**Date:** 2026-08-19
**Repo:** github.com/grokify/diskwise

This plan sequences the work defined in the [PRD](PRD.md) and [TRD](TRD.md). Roadmap items and phase membership are tracked in [ROADMAP.md](ROADMAP.md); this document explains ordering, dependencies, and verification per phase.

## Guiding constraints

- **Go stack first.** Prove the full engine via CLI + MCP before any SwiftUI work. The only feature that truly requires UI (visual screenshot review) is out of V1 scope.
- **Read-only V1.** No deletion code paths. Output is a manual-deletion worklist (directory rollups, `--paths`).
- **Library-first.** Every phase lands as library packages + thin adapter changes; each phase leaves `main` compiling, tested, and lint-clean.
- **Verify dependency versions** (cobra, MCP go-sdk, sqlite driver) at the moment of `go get`, per global convention.

## Phase 1 — Scanner & index foundation

**Goal:** `diskwise scan ~ && diskwise tree ~ --depth 2` gives correct, fast subtree totals with logical + allocated sizes.

Order of work:

1. Module scaffold: `go.mod`, package skeletons, CI (build/test/lint), README stub.
2. `platform/darwin`: allocated size (`st_blocks×512`), device/inode, volume capacity/mount detection.
3. `scan/`: breadth-first depth-limited walker → prioritized descent queue → full enumeration; hard-link dedup; mount-point and permission-denied handling (recorded, surfaced).
4. `index/`: SQLite schema (TRD §5), batched ingest, subtree rollup computation, `rescan <path>` subtree replacement.
5. `service/` + `cmd/diskwise`: `scan`, `rescan`, `summary`, `tree`, `largest` with `--json`; progress streaming to stderr.

**Dependencies:** none. **Decision point:** SQLite driver benchmark (modernc vs mattn) once ingest exists — pick and record in TRD.

**Verify:** fixture tests (sparse file → allocated < logical; hard links counted once); run on a real home directory; `tree` totals cross-checked against `du -sk` within tolerance (APFS clones explain deltas); first partial output ≤ 5 s.

## Phase 2 — Knowledge registry & hotspots

**Goal:** `diskwise hotspots` merges known-location hits (with semantics) and large unexplained subtrees.

Order of work:

1. `knowledge/`: registry types (probe, data paths, semantics, default action class) + initial entries (TRD §4.1: docker-desktop, xcode, homebrew, go, node, python, ai-models, ios-backups, browsers, trash).
2. Scanner integration: registry probed first; hits enqueued at top priority; claimed subtrees skipped by generic walk and marked `claimed`.
3. `entity/` + `detect/`: KnownLocationDetector emits entities/findings from registry hits; Docker.raw sparse-size semantics applied.
4. LargeUnexplainedDetector: large `complete` subtrees with no entity claim.
5. `hotspots` command + service query; `summary` gains entity attribution ("Docker 83 GB, Xcode 31 GB, …").

**Depends on:** Phase 1 index and walker.

**Verify:** on a machine with Docker Desktop, `hotspots` reports Docker at allocated (not logical) size; probe-failed entries cost ~0 scan time; golden findings for a fixture registry.

## Phase 3 — Archives, artifact families & opportunities

**Goal:** `diskwise opportunities` / `savings` produce a tiered, directly-actionable worklist.

Order of work:

1. `policy/`: action classes, fail-closed rules, managed-data guard, confidence-vs-class separation. (Land policy before detectors that feed it, so tests constrain them from day one.)
2. ArchiveInstallerDetector: extension catalog, extraction-sibling evidence, DMG installed-app evidence.
3. ArtifactFamilyDetector: filename pattern parsing (`<product>-<semver>-<platform>-<arch>`), family grouping, keep-newest vs remove-all scenarios, backup-name exclusions.
4. `rollup/`: highest fully-reclaimable directory; mixed-directory decomposition.
5. `opportunities` (with `--action`, `--min-confidence`, `--type`, `--paths`) and `savings` commands.

**Depends on:** Phase 1 index; Phase 2 entity model.

**Verify:** the Elasticsearch fixture (5 tarballs in Downloads) yields one family finding with both scenarios; adversarial policy tests (live-db lookalike, `backup` filenames, mixed dirs) never yield `safe_delete`; every emitted `opportunities` path is manually actionable as a single unit.

## Phase 4 — Developer detectors & explainability

**Goal:** deeper attribution for developer storage; every recommendation explainable.

Order of work:

1. Registry expansion to detector depth: Xcode (DerivedData vs Archives vs simulators, `.xip`), Docker directory-structure breakdown (images/volumes/build-cache by path — no Docker API), node_modules discovery across project trees, Go/Rust/Python build outputs.
2. ProjectDetector: attribute source + `node_modules` + `target/` + venv + `.git` to one project entity ("VisionStudio project — 21 GB").
3. `inspect <path|entity>` and `explain <finding-id>`: evidence chain rendering.
4. Findings persistence & staleness: findings tied to scan runs; `rescan` invalidates affected findings.

**Depends on:** Phase 2 registry/entities, Phase 3 policy.

**Verify:** project fixture attributes multi-ecosystem bloat to one entity; `explain` renders every evidence type; rescan after (manual) deletion updates savings correctly.

## Phase 5 — MCP server & contract hardening

**Goal:** agents get the same answers as the CLI, guaranteed.

Order of work:

1. `cmd/diskwise-mcp`: official Go SDK, stdio transport, tools per TRD §9 (read-only, mirrors service 1:1).
2. Contract tests: fixture → service structs ↔ CLI `--json` ↔ MCP results, structural equality enforced in CI.
3. Long-scan ergonomics over MCP: scan returns run id + progress polling.
4. Packaging: `go install` paths for both binaries; Homebrew formula/goreleaser deferred to release time; README with CLI + MCP setup (Claude Code config example).
5. End-to-end dogfood: register the MCP server in Claude Code, ask "why is my disk full?" and "safest way to free 30 GB?" — findings must come entirely from tool results.

**Depends on:** service layer stability (Phases 1–4).

**Verify:** contract tests green in CI; dogfood transcript reviewed; `golangci-lint` clean; coverage badge updated (`gocoverbadge -dir . -exclude cmd -badge-only`).

## Explicitly deferred (post-V1 backlog)

SwiftUI app + visual screenshot review grid; cleanup execution (Trash-first, verified backups); FSEvents incremental index + growth snapshots ("why did my disk grow?"); deep service adapters (Docker API, Postgres/MySQL queries); duplicate/near-duplicate detection; Linux port; versioned external rule corpus.

## Working conventions

- Conventional commits with `Refs: RMI-DISKWISE-NNN` trailers; atomic topical commits (scaffolding included — no monolithic "initial commit").
- Each phase reviewed and executed as a unit (org convention: review by phase, not individual RMI).
- Pre-push checklist per global CLAUDE.md (tests, lint, no local replace directives, coverage badge).
