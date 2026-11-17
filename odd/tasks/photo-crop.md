# Feature: photo-crop (roadmap item 21)

## Objective
Add a **Crop to** option to the Print sheet tab so photos print in fixed standard shapes: **None** (default, today's behaviour), **Instant mini**, **Square**, **Photo strip (3)**, **Photo strip (4)** and **Wallet**. Images are centre-cropped to fill their frame exactly, with no distortion.

## Problem / Why
People print their own photos onto sticker paper. The print sheet already takes any folder, but photos need fixed shapes.

## Scope / constraints
- Extend, don't replace: new `Options.Crop` (`json:"crop"`, a CropMode ID; `""` means `none`), a `CropMode` list + `CropModeByID` in `internal/sheet`, following the `CutMode` / `Size` / `Paper` pattern. New `Format.UsesCrop` flag: `true` only for **Cover sheet**; other formats ignore `Crop`.
- Crop modes (all sizes in mm, portrait unless noted):
  - `none` — today's behaviour, byte-identical output.
  - `instax` "Instant mini" — white frame 54 × 86, image window 46 × 62, 4 mm sides, 7 mm top (17 mm bottom band, empty).
  - `square` "Square" — side = cover size preset height (Mini 25, Standard 50, Large 80); image fills the cell.
  - `strip3` / `strip4` "Photo strip (3 frames)" / "(4 frames)" — 2 × 6 in strip (50.8 × 152.4), 3 mm white border around and between frames, frames stacked vertically, equal size; each frame takes the next image (a strip holds 3 or 4 consecutive photos; the last strip may be partial).
  - `wallet` "Wallet" — 64 × 89; image fills the cell.
- Size preset applies only to `none` and `square`. Cut mode applies where the image fills its cell (`none`, `square`, `wallet`); framed modes (`instax`, strips) always draw the grey frame border (cards/polaroid border colour/width), like Polaroid.
- Crop math: centre crop of the source pixels to the window aspect ratio (crop width when the image is wider, height when taller), then the existing 300 dpi downscale/encode; never upscale beyond today's rule. Source files never modified — cropping happens in memory at PDF build time. Per-image offset and face heuristics: out of scope.
- Grid: fixed-size cells via the existing fixed-grid helper (MinMargin, Gap, centred). PerPage counts photos (strips × frames for strip modes).
- PDF default name: unchanged for `none`; otherwise add the crop FileTag (follow the existing `pdfFileName` rule).
- Credits (attribution sidecars) keep working.
- UI: **Crop to** select in the Print tab, shown only for formats with `usesCrop`; Size hidden when the crop mode doesn't use it, Cut hidden when it doesn't apply. Page count updates. i18n labels (en + es, as the other selects).
- Docs: README Print sheet section (feature line + "Crop to" subsection).

## Acceptance criteria
- Cropped images fill their frames exactly, with no distortion. Default output unchanged (byte-identical page streams).
- Layout and crop math tests (table-driven).

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` and `npx vitest run` in `frontend/`; regenerate Wails bindings if Go structs change (`wails build -tags webkit2_41`, revert 0755 modes on `frontend/wailsjs/go/main/*` and `models.ts`).

## Delivery
- Branch `feat/photo-crop` from `2d82853`. Forecast ~800 authored lines (≈ 55 % tests). Strategy `single-pr` (user GUI-checks, then local `--no-ff` merge), as for earlier roadmap items. Push/merge are the user's decision.

## Tasks
- [x] T1 sheet: `CropMode` list, `Options.Crop`, `Format.UsesCrop`, crop geometry, centre-crop math, grids, PDF drawing, table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`).
- [x] T2 app + UI + docs: `PrintCropModes` binding, export name, Print tab control + i18n, frontend tests, README. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/photo-crop` from `2d82853`. RDD on (global).
- Review of 1095479..fc6a40c (anime-manga range on main + this plan; medium, 1237 lines): consent granted, reliability lens approved, acknowledged (authority burned). Advisory R3-001 SUGGESTION at `internal/providers/anilist_test.go:186` (pre-existing anime-manga test; follow-up only). Next reviewed boundary: fc6a40c.
- T1 done — `e1f9107` feat(sheet): add crop to standard print sizes. Files: `internal/sheet/crop.go` (new, 261), `crop_test.go` (new, 671), `layout.go`, `flow.go`, `pdf.go`; +1028 / −31 (≈ 65 % tests; over the ~400 advisory heuristic because of the table-driven tests, not split).
  - RED: first `go test ./internal/sheet/` failed to build (`undefined: DefaultCrop`, `CropModes`, `CropModeByID`…); with compiling stubs, TestCropModeByID, TestCropModesHaveFileSafeTags, TestFormatUsesCrop, TestCentreCrop (7 cases) and TestCropGeometry failed, then a panic (empty layout).
  - GREEN: `go test -tags webkit2_41 ./...` 1314 passed in 15 packages; `go vet` clean; `gofmt -l .` empty.
  - Defaults unchanged: page-stream SHA-256 of every format × paper × size × cut (189 combos, 6 mixed-shape fixtures) at `fc6a40c` vs after — identical (scratch test via `go test -overlay`, outside the repo). `TestBuildCropNoneKeepsTheCoverSheet` keeps `Crop: "none"` == `Crop: ""` for every paper × size × cut.
  - As built: `CropMode{ID, Label, FileTag, UsesSize, UsesCut, Frames}`; `CropModeByID` errors on unknown IDs (validated in every format, like cut modes); `Options.cropMode()` is None for formats without `UsesCrop`. `NewCropGrid` uses `newFixedGrid`. Pieces: instax 54 × 86, window 46 × 62 at (4, 7); square = cover height; wallet 64 × 89 filled; strips 50.8 × 152.4 with 3 mm borders, frames 44.8 × 46.8 (3) / 44.8 × 34.35 (4). `centreCrop` keeps full height (wider image) or width, centred, ≥ 1 px; `printCroppedImage` crops via `SubImage` then the existing 300 dpi `fitImage`/JPEG path (`encodePrinted`). Square/wallet draw through the shared `drawCovers` (border / none / bleed + crop marks); instax and strips draw the grey frame border and ignore cut. `PerPage` = cells × frames; `Pages`/`PerPageFor` skip shaped cells when cropping.
  - Photos per page (A4 / Letter / A5): instax 9 / 6 / 4; wallet 6 / 4 / 4; strip3 9 / 9 / 6 (3 / 3 / 2 strips); strip4 12 / 12 / 8; square mini 54 / 63 / 24, standard 15 / 12 / 6, large 6 / 6 / 2.
  - Deviation: none.

- T2 done — `238172c` feat(ui): offer crop to option in the print sheet. Files: `app.go`, `app_test.go`, `frontend/index.html`, `frontend/src/print.ts`, `print.test.ts`, `i18n/en.ts`, `i18n/es.ts`, regenerated `frontend/wailsjs/go/main/App.{d.ts,js}` + `models.ts` (modes restored to 644), `README.md`; +434 / −68 (≈ 32 authored lines are generated bindings).
  - RED: `go test .` failed to build (`unknown field UsesCrop`, `PrintCropModes undefined`, `undefined: PrintCropModeInfo`, `unknown field CropPerPage`); `vitest` failed on `main.PrintCropModeInfo` before the bindings existed and the helpers were exported.
  - GREEN: `go test -tags webkit2_41 ./...` 1329 passed in 15 packages; `go vet` clean; `gofmt -l .` empty; `npx tsc --noEmit` no errors; `npx vitest run` 75 passed (3 files). `git diff --summary` shows no mode changes.
  - As built: `PrintFormatInfo.usesCrop`; `PrintCropModes()` → `{id, label, usesSize, usesCut, frames, default}`; `PrintPaperInfo.cropPerPage[cropID][sizeID|""]` (every mode but None, from `sheet.Options.PerPage`); `widePerPage` is nil when cropping. File name: `<format>_<paper>[_<size> if the crop uses it]_[<crop> unless none][_<cut> if the crop uses it and it is not border]`, e.g. `covers_A4_instax.pdf`, `covers_A4_Large_square.pdf`, `covers_Letter_wallet_none.pdf`. UI: **Crop to** select between Paper and Cover size, shown only for `usesCrop` formats; Cover size hidden for instax/strips/wallet; Cut hidden for instax/strips; page count and size labels use `cropPerPage`; crop is part of the page-count request and shape keys. i18n `print.crop`, `print.cropTitle`, `print.errCrops`, `crop.<id>` in en + es. README: feature line, intro, steps 4, 5 and 8, new "Crop to" subsection with geometry and per-page tables.
  - Deviations: the Cover size field is *hidden* (not disabled) for fixed-size crop modes, as the spec says; formats without sizes keep today's disabled field. If `PrintCropModes` fails to load, the cover sheet falls back to no crop and shows `print.errCrops`.
- Parent spot check: `go test -tags webkit2_41 ./internal/sheet/ .` 771 passed.
- Assess fc6a40c..794bcd9: medium, 1581 lines, `review_due` (`slice_budget_reached`). Consent granted; reliability lens approved, acknowledged (authority burned). Advisory R3-001 WARNING at `internal/sheet/crop.go:186-192` (`printCroppedImage`; informational, no detail text; follow-up only). Next reviewed boundary: 794bcd9.

## Next step
User GUI-checks the Crop to option (rebuild `build/bin/cover-downloader` first), then local `--no-ff` merge to main and mark roadmap item 21 done.
