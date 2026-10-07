# Installation

DiskWise targets macOS (Apple Silicon and Intel). Linux is not supported in
V1.

## Install the CLI

```bash
go install github.com/grokify/diskwise/cmd/diskwise@latest
```

This needs a recent Go toolchain and puts `diskwise` in `$(go env GOPATH)/bin`
(usually `~/go/bin`). Make sure that directory is on your `PATH`:

```bash
diskwise --version
```

## Build from source

```bash
git clone https://github.com/grokify/diskwise
cd diskwise
go build -o diskwise ./cmd/diskwise
```

## Give your terminal Full Disk Access

macOS protects folders such as Desktop, Documents, Downloads, Photos, and
parts of `~/Library`. Without permission, a scan skips them and reports the
count as *denied*.

!!! tip "Run DiskWise from Terminal or iTerm"
    macOS attributes file access to the **app that launched the scan**. If you
    run `diskwise` from another app's embedded terminal, that app (not your
    terminal) receives the permission prompts for Photos, Desktop, and so on.
    Run it from Terminal.app or iTerm instead.

To make scans complete:

1. Open **System Settings > Privacy & Security > Full Disk Access**.
2. Add your terminal app and switch it on.
3. Quit and reopen the terminal.

`sudo` does **not** bypass these macOS privacy protections, so there is no
need to run DiskWise as root.

## Where DiskWise keeps its data

DiskWise stores the scan result in a local SQLite index:

```text
~/Library/Application Support/DiskWise/index.db
```

Pass `--db path/to/other.db` to any command to use a different index, for
example to keep a separate index per machine or per experiment. The index
contains the paths and sizes of what you scanned, so treat it like any file
listing of your disk.
