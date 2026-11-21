package sheet

import (
	"fmt"
	"testing"
	"time"
)

// Month lengths follow the Gregorian leap year rules: every fourth year,
// except centuries not divisible by 400.
func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2024, time.February, 29},
		{2023, time.February, 28},
		{2000, time.February, 29},
		{1900, time.February, 28},
		{2100, time.February, 28},
		{2024, time.January, 31},
		{2024, time.April, 30},
		{2024, time.December, 31},
	}
	for _, tt := range tests {
		if got := daysIn(tt.year, tt.month); got != tt.want {
			t.Errorf("daysIn(%d, %v) = %d, want %d", tt.year, tt.month, got, tt.want)
		}
	}
}

func TestMonthWeeks(t *testing.T) {
	tests := []struct {
		name      string
		year      int
		month     time.Month
		weekStart time.Weekday
		wantRows  int
		wantFirst [7]int // the first week row
		wantLast  [7]int // the last week row
	}{
		{
			// 1 September 2024 was a Sunday.
			"September 2024 from Monday", 2024, time.September, time.Monday, 6,
			[7]int{0, 0, 0, 0, 0, 0, 1}, [7]int{30, 0, 0, 0, 0, 0, 0},
		},
		{
			"September 2024 from Sunday", 2024, time.September, time.Sunday, 5,
			[7]int{1, 2, 3, 4, 5, 6, 7}, [7]int{29, 30, 0, 0, 0, 0, 0},
		},
		{
			// 1 February 2015 was a Sunday and the month has 28 days.
			"February 2015 from Sunday", 2015, time.February, time.Sunday, 4,
			[7]int{1, 2, 3, 4, 5, 6, 7}, [7]int{22, 23, 24, 25, 26, 27, 28},
		},
		{
			"February 2015 from Monday", 2015, time.February, time.Monday, 5,
			[7]int{0, 0, 0, 0, 0, 0, 1}, [7]int{23, 24, 25, 26, 27, 28, 0},
		},
		{
			// 1 February 2024 was a Thursday; 29 days.
			"February 2024 from Monday", 2024, time.February, time.Monday, 5,
			[7]int{0, 0, 0, 1, 2, 3, 4}, [7]int{26, 27, 28, 29, 0, 0, 0},
		},
		{
			// 1 April 2024 was a Monday.
			"April 2024 from Monday", 2024, time.April, time.Monday, 5,
			[7]int{1, 2, 3, 4, 5, 6, 7}, [7]int{29, 30, 0, 0, 0, 0, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weeks := monthWeeks(tt.year, tt.month, tt.weekStart)
			if len(weeks) != tt.wantRows {
				t.Fatalf("%d week rows, want %d: %v", len(weeks), tt.wantRows, weeks)
			}
			if weeks[0] != tt.wantFirst || weeks[len(weeks)-1] != tt.wantLast {
				t.Errorf("weeks %v, want first %v and last %v", weeks, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

// For every month from 1900 to 2100 and both week starts, the week rows
// hold each day of the month once, in order, each in its weekday's column,
// with no empty week row.
func TestMonthWeeksEveryMonth(t *testing.T) {
	for _, start := range []time.Weekday{time.Monday, time.Sunday} {
		for year := MinCalendarYear; year <= MaxCalendarYear; year++ {
			for month := time.January; month <= time.December; month++ {
				name := fmt.Sprintf("%d-%02d from %v", year, month, start)
				weeks := monthWeeks(year, month, start)
				if len(weeks) < 4 || len(weeks) > 6 {
					t.Fatalf("%s: %d week rows", name, len(weeks))
				}
				next := 1
				for r, week := range weeks {
					filled := 0
					for c, day := range week {
						if day == 0 {
							continue
						}
						filled++
						if day != next {
							t.Fatalf("%s: row %d column %d is day %d, want %d", name, r, c, day, next)
						}
						next++
						got := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Weekday()
						if want := time.Weekday((int(start) + c) % 7); got != want {
							t.Fatalf("%s: day %d (a %v) in the %v column", name, day, got, want)
						}
					}
					if filled == 0 {
						t.Fatalf("%s: week row %d is empty", name, r)
					}
				}
				if next-1 != daysIn(year, month) {
					t.Fatalf("%s: %d days, want %d", name, next-1, daysIn(year, month))
				}
			}
		}
	}
}

// Weekday headers start on the week start: Monday first as given, or
// Sunday moved to the front.
func TestWeekdaysFrom(t *testing.T) {
	en := PageLabels{}.resolved().Weekdays
	if got := fmt.Sprint(weekdaysFrom(en, time.Monday)); got != "[Mon Tue Wed Thu Fri Sat Sun]" {
		t.Errorf("Monday start: %s", got)
	}
	if got := fmt.Sprint(weekdaysFrom(en, time.Sunday)); got != "[Sun Mon Tue Wed Thu Fri Sat]" {
		t.Errorf("Sunday start: %s", got)
	}
	if en[0] != "Mon" {
		t.Errorf("weekdaysFrom changed its input: %v", en)
	}
}
