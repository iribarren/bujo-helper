# Feature: places (roadmap item 15)

## Objective
Add a **Places** media type (input `Place, Country`, country optional) that downloads a landmark or city photo from Wikimedia Commons via Wikidata, or a country's flag in per-item flag mode, stores a credit sidecar next to each image, and offers a credits page in the print sheet PDF.

## Problem / Why
Travel logs and "places visited" spreads use landmark pictures, postcard-style images and flags.

## Licensing check (read 2026-11-17)
- **Wikidata** (https://www.wikidata.org/wiki/Wikidata:Licensing): structured data is CC0, so labels, Q-ids and P18/P41 values need no credit. CC0 does not cover the image files.
- **Commons reuse** (https://commons.wikimedia.org/wiki/Commons:Reusing_content_outside_Wikimedia, https://commons.wikimedia.org/wiki/Commons:Credit_line): reuse "in print" is in scope. Credit the content creator (Author), not the uploader, and identify the licence ("stating or linking to the terms of the license"). Public domain: credit is not strictly required but "recommended". ShareAlike only matters for redistributed modifications. Personality rights and trademarks may apply to commercial use (not this app's use). The WMF gives no warranty on licence data, so the app stores it as given.
- **User-Agent** (https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy): required, `<client>/<version> (<contact>)`; a website is valid contact. The app's existing MusicBrainz UA already carries the public repo URL.
- **Rates** (https://www.mediawiki.org/wiki/API:Etiquette, https://wikitech.wikimedia.org/wiki/Robot_policy): API serial, below 5 requests/s; media (upload.wikimedia.org) concurrency at most 2; "Only use originals or one of our standard thumbnail sizes"; respect Retry-After on 429.
- **Thumbnails** (https://www.mediawiki.org/wiki/Common_thumbnail_sizes): standard widths 20, 40, 60, 120, 250, 330, 500, 960, 1280, 1920, 3840; other widths are rejected. Get the URL from `prop=imageinfo&iiurlwidth=…`, never build it.
- **Metadata** (https://www.mediawiki.org/wiki/API:Imageinfo, https://www.mediawiki.org/wiki/Extension:CommonsMetadata): `extmetadata` fields are HTML: `ObjectName`, `Artist`, `Credit`, `Attribution` (replaces Artist + Credit when set), `LicenseShortName`, `LicenseUrl`, `AttributionRequired` (string), `Copyrighted`.
- **Verdict:** nothing forbids the intended use. Obligations: per-file credit (author + licence + link), a contact User-Agent, standard thumbnail widths, gentle rates.

## Scope / constraints
- Media type `places`: label "Places", creator label "Country", date label "Visited on". Files are `Place - Country.ext`.
- Provider `wikimedia` (no key):
  1. Wikidata full-text search `"<place> haswbstatement:P18"` (limit 10), then `wbgetentities` (labels, descriptions, claims; `languagefallback`).
  2. When a country is given, resolve each hit's P17 (country) labels in one extra `wbgetentities` call and prefer hits in that country (also accepting a match on the description). A hit with no matching country is still allowed but ranked lower.
  3. Rank by label against the place (`match.Rank` style). The image is the hit's first P18 file.
  4. Commons `imageinfo` for that file: `iiprop=url|extmetadata|size`, `iiurlwidth=1920`. Image URLs: the 1920 thumbnail, then the same URL at 1280 (both standard widths).
  5. **Flag mode (per item):** a title `Flag of X` or `X flag` (case-insensitive) looks up country X (`haswbstatement:P41`) and uses its P41 flag file. The country column is ignored. SVG flags come back as PNG thumbnails.
  - Hits without a usable image are skipped with a clear reason (an error wrapping `media.ErrNotFound`).
- **Credit sidecar:** reuse item 14's `media.Candidate.Credit` and `<file>.credit.txt`. Content: title (ObjectName or the Wikidata label), author (Artist or Attribution, HTML stripped), credit/source, licence name and URL (or "public domain"), the Commons file page URL and the Wikidata item URL.
- **Headers and rates:** Wikimedia requests (Wikidata, Commons API, upload.wikimedia.org) send a contact User-Agent. Throttle: `commons.wikimedia.org` spaced like `www.wikidata.org` (200 ms); `upload.wikimedia.org` spaced so downloads stay gentle. The existing Wikidata → iTunes provider also switches to the contact User-Agent.
- **PDF credits page (Attribution convention, second part, deferred from item 14):** a Print sheet checkbox **Credits page** (default on). When on and at least one selected image has a credit sidecar, the PDF ends with a page (or pages) listing each credited image: a number, the file name and its credit text. Without sidecars, output is unchanged (books, movies, TV, albums, games stay byte-identical).
- **Non-2:3 images:** item 14's wide cells already place landscape photos and 3:2 flags; reuse them and add a test with a place-shaped image if one is missing.
- Catalog: selector, sample CSV (for example Eiffel Tower, Machu Picchu, Sydney Opera House, Flag of Japan, Colosseum), placeholder example, CLI `-type places`.
- README: Sources (places, flag mode, licence and credit handling, the User-Agent) and Print sheet (credits page).
- Delivery: branch `feat/places` from `19f7cd2`. Forecast about 1400 authored lines (more than half tests and fixtures). Strategy `ask-on-risk`, applied as in earlier items: one feature branch reviewed in slices; the user GUI-checks; then a local `--no-ff` merge. Push and merge are the user's decision.

## Acceptance criteria
- Provider tests with recorded fixtures: a landmark with its credit, a country flag with its credit, the country preference, a hit with no image skipped, API errors.
- Sheet tests: credits page present with sidecars, absent without them or with the box unchecked; default output unchanged.
- Live CLI: a well-known landmark and a country flag both download with a `.credit.txt`.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` and vitest in `frontend/`. If the bindings change, run `wails build -tags webkit2_41`, restore the 0644 modes on `frontend/wailsjs/go/main/*` and `models.ts`; the binary is `build/bin/bujo-helper`.

## Tasks
- [x] T1 Provider `wikimedia`: Wikidata search, country preference, flag mode, Commons imageinfo, credit text, headers, throttle, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [x] T2 Catalog `places`, CLI, frontend example, sample CSV, README Sources. Route: the same delegated writer, separate commit.
- [x] T3 PDF credits page: sheet option, sidecar reading, Print sheet checkbox, bindings, tests, README Print sheet. Route: delegated writer.
- [ ] T4 Live CLI smoke test and build. Route: inline.

## Progress
- Plan written; RDD on (global).
- T1 done in `304e85f` feat(providers): Wikidata/Commons place and flag search. 19 files, +3011/-9: `wikimedia.go` 598, `wikimedia_test.go` 508, 11 trimmed live fixtures under `testdata/wikimedia/` (about 1860 lines of JSON: Eiffel Tower, Córdoba, Japan flag), `web.go` +32/-5, `web_test.go` +37, throttle +6/+5, `wikidata.go`/`wikidata_test.go` UA switch. Authored code and tests about 1150 lines, over the 400 heuristic: one provider with its ranking, flag mode, credit and tests is one unit.
  - RED: `go test ./internal/web/ ./internal/providers/` failed to build (undefined `Wikimedia`, `NewWikimedia`, `web.ContactUserAgent`, `flagCountry`, `placeInCountry`, `htmlText`, `commonsLicense`, ...).
  - GREEN: `go test -tags webkit2_41 ./...` all 15 packages ok; `go vet` clean; `gofmt -l .` empty.
  - Deviations: Commons now serves thumbnails from `thumb.wikimedia.org` (originals from `upload.wikimedia.org`), so both hosts get the contact UA and spacing. The contact UA for image downloads is set in `web.SetBrowserHeaders` by host, without touching the downloader. Image hosts are spaced 500 ms (not 250 ms) to stay near the two-concurrent-downloads rule. Wikidata statements: the first preferred one wins, deprecated ones are skipped (Japan's P41 has the historical flag second). Flag mode returns only the best label match, maps `USA`/`UK` style aliases for ranking, and drops a leading "the". Credit fields are capped at 300 characters (some Commons Credit/Artist fields hold whole tables or galleries); Attribution replaces Artist when set.
- T2 done in `5dcc48a` feat(catalog): places media type. 9 files, +98/-16: catalog registration and samples, catalog/app/CLI tests, CLI usage, frontend example and en/es labels, README (Features, Sources → Places, Rate limits, CSV format, CLI, layout).
  - RED: `go test . ./cmd/... ./internal/catalog/` failed: media types lacked `places`; usage lacked `places`.
  - GREEN: `go test -tags webkit2_41 ./...` all ok; `go vet` clean; `gofmt -l .` empty; `npx tsc --noEmit` clean; `npx vitest run` 48/48.
  - Live CLI: `Eiffel Tower, France` → `Eiffel Tower - France.jpg` 1920×3553 + credit (Benh LIEU SONG, public domain); `Flag of Japan` → `Flag of Japan.png` 1920×1280 + credit. Also Machu Picchu, Sydney Opera House, Colosseum, `Córdoba, Argentina`, `USA flag`, `Flag of the United Kingdom`: all ok with credits.
- T3 done in `80dc910` feat(sheet): credits page for images with credit sidecars. Route: delegated writer. 17 files, +588/-33: `internal/sheet/credits.go` 216 (new), `credits_test.go` 234 (new), `pdf.go` +18/-9, `layout.go` +15, `labels_test.go`, `media.go` +12, `downloader/files.go`, `app.go` +12, `app_test.go` +35, `i18n/catalog.go` +2, frontend `index.html` +6, `print.ts`, `style.css`, en/es, regenerated `models.ts` +4, README +17. Authored about 380 lines, more than half tests.
  - Design: `media.CreditSuffix`/`media.CreditPath` moved from the downloader into `internal/media` (no dependencies); `downloader.CreditSuffix`/`CreditPath` stay as thin wrappers. `sheet` reads the sidecars itself (it already reads images by path) and imports `media`, not `downloader`. `Options.Credits` (`json:"credits"`, zero = off; the UI box defaults on) and `Labels.Credits` (default "Credits"; the app sets `pdf.credits`, "Créditos" in Spanish). `build()` appends credits pages after any format; when off or no sidecar it never touches the PDF. Entries: "N. Title — Creator" (`ParseName`) bold, then the sidecar lines indented, wrapped with `fitLines`; an entry that does not fit moves to the next page, one taller than a page splits; every page starts with the heading. `Options.CreditPages` counts them and `PrintPages` adds them (cheap, no deviation). A sidecar that exists but cannot be read fails the export naming it.
  - RED: `go test ./internal/sheet/ .` failed to build (undefined `media.CreditPath`, `credit`, `readCredits`, unknown field `Credits` in `sheet.Options`/`sheet.Labels`); then the non-Latin test panicked in fpdf `generateCIDFontMap` on U+1F600, and `labels_test`/`TestPDFLabelsFollowTheUILanguage` failed until the label and catalog key existed.
  - GREEN: `go test -tags webkit2_41 ./...` 15 packages ok; `go vet` clean; `gofmt -l .` empty; `wails build -tags webkit2_41` ok (only `models.ts` +4; 0644 modes kept); `npx tsc --noEmit` clean; `npx vitest run` 48/48.
  - Byte identity: `TestBuildWithoutCreditsIsUnchanged` compares on/off renders of sidecar-free images for all 7 formats with `bytes.Equal`. One-off check: with `opts.Credits = true` forced in the `render` test helper, every existing golden/content sheet test still passed (only the new on/off page-count test failed, by construction); helper restored.
  - Smoke render (scratchpad `places-smoke`, Eiffel Tower + Flag of Japan with sidecars): covers and cards PDFs 2 pages with the box on, 1 off; `pdftotext` of the last page shows "Credits", "1. Eiffel Tower — France" and "2. Flag of Japan" with all credit lines, the long Japan Source line wrapped. With Japanese/Greek/Cyrillic/emoji appended to a copy: Greek, Cyrillic and Łódź print; Japanese prints as empty boxes (text still extractable); emoji prints as "?". README says so.
  - Deviations: credits text maps invalid UTF-8 and runes above U+FFFF to "?" because fpdf v0.9.0 indexes UTF-8 glyph widths in a 65536-entry table and panics on output otherwise. The checkbox state is not persisted across restarts (no print option is).
  - Follow-up (pre-existing, not fixed): titles parsed from file names go through the same fpdf path in cards, spines, stubs, full page header, checklist and polaroids, so a file name with an emoji (non-BMP rune) likely panics those exports today.
- Review `fdb039c..7cf4528` (T1+T2; 29 files, 3148 lines, medium, `slice_budget_reached`): consent granted; lens review-reliability approved; acknowledged (authority burned). Advisory findings:
  - R3-flag-rank-slice WARNING (`wikimedia.go:143`, `rankPlaces(...)[:1]` could panic if ranking dropped entries): not a defect, `match.Rank` never drops entries. The real gap behind it: the P41 search returns any entity with a flag, so `Flag of <unknown>` downloaded an unrelated flag. Fixed below.
  - R3-flag-suffix-mode SUGGESTION: a place name ending in "flag" switches to flag mode. Accepted as a documented limitation (rare; `Flag of X` is the primary form).
- Fix (flag plausibility): flag mode now requires the best label to be a plausible (exact or prefix) match via `match.Plausible`, else ErrNotFound "no Wikidata country named %q with a flag". Route: inline (one file + test). RED: `TestWikimediaFlagModeRejectsUnrelatedCountries` requested imageinfo for "Flag of Japan (1870–1999).svg" on "Flag of Narnia"/"Atlantis flag". GREEN: providers 197 passed, vet clean, gofmt empty. Live: Flag of Japan, USA flag, Flag of the United Kingdom OK (Narnia really has a flag on Wikidata, so it downloads).

## Next step
T4.
