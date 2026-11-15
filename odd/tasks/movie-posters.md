# Feature: movie-posters (second media type)

## Objective
Download movie posters as well as book covers. Use several sources with fallback, and wait when a source rate-limits. Keep the print/PDF page and resize rules unchanged.

## Problem / Why
The user asked for movie posters "as smart as books": multiple sources, waiting on limits. The app has no rate limiting (no 429/Retry-After handling) and no settings storage. It also runs a book-only local-edition (ISBN) pass that would make wasted movie searches.

## Scope / constraints
- New media type `movies`. Input is `Title, Year` (year optional). Files are named `Title - Year.ext`.
- Sources in order: TMDB (user-entered key, saved in the settings file) → IMDb suggestion (keyless, unofficial) → Wikidata P6398 → iTunes lookup (keyless).
- Shared `web` throttle: 429/503 + Retry-After / exponential backoff, a per-host block shared by workers, optional per-host spacing, an OnWait hook.
- Settings file `os.UserConfigDir()/cover-downloader/settings.json` (0600). CLI precedence: `-tmdb-key` → `TMDB_API_KEY` → settings file.
- Print sheet unchanged. Different poster sizes are a future feature.
- TMDB attribution notice in the UI and README (TMDB terms).
- Plan: `~/.claude/plans/new-feature-be-able-sharded-leaf.md`
- Delivery: strategy `ask-on-risk` → chain strategy `feature-branch-chain` (user-chosen 2026-11-15). Forecast ~1,200–1,500 authored lines. One Conventional Commit per task.
  - Tracker branch `feat/movie-posters` (draft/no-merge PR to `main`), which holds this doc.
  - S1 `feat/movie-posters-s1-infra` (T1, T2) → targets the tracker.
  - S2 `feat/movie-posters-s2-tmdb-imdb` (T3, T4) → targets S1.
  - S3 `feat/movie-posters-s3-wikidata-catalog` (T5, T6) → targets S2.
  - S4 `feat/movie-posters-s4-ui-docs` (T7, T8) → targets S3.
  - Push and PR creation remain the user's decision.

## Acceptance criteria
- `go test ./...` and `go vet ./...` pass.
- The CLI downloads real posters for `Dune, 2021`, `Inception, 2010` and `The Matrix, 1999`, both with and without a TMDB key.
- A 429 with Retry-After makes the client wait and retry, not fail.
- Movies make no local-edition (pass 1) searches. Book behavior is unchanged.
- The GUI saves the TMDB key and keeps it across restarts, shows rate-limit waits, and prints posters on the existing sheet.

## Tasks
- [ ] T1 `internal/web` throttle (Retry-After/backoff, shared host block, spacing, OnWait). Route: delegated writer. Check: `go test ./internal/web/`.
- [ ] T2 `internal/settings` + App `GetSettings/SaveSettings` + CLI flag/env. Route: delegated writer. Check: `go test ./internal/settings/ ./cmd/... .`.
- [ ] T3 TMDB provider + fixtures. Route: delegated writer. Check: `go test ./internal/providers/`.
- [ ] T4 IMDb suggestion provider + fixtures. Route: delegated writer. Check: `go test ./internal/providers/`.
- [ ] T5 Wikidata→iTunes provider + fixtures. Route: delegated writer. Check: `go test ./internal/providers/`.
- [ ] T6 `MediaType.LocalEditions` gate + `movies` catalog registration. Route: delegated writer. Check: `go test ./...`.
- [ ] T7 Frontend: Settings dialog, TMDB notice, wait status, bindings. Route: delegated writer. Check: `wails build -tags webkit2_41`, manual GUI run.
- [ ] T8 README + live end-to-end smoke test. Route: inline. Check: CLI live run, GUI print sheet.

## Progress / evidence
- 2026-11-15: plan approved; branch created.

## Next step
T1.
