# Feature: paper-sizes (roadmap item 2)

## Objective
Let the print sheet use three paper sizes: **A4** (default), **US Letter** (215.9 × 279.4 mm) and **A5** (148 × 210 mm). The grid is recomputed per paper; margins, gap and cover height stay the same.

## Problem / Why
US users print on Letter, and many bullet journals are A5, so people want sheets that match the journal page.

## Scope / constraints
- Extend item 1, don't replace it: `NewGrid(page, height)` already takes the page. Add `Papers`, `PaperByID`, `DefaultPaper = "a4"` and `Options.Paper` (`""` means A4) next to the size presets.
- fpdf page size already follows `grid.Page`.
- Expected grids (gap 4, margin 8, cell ≥ 2:3):

  | Paper  | Mini        | Standard    | Large      |
  |--------|-------------|-------------|------------|
  | A4     | 9 × 9 = 81  | 5 × 5 = 25  | 3 × 3 = 9  |
  | Letter | 9 × 9 = 81  | 5 × 4 = 20  | 3 × 3 = 9  |
  | A5     | 6 × 6 = 36  | 3 × 3 = 9   | 2 × 2 = 4  |

- UI: a **Paper** select in the Print sheet tab; the per-page count (and page count in the toolbar) follows paper and size live.
- Delivery: branch `feat/paper-sizes`, forecast ~300 authored lines, strategy `ask-on-risk` (under budget, single PR). Push/PR/merge are the user's decision.

## Acceptance criteria
- A4 gives today's output (existing tests keep their numbers).
- Letter and A5 produce full, centred grids with no clipped covers; PDF MediaBox matches the paper.
- Layout tests per paper size. README explains how the grid is derived per paper.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41`.

## Tasks
- [x] T1 sheet: paper presets, `Options.Paper`, grid per paper; table-driven tests per paper × size, PDF MediaBox per paper. Route: inline (one package, layout already loaded; previous item used the same route).
- [x] T3 PDF file name names the paper and cover size (`covers_A4_Standard.pdf`, `covers_Letter_Mini.pdf`…), requested by the user after the GUI check. Route: inline (small, understood).
- [x] T2 app + UI + docs: binding exposes papers and per-page counts per paper × size, `ExportPDF` validates the paper before the save dialog, Paper select, live page count, README Print sheet section. Route: delegated writer (writer trigger: app.go, print.ts, index.html, README, bindings).

## Progress
- T1 `662ab2f` feat(sheet): `Papers` (A4 default, US Letter, A5), `PaperByID`, `Options.Paper`; `Options.Grid` uses the paper's page. Route: inline. RED: build failure (undefined A5/PaperByID, unknown field Paper). GREEN: `go test -tags webkit2_41 ./...` 262 passed (14 packages); vet and gofmt clean. Tests: grid per paper × size (full, centred), default = A4, unknown paper named, boxes inside margins on every paper, PDF MediaBox and page count per paper.
- Review of main..cf527df (T1 + doc, 255 lines, medium; started by the stop hook): consent granted, reliability lens approved with no findings, acknowledged (authority burned). Reviewed boundary is now cf527df.
- T2 `ff5a90a` feat(ui): `PrintPapers()` binding (`PrintPaperInfo` with `perPage` by size ID); `PerPage` moved off `PrintSizeInfo` so the paper is the single source of per-page counts; Paper select (before Cover size) whose change relabels the size options and the toolbar page count; `ExportPDF` gets `{size, paper}`; README grid table per paper and generic *W*/*H* formulas. Route: delegated writer (writer trigger). RED: `PrintPapers undefined`, `undefined: PrintPaperInfo`. Unknown-paper `ExportPDF` cases had no real RED: `ExportPDF` already validates through `opts.Grid()`, so those tests pin existing behaviour. GREEN (writer): `go test -tags webkit2_41 ./...` 269 passed; vet, gofmt clean; `tsc --noEmit` ok; `wails build -tags webkit2_41` ok; binding file modes restored to 644. Parent spot check: `go test` 269 passed.
- Assess cf527df..ff5a90a: medium, 257 lines, `review_due` false (`under_budget`).
- Whole-branch review main..32842ae (516 lines, medium; preflight before reporting): consent granted, reliability lens approved, acknowledged (authority burned). Advisory SUGGESTION R3-001 (not scheduled): no automated test for the print.ts paper/size logic; the repo has no frontend test runner, so the manual GUI check covers it.
- Manual GUI check passed (user, 2026-11-16).
- T3 `e79e5c3` feat(ui): `sheet.Paper.FileTag` (A4, Letter, A5); `ExportPDF` suggests `covers_<tag>.pdf`; README step updated. RED: `DefaultFilename = "covers.pdf"`. GREEN: `go test -tags webkit2_41 ./...` 275 passed; vet, gofmt clean. Assess 8bf6d56..e79e5c3: medium, 58 lines, `under_budget`.
- Whole-branch review main..c3fe6f7 (563 lines, medium): consent granted, reliability lens approved, acknowledged (authority burned). Same advisory SUGGESTION R3-001 (no frontend test runner); not scheduled.
- Review of main..8b16ae6 (docs-only delta, started by the stop hook): consent granted, approved, acknowledged.
- User reported the file name did not change: `build/bin/cover-downloader` was built 09:33, before e79e5c3 (10:03), so the app tried was stale. User also asked to add the cover size.
- T3a `72b2162` feat(ui): `sheet.Size.FileTag`; name is `covers_<paper>_<size>.pdf`. RED: `DefaultFilename = "covers_A5.pdf"`, want `covers_A5_Mini.pdf` (+ build failure on `FileTag`). GREEN: `go test -tags webkit2_41 ./...` 276 passed; vet, gofmt clean. Rebuilt `wails build -tags webkit2_41` (binary 10:19); no binding mode changes.
- Manual GUI check of the file name passed (user, rebuilt binary). Merged in `313c0cf`; ROADMAP item 2 marked done.

## Next step
Done: merged to main in 313c0cf and pushed.
