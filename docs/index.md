# DiskWise

**Know your disk. Use it wisely.**

DiskWise is a macOS storage-intelligence tool. Ordinary disk analyzers show
you *which files are big*. DiskWise answers the two questions behind that:

1. **Where did my storage actually go?** Space is attributed to meaningful
   things (Docker's VM disk, a Go build cache, a Photos library, five
   versions of the same installer) rather than raw paths.
2. **What can I safely reclaim, and how much?** Reclaimable bytes are
   classified by confidence and risk, then rolled up to the fewest actions
   you need to take.

!!! note "Discovery only"
    DiskWise never deletes, moves, or modifies anything it finds, not even to
    the Trash. It produces an explained worklist; you decide and act.

## A first look

```bash
go install github.com/grokify/diskwise/cmd/diskwise@latest

diskwise scan ~            # measure your home directory (read-only)
diskwise hotspots ~        # where the space is going
diskwise savings ~         # what could be reclaimed, by risk tier
diskwise review ~ --out review.md   # a checklist to work through
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

## What it can do

| You want to... | Use |
|---|---|
| Measure disk usage and keep the result for repeated queries | [`scan`](guides/scanning.md) |
| See the biggest folders, files, and known heavy locations | [`tree`, `largest`, `hotspots`](guides/exploring.md) |
| Get a risk-tiered list of what to clear | [`savings`, `opportunities`](guides/reclaiming.md) |
| Find archives that duplicate an extracted folder | [`pairs`](guides/archive-pairs.md) |
| Check you have room for an OS upgrade | [`preflight`](guides/upgrade-readiness.md) |
| Produce a checklist or shareable report | [`review`, `report`, `export`](guides/sharing.md) |
| Share output without exposing your paths | [`--redact` and `--redact-prefix`](guides/sharing.md#sharing-safely) |

## Principles

- **Accurate first.** The scanner measures both logical and on-disk
  (allocated) size, so sparse files and hard links do not inflate the
  numbers.
- **Conservative by construction.** A detector can propose how reclaimable
  something is, but a separate policy step can only make that proposal *more*
  cautious, never less. Databases, containers, VMs, installed apps, and
  app-managed libraries are structurally barred from being called safe to
  delete.
- **Each byte counted once.** No finding overlaps another, so totals add up.
- **Explainable.** Every finding carries a reason and a confidence, so you can
  see *why* something was suggested.

Read [How DiskWise Thinks](guides/concepts.md) for the model behind this, or
jump to the [Quick Start](guides/quickstart.md).

## Project documents

- [Product Requirements](specs/PRD.md) and [Technical Requirements](specs/TRD.md)
- [Plan](specs/PLAN.md) and [Roadmap](specs/ROADMAP.md)
- [Release notes](releases/v0.1.0.md)
