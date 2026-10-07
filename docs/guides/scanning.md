# Scanning

## Scan a directory

```bash
diskwise scan ~
diskwise scan ~/Library
diskwise scan /Volumes/Archive --cross-device
```

`scan` measures everything under the path and records it in the index. The
path defaults to the current directory, and DiskWise says so on stderr when it
uses that default.

| Flag | Meaning |
|---|---|
| `--depth N` | Stop descending after N levels (0 = unlimited, a full scan). Handy for a fast first look. |
| `--workers N` | Concurrent workers (0 = one per CPU). |
| `--cross-device` | Also descend into directories on other mounted volumes. By default these are skipped and counted. |

Known heavy locations are prioritized during the walk, so they resolve early.

## Read the summary

```text
/Users/you
  status:  partial
  elapsed: 2m7.291s
  dirs:    1927097
  files:   9804954
  denied:  140 inaccessible
           /Users/you/Library/Mail
           ...
           grant your terminal Full Disk Access (System Settings > Privacy & Security), then `diskwise rescan` the paths above
  dedup:   105710 hard-linked files, 4.0 GiB not double-counted
```

- **status** is `complete` only when every directory was measured. Anything
  denied, skipped, or depth-capped makes it `partial`.
- **denied** lists up to ten unreadable directories (the full sample, up to
  200 paths, is in `--json` output). See
  [Troubleshooting](troubleshooting.md#some-directories-were-denied).
- **dedup** shows space not double-counted because of hard links.

## Re-measure part of the tree

After clearing something, update just that subtree instead of rescanning
everything:

```bash
diskwise rescan ~/Library/Caches
```

The change in size is propagated to every ancestor, so `tree` and `savings`
stay consistent.

## Scanning more than once

Scanning a new path adds to the same index. Query commands accept any path
inside something you have scanned, so you can scan `~` once and ask about
`~/Downloads` later. If a path has never been scanned, DiskWise says so and
tells you which `scan` command to run.

!!! note "A stale index"
    The index is a snapshot. It does not watch the filesystem, so rescan
    after large changes. Query output always names the path it covers.
