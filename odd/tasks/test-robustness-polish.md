# Feature: test-robustness-polish (roadmap item 39)

## Objective
Close the small review advisories left open across earlier features: add the missing tests, fix the few real bugs they expose, and correct the README where it overstates.

## Problem / Why
Each advisory is cheap; together they harden the code (ROADMAP item 39).

## Scope / constraints
Mapped by a read-only explorer (sources: `odd/tasks/live-preview.md`, `book-spines.md`, `review-cards.md`, `print-then-cut.md`, `ticket-stubs.md`, `lettering-fonts.md`, `photo-crop.md`, `cover-size-presets.md`, `polaroid.md`).

Already done (no work):
- Live preview busy indicator cleared early: fixed in `5ccd026`, test `frontend/src/preview.test.ts:408`.
- Cover-size error text: asserted by `TestExportPDFRejectsUnknownSettingsBeforeAsking` (`app_test.go:724-747`).
- Polaroid A5 Standard 3 columns: pinned at `internal/sheet/polaroid_test.go:59`; only the tolerance itself is left (T3).

To do:
- Live preview: stale-load `void task.destroy()` (`frontend/src/preview.ts:115`) has no `.catch` → unhandled rejection; `previewMock.ts` `holdDraw()` overwrites the single `releaseDraw` closure, so earlier held draws never release; test a throwing/rejecting `loadingTask.destroy`.
- Print page: a failed `PrintFormats` binding leaves `formats = []` (`frontend/src/print.ts:816-822`) so `currentLayout()` is undefined and Export is disabled even for the plain covers sheet → fall back to the built-in covers format.
- Lettering: `letteringLoad.test.ts:24-26` waits one microtask hop; Go licence route `/fonts/go/licence` has no body check in `TestAssetHandlerServesFonts` (`app_test.go:907-940`).
- Print formats PDF: content-stream tests for spine colour/text colour/−90° rotation (`spines.go:75-93`), card text `Tj` (`cards.go:90,128`), bleed image `cm` placement from `paddedBox` (`pdf.go:206`).
- Bleed pad: `bleedPad(p.h, heightMM)` (`pdf.go:327`, `crop.go:186-192`) is applied horizontally too (`cut.go:115`); when W/w ≠ H/h after rounding the horizontal bleed can fall short → pad from both axes (bug fix, output may change by sub-pixel padding only where it was short).
- Spines colour buckets (`color.go:50-66`) can split a hue across a 4-bit bucket edge; README `README.md:530` overstates ("largest area of similar pixels") → correct the README wording; behaviour kept.
- Ticket stubs: notch curves not proved to bite inward at the perforation (`stubs.go:130-145`); `pdf_test.go:731` `LastIndex` slice panics when no dash reset is drawn and checks only the last perforation.
- AniList: `TestAniListMatchesAnyTitle` (`internal/providers/anilist_test.go:174-199`) advisory had no detail text → assert the request per subtest / order independence.
- Polaroid: direct `newFixedGrid` tolerance test (`card.go:63-82`, `tol = 1e-9`).

Out of scope: new features, output changes other than the bleed-pad fix, colour bucket algorithm change.

## Acceptance criteria
- Each fix has a test that fails before it; no output change except where a bug is fixed.
- ROADMAP item 39 ticked.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- `cd frontend && npx tsc --noEmit && npx vitest run`; delete `frontend/.vitest/` if created.

## Delivery
- Branch `feat/test-robustness-polish` from `8a93ec2`. Forecast ~450 authored lines (mostly tests). Strategy `single-pr` (cached project choice: user checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `8a93ec2`.
- Test-first: Go and vitest tests are runnable and deterministic → RED before GREEN for the bug fixes; test-only items pin current behaviour (GREEN on first run, stated as such).

## Tasks
- [x] T1 Frontend: preview destroy `.catch` + mock `holdDraw` fix + destroy-failure tests; print `PrintFormats` fallback + test; lettering retry test without the one-hop assumption. Route: delegated writer (writer trigger: several TS files).
- [ ] T2 Go print formats: spine/card/bleed stream tests, bleed pad from both axes + test (incl. crop window), ticket stubs notch + perforation test hardening, README colour wording. Route: delegated writer (writer trigger: several Go files).
- [ ] T3 Go misc + close: Go licence body test, AniList test hardening, `newFixedGrid` tolerance test, ROADMAP tick. Route: delegated writer (several test files).

## Progress
- 2026-11-24 Exploration done (explorer, path:line evidence above).
- 2026-11-24 T1 done (route: delegated writer). `preview.ts`: one `destroy(task)` helper guards throw and rejection for both the stale-load path and freeing the shown document. `previewMock.ts`: `destroyFails: 'throw' | 'reject'` option; `holdDraw` keeps a list of pending holds and `releaseDraw` releases them all. `print.ts`: a failed `PrintFormats` falls back to a built-in `covers` format (as `internal/sheet.Formats` describes it), error notice kept. `letteringLoad.test.ts`: retries with `vi.waitFor(() => letteringFonts())` instead of one microtask hop (test-only, green first run). RED: stale load with rejecting destroy → vitest "Unhandled Rejection: Error: cannot destroy" (`loadPdf preview.ts:115`); holdDraw twice → `expected [ false, true ] to deeply equal [ true, true ]`; PrintFormats rejected → `expected [] to deeply equal [ 'covers' ]` (format select empty, so Export disabled). Green first run (pin behaviour): shown-document destroy throw/reject, stale destroy throw. GREEN: `npx tsc --noEmit` no errors; `npx vitest run` 20 files, 575 tests passed, no unhandled errors.

## Next step
T2.
