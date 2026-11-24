# Configurable sections and theme

## Objective
The Settings dialog lets the user pick a theme (System / Light / Dark) and choose, with checkboxes, which parts of the app they see: Download (media types), Print (always shown: paper, format, cover size, crop, cut) and Make pages (page kinds).

## Problem / why
Every user sees every media type, print format, paper and page kind, though most use a few (a European user never prints on US Letter). The theme only follows the system.

## Scope (user decisions 2026-11-24)
- Theme: System (default, today's behaviour) / Light / Dark radios.
- Download and Make pages can be turned off entirely; Print is always shown.
- Lists only: Download → media types; Print → Paper, Format, Cover size, Crop, Cut; Make pages → page kinds. No sub-features (CSV import, Wikiquote search, …).
- Paper and Cut are shared: the lists under Print also filter Make pages.
- Every list requires at least one ticked item. Each section and list has an all/none toggle.
- Sections and items added later must appear in the configuration.
- Out: new functionality; Print's "Cards for" keeps every media type (it is Print's own selector).

## Constraints / design
- Settings save hidden IDs (`theme`, `view.hiddenSections`, `view.hidden[group]`), so new items show up ticked. `omitempty`/`omitzero`: older files load with everything shown.
- `Trimmed`: theme ""/light/dark; "print" never hidden; IDs trimmed and deduplicated; empty lists dropped.
- `SaveSettings` keeps the remembered Print/Pages prefs; theme and view come from the dialog.
- `frontend/src/view.ts` is the one registry of sections and their lists; a guard test fails when a tab has no entry.
- A list whose items are all hidden (stale or hand-edited file) shows its default, so no list is ever empty.
- Changes apply live after Save (no reload). A hidden current tab moves to the first shown tab.
- CSS uses the existing tokens; `data-theme` overrides `prefers-color-scheme`.
- IDs are saved, never labels.

## Tasks
- [ ] T1 Go: `Theme` and `ViewPrefs` in settings with `Trimmed` rules; `SaveSettings` passes them; Wails bindings regenerated; Go tests. Route: inline (two Go files plus mechanical test updates; no research).
- [ ] T2 Theme: CSS `data-theme` selectors, `theme.ts`, Appearance radios in Settings, applied at startup and on save, i18n, vitest. Route: delegated writer (CSS + index.html + settings.ts + main.ts + i18n + tests).
- [ ] T3 View model: `view.ts` registry, `sectionShown`, `visibleItems` with default fallback, unit tests, tab-registry guard test. Route: inline (one new module + its test).
- [ ] T4 Sections tree in Settings: `viewConfig.ts`, dialog markup, tri-state all/none, required minimum, disabled off sections, save sends `view`, i18n, tests. Route: delegated writer.
- [ ] T5 Apply visibility: `tabs.setHidden`, filtering in `main.ts`, `print.ts` and `pages.ts` with fallbacks, live through `onChange`, tests. Route: delegated writer.
- [ ] T6 Docs: README theme line and settings section; ROADMAP note on the `view.ts` registry. Route: inline.

## Acceptance
- Light and Dark override the system theme; System follows it.
- Turning Download off hides its tab; the app opens on Print. Turning Make pages off and back on keeps its kind choices.
- Unticking Letter hides it in Print and Make pages.
- The last ticked item of a list can't be unticked; a list's "none" leaves the default ticked.
- Settings saved by the previous version load with everything shown.
- Checks: `go build ./...`, `go vet ./...`, `gofmt -l internal app.go cmd`, `go test -tags webkit2_41 ./...`, `cd frontend && npx tsc --noEmit && npx vitest run && npm run build`.

## Delivery
Strategy: `exception-ok` — one feature branch, tried by the user, then merged to main locally (as in earlier features). Forecast ~900 authored lines (about half tests).

## Progress
- 2026-11-24: branch `feat/configurable-sections`; doc written.

## Next step
T1.
