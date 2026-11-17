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
- [ ] T1 Name resolver + provider `commons-nature`: Wikidata taxon search, Commons search strategies, licence filter, ranking, credit, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [ ] T2 Catalog `nature`, CLI, frontend example, sample CSV, README Sources. Route: same delegated writer, separate commit.
- [ ] T3 Provider `bhl` + key setting (settings, UI, CLI, key tip) + WebP conversion, fixtures and tests, README. Route: delegated writer.
- [ ] T4 Live CLI smoke test and build. Route: inline.

## Progress
- Licensing research done (delegated read-only worker); user chose Commons then BHL with key.

## Next step
T1 via delegated writer.
