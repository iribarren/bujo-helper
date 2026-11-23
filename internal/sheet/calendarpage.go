package sheet

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/image/font/gofont/goregular"
)

// CalendarKind is the ID of the month calendar page kind: a month grid of
// day cells to write in, weeks starting on Monday or Sunday.
const CalendarKind = "calendar"

// Week starts of a month calendar.
const (
	CalendarMonday = "mon"
	CalendarSunday = "sun"
)

// CalendarWeekStarts are the week starts to choose from.
var CalendarWeekStarts = []string{CalendarMonday, CalendarSunday}

// Month calendar drawing settings: lengths in millimetres, font sizes in
// points.
const (
	calendarDaySize    = 9.0 // day numbers
	calendarHeaderSize = 8.0 // weekday headers
	calendarPad        = 1.5 // between a cell's border and its day number
	calendarLineWidth  = 0.2 // cell borders
	// calendarMaxCellRatio caps a cell's height at this many times its
	// width, so that on tall paper (A4, Letter) with few week rows cells
	// stay roughly square-ish instead of stretching down the page; the grid
	// is then centred down the space below the title.
	calendarMaxCellRatio = 1.5
)

// The clock and time zone the month calendar and the habit tracker resolve
// "this month" in; tests replace them.
var (
	calendarNow      = time.Now
	calendarLocation = time.Local
)

// CalendarOptions are the options of a month calendar page.
type CalendarOptions struct {
	Month     int    `json:"month"`     // 1–12; 0 means the current month
	Year      int    `json:"year"`      // MinCalendarYear–MaxCalendarYear; 0 means the current year
	WeekStart string `json:"weekStart"` // one of CalendarWeekStarts
}

var defaultCalendar = CalendarOptions{WeekStart: CalendarMonday}

// decodeCalendar decodes month calendar options, like decodeOptions, and
// resolves a month or year of 0 to the current one.
func decodeCalendar(raw json.RawMessage) (pageDoc, error) {
	doc, err := decodeOptions(defaultCalendar)(raw)
	if err != nil {
		return nil, err
	}
	o := doc.(CalendarOptions)
	if o.Month, o.Year, err = currentMonth(o.Month, o.Year, "month calendar"); err != nil {
		return nil, err
	}
	return o, nil
}

// currentMonth resolves a month or year of 0 to the current one in
// calendarLocation. A current year outside MinCalendarYear–MaxCalendarYear
// (a wrong clock) is an error naming what.
func currentMonth(month, year int, what string) (int, int, error) {
	now := calendarNow().In(calendarLocation)
	if month == 0 {
		month = int(now.Month())
	}
	if year == 0 {
		year = now.Year()
		if year < MinCalendarYear || year > MaxCalendarYear {
			return 0, 0, fmt.Errorf("%s: the current year %d is not %d to %d; choose a year", what, year, MinCalendarYear, MaxCalendarYear)
		}
	}
	return month, year, nil
}

// validateMonth reports a month (0–12) or year (0 or
// MinCalendarYear–MaxCalendarYear) out of range, naming what.
func validateMonth(month, year int, what string) error {
	if month < 0 || month > 12 {
		return fmt.Errorf("%s month %d: want 1 to 12", what, month)
	}
	if year != 0 && (year < MinCalendarYear || year > MaxCalendarYear) {
		return fmt.Errorf("%s year %d: want %d to %d", what, year, MinCalendarYear, MaxCalendarYear)
	}
	return nil
}

func (o CalendarOptions) validate() error {
	if err := validateMonth(o.Month, o.Year, "month calendar"); err != nil {
		return err
	}
	if !oneOf(o.WeekStart, CalendarWeekStarts) {
		return fmt.Errorf("month calendar week start %q: want one of %v", o.WeekStart, CalendarWeekStarts)
	}
	return nil
}

// pages is always 1: a month fits one page.
func (o CalendarOptions) pages() int { return 1 }

// defaultTitle is "<Month> <Year>".
func (o CalendarOptions) defaultTitle(labels PageLabels) string {
	return labels.Months[o.Month-1] + " " + strconv.Itoa(o.Year)
}

// calendarWeekday is the weekday a week start names: Sunday for
// CalendarSunday, else Monday.
func calendarWeekday(weekStart string) time.Weekday {
	if weekStart == CalendarSunday {
		return time.Sunday
	}
	return time.Monday
}

