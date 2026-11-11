# Feature: media-cover-downloader (Go + Wails desktop app)

## Objective
Cross-platform (Windows + Linux) desktop app that batch-downloads cover images.
Books now; movies, music and games later, added as new providers + registry entries with no UI changes.

## Problem / Why
The Python CLI (`legacy/cover_downloader.py`) downloads one cover per run and is awkward to distribute.
The user wants a UI with:
- a multi-line `Title, Author` textbox
- CSV import, which replaces the textbox content

They chose Go + Wails: a single executable per OS and an HTML UI suited to image-heavy screens.

## Scope / constraints
- Providers to port from legacy: Apple Books (iTunes Search), Open Library, Amazon (scrape + bot-check detection + audiobook→print ASIN images).
- Ubuntu 26.04 dev host: build tag `webkit2_41` is required. Windows is cross-compiled from Linux; the user tests it on Windows.
- Out of scope: other media types, previews, saved settings, installers.
- Plan: `~/.claude/plans/let-s-make-it-easier-vivid-pearl.md`
- Delivery: local git only (no remote), branch `feat/wails-app`, one Conventional Commit per task. Strategy `ask-on-risk`; forecast ~1,500 authored lines.

## Acceptance criteria
- `go test ./...` passes.
- The CLI downloads real covers, and one failing item doesn't stop the batch.
- The GUI imports CSV (comma / semicolon, UTF-8 BOM / cp1252), downloads with live progress, and supports cancel and open folder.
- Linux and Windows binaries build.

## Tasks
- [x] T0 Prerequisites installed (Go, Wails CLI, GTK/WebKit dev headers, pkg-config). Run by the user (sudo).
- [x] T1 Scaffold Wails vanilla-ts, `.gitignore`, legacy script moved. Check: `wails build -tags webkit2_41`.
- [x] T2 `internal/media` + `internal/batch` (lines, CSV). Check: table-driven `go test`.
- [x] T3 `internal/providers` + ranking (Apple, Open Library, Amazon). Check: `httptest` fixtures.
- [x] T4 `internal/downloader` (fallback, validation, naming/dedupe, workers, cancel, events). Check: `httptest` + temp dir.
- [x] T5 `cmd/covers` CLI. Check: live smoke test.
- [x] T6 Frontend UI + bindings + events. Check: manual GUI run.
- [x] T7 Linux + Windows builds, README. Check: both binaries produced.
- [x] T8 Spanish editions groundwork:
  - `internal/locale` (country → OL language + ISBN prefixes)
  - `internal/isbn`
  - `Candidate.ISBN` filled by Apple (artwork URL), Amazon (ASIN ISBN-10) and Open Library
  - Check: table tests
- [x] T9 Prefer local editions:
  - downloader two-pass (local-ISBN pass first, then silent fallback; one search per provider per item)
  - Open Library `language:<lang>` edition search with Amazon CDN high-res by ISBN-10
  - default country `es` (app, UI, CLI)
  - README
  - Check: `go test`, live smoke (Dune / Sapiens / El nombre del viento → Spanish editions; `-country us` Dune → English)
- [x] T10 `internal/sheet`: A4 layout (50 mm covers, 5×5, gap 4, shrink-to-fit wide covers, 0.2 mm grey cut border) + PDF build with go-pdf/fpdf (300 dpi downscale). Check: layout + PDF tests.
- [x] T11 `internal/gallery` (List, allowlisted thumbnail handler) + app bindings (ListCovers, ExportPDF via save dialog, OpenFile) + main.go AssetServer.Handler. Check: `go test`.
- [x] T12 Frontend Download | Print sheet tabs, `print.ts` (folder preview grid, selection, export, Open PDF), README. Check: npm build, real 30-cover PDF (pdfinfo 2 pages A4, rendered page viewed), wails builds.

## Routes
- T0: user (needs sudo).
- T8–T9: one delegated writer (2+ non-trivial files).
- T10–T12: one delegated writer (new packages + frontend). Downloader code must not be modified (user request).
- T1–T7: delegated writer per task (Go + frontend, 2+ non-trivial files). The parent verifies.

## Progress / evidence
- 2026-11-11: `git init`. Python script moved to `legacy/`. Waiting on T0 (`sudo -n` unavailable).
- T0 done. Go 1.26.8 and Wails v2.17.1 installed. `wails doctor` reports libwebkit missing only because it looks for 4.0; 4.1-dev is present (pkg-config 2.52.6).
- T1 done (route: inline, generated scaffold). `wails build -tags webkit2_41` gives an 8.3M binary, and the window stayed up for 5s. Commit 60ce44b. Review: none. It is the root commit (generated scaffold, nothing to diff against); `main` is set to 60ce44b as the first review boundary.

- T2–T5 done (route: one delegated writer; 2+ non-trivial files). Commits f4dacee (core), 7c2873e (providers), 013e1ce (downloader), 897c856 (CLI).
  - The parent re-ran `go test ./internal/... ./cmd/...`: 59 passed in 8 packages. RED observed per task (compile failures).
  - Live smoke: Dune OK (apple, 1466x2625); bogus title FAIL with exit 1; "Cien años de soledad" with country es OK (3000x5114). OL and Amazon were checked live by the writer, nothing committed.
  - Writer deviations accepted: `internal/web` helper package; `MaxCandidates` default 1 (only the best match per provider); Run errors only on setup.
