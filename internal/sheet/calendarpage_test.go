package sheet

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fixCalendarClock pins the clock of the month calendar and the habit
// tracker to now and their time zone to UTC for the test.
func fixCalendarClock(t *testing.T, now time.Time) {
	t.Helper()
	oldNow, oldLoc := calendarNow, calendarLocation
	calendarNow = func() time.Time { return now }
	calendarLocation = time.UTC
	t.Cleanup(func() { calendarNow, calendarLocation = oldNow, oldLoc })
}

func TestDecodeCalendarOptions(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	kind, err := PageKindByID(CalendarKind)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		raw     string
		want    CalendarOptions
		wantErr string // part of the error; "" for none
	}{
		{"no options: this month from Monday", "", CalendarOptions{11, 2026, "mon"}, ""},
		{"null", "null", CalendarOptions{11, 2026, "mon"}, ""},
		{"empty object", "{}", CalendarOptions{11, 2026, "mon"}, ""},
		{"explicit", `{"month":2,"year":2024,"weekStart":"sun"}`, CalendarOptions{2, 2024, "sun"}, ""},
		{"only the year is current", `{"month":3,"year":0}`, CalendarOptions{3, 2026, "mon"}, ""},
		{"only the month is current", `{"year":1900}`, CalendarOptions{11, 1900, "mon"}, ""},
		{"last year", `{"year":2100,"month":12}`, CalendarOptions{12, 2100, "mon"}, ""},
		{"month 13", `{"month":13}`, CalendarOptions{}, "13"},
		{"negative month", `{"month":-1}`, CalendarOptions{}, "-1"},
		{"year too early", `{"year":1899}`, CalendarOptions{}, "1899"},
		{"year too late", `{"year":2101}`, CalendarOptions{}, "2101"},
		{"unknown week start", `{"weekStart":"sat"}`, CalendarOptions{}, `"sat"`},
		{"blank week start", `{"weekStart":""}`, CalendarOptions{}, `""`},
		{"week start in capitals", `{"weekStart":"Sun"}`, CalendarOptions{}, `"Sun"`},
		{"unknown field", `{"month":1,"holidays":true}`, CalendarOptions{}, `"holidays"`},
		{"month as a string", `{"month":"1"}`, CalendarOptions{}, "month"},
		{"trailing data", `{} {}`, CalendarOptions{}, "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := kind.decode(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := doc.(CalendarOptions); !ok || got != tt.want {
				t.Errorf("decoded %#v, want %#v", doc, tt.want)
			}
		})
	}
}

// The current month and year come from the clock in the calendar's time
// zone: 23:30 UTC on 31 December is already January two hours east.
func TestCalendarCurrentMonthInTheLocalZone(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC))
	calendarLocation = time.FixedZone("EET", 2*3600)
	doc, err := decodeCalendar(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.(CalendarOptions); got.Month != 1 || got.Year != 2027 {
		t.Errorf("current month %d/%d, want 1/2027", got.Month, got.Year)
	}
}

// A current year outside 1900–2100 (a wrong system clock) is rejected; an
// explicit year does not depend on the clock. Both kinds share the rule.
func TestCalendarCurrentYearOutOfRange(t *testing.T) {
	tests := []struct {
		name    string
		now     time.Time
		raw     string
		wantErr string // part of the error; "" for none
	}{
		{"clock before 1900", time.Date(1899, 6, 1, 12, 0, 0, 0, time.UTC), `{"year":0}`, "1899"},
		{"clock after 2100", time.Date(2101, 6, 1, 12, 0, 0, 0, time.UTC), "", "2101"},
		{"explicit year, clock after 2100", time.Date(2150, 6, 1, 12, 0, 0, 0, time.UTC), `{"month":1,"year":2024}`, ""},
		{"clock on the last year", time.Date(2100, 12, 31, 12, 0, 0, 0, time.UTC), "", ""},
	}
	for _, kind := range []string{CalendarKind, HabitsKind} {
		k, err := PageKindByID(kind)
		if err != nil {
			t.Fatal(err)
		}
		for _, tt := range tests {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				fixCalendarClock(t, tt.now)
				_, err := k.decode(json.RawMessage(tt.raw))
				if tt.wantErr == "" {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
			})
		}
	}
}

