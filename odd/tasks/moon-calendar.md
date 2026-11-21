# Feature: moon-calendar (roadmap item 18)

## Objective
A generated **Moon calendar** page kind in Make pages: one month or a whole year of moon phases from a local astronomical calculation, with new and full moon marked. Three layouts (Row, Grid, Year), a hemisphere option and the shared Colour-in toggle for outline-only moons.

## Problem / Why
Moon phase trackers are a common bujo spread. They need no network and no licensing; drawing 30 correct moons by hand is tedious.

## Decisions
- Kind id `moon`, label "Moon calendar", file tag `moon`. `UsesTitle: true`, `UsesCut: false`.
- Options (JSON): `month` (1–12), `year` (1900–2100), `layout` (`row` | `grid` | `year`), `hemisphere` (`north` | `south`). Defaults `{month: 0, year: 0, layout: "grid", hemisphere: "north"}`: 0 means the current month / year, resolved when decoding through an injectable clock; the form always sends explicit values (current month/year when the default is 0).
- Astronomy: Meeus, *Astronomical Algorithms* ch. 49 (true new/full moon times, with the periodic corrections), plus the lunation fraction between consecutive true new moons for each day's drawn phase. No new dependency.
- Days are calendar days in the computer's local time zone (`time.Local`, injectable for tests); an event is marked on the local date it falls on.
- Drawing: limb half-circle + terminator half-ellipse (`rx = r·|cos 2πp|`) as a vector path. Filled: dark disc `#3b4252`, lit part white, thin outline (canvas). Colour-in: no fills, disc outline + terminator only. South hemisphere mirrors the moon horizontally.
- Title: the shared Title (Lettering fieldset); empty → a localized default, "<Month> <Year>" for Row/Grid and "<Year>" for Year. Drawn with the lettering font (hollow with Colour-in). Weekday/month labels, day numbers and New/Full marks stay in the Go font.
- Labels (month and weekday names, New/Full marks, default title) come from the app's UI language (`i18n`), as the Print covers labels do. Grid weeks start on Monday.
- Layouts follow the canvas (Make pages artboard): Row = one line per day (day number + weekday, moon, mark) in two columns (1–16, 17–end); Grid = 7-column month grid with weekday headers, day number top-left, moon centred, mark below; Year = 12 month rows × days, month label left, day-number header row.
- Form (canvas): Month select + Year number input on one row, Layout segmented (Row | Grid | Year; Month hidden/disabled for Year), Hemisphere segmented (North | South).

## Scope / constraints
- Extend, don't replace: registry entry in `internal/sheet.PageKinds`, form in `frontend/src/pageForms.ts`; bindings unchanged.
- Dot grid and Print covers output unchanged.
- EN + ES strings for every label.
- Out of scope: quarter-moon marks, eclipses, blue moons, week start choice (item 28).

## Acceptance criteria
- New/full moon dates match a published reference table (USNO, test year 2024) within ±1 day (event times within a few hours, tested).
- The PDF renders all three layouts on every paper size; no moon or label leaves the page; Colour-in draws no fills; hemisphere mirrors.
- Table-driven Go tests; frontend tests for the form.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `wails build -tags webkit2_41`; reset 0644 modes on regenerated bindings; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/moon-calendar` from `7e576c0`. Forecast ~1300 authored lines (≈45 % tests). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `7e576c0`.

## Tasks
- [x] T1 Go backend: `internal/sheet/moon.go` (Meeus phases, per-day phase, local dates), `moonpage.go` (kind, options, three layouts, drawing, labels via i18n), registry entry, app wiring of labels; tests: reference table 2024, options decode/validate, layout geometry within margins per paper and layout, Colour-in no fills, hemisphere mirror, default title. Route: delegated writer (writer trigger: 3+ non-trivial Go files).
- [ ] T2 Make pages form: Month / Year / Layout / Hemisphere controls, defaults to current month/year, i18n EN/ES, vitest. Route: delegated writer (writer trigger: form + i18n + tests).
- [ ] T3 Docs + verify: README Print sheet / Make pages section (algorithm, time zone, accuracy), ROADMAP item 18 notes, full checks, `wails build`. Route: delegated writer + inline state checks.

## Progress
- Feature doc created; branch `feat/moon-calendar` from `7e576c0`.
- T1 done (delegated writer). Commits `86f7cc8` feat(sheet): moon phase calculation; `2819df6` feat(sheet): moon calendar page kind.
  - RED: `moon_test.go` and `moonpage_test.go` first failed to build (undefined `moonEvents`/`moonMonth`/`deltaT`, `PageLabels`/`MoonKind`/`moonSheet`). Behavioural REDs on the way: the local-date test (fixture zone fixed to UTC+2), the full-moon terminator expectation (test corrected: from p = 0.5 the lit limb is the left one), and the labels-only-regular-font check (impl fixed: the moon kind embeds only the regular Go face).
  - GREEN: `go test -tags webkit2_41 ./...` 1765 passed; `go vet` clean; `npx tsc --noEmit` clean after `wails generate module` (models.ts gains `sheet.PageLabels`; modes reset to 0644).
  - Accuracy: USNO 2024 table re-fetched from https://aa.usno.navy.mil/api/moon/phases/year?year=2024 (matches the brief); all 25 events within 1 minute (test asserts 10 minutes and the exact UTC dates).
  - Decisions: ΔT from the Espenak–Meeus polynomials (1900–2150); labels live in `sheet.PageLabels` as `PageSettings.Labels` (`json:"-"`, like `Options.Labels`), filled by `App.withPageLabels` in PreviewPage/ExportPage from i18n keys `pdf.months`/`pdf.weekdays` (comma lists), `pdf.moonNew`/`pdf.moonFull`; English fallback per entry. Default title via an optional `defaultTitler` interface in `buildPage`; `pageDoc.draw` gained a `labels` parameter (dot grid ignores it, output unchanged). Year layout marks new/full moon with the mark's initial (N/F, ES N/L) under the moon; short month names are the first three letters. Row: 16 rows ≤ 14 mm; Grid cells ≤ 1.25 × width; Year rows ≤ 3 cells tall; blocks centred below the title band.
  - Out-of-surface edit: `preview_test.go` unknown-kind fixture `"moon"` → `"starmap"` (also in pages_test/app_test), forced by `moon` becoming a valid kind.
- Next: T2 Make pages form.
