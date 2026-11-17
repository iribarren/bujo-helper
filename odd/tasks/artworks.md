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
- [x] T1 Providers: Met and Cleveland, with credit text, fixtures and tests. Route: delegated writer (2+ non-trivial files).
- [x] T2 Credit sidecar in the downloader (and gallery/sheet filtering), catalog `artworks`, CLI, frontend example, README Sources. Route: the same delegated writer, separate commit.
- [x] T3 Sheet: landscape cells in `Grid.Flow`, page counting, UI capacity, tests, README Print sheet. Route: delegated writer.
- [x] T4 Live CLI smoke test and build. Route: inline.

## Progress
- Plan commit `7e2a025`. RDD on (global); reviewed as low risk (non-executable only), approved and acknowledged.
- T1 `e0bd7b4` feat(providers): Met and Cleveland artwork search. Route: delegated writer. 13 files, +1010 (about 450 lines of tests, 250 of fixtures).
  - Met: `v1.1/search` by title; at most 10 object lookups (404s skipped); keeps `isPublicDomain && primaryImage`. Met spaced at 25 ms in the throttle; Cleveland has no entry (no stated limit).
  - Cleveland: bare `cc0` flag, retry without `artists`, CC0 only, print then web image, never the TIFF.
  - Deviation: a plausibility filter (title exact or prefix, plus the artist's surname when given) runs before the open-access check. Live, Met's title search matches any word ("Twilight in the Wilderness" returned 62 unrelated works), so without it the wrong artwork was saved instead of falling through to Cleveland.
  - Deviation: when the title search returns more than 10 IDs and an artist is given, the title + artist search (relevance-sorted) is used instead; "Self-Portrait, Rembrandt" has 492 title hits in collection order.
  - RED: build failed (`undefined: Met`, `NewMet`, `Cleveland`, unknown field `Credit`); throttle test wanted 25 ms for the Met. GREEN: providers 136 passed.
- T2 `e3c6a3b` feat(catalog): artworks media type with credit sidecars. 13 files, +308 / −18.
  - `downloader.CreditPath(img)` = `img + ".credit.txt"`, written after `saveUnique` (so duplicates get `…-2.jpg.credit.txt`). A failed sidecar write removes the image and fails the item.
  - Gallery, thumbnails and `ListCovers` already accept only image extensions; regression tests added (they passed at once, so no RED). The app has no delete/rename/overwrite path, so "the sidecar follows" has nothing to cover yet.
  - Sample swap: Monet's *Bridge over a Pond of Water Lilies* isn't public domain at the Met; replaced by Turner's *The Burning of the Houses of Lords and Commons* (Cleveland).
  - RED: `undefined: CreditPath`; catalog list missing `artworks`; CLI usage missing `artworks`. GREEN: all passed.
- Writer verification: `go test ./...` 811 passed, `go vet` clean, `gofmt` empty, `tsc` clean, vitest 39/39. No `wails build` (bindings unchanged). Live provider check: Met found Wheat Field, The Harvesters, Washington Crossing the Delaware, Rembrandt's Self-Portrait; Cleveland found Twilight in the Wilderness; Autumn Rhythm gave "1 match, none with a public-domain (CC0) image". Parent spot check: providers, downloader, catalog 182 passed.
- Review `7e2a025..e3c6a3b` (26 files, 1336 lines, medium, `slice_budget_reached`): consent granted; lens review-reliability approved; acknowledged (authority burned). Three advisory findings, fixed after T3:
  - R3-001 WARNING: when every Met object lookup fails, only the last error decides; an earlier 503/429 followed by a 404 is reported as "not found". Untested path.
  - R3-002 WARNING: an error in the free-text fallback search (title search with more than 10 IDs and an artist) discards the title IDs already found.
  - R3-003 SUGGESTION: when the sidecar write fails, the `os.Remove` error of the image is ignored.
- T3 `4970db8` feat(sheet): wide cells for landscape images. Route: delegated writer. 11 files, +553 / −77 (about 375 lines of tests).
  - Cell rule (cover sheet only), in order: square → H × H; `H·r ≤ CellWidth` → today's column; else a wide cell `min(H·r, usable width)`, shrink-to-fit. Square is checked first so square output can't change.
  - `Format.SquareCells` renamed `ShapedCells` (frontend never used it). New `Grid.Wide(r)`, `Options.PerPageFor(r)`; `SquarePerPage()` = `PerPageFor(1)`. `PrintPages` gains `widePerPage` (per size, only when every image gets a wide cell; median shape); other fields unchanged. Frontend size-label logic extracted to `sizePerPageOf` (vitest).
  - All-3:2 capacity (Mini / Standard / Large): A4 36 / 10 / 3, Letter 36 / 8 / 3, A5 18 / 3 / 2.
  - RED: build failed on the new names; with stubs 87 Go tests failed, plus 9 vitest failures. GREEN: 963 passed, vitest 48/48.
  - Byte identity vs `e3c6a3b` (399 renders): all 228 with only 2:3/narrow/square images are identical; non-cover formats identical except 6 checklist renders that are nondeterministic on base too (same checklist of repeated landscape images renders different bytes twice in one process; pre-existing, follow-up).
  - `wails build` ok; output is `build/bin/bujo-helper` (`wails.json` `outputfilename`), not `cover-downloader`.
- Fix `1144c63` fix(providers): R3-001 (the first non-404 object lookup error wins over "not found"), R3-002 (a failed fallback search keeps the title matches), R3-003 (a failed image removal is joined into the error; no test, removal failure isn't portable to provoke). Route: inline (small, understood). RED: `TestMetSearchReportsTransientObjectErrors` ("not found" instead of 503) and `TestMetSearchKeepsTitleMatchesWhenTheFallbackFails` (HTTP 500). GREEN: 965 passed, `go vet` clean, `gofmt` empty.
- [x] T4 live CLI (keyless), route inline: 5 of 6 OK, each with a `.credit.txt`. Met: Wheat Field with Cypresses (4000 × 3184), The Harvesters (3811 × 2809), Washington Crossing the Delaware (1937 × 1135). Cleveland: Twilight in the Wilderness (3400 × 2123), Burning of the Houses of Lords and Commons (3400 × 2525). Autumn Rhythm skipped: "met: not found: 1 match, none with a public-domain (CC0) image; cleveland: not found: no CC0 artworks with images".
- Review `e3c6a3b..1144c63` (14 files, 715 lines, medium, `slice_budget_reached`): consent granted; lens review-reliability approved; acknowledged (authority burned). Two advisory suggestions, left as follow-ups:
  - The `shapeKey` staleness guard in `frontend/src/print.ts` (`selectionShape`) is untested; only the pure `sizePerPageOf` is.
  - The joined removal error in the downloader (R3-003 fix) is untested.
- Follow-ups: the checklist nondeterminism with repeated landscape images (pre-existing); the two test gaps above; the PDF credits page (item 15).

- Manual GUI check passed (user, 2026-11-17). Merged to main (`cd132dc`) and pushed.

## Next step
Done.
