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

Early scaffold. The scanner, index, detectors, and CLI/MCP commands
described in the specs are not yet implemented.

## Components

- `github.com/grokify/diskwise` — Go library (scanner, detectors,
  policy engine, service layer)
- `cmd/diskwise` — Cobra CLI
- `cmd/diskwise-mcp` — MCP server ([modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk))

## Development

```bash
go build ./...
go test ./...
golangci-lint run
```
