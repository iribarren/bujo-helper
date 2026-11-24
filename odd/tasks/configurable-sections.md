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
- [x] T1 Go: `Theme` and `ViewPrefs` in settings with `Trimmed` rules; `SaveSettings` passes them; Wails bindings regenerated; Go tests. Route: inline (two Go files plus mechanical test updates; no research).
- [x] T2 Theme: CSS `data-theme` selectors, `theme.ts`, Appearance radios in Settings, applied at startup and on save, i18n, vitest. Route: delegated writer (CSS + index.html + settings.ts + main.ts + i18n + tests).
- [x] T3 View model: `view.ts` registry, `sectionShown`, `visibleItems` with default fallback, unit tests, tab-registry guard test. Route: inline (one new module + its test).
- [x] T4 Sections tree in Settings: `viewConfig.ts`, dialog markup, tri-state all/none, required minimum, disabled off sections, save sends `view`, i18n, tests. Route: delegated writer.
- [x] T5 Apply visibility: `tabs.setHidden`, filtering in `main.ts`, `print.ts` and `pages.ts` with fallbacks, live through `onChange`, tests. Route: delegated writer.
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

- 2026-11-24 T1 (inline): `Settings.Theme` (""/light/dark) and `Settings.View` (`ViewPrefs{HiddenSections, Hidden}`, `omitzero`); `ViewPrefs.Trimmed` trims, deduplicates, never hides "print", drops empty lists (zero value when nothing hidden). `SaveSettings` already passes them through (doc updated). Settings is no longer comparable: 19 test comparisons now use `reflect.DeepEqual`. Bindings regenerated (runtime churn reverted, modes 644). RED: settings and root packages failed to build (undefined `ViewPrefs`, `Theme`). GREEN: go test 3549 passed; go vet clean; gofmt empty; tsc ok; vitest 467/467. Commit `8e0404d`. Review assess: medium, not due (`under_budget`), stays pending in the slice.

- 2026-11-24 T3 (inline): `frontend/src/view.ts` — `SECTIONS` registry (download: media; print: paper, format, size, crop, cut; pages: pageKind; list id = Go list key = i18n label kind), `sectionShown`, `hiddenIds`, `defaultItem`, `visibleItems` (keeps the default when all hidden), `shownDefault` (default if shown, else first shown). i18n `view.*` list titles (en/es). Guard test: `SECTIONS` ids equal the `data-tab`s in index.html, in order. RED: view.test 1 failed (list titles untranslated). GREEN: vitest 477/477, tsc ok. Commit `ab4a2a8`.

- Review `main..ab4a2a8` (T1+T3; medium, 11 files, 497 lines, `slice_budget_reached`): consent granted; reliability approved; acknowledged, lineage `review-967b8b0e29df4ddf`. Advisory follow-ups: R3-view-trim-merge-order (WARNING, `settings.go` Trimmed merge order random) fixed with sorted keys + test; R3-tab-guard-regex (guard depended on attribute order) now parses the DOM; R3-view-binding-undefined (`view` absent in older files although typed required) is handled by `View = ViewPrefs | undefined` and must stay so in T4/T5. Reviewed boundary: `ab4a2a8`.

- 2026-11-24 T2 (delegated writer): `theme.ts` `applyTheme` sets `data-theme` light/dark on the root, removes it otherwise (System). `style.css`: dark tokens under `:root[data-theme="dark"]` and `@media (prefers-color-scheme: dark) :root:not([data-theme="light"])` (two identical blocks, comment says they must match; checked with diff); `color-scheme` light dark / light / dark per theme; BGG logo swap follows the same selectors. Settings: Theme field right after Language, a `.segmented` radio group (`radioGroup.ts`, System "" / Light / Dark, relabelled on language change); `fill()` shows the saved theme, `save()` sends `theme` (value captured before the await) and applies it after a successful save; a failed save keeps the current theme. `main.ts` `startLanguage()` applies `startSettings.theme` right after `GetSettings`. i18n `settings.theme*` (en/es). RED: theme.test failed to load (no `theme.ts`), 7 tests failed (6 settings Theme, 1 main startup). GREEN: vitest 486/486 (18 files); tsc ok; `npm run build` ok. CSS: structural readback. 232+/13- lines. Commit `b74b9eb` (before amend; see git log).

