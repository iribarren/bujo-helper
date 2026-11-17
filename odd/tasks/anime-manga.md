# Feature: anime-manga (roadmap item 19)

## Objective
Add **Anime** and **Manga** media types (input `Title, Year`) with cover art from the AniList GraphQL API (no key).

## Problem / Why
Anime and manga trackers are a common bujo spread among younger users (an assumption; the roadmap says no search result confirmed it).

## Licensing (read 2026-11-17)
- Source: the AniList API docs mirror at https://anilist.gitbook.io/anilist-apiv2-docs/. docs.anilist.co returned 403 to the fetcher.
  - Terms: "Free for non-commercial usage." Commercial use is free below $150 of revenue a month. "Using the AniList API as a backup or data storage service is strictly prohibited." "Hoarding" or mass collection of data is prohibited. An app named with "AniList" must state it is unofficial.
  - Auth: none needed for public reads ("Get anime and manga data" is listed under things you can do without authentication).
  - Rate limit: 90 requests/min normally, and a degraded **30/min** is in force now. A live response confirmed `x-ratelimit-limit: 30`. Going over gives a one-minute timeout: a 429 with `Retry-After` and `X-RateLimit-Reset`.
- Cover images are copyrighted, the same as book covers: personal use only. Downloads happen one at a time, only for titles the user asks for, so this is not hoarding.
- Live check (2026-11-17): endpoint `POST https://graphql.anilist.co`. `Page(perPage){media(search, type, sort: SEARCH_MATCH, isAdult: false)}` returns `title{romaji english native}`, `synonyms`, `startDate{year}`, `seasonYear`, `coverImage{extraLarge large color}` and `isAdult`. `extraLarge` is 460 × 657 (Berserk manga), close to 2:3.

## Decision (2026-11-17)
**Two media types**, `anime` and `manga`, instead of one type with a toggle. Every input row is `Title, Year`, and nothing else in the app has a per-item toggle. Both use one provider with a `Type` field (`ANIME` / `MANGA`), the same way TMDB has `TV`.

## Scope / constraints
- `anime`: label "Anime", creator label "Year", date label "Watched on". `manga`: label "Manga", creator label "Year", date label "Read on". Files are named `Title - Year.ext`. Each type has 5 samples.
- Provider `anilist` (`internal/providers/anilist.go`): one GraphQL POST per search with `perPage` 10, `sort: SEARCH_MATCH` and `isAdult: false`.
  - Candidate title: English if present, else romaji. Year: `startDate.year`, else `seasonYear`. ImageURLs: `extraLarge`, then `large`. Results with no cover are skipped.
  - Matching uses romaji, English, native and synonyms. An exact normalized match on any of them ranks first, then `match.Rank` on the best-matching title, with AniList's order kept on ties. The year (item.Creator) breaks ties, as for TV.
  - Errors: no results gives `ErrNotFound`. A GraphQL `errors` array in a 200 is an error. 429s are handled by the throttle.
- Throttle: `graphql.anilist.co` spaced at 2 s (30/min). Check that a 429 on the POST is retried (replayable body) and whether `Retry-After` is honoured.
- Catalog registration covers the selector, sample CSV, placeholder example and CLI `-type anime|manga`.
- Covers are about 2:3, so the print sheet doesn't change.
- **Defaults keep today's output:** the other media types behave the same.
- README: Sources (anime and manga, AniList terms and rate limit, copyright note) and the CLI types.
- Delivery: branch `feat/anime-manga` from `1095479`. The forecast is about 800 authored lines (about 55 % tests and fixtures), so the strategy is `ask-on-risk`. The user GUI-checks, then a local `--no-ff` merge. Push and merge are the user's decision.

