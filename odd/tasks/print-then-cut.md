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
- [ ] T1 sheet: cut modes, bleed geometry, crop marks, edge-extended bleed images, PDF rendering per mode; tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet).
- [ ] T2 app + UI + docs: cut modes binding, Cut mode select (shown only for formats with `UsesCut`), PDF name tag, README Print sheet "Cut mode" subsection with the print-then-cut note. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/print-then-cut`.
- Research (2026-11-16): Silhouette Type 1 default inset ≈ 0.625 in (≈ 16 mm) per third-party blogs, wider than the 8 mm margin; Cricut adds its own corner marks on "Make It". Registration marks dropped.

## Next step
T1.
