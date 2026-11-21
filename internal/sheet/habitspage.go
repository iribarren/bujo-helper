package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
)

// HabitsKind is the ID of the habit tracker page kind: a habit per row and
// a box per day of the month, to tick off each day a habit is kept.
const HabitsKind = "habits"

// Habit list limits.
const (
	MaxHabits     = 40 // habits in a tracker
	MaxHabitRunes = 60 // characters in a habit
)

// Habit tracker drawing settings: lengths in millimetres, font sizes in
// points.
const (
	habitsNameSize    = 7.0 // habit names
	habitsMaxDaySize  = 6.0 // day-number header, shrunk to fit narrow boxes
	habitsMinDaySize  = 4.0
	habitsMaxBox      = 6.0 // largest day box
	habitsMinName     = 20.0
	habitsMaxNameFrac = 0.3 // widest name column, of the width
	habitsRowRatio    = 1.5 // row height, of the box size
	habitsMinRow      = 6.0 // lowest row, room to write a habit by hand
	habitsPad         = 1.0 // around habit names and day numbers
)

// HabitsOptions are the options of a habit tracker page.
type HabitsOptions struct {
	Month int `json:"month"` // 1–12; 0 means the current month
	Year  int `json:"year"`  // MinCalendarYear–MaxCalendarYear; 0 means the current year
	// Habits are the rows, trimmed, blank ones dropped: at most MaxHabits
	// of at most MaxHabitRunes characters. None prints blank rows to write
	// habits in by hand.
	Habits []string `json:"habits"`
}

var defaultHabits = HabitsOptions{Habits: []string{}}

// decodeHabits decodes habit tracker options, like decodeOptions, with the
// habits trimmed and blank ones dropped, and resolves a month or year of 0
// to the current one.
func decodeHabits(raw json.RawMessage) (pageDoc, error) {
	doc, err := decodeOptions(defaultHabits)(raw)
	if err != nil {
		return nil, err
	}
	o := doc.(HabitsOptions)
	o.Habits = normalizeHabits(o.Habits)
	if o.Month, o.Year, err = currentMonth(o.Month, o.Year, "habit tracker"); err != nil {
		return nil, err
	}
	return o, nil
}

