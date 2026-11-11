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

## Routes
- T0: user (needs sudo).
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

## Next step
Review assessment on range 8607534..HEAD, then the user checks the GUI manually on Linux and Windows.