func TestCalendarRegistryEntry(t *testing.T) {
	k, err := PageKindByID(CalendarKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Month calendar" || k.FileTag != "calendar" || !k.UsesTitle || k.UsesCut {
		t.Errorf("calendar kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{"month":0,"year":0,"weekStart":"mon"}` {
		t.Errorf("defaults %s", raw)
	}
	// Moon, then the month calendar and the habit tracker, then the map.
	var ids []string
	for _, k := range PageKinds {
		ids = append(ids, k.ID)
	}
	if got := strings.Join(ids, ","); got != "dotgrid,moon,calendar,habits,map" {
		t.Errorf("page kinds in order %s", got)
	}
}

func TestCalendarDefaultTitle(t *testing.T) {
	tests := []struct {
		labels PageLabels
		want   string
	}{
		{PageLabels{}, "February 2024"},
		{spanishPageLabels, "Febrero 2024"},
	}
	for _, tt := range tests {
		o := CalendarOptions{Month: 2, Year: 2024, WeekStart: "sun"}
		if got := o.defaultTitle(tt.labels.resolved()); got != tt.want {
			t.Errorf("title %q, want %q", got, tt.want)
		}
	}
}

// calendarCases are months with 4, 5 and 6 week rows for either start:
// February 2015 starts on a Sunday, September 2024 too (6 rows from
// Monday), and May 2026 on a Friday (6 rows from Monday).
var calendarCases = []struct {
	year  int
	month time.Month
}{{2015, time.February}, {2024, time.September}, {2026, time.May}, {2024, time.February}}

// On every paper and week start, in English and Spanish, every cell and
// label lies inside the margins below the title band, each label fits its
// box, there is one cell per day in its weekday's column, cells do not
// overlap and are no taller than the cap.
func TestCalendarSheetGeometry(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	for _, p := range Papers {
		for _, start := range CalendarWeekStarts {
			for _, labels := range []PageLabels{{}, spanishPageLabels} {
				for _, c := range calendarCases {
					name := fmt.Sprintf("%s/%s/%d-%02d/%v", p.ID, start, c.year, c.month, labels.NewMoon)
					t.Run(name, func(t *testing.T) {
						o := CalendarOptions{Month: int(c.month), Year: c.year, WeekStart: start}
						s := o.sheet(p.Page, labels.resolved())
						area := moonArea(p.Page)
						checkGridSheet(t, s, area, measure)
						if n := daysIn(c.year, c.month); len(s.Boxes) != n {
							t.Fatalf("%d cells, want %d", len(s.Boxes), n)
						}
						weekStart := calendarWeekday(start)
						cellW := area.W / 7
						for i, b := range s.Boxes {
							day := time.Date(c.year, c.month, i+1, 0, 0, 0, 0, time.UTC)
							col := (int(day.Weekday()) - int(weekStart) + 7) % 7
							if got := int((b.X - area.X + cellW/2) / cellW); got != col {
								t.Errorf("day %d in column %d, want %d", i+1, got, col)
							}
							if b.H > b.W*calendarMaxCellRatio+1e-9 {
								t.Errorf("day %d: cell %v × %v mm, taller than the cap", i+1, b.W, b.H)
							}
							for j := i + 1; j < len(s.Boxes); j++ {
								if overlap(b, s.Boxes[j]) {
									t.Errorf("cells %d and %d overlap", i+1, j+1)
								}
							}
						}
					})
				}
			}
		}
	}
}

// checkGridSheet checks that everything on s lies within area and that
// each text fits its box.
func checkGridSheet(t *testing.T, s gridSheet, area Box, measure measureFunc) {
	t.Helper()
	const eps = 1e-9
	inside := func(what string, b Box) {
		if b.X < area.X-eps || b.Y < area.Y-eps || b.X+b.W > area.X+area.W+eps || b.Y+b.H > area.Y+area.H+eps {
			t.Errorf("%s %+v leaves the area %+v", what, b, area)
		}
	}
	for i, b := range s.Boxes {
		if b.W <= 1 || b.H <= 1 {
			t.Errorf("box %d is %v × %v mm", i, b.W, b.H)
		}
		inside(fmt.Sprintf("box %d", i), b)
	}
	for _, tx := range s.Texts {
		inside(fmt.Sprintf("text %q", tx.Text), tx.Box)
		if w := measure(tx.Text, false, tx.Size); w > tx.Box.W+eps {
			t.Errorf("text %q is %.2f mm wide in a %.2f mm box", tx.Text, w, tx.Box.W)
		}
		if h := lineHeight(tx.Size); h > tx.Box.H+eps {
			t.Errorf("text %q needs %.2f mm in a %.2f mm box", tx.Text, h, tx.Box.H)
		}
	}
	for _, r := range s.Lines {
		inside("line", Box{X: r.X1, Y: r.Y, W: r.X2 - r.X1, H: 0})
	}
}

// The calendar prints the weekday headers from its week start and every
// day number once, each at the top left of its cell.
func TestCalendarSheetLabels(t *testing.T) {
	es := spanishPageLabels.resolved()
	for _, tt := range []struct {
		start string
		first string
	}{{"mon", "Lun"}, {"sun", "Dom"}} {
		s := CalendarOptions{Month: 9, Year: 2024, WeekStart: tt.start}.sheet(A5, es)
		count := map[string]int{}
		for _, tx := range s.Texts {
			count[tx.Text]++
		}
		for _, wd := range es.Weekdays {
			if count[wd] != 1 {
				t.Errorf("%s: weekday %q printed %d times", tt.start, wd, count[wd])
			}
		}
		if s.Texts[0].Text != tt.first {
			t.Errorf("%s: first header %q, want %q", tt.start, s.Texts[0].Text, tt.first)
		}
		for day := 1; day <= 30; day++ {
			n := fmt.Sprint(day)
			if count[n] != 1 {
				t.Errorf("%s: day %s printed %d times", tt.start, n, count[n])
			}
		}
		// Day numbers follow the headers, in order, inside their cells.
		for i, b := range s.Boxes {
			tx := s.Texts[7+i]
			if tx.Text != fmt.Sprint(i+1) || tx.Centred || tx.Box.X < b.X || tx.Box.Y < b.Y || tx.Box.X > b.X+3 || tx.Box.Y > b.Y+3 {
				t.Errorf("%s: day %d label %+v in cell %+v", tt.start, i+1, tx, b)
			}
		}
	}
}

// The month calendar renders on every paper and week start, filled or for
// colouring in: one page, a stroked cell per day, and with Colour-in the
// title, and only the title, in hollow letters.
func TestBuildCalendarEveryPaper(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	for _, p := range Papers {
		for _, start := range CalendarWeekStarts {
			for _, colourIn := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/colourIn=%v", p.ID, start, colourIn)
				t.Run(name, func(t *testing.T) {
					opts := fmt.Sprintf(`{"month":9,"year":2024,"weekStart":%q}`, start)
					out := renderPage(t, CalendarKind, PageSettings{Paper: p.ID, ColourIn: colourIn}, opts)
					if n := strings.Count(string(out), "/Type /Page\n"); n != 1 {
						t.Errorf("%d pages, want 1", n)
					}
					stream := pageStreams(out)
					if n := strings.Count(stream, " re S"); n != 30 {
						t.Errorf("%d stroked cells, want 30", n)
					}
					if fills := len(paintOps.FindAllString(stream, -1)); fills != 0 {
						t.Errorf("%d fills, want none", fills)
					}
					checkHollowTitleOnly(t, stream, colourIn, 1)
				})
			}
		}
	}
}

