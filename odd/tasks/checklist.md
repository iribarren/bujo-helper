# Feature: checklist (roadmap item 10)

## Objective
Add a **Checklist** print format (TBR list / watchlist): a header the user types (e.g. "TBR 2026"), then one row per item with a checkbox, a small thumbnail, the title and the creator. Rows paginate.

## Problem / Why
To-be-read lists and watchlists are standard bujo collections.

## Scope / constraints
- Extend, don't replace: new `Format` `checklist` (Label "Checklist", FileTag "checklist", Noun "row", `UsesSize: false`, `UsesCut: false`, `UsesHeader: true`) in `sheet.Formats`, after Full page. Reuse `Options.Header`, the full-page header band and header drawing (shrink then "…"), the file-name parser in `names.go`, and `fitLines` / text fonts.
- Row layout (fixed row height, constants exported for tests): within the usable width (page − 2 × `MinMargin`): checkbox square, thumbnail, then title (bold) on line 1 and creator (regular, grey) on line 2, each one line, truncated with "…" — never past the right margin. Empty creator → title only. Thin light-grey rule under each row (writing-line look). No cut borders.
- Thumbnail fits the row height (row height − vertical padding) in a 2:3 box, keeping aspect ratio with the existing shrink-to-fit rule (`Grid.place` or equivalent); non-2:3 images stay inside the box.
- Rows per page = floor((usable height − header band if header) / row height); header repeated on every page; rows start right below the header (top-aligned, not vertically centred, like a list page). `Options.PerPage()` returns that count for the selected paper (header present or not must be reflected consistently with full page's approach — document the choice).
- Paper applies; size preset and cut mode don't. Defaults keep today's output: covers, cards, spines, stubs and full page byte-identical.
- PDF default name: `checklist_<paper>.pdf`.
- UI: the format list and Header field come from bindings (`PrintFormats`, `usesHeader`), so the Print tab should need little or no change; regenerate Wails models only if the Go structs change. README Print sheet: new "Checklist" subsection + feature list lines.
- Delivery: branch `feat/checklist` from `2629b90`; forecast ~500 authored lines (≈ 50 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- PDF export for books and movies, with long titles truncated.
- Layout tests for rows per page (per paper, header on/off), row geometry inside margins, thumbnail fitting, text truncation, pagination with the header repeated.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41` (revert 0755 modes it sets on `frontend/wailsjs/go/main/*` and `models.ts`).

## Tasks
- [ ] T1 sheet: `checklist` format, row geometry, `buildChecklist`, pagination, table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`).
- [ ] T2 app + UI + docs: app tests (formats list, export name), any Print tab tweak, README. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/checklist` from `2629b90`. RDD on (global).

## Next step
T1.
