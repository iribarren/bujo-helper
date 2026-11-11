# Cover Downloader

A desktop app for Windows and Linux that downloads cover images in batches.
Paste a list of `Title, Author` lines, or import a CSV file, and it saves one cover image per item to a folder.

Books are supported today. The core is media-agnostic, so movies, music or games can be added later as new providers, with no UI changes.

## Features

- Batch input: one `Title, Author` per line, or import a CSV file (the import replaces the list).
- Live progress: each item shows pending, saved (with file name and source), failed (with the reason) or cancelled.
- Cancel at any time. Items that have not finished are reported as cancelled.
- Fallback across sources: if one source has no match or no usable image, the next one is tried.
- Placeholder images, error pages and non-images are rejected, so you never get a blank "no cover" file.
- File names are safe on Windows and Linux (`Title - Author.jpg`). Duplicates get `-2`, `-3`, … and existing files are never overwritten.
- Store country selection (for example `us`, `gb`, `es`, `de`), which helps with localized titles.
- **Open folder** shows the results in the system file manager.
- Light and dark themes follow the system setting.
- Keyboard shortcut: **Ctrl+Enter** starts the download.
- A `covers` command-line tool that uses the same core.

## Sources

Sources are tried in this order for books:

1. **Apple Books** (iTunes Search API): large, high-quality covers. It uses the store country.
2. **Open Library**: a free, open catalog. Covers are often smaller.
3. **Amazon** (store search page): broad coverage, but Amazon may answer with a bot check. When that happens the item falls back to the other sources or fails with `blocked by bot check`.

## CSV format

- One item per row: `title,author`. The author column is optional and extra columns are ignored.
- An optional header row (for example `title,author`) is detected and skipped.
- Comma (`,`) or semicolon (`;`) delimiters are detected automatically.
- UTF-8 (with or without BOM) and Windows-1252 (the Excel default on Windows) are both accepted.
- Quote titles that contain the delimiter: `"Guns, Germs, and Steel",Jared Diamond`.

See [`sample_books.csv`](sample_books.csv) for an example.

In the text box, a line is split on its **last** comma, so `Guns, Germs, and Steel, Jared Diamond` works without quotes. Blank lines and lines starting with `#` are ignored.

## Using the app

1. Choose the media type (only **Books** for now).
2. Type or paste items, one per line, or click **Import CSV…**.
3. Check the output folder (default: `~/Pictures/Covers`, or `%USERPROFILE%\Pictures\Covers` on Windows), and change it with **Browse…** if needed.
4. Optionally change the two-letter store country (default `us`).
5. Click **Download covers** or press **Ctrl+Enter**.
6. When items finish, click **Open folder** to see the images.

## Using the command line

The `covers` CLI runs the same downloader without a window:

```sh
go run ./cmd/covers -o covers "Dune, Frank Herbert" "Emma, Jane Austen"
go run ./cmd/covers -country es -csv sample_books.csv -o covers
```

Flags must come before the items:

| Flag       | Default  | Meaning                                         |
|------------|----------|-------------------------------------------------|
| `-type`    | `books`  | Media type                                      |
| `-o`       | `covers` | Output folder                                   |
| `-country` | `us`     | Store country code                              |
| `-csv`     | (none)   | CSV file, added after the positional items       |

The exit code is `0` when every item succeeded, `1` when any item failed or was cancelled, and `2` for usage errors.

## Prerequisites

### Linux (Ubuntu / Debian)

```sh
sudo apt install golang-go nodejs npm pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

Recent Ubuntu releases only ship WebKitGTK 4.1, so every `wails` command on Linux needs the `webkit2_41` build tag (`-tags webkit2_41`). `wails doctor` may still report WebKit as missing because it looks for 4.0, and that can be ignored.

### Windows 10/11

- [Go](https://go.dev/dl/) and [Node.js](https://nodejs.org/) (LTS).
- The Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.
- WebView2 is built into Windows 10/11, so users need nothing else to run the `.exe`.

## Development

```sh
wails dev -tags webkit2_41        # Linux; on Windows just: wails dev
```

This runs the app with hot reload of the frontend. Wails regenerates the TypeScript bindings in `frontend/wailsjs/` when Go method signatures change (or run `wails generate module -tags webkit2_41`).

## Building

```sh
# Linux binary: build/bin/cover-downloader
wails build -tags webkit2_41

# Windows binary: build/bin/cover-downloader.exe
# Works from Windows or cross-compiled from Linux (no cgo needed).
wails build -platform windows/amd64
```

Each build is a single executable with the frontend embedded.

## Tests

```sh
go test ./internal/... ./cmd/... .
```

The root package embeds `frontend/dist`, so build the frontend once first (`cd frontend && npm install && npm run build`, or any `wails build`). On Linux, add `-tags webkit2_41` when testing the root package or running `go vet ./...`.

## Project layout

```
main.go, app.go        Wails entry point and the methods bound to the UI
frontend/              Vanilla TypeScript + Vite UI
  src/                 main.ts (UI logic) and style.css
  wailsjs/             generated bindings (committed)
cmd/covers/            command-line tool
internal/
  media/               domain types: Item, Candidate, Provider, MediaType, Registry
  catalog/             built-in media types and their providers
  providers/           Apple Books, Open Library, Amazon
  match/               title/creator matching and ranking
  batch/               text lines and CSV parsing
  downloader/          concurrent batch downloads, image validation, file naming
  web/                 shared HTTP client, headers and bounded reads
build/                 Wails build assets (icons, manifests); binaries go to build/bin/
legacy/                the original Python script
```

## Adding a new media type

1. Implement `media.Provider` (`Name()` and `Search(ctx, item, opts)`) for each source, usually in `internal/providers`. Return candidates best-first with their image URLs ordered best-first. Return an error wrapping `media.ErrNotFound` when there is no match.
2. Register a `media.MediaType` in `internal/catalog` with an ID, a label, a creator label (for example `Director` or `Artist`) and its providers in fallback order.

That's all. The UI lists media types from the registry and uses the creator label in its hints, so no frontend changes are needed.

## Legacy

`legacy/cover_downloader.py` is the original single-title Python script that this app replaces. It is kept for reference only and is not part of the build.
