package sheet

import (
	"math"
	"testing"
	"time"
)

// moonRef is a new or full moon of a reference table.
type moonRef struct {
	event moonEvent
	when  string // UTC, "2006-01-02 15:04"
}

// usno2024 are the new and full moons of 2024 in UTC, from the U.S. Naval
// Observatory (https://aa.usno.navy.mil/api/moon/phases/year?year=2024,
// fetched 2026-11-21), rounded to the minute.
var usno2024 = []moonRef{
	{newMoonEvent, "2024-01-11 11:57"}, {fullMoonEvent, "2024-01-25 17:54"},
	{newMoonEvent, "2024-02-09 22:59"}, {fullMoonEvent, "2024-02-24 12:30"},
	{newMoonEvent, "2024-03-10 09:00"}, {fullMoonEvent, "2024-03-25 07:00"},
	{newMoonEvent, "2024-04-08 18:21"}, {fullMoonEvent, "2024-04-23 23:49"},
	{newMoonEvent, "2024-05-08 03:22"}, {fullMoonEvent, "2024-05-23 13:53"},
	{newMoonEvent, "2024-06-06 12:38"}, {fullMoonEvent, "2024-06-22 01:08"},
	{newMoonEvent, "2024-07-05 22:57"}, {fullMoonEvent, "2024-07-21 10:17"},
	{newMoonEvent, "2024-08-04 11:13"}, {fullMoonEvent, "2024-08-19 18:26"},
	{newMoonEvent, "2024-09-03 01:55"}, {fullMoonEvent, "2024-09-18 02:34"},
	{newMoonEvent, "2024-10-02 18:49"}, {fullMoonEvent, "2024-10-17 11:26"},
	{newMoonEvent, "2024-11-01 12:47"}, {fullMoonEvent, "2024-11-15 21:28"},
	{newMoonEvent, "2024-12-01 06:21"}, {fullMoonEvent, "2024-12-15 09:02"},
	{newMoonEvent, "2024-12-30 22:27"},
}

func refTime(t *testing.T, s string) time.Time {
	t.Helper()
	when, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		t.Fatal(err)
	}
	return when
}

// The true new and full moons of 2024 fall within 10 minutes of USNO's.
func TestMoonEvents2024MatchUSNO(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	got := moonEvents(from, from.AddDate(1, 0, 0))
	if len(got) != len(usno2024) {
		t.Fatalf("%d events in 2024, want %d: %v", len(got), len(usno2024), got)
	}
	for i, ref := range usno2024 {
		want := refTime(t, ref.when)
		if got[i].Event != ref.event {
			t.Errorf("event %d (%s): kind %v, want %v", i, ref.when, got[i].Event, ref.event)
		}
		if d := got[i].Time.Sub(want); d.Abs() > 10*time.Minute {
			t.Errorf("event %d: %s, want %s (off by %v)", i, got[i].Time.Format(time.DateTime), ref.when, d)
		}
	}
}

// Each month marks the reference events on their UTC dates, and no other
// day.
func TestMoonMonthMarksTheReferenceDates(t *testing.T) {
	want := map[time.Time]moonEvent{}
	for _, ref := range usno2024 {
		when := refTime(t, ref.when)
		want[time.Date(when.Year(), when.Month(), when.Day(), 0, 0, 0, 0, time.UTC)] = ref.event
	}
	for m := time.January; m <= time.December; m++ {
		days := moonMonth(2024, m, time.UTC)
		if n := daysIn(2024, m); len(days) != n {
			t.Fatalf("%v: %d days, want %d", m, len(days), n)
		}
		for i, d := range days {
			date := time.Date(2024, m, i+1, 0, 0, 0, 0, time.UTC)
			if d.Day != i+1 {
				t.Errorf("%s: day %d", date.Format(time.DateOnly), d.Day)
			}
			if d.Event != want[date] {
				t.Errorf("%s: event %v, want %v", date.Format(time.DateOnly), d.Event, want[date])
			}
		}
	}
}

