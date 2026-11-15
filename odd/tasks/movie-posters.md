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
- [x] T1 `internal/web` throttle (Retry-After/backoff, shared host block, spacing, OnWait). Route: delegated writer. Check: `go test ./internal/web/`.
- [x] T2 `internal/settings` + App `GetSettings/SaveSettings` + CLI flag/env. Route: delegated writer. Check: `go test ./internal/settings/ ./cmd/... .`.
- [x] T1a S1 review follow-ups (settings fsync; throttle floor delay for a past/zero Retry-After + edge tests). Route: delegated writer. Check: `go test ./internal/web/ ./internal/settings/`.
- [x] T3 TMDB provider + fixtures. Route: delegated writer. Check: `go test ./internal/providers/`.
- [x] T4 IMDb suggestion provider + fixtures. Route: delegated writer. Check: `go test ./internal/providers/`.
- [ ] T3a S2 review follow-ups (TMDB: blank-title guard; retry without year when the year-filtered results have no posters). Route: delegated writer. Check: `go test ./internal/providers/`.
- [ ] T5 Wikidata→iTunes provider + fixtures. Route: delegated writer. Check: `go test ./internal/providers/`.
- [ ] T6 `MediaType.LocalEditions` gate + `movies` catalog registration. Route: delegated writer. Check: `go test ./...`.
- [ ] T7 Frontend: Settings dialog, TMDB notice, wait status, bindings. Route: delegated writer. Check: `wails build -tags webkit2_41`, manual GUI run.
- [ ] T8 README + live end-to-end smoke test. Route: inline. Check: CLI live run, GUI print sheet.

## Progress / evidence
- 2026-11-15: plan approved; branch created. Doc commit `d92a32a`: RDD low risk, approved and acknowledged.
- T1 `697bba0` feat(web): shared throttle. Route: delegated writer (2+ non-trivial files). RED: retry/shared-block/spacing/ctx tests failed against a pass-through stub. Checks: `go test -race -count=3 ./internal/web/` ok; `go test -tags webkit2_41 ./...` ok (14 packages); `go vet` ok; `gofmt -l` clean.
- T2 `9cecaa4` feat(settings): TMDB key in settings file + App Get/SaveSettings + CLI `-tmdb-key`/`TMDB_API_KEY`. Route: same writer, separate commit. RED: undefined Save/Load/resolveTMDBKey/loadSettings. Parent spot check: `go test ./internal/web/ ./internal/settings/ ./cmd/...` 44 passed.
- S1 review (range d92a32a..9cecaa4, 1161 lines, medium): consent granted; lens review-reliability approved; acknowledged (authority burned). Advisory, non-blocking: (a) settings Save has no fsync before rename; (b) the throttle lacks tests for a past/zero Retry-After (it retries back-to-back), negative MaxRetries, and an unparseable Retry-After.
- Deviations accepted: a request that can't be retried doesn't block its host after a 429; giving up (retries used up or Retry-After > 60s) doesn't block the host; the backoff cap is MaxWait (60s) and never reached with 3 retries.
- Follow-ups: T1a (S2) adds fsync, a minimum delay for a past/zero Retry-After, and edge tests. T6 makes the CLI `run` tests hermetic: they currently read the real settings file through resolveTMDBKey.

- T1a `c8cfab3` fix(web): wait the backoff step on a past/zero Retry-After; fsync settings. RED: the past-date and `Retry-After: 0` subtests failed (zero-delay retry). The fsync change was checked structurally only (it can't be observed in a unit test).
- T3+T4 `b9b0603` feat(providers): TMDB + IMDb. These share one commit because the tests share `movies_test.go`. Route: delegated writer. RED: build failure on undefined TMDB/IMDb symbols. Checks: `go test -tags webkit2_41 ./...` ok (15 packages); parent spot check `go test ./internal/providers/ ./internal/web/` 58 passed. Extras: `ErrNoTMDBKey`, `httpStatusError` (errors.As on status).
- A hook-issued review of the whole range main..HEAD was granted, but START refused it (`invalid_request`, untracked files while the writer was running; not started). That candidate went stale after the commits.
- S2 review (range 9cecaa4..b9b0603, 820 lines, medium): consent granted; lens review-reliability approved; acknowledged (authority burned). Advisory → T3a.

## Next step
S3: T3a, T5 Wikidata→iTunes, T6 LocalEditions gate + catalog movies + hermetic CLI tests.
