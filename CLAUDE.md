# CLAUDE.md

Project-specific guidance for working in `diskwise`. This supplements
(never overrides) `~/.claude/CLAUDE.md` and the `grokify` org
CLAUDE.md — see those for generic Go/commit/schema conventions.

## Scope boundary: discovery only

V1 never deletes, moves, or modifies anything it finds — not even to
Trash. Don't add deletion/cleanup-execution features without being
explicitly asked; that's tracked as unphased backlog in
`docs/specs/ROADMAP.md`, not an oversight.

## Architecture: scanner discovers, detector proposes, policy decides

Three layers, strictly separated:

1. `scan/` + `index/` discover bytes — accurate, mechanical, no
   judgment calls.
2. `detect/` proposes what those bytes mean (a `Finding` with a
   proposed `policy.ActionClass`).
3. `policy.Evaluate()` is the only place that can promote or demote a
   proposal, and it can only ever make it **more conservative**, never
   less. Managed data (`entity.KindDatabase`, `KindContainer`,
   `KindVM`, `KindApplication`) is structurally capped at
   `backup_then_delete` with no override; `safe_delete` requires ≥0.8
   confidence (`policy.MinConfidenceForSafeDelete`) or gets downgraded.

Every detector's proposed `ActionClass` must be routed through
`policy.Evaluate` before it reaches a `Finding` — never assign
`ActionClass` directly.

## No-double-counting contract

`service.Opportunities` runs `KnownLocationDetector`,
`ArchiveInstallerDetector`, `ArtifactFamilyDetector`, and
`LargeUnexplainedDetector` together, and no indexed path may appear in
more than one finding's `Paths`. If you add a new detector or change
an existing one's matching logic:

- Exclude any path already claimed by another detector
  (`isClaimed`/`claimedPaths` in `detect/`).
- If your detector could overlap with `ArchiveInstallerDetector` or
  `ArtifactFamilyDetector` (both operate on the same
  `FilesByExtensions` result set), defer to the family threshold
  (`minFamilyMembers` in `detect/archive.go`) the same way they do.
- Add or extend the fixture in
  `service/opportunities_test.go:TestService_Opportunities_NoDoubleCounting`
  to cover the new overlap risk — this is the test that would have
  caught a regression here.

## Testing: never depend on real machine state

`knowledge.Default` and the real `/Applications` reflect whatever
happens to be installed on the machine running the test. Tests must
use an injected registry (see `fixtureRegistry`/`fixtureRegistryFor`
helpers in `service` and `detect` test files) or an injectable field
(`ArchiveInstallerDetector.ApplicationsDir`) instead. If you add a
detector or service method that reads `knowledge.Default` or a fixed
OS path, make it injectable the same way.

## Insights IR: Go structs are the source of truth

`insights.Report` (and its JSON Schema) exist so an LLM agent can
write a narrative analysis once and regenerate Markdown/HTML from it
forever without redoing the analysis. If you change
`insights/types.go`:

```bash
go run insights/schema/gen/main.go
schemakit lint --property-case camelCase insights/schema/insights.schema.json
```

`insights/schema/gen/main.go` is `//go:build ignore`; its
`invopop/jsonschema` dependency is kept in `go.mod` only via the
blank import in `insights/schema/gen/tools.go` — don't remove that
file or `go mod tidy` will silently strip the dependency.

## Roadmap

RMI slug: `DISKWISE`. Phases and RMI IDs are tracked in
`docs/specs/ROADMAP.md`; commits implementing one carry the trailer
`Refs: RMI-DISKWISE-NNN` (see the org CLAUDE.md for the general
convention). Phase status there was reconciled against actual code
state as of v0.1.0 (RMI-001 through 015 and the unplanned Phase 6
RMI-026/027 marked `done`; Phase 4/5 still `planned`). It is not
automatically kept in sync with implementation — re-verify status
against the code before trusting a label, and update the table when
you notice drift instead of leaving it stale for the next session.
