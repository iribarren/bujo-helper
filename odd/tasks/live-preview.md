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
- [x] T2 Make pages live preview: `pdfjs-dist` dependency, `preview.ts` (load, render page to canvas, prev/next, debounce, stale drop), wired into `pages.ts`, i18n, tests with pdf.js mocked. Route: delegated writer.
- [x] T3 Print covers Gallery | Preview switch: segmented switch, preview of selected covers through `preview.ts`, i18n, tests. Route: delegated writer.
- [x] T4 Verify + docs: README (preview, pdf.js licence), page-stream hashes vs `main`, full checks, `wails build`, ROADMAP notes. Route: inline state checks + delegated build.
- [x] T5 Loading indicator (user GUI check 2026-11-20: preview works but slow loads give no sign): visible spinner + "Loading preview…" over the preview well from the moment a render is scheduled until the page is drawn or a message shows, in both tabs; reduced-motion safe; screen-reader status; also fixes final-review R3-001 (busy cleared early when paging during the first draw). Route: delegated writer (preview.ts, style.css, i18n, tests).

## Progress
- Feature doc created; branch `feat/live-preview` from `dbdba78`.
- T1 (`9657cae`): `internal/preview.Store` (last 4 renders, `/preview?id=N`, `application/pdf`, `no-store`, 404/405 like `/thumbs`) and `PageCount` (page tree root of fpdf output); `App.assetHandler()` mux serves `/thumbs` + `/preview` (main.go); bindings `PreviewPDF(paths, opts)` / `PreviewPage(kind, page, options)` → `PreviewInfo{url, pages}`, rendering through `buildCovers` / `buildPage`, which `exportTo` / `ExportPage` now use too; wailsjs regenerated (644). RED: preview package and bindings failed to build before the code. Tests compare preview vs export bytes (dot grid ×2, cover sheet ×2, Spanish stubs) and page counts vs `PrintPages`. Deviation: besides the dates, the tests pin `fpdf.SetDefaultCatalogSort(true)`, since stubs (text fonts) varied byte-wise even with pinned dates; with it whole-file bytes match, no page-stream fallback needed. Checks: go test/vet, tsc, vitest pass.
- T2 (`e72af35`): `pdfjs-dist` 6.5.438 (exact); `preview.ts` controller (300 ms debounce, numbered requests so only the newest is drawn, stale loads destroyed, old documents destroyed, each page drawn on a fresh canvas swapped in when done, devicePixelRatio-aware fit, redraw on resize, prev/next + "Page X of N", error/message text, `aria-busy` dims the old page); fetches the PDF bytes itself and opens them with `getDocument({data})` (pdf.js streams only http(s) URLs; Linux uses `wails://`). Make pages renders through `PreviewPage` on every kind/form/Page/Lettering change; the export status's page count now comes from the preview (PagePages no longer called by the UI). i18n `preview.*` EN/ES (`pages.pageOf` replaced). Tests mock pdf.js (`previewMock.ts`). RED: `preview.test.ts` (no module) and 14 `pages.test.ts` failures before wiring. Choices: page kept when options change, clamped to the new count (not reset to 1); **legacy** pdf.js build, because the modern one calls `Map.prototype.getOrInsertComputed`/`Math.sumPrecise` unguarded (only the newest engines; WKWebView follows the macOS version). `vite build` bundles the worker (1.3 MB asset) and warns about the 570 kB main chunk. Checks: tsc, vitest (164), go test/vet pass.
- T3 (`65f18dd`): **Gallery | Preview** segmented switch (`#print-view`, radioGroup: one tab stop, arrow keys) at the start of the Print covers toolbar; the pager (‹ Page X of N ›) at its end, shown in Preview only; the preview well replaces the gallery. Preview renders the selected covers through `PreviewPDF` with `printOptions` (the export's options) on every selection/option change, including cut mode and Cards for (which do not change the page count), a Refresh, and a language change (Go prints stub/card/credits labels in it); nothing renders while Gallery is shown (`cancel()`), no selection shows "Select covers to preview them." EN/ES `print.view*`. Toolbars now wrap on narrow windows. RED: 8 new `print.test.ts` tests failed before the code. Checks: tsc, vitest (172), go test/vet pass.
- Review `dbdba78..65f18dd` (medium): consent granted, reliability lens approved, acknowledged. Advisory findings fixed in `9ece906` (route: delegated writer): R3-001 a page that fails to draw now shows `preview.error` (cleared by the next page drawn); R3-002 the language change already re-renders Print covers (relabel → updateSelection → updatePreview, key includes the language), now tested, and Make pages needs nothing (Go does not localise page kinds' PDFs); R3-003 the old canvas stays, dimmed, until the new document's page is drawn. RED: 3 `preview.test.ts` tests failed before the fix; the R3-002 test passed on the code and fails with the language dropped from the key. Next reviewed boundary: `65f18dd`.
- T4 docs (`docs:` commit after `9ece906`): README (Print sheet Preview step, Make pages live preview, `/preview`, project layout, Third-party software table with pdf.js / Apache 2.0, bundled), ROADMAP item 30 notes (stays `[ ]` until the merge).

- T4 docs in `cd52b0a`; `package.json.md5` refreshed in `844face` (stale since `e72af35`, so every build reinstalled packages).
- T4 checks: page streams + sorted per-stream hashes of 504 combinations (7 formats × a4/letter/a5/b5 × 3 sizes × 3 cuts, 6 crop modes for Cover sheet, dot grid ×4/5/6 mm ×title ×cut; header, date label, credits; 4 mixed-shape fixtures) at `main` (`dbdba78`) vs `65f18dd`: identical (`cmp`). Whole-file hashes still vary between processes for cropped Cover sheet / Checklist even with catalog sort, because fpdf v0.9.0 sorts images only by width (equal widths keep map order): object numbers only. tsc clean; vitest 176/176; `go test -tags webkit2_41 ./...` ok; `wails build -tags webkit2_41` built `build/bin/bujo-helper` (14.4 MB), binding modes 644.
- Review `65f18dd..844face` (medium; final slice, run before delivery): consent granted; reliability lens approved; acknowledged (authority burned). Advisory R3-001 SUGGESTION (`preview.ts:233`): pressing next/previous while a new document's first page is still drawing clears `aria-busy` early, so the old canvas briefly shows undimmed under the new label; left as a known minor follow-up. Every code commit on the branch is now reviewed.
- T5 (`5ccd026`): `preview.ts` adds a `.preview-loading` indicator next to the sheet in its well (one place for both tabs): decorative spinner (`aria-hidden`) + status text `preview.loading` ("Loading preview…" / "Cargando vista previa…") in a `role="status"` `aria-live="polite"` region that stays in the DOM with empty text at rest (only its arrival is announced; same text is never rewritten). Busy = a request pending/loading **or** the page to show still drawing: `aria-busy` and the indicator now share one state, shown from `schedule` (debounce included) until the page is drawn, an error/message replaces it, or `cancel()` (Print covers → Gallery); a burst of changes keeps it up. Fixes R3-001: busy is cleared by the end of the newest draw, not by request staleness, so paging during a new document's first draw keeps the old page dimmed under the indicator. Resize redraws are quiet (no indicator). CSS: centred pill on `--surface`, spinner `--border`/`--accent`, fades in after 150 ms (CSS `transition-delay`, so quick page draws don't flash; a render always shows it within ~0.15 s of a change, since the 300 ms debounce counts) and hides at once; `prefers-reduced-motion: reduce` stops the spin. `previewMock` cancel now rejects with `RenderingCancelledException`, like pdf.js. RED: 10 new `preview.test.ts` tests failed (incl. R3-001: `aria-busy` `false` while page 2 drew, with the indicator checks removed) and the 2 new `pages.test.ts`/`print.test.ts` tests failed against the old controller; GREEN: tsc clean, vitest 188/188; `go test -tags webkit2_41 ./...` ok; `wails build -tags webkit2_41` built `build/bin/bujo-helper` (14.4 MB), binding modes reset to 644, no content changes.

- Review `2530522..7668a57` (T5, medium; final slice before delivery): consent granted; reliability lens approved; acknowledged (authority burned). Advisory SUGGESTION R3-quiet-redraw-untested (`preview.ts:357`): no test drives the ResizeObserver quiet redraw (indicator stays hidden; a quiet redraw replacing a loud one clears busy); code reads right, left as a minor follow-up. Every code commit on the branch is reviewed.

## Next step
User GUI check of the preview in the real app (WebKitGTK: `fetch('/preview?id=N')` over `wails://` and the pdf.js worker are untested outside it), then local `--no-ff` merge to `main` and ROADMAP item 30 `[x]`. Open: re-rendering a large cover selection re-encodes every image (no preview cache); `vite build` warns about the 570 kB main chunk.