// normalizeHabits returns habits trimmed, without the blank ones; never
// nil.
func normalizeHabits(habits []string) []string {
	out := []string{}
	for _, h := range habits {
		if h = strings.TrimSpace(h); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// validate checks the month, the year and the habits as they print:
// trimmed, blank ones dropped.
func (o HabitsOptions) validate() error {
	if err := validateMonth(o.Month, o.Year, "habit tracker"); err != nil {
		return err
	}
	habits := normalizeHabits(o.Habits)
	if len(habits) > MaxHabits {
		return fmt.Errorf("habit tracker: %d habits, want at most %d", len(habits), MaxHabits)
	}
	for _, h := range habits {
		if n := utf8.RuneCountInString(h); n > MaxHabitRunes {
			return fmt.Errorf("habit tracker habit %q: %d characters, want at most %d", h, n, MaxHabitRunes)
		}
	}
	return nil
}

// pages is the page count on A4, the default paper; PageCount asks pagesOn
// for the chosen paper.
func (o HabitsOptions) pages() int {
	n, _ := o.pagesOn(A4)
	return n
}

// pagesOn is how many pages the tracker takes on page: its rows run over
// pages when they do not fit.
func (o HabitsOptions) pagesOn(page Page) (int, error) {
	pdf := newPDF(page)
	if err := addRegularTextFont(pdf); err != nil {
		return 0, err
	}
	return len(o.sheets(page, pdfMeasure(pdf))), nil
}

// defaultTitle is the HabitsTitle label with the month name in place of
// "{month}", e.g. "Habits · February". The label is not a format string,
// so a translation cannot inject formatting verbs.
func (o HabitsOptions) defaultTitle(labels PageLabels) string {
	return strings.ReplaceAll(labels.HabitsTitle, "{month}", labels.Months[o.Month-1])
}

// draw draws the pages, each with the title centred in its band at the top
// in the lettering l and the tracker rows below it.
func (o HabitsOptions) draw(pdf *fpdf.Fpdf, page Page, title string, l lettering, _ PageLabels) error {
	if err := addRegularTextFont(pdf); err != nil {
		return err
	}
	for _, s := range o.sheets(page, pdfMeasure(pdf)) {
		pdf.AddPage()
		drawPageTitle(pdf, page, title, l)
		drawGridSheet(pdf, s)
	}
	return pdf.Error()
}

// habitsLayout is the geometry of a habit tracker page, below the title
// band: the name column, then a column of square boxes per day.
type habitsLayout struct {
	Area    Box
	Days    int
	NameW   float64 // the name column
	Box     float64 // a day box's side, and the day columns' width
	RowH    float64
	HeaderH float64 // the day-number header row
	DaySize float64 // the day numbers' font size
	PerPage int     // rows per page
}

// layout sizes the tracker on page. The name column fits the longest habit
// name (measured by measure) between habitsMinName and habitsMaxNameFrac
// of the width, or takes that widest share with no habits; the boxes share
// the rest, up to habitsMaxBox, and the name column takes any width the
// boxes leave. Rows are habitsRowRatio boxes high, at least habitsMinRow.
func (o HabitsOptions) layout(page Page, measure measureFunc) habitsLayout {
	area := moonArea(page)
	days := daysIn(o.Year, time.Month(o.Month))
	nameW := area.W * habitsMaxNameFrac
	if len(o.Habits) > 0 {
		longest := 0.0
		for _, h := range o.Habits {
			longest = max(longest, measure(h, false, habitsNameSize))
		}
		nameW = min(max(longest+2*habitsPad, habitsMinName), nameW)
	}
	box := min((area.W-nameW)/float64(days), habitsMaxBox)
	nameW = area.W - box*float64(days)

	// The widest day number fits its column with a little room.
	widest := func(pt float64) float64 {
		w := 0.0
		for d := 1; d <= days; d++ {
			w = max(w, measure(strconv.Itoa(d), false, pt))
		}
		return w
	}
	daySize := habitsMaxDaySize
	for daySize > habitsMinDaySize && widest(daySize) > box-habitsPad/2 {
		daySize -= 0.5
	}
	headerH := lineHeight(daySize) + 1.5
	rowH := max(box*habitsRowRatio, habitsMinRow)
	return habitsLayout{
		Area:    area,
		Days:    days,
		NameW:   nameW,
		Box:     box,
		RowH:    rowH,
		HeaderH: headerH,
		DaySize: daySize,
		PerPage: max(1, int(math.Floor((area.H-headerH)/rowH+1e-9))),
	}
}

// sheets lays out the tracker's pages, rows from the top of the space
// below the title band: on each page the day-number header row, then a row
// per habit, its name in the name column (cut with "…" if too long) and a
// box per day centred down the row. Rows that do not fit go on further
// pages. With no habits, one page of blank rows, each with a writing line
// in the name column.
func (o HabitsOptions) sheets(page Page, measure measureFunc) []gridSheet {
	g := o.layout(page, measure)
	var pages [][]string
	if len(o.Habits) == 0 {
		pages = [][]string{make([]string, g.PerPage)}
	}
	for i := 0; i < len(o.Habits); i += g.PerPage {
		pages = append(pages, o.Habits[i:min(i+g.PerPage, len(o.Habits))])
	}

	left := g.Area.X + g.NameW
	out := make([]gridSheet, len(pages))
	for p, rows := range pages {
		s := &out[p]
		for d := 0; d < g.Days; d++ {
			s.Texts = append(s.Texts, moonText{Text: strconv.Itoa(d + 1), Box: Box{X: left + float64(d)*g.Box, Y: g.Area.Y, W: g.Box, H: g.HeaderH}, Size: g.DaySize, Centred: true})
		}
		for r, habit := range rows {
			y := g.Area.Y + g.HeaderH + float64(r)*g.RowH
			boxY := y + (g.RowH-g.Box)/2
			if habit == "" {
				s.Lines = append(s.Lines, moonRule{X1: g.Area.X + habitsPad, X2: left - habitsPad, Y: boxY + g.Box})
			} else {
				nameBox := Box{X: g.Area.X + habitsPad, Y: y, W: g.NameW - 2*habitsPad, H: g.RowH}
				s.Texts = append(s.Texts, moonText{Text: fitText(habit, nameBox.W, habitsNameSize, measure), Box: nameBox, Size: habitsNameSize})
			}
			for d := 0; d < g.Days; d++ {
				s.Boxes = append(s.Boxes, Box{X: left + float64(d)*g.Box, Y: boxY, W: g.Box, H: g.Box})
			}
		}
	}
	return out
}
