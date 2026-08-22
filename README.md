# DiskWise

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
