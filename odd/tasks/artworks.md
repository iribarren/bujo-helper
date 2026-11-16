# Feature: artworks (roadmap item 14)

## Objective
Add an **Artworks** media type (input `Title, Artist`) that downloads public-domain (CC0) artwork images from museum open-access APIs, stores a credit sidecar next to each image, and gives landscape images proper cells on the cover sheet.

## Problem / Why
Museum-visit logs, "artwork of the month" pages and art history collections glue in mini prints of artworks.

## Licensing check (read 2026-11-17)
- **The Met** (https://metmuseum.github.io/): no key. "Please limit request rate to 80 requests per second." The dataset is CC0, and the API serves "high resolution images (JPEG format) that are in the public domain". `isPublicDomain` marks open-access works; `primaryImage` is set only for Open Access. `/v1/search` was retired on 2026-10-01; use `/public/collection/v1.1/search` (returns `objectIDs` only), then `/public/collection/v1/objects/{id}`. The metmuseum.org terms and Open Access pages returned HTTP 429 and were not read (gap).
- **Cleveland Museum of Art** (https://openaccess-api.clevelandart.org/, https://www.clevelandart.org/open-access, https://www.clevelandart.org/terms-and-conditions): no key. "only artworks with the CC0 `share_license_status` will additionally provide access to CC0 images". CC0 works "can be used in any application... all without fees or restriction", and no attribution is needed. No numeric rate limit: "CMA may limit the number of transactions". `/api/artworks/?title=&artists=&cc0&has_image=1&fields=…`. `images.print` is about 3400 px, `images.web` about 900 px.
- **Art Institute of Chicago** (https://api.artic.edu/docs/): the API data and public-domain images are CC0; 60 requests/minute anonymously; it asks for an `AIC-User-Agent` header and a caption "Artist. Title, Date. The Art Institute of Chicago." **The image server `www.artic.edu/iiif/2/…` answers every non-browser request with a Cloudflare JS challenge (HTTP 403, checked 2026-11-17 with app and browser User-Agents).** No other IIIF host works.
- **User decision (2026-11-17):** sources are **the Met, then Cleveland**. AIC is dropped and the README says why. Credits: **sidecar only** (`<file>.credit.txt`); the PDF credits page is left to item 15.

## Scope / constraints
- Media type `artworks`: label "Artworks", creator label "Artist", date label "Seen on". Files are `Title - Artist.ext`.
- Providers (no key, `web.SetAPIHeaders`):
  1. **Met** (`met`): `v1.1/search?q=<title>&title=true&hasImages=true`; if that gives no IDs, retry `q=<title artist>&hasImages=true`. Fetch at most about 10 objects. Keep only `isPublicDomain && primaryImage != ""`. Rank by title and artist (`match.Rank`). Image URLs: `primaryImage`, then `primaryImageSmall` (the primary image, never `additionalImages`).
  2. **Cleveland** (`cleveland`): `api/artworks/?title=…&artists=…&cc0&has_image=1&limit=20&fields=id,title,creators,creation_date,share_license_status,images,url`. If artist-filtered gives nothing, retry without `artists`. Keep only `share_license_status == "CC0"`. The creator is `creators[0].description` with the "(American, 1826–1900)" part stripped. Image URLs: `images.print.url`, then `images.web.url`.
  - Matches that exist but aren't open access are skipped with a clear reason (an error wrapping `media.ErrNotFound` that says no public-domain image was found).
- **Credit sidecar (Attribution convention, first part):** `media.Candidate` gets a credit (text). When it is set, the downloader writes `<image file>.credit.txt` next to the saved image, using the same final name (duplicate suffixes included). Content: title and date, artist, museum, licence (`CC0 1.0 Universal (public domain)` and its URL), and the object page URL. Other media types write no sidecar. The gallery and print sheet never show or count sidecars. If the app deletes or renames an image anywhere, its sidecar follows.
- **Non-2:3 images (cover sheet, landscape part):** extend item 12's `Grid.Flow`. A 2:3-or-narrower image keeps today's column cell; a square image keeps its H × H cell; any other image gets a cell `max(CellWidth, H × r)` wide, capped at the usable width (then shrink-to-fit). Page counting (`Options.Pages`, the `PrintPages` binding) follows. The other formats keep shrink-to-fit.
- **Defaults keep today's output:** books, movies, TV, albums and games behave the same, and sheets with only 2:3 and square images are byte-identical.
- Catalog: selector, sample CSV (5 famous CC0 works), placeholder example, CLI `-type artworks`.
- README: Sources (artworks, licence rules, the AIC note, the credit sidecar) and Print sheet (landscape cells). ROADMAP item 14 records the AIC decision.
- Delivery: branch `feat/artworks` from `5446277`. Forecast about 1200 authored lines (more than half tests and fixtures). Strategy `ask-on-risk`, applied as in earlier items: one feature branch, reviewed in slices; the user GUI-checks; then a local `--no-ff` merge. Push and merge are the user's decision.

## Acceptance criteria
- Provider tests with recorded fixtures for both sources, including a match that isn't open access (skipped with a clear reason) and the search fallbacks.
- Sidecar tests: written for artworks with the final file name, not written for other media types, and ignored by the gallery and print sheet.
- Layout tests: landscape cells, mixed folders with no overlap or overflow, and 2:3/square output unchanged.
- Live CLI: a famous public-domain painting downloads (for example `Wheat Field with Cypresses, Vincent van Gogh`) with its sidecar.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`, `gofmt -l .`
- Frontend: `npx tsc --noEmit` and the frontend tests in `frontend/`. If the bindings change, run `wails build -tags webkit2_41`, restore the 0644 modes on `frontend/wailsjs/go/main/*` and `models.ts`, and rebuild `build/bin/cover-downloader` before the GUI check.

## Tasks
- [ ] T1 Providers: Met and Cleveland, with credit text, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [ ] T2 Credit sidecar in the downloader (and gallery/sheet filtering), catalog `artworks`, CLI, frontend example, README Sources. Route: the same delegated writer, separate commit.
- [ ] T3 Sheet: landscape cells in `Grid.Flow`, page counting, UI capacity, tests, README Print sheet. Route: delegated writer.
- [ ] T4 Live CLI smoke test and build. Route: inline.

## Progress
- Plan commit pending.

## Next step
T1.
