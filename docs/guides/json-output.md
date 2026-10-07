# JSON Output

Every command accepts `--json` for scripting. The JSON is the contract shared by the CLI and the Go library (`service`
package), and may still change before 1.0. Field names are exactly the Go
field names (`PascalCase`), sizes are in **bytes**, and times are RFC 3339.

!!! tip
    Pipe through `jq`, for example the five largest safe-delete findings:

    ```bash
    diskwise opportunities ~ --action safe_delete --json \
      | jq -r '.[:5][] | "\(.Finding.AllocatedSize)\t\(.Finding.Path)"'
    ```

## Finding

The unit behind `opportunities`, `hotspots`, and the review list:

```json
{
  "Entity": {
    "ID": "/Users/you/Library/Caches/go-build",
    "Kind": "cache",
    "Name": "Go module cache and build cache",
    "Detector": "known-location:go-caches",
    "Confidence": 1
  },
  "Path": "/Users/you/Library/Caches/go-build",
  "Paths": ["/Users/you/Library/Caches/go-build"],
  "LogicalSize": 43700000000,
  "AllocatedSize": 43700000000,
  "Confidence": 1,
  "ActionClass": "safe_delete",
  "Reason": "Go module cache and build cache — regeneratable cache",
  "Scenarios": []
}
```

| Field | Meaning |
|---|---|
| `Path` | Anchor path (for a multi-file finding, the containing directory). |
| `Paths` | Every indexed node the finding covers. |
| `LogicalSize`, `AllocatedSize` | Claimed size and size actually occupied on disk. |
| `ActionClass` | `safe_delete`, `likely_safe`, `backup_then_delete`, `review`, `keep`, or `unknown`. |
| `Confidence` | Detection confidence, 0 to 1. |
| `Scenarios` | Optional alternative outcomes: `Name`, `Description`, `ReclaimableBytes`, `Paths`. |

## Per command

| Command | Top-level shape |
|---|---|
| `scan`, `rescan` | `{"Run": {ID, Root, StartedAt, FinishedAt, Status}, "Stats": {DirsScanned, FilesScanned, SymlinksScanned, DeniedPaths, CrossDeviceSkipped, DuplicateFiles, DuplicateBytesSaved, DeniedSample}}`. `DeniedSample` lists up to 200 unreadable directories. |
| `summary` | `{Path, Kind, LogicalSize, AllocatedSize, State, ScanRun, KnownLocations, Volume}` |
| `tree` | Nested `{Path, Name, Kind, LogicalSize, AllocatedSize, State, Children}` |
| `largest` | Array of `{Path, Name, Kind, LogicalSize, AllocatedSize}` |
| `hotspots` | `{"Root", "Known": [Finding], "Unexplained": [Finding]}` |
| `savings` | `{"Path", "Tiers": {"safe_delete": bytes, ...}}` |
| `opportunities` | Array of `{"Finding": Finding, "Paths": [rolled-up actionable paths]}` |
| `pairs` | Array of archive pairs, below |
| `preflight` | Preflight result, below |

`opportunities --json` is a **bare array**. The files written by
[`export`](sharing.md#export-everything-at-once) wrap it as
`{"Root": ..., "Opportunities": [...]}` so each file records what it was
computed for.

## Archive pair

```json
{
  "Archive": "/Users/you/Backup/Downloads.tar",
  "ArchiveAlloc": 3300000000,
  "Dir": "/Users/you/Backup/Downloads",
  "DirAlloc": 3300000000,
  "Compared": "all files",
  "ArchiveFiles": 3380,
  "ArchiveBytes": 3300000000,
  "DirFiles": 3380,
  "DirBytes": 3300000000,
  "Verdict": "same",
  "Detail": "",
  "ArchiveExtra": 0
}
```

`Verdict` is `same`, `same_count`, `archive_has_more`, `dir_has_more`,
`skipped`, or `unreadable`. `Compared` is `all files` or
`photos library originals only`. `ArchiveExtra` is how many more files the
archive lists than the directory holds, when positive. See
[Archives and Extracted Copies](archive-pairs.md).

## Preflight result

```json
{
  "Path": "/",
  "MountPoint": "/",
  "FSType": "apfs",
  "CapacityBytes": 1995218165760,
  "AvailableBytes": 255000000000,
  "ContainerFreeBytes": 255000000000,
  "LocalSnapshots": [],
  "NeedBytes": 53687091200,
  "Sufficient": true,
  "ShortfallBytes": 0,
  "Notes": ["purgeable space ... is not counted"]
}
```

## Redaction

With `--redact` or `--redact-prefix`, path fields (and names and reasons
derived from redacted paths) are rewritten before JSON is produced, so the
shapes above are unchanged. See
[Sharing safely](sharing.md#sharing-safely).

## The insights schema

The narrative `insights` document has its own JSON Schema, generated from the
Go types: run `diskwise insights schema`.