- Review: the T2–T5 range (base main) was assessed medium, `slice_budget_reached`. `.atl/` added to `.gitignore` (commit) to clear the untracked-files prompt.
  - The user granted review. One lens (reliability) ran and approved it; acknowledged with authority burned (lineage review-af11466919e07482). The review boundary is now the T2–T5 range head.
  - 3 non-blocking advisory findings:
    - empty main title counted as a prefix match (WARNING) → fixed
    - truncated image saved at the body cap (WARNING) → fixed
    - fetch failure reason lost (SUGGESTION) → assigned to T6
  - Fix commit 3c16317: RED observed for both new tests, then GREEN, 62 tests pass.
- T6+T7 done (route: one delegated writer).
  - Commits 2691a05 (UI; downloader failure reasons RED→GREEN), f960aab (README + sample_books.csv).
  - The parent re-ran `go test -tags webkit2_41 ./...`: 80 passed in 9 packages.
  - Linux binary 10.0M, launched 6s with no crash. Windows `.exe` 11.2M, cross-compiled without cgo. Screenshot shows the layout renders.
  - Pending manual checks (user): a real GUI download and cancel, CSV import variants, Browse, Open folder, the `.exe` on Windows.

- Before review: removed the stray executable bit from the generated Wails bindings (commit "chore(ui): drop executable bit…").
- Review of range 8607534..HEAD assessed high (process launch in app.go for Open folder). The user granted review.
  - 4 lenses ran concurrently (risk, resilience, readability, reliability) and approved it; acknowledged with authority burned (lineage review-1e74e06c26b78a69).
  - Advisory findings: 1 WARNING, 4 SUGGESTIONs.
    - WARNING R3-csv-roundtrip-comma, fixed in 2a37407: imported creators with commas ("Last, First") became "First Last" so the line parse keeps the title. RED observed, then GREEN; 81 tests pass. Assessed medium, under_budget, so it stays pending in the next slice.
    - SUGGESTIONs left as follow-ups:
      - main.ts:15-33 and main.ts:109-121 readability
      - web.go:26-29 readability
      - app.go:198-209 (mkdir before the running check)

## Next step
User manual checks:
- a real GUI download and cancel
- CSV import (comma, semicolon, cp1252)
- Browse and Open folder
- the `.exe` on Windows

Follow-ups: the 4 review SUGGESTIONs; consider limiting pass-1 Amazon searches if bot checks become frequent. Delivery (merge to main) is the user's decision.

### 2026-11-11: Spanish editions request
The user wants Spanish edition covers and `es` as the default country. Plan approved (`~/.claude/plans/let-s-make-it-easier-vivid-pearl.md`).
- Research:
  - Apple es mixes languages (English Dune listed first; Sapiens has no Spanish edition).
  - Open Library `language:spa` finds the Spanish editions plus their ISBNs.
  - The Amazon CDN by ISBN-10 gives a high-res Spanish cover.
  - Todos tus libros rejected (robots.txt disallows /busquedas).
- User decision: if no Spanish edition is found, fall back silently to any edition.
- T8 d2ae204 and T9 2392b49 done (route: one delegated writer).
  - RED→GREEN observed for both. The parent re-ran the tests: 105 passed in 11 packages.
  - Live smoke with the default `es` gave Spanish editions for all 3 books, checked visually:
    - Dune: Debolsillo, 9788466342667
    - Sapiens: Debate, 9788499924748
    - El nombre del viento: 9788401352799
  - With `-country us`, Dune gave the English edition (9781101658055).
  - Known limitation: the 97884 prefix also covers Catalan editions.
- Review of range c46654b..HEAD (includes 2a37407): medium, slice_budget_reached. The user granted review.
  - One lens (reliability) approved it; acknowledged with authority burned (lineage review-8d8e944caf067321).
  - WARNING R3-local-filter-before-rank-cut, fixed by `match.Plausible`: local editions must have a title score ≤1 and a matching surname. RED showed the study guide being chosen; GREEN, 107 tests.
  - Live re-smoke: Dune, Sapiens, El nombre del viento and La sombra del viento (Planeta) all gave Spanish editions, checked visually.
  - WARNING R3-pass1-extra-amazon-searches, accepted: Amazon.es is a good source of local editions. The cost is one cached Amazon search when Apple and OL have no local edition. Documented here.

### 2026-11-11: Print sheet request
End goal: print small covers on A4 for bullet journaling. The user said the downloader works and must not be touched.
- Plan approved: a new "Print sheet" tab that previews a folder (default = the download folder), selects covers and exports a PDF. Covers are 5 cm tall, 5×5 = 25 per A4 page, with extra pages as needed.
- User decisions: wide covers shrink to fit the 35.6 mm column; thin grey cut border.
- T10 9d2e014, T11 4231975, perf 4b28140, T12 3741d3c done (route: one delegated writer). RED→GREEN for the Go parts; the frontend was checked by tsc and vite plus a real PDF.
  - The parent re-ran the tests: 137 passed in 13 packages. `git diff 86d56fb -- <downloader packages>` is empty, so the downloader is untouched.
  - Real 30-cover PDF: 2 A4 pages, 326 KB, 6.3 s.
  - The parent viewed both pages: a 5×5 grid with cut borders; page 2 has 5 covers and the square one is shrunk to fit.
  - Pending manual checks (user): tabs and keyboard, thumbnails in WebKitGTK and WebView2, Export/Open PDF on Windows, and that a printed page measures 5 cm per cover.
- Review assessment of range e5ea9f6..HEAD: high (process launch in app.go `OpenFile`).
