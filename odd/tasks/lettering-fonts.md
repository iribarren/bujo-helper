# Feature: lettering-fonts (roadmap item 31)

## Objective
Ten bundled lettering fonts (OFL / Apache 2.0) for display text, a font picker and a shared **Colour-in** toggle in the Lettering fieldset, and hollow (stroke-only) letters drawn with the PDF `Tr` operator.

## Problem / Why
Printed headers in plain Go sans look out of place on a hand-drawn spread. Hollow letters let the user colour headers by hand. The generated printables (items 16, 18, 27, 28) need the Lettering controls.

## Research (see ROADMAP item 31, 2026-11-20)
- OFL 1.1 / Apache 2.0 allow bundling with the licence text; documents made with the fonts are free; ship the original TTFs unmodified (Reserved Font Names).
- fpdf v0.9.0: `AddUTF8FontFromBytes` (static `glyf` TTFs only), automatic subsetting, `SetTextRenderingMode(1)` for stroke (not tracked: reset to 0), no shaping/kerning; fpdf is archived.
- Downloaded 2026-11-20 from `github.com/google/fonts` (`main`): all ten are static `glyf` TTFs (no `CFF `, no `fvar`), 1.7 MB with licences, in `internal/sheet/fonts/<id>/`.

## Decisions
- Font ids: `go` (default; empty = `go`), `kaushan-script`, `pacifico`, `sacramento`, `patrick-hand`, `amatic-sc`, `titan-one`, `bungee`, `luckiest-guy`, `bungee-outline`, `londrina-outline`. Groups: `sans`, `brush`, `monoline`, `handwritten`, `display`, `hollow`. Hollow capability `stroke` / `none` (Sacramento, Amatic SC) / `native` (Bungee Outline, Londrina Outline).
- Lettering applies to: Full page and Checklist headers, Dot grid title, Review card titles. Captions, dates, stubs, spines, polaroid captions, checklist rows and credits stay in Go.
- Go keeps today's bold header; bundled fonts are single-style (registered with style `""`).
- Hollow: mode 1, stroke ≈0.25 mm scaled with size (min 0.2 mm), black; only at ≥14 pt, else filled; always reset to mode 0. `native` fonts are never stroked; `none` fonts stay filled.
- Credits: the PDF credits page gains a "Fonts" section only when that page is printed anyway (Credits on and at least one image credit) and a bundled font was used — keeps "no sidecar → no page" and `CreditPages` unchanged. Page kinds have no credits page. README + Settings About list every bundled font and licence.
- Print covers: a `usesLettering` flag (Full page, Checklist, Review cards) shows Font + Colour-in; the Header text stays tied to `usesHeader`. Make pages always shows them.
- Fonts reach the webview through the asset handler (`/fonts/<id>.ttf`, licence at `/fonts/<id>/licence`), not the Vite bundle.

## Scope / constraints
- Extend, don't replace: `sheet.Options` and `sheet.PageSettings` gain `Font` + `ColourIn`; header fitting/drawing take the chosen font.
- Default font + Colour-in off → every existing format byte-identical (page streams).
- EN + ES for every label; keyboard-navigable listbox (`role="listbox"`); dark theme.
- Out of scope: user fonts, OpenType shaping, line-art photos.

