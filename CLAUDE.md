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

The contract also holds at the **byte** level for unexplained
directories, not just path lists: `LargeUnexplainedDetector` reports
only outermost, mutually disjoint directories, and
`service.excludeExplained` subtracts bytes any other finding already
reports (dropping directories left with nothing unexplained). A new
detector's claimed paths flow into that subtraction via
`Opportunities`; `TestService_Opportunities_UnexplainedExcludesExplainedBytes`
is the guard. `Savings` totals must never exceed what the scanned root
holds, and `SavingsByTier.Reclaimable()` deliberately excludes the
`keep` and `unknown` tiers.

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

## Report document: Go structs are the source of truth

`reportdoc.Document` is the one file every report rendering (HTML, XLSX,
Markdown) is a pure function of. Renderers read the document and nothing
else, and must not call the clock: the only time shown is
`measuredAt`. If you change `reportdoc/types.go`, regenerate and lint the
schema (properties are camelCase per the document-format convention):

```bash
go run reportdoc/schema/gen/main.go
schemakit lint --property-case camelCase reportdoc/schema/reportdoc.schema.json
```

`reportdoc/schema/gen/main.go` is `//go:build ignore`; its dependency is kept
by `reportdoc/schema/gen/tools.go`, like the insights generator. A test fails
when the embedded schema's properties drift from the types. Redaction happens
once, on the document (`redact.Redactor.Document`), before rendering.

## Releasing

Before tagging, besides the org checklist, confirm every commit hash in
`CHANGELOG.json` is reachable from `main`:

```bash
go test -count=1 -run TestRepoChangelog ./internal/changelogcheck/
```

CI being green does not cover this (the shared Go CI checks out one commit),
so a tag can ship with dead changelog links. The `Changelog References`
workflow runs the same check on full history for pushes, PRs and `v*` tags.

**Record the release commit after tagging.** Commits often land between the
last documentation change and the tag, so the commit a release will be tagged
on cannot be predicted. Write the new release's top-level `commit` in
`CHANGELOG.json` as the last substantive commit before tagging (the check
accepts any reachable hash), then after tagging set it to
`git rev-parse --short=7 vX.Y.Z^{commit}`, add any commits that landed late
(for example a dependency bump) as entries, regenerate `CHANGELOG.md`, and push
that as a follow-up commit. Change only the release-level `commit`; entries
keep the commits they describe.

`schangelog generate` collapses dependency lists to a count ("_1 dependency
update_"). The committed `CHANGELOG.md` renders them as explicit bullets, so
re-expand them after regenerating.

## Roadmap

RMI slug: `DISKWISE`. Phases and RMI IDs are tracked in
`docs/specs/ROADMAP.md`; commits implementing one carry the trailer
`Refs: RMI-DISKWISE-NNN` (see the org CLAUDE.md for the general
convention). Phase status there was reconciled against actual code
state as of v0.2.0 (RMI-001 through 015, the unplanned Phase 6
RMI-026/027, and Phases 7-9 RMI-028 through 040 marked `done`; Phase 4/5
still `planned`). It is not
automatically kept in sync with implementation — re-verify status
against the code before trusting a label, and update the table when
you notice drift instead of leaving it stale for the next session.
