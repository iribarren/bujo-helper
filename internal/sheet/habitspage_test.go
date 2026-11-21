package sheet

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// habitList is n habits named "Habit 1" to "Habit n".
func habitList(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Habit %d", i+1)
	}
	return out
}

// habitsJSON is the JSON of a list of habits.
func habitsJSON(habits []string) string {
	raw, _ := json.Marshal(habits)
	return string(raw)
}

func TestDecodeHabitsOptions(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	kind, err := PageKindByID(HabitsKind)
	if err != nil {
		t.Fatal(err)
	}
	longest := strings.Repeat("é", MaxHabitRunes) // 60 runes, 120 bytes
	blanks := append(habitList(MaxHabits), "", "  ")
	tests := []struct {
		name    string
		raw     string
		want    HabitsOptions
		wantErr string // part of the error; "" for none
	}{
		{"no options: this month, no habits", "", HabitsOptions{11, 2026, []string{}}, ""},
		{"empty object", "{}", HabitsOptions{11, 2026, []string{}}, ""},
		{"null habits", `{"habits":null}`, HabitsOptions{11, 2026, []string{}}, ""},
		{"explicit", `{"month":2,"year":2024,"habits":["Read","Walk"]}`, HabitsOptions{2, 2024, []string{"Read", "Walk"}}, ""},
		{"trimmed, blanks dropped", `{"habits":["  Read ", "", "   ", "\tWalk\n"]}`, HabitsOptions{11, 2026, []string{"Read", "Walk"}}, ""},
		{"only blanks", `{"habits":[" ", ""]}`, HabitsOptions{11, 2026, []string{}}, ""},
		{"control characters become one space", `{"habits":["Read\tbooks", "Drink\r\n water", "Walk\u0007\u0000daily", "\u0085Stretch\u001b"]}`, HabitsOptions{11, 2026, []string{"Read books", "Drink water", "Walk daily", "Stretch"}}, ""},
		{"only control characters dropped", `{"habits":["\u0000\u0007", "\t\r\n", "Read"]}`, HabitsOptions{11, 2026, []string{"Read"}}, ""},
		{"60 runes after control characters collapse", `{"habits":["` + strings.Repeat("é", 30) + `\t\n` + strings.Repeat("é", 29) + `"]}`, HabitsOptions{11, 2026, []string{strings.Repeat("é", 30) + " " + strings.Repeat("é", 29)}}, ""},
		{"40 habits", `{"habits":` + habitsJSON(habitList(MaxHabits)) + `}`, HabitsOptions{11, 2026, habitList(MaxHabits)}, ""},
		{"40 habits and blanks", `{"habits":` + habitsJSON(blanks) + `}`, HabitsOptions{11, 2026, habitList(MaxHabits)}, ""},
		{"41 habits", `{"habits":` + habitsJSON(habitList(MaxHabits+1)) + `}`, HabitsOptions{}, "41"},
		{"60 runes", `{"habits":["  ` + longest + `  "]}`, HabitsOptions{11, 2026, []string{longest}}, ""},
		{"61 runes", `{"habits":["` + longest + `x"]}`, HabitsOptions{}, "61"},
		{"month 13", `{"month":13}`, HabitsOptions{}, "13"},
		{"year too early", `{"year":1899}`, HabitsOptions{}, "1899"},
		{"year too late", `{"year":2101}`, HabitsOptions{}, "2101"},
		{"habits as a string", `{"habits":"Read"}`, HabitsOptions{}, "habits"},
		{"a habit as a number", `{"habits":[1]}`, HabitsOptions{}, "habits"},
		{"unknown field", `{"habits":[],"weeks":true}`, HabitsOptions{}, `"weeks"`},
		{"trailing data", `{} {}`, HabitsOptions{}, "options"},
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
			if got, ok := doc.(HabitsOptions); !ok || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %#v, want %#v", doc, tt.want)
			}
		})
	}
	if len(defaultHabits.Habits) != 0 {
		t.Errorf("decoding changed the defaults: %v", defaultHabits.Habits)
	}
}

