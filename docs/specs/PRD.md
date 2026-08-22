# DiskWise — Product Requirements Document (PRD)

**Status:** Draft
**Date:** 2026-08-19
**Repo:** github.com/grokify/diskwise

## 1. Summary

DiskWise is a storage-intelligence tool for macOS that answers two questions ordinary disk analyzers don't:

1. **Where did my storage actually go?** — attributed to meaningful entities (apps, databases, containers, caches, projects, artifact families), not just paths.
2. **What can I safely reclaim, and how much?** — reclaimable bytes classified by confidence and risk, rolled up to actionable units (directories), so the user can act efficiently.

The initial product is **discovery-only**: DiskWise identifies, explains, and prioritizes reclaimable storage; the user deletes manually. No file deletion, Trash integration, or cleanup execution is in scope for V1.

Tagline: **Know your disk. Use it wisely.**

## 2. Problem

macOS users — especially developers — routinely run out of disk space and cannot tell why. Existing tools (DaisyDisk, GrandPerspective, `du`) visualize *usage* but do not understand *meaning*:

- A 92 GB `~/Library/Containers/com.docker.docker` directory is shown as an opaque blob, not as "39 GB images (22 GB unused), 31 GB volumes, 18 GB build cache."
- Five `elasticsearch-*.tar.gz` files in ~/Downloads are shown as five unrelated files, not as "one product, five versions, four superseded — 2.3 GB reclaimable."
- A PostgreSQL data directory looks like deletable files, when deleting it manually would destroy databases.
- Logical size overstates real APFS consumption (clones, sparse files like `Docker.raw`).

The user's actual question is not "what files are big?" but **"what is consuming my space, what can I get rid of, and how do I do it with the fewest actions?"**

## 3. Target users

| User | Need |
|------|------|
| **Developers on macOS** (primary) | Understand Docker, Xcode, package caches, AI models, project bloat; reclaim tens of GB confidently |
| **Power users** | Find heavy Downloads/Desktop/Documents content, old installers, archives |
| **Agents (Claude Code, etc.)** | Query structured storage findings via MCP to answer "why is my disk full?" and plan cleanup with the user |

## 4. Goals (V1)

- **G1 — Fast discovery:** Useful answers within seconds via a breadth-first, depth-limited scan that refines progressively; full subtree accuracy follows.
- **G2 — Knowledge-aware targeting:** A curated registry of known heavy locations (app adapters: Docker Desktop, Xcode, Homebrew, Go/Node/Python caches, AI models, iOS backups, …) probed directly, before/alongside the generic walk.
- **G3 — Accurate sizing:** Track logical **and** allocated size (APFS sparse files and clones) so reported savings reflect real disk consumption.
- **G4 — Semantic attribution:** Findings attributed to entities (application data, cache, archive, artifact family, database, VM image) with evidence and confidence.
- **G5 — Actionable output:** Opportunities rolled up to the highest fully-reclaimable directory, classified by action tier, sorted by bytes — a manual-deletion worklist.
- **G6 — Three consistent surfaces from one core:** Go library (`diskwise` packages), Cobra CLI, and MCP server share one service layer and one SQLite index. JSON output is a first-class contract.

## 5. Non-goals (V1)

- **No deletion or cleanup execution** — no Trash, no `rm`, no backup workflows. Discovery and recommendation only.
- **No SwiftUI/macOS app** — deferred until the Go stack proves the engine. The UI is required for visual screenshot review, which is deferred with it.
- **No screenshot/image analysis** (perceptual hashing, thumbnail grid) — needs UI; deferred.
- **No background daemon / FSEvents live monitoring** — index staleness is handled by cheap subtree rescans.
- **No deep service interrogation** in early phases (Docker API, `SHOW data_directory`, MySQL `information_schema`) — path-and-file evidence first; service-native inspection is a later enhancement.
- **No Linux support** in V1, but platform-specific code stays behind a `platform` boundary to keep the door open.

## 6. Core concepts

- **Two-track scanning:**
  - *Track 1 — Knowledge registry:* known heavy locations with probes ("Docker.app installed?") and semantics ("`Docker.raw` is a sparse VM disk; allocated ≠ logical").
  - *Track 2 — Progressive tree sizing:* breadth-first depth-limited walk with live-refining subtree totals; descent prioritized by knowledge hits and largest partial counts; adapter-claimed subtrees are skipped by the generic walker.
- **Entity model:** paths are *evidence*; findings belong to semantic entities (Application, Cache, Archive, Installer, ArtifactFamily, Database, Container, VM, Model, Project).
- **ArtifactFamily:** versioned downloads of the same product (`elasticsearch-9.1.3-darwin-aarch64.tar.gz` × 5) grouped into one finding with keep-newest / remove-all savings scenarios.
- **Action classes (risk tiers):** `safe_delete`, `likely_safe`, `backup_then_delete`, `review`, `keep`, `unknown`. Detection confidence is tracked separately from action class. The policy engine fails closed: insufficient evidence → `unknown`/`review`, never `safe_delete`.
- **Directory rollups:** the actionable unit is the highest directory that is entirely reclaimable; file-level findings only inside mixed directories.

## 7. Key user flows (CLI, V1)

```bash
diskwise scan ~              # index home directory (progressive)
diskwise summary             # volume + top-level attribution
diskwise tree ~ --depth 2    # subtree totals, sorted by size
diskwise hotspots            # knowledge-base hits + largest unexplained subtrees
diskwise largest --limit 50  # largest files and directories
diskwise opportunities       # reclaimable findings, grouped by action class
diskwise savings             # bytes per action tier (conservative → maximum)
diskwise inspect <path|entity>   # what is this, and why does it matter
diskwise explain <finding-id>    # evidence behind a recommendation
diskwise rescan <path>       # re-walk one subtree after manual deletion
```

All read-oriented commands support `--json`; `opportunities` supports `--paths` (one path per line) for piping.

**MCP flow:** an agent calls `get_storage_summary`, `list_cleanup_opportunities` (filterable by confidence/risk), `inspect_entity`, `explain_finding` against the same SQLite index — answering "why is my disk full?" and "what's the safest way to free 30 GB?" from deterministic data. The LLM never decides what is safe; the policy engine does.

## 8. Success criteria

- On a real developer Mac, `diskwise hotspots` surfaces the top 5 storage consumers (e.g. Docker, Xcode, AI models, package caches, Downloads archives) within one scan, with correct semantics.
- `diskwise opportunities` output can be acted on manually: each `safe_delete` row is a single directory or file path that is genuinely safe to remove, verified against the knowledge base.
- Sparse files (Docker.raw) and APFS clones are reported at allocated size, not logical size.
- CLI `--json`, MCP responses, and library return values are structurally identical for the same query (contract tests enforce this).
- Findings never classify managed data (live database directories, application-internal stores) as `safe_delete`.

## 9. Future direction (post-V1)

Ordered by expected value: SwiftUI app with visual screenshot review → cleanup execution (Trash-first, verified backups) → FSEvents incremental index + "why did my disk grow?" snapshots → deep service adapters (Docker API, Postgres/MySQL queries) → duplicate detection → Linux support.
