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
- [ ] T1 sheet: `stubs` format, `NewStubGrid`, `layoutStub`, stub outline path with notches, perforation, PDF rendering; table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet).
- [ ] T2 app + UI + docs: per-format per-page counts and `ExportPDF` name for stubs (check what is already generic), Print tab copy, README Print sheet "Ticket stubs" subsection and feature lists. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/ticket-stubs` from `0307fdd`. RDD on (global).

## Next step
T1 + T2 via one delegated writer.
