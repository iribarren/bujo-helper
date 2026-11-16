# Feature: full-page (roadmap item 9)

## Objective
Add a **Full page** print format: a header the user types (e.g. "Books of 2026"), then the selected covers in a tight grid that fills the page, cover size chosen automatically so all covers fit on one page when possible. No cut borders. Beyond the per-page cap it paginates, repeating the header.

## Problem / Why
Some users want a finished page to glue or bind into the journal, with no cutting.

## Scope / constraints
- Extend, don't replace: new `Format` `fullpage` (Label "Full page", FileTag "fullpage", Noun "cover", `UsesSize: false`, `UsesCut: false`, new `UsesHeader: true`) in `sheet.Formats`. Other formats get `UsesHeader: false`.
- New `Options.Header string` (`json:"header"`), trimmed; "" means no header band (grid uses the full height). Ignored by other formats.
- Per-page cap `FullPageMax = 60`. New `Grid.Limit int`: when > 0, `Grid.PerPage()` returns `min(Columns*Rows, Limit)`; zero keeps today's behaviour. `Options.PerPage()` for full page therefore returns 60 (the UI page count stays generic: `ceil(n / 60)`).
- Auto size `NewFullPageGrid(page, count, header bool)`: n = min(count, 60). Usable area = page − 2 × `MinMargin`, minus the header band when there is a header. Tight gap 3 mm. For each column count c in 1..n: rows r = ceil(n / c); cover height h = min((usableW − (c−1)·gap)/c ÷ (2/3), (usableH − (r−1)·gap)/r, `FullPageMaxCover` = 80 mm). Pick the c with the largest h (deterministic tie-break, documented). Cells are 2:3 (width = h·2/3); covers keep their aspect ratio and the shrink-to-fit rule (`Grid.place`). Grid centred horizontally; header + grid block centred vertically (header directly above the grid). Every page (including the last, partial one) uses the same grid, so covers keep one size across pages.
- `Options.Grid()` for full page returns the grid for 60 covers with a header iff `Header` is non-empty (that's what `PerPage` needs); `build` computes the real grid from the actual count.
- Header: bold, centred, UTF-8 via `addTextFonts`, one line, shrunk within limits or truncated with "…" (reuse `fitLines`) — never wider than the usable width.
- Paper applies; size preset and cut mode don't. No borders, no frame (optional rounded corners/frame left out).
- Defaults keep today's output: covers, cards, spines and stubs byte-identical.
- PDF default name: `fullpage_<paper>.pdf`.
- UI: a "Header" text input in the Print sheet tab, enabled only when the selected format `usesHeader` (`PrintFormatInfo.UsesHeader`, Wails models regenerated). Header value sent in `opts.header`.
- Delivery: branch `feat/full-page` from `b7fd51b`; forecast ~550 authored lines (≈ 50 % tests); strategy `single-pr` (user GUI-checks, then local `--no-ff` merge). Push/merge are the user's decision.

## Acceptance criteria
- 1 to 60 covers fit on one page; 61+ paginate with the header repeated on every page.
- Tests for the auto-size calculation (per paper, header on/off, 1 / few / 60 covers, cap at 80 mm, cells inside the usable area below the header), `Grid.Limit`, pagination, header fitting.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`; `wails build -tags webkit2_41` (revert 0755 modes it sets on `frontend/wailsjs/go/main/*`).

## Tasks
- [ ] T1 sheet: `fullpage` format, `UsesHeader`, `Options.Header`, `Grid.Limit`, `NewFullPageGrid`, header drawing, `buildFullPage`; table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet).
- [ ] T2 app + UI + docs: `PrintFormatInfo.UsesHeader`, export name test, Header input in Print tab, Wails models, README Print sheet "Full page" subsection and feature lists. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/full-page` from `b7fd51b`. RDD on (global).

## Next step
T1.
