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
- **Steam store search** (`https://store.steampowered.com/api/storesearch/?term=…&l=english&cc=US`): no key. It is undocumented, like the IMDb fallback for movies. Portrait art is at `https://cdn.cloudflare.steamstatic.com/steam/apps/{appid}/library_600x900_2x.jpg` (600 × 900, checked live) or `library_600x900.jpg` (300 × 450). It covers PC/Steam games only, and the search has no release date. The Steam Web API terms (https://steamcommunity.com/dev/apiterms) don't name store endpoints, grant no image rights and forbid implying Valve endorsement. Use is personal.
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
- [x] T1 Providers: IGDB (token cache, search, ranking), Steam, throttle entry, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [x] T2 Settings, catalog `games`, CLI flags and env, Settings dialog and tip, frontend example, README. Route: the same delegated writer, in a separate commit.
- [x] T3 Live CLI smoke test (keyless) and build. Route: inline.

## Progress
- Plan commit `863b0ce`. RDD on (global); the plan commit was reviewed as low risk and acknowledged.
- T1 `0880db6` feat(providers): IGDB and Steam game cover search. Route: delegated writer (2+ non-trivial files). 13 files, +1210 / −8 (567 lines of tests, 8 fixtures).
  - The IGDB token is cached per (ID, Secret) pair and renewed 60 s before expiry, under a mutex. A 401 drops the token and retries once. Missing credentials return `ErrNoIGDBCredentials` with no HTTP call.
  - Deviation: the token request sends the ID and secret in the form body, not the URL, so the secret stays out of URLs and errors (`redactKey`, with a test).
  - Throttle: a 429 on a POST used to pass straight through. Requests with a replayable body (`GetBody`) are now retried with the usual backoff; one-shot bodies still pass through. `api.igdb.com` is spaced at 250 ms.
  - `rankGames`: exact full-title matches come first, then `match.Rank`. Without it, "Hollow Knight: Silksong" tied with "Hollow Knight", since `match.Rank` compares the part before the colon.
  - Steam year tie-break is implemented: `appdetails` release dates for up to 3 title-tied results. A failed lookup keeps the order.
  - RED: build failed (`undefined: IGDB`, `NewIGDB`, `Steam`…). `TestThrottleRetriesReplayableBodies` and the `api.igdb.com` interval failed. `TestSteamSearch` ranked Silksong first. GREEN: all pass.
- T2 `269ba6d` feat(catalog): games media type. 14 files, +543 / −72. Settings fields, `catalog.Keys.IGDB`, CLI flags and env (resolved lazily with `sync.OnceValue`, so the settings file is read only when a key is needed), Settings dialog fields and link, the generalized `#key-tip`, README.
  - RED: `unknown field IGDBClientID`, `unknown field IGDB` in `Keys`. GREEN: 737 passed.
- Writer verification: `go test ./...` 737 passed, `go vet` clean, `gofmt` empty, `tsc` clean, `wails build` ok (modes restored, `build/bin/cover-downloader` rebuilt). Parent spot check: `go test ./internal/providers/ ./internal/web/ ./cmd/...` 185 passed.
- T3 live CLI (keyless, empty config). Route: inline. 5 of 6 OK from Steam, all at 600 × 900: Hollow Knight, Stardew Valley, The Witcher 3, Celeste, Prey 2017. Zelda: Breath of the Wild fails as expected (not on Steam; the error names the missing IGDB credentials). IGDB is not tested live (no credentials).
  - Fix `3061b9b`: Steam `_2x` art is 600 × 900, not 1200 × 1800 (the plain one is 300 × 450). The code comment and README were corrected.
- Review `863b0ce..51c9418` (28 files, 1857 lines, medium, `slice_budget_reached`): consent granted. The review-reliability lens approved it, and it was acknowledged (authority burned). Two advisory findings:
  - R3-001 WARNING: any 4xx from the token endpoint, 429 included, was reported as "Twitch rejected the IGDB credentials". Fixed.
  - R3-002 SUGGESTION: the CLI credential tests blocked on `<-got` and would hang if run exited early. Fixed.
  - The Stop hook's selectorless STATUS proposed the whole branch `f74d951..HEAD`. It was not started: that range is this slice plus the plan commit, which was already acknowledged.
- Fix `7517b31` fix(providers): only 400, 401 and 403 from the token endpoint mean bad credentials; a 429 or 5xx is reported as a token error. The CLI tests use a non-blocking `received` helper.
  - RED: `TestIGDBRejectedCredentials/token_endpoint_answers_429` failed ("Twitch rejected the IGDB credentials (HTTP 429)"). GREEN: `go test ./...` 739 passed, `go vet` clean, `gofmt` empty.
  - Assess `51c9418..7517b31`: medium, 38 lines, `under_budget`, so no review is due.
  - `wails build` was rerun with modes restored, and `build/bin/cover-downloader` was rebuilt.

- Manual GUI check passed (user, 2026-11-16). Merged to main (`be6c483`) and pushed.

## Next step
Done.
