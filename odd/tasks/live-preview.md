# Feature: live-preview (roadmap item 30)

## Objective
A live PDF preview: Go renders the current options to PDF bytes in memory and serves them through the app's asset handler; the frontend draws them with pdf.js onto a `<canvas>`, page by page. Make pages shows it by default (replacing the item 29 placeholder); Print covers gets a **Gallery | Preview** switch.

## Problem / Why
Generated pages have no thumbnails, so the user exports blind. Spines, stubs and polaroids are hard to imagine from the gallery.

## Research
- **Native PDF in an `<iframe>`:**
  - WebKitGTK: the installed 2.52.6 has a built-in PDF.js viewer behind the `PDFJSViewerEnabled` setting (strings in `libwebkit2gtk-4.1.so.0`), but Wails doesn't expose that setting, older distro WebKitGTK builds lack it, and it shows a full viewer UI. Not usable.
  - WebView2 uses the Edge PDF viewer (works, but policies can disable it); WKWebView renders PDFs natively. Both show their own viewer chrome.
  - Decision: bundle **pdfjs-dist** (Apache 2.0) and draw to `<canvas>` on every platform: one code path, our own page controls.
- **Bytes:** fpdf stamps `CreationDate` with the current time, so two renders differ in that field only. Preview and export share one render function; tests pin the date with `fpdf.SetDefaultCreationDate`/`SetDefaultModificationDate` and compare bytes (page streams for Checklist/Polaroid, whose whole-file bytes already vary between runs).

## Scope / constraints
- Backend: render to memory (no temp file). Bindings return a preview URL and page count; the asset handler serves the PDF at that URL. Only renders made by the bindings are served (like `/thumbs`, nothing arbitrary); keep only the last few renders in memory.
- Preview uses exactly the export path (`sheet.Build` with `withLabels`, `sheet.BuildPage`).
- Frontend: pdf.js bundled by Vite (worker included, no CDN); page 1 first, previous/next for more; debounce ~300 ms after the last option change; drop stale renders (a newer request wins).
- Make pages: live preview by default, in the existing preview well. Print covers: **Gallery | Preview** segmented switch on the right-hand pane; preview of the selected covers.
- EN + ES for every new label; dark theme; keyboard reachable controls.
- README records the pdf.js licence.
- Out of scope: fonts / hollow letters (item 31), new page kinds.

## Acceptance criteria
- Preview bytes equal export bytes for the same options (pinned dates; Go tests).
- Changing an option updates a one-page preview within about a second.
- Stale responses never overwrite a newer preview (frontend test).
- Print covers output unchanged (page-stream hashes vs `main`).
- `go test -tags webkit2_41 ./...`, `tsc --noEmit`, `vitest run`, `wails build -tags webkit2_41` pass.

## Checks
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `go test -tags webkit2_41 ./...`
- `wails build -tags webkit2_41`; reset 0755 modes on regenerated bindings; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/live-preview` from `dbdba78`. Forecast ~1000 authored lines (≈ 40 % tests). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `dbdba78`.

## Tasks
- [x] T1 Preview backend: in-memory preview store + `/preview` route next to `/thumbs` (one mux in `main.go`), bindings for Make pages and Print covers returning URL + pages, wailsjs regenerated, Go tests (bytes equal export, unknown id 404, store bounded). Route: delegated writer (writer trigger: Go package + app.go + main.go + bindings).
- [ ] T2 Make pages live preview: `pdfjs-dist` dependency, `preview.ts` (load, render page to canvas, prev/next, debounce, stale drop), wired into `pages.ts`, i18n, tests with pdf.js mocked. Route: delegated writer.
- [ ] T3 Print covers Gallery | Preview switch: segmented switch, preview of selected covers through `preview.ts`, i18n, tests. Route: delegated writer.
- [ ] T4 Verify + docs: README (preview, pdf.js licence), page-stream hashes vs `main`, full checks, `wails build`, ROADMAP notes. Route: inline state checks + delegated build.

## Progress
- Feature doc created; branch `feat/live-preview` from `dbdba78`.
- T1 (`d3bea91`): `internal/preview.Store` (last 4 renders, `/preview?id=N`, `application/pdf`, `no-store`, 404/405 like `/thumbs`) and `PageCount` (page tree root of fpdf output); `App.assetHandler()` mux serves `/thumbs` + `/preview` (main.go); bindings `PreviewPDF(paths, opts)` / `PreviewPage(kind, page, options)` → `PreviewInfo{url, pages}`, rendering through `buildCovers` / `buildPage`, which `exportTo` / `ExportPage` now use too; wailsjs regenerated (644). RED: preview package and bindings failed to build before the code. Tests compare preview vs export bytes (dot grid ×2, cover sheet ×2, Spanish stubs) and page counts vs `PrintPages`. Deviation: besides the dates, the tests pin `fpdf.SetDefaultCatalogSort(true)`, since stubs (text fonts) varied byte-wise even with pinned dates; with it whole-file bytes match, no page-stream fallback needed. Checks: go test/vet, tsc, vitest pass.

## Next step
T2.
