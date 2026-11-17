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
- [ ] T1 sheet: `CropMode` list, `Options.Crop`, `Format.UsesCrop`, crop geometry, centre-crop math, grids, PDF drawing, table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`).
- [ ] T2 app + UI + docs: `PrintCropModes` binding, export name, Print tab control + i18n, frontend tests, README. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/photo-crop` from `2d82853`. RDD on (global).

## Next step
T1.
