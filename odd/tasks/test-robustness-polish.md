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
- [x] T1 Frontend: preview destroy `.catch` + mock `holdDraw` fix + destroy-failure tests; print `PrintFormats` fallback + test; lettering retry test without the one-hop assumption. Route: delegated writer (writer trigger: several TS files). Commit `865cf63`.
- [x] T2 Go print formats: spine/card/bleed stream tests, bleed pad from both axes + test (incl. crop window), ticket stubs notch + perforation test hardening, README colour wording. Route: delegated writer (writer trigger: several Go files). Commit `934d956`.
- [x] T3 Go misc + close: Go licence body test, AniList test hardening, `newFixedGrid` tolerance test, ROADMAP tick. Route: delegated writer (several test files). Commit `1e249cd`.

## Progress
- 2026-11-24 Exploration done (explorer, path:line evidence above).
- 2026-11-24 T1 done (route: delegated writer). `preview.ts`: one `destroy(task)` helper guards throw and rejection for both the stale-load path and freeing the shown document. `previewMock.ts`: `destroyFails: 'throw' | 'reject'` option; `holdDraw` keeps a list of pending holds and `releaseDraw` releases them all. `print.ts`: a failed `PrintFormats` falls back to a built-in `covers` format (as `internal/sheet.Formats` describes it), error notice kept. `letteringLoad.test.ts`: retries with `vi.waitFor(() => letteringFonts())` instead of one microtask hop (test-only, green first run). RED: stale load with rejecting destroy → vitest "Unhandled Rejection: Error: cannot destroy" (`loadPdf preview.ts:115`); holdDraw twice → `expected [ false, true ] to deeply equal [ true, true ]`; PrintFormats rejected → `expected [] to deeply equal [ 'covers' ]` (format select empty, so Export disabled). Green first run (pin behaviour): shown-document destroy throw/reject, stale destroy throw. GREEN: `npx tsc --noEmit` no errors; `npx vitest run` 20 files, 575 tests passed, no unhandled errors.
- 2026-11-25 T2 done (route: delegated writer). Bleed pad fix: `boxBleedPad(w, h, box)` (`cut.go`) takes the larger of the per-axis pads; `printImage` now takes the box, and `printCroppedImage` pads for its window via `encodePrintedOn` (`pdf.go`, `crop.go`); `encodePrinted` (plant page, no bleed) keeps its signature and assumes a box of the image's own shape. Output changes only where the horizontal pad was short; no existing stream/byte expectation changed. RED: `TestBleedPadCoversBothAxes` → "pad 2: image drawn on {X:9.0074… W:35.2350…} does not cover the bleed box {X:9 … W:35.25}" (67 × 100 px on 33.25 × 50 mm) and "{X:4.0476… W:11.9047…} does not cover … {X:4 … W:12}" (21 × 30 px on 10 × 15 mm); `TestPrintCroppedImage` (100 × 100 photo, 33.25 × 50 mm window, bleed) → same 35.235 < 35.25 mm shortfall. Test-only, green first run (pin behaviour): `formatstream_test.go` — spine fill `rg`, `re B` rect, −90° `cm` and black/white text grey with decoded title/creator; card title/creator/date label decoded from the UTF-16BE `Tj` strings with greys and x positions (default and custom label); stub notches lie on their circle, inside the stub, centred on the perforation, spanning the diameter (radial tolerance 0.05 pt for fpdf's Bézier arcs). `pdf_test.go`: bleed test asserts clip then `cm` from `paddedBox` per image; stubs test drops the panicking `LastIndex` slice, checks every dash reset follows an `l S`, and each stub's exact dashed perforation then reset. README spines dominant-colour wording corrected (coarse 16-level buckets, gradients can split); `TestDominantColorSplitsAHueAcrossABucketEdge` pins it (green first run). GREEN: `go test -tags webkit2_41 ./...` all packages ok (3715 tests/subtests passed, 0 failed); `go vet` clean; `gofmt -l .` empty.
- 2026-11-25 T3 done (route: delegated writer). Test-only plus one comment; no output change. All green on the first run (they pin current behaviour). `app_test.go` `TestAssetHandlerServesFonts`: `/fonts/go/licence` case (200, `text/plain; charset=utf-8`, body equal to the registry's `LicenceText` from `fonts/go/LICENSE`), and every body check now fails if the registry body is empty. `internal/providers/anilist_test.go` `TestAniListMatchesAnyTitle`: the R3-001 advisory had no detail text; at `fc6a40c` line 186 was the "English" case, which served the manga fixture `anilist_berserk_manga.json` through the anime provider, and the cases where the expected title is already AniList's top hit proved no ranking. Inferred meaning: the test did not check the request it sent nor that matching is independent of fixture order. Now each case names its media type (Berserk manga → `MANGA`), runs on the fixture as sent and with `data.Page.media` reversed (`reverseAniListMedia`), and asserts one request with `search` = the query and `type` = the media type (12 subtests). `internal/sheet/card_test.go` `TestNewFixedGridToleratesAnExactFit`: on A4/Letter/A5, n = 1..12 exactly fitting items give n × n; 1e-6 mm wider and taller give (n-1) × (n-1), or an error for n = 1; it also requires at least one exact fit whose float quotient falls short of n. Mutation check: with `tol = 0` it fails on "letter, 11 × 11 exact fit: grid is 10 × 11" and "a5, 7 × 7 exact fit: grid is 6 × 7"; `TestPolaroidGridPapersAndSizes` (A5 Standard 3 columns) still passed with `tol = 0`, so the new test is the one that pins the tolerance. `card.go`: comment at `tol` points to the test. ROADMAP item 39 ticked with a "Done in branch" block; the merge hash is added at merge, as for earlier items.
- 2026-11-24 Plan commit review: d4eb863 low risk (non_executable_only), auto-approved, acknowledged (lineage review-7b6582ea1e2e2e20). Reviewed boundary → d4eb863.
- 2026-11-24 T1 review: assess --base-ref d4eb863 → medium, under_budget; Stop hook preflight over 8a93ec2..865cf63 → medium, consent granted, 1 lens (reliability) approved, acknowledged, authority burned (lineage review-cef46d5b16b9ab3e). Advisory (non-blocking): R3-covers-fallback-drift — the built-in covers fallback in frontend/src/print.ts:201-207 can drift from the Go covers entry → follow-up. Reviewed boundary → 865cf63.
- 2026-11-25 T2 review: assess --base-ref 865cf63 → medium, slice_budget_reached; consent granted; 1 lens (reliability) approved, no advisories, acknowledged, authority burned (lineage review-9ce38c61a07c975f). Reviewed boundary → 934d956.
- 2026-11-25 Whole-branch review (Stop hook, `8a93ec2..934d956`): medium, consent **declined** by the user for this candidate (T1 and T2 already reviewed individually; lineage review-2f38098efa9d6a49).
- 2026-11-25 T3 commit `1e249cd`: assess `--base-ref 934d956 --committed-only` → medium, `under_budget` (pending in slice, no review due). Parent spot check: `go test` sheet/providers/root ok.
- 2026-11-25 Whole-branch review (Stop hook, `8a93ec2..b358c80`, incl. T3): medium, consent granted, reliability lens approved, acknowledged, authority burned (lineage review-1099ae8ef47e431b). Advisory (non-blocking): R3-001 at `internal/sheet/cut.go:95-97` (`boxBleedPad`), informational, no detail text → follow-up.
- 2026-11-25 Re-review after the doc commit (`8a93ec2..1698c0a`, same code): consent granted, reliability approved with the same R3-001 advisory, acknowledged (lineage review-2db3e44fe2211ef5). This record stays uncommitted until the merge, so it does not open another candidate.

## Follow-ups
- R3-covers-fallback-drift: the built-in covers fallback in `frontend/src/print.ts:201-207` can drift from the Go `PrintFormats()` covers entry. Add a parity check: a Go test that emits the covers entry as JSON and compares it with a fixture the TS test also reads, or at least a comment in each place cross-referencing the other.
- Branch R3-001 (`cut.go:95-97`, `boxBleedPad`): suggestion without detail text; likely a direct unit test of `boxBleedPad` for both "wider" and "taller" rounding cases and degenerate boxes.

- 2026-11-25: user asked for the merge. Merged to main locally in `0585dd0`.

## Next step
Done. Open follow-ups: see **Follow-ups**.
