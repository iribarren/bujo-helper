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
- [ ] T2 app + UI + docs: binding exposes papers and per-page counts per paper × size, `ExportPDF` validates the paper before the save dialog, Paper select, live page count, README Print sheet section. Route: delegated writer (writer trigger: app.go, print.ts, index.html, README, bindings).

## Progress
- T1 `662ab2f` feat(sheet): `Papers` (A4 default, US Letter, A5), `PaperByID`, `Options.Paper`; `Options.Grid` uses the paper's page. Route: inline. RED: build failure (undefined A5/PaperByID, unknown field Paper). GREEN: `go test -tags webkit2_41 ./...` 262 passed (14 packages); vet and gofmt clean. Tests: grid per paper × size (full, centred), default = A4, unknown paper named, boxes inside margins on every paper, PDF MediaBox and page count per paper.

## Next step
T2 (delegated writer).