## Acceptance criteria
- Every bundled font loads through fpdf with no `pdf.Err()`; each covers the EN + ES alphabets (á é í ó ú ñ ü ¿ ¡).
- Hollow mode emits `1 Tr` … `0 Tr`; no `Tr` state leaks to the next text or page.
- Default font + Colour-in off: byte-identical output for every format.
- Headers auto-fit without overflow in every font (table-driven over fonts).

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `wails build -tags webkit2_41`; reset 0644 modes on regenerated bindings; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/lettering-fonts` from `782ab17`. Forecast ~1500 authored lines (≈40 % tests; font binaries/licences excluded). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `782ab17`.

## Tasks
- [x] T1 Font registry + delivery: embedded fonts/licences, `internal/sheet` registry (id, label, group, TTF, licence name/text, hollow capability), `LetteringFonts()` binding, `/fonts/` asset route, wailsjs regenerated; tests: every font loads in fpdf, EN/ES coverage (sfnt), route serves TTF/licence, unknown 404. Route: delegated writer (writer trigger: new Go package files + app.go + bindings).
- [x] T2 Lettering in the PDF: `Font`/`ColourIn` on `Options` and `PageSettings`, hollow helper, header/title fitting and drawing in the chosen font (full page, checklist, dot grid, review card titles), swash safety margin, credits Fonts section; tests: `Tr` pairing/no leak, fit table over fonts, byte-identical defaults. Route: delegated writer (writer trigger: 6+ Go files).
- [x] T3 Lettering fieldset UI: font picker button + listbox popover (rows in own face, group, badge), `@font-face` from `/fonts/`, Colour-in checkbox + hint, `usesLettering` in Print covers, Make pages wiring, options sent to Go, i18n EN/ES, vitest. Route: delegated writer.
- [x] T4 Docs + About + verify: Settings About (fonts + licences), README (Lettering section, Third-party table incl. Go fonts), ROADMAP notes, page-stream comparison vs `782ab17`, full checks, `wails build`. Route: delegated writer + inline state checks.

## Progress
- Feature doc created; branch `feat/lettering-fonts` from `782ab17`.
- T1 (`6266d44`): `internal/sheet/lettering.go` registry (`Font{ID, Label, Group, TTF, Bold, Licence, LicenceText, Hollow}`, `LetteringFonts` default first, `LetteringFont(id)` falls back to `go` for empty/unknown, `LetteringFontByID` exact, `addLetteringFont(pdf, f)` registers family `f.ID`, `HeaderStyle()` "B" for Go only); Go licence copied from x/image `font/gofont/ttfs/README` to `internal/sheet/fonts/go/LICENSE`; Apache detected by `LICENSE.txt` (Luckiest Guy). `internal/fonts` route: `/fonts/<id>.ttf` (header face; Go Bold for go; `max-age=86400`), `/fonts/<id>/licence`; 404 unknown/other paths, 405 + `Allow` for non-GET/HEAD. `LetteringFonts()` binding; wailsjs regenerated (modes reset to 644). RED: three packages failed to build (undefined registry/handler symbols). GREEN: go test 1586 pass, vet clean, tsc clean, vitest 193/193. No glyph gaps: every face maps the EN/ES alphabets. Existing drawing untouched.
- Review `782ab17..8ec5484` (T1 + doc, medium, 1766 lines mostly licence text): consent granted; reliability lens approved; acknowledged (authority burned). Advisory SUGGESTIONs folded into T2: R3-id-charset-invariant-untested (assert ids match `^[a-z0-9-]+$`), R3-cache-across-upgrades (`max-age` with no validator; add a content-hash ETag). Next reviewed boundary: `8ec5484`.
- Re-review `782ab17..c146a59` (stop-hook preflight; T1 + doc): consent granted; reliability approved; acknowledged. Extra advisory R3-app-route-body-unchecked (app-level route test checks status/type only; body equality proven at handler level) — minor follow-up. Reviewed boundary `c146a59`.
- T2 (`64c77bb`, `867dd0a`; route: delegated writer, interrupted once by a usage limit and resumed): `Font`/`ColourIn` (`font`, `colourIn`) on `Options` and `PageSettings`; `UsesLettering`/`usesLettering` (Full page, Checklist, Review cards); internal `lettering{font, colourIn}` threaded through `fitHeader`, `drawHeader`, `drawTitleAndCreator`, `pageDoc.draw`; stubs keep Go. Bundled fonts fit into 96 % of the width (Go keeps 100 %). Hollow only for `stroke` fonts at ≥14 pt with Colour-in; stroke `max(0.2, 0.25 × pt/20)` mm black; a `defer` restores `0 Tr`, line width and draw colour. Review card titles are 9.5 pt → always filled. Credits: "Fonts"/"Fuentes" section (`pdf.fonts`, `Labels.Fonts`) after the image credits only when the page prints anyway and a bundled font is used; `CreditPages` uses the same layout. T1 advisories: id charset `^[a-z0-9-]+$` asserted; ETag = quoted first 8 bytes of sha256 (304 on `If-None-Match`). `renderPage` test helper now pins catalog sort (dot grid was flaky ~3/20). RED: sheet package failed to build (undefined `lettering`, …); `TestETagRevalidates` failed (`ETag ""`). GREEN: go test 1629 pass, vet clean, tsc clean, vitest 193/193, no mode changes. Base comparison vs `782ab17` (worktree, removed): 2700/2700 PDFs identical page streams + sorted per-stream hashes, 2592/2592 `CreditPages` identical, 0 errors (whole-file bytes vary run to run even base vs base).
- Review `c146a59..7087232` (T2, medium, 899 lines): consent granted; reliability approved with no findings; acknowledged. Stop-hook branch review `782ab17..7087232` (2656 lines): consent granted; approved (only R3-app-route-body-unchecked again); acknowledged. Reviewed boundary `7087232`. Local `.git/info/exclude` gained `.opencode/` (tool state blocked the untracked inventory).
- T3 (`7ce9e03`, `b22d7e7`; route: delegated writer, run while T2 was reviewed): `frontend/src/lettering.ts` picker (button in the font's face + chevron, popover `role="listbox"` with `aria-activedescendant`; arrows/Home/End/Enter/Space/Escape/Tab, click-outside and focus-out close), rows with label in own face, translated group and badge; one `<style id="lettering-font-faces">` with `@font-face "lettering-<id>"` from `/fonts/<id>.ttf` (Go → falls back to system-ui); fetch failure not cached (`lettering.errFonts`). Colour-in checkbox + hint per hollow capability; with a `native` font it shows checked and snaps back on click, sending the stored preference. Each tab keeps its own font/colour-in (like paper); no localStorage. Print covers: Font + Colour-in when `usesLettering`, others send `''`/`false`; font in the page-count key (credits Fonts section), both in the preview key. Make pages sends both in `PageSettings`. EN/ES strings. RED: 17 failing tests (missing module/ids/options). GREEN: tsc clean, vitest 222/222 (parent re-ran: 222/222), go test 1629. Known: Review card titles are 9.5 pt so Colour-in has no visible effect there; the open listbox lengthens the scrolling options column.
- Review `7087232..4a160d5` (T3, medium, 1194 lines): consent granted; reliability approved; acknowledged. Advisories (two WARNINGs on untested font-load failure paths, one SUGGESTION on the id/URL filter) folded into T4. Reviewed boundary `4a160d5`.
- Stop-hook branch review `782ab17..4a160d5` (3846 lines): consent granted, but START stopped with `lens_context_budget_exceeded` (terminal, no authority created). Every slice was already reviewed separately.
- T4 (`bcaab2c`, `1c41692`, `bfc4376`; route: delegated writer): tests for the T3 advisories (`letteringLoad.test.ts`, `letteringFailure.test.ts`; no bug found; a mutation check confirmed they fail without the cache reset or the filter) and for the app-level font route bodies; Settings About (`<details>` per font with a lazy licence fetch, pdf.js named; EN/ES); README `## Lettering` section, font table, Third-party rows, layout and `/fonts/`; ROADMAP item 31 "Done in branch" block. RED: 5 About tests failed first. GREEN: go test 1629, vet clean, tsc clean, vitest 233/233, `wails build -tags webkit2_41` → `build/bin/bujo-helper` (16.8 MB); bindings 644, no content changes.
- Review `4a160d5..008b3cc` (T4, medium, 652 lines): consent granted; reliability approved; acknowledged. Advisory SUGGESTIONs left as follow-ups: R3-go-licence-route-unasserted (`app_test.go:697`: no body check for the Go licence route), R3-microtask-ordering-assumption (`letteringLoad.test.ts:24`: retry test relies on one microtask hop). Every code commit on the branch is reviewed.

## Next step
The user's GUI check, then the user's GUI check, then a local `--no-ff` merge into `main` and ROADMAP item 31 marked `[x]`.