// draw draws the page: the title centred in its band at the top in the
// lettering l, and the month grid below it.
func (o CalendarOptions) draw(pdf *pdfDoc, page Page, title string, l lettering, labels PageLabels) error {
	if err := addRegularTextFont(pdf); err != nil {
		return err
	}
	pdf.AddPage()
	drawPageTitle(pdf, page, title, l)
	drawGridSheet(pdf, o.sheet(page, labels))
	return pdf.Error()
}

// sheet lays out the month grid in the space below the title band: a
// weekday header row from the week start, then a cell per day in 7
// columns, the day number at its top left. Cells are as tall as fit, up
// to calendarMaxCellRatio times their width, and the grid is centred down
// the space. Empty cells before the first day and after the last have no
// box.
func (o CalendarOptions) sheet(page Page, labels PageLabels) gridSheet {
	area := moonArea(page)
	start := calendarWeekday(o.WeekStart)
	weeks := monthWeeks(o.Year, time.Month(o.Month), start)
	cellW := area.W / 7
	headerH := lineHeight(calendarHeaderSize) + 2
	cellH := min((area.H-headerH)/float64(len(weeks)), cellW*calendarMaxCellRatio)
	top := area.Y + (area.H-headerH-cellH*float64(len(weeks)))/2

	var s gridSheet
	for c, wd := range weekdaysFrom(labels.Weekdays, start) {
		s.Texts = append(s.Texts, moonText{Text: wd, Box: Box{X: area.X + float64(c)*cellW, Y: top, W: cellW, H: headerH}, Size: calendarHeaderSize, Centred: true})
	}
	for r, week := range weeks {
		for c, day := range week {
			if day == 0 {
				continue
			}
			cell := Box{X: area.X + float64(c)*cellW, Y: top + headerH + float64(r)*cellH, W: cellW, H: cellH}
			s.Boxes = append(s.Boxes, cell)
			s.Texts = append(s.Texts, moonText{
				Text: strconv.Itoa(day),
				Box:  Box{X: cell.X + calendarPad, Y: cell.Y + calendarPad, W: cellW - 2*calendarPad, H: lineHeight(calendarDaySize)},
				Size: calendarDaySize,
			})
		}
	}
	return s
}

// gridSheet is what a month calendar or habit tracker page draws below its
// title: boxes with thin grey borders, lines of text in the Go font and
// grey writing lines.
type gridSheet struct {
	Boxes []Box
	Texts []moonText
	Lines []moonRule
}

// addRegularTextFont embeds the regular text font in pdf: the labels of
// the calendar kinds are all regular, so only that face is embedded (the
// default lettering font adds its bold face for the title).
func addRegularTextFont(pdf *pdfDoc) error {
	pdf.AddUTF8FontFromBytes(textFont, "", goregular.TTF)
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("load fonts: %w", err)
	}
	return nil
}

// drawPageTitle draws title, if any, centred in its band at the top of
// page in the lettering l.
func drawPageTitle(pdf *pdfDoc, page Page, title string, l lettering) {
	if title == "" {
		return
	}
	band := dotGridTitleBand(page)
	drawHeader(pdf, l, band, fitHeader(pdf, l, title, band.W))
}

// drawGridSheet draws s on the current page.
func drawGridSheet(pdf *pdfDoc, s gridSheet) {
	pdf.SetDrawColor(writingGrey, writingGrey, writingGrey)
	pdf.SetLineWidth(calendarLineWidth)
	for _, b := range s.Boxes {
		pdf.Rect(b.X, b.Y, b.W, b.H, "D")
	}
	for _, r := range s.Lines {
		pdf.Line(r.X1, r.Y, r.X2, r.Y)
	}
	pdf.SetTextColor(textGrey, textGrey, textGrey)
	for _, t := range s.Texts {
		pdf.SetFont(textFont, "", t.Size)
		x := t.Box.X
		if t.Centred {
			x += (t.Box.W - pdf.GetStringWidth(t.Text)) / 2
		}
		pdf.Text(x, baseline(t.Box.Y+(t.Box.H-lineHeight(t.Size))/2, t.Size), t.Text)
	}
}
