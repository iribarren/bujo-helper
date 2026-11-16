# Feature: checklist (roadmap item 10)

## Objective
Add a **Checklist** print format (TBR list / watchlist): a header the user types (e.g. "TBR 2026"), then one row per item with a checkbox, a small thumbnail, the title and the creator. Rows paginate.

## Problem / Why
To-be-read lists and watchlists are standard bujo collections.

## Scope / constraints
- Extend, don't replace: new `Format` `checklist` (Label "Checklist", FileTag "checklist", Noun "row", `UsesSize: false`, `UsesCut: false`, `UsesHeader: true`) in `sheet.Formats`, after Full page. Reuse `Options.Header`, the full-page header band and header drawing (shrink then "…"), the file-name parser in `names.go`, and `fitLines` / text fonts.
- Row layout (fixed row height, constants exported for tests): within the usable width (page − 2 × `MinMargin`): checkbox square, thumbnail, then title (bold) on line 1 and creator (regular, grey) on line 2, each one line, truncated with "…" — never past the right margin. Empty creator → title only. Thin light-grey rule under each row (writing-line look). No cut borders.
- Thumbnail fits the row height (row height − vertical padding) in a 2:3 box, keeping aspect ratio with the existing shrink-to-fit rule (`Grid.place` or equivalent); non-2:3 images stay inside the box.
- Rows per page = floor((usable height − header band if header) / row height); header repeated on every page; rows start right below the header (top-aligned, not vertically centred, like a list page). `Options.PerPage()` returns that count for the selected paper (header present or not must be reflected consistently with full page's approach — document the choice).
- Paper applies; size preset and cut mode don't. Defaults keep today's output: covers, cards, spines, stubs and full page byte-identical.
- PDF default name: `checklist_<paper>.pdf`.
- UI: the format list and Header field come from bindings (`PrintFormats`, `usesHeader`), so the Print tab should need little or no change; regenerate Wails models only if the Go structs change. README Print sheet: new "Checklist" subsection + feature list lines.
- Delivery: branch `feat/checklist` from `2629b90`; forecast ~500 authored lines (≈ 50 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- PDF export for books and movies, with long titles truncated.
- Layout tests for rows per page (per paper, header on/off), row geometry inside margins, thumbnail fitting, text truncation, pagination with the header repeated.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41` (revert 0755 modes it sets on `frontend/wailsjs/go/main/*` and `models.ts`).

## Tasks
- [x] T1 sheet: `checklist` format, row geometry, `buildChecklist`, pagination, table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`).
- [x] T2 app + UI + docs: app tests (formats list, export name), any Print tab tweak, README. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/checklist` from `2629b90`. RDD on (global).

- T1 done in `132cf23` feat(sheet): add checklist print format. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`). 7 files, +677 / −38 (≈ 715 authored lines, ≈ 58 % tests: `checklist_test.go` +388, `format_test.go` +9, `app_test.go` +19/−15). Over the ~400-line heuristic because row geometry, text fitting and PDF pagination each need table-driven coverage; no artificial split.
  - Files: `internal/sheet/checklist.go` (constants, `NewChecklistGrid`, `checklistRow`, `layoutChecklistRow`, `fitChecklistText`, `buildChecklist`, `drawChecklistRow`), `layout.go` (`checkFormat`, Formats entry, `Grid.Flush`, `gap()` 0 when flush, `Options.Grid` case, docs), `fullpage.go` (header drawing extracted into `pdfMeasure`, `fitHeader`, `drawHeader`, shared with the checklist; full-page output unchanged), `pdf.go` (build switch, `Build` doc), `format_test.go` (checklist uses-header row, per-page counts), `app_test.go` (`TestPrintFormats` / `TestPrintPapers` gain the checklist entry to keep the commit green).
  - RED: `go test -tags webkit2_41 ./internal/sheet/` → build failed: `undefined: NewChecklistGrid`, `undefined: ChecklistRowHeight`, `undefined: layoutChecklistRow` …
  - GREEN: `go test -tags webkit2_41 ./...` → 538 passed in 14 packages; `go vet` clean; `gofmt -l .` empty.
  - Defaults unchanged: page streams of covers, cards, spines, stubs and full page for every format × paper × size × cut × header ("" / "Books of 2026") — 768 option combinations, 33 mixed covers, uncompressed, fixed dates, via a `go test -overlay` scratch file outside the repo — are byte-identical before and after.
  - Design notes: `ChecklistRowHeight` 16 mm (rule included), `ChecklistBoxSize` 5 mm (0.3 mm #555 outline), 1.5 mm padding → 13 mm tall 2:3 thumbnail box, 3 mm gaps; title bold 10 pt `titleGrey`, creator 8.5 pt `textGrey`, each one line cut with "…" via `fitLines`; empty creator → title centred alone; 0.2 mm `writingGrey` rule along each row's bottom edge; no cut borders. Rows start at the top margin, or right below the 12 mm `FullPageHeaderHeight` band (same band box and drawing as full page). Rows per page = floor((page height − 2 × `MinMargin` − band) / 16): A4 17 / 16 with header, Letter 16 / 15, A5 12 / 11. `Options.Grid()` returns the real row grid for the header presence in `Options.Header` (unlike full page, whose count does not depend on the header), so `Options.PerPage()` is exact; `Grid.Flush` makes rows touch so `Grid.Cells` paginates them. Image centred in its thumbnail box (shrink-to-fit rule as in `Grid.place`). Fonts always embedded (titles).
  - Visual check (`pdftoppm -r 100`): A4 20 mixed covers + "TBR 2026" → 16 rows then 4, header repeated; A5 long accented header shrunk, long titles cut with "…" inside the right margin; square, wide and narrow images inside their thumbnail boxes.

- T2 done in `5e9c515` feat(ui): offer checklist format in the print sheet. Route: delegated writer. 6 files, +108 / −29.
  - Files: `app.go` (`PrintPaperInfo.HeaderPerPage` `json:"headerPerPage"`: per-page count with a header, by format ID, for formats that print one; `pdfFileName` doc), `app_test.go` (export names `checklist_A4.pdf`, `checklist_Letter.pdf` with a header, `checklist_A5.pdf` with size/cut ignored; `TestPrintPapers` header counts; `TestPrintPapersMatchTheSheet` checks them against `Options{Header}.PerPage()`), `frontend/src/print.ts` (page count uses `headerPerPage` when the format prints a header and one is typed; header `input` refreshes the count; placeholder "TBR 2026" for the checklist, "Books of 2026" otherwise; file comment), `frontend/index.html` (Format tooltip, Header tooltip generic), `frontend/wailsjs/go/models.ts` (regenerated: `headerPerPage`), README (feature line, Print sheet intro, Format step, Cover size note, page-count step, file names, Cut mode note, new "Checklist" subsection, `internal/sheet` line).
  - Why `HeaderPerPage`: checklist rows per page depend on the header (A4 17 / 16), so the toolbar page count would be one page off without it; full page reports 60 both ways.
  - RED: `go test -tags webkit2_41 .` → build failed: `unknown field HeaderPerPage in struct literal of type PrintPaperInfo`, `p.HeaderPerPage undefined`. Export-name cases are characterisation (`pdfFileName` is already generic).
  - GREEN: `go test -tags webkit2_41 ./...` → 541 passed in 14 packages; `go vet` clean; `gofmt -l .` empty; `npx tsc --noEmit` no errors; `wails build -tags webkit2_41` built. It set 0755 on `frontend/wailsjs/go/main/App.{d.ts,js}` and `models.ts`; reverted to 0644, `git diff --summary main..HEAD` shows no mode changes.
  - Pending: manual GUI check (Checklist in the Format list, Header placeholder per format, page count changing as a header is typed, export with and without a header).

## Next step
Parent: native review / manual GUI check.
