# Remember Print covers and Make pages options

## Objective
The Print covers options and the shared Make pages options survive an app restart (follow-up item 2 from the ranked review follow-up list; source places.md:65/76).

## Problem / why
Every option in both tabs is an in-memory closure or DOM value, re-seeded from the Go defaults at each launch (`frontend/src/print.ts:189`, `frontend/src/pages.ts:54`). Users re-pick paper, format, font and the credits checkbox every time. The only remembered state is the UI language in `settings.json`.

## Scope (user decision 2026-11-23: "Print + shared Pages")
- Print covers: format, cards-for media (review cards; the media id, only once the user picked one), size, crop, paper, cut, credits, header, font, colour-in (the user's choice, `options().colourIn`, not `box.checked`).
- Make pages: last kind, paper, cut, title, font, colour-in.
- Out: folders, the Gallery/Preview view, each page kind's own form options, month/year.

## Constraints / design
- Storage: `internal/settings.Settings` gains `print` and `pages` sub-objects (omitempty; older files load with zero values). Same atomic save (temp + fsync + rename).
- `SaveSettings` (Settings dialog) replaces the whole struct with six fields only, so it must keep the stored print/pages prefs. New bindings `SavePrintPrefs` and `SavePagesPrefs` merge one sub-object under `settingsMu`. `GetSettings` returns them for startup.
- Restore after the Go lists load (`loadPresets`, `print.ts:723`; pages kinds/papers/cuts/fonts). A saved id not offered falls back to the default; empty prefs leave today's defaults (credits stays on by default).
- Save on each user change; free text (header, title) saves on `change`/debounced input, not every keystroke. A failed save is logged, never shown as an error.
- IDs are saved, never labels (i18n re-labels). Header/title keep the 120-char limit.

## Tasks
- [x] T1 Go: settings sub-structs + `Trimmed`, `SaveSettings` keeps prefs, `SavePrintPrefs`/`SavePagesPrefs` bindings, regenerated Wails bindings, Go tests. Route: delegated writer.
- [x] T2 Print covers: restore saved prefs after presets load with fallback, save on change, vitest tests. Route: delegated writer.
- [ ] T3 Make pages: restore kind + shared fields, save on change, vitest tests. Route: delegated writer.
- [ ] T4 README: what is remembered and where. Route: same writer.

Route evidence: writer trigger (Go settings + app + bindings + print.ts + pages.ts + fieldsets/lettering + tests).

## Acceptance
- Restarting shows the last Print covers options and the last Make pages kind and shared fields.
- Saving the Settings dialog doesn't wipe them.
- A stale or unknown saved id falls back to the default; export never ends up disabled by a restored value.
- Checks: `go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l internal app.go cmd`, `cd frontend && npx tsc --noEmit && npx vitest run && npm run build`.

## Delivery
Strategy: `ask-on-risk`. Forecast ~700 authored lines (about half tests).

## Progress
- 2026-11-23: branch `feat/remember-print-options`; doc written.
- 2026-11-23 T1 (delegated writer): `settings.PrintPrefs`/`PagesPrefs` (`print`/`pages`, `omitzero`; `Credits *bool`, nil = default on; header/title capped at `MaxTextLen` = 120 runes), `SaveSettings` keeps them, `SavePrintPrefs`/`SavePagesPrefs` merge under `settingsMu` via `saveSettingsLocked`; bindings regenerated (modes 644, runtime reverted). RED: `go test ./internal/settings/ ./` build failed (undefined `PrintPrefs`, `SavePrintPrefs`). GREEN: both packages ok. Checks: go build/vet ok, go test 3460 passed, gofmt empty, tsc ok, vitest 368/368, npm run build ok. Commit `348bb3f`.
- 2026-11-24 T2 (delegated writer): new `frontend/src/prefs.ts` (`restoreSelect`, `prefsSaver`: starts after restore, skips unchanged values, one save at a time with the latest last, failures `console.warn`); `SharedFieldsets.restore` and `Lettering.restore` (user Colour-in choice kept, unlisted font keeps the shown one); Print covers restores in `loadPresets` before `updateFormat`, media only once offered (sets `mediaChosen`), saves on every option change and the header on `change`; `main.ts` keeps the startup `GetSettings` result and passes `savedPrefs`. RED: `prefs.test.ts` failed to resolve `./prefs`; print.test 5 failed / 58 passed; main.test 1 failed; lettering.test 3 failed without `restore`. GREEN: vitest 385/385. Checks: tsc ok, npm run build ok, go build/vet ok. Commit: recorded in the T3 entry.

## Next step
T1–T4 via one delegated writer.