- 2026-11-24 T4 (delegated writer): `viewConfig.ts` `initViewConfig(container, onChange)` builds the tree from `SECTIONS` and the Go lists: a `<fieldset>` per section (legend checkbox; Print checked, disabled, "Always shown") and per list (legend all/none checkbox, `indeterminate` when mixed; "none" keeps `defaultItem`), one labelled checkbox per item (`label(list.id, …)`). The last ticked item is disabled; an unticked optional section greys out (`.off`) and disables its lists, keeping their ticks. `load()` fetches the lists not loaded yet (a failed or non-array list shows "Could not load this list." and retries at the next open); `fill(view)` ticks via `sectionShown`/`visibleItems` (also for lists that load later); `read()` returns `ViewPrefs` with unticked optional sections and unticked IDs per loaded list (stale IDs dropped, empty entries omitted), keeping the filled view's IDs for unloaded lists. Settings: "Sections shown" part after Theme (`#settings-sections`, `#view-config`), lists loaded on open, `fill()` restores the saved view at every open, `save()` captures `view` before the await and sends it; a box change clears the status. TMDB field got `settings-section` spacing. CSS: indented hierarchy, item grid columns, muted notes; dialog `max-height` + `overflow-y: auto`. i18n `settings.sections`, `sectionsHint`, `alwaysShown`, `listUnavailable` (en/es). RED: viewConfig.test failed to load (no `viewConfig.ts`); settings.test 9 failed (no `#settings-sections`/`#view-config`). GREEN: vitest 508/508 (19 files); tsc ok; `npm run build` ok. CSS: structural readback. Commit `d9b9a7d`.

- Review `ab4a2a8..a18a0c3` (T2+T4; medium, 16 files, 1031 lines, `slice_budget_reached`): a first consent went stale (T4 was committed while it was asked; nothing started); asked again, consent granted; reliability approved; acknowledged, lineage `review-fcc51a6ca79d76ba`. Advisory follow-ups closed inline: R3-dark-token-drift — new theme.test guard that the two dark token blocks match (RED by mutating one token: 1 failed; restored: GREEN); R3-startup-theme-sync-assert — the startup theme test now waits (`vi.waitFor`). Reviewed boundary: `a18a0c3`.

- 2026-11-24 T5 (delegated writer): `view.ts` `hiddenSections(view)` and `chooseShown(view, list, items, wanted)` (first wanted id that is shown, else `shownDefault`). `tabs.ts` `setHidden(names)`: hidden tab buttons, arrows/Home/End skip them, a hidden selected tab moves to the first shown one (through `onSelect`), `select()` of a hidden tab is ignored. `fieldsets.ts`: `fillShown` replaces `fillSelect` (fills a select with the shown presets and picks via `chooseShown`); the shared Page fieldset keeps every Go paper and cut mode, takes an initial `view`, and `setView(view, fallback)` refills Paper and Cut (current, else remembered, else shown default); `restore` only restores a shown paper. `print.ts`: `PrintDeps.view`, `PrintSheet.applyView` refills format, size, crop, paper and cut, then `syncMedia` and `updateFormat`; startup fills the shown defaults before `restorePrefs`, so a hidden remembered option falls back. Cards for keeps every media type. `pages.ts`: `initPages` returns `PagesTab {applyView}` (kinds, paper, cut); kind forms keep their options while hidden. Both `applyView`s skip an unchanged view (JSON key) and never save prefs. `settings.ts`: `SettingsDialog.view()` (the loaded settings' view). `main.ts`: `applyView` hides tabs, filters the Download media types (keeps the choice while shown, else the shown default; `updateMediaType` only on change) and forwards to Print/Make pages; it runs right after `startLanguage` and in Settings `onChange` (startup, dialog open, after Save). With Download hidden the app opens on Print covers and loads its folder once the output folder is known (activation deferred to the end of `init`). RED: 28 failed + 2 skipped (view 3, tabs 5, settings 2, print 8, pages 7, main 3 + startup suite). GREEN: vitest 542/542 (19 files); tsc ok; `npm run build` ok. Commit: see report.

## Next step
T6 (inline).
