# Feature: book-spines (roadmap item 4)

## Objective
Add a **Spines** print format for bookshelf trackers: per selected image, a narrow vertical strip filled with the cover's dominant colour, with the title (and the creator, smaller) rotated 90° in black or white, whichever contrasts better.

## Problem / Why
Bookshelf trackers (a drawn shelf filled with spines) are a very popular spread, and drawing the spines by hand is tedious.

## Scope / constraints
- Extend item 3's **Format** control: add `{"spines", "Spines", "spines", UsesSize: true}` to `sheet.Formats`. Reuse `ParseName`, `fitLines`, the Go fonts and the cut-border style.
- Spine height = the cover size preset (Mini 25, Standard 50, Large 80 mm), so the Cover size select applies.
- Width varies a little for a natural shelf look, deterministically by column position (a fixed pattern of 8–12 mm widths), so every full row holds the same number of spines and the per-page count stays constant for the UI.
- Spines in a row are separated by a small gap (~2 mm), rows by `Gap`; rows centred on the page. Grey cut border around each spine.
- Text reads top-to-bottom (English spine convention). Title bold, creator smaller; both fitted along the spine length: shrink within limits, then truncate with "…". Never overflow the strip.
- Dominant colour: computed from the image with the standard library only (downscale, quantize into buckets, take the most populous bucket's mean). Text colour: black or white by WCAG contrast ratio.
- Per-page counts: the UI needs counts for a size-based format other than the cover sheet; unify the per-page data the `PrintPapers` binding sends (format × size) instead of adding a third ad-hoc map.
- Defaults keep today's output (cover sheet unchanged). Default PDF name `spines_<paper>_<size>.pdf`.
- Delivery: branch `feat/book-spines`, forecast ~700 authored lines (≈60 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- Text readable on light and dark colours; long titles shrink or truncate to fit.
- Tests for colour extraction (solid-colour fixtures) and the contrast choice; layout tests for spine rows per paper × size.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41`.

## Tasks
- [x] T1 sheet: spines format, dominant colour, contrast choice, spine layout, rotated-text PDF rendering; tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet).
- [x] T2 app + UI + docs: unified per-page counts (format × size), page count for spines, README Print sheet section. Route: same delegated writer, separate commit (writer trigger: app.go, print.ts, README).

## Progress
- Feature doc created; branch `feat/book-spines` (`b18fe19`). Its review: low risk, closed approved, acknowledged.
- T1 `2d83a68` feat(sheet): `spines` format (`UsesSize`, `Noun` on `Format`), `dominantColor` (flatten on white, ≤ 64 px, 4-bit buckets, majority bucket mean), `textColorFor` (WCAG; tie → black), spine layout via `Grid.Spines` (width pattern 10, 12, 9, 11, 8, 12, 10, 9, 11, 10 mm; 2 mm `spineGap`), rotated text (`TransformRotate(-90)`, reads top-to-bottom), shared `forEachImage`/`decodeImage`/`addTextFonts`. 11 files, +960/−25. RED: compile failures (`dominantColor`, `textColorFor`, `spineLine` undefined), then "PDF embeds no TrueType font" while spines still drew as a cover sheet. GREEN: `go test` all ok; vet, gofmt clean.
- T2 `52c01a4` feat(ui): `PrintPaperInfo.PerPage` unified as format → size → count (`""` size for cards; `formatPerPage` removed), `PrintFormatInfo.noun`, size labels show spines per page, `spines_<paper>_<size>.pdf`, README Spines subsection. 8 files, +162/−77. RED: unknown field `Noun`, `PerPage` not nested. GREEN (writer): `go test` ok; vet, gofmt clean; `tsc --noEmit` ok; `wails build -tags webkit2_41` ok (binary 12:42:45); no mode changes. Parent spot check: `go test -tags webkit2_41 ./...` 383 passed.
- Spines per page: A4 144 / 80 / 48 (Mini / Standard / Large, 16 per row); Letter 144 / 64 / 48; A5 66 / 33 / 22 (11 per row).
- Visual check (12 colour fixtures, 3 sizes): black/white text chosen correctly, two-tone takes the majority colour, long titles end in "…", no overflow. Parent viewed the Standard close-up.
- Route: one delegated writer for T1 and T2 (writer trigger), two commits.
- Review of b18fe19..52c01a4 (1220 lines, medium, `slice_budget_reached`): consent granted, reliability lens approved, acknowledged. Advisory, not scheduled: R3-spine-render-unasserted (no test asserts fill/text colour or rotation in the PDF stream), R3-bucket-boundary-split (fixed buckets can split a majority hue across bucket edges; README wording "largest area of similar pixels" slightly overstates).

## Next step
Whole-branch review, then the user's GUI check of the rebuilt binary, then merge.
