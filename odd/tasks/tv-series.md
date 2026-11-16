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
- [ ] T1 Providers: TMDB TV mode + IMDb TV kinds, fixtures, tests. Route: delegated writer (2+ non-trivial files).
- [ ] T2 Catalog `tv` registration, CLI usage text, frontend tip/example, README, tests. Route: same delegated writer, separate commit.
- [ ] T3 Live CLI smoke (keyless) + build. Route: inline.

## Progress / evidence

## Next step
T1.
