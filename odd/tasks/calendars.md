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
- [x] T2 Make pages forms for both kinds, i18n EN/ES, vitest. Route: delegated writer (writer trigger: form + i18n + tests).
- [x] T3 Docs + verify: README Make pages sections, ROADMAP item 28 notes, full checks, `wails build`; T1 review advisories R3-habit-control-chars (reject or normalise control characters in habit names, with a test) and R3-habits-pages-swallows-error (`HabitsOptions.pages()` drops the `pagesOn` error and can return 0). Route: delegated writer + inline state checks.

## Progress
- Feature doc created; branch `feat/calendars` from `7ad9de8`.
- T1 done (delegated writer). Commits: `a57b5fa feat(sheet): calendar math`, `7d8ffdb feat(sheet): month calendar page kind` (includes the parent's `mappage_test.go` fix: the map is last; full order in `TestCalendarRegistryEntry`), `d7a6b6c feat(sheet): habit tracker page kind`. Each commit builds and passes `go test`/`go vet` on its own (calendar commit: 2495 passed).
  - RED: `go test ./internal/sheet -run 'TestDaysInMonth|TestMonthWeeks|TestWeekdaysFrom'` → build failed (undefined `monthWeeks`, `MinCalendarYear`, `weekdaysFrom`); `-run Calendar` → undefined `CalendarKind`, `CalendarOptions`, `gridSheet`, `calendarNow`; `-run Habits` → undefined `HabitsOptions`, `HabitsKind`, `MaxHabits`.
  - GREEN: `go test -tags webkit2_41 ./...` → 2580 passed in 18 packages; `go vet -tags webkit2_41 ./...` → no issues; `cd frontend && npx tsc --noEmit` → no errors; `git diff --summary 7ad9de8..HEAD | grep "mode change"` → empty (regenerated bindings reset to 0644; only `models.ts` changed, gaining `HabitsTitle`).
  - Decisions: `daysIn` (moon.go) is reused as the shared days-in-month helper (moon.go outside the surface; no duplicate). Calendar cells are capped at 1.5 × their width and the grid is centred below the title, like the moon grid; borders 0.2 mm in the writing grey; day numbers 9 pt, headers 8 pt. Habits: name column fits the longest name between 20 mm and 30 % of the width (30 % with no habits) and takes any width the boxes leave; boxes `min(rest/days, 6 mm)`; rows `max(1.5 × box, 6 mm)`, top-aligned; day numbers 6 pt shrinking to 4 pt to fit; names 7 pt cut with "…" (`fitText`). Page count depends on the paper: new optional `paperPager.pagesOn(Page)` in `pages.go`, used by `PageCount`; habits' `pages()` reports A4. `PageLabels.HabitsTitle` is "Habits · {month}" (ES "Hábitos · {month}"), substituted with `strings.ReplaceAll`, never a format string.
- Review `7ad9de8..6d4716a` (T1 + docs, medium, 14 files, 1537 lines; `slice_budget_reached`): consent granted; reliability lens approved; acknowledged (authority burned). Advisory SUGGESTIONs folded into T3: R3-habit-control-chars, R3-habits-pages-swallows-error. Parent relaxed `TestMapRegistryEntry` (map last only), committed in `7d8ffdb`. Parent spot check: `go test ./internal/sheet/` 1704 passed. Reviewed boundary `6d4716a`.
- T2 done (delegated writer). Commits: `88bf494 refactor(frontend): shared month and year fields` (new `frontend/src/monthYearField.ts` + tests; moon form uses it, moon tests unchanged and green; 282 vitest passed on that commit alone), `b9afcc9 feat(frontend): month calendar and habit tracker forms` (forms `calendar`, `habits` registered in `pageForms`; EN/ES strings; `.habits-field` shares the map textarea's UI-font CSS; `pages.test.ts` mocks both kinds after moon, before map).
  - RED: `npx vitest run` → 20 failed (calendar/habits form tests, pages integration, ES kind labels) and `monthYearField.test.ts` failed to import the missing module.
  - GREEN: `npx tsc --noEmit` → no errors; `npx vitest run` → 301 passed in 13 files; `go test -tags webkit2_41 ./internal/sheet/` → ok (map contract test still reads `pageForms.ts`).
  - Decisions: `monthYearField(container, idPrefix, defaults, onChange)` returns `{monthField, month(), year(), dispose()}`, keeps the moon's IDs (`<prefix>-month-field`, `-month`, `-year`) and year typing/commit rules, and relabels months itself; forms dispose both their own and its language listener. Calendar weekStart segmented `mon`|`sun` (unknown → `mon`), group ID `<prefix>-weekStart`. Habits limits: the form sends what the user typed (no clamping, no truncation) and shows a static muted hint "Up to 40 habits of 60 characters. Leave it empty to write them by hand."; over the limits, Go's error shows in the preview's existing error display. `habitList` splits on `\r\n`, `\r`, `\n`, trims, drops blanks; onChange fires only when the list changes. No new Go contract test: the 40/60 limits only appear in hint strings.
- Next: T3 (docs + verify + T1 review advisories). RDD assessment of the T2 commits is parent-owned.
- The broad stop-hook review (base `782ab17`, 96 files, 11916 lines): consent granted, but the frozen target went stale when the T2 commits landed, so no review ran. That scope also overlaps lettering-fonts, moon and map, which were already reviewed.
- Review `6d4716a..b8a6f76` (T2 + doc, medium, 9 files, 721 lines; `slice_budget_reached`): consent granted; reliability lens approved with no findings; acknowledged (authority burned). Parent spot check: `npx vitest run` 301 passed (13 files). Reviewed boundary `b8a6f76`.
- T3 done (delegated writer). Commits: `7750464 fix(sheet): normalise habit names and page count`, `1ae51d7 docs: month calendar and habit tracker in README and roadmap`.
  - RED: `go test -tags webkit2_41 ./internal/sheet -run 'TestDecodeHabitsOptions|TestHabitsControl|TestHabitsPagesOnA4'` → 4 failures: control characters kept (`"Read\tbooks"`), a habit of only control characters kept, a 61-rune name collapsing to 60 rejected, `"Read\tbooks"` printed with the tab. `TestHabitsPagesOnA4` passed already (happy path only).
  - GREEN: `go test -tags webkit2_41 ./...` → 2585 passed in 18 packages; `go vet -tags webkit2_41 ./...` → no issues; `npx tsc --noEmit` → no errors; `npx vitest run` → 301 passed in 13 files; `wails build -tags webkit2_41` → built `build/bin/bujo-helper`; it regenerated the three binding files with only a 0644 → 0755 mode change, reset to 0644 (nothing committed); `frontend/.vitest/` deleted; `git status --short` clean; `git diff --summary 7ad9de8..HEAD | grep "mode change"` → empty.
  - Decisions: R3-habit-control-chars — normalise, not reject: `normalizeHabit` turns each run of `unicode.IsControl` or `unicode.IsSpace` runes into one space and trims, before the length check and the measuring (a pasted list can carry invisible characters; an error naming one would confuse). Double spaces inside a name also collapse to one. R3-habits-pages-swallows-error — `pages()` returns 1 if `pagesOn(A4)` fails and `max(1, n)` otherwise, commented that `PageCount` uses `pagesOn` and returns its error; the failure path (the embedded Go font not loading) cannot be provoked in a test, so `TestHabitsPagesOnA4` pins `pages() == pagesOn(A4) >= 1` for 0, 1 and 40 habits. README: Month calendar and Habit tracker sections between Moon calendar and Visited map, Lettering "What uses the font" and Colour-in lines, Project layout `sheet/` line; measured 40 habits = 2 pages on A5, B5, A4 and Letter. The export-blocking claim was left out of the habits section (not verified). ROADMAP item 28 gains a "Done in branch" block; heading stays `[ ]`.
- Next: parent RDD assessment of `b8a6f76..` the T3 record commit (base: the last reviewed boundary `b8a6f76`), then the user's GUI check and the local `--no-ff` merge; add the merge commit to ROADMAP item 28 and check it.
