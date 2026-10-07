# Quick Start

DiskWise works in two steps: **scan** a directory into a local index, then
**query** that index as often as you like without walking the disk again.
Everything here is read-only.

## 1. Scan

```bash
diskwise scan ~
```

A full home-directory scan takes a couple of minutes and shows live progress.
The summary reports how many directories and files were measured, and lists
any directories it could not read. See [Scanning](scanning.md) if some were
denied.

## 2. See where the space is

```bash
diskwise hotspots ~
```

`hotspots` lists **known heavy locations** (Docker's VM disk, Go and npm
caches, local AI models, simulators, and so on) with their paths, plus
**large unexplained directories** worth a look. For a plain size breakdown:

```bash
diskwise tree ~ --depth 2
diskwise largest ~ --dirs
```

## 3. See what could be reclaimed

```bash
diskwise savings ~
diskwise opportunities ~ --action safe_delete
```

Findings are grouped into [risk tiers](concepts.md#action-tiers). Start with
`safe_delete` (regenerates on demand) and work down.

## 4. Build a checklist

```bash
diskwise review ~ --out review.md
```

You get a Markdown checklist grouped by tier. Tick what you want gone and
delete it yourself, in Finder or the shell. DiskWise will not do it for you.

## 5. Re-measure after cleaning up

```bash
diskwise rescan ~/Library/Caches
diskwise savings ~
```

`rescan` re-walks only that subtree and updates the totals above it, so you
can confirm the space came back without a full rescan.

## Next steps

- Understand the model: [How DiskWise Thinks](concepts.md)
- Preparing for an OS upgrade: [Checking Free Space](upgrade-readiness.md)
- Looking for duplicate archives: [Archives and Extracted Copies](archive-pairs.md)
- Sending results to someone else: [Sharing](sharing.md)