func TestHabitsRegistryEntry(t *testing.T) {
	k, err := PageKindByID(HabitsKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Habit tracker" || k.FileTag != "habits" || !k.UsesTitle || k.UsesCut {
		t.Errorf("habits kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{"month":0,"year":0,"habits":[]}` {
		t.Errorf("defaults %s", raw)
	}
}

// The default title puts the month name in the HabitsTitle label at
// "{month}"; anything else in the label, such as a "%", prints as is.
func TestHabitsDefaultTitle(t *testing.T) {
	es := spanishPageLabels
	es.HabitsTitle = "Hábitos · {month}"
	tests := []struct {
		labels PageLabels
		want   string
	}{
		{PageLabels{}, "Habits · February"},
		{es, "Hábitos · Febrero"},
		{PageLabels{HabitsTitle: "100% %s {month} %d"}, "100% %s February %d"},
		{PageLabels{HabitsTitle: "  "}, "Habits · February"},
	}
	for _, tt := range tests {
		o := HabitsOptions{Month: 2, Year: 2024}
		if got := o.defaultTitle(tt.labels.resolved()); got != tt.want {
			t.Errorf("label %q: title %q, want %q", tt.labels.HabitsTitle, got, tt.want)
		}
	}
}

// habitsCases are habit lists to lay out: none, a few short ones, names
// too long for any column, and the most there may be.
var habitsCases = map[string][]string{
	"none":    {},
	"short":   {"Read", "Walk", "Water"},
	"long":    {strings.Repeat("W", MaxHabitRunes), "Meditar diez minutos al día sin excepción alguna", "Yoga"},
	"maximum": habitList(MaxHabits),
}

// On every paper, for months of 28 and 31 days and any habit list, every
// page keeps everything inside the margins below the title band, labels
// fit their boxes, habit names never reach into the day boxes, day boxes
// are square, at most habitsMaxBox wide and share one column per day.
func TestHabitsSheetGeometry(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	for _, p := range Papers {
		for name, habits := range habitsCases {
			for _, month := range []int{2, 1} {
				t.Run(fmt.Sprintf("%s/%s/%d", p.ID, name, month), func(t *testing.T) {
					o := HabitsOptions{Month: month, Year: 2023, Habits: habits}
					days := daysIn(2023, time.Month(month))
					area := moonArea(p.Page)
					sheets := o.sheets(p.Page, measure)
					if len(sheets) == 0 {
						t.Fatal("no pages")
					}
					rows := 0
					for i, s := range sheets {
						checkGridSheet(t, s, area, measure)
						if len(s.Boxes)%days != 0 || len(s.Boxes) == 0 {
							t.Fatalf("page %d: %d boxes for %d days", i, len(s.Boxes), days)
						}
						rows += len(s.Boxes) / days
						firstBox := s.Boxes[0]
						for _, b := range s.Boxes {
							if !near(b.W, b.H) || b.W > habitsMaxBox+1e-9 {
								t.Fatalf("page %d: box %v × %v mm", i, b.W, b.H)
							}
							if b.X < firstBox.X-1e-9 {
								t.Fatalf("page %d: box at x %v left of the first day column %v", i, b.X, firstBox.X)
							}
						}
						for _, tx := range s.Texts {
							if !tx.Centred && tx.Box.X+tx.Box.W > firstBox.X+1e-9 {
								t.Errorf("page %d: habit %q runs into the day boxes", i, tx.Text)
							}
						}
						for _, l := range s.Lines {
							if l.X2 > firstBox.X+1e-9 {
								t.Errorf("page %d: writing line runs into the day boxes", i)
							}
						}
					}
					if len(habits) > 0 && rows != len(habits) {
						t.Errorf("%d rows for %d habits", rows, len(habits))
					}
				})
			}
		}
	}
}

// Habit names print whole when they fit and are cut with "…" otherwise,
// once each, in order, across the pages.
func TestHabitsSheetNames(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	for _, p := range Papers {
		for name, habits := range habitsCases {
			o := HabitsOptions{Month: 1, Year: 2023, Habits: habits}
			var names []string
			for _, s := range o.sheets(p.Page, measure) {
				for _, tx := range s.Texts {
					if !tx.Centred {
						names = append(names, tx.Text)
					}
				}
			}
			if len(names) != len(habits) {
				t.Fatalf("%s/%s: names %q", p.ID, name, names)
			}
			for i, got := range names {
				if got != habits[i] && (!strings.HasSuffix(got, "…") || !strings.HasPrefix(habits[i], strings.TrimSuffix(got, "…"))) {
					t.Errorf("%s/%s: habit %q printed as %q", p.ID, name, habits[i], got)
				}
			}
		}
	}
	// The 60-letter name does not fit even on Letter.
	s := HabitsOptions{Month: 1, Year: 2023, Habits: habitsCases["long"]}.sheets(Letter, measure)
	if got := s[0].Texts[len(s[0].Texts)-3].Text; !strings.HasSuffix(got, "…") {
		t.Errorf("the 60-letter habit printed whole: %q", got)
	}
}

// The day-number header prints 1 to the last day of the month on every
// page.
func TestHabitsSheetDayHeader(t *testing.T) {
	pdf := newPDF(A5)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	sheets := HabitsOptions{Month: 2, Year: 2024, Habits: habitList(MaxHabits)}.sheets(A5, pdfMeasure(pdf))
	for i, s := range sheets {
		var days []string
		for _, tx := range s.Texts {
			if tx.Centred {
				days = append(days, tx.Text)
			}
		}
		if len(days) != 29 || days[0] != "1" || days[28] != "29" {
			t.Errorf("page %d: day header %v", i, days)
		}
	}
}

// With no habits, one page is filled with blank rows, each a writing line
// in the name column and a box per day; another row would not fit.
func TestHabitsBlankRows(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	for _, p := range Papers {
		sheets := HabitsOptions{Month: 1, Year: 2023, Habits: []string{}}.sheets(p.Page, measure)
		if len(sheets) != 1 {
			t.Fatalf("%s: %d pages, want 1", p.ID, len(sheets))
		}
		s := sheets[0]
		rows := len(s.Lines)
		if rows < 10 || len(s.Boxes) != rows*31 {
			t.Fatalf("%s: %d writing lines, %d boxes", p.ID, rows, len(s.Boxes))
		}
		area := moonArea(p.Page)
		// Lines sit at the bottom of the boxes, which are centred down
		// their rows.
		rowH := s.Lines[1].Y - s.Lines[0].Y
		box := s.Boxes[0].H
		if bottom := s.Lines[rows-1].Y + (rowH-box)/2; area.Y+area.H-bottom >= rowH-1e-9 {
			t.Errorf("%s: room for another %v mm row below %v", p.ID, rowH, bottom)
		}
		for _, tx := range s.Texts {
			if !tx.Centred {
				t.Errorf("%s: blank page prints %q", p.ID, tx.Text)
			}
		}
	}
}

// Rows run over pages when they do not fit: the most habits need two A5
// pages, and the page count PageCount reports is the one drawn.
func TestHabitsPagination(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	opts := `{"month":1,"year":2023,"habits":` + habitsJSON(habitList(MaxHabits)) + `}`
	n, err := PageCount(HabitsKind, PageSettings{Paper: "a5"}, json.RawMessage(opts))
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Errorf("40 habits on A5: %d pages, want several", n)
	}
	out := renderPage(t, HabitsKind, PageSettings{Paper: "a5"}, opts)
	if got := strings.Count(string(out), "/Type /Page\n"); got != n {
		t.Errorf("PageCount %d, drew %d pages", n, got)
	}
	if n, err := PageCount(HabitsKind, PageSettings{Paper: "a5"}, json.RawMessage(`{"habits":["Read"]}`)); n != 1 || err != nil {
		t.Errorf("one habit: PageCount = %d, %v", n, err)
	}
}

