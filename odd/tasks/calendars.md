# Feature: calendars (roadmap item 28)

## Objective
Two generated page kinds in Make pages: a **Month calendar** (any month/year, week start Monday or Sunday) and a **Habit tracker** (habit list × the days of the month grid), sized for every paper the app supports (A5, B5, A4, Letter).

## Problem / Why
Monthly calendars and habit trackers are core bujo pages. Drawing a correct grid by hand each month is tedious.

## Decisions
- Kind ids `calendar` (label "Month calendar", file tag `calendar`) and `habits` (label "Habit tracker", file tag `habits`); both `UsesTitle: true`, `UsesCut: false`. Registered after `moon`, before `map`, as the canvas lists them.
- Calendar options: `month` (1–12), `year` (1900–2100), `weekStart` (`mon` | `sun`). Defaults `{month: 0, year: 0, weekStart: "mon"}`; 0 means current, resolved in Go through an injectable clock (as moon). A clock year outside 1900–2100 is an error.
- Habits options: `month`, `year` (as calendar), `habits` ([]string, one per row; blank entries dropped, trimmed; at most 40 habits of at most 60 runes each, else an error). Defaults `{month: 0, year: 0, habits: []}`. No habits → blank rows filling the page (to write habits by hand).
- Shared calendar math in `internal/sheet/calendar.go`: days in month (leap years), the month's week rows for a week start. Moon calendar code and output are unchanged.
- Default titles (empty shared Title): calendar "<Month> <Year>"; habits "Habits · <Month>" (ES "Hábitos · <Mes>"), via `defaultTitler`. Title drawn with the lettering font, hollow with Colour-in; weekday headers, day numbers and habit names stay in the Go font.
- Calendar layout (canvas): 7-column grid, weekday headers (Sunday-first rotation for `sun`), day number top-left in each cell, 5 or 6 week rows filling the page below the title band; empty leading/trailing cells have no box.
- Habits layout (canvas): habits as rows, days as columns; a day-number header row; a habit-name column (ellipsized to fit); square boxes sized to the width (capped), rows paginate with the title repeated when they do not fit.
- Form (canvas): calendar = Month select + Year input row, "Week starts on" segmented (Monday | Sunday); habits = Month + Year row, "Habits, one per line" textarea. Reuses the moon form's month/year helpers.

## Scope / constraints
- Extend, don't replace: entries in `PageKinds`, forms in `pageForms.ts`; bindings unchanged.
- Dot grid, moon, map and Print covers output unchanged.
- EN + ES strings for every label.
- Out of scope: weekly spreads, yearly calendar, habit tracker layout with days as rows, holidays.

## Acceptance criteria
- Calendar dates correct for leap years (2024, 2000, 1900, 2100) and both week starts (unit tests).
- The PDF renders both kinds on every paper size; nothing leaves the page margins; long habit names never overflow their column.
- Table-driven Go tests; frontend tests for both forms.

## Checks
- `go test -tags webkit2_41 ./...`, `go vet -tags webkit2_41 ./...`
- `cd frontend && npx tsc --noEmit && npx vitest run`
- `wails build -tags webkit2_41`; reset 0644 modes on regenerated bindings; delete `frontend/.vitest/`.

## Delivery
- Branch `feat/calendars` from `7ad9de8`. Forecast ~1500 authored lines (≈45 % tests). Strategy `single-pr` (cached project choice: user GUI-checks, then local `--no-ff` merge).
- RDD on (global). First reviewed boundary: `7ad9de8`.

## Tasks
- [x] T1 Go backend: `calendar.go` (days in month, week rows), `calendarpage.go` (kind `calendar`), `habitspage.go` (kind `habits`), registry entries, new labels (habits default title) wired from i18n in the app; tests. Route: delegated writer (writer trigger: 3+ non-trivial Go files).
- [ ] T2 Make pages forms for both kinds, i18n EN/ES, vitest. Route: delegated writer (writer trigger: form + i18n + tests).
- [ ] T3 Docs + verify: README Make pages sections, ROADMAP item 28 notes, full checks, `wails build`. Route: delegated writer + inline state checks.

## Progress
- Feature doc created; branch `feat/calendars` from `7ad9de8`.
- T1 done (delegated writer). Commits: `a57b5fa feat(sheet): calendar math`, `7d8ffdb feat(sheet): month calendar page kind` (includes the parent's `mappage_test.go` fix: the map is last; full order in `TestCalendarRegistryEntry`), `d7a6b6c feat(sheet): habit tracker page kind`. Each commit builds and passes `go test`/`go vet` on its own (calendar commit: 2495 passed).
  - RED: `go test ./internal/sheet -run 'TestDaysInMonth|TestMonthWeeks|TestWeekdaysFrom'` → build failed (undefined `monthWeeks`, `MinCalendarYear`, `weekdaysFrom`); `-run Calendar` → undefined `CalendarKind`, `CalendarOptions`, `gridSheet`, `calendarNow`; `-run Habits` → undefined `HabitsOptions`, `HabitsKind`, `MaxHabits`.
  - GREEN: `go test -tags webkit2_41 ./...` → 2580 passed in 18 packages; `go vet -tags webkit2_41 ./...` → no issues; `cd frontend && npx tsc --noEmit` → no errors; `git diff --summary 7ad9de8..HEAD | grep "mode change"` → empty (regenerated bindings reset to 0644; only `models.ts` changed, gaining `HabitsTitle`).
  - Decisions: `daysIn` (moon.go) is reused as the shared days-in-month helper (moon.go outside the surface; no duplicate). Calendar cells are capped at 1.5 × their width and the grid is centred below the title, like the moon grid; borders 0.2 mm in the writing grey; day numbers 9 pt, headers 8 pt. Habits: name column fits the longest name between 20 mm and 30 % of the width (30 % with no habits) and takes any width the boxes leave; boxes `min(rest/days, 6 mm)`; rows `max(1.5 × box, 6 mm)`, top-aligned; day numbers 6 pt shrinking to 4 pt to fit; names 7 pt cut with "…" (`fitText`). Page count depends on the paper: new optional `paperPager.pagesOn(Page)` in `pages.go`, used by `PageCount`; habits' `pages()` reports A4. `PageLabels.HabitsTitle` is "Habits · {month}" (ES "Hábitos · {month}"), substituted with `strings.ReplaceAll`, never a format string.
- Next: T2 (Make pages forms for both kinds). RDD review of the T1 commits not yet run (parent-owned).
