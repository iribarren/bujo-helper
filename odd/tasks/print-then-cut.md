# Feature: print-then-cut (roadmap item 6)

## Objective
Add a **Cut mode** to the cover sheet: **Border** (default, today's grey line), **Bleed** (each image extends 1 mm past its cut line, with crop marks), **None** (no border, no marks).

## Problem / Why
Users print onto sticker paper and cut with a Cricut or Silhouette, or cut by hand and want no white edges.

## Scope / constraints
- `sheet.Options.Cut` (`json:"cut"`), IDs `border` (default, `""`), `bleed`, `none`; a `CutModes` list with `CutModeByID`, like `Papers`/`Sizes`.
- `Format.UsesCut`: true for the cover sheet only. Review cards and spines keep their borders and ignore the option (follow-up, out of scope).
- Bleed: 1 mm (`Bleed` constant). The image is drawn with its edge pixels replicated outward by the bleed, so the content inside the cut line is identical to Border mode (no zoom, no crop). The cut line stays the box Border mode uses.
- Crop marks (Bleed only): at each cover corner, two thin black lines continuing the cut edges outward, from `Bleed` to `Gap/2` from the cut line, so marks never reach a neighbour's half of the gap. No grey border in Bleed mode.
- Grid, per-page counts and page count don't change with the cut mode.
- **No registration marks** (user decision, 2026-11-16): Cricut Design Space and Silhouette Studio add their own marks when the page is imported; Cricut warns marks printed outside its flow can fail. README note only.
- Defaults keep today's output exactly.
- Default PDF name gains a cut tag only for a non-default mode, e.g. `covers_A4_Standard_bleed.pdf`.
- Delivery: branch `feat/print-then-cut`, forecast ~500 authored lines (≈60 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- Default output unchanged. With bleed on, bleed areas don't overlap neighbours (the gap absorbs the bleed); crop marks stay within each cover's half of the gap.
- Layout tests for bleed geometry (per paper × size, including shrink-to-fit covers).

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41`.

## Tasks
- [x] T1 sheet: cut modes, bleed geometry, crop marks, edge-extended bleed images, PDF rendering per mode; tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet). Commit `a740646` — 7 files, +691/−47 (cut.go 118, cut_test.go 287, pdf_test.go +187).
- [x] T2 app + UI + docs: cut modes binding, Cut mode select (shown only for formats with `UsesCut`), PDF name tag, README Print sheet "Cut mode" subsection with the print-then-cut note. Route: same delegated writer, separate commit. Commit `b569f0f` — 8 files, +140/−16.

## Progress
- Feature doc created; branch `feat/print-then-cut`.
- Research (2026-11-16): Silhouette Type 1 default inset ≈ 0.625 in (≈ 16 mm) per third-party blogs, wider than the 8 mm margin; Cricut adds its own corner marks on "Make It". Registration marks dropped.

- T1 (2026-11-16): RED = `go test ./internal/sheet` build failure (undefined `DefaultCut`, `CutModes`, `CutModeByID`, `Line`). GREEN = all sheet tests pass. Baseline: SHA-256 of the uncompressed default cover-page stream (A4 Standard, Letter Mini, A5 Large; 3 mixed-ratio covers) taken before the change and identical after it; `TestBuildBorderCutKeepsTodaysCoverSheet` pins today's stream for "" and "border".
- T1 design note: in Bleed mode each image is also clipped to its bleed box (`ClipRect`), so a low-resolution image whose whole-pixel padding exceeds 1 mm still stops exactly at the bleed. Unknown cut IDs are rejected by `Options.Grid()` for every format (and so by `Build` and `ExportPDF` before the dialog).
- T2 (2026-11-16): RED = `go test .` build failure (unknown field `UsesCut`, undefined `PrintCutModes`/`PrintCutModeInfo`). GREEN = all app tests pass. Bindings regenerated with `wails generate module -tags webkit2_41`; it and `wails build` set 0755 on frontend/wailsjs files, reverted to 0644 (runtime/ files restored). The Cut mode field is hidden (not disabled) when the format does not use it.
- Checks after T2: `go test -tags webkit2_41 ./...` ok; `go vet -tags webkit2_41 ./...` no issues; `gofmt -l .` empty; `npx tsc --noEmit` no errors; `wails build -tags webkit2_41` built; `git diff --summary HEAD~2` no mode changes.
- Parent spot check: `go test -tags webkit2_41 ./...` 461 passed. Visual check: rendered a Bleed A4 sheet (portrait, square, landscape, mixed ratios) at 200 dpi; images run past the cut line, crop marks sit in the gaps at each corner.
- Review of 108f5af..fc8c74d (906 lines, medium, `slice_budget_reached`): consent granted, reliability lens approved, acknowledged. Advisory, not scheduled: R3-bleed-placement-unproved (WARNING: no test asserts the drawn image coordinates of the padded image in the PDF), R3-bleed-horizontal-pad-from-height (SUGGESTION: the horizontal pad uses the vertical px/mm; equal up to rounding since aspect is kept).
- Pending: GUI check by the user (single-pr strategy).

## Next step
User GUI check of the Cut mode select and a Bleed PDF (print at 100 %), then review and local `--no-ff` merge.
