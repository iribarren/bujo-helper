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
- [ ] T1 sheet: `Formats`/`FormatByID`, `Options.Format`/`DateLabel`, `ParseName`, text fitting, card grid and card parts layout, card PDF rendering with Go fonts; table-driven tests. Route: delegated writer (writer trigger: 4+ non-trivial files in internal/sheet).
- [ ] T2 app + UI + docs: `DateLabel` on media types and `MediaTypeInfo`, `PrintFormats` binding and per-format per-page counts, `ExportPDF` file name per format, Format and Cards-for selects (cover size disabled for cards), live page count, README Print sheet section. Route: delegated writer (writer trigger: app.go, catalog, print.ts, index.html, README).

## Progress
- Feature doc created; branch `feat/review-cards`.

## Next step
T1.