## Acceptance criteria
- Provider tests with recorded fixtures: anime and manga searches, matching on a romaji or synonym title, the year tie-break, no results, a GraphQL error, and the request body (type, search and isAdult false).
- A well-known title downloads live for each type (for example `Frieren, 2023` anime and `Berserk, 1989` manga).
- The throttle spaces AniList requests, and a 429 is retried.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`. If the bindings change, run `wails build -tags webkit2_41`, restore the 0644 modes on `frontend/wailsjs/go/main/*` and `models.ts`, and rebuild `build/bin/` before the GUI check.

## Tasks
- [x] T1 Provider: AniList (GraphQL search, ranking, year tie-break), throttle entry, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [x] T2 Catalog `anime` and `manga`, CLI, frontend example, README. Route: the same delegated writer, in a separate commit.
- [ ] T3 Live CLI smoke test and build. Route: inline.

## Progress
- Plan written.
- T1 (route: delegated writer, 2+ non-trivial files) — commit 335b0f1 `feat(providers): AniList anime and manga search`. 8 files, +962/−2 (provider 226, tests 342, fixtures 387, throttle 9).
  - Design: `AniList{Client, BaseURL, Type}`, `NewAniList(client, type)` plus `NewAniListAnime` / `NewAniListManga`; `isAdult` is sent as a GraphQL variable (`false`). Ranking reuses Steam's `rankGames` (exact first, then `match.Rank`, stable) on each result's best-matching title (`bestTitle`: an exact romaji/English/native/synonym match, else the top `match.Rank` title). Only a 4-digit `item.Creator` is passed as the tie-break year.
  - Fixtures recorded live 2026-11-17 (4 requests, `x-ratelimit-limit: 30`) and trimmed: `anilist_frieren_anime.json`, `anilist_berserk_anime.json`, `anilist_berserk_manga.json`, `anilist_shingeki_anime.json`.
  - RED: `go test ./internal/providers/ -run AniList` failed to build (`undefined: AniList`, `NewAniListAnime`, `NewAniListManga`).
  - GREEN: 24 AniList tests and subtests pass (search anime/manga with request-body checks, romaji/native/synonym/English matching, year tie-break, cover/year fallbacks, NotFound, GraphQL error, 500, bad JSON, 429 retried with the same body); `go test -tags webkit2_41 ./...` 1239 passed.
  - Throttle finding: `graphql.anilist.co` spaced 2 s. `postJSON` builds the request from a `strings.Reader`, so `http.NewRequest` sets `GetBody` and the throttle retries a 429 POST with the same body. `Retry-After` (seconds or HTTP-date) is honoured up to `MaxWait` (60 s): AniList's one-minute block (`Retry-After: 60`) is waited out and retried, and a longer one returns the 429 at once (an error, not NotFound).
  - Deviation: one year rule shared with games: a year-matching main-title hit (e.g. "Berserk: The Golden Age Arc II", 2012, for `Berserk, 2012`) can outrank a non-year exact match; without a year, exact matches stay first.
- T2 (route: same delegated writer) — commit 79dddc9 `feat(catalog): anime and manga media types`. 8 files, +112/−20.
  - `anime` (Anime, Year, Watched on) and `manga` (Manga, Year, Read on) registered after `games`; 5 samples each. CLI usage/doc comment, frontend placeholder examples (`Frieren, 2023`, `Berserk, 1989`), README (features, Sources › Anime and Manga, rate limits, CSV, credits, CLI examples and `-type`, layout).
  - RED: `TestDefault`, `TestDefaultAppMoviesUseTheSavedTMDBKey`, `TestUsageListsMediaTypes` failed (old ID lists/usage); vitest 2 new placeholder tests failed.
  - GREEN: `go test -tags webkit2_41 ./...` 1241 passed in 15 packages; `go vet` clean; `gofmt -l .` empty; `npx tsc --noEmit` clean; `npx vitest run` 56 passed.
  - Not changed: `frontend/src/i18n/{en,es}.ts` (outside the edit surface). The UI falls back to Go's English labels for anime/manga, so the Spanish UI shows "Anime"/"Manga" (same word) but "Watched on"/"Read on" in English and the generic sample-CSV button text. No binding changes; `wails build` not run.

## Next step
Follow-up decision: add `media/creator/date/sample` i18n keys for anime and manga in `en.ts`/`es.ts`. Then T3 (live CLI smoke test and build).
