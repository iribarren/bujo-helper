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
- [ ] T1 Provider: AniList (GraphQL search, ranking, year tie-break), throttle entry, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [ ] T2 Catalog `anime` and `manga`, CLI, frontend example, README. Route: the same delegated writer, in a separate commit.
- [ ] T3 Live CLI smoke test and build. Route: inline.

## Progress
- Plan written.

## Next step
T1.
