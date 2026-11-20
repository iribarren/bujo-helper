# Feature: make-pages (roadmap item 29, remaining part)

## Objective
Finish ROADMAP item 29: a third **Make pages** tab for generated (no-image) printables, with a no-image backend and a first kind, **Dot grid**; **B5** paper; the Download tab's "Sources tried" line and **Print these covers** link; shared **Page** and **Lettering** fieldsets rendered by both tabs; the muted Lettering hint in Print covers.

## Problem / Why
Generated printables (items 16, 18, 27, 28) need typed options and no folder. The image pipeline (`sheet.build`, `ExportPDF`, `PrintPages`) fails without image paths. The layout restyle (`odd/tasks/ui-restyle.md`, merged 5e7d2d3) already did tabs, the Download layout and the Print covers fieldsets.

## Scope / constraints
- Design canvas https://claude.ai/artifact/6gdFFavUM9kyA76MsCpB1R, artboards **Make pages** (`project/Main.dc.html`) and **Download** (`project/Download.dc.html`); local copies in the session scratchpad. Canvas wins for layout, ROADMAP for behaviour/scope. Rebuild with `style.css` tokens/components; don't copy canvas markup.
- Make pages: three columns: page kinds list (~196 px; kinds from Go, unbuilt kinds not listed), options column (~320 px: kind form, then shared Page and Lettering fieldsets), preview (page-shaped placeholder in the paper's aspect ratio on a grey well, "Page 1 of N") with status + Open PDF + Export PDF below.
- Dot grid kind: dot spacing segmented **4 / 5 / 6 mm** (default 5, canvas), optional title (printed with the existing header helpers, Go font until item 31). Dot positions centred within margins.
- Option controls: 2–4 values → segmented group (`radioGroup.ts`), longer → select. Paper has 4 values with B5 → segmented.
- Backend: page kinds registry; each kind declares a typed options struct decoded from JSON (unknown fields rejected), so new kinds don't touch the bindings. Bindings `PageKinds()`, `PagePages(...)`, `ExportPage(...)` reuse `newPDF`, papers, cut modes, text helpers, save dialog, atomic write, `OpenFile`.
- Sources tried: `MediaTypeInfo` gains the ordered provider display names (Go-side labels for provider ids; test that every catalog provider has one).
- Print covers PDF output byte-identical to `main` for the same options.
- EN + ES for every new label; dark theme; ≤700 px columns stack.
- Out of scope: live preview (item 30), Font / Colour-in (item 31), other page kinds.

## Acceptance criteria
- Tabs keyboard-navigable; Download still works; media type buttons reachable by keyboard.
- Print covers page streams identical to `main` (hash check, as in ui-restyle T4).
- Make pages exports a dot grid PDF on every paper. Go tests: dot positions + margins per paper, options decoding, kind registry, bindings. Frontend tests: tab switching (3 tabs), fieldset visibility, Make pages form → bindings.
- `go test -tags webkit2_41 ./...`, `tsc --noEmit`, `vitest run`, `wails build -tags webkit2_41` pass.

## Checks
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `go test -tags webkit2_41 ./...`
- Page-stream hashes: scratch `go test -overlay` test, base tree from `git archive main`.
- `wails build -tags webkit2_41`; reset 0755 modes on regenerated bindings.

## Delivery
- Branch `feat/make-pages` from `24b0691`. Forecast ~1300 authored lines (≈ 40 % tests). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `24b0691`.

## Tasks
- [x] T1 B5 paper: Go constant + `Papers`, `TestPrintPapers`, i18n `paper.b5`, README. Route: delegated writer A (writer trigger: Go + TS + docs).
- [x] T2 Sources tried line + Print these covers link: `MediaTypeInfo` provider names, Download line under the media types, link in the Download footer switching to Print covers, tests. Route: writer A.
- [x] T3 No-image backend: page kinds registry + Dot grid builder in `internal/sheet`, bindings, wailsjs regenerated, Go tests. Route: writer A.
- [x] T4 Make pages tab: markup, `pages.ts`, shared Page/Lettering fieldsets used by both tabs, segmented controls CSS, Lettering hint in Print covers, i18n, tests. Route: delegated writer B (writer trigger: index.html, TS modules, CSS, i18n, tests).
- [x] T5 Verify: page-stream hashes vs `main`, full checks, `wails build`, README Print sheet / Make pages docs, ROADMAP item 29 marked. Route: inline state checks + delegated hash check.

## Progress
- Feature doc created; branch `feat/make-pages` from `24b0691`. Mapping by a read-only explorer; canvas read by the parent.

- T1 done in `ca1b8bf`: B5 176 × 250 mm after A5; `TestPrintPapers` B5 counts; README per-paper tables. RED: `TestPaperByID` (b5), `TestPrintPapers`.
- T2 done in `d45c5b6`: optional `media.Labeler` (`Label()`), `media.ProviderLabel` falls back to `Name()`; `MediaTypeInfo.Sources`; `#sources` muted line under the media types; `#print-these` (`.accent-outline`) after Open folder, shown with it, selects the Print covers tab. RED: `TestMediaTypes`, `TestMediaTypesListSourcesInOrder`, `TestCatalogProvidersHaveDisplayNames`, 6 vitest tests.
- T3 done in `168e2cd`: `internal/sheet/pages.go` (`PageSettings{Paper,Cut,Title}`, `PageKinds`, `PageKindByID`, `PageCount`, `BuildPage`; options decoded over defaults, unknown fields / wrong types / trailing data rejected), `dotgrid.go` (spacing 4/5/6 mm, default 5; 0.25 mm grey dots; 8 mm `MinMargin`, centred; 12 mm title band via `fitHeader`/`drawHeader`). Bindings `PageKinds()` (defaults as JSON string), `PagePages(kind, page, options string)`, `ExportPage(kind, defaultDir, page, options string)`; default file name `dotgrid_A5.pdf` (`pdfFileName` style). RED: sheet tests, `TestPageKinds`, `TestPagePages`, `TestExportPage*`.
- Writer A checks: `go test -tags webkit2_41 ./...` all ok; `tsc --noEmit` clean; vitest 112/112; `go vet` clean. Parent spot check: `go test ./internal/sheet/ .` 900 passed.
- Open from T2: the link only switches tabs; a folder chosen earlier in Print covers wins. T4 resets Print covers to the download folder on the link.

- Review `24b0691..cb08ad6` (medium, 34 files, 1248 lines, `slice_budget_reached`): consent granted; reliability lens approved; acknowledged (authority burned). Advisory R3-print-these-folder WARNING (`main.ts:631`: the link doesn't point Print covers at the finished job folder; test checks only the tab) → folded into T4. Next reviewed boundary: `cb08ad6`.

- Passive check `cb08ad6..36135d7` (docs only): `passive`, no review; boundary advances to `36135d7`.
- T4 done in `b1353f8` (14 files, +1360/−172, ~600 test lines): third tab `pages` (Make pages / Crear páginas); `pages.ts` + kind form registry (`pageForms.ts`; a kind without a form exports its defaults); `fieldsets.ts` renders the shared Page and Lettering fieldsets in both tabs (Print covers keeps every id; Page/Lettering no longer static markup, created in `initPrint`); Paper is a segmented group in both tabs; Lettering hint "This format prints no header." instead of hiding; Print these covers points Print covers at `jobFolder` (review finding R3-print-these-folder); Ctrl+Enter only starts downloads on Download; `#key-tip` hidden on Make pages; segmented CSS, three-column grid, preview well (+2 tokens light/dark); EN/ES; README dot grid how-to. RED: 4 tab tests + Print these folder test (`main.test.ts`), `pages.test.ts` (24), 9 Lettering/legend + 2 paper tests (`print.test.ts`). Deviations: selected segment = accent-soft fill (canvas: solid accent); Title in Lettering fieldset (canvas: dot grid form); status line empty until export; options not persisted (Print covers doesn't persist either).
- Writer B checks: `tsc --noEmit` clean; vitest 135/135; `go test -tags webkit2_41 ./...` 1458 passed; `git diff 36135d7 --stat -- '*.go' frontend/wailsjs` empty; headless Chrome 1200×820 + 680 px, light/dark, EN/ES. Parent spot check: vitest 135/135; screenshot of Make pages matches the canvas layout.

- Review `36135d7..b1353f8` (medium, 14 files, 1532 lines, `slice_budget_reached`): consent granted; reliability lens approved; acknowledged (authority burned). Advisory findings R3-001 (dot grid default outside 4/5/6), R3-002 (stale page count after a `PagePages` failure), R3-003 (Print these covers test isolation), R3-004 (Make pages `defaultDir` untested) → fixed as follow-up. Passive `b1353f8..d5f62f1` (docs). Next reviewed boundary: `d5f62f1`.
- Follow-up done in `71fb1a2`: dot grid falls back to 5 mm when the Go default isn't 4/5/6 (the radio group otherwise checks its first option, 4 mm); count cleared when `PagePages` fails; `resetFolders` restores state/mocks in `main.test.ts`; two `defaultDir` tests (Print covers folder / output folder; `main.ts` already right). RED: 4 `dot grid form > checks and sends 5 mm for …`, `page count > shows no count for settings PagePages could not count`; R3-004 proved by a temporary mutation. Checks: `tsc` clean; vitest 143/143; shuffled seeds 1–5 50/50. Assessment `d5f62f1..71fb1a2`: medium, `under_budget` (153 lines) → pending in the slice.
- T5 done: page-stream SHA-256 of 324 Print covers combos (7 formats × a4/letter/a5 × 3 sizes × 3 cuts + 6 crop modes for Cover sheet; header, date label, credits; 4 mixed-shape fixtures) at `main` (`git archive`) vs `d5f62f1`: identical. (Whole-file hashes of Checklist/Polaroid vary between runs of the same tree; page streams are the valid check.) `wails build -tags webkit2_41` built `build/bin/bujo-helper` (12.5 MB); binding modes reset to 644. ROADMAP item 29 notes what this branch did; `[x]` + merge hash at merge.

- Review `d5f62f1..bce0e1b` (medium, 6 files, 168 lines; final slice, run before delivery): consent granted; reliability lens approved with no findings; acknowledged (authority burned). Every code commit on the branch is now reviewed.

- GUI check passed (user, 2026-11-20); merged `feat/make-pages` into `main` in `e7f8b6f` (`--no-ff`); ROADMAP item 29 marked `[x]`.

## Next step
Done. Not pushed.
