# DiskWise

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

 [go-ci-svg]: https://github.com/grokify/diskwise/actions/workflows/go-ci.yaml/badge.svg?branch=main
 [go-ci-url]: https://github.com/grokify/diskwise/actions/workflows/go-ci.yaml
 [go-lint-svg]: https://github.com/grokify/diskwise/actions/workflows/go-lint.yaml/badge.svg?branch=main
 [go-lint-url]: https://github.com/grokify/diskwise/actions/workflows/go-lint.yaml
 [go-sast-svg]: https://github.com/grokify/diskwise/actions/workflows/go-sast-codeql.yaml/badge.svg?branch=main
 [go-sast-url]: https://github.com/grokify/diskwise/actions/workflows/go-sast-codeql.yaml
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/grokify/diskwise
 [docs-godoc-url]: https://pkg.go.dev/github.com/grokify/diskwise
 [docs-mkdoc-svg]: https://img.shields.io/badge/Go-dev%20guide-blue.svg
 [docs-mkdoc-url]: https://grokify.github.io/diskwise
 [viz-svg]: https://img.shields.io/badge/visualizaton-Go-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=grokify%2Fdiskwise
 [loc-svg]: https://tokei.rs/b1/github/grokify/diskwise
 [repo-url]: https://github.com/grokify/diskwise
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/grokify/diskwise/blob/main/LICENSE

Know your disk. Use it wisely.

DiskWise is a macOS storage-intelligence tool. It discovers what is
actually consuming disk space — attributed to meaningful entities like
apps, databases, containers, caches, and artifact families rather than
raw paths — and classifies what can be safely reclaimed by confidence
and risk, rolled up into a directly actionable, manual-deletion worklist.

V1 is discovery-only: DiskWise identifies and explains reclaimable
storage; it does not delete anything.

See [`docs/specs/PRD.md`](docs/specs/PRD.md) for the product definition,
[`docs/specs/TRD.md`](docs/specs/TRD.md) for the architecture, and
[`docs/specs/PLAN.md`](docs/specs/PLAN.md) / [`docs/specs/ROADMAP.md`](docs/specs/ROADMAP.md)
for the implementation sequence.

## Status

**v0.1.0** — Phases 1–3 of the roadmap are implemented: the scanner,
SQLite index, knowledge registry, detectors, policy engine, and the
`diskwise` CLI. The MCP server (`cmd/diskwise-mcp`) is a skeleton with
no tools registered yet. See
[`docs/releases/v0.1.0.md`](docs/releases/v0.1.0.md) for the full
release notes and [`CHANGELOG.md`](CHANGELOG.md) for the commit-level
history.

## Install

```bash
go install github.com/grokify/diskwise/cmd/diskwise@latest
```

## Components

- `github.com/grokify/diskwise` — Go library (scanner, detectors,
  policy engine, service layer)
- `cmd/diskwise` — Cobra CLI
- `cmd/diskwise-mcp` — MCP server skeleton ([modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)); not yet wired to any tools

## User Guide

DiskWise works in two steps: **scan** a directory into a local SQLite
index, then **query** that index as many times as you like without
re-walking the filesystem. Scanning is read-only — DiskWise never
deletes, moves, or modifies anything it finds.

### 1. Scan a directory

```bash
diskwise scan ~
```

This walks the tree concurrently, records logical and allocated
(on-disk) byte sizes for every file and directory, and shows a live
progress meter in the terminal. The index defaults to
`~/Library/Application Support/DiskWise/index.db`; pass `--db path/to.db`
to use a different one. Re-walk just one subtree later with:

```bash
diskwise rescan ~/Downloads
```

### 2. See what's using space

```bash
diskwise summary ~        # aggregate stats for a scanned path
diskwise tree ~           # subtree totals, sorted by size
diskwise largest ~        # largest individual files/directories
diskwise hotspots ~       # known heavy-storage hits + large unexplained dirs
```

### 3. Find reclaimable space

```bash
diskwise savings ~
```

```
Potential savings under /Users/you: 2.8 TiB

  SAFE DELETE          83.6 GiB
  LIKELY SAFE          11.0 GiB
  REVIEW               240.2 GiB
  KEEP                 0 B
  UNKNOWN              2.5 TiB
```

Tiers, most-actionable first:

| Tier | Meaning |
|------|---------|
| `safe_delete` | High-confidence regeneratable data (build caches, package-manager caches) |
| `likely_safe` | Probably safe, lower confidence (e.g. an extraction sibling exists) |
| `backup_then_delete` | Managed data (databases, containers, VMs, apps) — never auto-promoted higher, regardless of confidence |
| `review` | Needs a human look before acting |
| `keep` / `unknown` | Not a reclaim candidate, or no classification was possible |

Drill into one tier, or one entity kind, or filter by confidence:

```bash
diskwise opportunities ~ --action safe_delete
diskwise opportunities ~ --type artifact_family     # repeated downloaded versions
diskwise opportunities ~ --min-confidence 0.8
diskwise opportunities ~ --paths | xargs -I{} du -sh {}   # pipe actionable paths elsewhere
```

Every command accepts `--json` for scripting instead of the
human-readable table.

### 4. Generate a shareable report

```bash
diskwise report ~ --format html --out report.html    # self-contained, sortable/filterable
diskwise report ~ --format xlsx --out report.xlsx     # frozen header, autofilter
```

### 5. Narrative analysis (insights)

DiskWise's detectors are deliberately conservative — they never guess.
Patterns like "this is a duplicate 2022 migration backup" or "this
archive is byte-identical to that other file" require judgment an LLM
agent can supply after reading DiskWise's raw output. `insights` gives
that agent a typed contract to write its analysis into, and turns it
back into a document deterministically:

```bash
diskwise insights schema                                  # the JSON Schema contract for an agent to fill out
diskwise insights render insights.json --format md  --out insights.md
diskwise insights render insights.json --format html --out insights.html
```

Re-running `render` never re-derives the analysis — it's a pure
function of the JSON document.

## Development

```bash
go build ./...
go test ./...
golangci-lint run
```

## Release History

- [`CHANGELOG.md`](CHANGELOG.md) — commit-level, generated from [`CHANGELOG.json`](CHANGELOG.json) via [`schangelog`](https://github.com/grokify/structured-changelog)
- [`docs/releases/`](docs/releases/) — per-version release notes
