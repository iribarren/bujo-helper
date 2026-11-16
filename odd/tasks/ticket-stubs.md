# Feature: ticket-stubs (roadmap item 8)

## Objective
Add a **Ticket stubs** print format: per selected image, a movie-ticket-shaped stub with a poster thumbnail, the title and creator (year for movies, author for books) from the file name, empty "Date" and "Where" fields, 5 empty rating stars, and a perforation-style dashed line separating a small "ADMIT ONE" stub end. Stubs tile on the page with grey cut borders.

## Problem / Why
Ticket stubs are a classic movie-tracker element.

## Scope / constraints
- Extend, don't replace: new `Format` `stubs` (Label "Ticket stubs", FileTag "stubs", Noun "stub", `UsesSize: false`, `UsesCut: false`) in `sheet.Formats`; reuse `ParseName`, `fitLines`, `addTextFonts`, `starPoints`, `baseline`/`lineHeight`, `printImages`, `Grid.Cells`.
- Stub: fixed 90 × 40 mm, tiled in a centred grid (gap 4, margin ≥ 8), as the card grid. Expected per page: A4 2 × 6 = 12, Letter 2 × 6 = 12, A5 1 × 4 = 4.
- Shape (the cut line, grey border): a rectangle with a semicircular notch (radius ~2.5 mm) in the top and bottom edges where the perforation sits, ~20 mm from the right edge. Perforation: a vertical dashed line between the notches.
- Main part (left of the perforation), inside 3 mm padding: poster thumbnail, full height, 2:3 area, shrink-to-fit, left-aligned, centred vertically. Text column right of it: title (bold, up to 2 lines, "…" overflow), creator (1 line), "Date:" and "Where:" labels each with a rule to write on, 5 empty stars.
- Stub end (right of the perforation): "ADMIT ONE" rotated 90°, centred. Labels are fixed ("Date", "Where"): movie-first wording that also reads fine for books; `Options.DateLabel` does not apply.
- Cover size and cut mode don't apply (stubs keep their grey border, like review cards).
- Defaults keep today's output: cover sheet, cards, spines unchanged.
- PDF default name: `stubs_<paper>.pdf`.
- Delivery: branch `feat/ticket-stubs`, forecast ~600 authored lines (≈55 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- PDF export with several stubs per page and cut borders. Text fits as in review cards (wraps/truncates with "…", never overflows).
- Layout tests: grid per paper, parts inside the stub and the main part, notches and perforation aligned, thumbnail aspect kept.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41`.

## Tasks
- [x] T1 sheet: `stubs` format, `NewStubGrid`, `layoutStub`, stub outline path with notches, perforation, PDF rendering; table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet).
- [x] T2 app + UI + docs: per-format per-page counts and `ExportPDF` name for stubs (check what is already generic), Print tab copy, README Print sheet "Ticket stubs" subsection and feature lists. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/ticket-stubs` from `0307fdd`. RDD on (global).

- T1 done in `1cf932a` feat(sheet): add ticket stubs print format. Route: delegated writer. 10 files, +725 / −36 (≈ 761 authored lines, ≈ 57 % tests: `stub_test.go` +300, `pdf_test.go` +124, `format_test.go` +7, `app_test.go` +4). Over the ~400-line heuristic because the format needs its own layout, drawing and full table-driven tests; no artificial split.
  - Files: `internal/sheet/stub.go` (constants, `NewStubGrid`, `stubParts`, `layoutStub`, `padded`), `internal/sheet/stubs.go` (`buildStubs`, `drawStub`, `drawStubOutline`), `layout.go` (`stubsFormat`, Formats entry, `Options.Grid` fixed-size switch), `pdf.go` (build switch), `card.go` (`NewCardGrid` → shared `newFixedGrid(page, w, h, noun)`), `cards.go` (title/creator drawing extracted to `drawTitleAndCreator`, shared with stubs). `app_test.go`: `TestPrintFormats` / `TestPrintPapers` list every format, so they gain the stubs entry (12 / 12 / 4) here to keep the commit green.
  - RED: `go test -tags webkit2_41 ./internal/sheet/` → build failed: `undefined: StubWidth`, `undefined: NewStubGrid`, `undefined: layoutStub`, `undefined: stubEndWidth` …
  - GREEN: `go test -tags webkit2_41 ./...` → all packages ok; `go vet` clean; `gofmt -l .` empty.
  - Defaults unchanged: page streams of covers, cards and spines (uncompressed, fixed dates, `cutFixtures`) captured before and after the change are byte-identical (scratch comparison; refactors of `NewCardGrid` and the title/creator drawing are output-neutral).
  - Design notes: stub 90 × 40 mm, perforation 20 mm from the right edge, notch radius 2.5 mm (inside the 3 mm padding). Reuses the card padding, 2:3 cover area, `cardTextGap`, text sizes and stars. Vertical stack: title (2 lines) + creator at the top, stars on the bottom padding, "Date:" and "Where:" spread with three equal gaps (≈ 3.9 mm) between them; both rules start after the longer label. "ADMIT ONE" bold 8 pt, rotated +90° (reads bottom to top) about the centre of the stub end. Perforation dashes 1.0 / 0.8 mm, dash pattern reset right after. Outline: one closed path (`MoveTo`/`LineTo`/`ArcTo`/`ClosePath`/`DrawPath("D")`).
  - Visual check: A4 sample with 5 mixed-ratio images and long accented titles rendered with `pdftoppm -r 200`: notches, dashed perforation, rotated ADMIT ONE, five stars, all text inside the main part, "…" truncation.

- T2 done in `31bb964` feat(ui): offer ticket stubs in the print sheet. Route: delegated writer. 5 files, +42 / −12.
  - App and UI were already generic over `sheet.Formats` (`PrintFormats`, per-format per-page counts, `pdfFileName` from `FileTag`/`UsesSize`/`UsesCut`, Print tab size/cut toggles from `usesSize`/`usesCut`): no wiring code needed, no Go binding type changed, so no Wails bindings regenerated.
  - Changes: `app_test.go` export names (`stubs_A4.pdf`, `stubs_Letter.pdf`, size/cut ignored → `stubs_A5.pdf`); `app.go` `pdfFileName` doc example; `frontend/src/print.ts` header comment; `frontend/index.html` Format tooltip; README feature line, Print sheet intro/format list/size step/file names, Cut mode note, new "Ticket stubs" subsection, `internal/sheet` package line. Per-page counts (12 / 12 / 4) were added to `TestPrintPapers` in T1.
  - RED exception: the export-name cases pass on first run because `pdfFileName` is already generic; they pin behaviour (characterisation), not drive it.
  - GREEN: `go test -tags webkit2_41 ./...` all ok; `go vet` clean; `gofmt -l .` empty; `npx tsc --noEmit` no errors; `wails build -tags webkit2_41` built. `wails build` set 0755 on `frontend/wailsjs/go/main/App.{d.ts,js}` and `models.ts` (content unchanged); modes reverted to 0644, `git diff --summary 08579b4..HEAD` shows no mode changes.

- Parent spot check: `go test -tags webkit2_41 ./internal/sheet/ .` 274 passed; rendered A4 page reviewed (notches, perforation, ADMIT ONE, truncation all as designed).
- Plan commit 08579b4: low risk, auto-approved, acknowledged.
- Assess 08579b4..dd16e2d: medium, 835 lines, `review_due` (`slice_budget_reached`). Consent granted; reliability lens approved, acknowledged (authority burned). Advisory, not scheduled: R3-001 (SUGGESTION, `pdf_test.go:725-730`), R3-002 (SUGGESTION, `pdf_test.go:595`).

## Next step
User GUI check (Format → Ticket stubs: Cover size disabled, Cut mode hidden, page count, export `stubs_A4.pdf`), native review, then local `--no-ff` merge per `single-pr`.
