# Exploring Usage

These commands read the index and never touch the disk.

## Summary of a path

```bash
diskwise summary ~/Library
```

Shows the logical and allocated size of the path, when and how completely it
was scanned, any known locations inside it, and the capacity of the volume.

## Subtree totals

```bash
diskwise tree ~ --depth 2
diskwise tree ~/go --depth 3 --min-size 1gb
```

Prints each folder with its size, largest first. `--depth` controls how many
levels of children are shown (default 2); `--min-size` hides small entries
(`100mb`, `1gb`, `2gib`).

## Largest items

```bash
diskwise largest ~ --files --limit 30
diskwise largest ~ --dirs
```

Lists the largest files and/or directories under a path.

## Hotspots

```bash
diskwise hotspots ~
```

`hotspots` combines two views:

- **Known locations** that DiskWise recognizes (see
  [Known Locations](known-locations.md)), each with its path, size, and
  action tier. Several caches of the same type appear as separate rows, so
  you can tell exactly which directories they are.
- **Large unexplained directories**: big folders no detector explains, shown
  at their outermost level with already-explained bytes subtracted. They are
  a prompt to investigate with `tree`, not a recommendation.

| Flag | Meaning |
|---|---|
| `--min-size` | Hide unexplained entries below this size (default `1gb`). |
| `--limit` | Maximum unexplained entries (default 20). |

!!! tip "Drilling down"
    When `hotspots` flags a large unexplained directory, run
    `diskwise tree <that path> --depth 2` to see where inside it the space
    sits.
