# Review follow-ups 1 (emoji titles, PDF determinism, backend hardening)

## Objective
Close three open review follow-ups chosen by the user from the ranked follow-up list (items 1, 3, 4).

## Problem / why
- Titles from file names (and other drawn text) can contain runes above U+FFFF (emoji). fpdf's UTF-8 fonts index widths by 16-bit rune and panic. Only the credits page sanitizes (`printable()`, `internal/sheet/credits.go:71`). Sources: places.md:67/76, nature.md:101.
- The same checklist of repeated landscape images renders different bytes twice in one process. Likely cause: `newPDF` (`internal/sheet/pdf.go:118`) never calls `SetCatalogSort(true)`; only tests do. Source: artworks.md:67/74.
- Backend hardening from media-cover-downloader.md review suggestions: `OpenFile` opens any regular file (`app.go` OpenFile); thumbnail decodes have no pixel cap (`internal/gallery/thumbs.go`, only concurrency capped); `Start` creates the output folder before checking whether a job is already running (`app.go` Start).

## Scope
- In: sanitize every fpdf-drawn text that can carry non-BMP runes (single chokepoint preferred); deterministic PDF output; OpenFile limited to `.pdf` (its only callers open exported PDFs: `frontend/src/print.ts:775`, `frontend/src/pages.ts:220`); thumbnail pixel cap via `image.DecodeConfig` before the full decode; running check before `MkdirAll` in `Start`.
- Out: Print option persistence (item 2), frontend test gaps, anything else in the follow-up list.

## Constraints / decisions
- Non-BMP runes print as `?` (same rule as the credits page); file names on disk are untouched.
- Thumbnail cap: 50 megapixels; larger images get no thumbnail (handler error, like an unreadable file), the print export still uses them.
- OpenFile: `.pdf` only (case-insensitive); anything else returns an error without calling the opener.

## Tasks
- [ ] T1 Sanitize drawn text for non-BMP runes in all formats and page kinds. Route: delegated writer (sheet package, several formats). Check: RED test exporting each format/page kind with an emoji title panics or fails; GREEN after.
- [ ] T2 Deterministic PDFs: `SetCatalogSort(true)` in `newPDF`; test that the same checklist with repeated landscape images renders identical bytes twice (fixed creation date). Route: delegated writer (same writer, separate commit).
- [ ] T3 Backend hardening: OpenFile `.pdf` only; thumbnail 50 MP cap; Start running check before MkdirAll (no folder created when a job runs). Route: delegated writer (app.go + gallery).

Route evidence: writer trigger (2+ non-trivial files across sheet, app.go, gallery). One writer, one commit per task.

## Acceptance
- No export panics on an emoji in a title, creator, typed page title or quote text.
- Same inputs give byte-identical PDFs (fixed creation date).
- OpenFile refuses non-PDF files; oversized images get no thumbnail; Start with a running job creates no folder.
- `go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l` clean.

## Delivery
Strategy: `ask-on-risk`. Forecast ~300 authored lines (mostly tests): single PR/merge.

## Progress
- 2026-11-23: branch `fix/review-followups` created; doc written.

## Next step
T1–T3 via one delegated writer.
