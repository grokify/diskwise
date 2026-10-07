# Troubleshooting

## `savings` or `hotspots` shows 0 B or very little

Query commands default to the **current directory** when you give no path,
and DiskWise prints which one it used on stderr:

```text
diskwise: no path given; using current directory /Users/you/project
```

If you meant the whole home directory, pass it: `diskwise savings ~`. If the
path was never scanned you get an error telling you which `scan` command to
run.

## "is not indexed; run `diskwise scan ...` first"

The path is not inside anything you scanned. Scan it (or a parent), then
re-run the command.

## Some directories were denied

A scan reports `status: partial` and a `denied:` count when macOS blocked some
folders. The summary lists up to ten of them.

1. Grant your terminal **Full Disk Access** (System Settings > Privacy &
   Security > Full Disk Access), then restart the terminal.
2. Re-measure just those paths with `diskwise rescan <path>`.

`sudo` does not help: these are macOS privacy protections granted per app, not
Unix permissions. Some system-managed locations stay unreadable regardless.

## macOS asks "would like to access your Photos Library" (or Desktop, Documents)

The prompt names the app that launched the scan. Run `diskwise` from
Terminal.app or iTerm instead of another app's embedded terminal, and grant
that terminal Full Disk Access once. See
[Installation](installation.md#give-your-terminal-full-disk-access).

## Findings are marked `[missing]`, or DiskWise warns the scan is old

The index is a snapshot taken when you scanned. DiskWise compares each finding
with the disk and marks ones whose paths have since been removed (for example a
cache you cleaned) as `[missing]`, warns on stderr, and still counts them in
tier totals because the totals describe the index. `savings` also reports how
many bytes that is. Run `diskwise rescan <parent path>` to refresh.

Every result states when it was measured, and warns once the scan is over a
week old. A path that still exists but has changed size is *not* detected, so
rescan after cleaning up and then re-run `savings`.

## `review` or `export` leaves out small findings

Findings under `--min-size` (default `1mb`) are omitted, and the output says how
many and how much. Use `--min-size 0` to list everything.

## `UNKNOWN` is huge

`unknown` is the set of large directories nothing explains. It is **not** a
savings estimate and is excluded from the headline total. Use
`diskwise tree <path> --depth 2` to drill into the biggest ones. If it is a
location other people's machines would share, it may deserve an entry in the
[known-locations registry](known-locations.md).

## `pairs` or `export --pairs` is slow

Archives are read from disk, and `.gz` / `.bz2` must be decompressed in full.
Each archive is announced on stderr as it is read. To speed it up, scope the
path (`diskwise pairs ~/Downloads`), raise `--min-size`, or lower
`--max-compressed`. Plain `.tar` and `.zip` are always fast.

## A cache I deleted came back

That is what `safe_delete` means: it is regenerated on demand (build caches,
package caches). The space returns as you use the tool again.

## Docker's disk did not shrink after pruning

Docker Desktop keeps a sparse VM disk that only shrinks after
`docker system prune` **and** a Docker Desktop restart. DiskWise reports
Docker at allocated (real) size, so `rescan` after the restart.

## `preflight` says there is not enough room, but macOS says there is

macOS can also count purgeable space (caches and snapshots it evicts when
needed), which is not measurable from the command line, so `preflight` does
not include it. Its verdict is conservative; see
[Checking Free Space](upgrade-readiness.md).
