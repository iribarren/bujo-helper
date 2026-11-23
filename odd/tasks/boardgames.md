# Feature: board games (roadmap item 24)

## Objective
A new media type **Board games** (id `boardgames`), input `Title, Year` (year optional), that downloads box art from BoardGameGeek (BGG) through the XML API2 with a token built into release builds.

## Problem / Why
Board game trackers exist among hobby journalers. BGG is the only practical source, and since 2025 its XML API requires a registered application and a bearer token.

## Licensing (checked 2026-11-23)
- Sources: [Using the XML API](https://web.archive.org/web/20261113172631/https://boardgamegeek.com/using_the_xml_api) (version 2025-07-02), [XML API Terms of Use](https://web.archive.org/web/20261021002430/https://boardgamegeek.com/wiki/page/XML_API_Terms_of_Use), [BGG XML API2](https://web.archive.org/web/20261110211632/https://boardgamegeek.com/wiki/page/BGG_XML_API2). BGG answers 403 to scripted page reads, so the Wayback copies were read.
- `https://boardgamegeek.com/xmlapi2/search?...` without a token answers `401 Unauthorized. See https://boardgamegeek.com/using_the_xml_api` (curl, 2026-11-23).
- Registration at boardgamegeek.com/applications; manual approval "may be a week or more". Non-commercial licence, free. Header `Authorization: Bearer <token>`, host `boardgamegeek.com` without `www`, on every request.
- An app for end users ships its developer's token; BGG says not to ask non-programmer end users for their own token. Requests should be few; client-side traffic risks suspension.
- Terms: strictly non-commercial; credit BoardGameGeek by name in all uses; "Powered by BGG" logo linked to BGG in public-facing uses; no modification of the data; no AI/LLM training; revocable at any time.
- Rate limit: none published; BGG answers 500/503 when throttling; "a 5-second wait between requests seems to suffice".
- Box art is copyrighted by its publishers: personal use only, like book covers.
- Logo: official `powered-by-bgg-rgb.svg` and `powered-by-bgg-reversed-rgb.svg` from the BGG logo folder linked from the terms (Google Drive, Color/SVG). Checked: plain SVG, no scripts or links.

## Decisions
- User decision 2026-11-23: **built-in token**. The release build injects it from the CI secret `BGG_TOKEN` with `-ldflags -X`; it is never committed. A token in Settings (or `-bgg-token` / `BGG_TOKEN` for the CLI) overrides it. With neither, the provider returns a `media.Skipped` error naming the missing token.
- Id `boardgames`, label "Board games" / "Juegos de mesa", CreatorLabel "Year", registered after `products`. One provider, `BoardGameGeek`.
- Lookup: `search?query=<title>&type=boardgame&exact=1`, then without `exact` when empty; only `type="boardgame"` items (no expansions). Candidates scored by title (primary name, then alternate names) and year; up to 20 best fetched in one `thing?id=...&stats=1` call; ties broken by number of ratings (`usersrated`); the `<image>` of the best candidate that has one.
- Requests spaced 5 s apart; 401/403 reported as a rejected token; 500/503/429 reported as BGG busy.
- Credit sidecar for every download: game and year, "Image: BoardGameGeek", the game page URL, and "Box art © its publisher; personal use only".
- "Powered by BGG" logo (linked to https://boardgamegeek.com) shown in the Download tab while Board games is selected, and in Settings → About next to the data sources; light and reversed variants follow the theme.

## Scope / constraints
- In: provider, fixtures and tests, catalog/app/settings/CLI wiring, key tip, Settings field, logo, release workflow injection, README and ROADMAP.
- Out: collections, expansions, accessories, BGG ratings or other metadata on prints, any other board game source.
- Fixtures are hand-written from the documented XML format (no token available yet); a live smoke test waits for BGG to approve the app.

## Acceptance criteria
- Provider tests with fixtures: exact match, non-exact fallback, year preference, expansion ignored, tie broken by ratings, no image → next candidate, no match, missing token skipped, 401 and 503 messages, bearer header and no-`www` host.
- The type appears in the media type grid, sample CSV, key tip (hidden when a token is built in or saved) and CLI.
- README Sources documents access, licence, credit, token setup and the build-time injection.

## Checks
- `go test ./...`, `go vet ./...`, `cd frontend && npx vitest run && npx tsc --noEmit`, `wails build` (Linux, `-tags webkit2_41`) compiles.

## Delivery
- Branch `feat/board-games` from `fdb2556`. Forecast ~1,100 authored lines (≈50 % tests, fixtures and the logo excluded). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `fdb2556`.

## Tasks
- [x] T1 Provider `BoardGameGeek` (Go): `internal/buildinfo` token var, XML search + thing, scoring, credit, bearer header, UA + 5 s throttle, skip without token; catalog registration + `Keys.BGG`; fixtures and tests. Route: delegated writer (writer trigger: provider + web + catalog + tests).
- [ ] T2 Wiring: settings field, app token fallback + `HasBuiltInBGGToken` binding, CLI flag/env, app/catalog/CLI tests; frontend example, key tip, Settings input, EN/ES strings, logo in Download and About. Route: delegated writer (2+ files).
- [ ] T3 Build + docs: release workflow `-ldflags -X` from `BGG_TOKEN`, README (Features, Sources, token setup, Building, Rate limits, CSV), ROADMAP item 24 notes, full checks. Route: delegated writer + inline checks.

## Progress
- 2026-11-23: licensing check done, user chose the built-in token; codebase mapped (explorer).
- 2026-11-23: T1 done (route: delegated writer). `internal/buildinfo.BGGToken`, provider `BoardGameGeek` (`internal/providers/bgg.go`, 6 hand-written XML fixtures in `testdata/bgg/`), `web.SetBGGHeaders` (contact UA + `Authorization: Bearer`), `boardgamegeek.com` 5 s throttle, `catalog.Keys.BGG` + `boardgames` type (samples Catan 1995, Wingspan 2019, Azul 2017), app/CLI pass the built-in token. Scoring: title (0 same, 1 prefix, 2 contains, 3 other; best of primary and alternate names), then year (0 exact or none asked, 1 off by one, 2 unknown, 3 other), then `usersrated`; up to 3 candidates with an image. RED: `go test ./internal/providers/ -run BGG` build failed (undefined `BoardGameGeek`, `ErrNoBGGToken`, `bggCredit`); also `internal/web` (undefined `SetBGGHeaders`) and catalog (unknown field `BGG`), CLI usage assertions failed. GREEN: `go build ./...` ok, `go vet ./...` ok, `go test ./...` all ok, `gofmt -l internal app.go cmd` empty.

## Next step
T2.
