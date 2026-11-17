# Feature: nature (roadmap item 17)

## Objective
Add a **Nature** media type (input `Name`, common or scientific, e.g. `Robin`, `Monstera deliciosa`) that downloads a vintage natural-history illustration (a plate, not a photo) from Wikimedia Commons, or from the Biodiversity Heritage Library (BHL) when the user has set a BHL API key, and stores a credit sidecar next to each image.

## Problem / Why
Bird-watching logs, plant and garden journals and nature journals use vintage natural-history plates; collage sheets of them are very popular.

## Licensing check (read 2026-11-17)
- **BHL** (https://about.biodiversitylibrary.org/help/copyright-and-reuse/, https://about.biodiversitylibrary.org/tools-and-services/developer-and-data-tools/): public-domain works may be reused, commercial reuse included. Copyright Status `NOT_IN_COPYRIGHT`, "Not specified" or blank means public domain; "No known copyright restrictions" means reuse OK with caution; "In copyright. Digitized with the permission…" is usually CC BY-NC-SA and is skipped. Requested credit: `Image from the Biodiversity Heritage Library. Contributed by <Holding Institution>. | www.biodiversitylibrary.org` (appreciated for public domain). Metadata is CC0. API v3 needs a free per-user key (name + email, emailed); without one, HTTP 401. IP rate limits exist but the numbers are unpublished. The HTML site sits behind a Cloudflare challenge; `api3` and `/pageimage/` were not challenged. Page images redirect to WebP on S3 (no JPEG variant). Nothing forbids downloading a public-domain page image for personal printing. Each user brings their own key; none ships in the repo.
- **Wikidata** (https://www.wikidata.org/wiki/Wikidata:Licensing): CC0.
- **Commons**: same rules as item 15 (see `odd/tasks/places.md`): credit author + licence + link, contact User-Agent, standard thumbnail widths, gentle rates. Licence data comes from `imageinfo` `extmetadata`.
- **Verdict:** nothing forbids the intended use.

## Decisions
- **Source order (user, 2026-11-17):** Commons first (no key), then BHL when a key is set. The roadmap listed BHL first; research found BHL's name index is OCR text-based (mostly text pages), WebP-only, keyed and Cloudflare-fronted, while Commons already holds many BHL plates as public-domain files.
- **Licences accepted:** public domain, CC0, CC BY, CC BY-SA (open, credit stored). NC and ND licences and anything unknown are skipped with a reason. BHL: only public-domain / not-in-copyright / no-known-restrictions items.

## Scope / constraints
- Media type `nature`: label "Nature", creator label empty or unused (input is one column `Name`), date label "Seen on". Files are `Name.ext`.
- **Name resolution** (shared by both providers, cached in memory per name): Wikidata CirrusSearch `<name> haswbstatement:P31=Q16521` (taxon), then `wbgetentities` for labels, P225 (taxon name), P373 (Commons category). Ranked by label/taxon-name match. No hit → `media.ErrNotFound`.
- **Provider `commons-nature`** (no key), searches File namespace in order until one gives an accepted image:
  1. `haswbstatement:P180=<Q> haswbstatement:P6216=Q19652 filetype:bitmap` (depicts the taxon, public domain)
  2. `incategory:"<P373> - botanical illustrations"` (plants)
  3. `"<taxon name>" plate filetype:bitmap`
  Then Commons `imageinfo` (`extmetadata`, `iiurlwidth` 1920 then 1280); licence filter above; rank files with "Plate" in the title or a BHL Flickr credit first. Reuse `wikimedia.go` helpers (search, entities, imageinfo, `htmlText`, `commonsLicense`, credit text, contact UA, throttle).
- **Provider `bhl`** (key; skipped cleanly without one, like TMDB): `GetNameMetadata` by taxon name (capped response size), keep pages with page type `Illustration` or page number prefix `Plate`, prefer older publications; `GetItemMetadata` for the chosen item's rights; accepted statuses only; image `https://www.biodiversitylibrary.org/pageimage/<PageID>`. WebP is converted to JPEG (or PNG) at download so the print sheet keeps working, via `golang.org/x/image/webp` (new dependency, decode only). Key travels in the URL, so errors redact it. Credit: BHL's requested line plus the page URL and title/year.
- **Key setting:** BHL key stored and edited like the TMDB key (settings, Settings UI, key tip, CLI flag/env), `catalog.Keys.BHL`.
- **Credit sidecar:** reuse `media.Candidate.Credit` and `<file>.credit.txt`; the item 15 credits page lists them.
- **Non-2:3 images:** plates come in any shape; reuse the wide/square cells from items 12/14.
- Catalog: selector, sample CSV (e.g. Robin, Monstera deliciosa, Barn owl, Common sunflower, Monarch butterfly), placeholder example, CLI `-type nature`.
- README: Sources (Nature: pipeline, rights check, credit format, BHL key setup) and the key tip.
- **Out of scope:** the optional **Plant care card** print format (follow-up); GBIF.
- Delivery: branch `feat/nature` from `4fa0a6b`. Forecast about 1300 authored lines (more than half tests and fixtures). Strategy `ask-on-risk` as in earlier items: one feature branch reviewed in slices; the user GUI-checks; then a local `--no-ff` merge. Push and merge are the user's decision.

## Acceptance criteria
- Provider tests with recorded fixtures: a common bird and a common houseplant each return an illustration with its credit; a non-open licence is skipped with a reason; no taxon found; BHL skipped without a key; BHL in-copyright item skipped; BHL WebP converted.
- Live CLI: `Robin` and `Monstera deliciosa` download with a `.credit.txt` (Commons). BHL live check only if the user provides a key.
- README documents the rights check and the credit format.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` and vitest in `frontend/`. If bindings change, `wails build -tags webkit2_41`, restore 0644 modes on `frontend/wailsjs/go/main/*` and `models.ts`.

## Tasks
- [x] T1 Name resolver + provider `commons-nature`: Wikidata taxon search, Commons search strategies, licence filter, ranking, credit, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [x] T2 Catalog `nature`, CLI, frontend example, sample CSV, README Sources. Route: same delegated writer, separate commit.
- [ ] T3 Provider `bhl` + key setting (settings, UI, CLI, key tip) + WebP conversion, fixtures and tests, README. Route: delegated writer.
- [ ] T4 Live CLI smoke test and build. Route: inline.

## Progress
- Licensing research done (delegated read-only worker); user chose Commons then BHL with key.
- T1 done in `6223b72` feat(providers): Commons nature plates via Wikidata taxa. Route: delegated writer. 17 files, +2491/-27: `taxon.go` 145 (new), `nature.go` 244 (new), `wikimedia.go` +115/-27 (shared helpers), `taxon_test.go` 258, `nature_test.go` 587, 12 trimmed live fixtures under `testdata/nature/` (about 1170 lines of JSON: Robin, Monstera deliciosa). Authored code about 480 lines and tests about 845, over the 400 heuristic: resolver, provider, licence filter, ranking and credit with their tests are one unit; fixtures are recorded data.
  - RED: `go test ./internal/providers/` failed to build (undefined `TaxonResolver`, `NewTaxonResolver`, `CommonsNature`, `NewCommonsNature`, `openLicense`, `visibleHTMLText`).
  - GREEN: `go test -tags webkit2_41 -race ./internal/providers/` 256 passed (59 new nature/taxon cases); `go vet` clean; `gofmt -l .` empty. Places tests unchanged and passing.
  - Design: `TaxonResolver` (`Resolve(ctx, name) (Taxon, error)`; `Taxon{ID, Label, Name, Category}`) caches successes and ErrNotFound per normalized name under a mutex; network/API failures are not cached. `NewCommonsNature(client, taxa)` takes a shared resolver (nil = new); the catalog shares one for T3. Provider name `commons-nature`. Candidate title = taxon English label, creator = P225 when it differs from the label (e.g. "European robin - Erithacus rubecula" in the log).
  - `wikimedia.go` refactor: `mediawikiSearch` (any namespace), `wikidataItemSearch`, `wikidataEntities`, `commonsImageInfo` (metadata filter parameter) and `commonsCreditLines` (field reader + optional date) are now free functions the Places methods call; `firstFile` renamed `firstString`; entities decode `aliases`; new `visibleHTMLText`/`visibleMetadata` drop `display: none` elements. Places requests and credits are byte-identical (its tests unchanged).
  - Deviations: (1) ranking uses English label **and aliases**, not P225: `wbgetentities` with claims for 10 bird hits is 1.1 MB (European robin alone 180 KB), so labels/aliases are fetched for all hits (2.5 KB) and claims only for the chosen one (3 Wikidata requests). All 13 recorded hits hold P225 as label or alias. Plausibility = `match.Plausible` on the best label/alias (exact or prefix). (2) BHL detection also accepts `biodiversitylibrary.org` credits (BHL page links, e.g. the May Rivers plates), not only the BHL Flickr account. (3) The nature credit adds a `Date:` line and uses visible text only, which drops Commons' hidden "date QS:…" data and duplicated "Unknown author" spans; Places keeps its credit as before. (4) Commons hosts no NC/ND files, so the non-open licence cases modify the recorded imageinfo in the test (`withLicense`) instead of a live recording. (5) Search hits with "Plate" in the title are moved first before the 5-file imageinfo cut, so plates past the 5th hit are still considered. (6) Category search uses spaces (underscores work the same live). Up to 3 candidates are returned; the not-found error names up to 5 skipped files with reasons.
  - Live findings: `Robin` → Q25334 (European robin; ties with American robin's "robin" alias keep search order); `Swiss cheese plant` → Q161077 via alias; depicts+PD search finds 7 robin files and 4 monstera files; robin has no botanical-illustrations category; the "plate" search for *Erithacus rubecula* returns BHL Flickr plates. Strategy 1 can return photos from old books (e.g. "Ceriman blossoms… photo"), ranked after plates/BHL.
- T2 done in `34b71d4` feat(catalog): nature media type. Route: same delegated writer. 9 files, +120/-19: catalog registration (shared `TaxonResolver`, provider `commons-nature`, samples Robin, Monstera deliciosa, Barn owl, Common sunflower, Monarch butterfly), catalog/app/CLI tests, CLI usage (`-type nature`, `"Name"`), frontend example `Robin`, en/es labels, README (Features, Sources → Nature with pipeline, rights check, credit format; Artworks/Credits page cross-links, Rate limits, CSV format, CLI, layout). No BHL text.
  - Creator column: `CreatorLabel` is empty (one `Name` column). CSV parsing already accepts a missing second column (`Creator` = ""), and `Name.ext` file names follow. The frontend now shows a name-only placeholder (`download.placeholderName[Example]`) and error (`download.errNoNames`) for a type without a creator label; the sample CSV title has its own `sample.title.nature`. The Go sample CSV header is `title,` (app.go builds `title,<creator label>`; it reads back unchanged and `TestSampleCSVForEachBuiltInType` passes); app.go was outside the edit surface.
  - RED: with catalog.go and main.go reverted, `go test ./internal/catalog/ ./cmd/covers/ .` failed: media types lacked `nature` (TestDefault, TestDefaultAppMoviesUseTheSavedTMDBKey); usage lacked `nature` (TestUsageListsMediaTypes).
  - GREEN: `go test -tags webkit2_41 ./...` 15 packages ok (1125 passed); `go vet` clean; `gofmt -l .` empty; `npx tsc --noEmit` clean; `npx vitest run` 48/48 (no `.vitest/` left).
  - Live CLI (scratchpad `nature-smoke`): `Robin` → `Robin.jpg` 1211×1956 (Lilford, *Coloured figures of the birds of the British Islands*, 1885–1897, BHL Flickr, public domain) + credit with the BHL line; `Monstera deliciosa` → `Monstera deliciosa.jpg` 1192×1531 (*Berliner allgemeine Gartenzeitung*, 1857, BHL Flickr, public domain) + credit. Also `Barn owl` 1920×2528 (*Der naturen bloeme*, KB), `Common sunflower` 1204×1987 (*Illustratio systematis sexualis Linnaeani* Tab. 68), `Monarch butterfly` 1920×1272 ("Painted caterpillar"): all OK with credits.

## Next step
Native review assessment of `6223b72..34b71d4` (parent), then T3 (BHL provider + key) via delegated writer. Open question: the sample CSV header for a type without a creator label is `title,` (app.go); consider writing `name` alone.
