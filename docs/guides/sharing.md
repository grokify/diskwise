# Reports, Review Lists and Sharing

DiskWise can turn findings into documents. All of them are read-only views of
the index.

## Review checklist

```bash
diskwise review ~ --out review.md
diskwise review ~ --pairs --out review.md      # also compare archives with extracted copies
```

A Markdown checklist grouped by tier, most actionable first. Every finding
is an unticked checkbox with its size, kind, path, and reason. Tick what you
want removed and act on it yourself.

The document states **when the data was measured** and flags a stale scan.
Findings whose paths no longer exist are marked _(no longer exists)_. Findings
smaller than `--min-size` (default `1mb`) are left out, and the document says
how many and how much; use `--min-size 0` to list everything.

The only time shown is the scan time recorded in the index, never the clock,
so regenerating after a rescan gives a clean diff and the file is safe to keep
under version control.

## HTML and spreadsheet reports

```bash
diskwise report ~ --format html --out report.html   # self-contained, sortable and filterable
diskwise report ~ --format xlsx --out report.xlsx   # frozen header, autofilter
```

The HTML report is a single file with no external resources, so it opens
anywhere and can be attached to an email.

## Export everything at once

```bash
diskwise export ~ --out ./diskwise-export
diskwise export ~ --out ./diskwise-export --pairs
```

Writes these files into the directory:

| File | Contents |
|---|---|
| `savings.json` | Savings by tier |
| `opportunities.json` | The [opportunities report](json-output.md#opportunities-report): findings plus root, scan time, and what `--min-size` omitted |
| `hotspots.json` | Known locations and large unexplained directories |
| `review.md` | The checklist above |
| `pairs.json` | Archive comparisons (with `--pairs`) |

Every JSON file records the root it was computed for, so output generated for
the wrong directory is recognizable. `--min-size` (default `1mb`) applies to
`opportunities.json` and `review.md`, and both record what it omitted; savings
totals always include everything. See [JSON Output](json-output.md). Existing
files in the directory are overwritten.

## Sharing safely

Findings contain real paths, which can reveal your username, project names, or
client names. `opportunities`, `hotspots`, `savings`, `pairs`, `report`,
`review`, and `export` accept two flags:

```bash
diskwise export ~ --out out --redact
diskwise export ~ --out out --redact-prefix ~/work/clientname --redact-prefix ~/Documents/legal
```

| Flag | Effect |
|---|---|
| `--redact` | Shows your home directory as `~`. |
| `--redact-prefix PATH` | Replaces every path under `PATH` with a stable opaque token such as `<redacted:1a2b3c4d>`. Repeatable. Implies `--redact`. |

For a path under a redacted prefix, DiskWise also scrubs the names and
descriptions derived from it (file names in reasons and scenarios, the entity
name), and the prefix itself wherever it appears in text. Tokens are derived
from the original path, so the same file keeps the same token across outputs
and you can still correlate them, but the original cannot be read back.

!!! warning "Redaction is prefix-based"
    Only paths under the prefixes you name are replaced. A sensitive name that
    appears in a path outside those prefixes (say, a client's name in a file in
    `~/Downloads`) is left as is. Name every prefix that matters, and check
    the output before sharing it.

## Narrative write-ups (insights)

DiskWise's detectors never guess. Judgments such as "this looks like a
duplicate backup" need a human or an LLM agent reading the raw output.
`insights` gives that author a typed place to write the analysis and turns it
back into a document deterministically:

```bash
diskwise insights schema                                    # the JSON Schema to fill in
diskwise insights render insights.json --format md --out insights.md
diskwise insights render insights.json --format html --out insights.html
```

Re-running `render` never redoes the analysis. It is a pure function of the
JSON document.
