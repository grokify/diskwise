# How DiskWise Thinks

DiskWise separates **measuring**, **interpreting**, and **deciding** into
three layers. Keeping them apart is what makes the numbers trustworthy.

```text
scan + index   ->   detectors   ->   policy
(measure bytes)     (propose)        (decide, only ever more cautious)
```

## 1. Scan and index: measuring

The scanner walks the tree and records, for every file and directory, both
sizes that matter on macOS:

- **Logical size**: what the file claims to be.
- **Allocated size**: what it actually occupies on disk.

They differ for sparse files (Docker's VM disk can claim 100 GB and use
20 GB) and are what lets DiskWise report real savings. Hard-linked files are
counted once. The result is stored in a SQLite index, so later commands
answer instantly.

The scanner makes no judgment calls. It never decides whether something is
important.

## 2. Detectors: interpreting

Detectors read the index and propose what bytes *mean*:

| Detector | Finds |
|---|---|
| Known locations | Heavy locations DiskWise recognizes, from a built-in [registry](known-locations.md) |
| Managed bundles | App-managed packages such as a Photos library |
| Archives and installers | Downloaded `.zip`, `.dmg`, `.tar.gz`, and similar, with evidence of whether they were already used |
| Artifact families | Several downloaded versions of the same product |
| Large unexplained | Big directories nothing else explains: a prompt to investigate, never a recommendation |

Each finding has a **confidence** between 0 and 1 and a plain-language
**reason**.

## 3. Policy: deciding

A detector *proposes* an action class. The policy step is the only place that
can change it, and it can only make it **more conservative**:

- Managed data (databases, containers, VMs, installed applications, and
  app-managed bundles) is capped at `backup_then_delete`. There is no
  override.
- `safe_delete` requires confidence of at least 0.8, otherwise it is
  downgraded to `likely_safe`.

## Action tiers

Tiers run from most to least actionable:

| Tier | Meaning |
|---|---|
| `safe_delete` | High-confidence regenerable data: build caches, package-manager caches, browser caches. Removing it costs only rebuild or re-download time. |
| `likely_safe` | Probably unneeded, with lower confidence, for example an archive that sits beside its extracted copy. Skim before removing. |
| `backup_then_delete` | Live data you could remove only after backing it up. Never promoted higher. |
| `review` | No strong evidence either way. Decide item by item. |
| `keep` | Explained but not for removal here, such as an app-managed bundle. |
| `unknown` | Large directories that nothing explains. Not a savings estimate. |

## Each byte is counted once

No finding overlaps another, so tier totals add up and never exceed what the
scanned folder holds:

- Detectors exclude each other's territory.
- "Unexplained" directories are reported only at their outermost level, and
  any bytes another finding already accounts for are subtracted from them.
- Everything inside a managed bundle belongs to the bundle.

This is why `savings` reports a **reclaimable** total that excludes `keep` and
`unknown`: those tiers explain where space went, but do not count as savings.

## Discovery only

DiskWise has no delete, move, or clean command, by design. It produces
evidence and a worklist; acting on it is always a deliberate, manual step.