// A habit with control characters prints with a space in their place, and
// is measured that way.
func TestHabitsControlCharactersPrintAsSpaces(t *testing.T) {
	kind, err := PageKindByID(HabitsKind)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := kind.decode(json.RawMessage(`{"month":1,"year":2023,"habits":["Read\tbooks"]}`))
	if err != nil {
		t.Fatal(err)
	}
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	o := doc.(HabitsOptions)
	var names []string
	for _, tx := range o.sheets(A4, measure)[0].Texts {
		if !tx.Centred {
			names = append(names, tx.Text)
		}
	}
	if !reflect.DeepEqual(names, []string{"Read books"}) {
		t.Errorf("printed %q, want [\"Read books\"]", names)
	}
	spaced := HabitsOptions{Month: 1, Year: 2023, Habits: []string{"Read books"}}
	if got, want := o.layout(A4, measure), spaced.layout(A4, measure); got != want {
		t.Errorf("layout %+v, want the layout of \"Read books\" %+v", got, want)
	}
}

// pages, the A4 count of the pageDoc interface, is the A4 page count and
// never below 1, so a caller that asks it never gets an empty document.
// (PageCount asks pagesOn; pagesOn fails only if the embedded Go font does
// not load, which a test cannot provoke.)
func TestHabitsPagesOnA4(t *testing.T) {
	for _, habits := range [][]string{{}, {"Read"}, habitList(MaxHabits)} {
		o := HabitsOptions{Month: 1, Year: 2023, Habits: habits}
		want, err := o.pagesOn(A4)
		if err != nil {
			t.Fatal(err)
		}
		if got := o.pages(); got != want || got < 1 {
			t.Errorf("%d habits: pages() = %d, pagesOn(A4) = %d", len(habits), got, want)
		}
	}
}

