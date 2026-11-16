# Feature: tv-series (roadmap item 5)

## Objective
Add a **TV series** media type: input `Title, Year` (first air year optional), posters from TMDB TV search with a keyless IMDb fallback.

## Problem / Why
TV trackers are as common as movie trackers. TMDB covers TV, so the existing API key is reused.

## Scope / constraints
- Media type `tv`, label "TV series", creator label "Year", date label "Watched on". Files are `Title - Year.ext`.
- Sources in order: TMDB `/search/tv` (key needed, localized posters, `first_air_date_year` filter with a retry without it) → IMDb suggestion (keyless; kinds `tvSeries`, `tvMiniSeries`).
- No Wikidata → iTunes fallback: it links Apple TV *movie* IDs only.
- Reuse the movie providers (a TV mode on TMDB and IMDb), not copies.
- Shown in the media type selector (from the registry), sample CSV, TMDB key tip, placeholder example, CLI `-type tv`.
- Licensing: same TMDB API and terms as movies (attribution already in the UI/README); IMDb suggestion endpoint is the same unofficial keyless one used for movies. README records it.
- Delivery: `ask-on-risk`; forecast ~350–450 authored lines, one branch `feat/tv-series`. Push/merge are the user's decision.

## Acceptance criteria
- `go test -tags webkit2_41 ./...` and `go vet` pass; gofmt clean.
- Provider tests with recorded fixtures for TMDB TV and IMDb TV.
- A well-known show (e.g. `Breaking Bad, 2008`) downloads a poster with no key (IMDb) via the CLI; with a key via TMDB when one is available.
- Movie and book behaviour unchanged.
- README Sources section updated.

## Tasks
- [x] T1 Providers: TMDB TV mode + IMDb TV kinds, fixtures, tests. Route: delegated writer (2+ non-trivial files).
- [x] T2 Catalog `tv` registration, CLI usage text, frontend tip/example, README, tests. Route: same delegated writer, separate commit.
- [x] T3 Live CLI smoke (keyless) + build. Route: inline.

## Progress / evidence
- T1 (route: delegated writer, 2+ non-trivial files) — commit 0ed8b33 `feat(providers): TMDB and IMDb TV series search`.
  - Design: `TMDB.TV` / `NewTMDBTV` (`/search/tv`, `first_air_date_year`, no `region`, decodes `name`/`first_air_date`); `IMDb.TV` / `NewIMDbTV` (kinds `tvSeries`, `tvMiniSeries`; not-found says "no TV series results with a poster"). Movie mode unchanged.
  - RED: `go test ./internal/providers/` failed to build (`undefined: NewTMDBTV`, `undefined: NewIMDbTV`).
  - GREEN: providers package 54 tests pass (9 new in `tv_test.go`, fixtures `tmdb_search_tv.json`, `imdb_suggestion_tv.json`); existing movie tests unchanged and passing.
- T2 (route: same delegated writer) — commit 33f8eab `feat(catalog): TV series media type`.
  - RED: `TestDefault`, `TestDefaultPassesTheTMDBKey`, `TestDefaultAppMoviesUseTheSavedTMDBKey`, `TestUsageListsMediaTypes` failed (ID list `books, movies`, usage `books|movies`).
  - GREEN: `go test -tags webkit2_41 ./...` 401 passed in 14 packages; `go vet -tags webkit2_41 ./...` clean; `gofmt -l .` clean; `npx tsc --noEmit` clean.
  - CLI usage now derives `-type books|movies|tv` from the registry. Frontend: `tv` placeholder example, TMDB tip for movies and TV. README Sources, licensing note, CSV, usage, `-type` flag, layout updated.

- T3 (route: inline). Parent spot check `go test ./internal/providers/ ./internal/catalog/` 69 passed. Live keyless CLI (empty config, no `TMDB_API_KEY`): 5/5 OK via IMDb: Breaking Bad 2000x3000, Dark 1382x2048, The Office 1280x1922, Stranger Things 510x755, La casa de papel 1080x721 (landscape: IMDb's primary image for that title is a still, not a poster). `wails build -tags webkit2_41` ok (generated bindings mode-only change restored).
  - Known limit: the keyless IMDb fallback can return a landscape still or a small image; TMDB (with a key) gives posters. Landscape images print shrink-to-fit in the 2:3 cell until the Non-2:3 images convention lands (items 12/14).

## Next step
User: GUI check (TV series in the selector, tip, sample CSV, print sheet), live TMDB path with a key; then merge/push.
