# CLI Reference

```text
diskwise [command] [flags]
```

## Global flags

| Flag | Meaning |
|---|---|
| `--db PATH` | Index to use (default `~/Library/Application Support/DiskWise/index.db`). |
| `--json` | Print JSON instead of a table. See [JSON Output](json-output.md). |
| `-v`, `--version` | Print the version. |
| `-h`, `--help` | Help for any command. |

Commands that take `[path]` default to the current directory, and announce it
on stderr when they do.

## Measuring

| Command | Purpose |
|---|---|
| `scan [path]` | Measure a directory and record it in the index. Flags: `--depth N`, `--workers N`, `--cross-device`. |
| `rescan <path>` | Re-walk one subtree and propagate the size change to its ancestors. |

## Exploring

| Command | Purpose |
|---|---|
| `summary [path]` | What is known about a scanned path. |
| `tree [path]` | Subtree totals, largest first. Flags: `--depth N` (default 2), `--min-size SIZE`. |
| `largest [path]` | Largest files and directories. Flags: `--files`, `--dirs`, `--limit N` (default 20). |
| `hotspots [path]` | Known heavy locations plus large unexplained directories. Flags: `--min-size` (default `1gb`), `--limit` (default 20), redaction flags. |

## Reclaiming

| Command | Purpose |
|---|---|
| `savings [path]` | Potential savings by tier. Redaction flags. |
| `opportunities [path]` | Every finding, grouped by tier. Flags: `--action TIER`, `--type KIND`, `--min-confidence 0-1`, `--min-size SIZE` (default: no cut), `--paths`, redaction flags. `--json` prints the [opportunities report](json-output.md#opportunities-report). |
| `pairs [path]` | Compare archives with extracted directories beside them. Flags: `--min-size` (default `10mb`), `--max-compressed` (default `8gib`, `0` = no cap), redaction flags. |

## Preparing and sharing

| Command | Purpose |
|---|---|
| `preflight [path]` | Check free space against `--need SIZE` (path defaults to `/`). Exits non-zero when short. |
| `review [path]` | Markdown checklist. Flags: `--format md`, `--out FILE`, `--min-size SIZE` (default `1mb`; `0` lists everything), `--pairs`, redaction flags. |
| `report [path]` | Self-contained HTML or XLSX report. Flags: `--format html\|xlsx`, `--out FILE`, redaction flags. |
| `export [path]` | Write savings, opportunities, hotspots, review (and pairs) to a directory. Flags: `--out DIR` (required), `--min-size SIZE` (default `1mb`), `--pairs`, redaction flags. |
| `insights schema` / `insights render FILE` | Print the narrative-report JSON Schema, or render a report document. `render` takes `--format md\|html` and `--out FILE`. |

## Redaction flags

Accepted by `opportunities`, `hotspots`, `savings`, `pairs`, `report`,
`review`, and `export`:

| Flag | Meaning |
|---|---|
| `--redact` | Show the home directory as `~`. |
| `--redact-prefix PATH` | Replace every path under `PATH` with an opaque token. Repeatable; implies `--redact`. |

## Sizes

Wherever a size is accepted (`--min-size`, `--need`, `--max-compressed`) you
can use a plain byte count or a number with a suffix: `k`, `kb`, `kib`, `m`,
`mb`, `mib`, `g`, `gb`, `gib`, `t`, `tb`, `tib` (case-insensitive, always
1024-based).

## Exit status

`0` on success. `1` on any error, including `preflight` finding too little
free space.
