# Reports, Review Lists and Sharing

DiskWise can turn findings into documents. All of them are read-only views of
the index.

## One report document, many renderings

Every report DiskWise produces comes from a single **report document**: one
versioned JSON file (`report.json`) holding the root, when it was measured, the
tier totals, what was filtered out or has vanished from disk, the findings, and
optionally archive comparisons. HTML, spreadsheet, and Markdown are rendered
from it, so they always agree.

```bash
diskwise export ~ --out ./diskwise-export        # report.json + report.html + review.md
diskwise report ~ --format html --out report.html
diskwise report ~ --format xlsx --out report.xlsx
diskwise report ~ --format md   --out review.md
diskwise report ~ --format json --out report.json
```

Rendering is **deterministic**: the same document always gives the same output.
The only time shown is when the data was measured (stored in the document),
never the clock, so regenerating after a rescan produces a clean diff.

### Render a saved document anywhere

Because the document is self-contained, you can render it later or elsewhere,
with no index, no scan, and no access to the files:

```bash
diskwise report --from report.json --format html --out report.html
diskwise report --from report.json --format md   --out review.md
diskwise review --from report.json
```

You can also redact at render time, so one saved document can produce a
shareable copy:

```bash
diskwise report --from report.json --redact-prefix ~/work/clientname --out shareable.html
```

### What each rendering contains

| Format | Contents |
|---|---|
| `html` | A single self-contained page (no external script, stylesheet, or font): notices for stale, partial, vanished, or filtered data; tier totals; the known heavy locations and large unexplained directories; any archive comparisons; and the full sortable, filterable findings table. |
| `md` | The review checklist: findings grouped by tier, each an unticked checkbox. Tick what you want removed and act on it yourself. |
| `xlsx` | The findings, with a frozen header row and autofilter. |
| `json` | The document itself. `diskwise report --schema` prints its [JSON Schema](json-output.md#report-document). |

### Staleness, missing paths, and small findings

The document says **when the data was measured** and flags a scan more than a
week old. Findings whose paths no longer exist are marked _(no longer exists)_.
Findings smaller than `--min-size` (default `1mb`) are left out, and the report
states how many and how much; use `--min-size 0` to list everything. The tier
totals always cover every finding, including any left out of the list.

### Export everything at once

```bash
diskwise export ~ --out ./diskwise-export
diskwise export ~ --out ./diskwise-export --pairs
```

| File | Contents |
|---|---|
| `report.json` | The report document |
| `report.html` | The HTML rendering |
| `review.md` | The checklist rendering |

`--pairs` adds archive/extracted-directory comparisons to the document; they
read archive member lists and can take minutes on a large tree. Existing files
in the directory are overwritten.

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
