# Feature: ui-restyle (layout part of roadmap item 29)

## Objective
Restyle the existing Download and Print tabs to match the design canvas (https://claude.ai/artifact/6gdFFavUM9kyA76MsCpB1R, artboards **Download** and **Print covers**) with no change in behaviour.

## Problem / Why
The Print options are one long flex row, and the media type sits in the global header although only Download (and Print's "Cards for" default) use it. Items 29–31 need a fieldset-based layout to grow into. This restyle lands the layout first so item 29 only adds the Make pages tab and the backend.

## Scope / constraints
- Frontend only: `frontend/index.html`, `frontend/src/main.ts`, `print.ts` (only if needed), `style.css`, `i18n/en.ts`, `i18n/es.ts`, frontend tests. **No Go, bindings or PDF change**; if one seems needed, stop and ask.
- Canvas = layout and visuals only. Rebuild with `style.css` tokens and components (`.field`, `button.primary`, `.tab`, …); no inline styles from the canvas.
- Tabs: rename "Print sheet" → "Print covers" / "Imprimir portadas"; internal id stays `print`. Tab list data-driven (still exactly two tabs), same keyboard nav (arrows, Home, End).
- Download: media type becomes a radio group of buttons (5 per row, selected = accent border + tinted background), inside the Download panel, fed by `MediaTypes()`; `role="radiogroup"`, `aria-checked`, arrow keys move the selection. Every reader of the current media type keeps working (Print "Cards for" follows it). Remove `#app[data-tab="print"] #media-type`. Two columns: input left, progress + results + footer right.
- "Sources tried" line: **skipped** — `MediaTypeInfo` has only `id`, `label`, `creatorLabel`, `dateLabel`; provider names would need a Go change.
- Print covers: options column (~300 px) with fieldsets Covers (folder, Browse, Refresh), Layout (Format, Cover size, Crop to, Cards for), Page (Paper, Cut mode, Credits page), Lettering (Header). Toolbar + gallery + footer on the right. Every element id kept (`print.ts` `byId` throws). `updateFormat` rules unchanged; additionally a fieldset with no visible field is hidden (Lettering only for header formats). Gallery: 6 cards per row at the default window size, round check badge top-right, accent border when selected; same selection logic.
- Styling: new colours only through CSS variables, dark theme works; ≤700 px columns stack; EN + ES for every new label.
- Out of scope: Make pages tab, no-image backend (item 29), Gallery | Preview (30), Font picker / Colour-in (31), "Print these covers" link, B5.

## Acceptance criteria
- Exported PDFs byte-identical to `main` for the same options (page-stream hashes per format).
- `go test ./...` passes unchanged. Frontend tests pass, with tests for tab switching + keyboard nav, the media type radio group (click + arrows update selection; "Cards for" follows), fieldset visibility per format.
- `wails build -tags webkit2_41` builds `build/bin/bujo-helper`.

## Checks
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `go test -tags webkit2_41 ./...`, `git diff main --stat -- '*.go' frontend/wailsjs` empty
- Page-stream hashes: scratch test via `go test -overlay`, base tree from `git archive main`.
- `wails build -tags webkit2_41`; revert any 0755 mode change on regenerated bindings.

## Delivery
- Branch `feat/ui-restyle` from `c774745`. Forecast ~700 authored lines (≈ 35 % tests). Strategy `single-pr` (user GUI-checks before a local `--no-ff` merge), as earlier items. Merge only after the user tries the build.
- RDD on (global). First reviewed boundary: `c774745`.

## Tasks
- [x] T1 Tabs: data-driven tab list, rename, keyboard-nav tests. Route: delegated writer (writer trigger: index.html, main.ts, i18n, tests).
- [x] T2 Download tab: media type radio group in the panel, two-column layout, CSS, tests. Route: same delegated writer, own commit.
- [x] T3 Print covers tab: fieldsets, fieldset visibility, gallery cards, CSS, tests. Route: same delegated writer, own commit.
- [x] T4 Verify: PDF page-stream hashes vs `main`, Go tests, `wails build`; ROADMAP item 29 notes the done layout bullets. Route: inline (state checks) + delegated hash check.

## Progress
- Feature doc created; branch `feat/ui-restyle` from `c774745`.

- T1 done in `12513cb`: new `frontend/src/tabs.ts` (`initTabs` reads `[role=tab]` with `data-tab` + `aria-controls` from the markup; adding a tab is markup only, labels stay `data-i18n`), `main.ts` wires it, tab renamed (EN "Print covers", ES "Imprimir portadas"). Tests: `tabs.test.ts` (click, arrows wrap, Home/End, focus, missing panel), `main.test.ts` tabs block. RED: `tabs.test.ts` (module missing), `main.test.ts > tabs > are named Download and Print covers`; GREEN: 85/85. Deviation: tabs discovered from markup instead of a TS `TABS` array (keeps static markup + i18n as is).

- T2 done in `6c92f1f`: new `frontend/src/radioGroup.ts` (select-like `value`/`disabled`/`replaceOptions`, roving tab stop, arrows move + select with wrap, Home/End; setting `value` does not fire onChange), `#media-type` is now the radiogroup (labelled by the visible "Media type" label via `aria-labelledby`) at the top of the Download input column; `ui.mediaType.value` readers unchanged. Two-column `#panel-download` (`.panel.split`, input column on a surface, progress/results/footer right), `--accent-soft` (light + dark), `#app[data-tab="print"] #media-type` removed, narrow-window block moved to the end of `style.css` so it overrides the new rules. Tests: `radioGroup.test.ts`; `main.test.ts` media type block (click, arrows, "Cards for" follows). RED: `main.test.ts` beforeAll (no radios), `radioGroup.test.ts` (module missing); GREEN: 97/97.

- T3 done in `20e2c1b`: `#panel-print` split into a 300 px `#print-options` column (fieldsets `print-group-covers|layout|page|lettering`, legends `print.groupCovers|Layout|Page|Lettering` EN + ES) and the toolbar + gallery + footer column; every element id and `title`/`data-i18n-*` kept. `print.ts`: `syncFieldsets()` at the end of `updateFormat`'s visibility rules hides a fieldset whose `.field`s are all hidden (Lettering shows only for Full page / Checklist); `updateFormat` rules unchanged. Gallery: first built as `repeat(auto-fill, …)` (6 a row at 900 px, more when wider); since the app starts maximised, the parent pinned it to `repeat(6, minmax(0, 1fr))` in `5bd1c5a`; cards: 2 px accent ring when selected, CSS-only round check badge (`--badge-ring`, `--badge-off`, light + dark), 2:3 thumbnail box, checkbox kept but visually hidden (focus shown by the card outline). Credits page is now box-then-label. Tests: `print.test.ts` "option groups" (legend/field grouping, Lettering per format for all 7 formats with full fixtures, generic all-hidden group). RED: 9 `print.test.ts > option groups` tests; GREEN: 106/106. Headless Chrome renders of static copies checked at 900, 1000/1400 and 680 px, light and dark; badge needed `z-index: 1` (the image's drop-shadow filter painted over it).
- Checks after T3: `tsc --noEmit` clean; vitest 106/106; `go test -tags webkit2_41 ./...` 1329 passed; `git diff c774745 --stat -- '*.go' frontend/wailsjs` empty.

- Parent: `5bd1c5a` style(ui): keep six gallery cards a row (the app starts maximised, so 6 a row is the default view). Spot check: vitest 106/106, `tsc --noEmit` clean.
- T4 done: page-stream SHA-256 of 324 combos (every format × paper × size × cut, every crop mode for Cover sheet; header, date label and credits set; 4 mixed-shape fixtures) at `main` (`git archive`) vs `5bd1c5a` via `go test -overlay` scratch test: identical. `go test -tags webkit2_41 ./...` 1329 passed. `git diff main --stat -- '*.go' frontend/wailsjs` empty. `wails build -tags webkit2_41` built `build/bin/bujo-helper`; binding modes reset to 644. ROADMAP item 29 notes the done layout bullets and what remains.
- Review `64a6919..5bd1c5a` (medium, 13 files, 1157 lines, `slice_budget_reached`): consent granted; reliability lens approved; acknowledged (authority burned). Advisory R3-gallery-columns-claim WARNING at this doc's T3 entry (it described the replaced auto-fill rule); fixed in this doc. Next reviewed boundary: `5bd1c5a`.
- "Sources tried" line skipped: `MediaTypeInfo` has no provider names (Go change needed).

## Next step
Done. GUI check passed (user, 2026-11-20); merged to main in 5e7d2d3 and pushed.
