# Feature: podcasts (roadmap item 20)

## Objective
A new media type **Podcasts** (id `podcasts`), input `Podcast, Host` (host optional), that downloads square show artwork from the iTunes Search API.

## Problem / Why
Podcast trackers sit next to reading and watching logs. The iTunes Search API the app already uses returns podcast artwork, with no key.

## Licensing (checked 2026-11-23)
- Not a new source: the iTunes Search API terms were read for item 12 (see `odd/tasks/albums.md`, https://performance-partners.apple.com/search-api). No key, about 20 calls a minute; artwork may be used "only to promote store content", the same terms as the Apple Books, Apple TV and Apple Music sources. Podcast artwork belongs to each show; downloads are for personal use, as the README already says for the other Apple sources.
- Live probe 2026-11-23: `https://itunes.apple.com/search?term=The+Daily+The+New+York+Times&media=podcast&limit=5&country=us` returns `kind: "podcast"` results with `collectionName` (show), `artistName` (publisher/host) and `artworkUrl100`; the `10000x10000bb` rewrite answers a 3000 × 3000 JPEG. Recorded as `internal/providers/testdata/apple_podcast_search.json` (fields trimmed).

## Decisions
- One source: the existing `Apple` provider with `Media: "podcast"` (`NewApplePodcasts`), title from `collectionName`, creator from `artistName`, label "Apple Podcasts". No fallback: the only keyless alternative is the show's RSS feed, which needs the iTunes lookup anyway; Podcast Index needs a key.
- Id `podcasts`, label "Podcasts", CreatorLabel "Host", DateLabel "Listened on", registered last (after `boardgames`) so existing type order is unchanged. Samples: The Daily (The New York Times), Radiolab (WNYC Studios), This American Life, Serial (Serial Productions), Hardcore History (Dan Carlin).
- Square cells already exist (item 12), so the print sheet needs no change; a test confirms square artwork gets square cells via the existing square path.

## Scope / constraints
- In: provider constructor and branch, fixture and tests, catalog registration, frontend example and strings (EN/ES), CLI usage/doc, sample CSV, README Sources and the lists that name media types, ROADMAP item 20.
- Out: episode artwork, RSS feed parsing, any other podcast source.

## Acceptance criteria
- Provider test with the recorded fixture: `media=podcast`, no `entity`, title from `collectionName`, creator from `artistName`, large artwork URL, ranking prefers the exact show and host.
- Books and albums requests stay unchanged (existing tests).
- The type appears in the media type grid, sample CSV and CLI `-type podcasts`.
- Live CLI downloads `The Daily, The New York Times` as a square image.

## Checks
- `go test ./...`, `go vet ./...`, `gofmt -l internal app.go cmd`, `cd frontend && npx tsc --noEmit && npx vitest run`.

## Delivery
- Branch `feat/podcasts` from `f1756b7`. Forecast ~250 authored lines. Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `f1756b7`.

## Tasks
- [x] T1 Provider + wiring: `NewApplePodcasts`, catalog `podcasts`, frontend example + EN/ES label strings, CLI doc/usage, tests (provider, catalog, CLI, i18n). Route: delegated writer (writer trigger: provider + catalog + frontend + CLI files).
- [x] T2 Docs + live smoke: README (Sources, Features, CSV, CLI, layout lists), ROADMAP item 20, live CLI download. Route: inline (mechanical docs) + inline smoke.

## Progress
- 2026-11-23: licensing reused from item 12; live API probed and fixture recorded; branch created.
- 2026-11-23: T1 done (route: delegated writer). `NewApplePodcasts` + `podcasts()` helper (title from `collectionName`, no ISBN, label "Apple Podcasts"); `podcasts` registered last; CLI doc, `-type` help and usage shapes (hand-written) gain "Podcast, Host"; frontend example and EN/ES strings ("Pódcasts", "Presentador", "Escuchado el"). No sample file: `App.sampleCSV` builds `title,host` from the registry. RED: `go test ./internal/providers/ -run Podcast` build failed (undefined `NewApplePodcasts`); catalog `TestDefault` failed (no `podcasts`); CLI usage test failed. GREEN: `go build ./...` ok, `go vet ./...` ok, `go test ./...` 3320 passed in 20 packages, `gofmt -l` empty, `npx tsc --noEmit` ok, `npx vitest run` 368 passed. Parent spot check: `go test ./internal/providers/ ./internal/catalog/` 453 passed.

- 2026-11-23: T1 commit `c3652fd`. Assess (base `f1756b7`, committed-only): medium, `review_due` false, `under_budget`; stays pending in the slice.
- 2026-11-23: T2 done (route: inline; mechanical docs). README: intro, Features input list, podcasts paragraph in Sources (source, input, size, square cells, no fallback, terms), CSV columns and Sample CSV list, Using the app step 1, square cells, credits page list, CLI example and `-type` row, project layout. ROADMAP item 20 decision bullet (still `[ ]` until merge). Live smoke: `go run ./cmd/covers -type podcasts -o <scratch> "The Daily, The New York Times" "Radiolab"` → 2 ok, both 3000 × 3000 JPEG from `apple` (square, so the existing square-cell rule applies). Docs-only, no runnable RED (exception: passive docs).

## Next step
User GUI check (Download: Podcasts button, Sample CSV, a download; Print covers: square cells), then the local `--no-ff` merge and ROADMAP tick.
