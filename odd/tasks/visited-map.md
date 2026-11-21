# Feature: visited-map (roadmap item 16)

## Objective
A generated **Visited map** page kind in Make pages: a vector map of the world or of one continent with the visited countries filled (or outlined with Colour-in), drawn from an embedded public-domain dataset. No network at print time.

## Problem / Why
A map with the visited countries coloured in is one of the most common travel spreads, and drawing one by hand is hard. The user asked for world and continent maps now; single-country (regional) maps are a later follow-up.

## Licensing check (done 2026-11-21)
- Natural Earth terms of use (https://www.naturalearthdata.com/about/terms-of-use/): "All versions of Natural Earth raster + vector map data found on this website are in the public domain. You may use the maps in any manner, including modifying the content and design ... No permission is needed to use Natural Earth. Crediting the authors is unnecessary." Its multilingual names come from Wikidata (CC0). Intended use (embed a simplified copy in the binary, print it) is allowed. Record this in the README.

## Decisions
- Kind id `map`, label "Visited map" (canvas), file tag `map`. `UsesTitle: true`, `UsesCut: false`. Default title (shared Title empty): "Places I've been" / ES "Lugares que he visitado" (canvas), drawn in the lettering font (hollow with Colour-in).
- Options (JSON), defaults `{region: "world", countries: [], colour: "teal", list: true}`:
  - `region`: `world` | `africa` | `asia` | `europe` | `north-america` | `south-america` | `oceania`. No Antarctica region; the world view stops at about 58° S (Antarctica left out).
  - `countries`: list of strings, each a country name (English or Spanish, case- and accent-insensitive) or an ISO 3166-1 alpha-2/alpha-3 code. Blank entries ignored, duplicates collapse. An unknown entry is an error `unknown country "X"` (the preview shows it, export is blocked); an entry matching two countries is an error naming them.
  - `colour`: fill colour of visited countries, one of 4 swatches `teal` | `coral` | `sage` | `lavender`.
  - `list`: print "Countries visited: N" and the visited country names (in the UI language, Go font) under the map, in columns.
- Data: Natural Earth **1:50m admin-0 countries** (241 countries, includes small states), pinned release, simplified (Douglas–Peucker) and quantized by a committed generator into a compact embedded file (target < 600 KB). Per country: ISO A2/A3 (with the `_EH` fallbacks where NE has -99, e.g. France, Norway, Kosovo XK), NE names (NAME, NAME_LONG, ADMIN, FORMAL_EN, NAME_EN, NAME_ES) and a small alias table (USA, UK, Holland, Czech Republic, Ivory Coast, Burma, Türkiye, South/North Korea, Vatican...). NE default (de facto) borders; disputed areas as NE draws them.
- Projections: world = Natural Earth projection (Šavrič et al.); continents = Lambert azimuthal equal-area centred on the region, view = projected bounds of a hand-tuned lon/lat box per region, clipped to the map rectangle. Rings more than ~100° from a continent's centre are skipped (no antipode artefacts). Every visible country is drawn the same way (no "outside the continent" styling), so a region is only a view.
- Drawing: filled mode = unvisited land light grey, visited land in the chosen colour, thin white-ish/grey borders; world view also draws the projection outline thinly. Colour-in = no fills, thin black outlines, visited countries with a heavier outline. Visited countries too small to see (projected bbox under about 1.5 mm) get a small dot marker at their centre (filled, or outlined with Colour-in).
- Layout: portrait page; title band as the other kinds; map fills the width below the title; with `list` the names go below the map, otherwise the map is centred vertically in the area.
- Labels (`PageLabels`, filled by `App.withPageLabels` from i18n): map default title, "Countries visited" caption, and the language for country names (EN `NAME` / ES `NAME_ES`).
- Form (canvas Make pages): Region select, "Visited countries (names or ISO codes)" textarea (split on commas, semicolons and new lines), Colour segmented swatches, "List countries" checkbox. EN + ES.

## Scope / constraints
- Extend, don't replace: registry entry in `internal/sheet.PageKinds`, form in `frontend/src/pageForms.ts`; bindings unchanged.
- Dot grid, moon calendar and Print covers output unchanged.
- Out of scope: single-country / regional maps (US states, provinces) — follow-up; Antarctica; picking countries by clicking the map.

## Acceptance criteria
- The PDF draws a clean vector map for every region and paper; selected countries are filled (Colour-in: no fills, heavier outline); nothing leaves the page.
- Name/ISO matching accepts common English and Spanish names and ISO codes; unknown and ambiguous entries are errors.
- Table-driven Go tests (matching, projection, "selected countries are filled", bounds per region/paper, Colour-in no fills, data integrity); frontend tests for the form. README notes the dataset's source and licence.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `wails build -tags webkit2_41`; reset 0644 modes on regenerated bindings; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/visited-map` from `b954bbe`. Forecast ~1800 authored lines (excluding the generated data file). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `b954bbe`.

## Tasks
- [x] T1 Map data: generator (download pinned NE 50m, simplify, quantize, encode), embedded dataset, decoder, country matching + aliases; tests (decode integrity, matching EN/ES/ISO/aliases/unknown/ambiguous). Route: delegated writer (writer trigger: generator + data + matcher + tests).
- [ ] T2 Map page kind: projections, region views, clipping, drawing, small-country markers, country list, labels + app wiring, registry entry; tests (projection, bounds per region × paper, filled countries, Colour-in, default title). Route: delegated writer.
- [ ] T3 Make pages form: Region / Visited countries / Colour / List, i18n EN/ES, vitest. Route: delegated writer.
- [ ] T4 Docs + verify: README Visited map section (data source, licence, projections, matching), ROADMAP item 16 notes, full checks, `wails build`. Route: delegated writer + inline state checks.

## Progress
- Feature doc created; branch `feat/visited-map` from `b954bbe`.
- T1 done (delegated writer). Commits `b509bb0` feat(sheet): embedded Natural Earth country data; `817fd6f` feat(sheet): visited country matching. ~1370 authored lines incl. tests (over the 400 heuristic: generator + format + matcher + tests are one data unit).
  - Files: `internal/sheet/mapdata/format.go` (package `mapdata`: `Encode`/`Decode`, shared by generator and sheet), `internal/sheet/mapdata/countries.bin` (embedded), `internal/sheet/mapgen/main.go` (`//go:build ignore`, `go generate` in `internal/sheet`), `internal/sheet/mapdata.go` (`mapCountries()`, `mapCountryByISO3`), `internal/sheet/mapmatch.go` (`matchCountries`, `normalizeCountry`, `countryAliases`).
  - Data: natural-earth-vector **v5.1.2** `ne_50m_admin_0_countries.geojson` (sha256 3e458fc0...7fdeb); 241 countries (ATA dropped), 1502 rings, 48917 points; Douglas–Peucker 0.02°, quantized 0.001° (0.01° would erase Vatican/Monaco), zigzag-delta varints, gzip: **146,630 bytes**. Deterministic (download and local runs byte-identical). Decoded rings are closed; outer rings and holes are flat per country (fill with even-odd). Micro-states whose rings all collapse keep their largest ring unsimplified.
  - ISO codes: `_EH` fallback only when no other feature owns the code natively (Indian Ocean Ter. and Ashmore and Cartier lose AU/AUS); Taiwan TW (NE ISO_A2 is "CN-TW"); Kosovo XK, no ISO3 (alias XKX); Somaliland, N. Cyprus, Siachen have no codes (match by name).
  - Matching: x/text `norm` (already a direct dep) + small fold table; `&` = "and", leading "the" dropped. Only NE key collision: ES "San Martín" (Saint Martin / Sint Maarten), left ambiguous. "Congo" = Republic of the Congo. Aliases keyed by NE NAME, test checks targets exist and no alias shadows another country.
  - RED/GREEN: format, data and matching tests each failed to compile first, then passed. Checks: `go test -tags webkit2_41 ./...` 1818 passed; `go vet` clean; `gofmt -l internal/` empty.
- Review `b954bbe..824e95a` (T1 + doc, medium, 1430 lines): consent granted; reliability lens approved; acknowledged (authority burned). Reviewed boundary `824e95a`. Advisories folded into T4: WARNING R3-decode-validation-untested (`mapdata/format_test.go:136`), WARNING R3-mapgen-unverified-truncating-download (`mapgen/main.go:97-111`: check HTTP status, size/sha256 of the pinned file), SUGGESTION R3-decode-accepts-trailing-bytes (`format.go:153`), R3-encode-country-limit-asymmetry (`format.go:71`), R3-france-test-nil-deref (`mapdata_test.go:113`).
