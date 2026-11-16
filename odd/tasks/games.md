# Feature: games (roadmap item 13)

## Objective
Add a **Games** media type (input `Title, Year`) with box art from IGDB when the user has Twitch credentials, and a keyless Steam fallback.

## Problem / Why
Game trackers exist in gaming bullet journals. The roadmap asks for a source comparison first, then the user's choice.

## Licensing and source comparison (read 2026-11-16)
- **IGDB** (https://api-docs.igdb.com/; WebFetch gets 403, curl with a browser User-Agent works):
  - "The IGDB.com API is free for non-commercial usage under the terms of the Twitch Developer Service Agreement." Commercial use needs a partnership with user-facing attribution. Caching is allowed: "we prefer if you store and serve the data to your end users."
  - Auth: a Twitch account with 2FA, an app in the Twitch Developer Console (Client Type *Confidential*, OAuth redirect `localhost`), then a Client ID and Client Secret. Token: `POST https://id.twitch.tv/oauth2/token?client_id=…&client_secret=…&grant_type=client_credentials` returns `access_token` and `expires_in` (about 64 days). Requests are `POST https://api.igdb.com/v4/<endpoint>` with the `Client-ID` and `Authorization: Bearer …` headers, and an Apicalypse body.
  - Rate limit: 4 requests/s and at most 8 open requests; a 429 when over.
  - Images: `https://images.igdb.com/igdb/image/upload/t_{size}/{image_id}.jpg`. Sizes: `cover_big` 264 × 374, `720p` 1280 × 720 Fit, `1080p` 1920 × 1080 Fit, and `_2x` for retina.
- **RAWG** (https://rawg.io/tos_api, https://rawg.io/apidocs): needs a key and an active link to RAWG on every page that uses its data. The free tier's commercial terms contradict each other. Its only image is a landscape `background_image`, with no box art. **Rejected.**
- **Steam store search** (`https://store.steampowered.com/api/storesearch/?term=…&l=english&cc=US`): no key. It is undocumented, like the IMDb fallback for movies. Portrait art is at `https://cdn.cloudflare.steamstatic.com/steam/apps/{appid}/library_600x900_2x.jpg` (1200 × 1800, checked live) or `library_600x900.jpg`. It covers PC/Steam games only, and the search has no release date. The Steam Web API terms (https://steamcommunity.com/dev/apiterms) don't name store endpoints, grant no image rights and forbid implying Valve endorsement. Use is personal.
- **User decision (2026-11-16):** IGDB first when Twitch credentials are set, then Steam (keyless).

## Scope / constraints
- Media type `games`: label "Games", creator label "Year", date label "Played on". Files are `Title - Year.ext`.
- Sources, in order:
  1. **IGDB** (`igdb`): skipped cleanly (like TMDB with no key) when the Client ID or Secret is empty. The token is fetched with client credentials and cached in memory per credential pair until shortly before `expires_in`. A 401 drops the cached token and retries once. Search with `search "<escaped title>"; fields name,first_release_date,cover.image_id; where cover != null; limit 20;`. Rank by title and year (`first_release_date` is a Unix time; use the existing `match` package). Image URLs in order: `t_1080p`, `t_720p`, `t_cover_big_2x`.
  2. **Steam** (`steam`): store search, `type == "app"` only, ranked by title. Image URLs: `library_600x900_2x.jpg`, then `library_600x900.jpg`. On a 404 the downloader already moves on to the next URL. If a year is given, the year may break title ties among at most 3 top candidates through `https://store.steampowered.com/api/appdetails?appids=…&filters=release_date`. Steam's search has no dates.
- Throttle: `api.igdb.com` is spaced at 250 ms (4/s). Check whether the throttle retries POST requests on a 429 and document what happens.
- Credentials are stored like the TMDB key:
  - `settings.Settings` gets `IGDBClientID` (`igdbClientId`) and `IGDBClientSecret` (`igdbClientSecret`), trimmed on save.
  - `catalog.Keys` gets an IGDB credential func that is read live.
  - CLI: `-igdb-id` and `-igdb-secret`, else `IGDB_CLIENT_ID` and `IGDB_CLIENT_SECRET`, else the settings file.
  - GUI: the Settings dialog gets the two fields (secret masked, with Show/Hide) and a link to the Twitch Developer Console. The key tip generalizes: games without IGDB credentials suggest adding them (Steam covers PC games only).
- Catalog registration covers the selector, sample CSV (5 samples), placeholder example and CLI `-type games`.
- Covers are about 3:4 (IGDB) or 2:3 (Steam). The existing shrink-to-fit 2:3 column handles them, so the print sheet doesn't change.
- **Defaults keep today's output:** books, movies, TV and albums behave the same.
- README: Sources (games, the licensing notes, the Twitch setup steps) and the CLI flags.
- Delivery: branch `feat/games` from `f74d951`. The forecast is about 1000 authored lines (about 55 % tests and fixtures), so the strategy is `ask-on-risk`. The user GUI-checks, then a local `--no-ff` merge. Push and merge are the user's decision.

## Acceptance criteria
- Provider tests with recorded fixtures: the IGDB token and search (including the 401 refresh and the no-credentials skip), and the Steam search with the 404 fallback.
- The media type is usable with no key (Steam), and IGDB is skipped cleanly when no credentials are set.
- The live CLI downloads a well-known PC game keyless (for example `Hollow Knight, 2017`). With credentials, IGDB serves it (only if the user provides credentials).
- README Sources is updated with the key setup steps.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` in `frontend/`. If the bindings change, run `wails build -tags webkit2_41`, restore the 0644 modes on `frontend/wailsjs/go/main/*` and `models.ts`, and rebuild `build/bin/cover-downloader` before the GUI check.

## Tasks
- [ ] T1 Providers: IGDB (token cache, search, ranking), Steam, throttle entry, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [ ] T2 Settings, catalog `games`, CLI flags and env, Settings dialog and tip, frontend example, README. Route: the same delegated writer, in a separate commit.
- [ ] T3 Live CLI smoke test (keyless) and build. Route: inline.

## Progress
- RDD on (global).

## Next step
T1.
