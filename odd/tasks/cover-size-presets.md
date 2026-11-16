# Feature: cover-size-presets (roadmap item 1)

## Objective
Let the print sheet use three cover sizes: **Mini** (25 mm tall), **Standard** (50 mm, default) and **Large** (80 mm). The grid is derived from the cover height and the page size.

## Problem / Why
50 mm covers are too big for "year in books" grids and too small for a featured cover. Small covers are the most requested way to fit many items on one spread.

## Scope / constraints
- Presets Mini 25 mm, Standard 50 mm (default), Large 80 mm. No free-form size input.
- Grid derivation (A4 today, any page later for roadmap item 2):
  - Gap 4 mm and minimum margin 8 mm stay fixed.
  - A cell is at least 2:3 (the shape of a typical cover): `minCell = height × 2/3`.
  - `columns = ⌊(pageW − 2·8 + 4) / (minCell + 4)⌋`; the cell width then stretches so the side margins are exactly 8 mm.
  - `rows = ⌊(pageH − 2·8 + 4) / (height + 4)⌋`; the top and bottom margins centre the grid.
  - Standard on A4 gives 5 × 5, 35.6 mm columns, 15.5 mm top margin: today's layout exactly.
  - Mini: 9 × 9 = 81 per page. Large: 3 × 3 = 9 per page.
- `sheet.Options{Size}` is passed to `sheet.Build`; an empty size means Standard. Extensible for later items (paper, format, grayscale).
- UI: a **Cover size** select in the Print sheet tab, filled from a `PrintSizes()` binding that returns the per-page count, so the page count updates live with no constant to keep in sync.
- Delivery: one branch `feat/cover-size-presets`, forecast ~300 authored lines, strategy `ask-on-risk` (under budget, single PR). Push/PR/merge are the user's decision.

## Acceptance criteria
- Standard gives today's layout exactly (existing tests keep their numbers).
- Mini fits more covers per page, Large fewer. Every cover keeps its aspect ratio and the shrink-to-fit rule.
- Layout tests for each preset. README explains how the grid is derived.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet`, `gofmt -l`.
- Frontend: `tsc --noEmit` (no frontend test runner); `wails build -tags webkit2_41`.

## Tasks
- [x] T1 sheet: `Grid` derived from page + cover height, size presets, `Options`, `Build(w, paths, opts)`; table-driven tests per preset. Route: inline (context already loaded, one package).
- [ ] T2 app + UI + docs: `PrintSizes()` binding, `ExportPDF(paths, dir, opts)`, Cover size select, live page count, README Print sheet section. Route: inline.

## Progress
- T1 `07e1259` feat(sheet): Grid derived from page + cover height, presets, Options, `Build(w, paths, opts)`; fpdf page size follows the grid. Route: inline. RED: build failure on undefined Grid/Options/Sizes/A4. GREEN: `go test -tags webkit2_41 ./...` 252 passed (14 packages); vet and gofmt clean. Existing Standard numbers (35.6 mm columns, 15.5 mm margin, A4 MediaBox) unchanged.

## Next step
T2.