// checkHollowTitleOnly checks that a stream draws pages hollow texts (the
// title on each page) with Colour-in and none without.
func checkHollowTitleOnly(t *testing.T, stream string, colourIn bool, pages int) {
	t.Helper()
	if !colourIn {
		if strings.Contains(stream, " Tr") || strings.Contains(stream, "\n1 Tr") {
			t.Error("filled lettering uses a text rendering mode")
		}
		return
	}
	if got := trPairs(t, "page", stream); got != pages {
		t.Errorf("%d hollow texts, want %d (the titles)", got, pages)
	}
}

// The calendar always prints a title: the shared one, else "<Month>
// <Year>" in the UI language, drawn like a set title.
func TestBuildCalendarTitle(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	opts := `{"month":2,"year":2024}`
	none := pageStreams(renderPage(t, CalendarKind, PageSettings{}, opts))
	explicit := pageStreams(renderPage(t, CalendarKind, PageSettings{Title: "February 2024"}, opts))
	other := pageStreams(renderPage(t, CalendarKind, PageSettings{Title: "Plans"}, opts))
	if none != explicit {
		t.Error("the default title is not \"February 2024\"")
	}
	if none == other {
		t.Error("a set title did not replace the default")
	}
	es := pageStreams(renderPage(t, CalendarKind, PageSettings{Labels: spanishPageLabels}, opts))
	esExplicit := pageStreams(renderPage(t, CalendarKind, PageSettings{Labels: spanishPageLabels, Title: "Febrero 2024"}, opts))
	if es != esExplicit || es == none {
		t.Error("the Spanish default title is not \"Febrero 2024\"")
	}
	out := string(renderPage(t, CalendarKind, PageSettings{Font: "pacifico"}, opts))
	if !strings.Contains(out, "/BaseFont /utf8pacifico") || !strings.Contains(out, "/BaseFont /utf8go\n") {
		t.Error("the default title is not in the lettering font")
	}
	if strings.Contains(out, "/BaseFont /utf8goB") {
		t.Error("the bold Go font is embedded for regular labels")
	}
}
