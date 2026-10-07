# DiskWise

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Docs][docs-mkdoc-svg]][docs-mkdoc-url]
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
 [docs-mkdoc-svg]: https://img.shields.io/badge/docs-user%20guide-blue.svg
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

## Documentation

The full **[user guide](https://grokify.github.io/diskwise)** covers
installation, every command, how findings are classified, and
troubleshooting. Project documents live in this repo:

- [`docs/guides/`](docs/guides/) — user guide (source of the site)
- [`docs/specs/PRD.md`](docs/specs/PRD.md) — product definition
- [`docs/specs/TRD.md`](docs/specs/TRD.md) — architecture
- [`docs/specs/PLAN.md`](docs/specs/PLAN.md) / [`docs/specs/ROADMAP.md`](docs/specs/ROADMAP.md) — implementation sequence
- [`docs/releases/`](docs/releases/) — per-version release notes

## Status

**v0.1.0** shipped the scanner, SQLite index, knowledge registry,
detectors, policy engine, report/insights output formats, and the `diskwise`
CLI (roadmap Phases 1–3 and 6). **v0.2.0** (Phases 7–9) adds:

- Totals that add up: unexplained directories are disjoint, bytes another
  finding already reports are excluded, and `savings` no longer counts the
  `unknown` tier as savings.
- Results that say how old they are, and mark findings whose paths no longer
  exist.
- App-managed bundles (e.g. Photos libraries) classified as `keep`.
- `pairs` compares archives with their extracted copies; `preflight` checks
  free space for an OS upgrade; `review` and `export` produce a checkbox
  worklist and a consistent set of JSON files; `--redact` /
  `--redact-prefix` make output safe to share.

`opportunities --json` is now an object rather than a bare array; see the
[v0.2.0 release notes](docs/releases/v0.2.0.md).

The MCP server (`cmd/diskwise-mcp`) is a skeleton with no tools registered
yet (Phases 4–5, still planned). See
[`docs/releases/v0.1.0.md`](docs/releases/v0.1.0.md) and
[`CHANGELOG.md`](CHANGELOG.md) for history.

## Install

```bash
go install github.com/grokify/diskwise/cmd/diskwise@latest
```

macOS only. Run it from Terminal or iTerm, and grant that terminal Full Disk
Access (System Settings > Privacy & Security) for a complete scan. See the
[installation guide](docs/guides/installation.md).

## Quick start

```bash
diskwise scan ~                      # measure (read-only); the result is kept in a local index
diskwise hotspots ~                  # known heavy locations + large unexplained directories
diskwise savings ~                   # what could be reclaimed, by risk tier
diskwise review ~ --out review.md    # checkbox worklist to work through yourself
```

```text
Potential savings under /Users/you: 335.1 GiB
(excludes KEEP and UNKNOWN; UNKNOWN is unexplained large directories, not a savings estimate)

  SAFE DELETE          83.6 GiB
  LIKELY SAFE          11.0 GiB
  REVIEW               240.5 GiB
  KEEP                 98.1 GiB
  UNKNOWN              790.0 GiB
```

DiskWise never deletes, moves, or modifies anything it finds. It produces an
explained worklist; you act on it.

## Commands

| Command | Purpose |
|---|---|
| `scan`, `rescan` | Measure a directory into the index; re-measure one subtree |
| `summary`, `tree`, `largest` | Browse sizes from the index |
| `hotspots` | Known heavy locations and large unexplained directories |
| `savings`, `opportunities` | Reclaimable space by tier; the full findings list with filters |
| `pairs` | Compare archives with the extracted directories beside them |
| `preflight` | Check free space against a requirement (e.g. `--need 50gib`) |
| `review`, `report`, `export` | Checklist, HTML/XLSX report, or a set of JSON files |
| `insights` | Render a narrative savings write-up from a JSON document |

Every command accepts `--json`. See the
[CLI reference](docs/guides/cli-reference.md) for all flags, and
[JSON output](docs/guides/json-output.md) for the shapes.

## Components

- `github.com/grokify/diskwise` — Go library (scanner, detectors,
  policy engine, service layer)
- `cmd/diskwise` — Cobra CLI
- `cmd/diskwise-mcp` — MCP server skeleton ([modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)); not yet wired to any tools

Detectors propose and the policy engine decides: a proposal can only be made
more conservative, never less, and managed data (databases, containers, VMs,
installed apps, app-managed bundles) can never be called safe to delete. See
[How DiskWise Thinks](docs/guides/concepts.md).

## Development

```bash
go build ./...
go test ./...
golangci-lint run
```

Preview the documentation site locally:

```bash
pip install mkdocs-material
mkdocs serve
```

## Release History

- [`CHANGELOG.md`](CHANGELOG.md) — commit-level, generated from [`CHANGELOG.json`](CHANGELOG.json) via [`schangelog`](https://github.com/grokify/structured-changelog)
- [`docs/releases/`](docs/releases/) — per-version release notes