// An event is marked on the local date it falls on: the new moon of
// 9 February 2024 at 22:59 UTC is on the 10th two hours east of Greenwich.
func TestMoonMonthUsesTheLocalDate(t *testing.T) {
	eet := time.FixedZone("EET", 2*3600)
	tests := []struct {
		loc  *time.Location
		want int
	}{
		{time.UTC, 9},
		{eet, 10},
	}
	for _, tt := range tests {
		var marked []int
		for _, d := range moonMonth(2024, time.February, tt.loc) {
			if d.Event == newMoonEvent {
				marked = append(marked, d.Day)
			}
		}
		if len(marked) != 1 || marked[0] != tt.want {
			t.Errorf("%s: new moon on %v, want [%d]", tt.loc, marked, tt.want)
		}
	}
}

// The drawn phase grows through each lunation, from about 0 on the day of
// a new moon to about 0.5 on the day of a full moon, and wraps to 0 once
// per new moon.
func TestMoonMonthPhase(t *testing.T) {
	var prev float64 = -1
	wraps := 0
	for m := time.January; m <= time.December; m++ {
		for _, d := range moonMonth(2024, m, time.UTC) {
			date := time.Date(2024, m, d.Day, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)
			if d.Phase < 0 || d.Phase >= 1 {
				t.Fatalf("%s: phase %v out of [0, 1)", date, d.Phase)
			}
			if prev >= 0 && d.Phase < prev {
				wraps++
				if prev < 0.9 || d.Phase > 0.1 {
					t.Errorf("%s: phase fell from %v to %v mid-lunation", date, prev, d.Phase)
				}
			}
			prev = d.Phase
			switch d.Event {
			case newMoonEvent:
				// Noon is at most half a day (1/59 lunation) from the event.
				if dist := math.Min(d.Phase, 1-d.Phase); dist > 0.02 {
					t.Errorf("%s: new moon drawn at phase %v", date, d.Phase)
				}
			case fullMoonEvent:
				// The full moon is not exactly mid-lunation.
				if math.Abs(d.Phase-0.5) > 0.05 {
					t.Errorf("%s: full moon drawn at phase %v", date, d.Phase)
				}
			}
		}
	}
	// 13 new moons in 2024, but the year starts mid-lunation: 13 wraps.
	if wraps != 13 {
		t.Errorf("%d wraps in 2024, want 13", wraps)
	}
}

// The calculation stays sane across the supported years: lunations
// 29.27–29.84 days long, full moons between them.
func TestMoonEventsAcrossTheRange(t *testing.T) {
	for _, year := range []int{1900, 1950, 2000, 2050, 2100} {
		from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		events := moonEvents(from, from.AddDate(1, 0, 0))
		if len(events) < 24 || len(events) > 26 {
			t.Errorf("%d: %d events", year, len(events))
		}
		for i := 1; i < len(events); i++ {
			if events[i].Event == events[i-1].Event {
				t.Errorf("%d: two %v in a row at %v", year, events[i].Event, events[i].Time)
			}
			if i >= 2 {
				lunation := events[i].Time.Sub(events[i-2].Time).Hours() / 24
				if lunation < 29.2 || lunation > 29.9 {
					t.Errorf("%d: lunation of %.2f days at %v", year, lunation, events[i].Time)
				}
			}
		}
	}
}

func TestDeltaT(t *testing.T) {
	tests := []struct {
		year     float64
		min, max float64 // seconds
	}{
		{1900, -4, 0},
		{1950, 28, 31},
		{2000, 63, 65},
		{2024, 68, 76},
		{2100, 150, 250},
	}
	for _, tt := range tests {
		if got := deltaT(tt.year); got < tt.min || got > tt.max {
			t.Errorf("deltaT(%v) = %v s, want %v..%v", tt.year, got, tt.min, tt.max)
		}
	}
}
