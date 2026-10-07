# Checking Free Space for an Upgrade

Before a major macOS upgrade, check that the volume has room for the
installer and its working space.

```bash
diskwise preflight --need 50gib
```

```text
/ (apfs, mounted at /)
  capacity:   1.8 TiB
  available:  237.5 GiB
  container:  237.5 GiB free (shared by all volumes in the APFS container)
  snapshots:  0 local

OK: 237.5 GiB available, 50.0 GiB needed
  note: purgeable space (caches and snapshots macOS evicts on demand) is not measurable from the command line and is not counted; the verdict uses space available right now
```

`preflight` reads the live system, not the index, so you do not need to scan
first. Without `--need` it simply reports the numbers.

## What it reports

- **available**: space the system reports as free for ordinary use right now.
  The verdict is based on this number alone.
- **container**: free space of the APFS container. Every volume in a container
  shares one pool, so this can be larger than a single volume suggests.
- **snapshots**: Time Machine local snapshots. macOS can reclaim these on its
  own when an installer needs room, so a shortfall with snapshots present may
  be smaller in practice.

!!! note "Purgeable space is not counted"
    macOS can also evict caches on demand ("purgeable" space), but that figure
    is not available from the command line. DiskWise says so rather than
    guessing. The result is conservative: the real headroom may be larger.

## Use it in a script

`preflight` exits non-zero when the volume is short, so it can gate a step:

```bash
diskwise preflight --need 50gib || echo "free some space first"
```

Size values accept `kb`, `mb`, `gb`, `tb` and binary `kib`, `mib`, `gib`,
`tib` (all case-insensitive). Use `--json` for machine-readable output.

## If you are short

1. `diskwise scan ~` then `diskwise savings ~` to see what could be freed.
2. Start with `safe_delete`: caches that rebuild themselves. For example
   `go clean -cache -modcache` alone often frees tens of GiB on a developer
   machine.
3. Work through `review` items with `diskwise review ~ --out review.md`.
4. Re-run `preflight --need ...` to confirm.
