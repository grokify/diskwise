# Archives and Extracted Copies

People often keep `Downloads.tar` and an extracted `Downloads/` side by side
and later cannot tell whether the archive is still needed. `pairs` answers
that without extracting anything.

```bash
diskwise pairs ~/Downloads
```

For every readable archive that has a **directory of the same name beside
it**, DiskWise compares the archive's member list with the directory's
indexed contents.

```text
3.1 GiB    same              /Users/you/Backup/Downloads.tar
           beside directory /Users/you/Backup/Downloads (3.1 GiB)
           compared all files: archive 3380 files (3.1 GiB), directory 3380 files (3.1 GiB)
```

## Verdicts

| Verdict | Meaning |
|---|---|
| `same` | Same file count and same total bytes. Strong evidence one copy is redundant. |
| `same_count` | Same file count but different total bytes. Contents differ somewhere. |
| `archive_has_more` | The archive lists more files than the directory holds. **Removing the archive could lose files.** |
| `dir_has_more` | The directory has more files than the archive (it grew, or the archive is partial). |
| `skipped` | A large compressed archive was not read (see `--max-compressed`). |
| `unreadable` | The archive could not be read. The reason is shown. |

!!! warning "This is evidence, not proof"
    Contents are never hashed. `same` means the counts and sizes line up,
    which is very strong evidence but not a guarantee. For anything you care
    about, spot-check before deleting either side.

## Supported formats

`.tar`, `.tar.gz` / `.tgz`, `.tar.bz2` / `.tbz2`, and `.zip`. Other formats
are ignored.

- Plain `.tar` and `.zip` are read by their headers, so even very large ones
  are quick.
- `.gz` and `.bz2` must be decompressed end to end. Archives larger than
  `--max-compressed` (default `8gib`; `0` means no cap) are reported as
  `skipped`.
- Directories and macOS `._*` AppleDouble sidecar files are not counted,
  because extraction folds those into extended attributes.

DiskWise announces each archive on stderr as it reads it, so a long run is not
silent.

## Photos libraries

A Photos library keeps the **same original photos** in a different internal
layout from one app version to the next (older libraries use `Masters/`,
newer ones `originals/`). Comparing every file would report thousands of
phantom differences, so when either side contains a `.photoslibrary`, DiskWise
compares **original media only** and says so:

```text
98.8 GiB   archive_has_more  /Users/you/Backup/Pictures.tar
           beside directory /Users/you/Backup/Pictures (98.1 GiB)
           compared photos library originals only: archive 22008 files (89.7 GiB), directory 21747 files (89.2 GiB)
           the archive lists 261 more file(s) than the directory holds; check before removing it
```

Here the extracted library is not a complete replacement for the archive: 261
more originals exist in the archive. They may be photos deleted after a
migration, but only you can tell. Extract just those files and look before
discarding the archive.

## Options

| Flag | Meaning |
|---|---|
| `--min-size` | Skip archives smaller than this (default `10mb`). |
| `--max-compressed` | Largest gzip/bzip2 archive to read (default `8gib`; `0` = no cap). |
| `--redact`, `--redact-prefix` | See [Sharing safely](sharing.md#sharing-safely). |

`review --pairs` and `export --pairs` include the same comparisons in their
output.
