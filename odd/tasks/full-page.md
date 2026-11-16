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
- [x] T1 sheet: `fullpage` format, `UsesHeader`, `Options.Header`, `Grid.Limit`, `NewFullPageGrid`, header drawing, `buildFullPage`; table-driven tests. Route: delegated writer (writer trigger: several non-trivial files in internal/sheet).
- [x] T2 app + UI + docs: `PrintFormatInfo.UsesHeader`, export name test, Header input in Print tab, Wails models, README Print sheet "Full page" subsection and feature lists. Route: same delegated writer, separate commit.

## Progress
- Feature doc created; branch `feat/full-page` from `b7fd51b`. RDD on (global).

- T1 done in `68d75e7` feat(sheet): add full page print format. Route: delegated writer (writer trigger: several non-trivial files in `internal/sheet`). 6 files, +730 / −31 (≈ 761 authored lines, ≈ 68 % tests: `fullpage_test.go` +470, `format_test.go` +15/−9, `app_test.go` +16/−12). Over the ~400-line heuristic because the auto-size rule, header fitting and PDF pagination each need table-driven coverage; no artificial split.
  - Files: `internal/sheet/fullpage.go` (constants, `NewFullPageGrid`, `fullPageHeaderBox`, `headerLine`, `layoutFullPageHeader`, `buildFullPage`), `layout.go` (`fullFormat`, `Format.UsesHeader`, Formats entry, `Options.Header` + `header()`, `Grid.Spacing`, `Grid.Limit`, `PerPage` cap, `cell` uses `gap()`, `Options.Grid` case, package doc), `pdf.go` (build switch, `Build` doc), `format_test.go` (uses-header column, per-page 60), `app_test.go` (`TestPrintFormats` / `TestPrintPapers` gain the fullpage entry, 60 on every paper, to keep the commit green).
  - RED: `go test -tags webkit2_41 ./internal/sheet/` → build failed: `f.UsesHeader undefined`, `unknown field Header in struct literal of type Options`, `undefined: FullPageHeaderHeight`, `undefined: FullPageGap`, `undefined: NewFullPageGrid` …
  - GREEN: `go test -tags webkit2_41 ./...` → 511 passed in 14 packages; `go vet` clean; `gofmt -l .` empty.
  - Defaults unchanged: page streams of covers, cards, spines and stubs for every paper × size × cut (135 option combinations, 33 mixed covers, uncompressed, fixed dates, via a `go test -overlay` scratch file outside the repo) are byte-identical before and after; also identical with `Header` set on those formats.
  - Design notes: `FullPageGap` 3 mm, header band `FullPageHeaderHeight` 12 mm, header bold 20 pt shrinking (0.1 pt steps, proportional) to 12 pt, then cut with "…" via `fitLines`; text `titleGrey`, centred in the band across the usable width. Tie-break: tallest cells; on a tie (1e-9), fewer empty cells, then more columns (A4: 2 → 2 × 1, 3 → 3 × 1, 4 → 2 × 2, 60 → 8 × 8 at 32.4 mm, 31 mm with header). `Grid.Spacing` (0 = `Gap`) carries the 3 mm gap; `Grid.Limit` = covers on the page (≤ 60). `Options.Grid` returns the 60-cover grid; `build` uses the grid for the actual count, same grid on every page. Fonts embedded only with a header. Header whitespace runs collapse.
  - Visual check (`pdftoppm -r 100`): A4 7 mixed covers + "Books of 2026" → 3 × 3 at 80 mm, header right above, block centred, no borders; A4 65 covers → page 1 8 × 8 (60), page 2 five covers in the same cells, header repeated; A5 long accented header → shrunk to 12 pt and cut with "…" inside the margins.

- T2 done in `3b6583f` feat(ui): offer full page format in the print sheet. Route: delegated writer. 7 files, +73 / −26.
  - Files: `app.go` (`PrintFormatInfo.UsesHeader` `json:"usesHeader"`, set from `sheet.Format`; `pdfFileName` doc example), `app_test.go` (`TestPrintFormats` fullpage `UsesHeader: true`; export names `fullpage_A4.pdf`, `fullpage_Letter.pdf` with a header, `fullpage_A5.pdf` with size/cut ignored), `frontend/index.html` (Header field `#print-header`, placeholder "Books of 2026", hidden and disabled by default; Format tooltip), `frontend/src/print.ts` (header shown/enabled only when `usesHeader`; `header` sent trimmed in `opts`, '' for other formats; file comment), `frontend/src/style.css` (`#print-header` 12em wide), `frontend/wailsjs/go/models.ts` (regenerated: `usesHeader`, `header`), README (feature line, Print sheet intro, Format step, Cover size note, file names, Cut mode note, new "Full page" subsection, `internal/sheet` line). Package doc in `layout.go` was updated in T1.
  - RED: `go test -tags webkit2_41 .` → build failed: `unknown field UsesHeader in struct literal of type PrintFormatInfo`. Export-name cases are characterisation (`pdfFileName` is already generic).
  - GREEN: `go test -tags webkit2_41 ./...` → 514 passed in 14 packages; `go vet` clean; `gofmt -l .` empty; `npx tsc --noEmit` no errors; `wails build -tags webkit2_41` built. It set 0755 on `frontend/wailsjs/go/main/App.{d.ts,js}` and `models.ts`; reverted to 0644, `git diff --summary main..HEAD` shows no mode changes.
  - Pending: manual GUI check (Header field visibility per format, export with a header).

- Parent spot check: `go test -tags webkit2_41 ./internal/sheet/ .` 313 passed; A4 7-cover render reviewed (header above grid, block centred, no borders).
- Plan commit 1251881: low risk, auto-approved, acknowledged.
- Assess 1251881..3a040a3: medium, 878 lines, `review_due` (`slice_budget_reached`). Consent granted; reliability lens approved, acknowledged (authority burned). Advisory R3 SUGGESTION (`fullpage_test.go:212-220`, `TestGridLimit` could panic instead of reporting a wrong page count) applied in `36fece7` (`t.Fatalf` on the page counts); `TestGridLimit` passes.

## Next step
User GUI check (Header field visibility per format, export with a header), then local `--no-ff` merge.
