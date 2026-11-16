# Feature: polaroid (roadmap item 11)

## Objective
Add a **Polaroid** print format: each cover sits inside a white instant-photo frame with a thicker bottom margin that holds the title as a caption, with a grey cut border around the frame.

## Problem / Why
Instant-photo-style frames are a popular decoration style for media spreads.

## Scope / constraints
- Extend, don't replace: new `Format` `polaroid` (Label "Polaroid", FileTag "polaroid", Noun "polaroid", `UsesSize: true`, `UsesCut: false`, `UsesHeader: false`) in `sheet.Formats`, after Checklist. Reuse `ParseName` (title only), `fitLines` / `addTextFonts`, `printImages`, `newFixedGrid`, `Grid.Cells`, the card border colour/width.
- Frame geometry from the size preset's cover height H (so Mini / Standard / Large all work); constants/functions exported or package-level for tests:
  - image window: (H × 2/3) wide × H tall;
  - side and top border: 0.08 × H (Standard 4 mm);
  - bottom band (caption): 0.28 × H (Standard 14 mm);
  - frame = window + borders (Standard ≈ 41.3 × 68 mm).
- Frames tile via `newFixedGrid(page, frameW, frameH, "polaroid")` (MinMargin, Gap, centred). Expected per page (check in tests): A4 Mini 56, Standard 12, Large 4; A5 Standard 6, Large 1; Letter computed the same way.
- Image: shrink-to-fit into the window, aspect ratio kept, centred in the window; non-2:3 images stay inside the window. Frame is white (no fill needed on white paper); the grey cut border is drawn around the frame only (cards' border colour and width).
- Caption: the title from the file name (no creator), regular text font, size scaled with H: clamp(0.18 × H pt, 6, 12) (Standard 9 pt). Up to 2 lines, centred horizontally, block centred vertically in the bottom band, within the window width (side borders as padding); overflow cut with "…", never outside the band. Empty title → no caption.
- Paper and size apply; cut mode and header don't (cut mode ignored, like cards — the border always stays).
- Defaults keep today's output: covers, cards, spines, stubs, full page and checklist byte-identical.
- PDF default name: `polaroid_<paper>.pdf` (check whether the size tag is added for `UsesSize` formats, as for covers/spines, and follow the existing rule).
- UI: format list comes from bindings (`PrintFormats`, `usesSize`), so the Print tab should need little or no change; regenerate Wails models only if Go structs change. README Print sheet: new "Polaroid" subsection + feature list lines.
- Delivery: branch `feat/polaroid` from `56b3551`; forecast ~550 authored lines (≈ 55 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- PDF export with the caption fitted to the frame (wraps to 2 lines, "…" overflow, never outside the bottom band).
- Layout tests: frame geometry per size preset, grid per paper × size, window/caption boxes inside the frame, image aspect kept inside the window, caption fitting.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41` (revert 0755 modes it sets on `frontend/wailsjs/go/main/*` and `models.ts`).

## Tasks
- [ ] T1 sheet: `polaroid` format, frame geometry, `NewPolaroidGrid`, `layoutPolaroid`, `buildPolaroids`, table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`).
- [ ] T2 app + UI + docs: app tests (formats list, per-page, export name), any Print tab tweak, README. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/polaroid` from `56b3551`. RDD on (global).

## Next step
T1.
