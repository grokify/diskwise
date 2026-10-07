# Finding Reclaimable Space

## Savings by tier

```bash
diskwise savings ~
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

The headline number counts only tiers you could actually reclaim
(`safe_delete`, `likely_safe`, `backup_then_delete`, `review`). `keep` and
`unknown` are shown so every byte is explained, but they are not savings. See
[action tiers](concepts.md#action-tiers).

## The full list

```bash
diskwise opportunities ~
```

Each finding shows its size, kind, path, and the **reason** it was reported.
Findings that offer a choice also list *scenarios*, for example "keep the
newest, remove the other five" with the bytes each would free.

### Filter it

```bash
diskwise opportunities ~ --action safe_delete        # one tier
diskwise opportunities ~ --type artifact_family      # one kind of thing
diskwise opportunities ~ --min-confidence 0.8        # only well-supported findings
diskwise opportunities ~ --paths                     # just the actionable paths, one per line
```

Entity kinds include `cache`, `archive`, `artifact_family`, `model`,
`container`, `application`, `managed_bundle`, and `unknown`.

Output is rolled up to the **highest fully-covered directory**: if every item
in a folder is part of a finding, you get one path to remove instead of a
scattered file list. A folder that is only partly covered stays decomposed
into its individual parts.

## What the main findings mean

### Known locations

Caches and heavy directories DiskWise recognizes by path: Go, npm/pnpm/Yarn,
pip/uv, Homebrew downloads, Xcode DerivedData, and browser caches are
`safe_delete`. Local AI models, Docker's VM disk, iOS backups, and simulators
are `review`. See [Known Locations](known-locations.md) for the whole table.

### Archives and installers

Downloaded `.zip`, `.dmg`, `.pkg`, `.tar`, `.tar.gz`, and similar files,
classified by evidence:

| Evidence | Tier | Confidence |
|---|---|---|
| A newer `.app` with a matching name is installed | `safe_delete` | 0.9 |
| A directory with the same name sits beside it | `likely_safe` | 0.7 |
| Name looks like a deliberate backup or export | `review` | 0.2 |
| No evidence either way | `review` | 0.3 |

For a closer check of the "sits beside an extracted copy" case, use
[`pairs`](archive-pairs.md).

### Artifact families

Several downloaded versions of the same product (`tool-1.0.zip`,
`tool-2.0.zip`, ...) are grouped into one finding with *keep newest* and
*remove all* scenarios, instead of five unrelated files.

### Managed bundles

App-managed packages (Photos, Music, TV, iMovie, Final Cut, Logic,
GarageBand libraries, sparse bundles) are reported as `keep`. DiskWise
explains their size but treats the inside as the owning app's business:
manage them from the app, not by deleting internals. Only the outermost
bundle is reported, and nothing inside it is reported separately.

### Unexplained directories

Large folders nothing else explains appear in the `unknown` tier at their
outermost level, with the bytes other findings already account for
subtracted. They are a prompt to look, not a recommendation.

## Acting on the results

DiskWise stops at the list. To act:

- Re-read the **reason** for anything you are unsure about.
- For caches, prefer the tool's own cleaner when it has one, for example
  `go clean -cache -modcache`, `npm cache clean --force`, or
  `docker system prune`. Docker's disk file only shrinks after the prune and a
  Docker Desktop restart.
- After cleaning, run `diskwise rescan <path>` and `diskwise savings` to
  confirm the space came back.
