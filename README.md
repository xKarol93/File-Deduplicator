# File Deduplicator

A fast, concurrent file deduplicator that scans a directory tree, computes cryptographic hashes (MD5/SHA1/SHA256/SHA512) for all files, detects duplicates, and writes them to `duplicate_files.txt`. The CLI in this repository is standalone. A Fyne-based GUI exists in a separate project (not part of this GitHub repo). Both implementations share the same scanning and hashing logic so results are consistent.

## Features
- Recursive directory scan
- Concurrent hashing with a worker pool
- Selectable hash algorithm (default: SHA512)
- Duplicate detection with total potential space savings
- Outputs duplicates list to `duplicate_files.txt`
- Optional GUI (Fyne) exists as a separate project and is not included in this repository.

## Requirements
- Go 1.20+

## Project Layout
- CLI implementation: this folder
  - `file_explorer.go`: core dedup logic (scan + hashing + duplicate detection)
- GUI implementation (optional): separate project (not included here)
  - `ui.go`: Fyne-based UI that calls `ScanDirectory`
  - `file_explorer.go`: same dedup logic reused by the GUI

Shared code: the `ScanDirectory` function and hashing pipeline are identical in both the CLI and GUI, ensuring parity of duplicate detection and performance characteristics.

## Build & Run (CLI)
From the current folder (`File-Deduplicator`):

```bash
# Initialize module (first time)
go mod init filededuplicator
go mod tidy

# Run the deduplicator (default SHA512) via CLI build tag
# (uses cli_main.go; excludes GUI files)
go run -tags cli . /path/to/scan sha512

# Or run by listing files (without tags)
# (explicitly compiles only CLI files)
go run cli_main.go file_explorer.go /path/to/scan sha256

# Optional: delete duplicates after scan
# (reads duplicate_files.txt and removes files)
go run -tags cli . /path/to/scan sha512 --delete

# Build a CLI binary
# (outputs ./dedup in the current directory)
go build -tags cli -o dedup
./dedup /path/to/scan sha512
```

Notes:
- The CLI expects two args: directory and algorithm.
- It writes duplicates to `duplicate_files.txt` in the current working directory.
- Prefer `-tags cli` to include the CLI entrypoint (`cli_main.go`) and exclude GUI files.
- If you see "no required module provides package" for Fyne, you're running GUI files from the wrong folder — use the GUI instructions below.

## Quick Start

```bash
# From File-Deduplicator
go mod init filededuplicator && go mod tidy
go run -tags cli . /path/to/scan sha512
# or with deletion
go run -tags cli . /path/to/scan sha512 --delete
```

## Run the GUI (optional)
The GUI is a separate project and not part of this repository. If you have the GUI project locally, run it from its own module folder. If you keep GUI code in this folder, ensure `ui.go` is guarded with a build tag (e.g., `//go:build gui`) so CLI runs don’t include GUI deps.

```bash
# From the GUI project folder
# run the module that contains ui.go and file_explorer.go
# go run .

Or, if GUI code is co-located and guarded with `//go:build gui`:

```bash
go get fyne.io/fyne/v2@latest
go mod tidy
go run -tags gui .
```
```

In the GUI:
- Enter the directory path to scan
- Choose the hashing algorithm (MD5/SHA1/SHA256/SHA512)
- Click "Scan" to begin
- Duplicates appear under "Duplicates:"; you can remove listed files (careful!)

Notes:
- The GUI and CLI produce the same dedup results because they share the core logic.
- If you build binaries for both, prefer the same Go version to avoid subtle differences.
 - The GUI requires its own module with Fyne dependencies in its project. Do not run GUI files from `File-Deduplicator`.

## Hash Algorithm Choice
- MD5/SHA1 are faster but weaker; use for quick local dedup checks
- SHA256/SHA512 are stronger; recommended when integrity matters

## Performance
- Hashing runs concurrently with a bounded worker pool (semaphore)
- Adjust concurrency in the code (`sem := make(chan struct{}, 128)`) to suit your hardware

## Output
- `duplicate_files.txt`: contains absolute paths of files considered duplicates (same hash)
- Console/GUI log: summary of duplicates and potential space savings

## Caveats & Safety
- Duplicate detection is hash-based; collisions are extremely unlikely with SHA512, but always review before deletion
- The GUI "Remove Files" action only logs deletions in this version; actual removal is commented or guarded where needed

## Building the GUI (optional)
From the GUI project you can build a GUI binary (requires Fyne dependencies on your OS):

```bash
# cd path/to/gui/project
# go build -o DeduplicatorGUI
# ./DeduplicatorGUI
```

On macOS, Fyne can also package an app bundle; see Fyne docs for `fyne package`.

## License
Internal project. Do not redistribute without permission.
