package sheet

import "time"

// Calendar math shared by the month calendar and the habit tracker. The
// number of days in a month is daysIn (moon.go), which the moon calendar
// already uses.

// Supported years of a month calendar or habit tracker.
const (
	MinCalendarYear = 1900
	MaxCalendarYear = 2100
)

// monthWeeks lays out month of year in week rows of 7 days, the first
// column being weekStart: each cell is its day of the month, or 0 for the
// empty cells before the first day and after the last. A month takes 4 to
// 6 rows.
func monthWeeks(year int, month time.Month, weekStart time.Weekday) [][7]int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Weekday()
	offset := (int(first) - int(weekStart) + 7) % 7
	n := daysIn(year, month)
	weeks := make([][7]int, (offset+n+6)/7)
	for day := 1; day <= n; day++ {
		pos := offset + day - 1
		weeks[pos/7][pos%7] = day
	}
	return weeks
}

// weekdaysFrom returns the 7 weekday names, given Monday first, starting
// from weekStart (Monday or Sunday).
func weekdaysFrom(mondayFirst []string, weekStart time.Weekday) []string {
	out := make([]string, 0, 7)
	// Monday is index 0, Sunday index 6.
	start := (int(weekStart) + 6) % 7
	for i := 0; i < 7; i++ {
		out = append(out, mondayFirst[(start+i)%7])
	}
	return out
}