// The habit tracker renders on every paper, filled or for colouring in,
// with no habits or the most: as many pages as PageCount says, a stroked
// box per day and row, no fills, and with Colour-in the title on every
// page, and nothing else, in hollow letters.
func TestBuildHabitsEveryPaper(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	for _, p := range Papers {
		for _, habits := range [][]string{{}, habitList(MaxHabits)} {
			for _, colourIn := range []bool{false, true} {
				name := fmt.Sprintf("%s/%d habits/colourIn=%v", p.ID, len(habits), colourIn)
				t.Run(name, func(t *testing.T) {
					opts := `{"month":2,"year":2024,"habits":` + habitsJSON(habits) + `}`
					page := PageSettings{Paper: p.ID, ColourIn: colourIn}
					want, err := PageCount(HabitsKind, page, json.RawMessage(opts))
					if err != nil {
						t.Fatal(err)
					}
					out := renderPage(t, HabitsKind, page, opts)
					if n := strings.Count(string(out), "/Type /Page\n"); n != want {
						t.Errorf("%d pages, want %d", n, want)
					}
					stream := pageStreams(out)
					boxes := strings.Count(stream, " re S")
					if boxes == 0 || boxes%29 != 0 || len(habits) > 0 && boxes != 29*len(habits) {
						t.Errorf("%d stroked boxes for %d habits", boxes, len(habits))
					}
					if fills := len(paintOps.FindAllString(stream, -1)); fills != 0 {
						t.Errorf("%d fills, want none", fills)
					}
					checkHollowTitleOnly(t, stream, colourIn, want)
				})
			}
		}
	}
}

// The habit tracker always prints a title: the shared one, else "Habits ·
// <Month>" in the UI language, on every page.
func TestBuildHabitsTitle(t *testing.T) {
	fixCalendarClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	opts := `{"month":2,"year":2024,"habits":["Read"]}`
	none := pageStreams(renderPage(t, HabitsKind, PageSettings{}, opts))
	explicit := pageStreams(renderPage(t, HabitsKind, PageSettings{Title: "Habits · February"}, opts))
	other := pageStreams(renderPage(t, HabitsKind, PageSettings{Title: "Routines"}, opts))
	if none != explicit {
		t.Error("the default title is not \"Habits · February\"")
	}
	if none == other {
		t.Error("a set title did not replace the default")
	}
	es := spanishPageLabels
	es.HabitsTitle = "Hábitos · {month}"
	esNone := pageStreams(renderPage(t, HabitsKind, PageSettings{Labels: es}, opts))
	esExplicit := pageStreams(renderPage(t, HabitsKind, PageSettings{Labels: es, Title: "Hábitos · Febrero"}, opts))
	if esNone != esExplicit || esNone == none {
		t.Error("the Spanish default title is not \"Hábitos · Febrero\"")
	}
	// Every page carries the title: with Colour-in it is the one hollow
	// text of each page.
	many := `{"month":2,"year":2024,"habits":` + habitsJSON(habitList(MaxHabits)) + `}`
	pages := pageStreamsEach(renderPage(t, HabitsKind, PageSettings{Paper: "a5", Title: "Routines", ColourIn: true}, many))
	if len(pages) < 2 {
		t.Fatalf("%d pages", len(pages))
	}
	for i, s := range pages {
		if n := trPairs(t, fmt.Sprintf("page %d", i), s); n != 1 {
			t.Errorf("page %d: %d hollow texts, want the title", i, n)
		}
	}
}
