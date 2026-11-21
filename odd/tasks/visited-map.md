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
- [x] T2 Map page kind: projections, region views, clipping, drawing, small-country markers, country list, labels + app wiring, registry entry; tests (projection, bounds per region × paper, filled countries, Colour-in, default title). Route: delegated writer.
- [x] T3 Make pages form: Region / Visited countries / Colour / List, i18n EN/ES, vitest. Route: delegated writer.
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
- T2 done (delegated writer). Commits `6269630` feat(sheet): map projections and region views (`mapproj.go` + tests, 374 lines); `739b04d` feat(sheet): visited map page kind (`mappage.go`, `pages.go`, i18n, app wiring + tests, 1093 lines). ~1470 authored lines, over the 400 heuristic: projection + layout + drawing + list + table-driven tests (bounds per region × paper × list × labels) form one page-kind unit.
  - RED/GREEN: `mapproj_test.go`/`mappage_test.go` failed to compile first (undefined `MapOptions`, `PageLabels.MapTitle`...), then passed; app wiring tests (`TestPageLabelsFollowTheUILanguage` map labels, `TestPreviewVisitedMap` EN≠ES) failed with `app.go`/catalog stashed, then passed. Two first-run fixes were test expectations (a hand-guessed 19.6946° → 19.6931°; a "half" ring point at 99° instead of >100°), and the far-point bound became 2× the map size from its centre (Canada/USA legitimately reach ~66° from Europe's centre).
  - Projections: Natural Earth polynomial as in d3-geo-projection `naturalEarth1` (coefficients match the brief; pole y 1.42239, x(180°) 0.8707π); LAEA with λ−λ0 wrapped to [−180, 180) (Oceania 110..200 works). Region boxes as suggested in the brief, unchanged after visual check; world view −180..180 × −58..84. View rect = bbox of the box edges projected every 0.5°. Continent rings with any point > 100° from the centre are skipped; rings whose projected bbox misses the map rect are dropped; the map is clipped (`ClipRect`/`ClipEnd`).
  - Drawing: fpdf v0.9.0 supports `DF*` (`B*`) and `F*`: each country is one path, even-odd (Lesotho hole visible). Filled: land #e3e6ea, borders white 0.12 mm, world frame #cccccc 0.2 mm; colours teal #2a9d8f, coral #ee8466, sage #8fae7e, lavender #a393c9. Colour-in: black 0.15 mm, visited 0.45 mm drawn last, frame black 0.15 mm, no fills. Markers (r 0.8 mm) when the visited country's largest visible ring is < 1.5 mm both ways (Malta, Singapore, Vatican on the world map); filled with a #555 0.15 mm edge, or heavy outline only. Points closer than 0.08 mm are dropped (rings left < 3 points kept whole); round line joins; state restored inside the clip. World A4 PDF ~240 KB.
  - Layout: map at the top of `moonArea` (title band as moon) and full width; with the list it shrinks to leave room for ⌈N/3⌉ rows at 9 pt but keeps ≥ 60 % of the area height; without list/countries it is centred. List: caption 10 pt "<MapVisited>: N", names 9 pt → 6 pt floor in 2–4 columns (fewest columns at the largest size that fits), then ellipsis truncation and a final "+K" cell (language neutral). Sorted by `normalizeCountry` key (accent-insensitive).
  - Labels: `PageLabels.MapTitle`/`MapVisited`/`Language` (default "Places I've been", "Countries visited", "en"; Language is "es" or "en"); i18n keys `pdf.mapTitle`, `pdf.mapVisited` (ES "Lugares que he visitado", "Países visitados"); `Language` from `a.language()`. `MapRegions`/`MapColours` exported for T3. Bindings and `models.ts` unchanged (`Labels` is `json:"-"`).
  - Samples (scratchpad, throwaway test removed): `/tmp/claude-1001/-home-aritz-projects-cover-downloader/a80e309d-fb88-45e5-aeb1-4600560b5ea6/scratchpad/map-samples/` (world, world-ci, europe-ci, oceania, asia, africa, namerica, samerica, a5-world with 150 countries: PDFs + PNGs). Checked: no streaks or giant polygons, continents recognisable; Asia is large and tilted (accepted).
  - Checks: `go test -tags webkit2_41 ./...` 2343 passed; `go vet` clean; `gofmt -l internal/ *.go` empty; `npx tsc --noEmit` clean; `npx vitest run` 258 passed.
- Review `824e95a..1601b5d` (T2, medium, 1478 lines): consent granted; reliability lens approved; acknowledged (authority burned). Reviewed boundary `1601b5d`. Advisories folded into T4: SUGGESTION R3-map-list-ellipsis-path-unasserted (`mappage.go:366-375`: assert the "…"/"+K" truncation path), R3-map-list-forced-row-overflow (`mappage.go:366`: a forced row at the font floor could overflow; test or guard).
- T3 done (delegated writer). Commit `937f902` feat(frontend): visited map form (8 files, +464/−7 incl. tests).
  - Form (`pageForms.ts` `mapForm`): Region `<select>` (7 regions), Visited countries `<textarea rows=4>` (label `for`, placeholder and hint via `aria-describedby`), Colour segmented swatches, List countries checkbox with hint. Defaults from Go; unknown region/colour → world/teal, non-boolean `list` → true, non-array `countries` → []. `countryList()` splits on `,` `;` and new lines, trims, drops blanks; the textarea text is never reformatted, and `onChange` fires only when the parsed list changes (typing a trailing comma sends nothing). Relabels on language change keeping values; `dispose()` unsubscribes.
  - Swatches: `RadioOption.swatch` (CSS colour) in `radioGroup.ts` renders an `aria-hidden` dot, the label becomes `aria-label` (tooltip unchanged); options without it keep the old text-button DOM. Dot only (no visible text): four ES names would ellipsize in the 300 px column. Colours mirror Go `MapColours`.
  - CSS: `.check-hint-field` shares the Colour-in checkbox+hint rules; `.map-countries-field textarea` uses the UI font, auto height, pre-wrap; `.swatch`/`.swatch-button`.
  - i18n: `pages.mapRegion*`, `pages.region*`, `pages.mapCountries*`, `pages.mapColour*`, `pages.colour*`, `pages.mapList*`, `pageKind.map` (EN "Visited map", ES "Mapa de viajes").
  - RED/GREEN: 20 new tests failed first (15 `pageForms.test.ts`, 4 `pages.test.ts`, 1 `radioGroup.test.ts`), then passed. Checks: `npx tsc --noEmit` clean; `npx vitest run` 278 passed; shuffled `pages.test.ts`+`pageForms.test.ts` seeds 3/7/11: 73 passed each; `go test -tags webkit2_41 ./...` 2343 passed. Go code and bindings unchanged.
- Review `1601b5d..e5b6f37` (T3, medium, 480 lines): consent granted; reliability lens approved; acknowledged (authority burned). Reviewed boundary `e5b6f37`. Advisory folded into T4: SUGGESTION R3-001 (`pageForms.ts:219-237`: `MAP_REGIONS`/`MAP_COLOURS` hand-mirror Go `MapRegions`/`MapColours` incl. swatch hex; add a contract test).
