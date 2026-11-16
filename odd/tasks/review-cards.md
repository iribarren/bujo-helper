# Feature: review-cards (roadmap item 3)

## Objective
Add a **Review cards** print format: per selected image, a 90 × 55 mm card with the cover on the left, then the title and creator (parsed from the file name), an empty date field ("Read on" for books, "Watched on" for movies), 5 empty stars and 3 writing lines. Cards tile on the page with grey cut borders.

## Problem / Why
A cover with a rating, a date and a few notes is the most common reading-log element.

## Scope / constraints
- Extend, don't replace: first item to touch the shared **Format** control and the file-name parser, so both are created here (`sheet.Formats`, `Options.Format`, `""` = `covers` = today's sheet; `sheet.ParseName`).
- File names: `Title - Creator.ext`, duplicates `Title - Creator-2.ext`. Split on the last `" - "`; no `" - "` means title only. Movies' creator is the year.
- Card: fixed 90 × 55 mm, tiled in a centred grid (gap 4, margin ≥ 8). Expected per page: A4 2 × 4 = 8, Letter 2 × 4 = 8, A5 1 × 3 = 3. Cover size select does not apply to cards.
- Date label: `Options.DateLabel` (`""` → "Date"). Media types get a `DateLabel` ("Read on", "Watched on"); the Print tab offers a **Cards for** select (media types) shown for Review cards, defaulting to the Download tab's media type.
- Text: Go fonts (`golang.org/x/image/font/gofont`, already a dependency) embedded as UTF-8 fonts, so accented titles print. Title wraps to 2 lines max, creator 1 line, overflow ends with "…". Fitting is a pure function over a width measure, tested without a PDF.
- Defaults keep today's output: Cover sheet format unchanged.
- PDF default name: `cards_<paper>.pdf` for cards; covers keep `covers_<paper>_<size>.pdf`.
- Delivery: branch `feat/review-cards`, forecast ~650 authored lines; strategy `single-pr` (as items 1–2: user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- PDF export works for books and movies; long titles wrap or truncate with an ellipsis and never overflow the card.
- Cover sheet output unchanged with default options.
- Tests for card layout, text fitting and file-name parsing.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41`.

## Tasks
- [x] T1 sheet: `Formats`/`FormatByID`, `Options.Format`/`DateLabel`, `ParseName`, text fitting, card grid and card parts layout, card PDF rendering with Go fonts; table-driven tests. Route: delegated writer (writer trigger: 4+ non-trivial files in internal/sheet).
- [x] T2 app + UI + docs: `DateLabel` on media types and `MediaTypeInfo`, `PrintFormats` binding and per-format per-page counts, `ExportPDF` file name per format, Format and Cards-for selects (cover size disabled for cards), live page count, README Print sheet section. Route: delegated writer (writer trigger: app.go, catalog, print.ts, index.html, README).

## Progress
- Feature doc created; branch `feat/review-cards` (`2de0bef`). Its review: low risk, closed approved, acknowledged.
- T1 `29a8e3d` feat(sheet): `Format`/`Formats`/`FormatByID` (covers default, cards), `Options.Format`/`DateLabel`, `Options.PerPage()`, `NewCardGrid`, `Grid.Cells`, `ParseName`, `fitLines`, `layoutCard`, card PDF with Go fonts (UTF-8). 11 files, +1148/−27 (~600 test lines; over the ~400 heuristic because the brief bundled four units with their tests). Route: delegated writer. RED: compile failures (`unknown field Format`, `undefined: CardWidth`…). GREEN: `go test -tags webkit2_41 ./...` 338 passed; vet, gofmt clean; app builds. Cover sheet PDF bytes identical before/after for 3 option sets (fixed creation date). Cards per page: A4 8 (2 × 4), Letter 8, A5 3 (1 × 3). Visual check of a card at 200 dpi: wrapped accented title with "…", creator, "Read on:" rule, 5 stars, 3 lines, all inside the card. Parent spot check: `go test ./internal/sheet` 92 passed.
- Known ambiguity: a creator-less title ending in `-N` (N ≥ 2) parses as a duplicate (`Catch-22.jpg` → "Catch"); with a creator it parses correctly.
- Assess 2de0bef..29a8e3d: medium, 1175 lines, `review_due` (`slice_budget_reached`). Consent granted; reliability lens approved, acknowledged (authority burned). Reviewed boundary is now 29a8e3d. Informational findings: R3-cards-text-unasserted (WARNING: card PDF tests don't prove title/creator/label are drawn; covered by the visual check, not scheduled), R3-date-rule-colour (SUGGESTION: date rule colour left over from the cut border; fixed in T2).
- T2 `5bea644` feat(ui): `MediaType.DateLabel` ("Read on", "Watched on") and `MediaTypeInfo.dateLabel`; `PrintFormats()` binding; `PrintPaperInfo.formatPerPage` (cards: A4 8, Letter 8, A5 3); `ExportPDF` names `cards_<paper>.pdf` (size only when the format uses it; unknown size ignored for cards); Format select, Cards for select (only for Review cards, follows the Download tab's media type until changed by hand), Cover size disabled for cards, live page count per format; date rule colour set explicitly (R3-date-rule-colour); README Print sheet + Review cards subsection. 14 files, +320/−45. Route: delegated writer. RED: compile failures (`unknown field DateLabel`, `PrintFormats undefined`), then `covers_A5_Standard.pdf` instead of `cards_A5.pdf`. Unknown-format cases had no real RED (`opts.Grid()` already rejected them). GREEN (writer): `go test -tags webkit2_41 ./...` 346 passed; vet, gofmt clean; `tsc --noEmit` ok; `wails build -tags webkit2_41` ok (binary 12:02:55); no mode changes. Parent spot check: `go test` 346 passed.
- Assess 29a8e3d..5bea644: medium, 365 lines, `review_due` false (`under_budget`).

## Next step
User's GUI check of the rebuilt binary (`build/bin/cover-downloader`), then merge.
