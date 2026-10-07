# DiskWise — Roadmap (ROADMAP)

**Initiative:** `INIT-DISKWISE-001`
**Repository:** `github.com/grokify/diskwise`

**Status:** v0.1.0 released 2026-08-22; Phases 1–3 and 6 shipped in it, Phases 7–8 are done since (unreleased), Phases 4–5 planned
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

## Phase 7 — Trustworthy Numbers & Scan Transparency
**Theme:** Make reported totals add up and make the output self-explanatory — disjoint unexplained findings, no byte attributed twice, visible paths, honest savings semantics, and clear guidance when a scan is incomplete.

Not part of the original PRD/TRD phased plan; added after dogfooding on a full home-directory scan.

- [x] `RMI-DISKWISE-028` Disjoint unexplained directories: report only outermost large directories, and subtract bytes other findings already claim so tier totals never exceed the scanned root; extends the no-double-counting contract to byte level
- [x] `RMI-DISKWISE-029` Honest savings output: `Reclaimable` total excludes keep and unknown tiers; hotspots text shows every known-location path
- [x] `RMI-DISKWISE-030` Implicit-path announcement: commands defaulting to the current directory say so on stderr
- [x] `RMI-DISKWISE-031` Scan transparency: list denied directories (capped sample in `ScanResult` JSON), macOS Full Disk Access guidance in place of the misleading elevated-permissions hint, and a run-from-Terminal note in `scan --help` and the README

## Phase 8 — Upgrade Readiness & Review Workflow
**Theme:** Answer "do I have enough room for X?" directly, and turn findings into a reviewable, shareable worklist without hand-assembly.

- [x] `RMI-DISKWISE-032` `preflight --need <size>`: free space, APFS container free space and Time Machine local snapshots (read-only), with a yes/no verdict and a non-zero exit when insufficient; purgeable space is called out as not measurable rather than estimated
- [x] `RMI-DISKWISE-033` `pairs`: compare an archive with its sibling extracted directory by file count and bytes (same, same_count, archive_has_more, dir_has_more); Photos libraries compared on original media only; plain `.tar` now recognized as an archive
- [x] `RMI-DISKWISE-034` Bundle-aware classification: managed bundles (e.g. `.photoslibrary`) reported as keep-tier `managed_bundle` findings instead of "unknown", with no byte reported twice
- [x] `RMI-DISKWISE-035` `review --format md`: checkbox worklist grouped by tier, deterministic output, optional archive-pair section
- [x] `RMI-DISKWISE-036` `export --out <dir>`: savings, opportunities, hotspots, review (and pairs with `--pairs`) with the scanned root recorded in every JSON file; `opportunities --json` keeps its bare array for compatibility
- [x] `RMI-DISKWISE-037` Path redaction: `--redact` abbreviates the home directory, `--redact-prefix` replaces everything under a prefix with a stable opaque token, applied to opportunities, hotspots, savings, report, pairs, review and export

## Post-V1 backlog (unphased)

Not yet broken into RMIs; not parsed by `vistudio roadmap import` since items here have no RMI ID yet. Promote into a phase above once scheduled.

- SwiftUI macOS app; visual screenshot review grid (resizable thumbnails, near-duplicate groups)
- Cleanup execution: Trash-first deletion, verified backup-then-delete workflows
- FSEvents incremental index; growth snapshots ("why did my disk grow?")
- Deep service adapters: Docker API, PostgreSQL/MySQL native queries
- Duplicate and near-duplicate file detection (Phase 6's insights/ IR lets an LLM agent surface these manually today — see docs/releases/v0.1.0.md Known Limitations — but there's no built-in detector)
- Linux support; versioned external rule corpus
- More known locations: Time Machine local snapshots, MobileSync device backups, Mail downloads, per-project build artifacts (Rust `target/`, `node_modules`, Python venvs, Gradle caches), large `.git` object stores
- Age/staleness signal (atime/mtime) to rank the review tier
