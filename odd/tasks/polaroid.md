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
- [x] T1 sheet: `polaroid` format, frame geometry, `NewPolaroidGrid`, `layoutPolaroid`, `buildPolaroids`, table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`).
- [x] T2 app + UI + docs: app tests (formats list, per-page, export name), any Print tab tweak, README. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/polaroid` from `56b3551`. RDD on (global). Plan commit 6f28aa8: low risk, auto-approved, acknowledged.

- T1 done in `9d2a6fb` feat(sheet): add polaroid print format. Route: delegated writer. 7 files, +672 / −8 (≈ 460 test lines). Over the ~400-line heuristic because geometry, grid, caption fitting and PDF output each need table-driven coverage; no artificial split.
  - Files: `internal/sheet/polaroid.go` (geometry, `NewPolaroidGrid`, `layoutPolaroid`, `polaroidFontSize`, `layoutPolaroidCaption`), `polaroids.go` (`buildPolaroids`, drawing), `polaroid_test.go`, `layout.go` (Formats entry after Checklist, `Options.Grid()` case), `pdf.go` (build switch), `format_test.go`, `app_test.go` (expected lists).
  - RED: `go test -tags webkit2_41 ./internal/sheet/` → build failed: `undefined: polaroidFrame`, `layoutPolaroid`, `NewPolaroidGrid`, `polaroidFontSize`.
  - GREEN: `go test ./...`, `go vet` clean, `gofmt -l .` empty.
  - Defaults unchanged: page-stream hashes of 896 renders (every existing format × paper × size × cut × header, fixed dates, mixed fixtures) identical before (6f28aa8) and after, via `go test -overlay` scratch test outside the repo.
  - Design as built: window ⅔H × H, border 0.08H, bottom band 0.28H; frames Mini 20.67 × 34, Standard 41.33 × 68, Large 66.13 × 108.8 mm. Per page A4 / Letter 56 / 12 / 4, A5 25 / 6 / 1. Caption clamp(0.18H, 6, 12) pt → 6 / 9 / 12 pt, ≤ 2 lines (capped to what fits the band), centred; empty title → none. 0.2 mm grey border around the frame only. Cut mode and header ignored (tested).
- T2 done in `83c3303` feat(ui): offer polaroid format in the print sheet. 5 files, +54 / −20: `app_test.go` export names, `app.go` doc, `frontend/index.html` tooltips, `frontend/src/print.ts` comments, README (feature line, steps, Cut mode note, "Polaroid" subsection). No Go struct change → no `wails build`. RED: none meaningful (export-name cases are characterisation). GREEN: tests, vet, gofmt, `npx tsc --noEmit` clean.
  - Deviation: PDF name is `polaroid_<paper>_<size>.pdf` (existing `pdfFileName` rule for `UsesSize` formats, like spines), not `polaroid_<paper>.pdf`.
- Parent spot check: `go test -tags webkit2_41 ./internal/sheet/ .` 398 passed; A4 Standard render reviewed (4 × 3 frames, thick bottom band, centred captions, long title cut with "…", square/wide/narrow images inside the window).

- Assess 6f28aa8..cfdbf6b: medium, 772 lines, `review_due` (`slice_budget_reached`). Consent granted; reliability lens approved, acknowledged (authority burned). Advisory R3 SUGGESTION (`internal/sheet/polaroid_test.go:59`): A5 Standard fills the width exactly (3 × 41.33 + 2 × 4 = 132 mm), so its 3 columns depend on `newFixedGrid`'s tolerance; the test pins the boundary. Left as is (follow-up only if the tolerance changes).

## Next step
Manual GUI check (Polaroid in the Format list, Cover size applies, Cut mode hidden, page count per size, export).
