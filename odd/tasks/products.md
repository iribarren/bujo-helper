# Feature: products (roadmap item 23)

## Objective
A **Products** media type for food and drink logs (tea, coffee, wine, beer, snacks): input `Product, Brand` or a barcode; the app downloads the product's front image from Open Food Facts and stores its CC BY-SA credit next to it.

## Problem / Why
Tea, coffee, wine, beer and snack logs glue in product or label pictures. Open Food Facts is the open product database; its images are CC BY-SA, so each needs credit, and quality varies, so small images are skipped.

## Licensing (checked 2026-11-23)
- Terms (https://world.openfoodfacts.org/terms-of-use): product images are under CC BY-SA 3.0; re-users "mention the licence and attribute the authorship to Open Food Facts with a link to https://openfoodfacts.org ... or the product page"; contributors are credited "by a link to the product to which they contribute". Images "may contain graphical elements subject to copyright or other rights" (brand designs, trademarks): re-users must check those themselves. Reuse is allowed "for all purposes, including commercial use" under attribution and share-alike. Nothing forbids a distributed app.
- API (https://openfoodfacts.github.io/openfoodfacts-server/api/): reads need no key, only a custom User-Agent `AppName/Version (ContactEmail)`. Limits per IP: 15 req/min for product reads, 10 req/min for searches; breaches can lead to an IP ban. "1 API call = 1 real scan by a user"; scraping is blocked.
- Search: the legacy `/cgi/search.pl` answered "Page temporarily unavailable" (2026-11-23 probe), and full-text search is not in the v2 API. Search-a-licious (`https://search.openfoodfacts.org/search`, Lucene query syntax, `fields=`, `langs=`) works and returns the `images` map with `rev`, `imgid`, `sizes.full.w/h` and per-upload `uploader`.
- Images: `https://images.openfoodfacts.org/images/products/<folder>/front_<lang>.<rev>.full.jpg`; folder = the code when it has 8 digits or fewer, else the code left-padded to 13 and split 3/3/3/rest (probe: `800/050/027/8802/front_en.33.full.jpg` → 200 image/jpeg).

## Decisions
- Media type id `products`, label "Products", creator label "Brand", date label "Tried on" (ES: Productos / Marca / Probado el). Registered after `nature`.
- Provider `OpenFoodFacts`, `Label()` "Open Food Facts", keyless.
  - Barcode: the title (spaces and hyphens removed) is 8–14 digits and the brand is empty → `GET https://world.openfoodfacts.org/api/v2/product/<code>?fields=code,product_name,brands,lang,images`.
  - Otherwise Search-a-licious: `q = <name> brands:"<brand>"` (brand clause only when set; Lucene special characters escaped), `page_size=10`, `fields=code,product_name,brands,lang,images`. Hits ranked by name match; at most 3 candidates.
  - Front image choice: `front_<UI/country language>` if known, then `front_<product lang>`, `front_en`, then any `front_*`.
  - Minimum resolution: the full image's shorter side must be at least 400 px (a 50 mm cover at about 200 dpi); unknown sizes are skipped. Nothing usable → `ErrNotFound` with a reason per product.
  - Credit lines: `Title: <name> (<brand>)`, `Contributor: <uploader>` (when known), `Source: Open Food Facts`, `License: CC BY-SA 3.0 (https://creativecommons.org/licenses/by-sa/3.0/)`, `Product page: https://world.openfoodfacts.org/product/<code>`.
  - Contact User-Agent; throttle 4 s for `world.openfoodfacts.org` (15/min), 6 s for `search.openfoodfacts.org` (10/min).
- A barcode line saves as `<barcode>.jpg` (file names come from the input); noted in the README.

## Scope / constraints
- Extend, don't replace: one `mustRegister`, provider in `internal/providers`, existing credit sidecar and credits page.
- EN + ES strings; CLI, sample CSV, README Sources.
- Out of scope: naming barcode files after the product, other product databases, uploading photos back to OFF, Open Beauty/Pet Food Facts.

## Acceptance criteria
- Provider tests with recorded fixtures: a common product (by name and brand, and by barcode) returns a usable front image with its credit; small, missing-front and not-found products are skipped with a reason; query escaping; image URL/folder rule.
- Catalog/app/CLI tests list the new type; sample CSV works.
- README Sources documents the licence, credit format, rate limits and trademark caveat.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`
- `cd frontend && npx tsc --noEmit && npx vitest run`; delete `frontend/.vitest/`.
- Live smoke: one name+brand and one barcode download through the CLI.

## Delivery
- Branch `feat/products` from `9cf6705`. Forecast ~750 authored lines (≈50 % tests, fixtures excluded). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `9cf6705`.

## Tasks
- [x] T1 Provider `OpenFoodFacts` (Go): barcode lookup, Search-a-licious search, front image choice, min resolution, credit, UA + throttle; fixtures and tests. Route: delegated writer (writer trigger: provider + web + tests + fixtures).
- [ ] T2 Wiring: catalog registration, app/catalog/CLI tests, CLI help, frontend example + EN/ES strings. Route: delegated writer (2+ files).
- [ ] T3 Docs + verify: README Features/Sources/Rate limits/CSV/Credits page, ROADMAP item 23 notes, full checks, live smoke. Route: delegated writer + inline checks.

## Progress
- Feature doc created; branch `feat/products` from `9cf6705`.
- T1 done in commit `feat(providers): Open Food Facts product images` (hash: the T1 commit on `feat/products`; a commit cannot hold its own hash). Route: delegated writer. Authored: 875 lines (446 provider, 374 tests, 55 web/throttle + tests; fixtures excluded), over the ~400 heuristic because the provider has two APIs plus lenient JSON decoding, and tests were not cut. Checks: `go test -tags webkit2_41 ./...` 3253 passed; `go vet` clean; `gofmt -l internal/` empty. Fixtures: 3 live requests (Nutella product, `nutella brands:"ferrero"` search, unknown code) trimmed, plus synthetic skipped/empty searches.
- T1 choices: a too-small or sizeless preferred front falls through to the next language (reason = first front tried); search ranking ties prefer hits with the brand anywhere in their brand list; codes that are not all digits are skipped ("invalid barcode"); language from MARC `Options.Language`, else `locale.For(Country)`.

## Next step
T2.
