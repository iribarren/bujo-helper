package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"golang.org/x/image/font/gofont/goregular"
)

// MoonKind is the ID of the moon calendar page kind: a month or a year of
// moon phases, with the new and full moons marked.
const MoonKind = "moon"

// Moon calendar layouts and hemispheres.
const (
	MoonRow   = "row"  // one line per day, in two columns
	MoonGrid  = "grid" // a month grid, weeks starting on Monday
	MoonYear  = "year" // 12 month rows of days
	MoonNorth = "north"
	MoonSouth = "south"
)

// MoonLayouts and MoonHemispheres are the values to choose from.
var (
	MoonLayouts     = []string{MoonRow, MoonGrid, MoonYear}
	MoonHemispheres = []string{MoonNorth, MoonSouth}
)

// Supported years of a moon calendar.
const (
	MinMoonYear = 1900
	MaxMoonYear = 2100
)

// Moon calendar drawing settings: lengths in millimetres, font sizes in
// points.
const (
	moonLabelSize   = 9.0  // row labels and marks, grid day numbers
	moonHeaderSize  = 8.0  // grid weekday headers
	moonMarkSize    = 7.0  // grid marks, year month names
	moonYearDaySize = 6.0  // year day-number header
	moonInitialSize = 5.0  // year marks, the mark's initial
	moonRuleWidth   = 0.2  // grey rules
	moonLineWidth   = 0.2  // moon outlines
	moonDark        = 0x3b // the dark disc, #3b4252, with moonDarkG and B
	moonDarkG       = 0x42
	moonDarkB       = 0x52
	moonRowGap      = 6.0  // between the two columns of the row layout
	moonMaxRow      = 14.0 // tallest row of the row layout
	moonMaxDisc     = 10.0 // largest moon of the row layout
	moonYearLabel   = 9.0  // the month-name column of the year layout
	// moonKappa places the control points of a cubic Bézier quarter of a
	// circle or ellipse.
	moonKappa = 0.5522847498
)

// The clock and time zone a moon calendar resolves "this month" and its
// calendar days in; tests replace them.
var (
	moonNow      = time.Now
	moonLocation = time.Local
)

// MoonOptions are the options of a moon calendar page.
type MoonOptions struct {
	Month      int    `json:"month"`      // 1–12; 0 means the current month
	Year       int    `json:"year"`       // MinMoonYear–MaxMoonYear; 0 means the current year
	Layout     string `json:"layout"`     // one of MoonLayouts
	Hemisphere string `json:"hemisphere"` // one of MoonHemispheres
}

var defaultMoon = MoonOptions{Layout: MoonGrid, Hemisphere: MoonNorth}

// decodeMoon decodes moon calendar options, like decodeOptions, and
// resolves a month or year of 0 to the current one in moonLocation. A
// current year outside MinMoonYear–MaxMoonYear (a wrong clock) is an error.
func decodeMoon(raw json.RawMessage) (pageDoc, error) {
	doc, err := decodeOptions(defaultMoon)(raw)
	if err != nil {
		return nil, err
	}
	o := doc.(MoonOptions)
	now := moonNow().In(moonLocation)
	if o.Month == 0 {
		o.Month = int(now.Month())
	}
	if o.Year == 0 {
		o.Year = now.Year()
		if o.Year < MinMoonYear || o.Year > MaxMoonYear {
			return nil, fmt.Errorf("moon calendar: the current year %d is not %d to %d; choose a year", o.Year, MinMoonYear, MaxMoonYear)
		}
	}
	return o, nil
}

func (o MoonOptions) validate() error {
	if o.Month < 0 || o.Month > 12 {
		return fmt.Errorf("moon calendar month %d: want 1 to 12", o.Month)
	}
	if o.Year != 0 && (o.Year < MinMoonYear || o.Year > MaxMoonYear) {
		return fmt.Errorf("moon calendar year %d: want %d to %d", o.Year, MinMoonYear, MaxMoonYear)
	}
	if !oneOf(o.Layout, MoonLayouts) {
		return fmt.Errorf("moon calendar layout %q: want one of %v", o.Layout, MoonLayouts)
	}
	if !oneOf(o.Hemisphere, MoonHemispheres) {
		return fmt.Errorf("moon calendar hemisphere %q: want one of %v", o.Hemisphere, MoonHemispheres)
	}
	return nil
}

// oneOf reports whether s is in list.
func oneOf(s string, list []string) bool {
	for _, v := range list {
		if s == v {
			return true
		}
	}
	return false
}

// pages is always 1: a month and a year each fit one page.
func (o MoonOptions) pages() int { return 1 }

// defaultTitle is "<Month> <Year>", or "<Year>" for the year layout.
func (o MoonOptions) defaultTitle(labels PageLabels) string {
	year := strconv.Itoa(o.Year)
	if o.Layout == MoonYear {
		return year
	}
	return labels.Months[o.Month-1] + " " + year
}

// draw draws the page: the title centred in its band at the top in the
// lettering l, and the calendar below it, its days in moonLocation.
func (o MoonOptions) draw(pdf *pdfDoc, page Page, title string, l lettering, labels PageLabels) error {
	// Labels are all regular: only that face of the text font is embedded
	// (the default lettering font adds its bold face for the title).
	pdf.AddUTF8FontFromBytes(textFont, "", goregular.TTF)
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("load fonts: %w", err)
	}
	pdf.AddPage()
	if title != "" {
		band := dotGridTitleBand(page)
		drawHeader(pdf, l, band, fitHeader(pdf, l, title, band.W))
	}
	s := o.sheet(page, labels, moonLocation)

	pdf.SetDrawColor(writingGrey, writingGrey, writingGrey)
	pdf.SetLineWidth(moonRuleWidth)
	for _, r := range s.Rules {
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
	south := o.Hemisphere == MoonSouth
	for _, m := range s.Moons {
		drawMoon(pdf, m, south, l.colourIn)
	}
	return pdf.Error()
}

// moonCircle is one day's moon: centre, radius and phase (see moonDay).
type moonCircle struct {
	C     Point
	R     float64
	Phase float64
}

// moonText is a line of text in the Go font: left-aligned or centred in
// Box, centred down it.
type moonText struct {
	Text    string
	Box     Box
	Size    float64
	Centred bool
}

// moonRule is a horizontal grey rule from X1 to X2 at Y.
type moonRule struct {
	X1, X2, Y float64
}

// moonSheet is what a moon calendar page draws below its title.
type moonSheet struct {
	Moons []moonCircle
	Texts []moonText
	Rules []moonRule
}

// moonArea is the space a moon calendar takes: within MinMargin margins,
// below the title band.
func moonArea(page Page) Box {
	top := MinMargin + DotGridTitleHeight
	return Box{X: MinMargin, Y: top, W: page.Width - 2*MinMargin, H: page.Height - MinMargin - top}
}

// sheet lays out the calendar o describes on page, its days in loc.
func (o MoonOptions) sheet(page Page, labels PageLabels, loc *time.Location) moonSheet {
	area := moonArea(page)
	if o.Layout == MoonYear {
		months := make([][]moonDay, 12)
		for i := range months {
			months[i] = moonMonth(o.Year, time.Month(i+1), loc)
		}
		return layoutMoonYear(area, months, labels)
	}
	month := time.Month(o.Month)
	days := moonMonth(o.Year, month, loc)
	// Monday is 0.
	first := (int(time.Date(o.Year, month, 1, 0, 0, 0, 0, time.UTC).Weekday()) + 6) % 7
	if o.Layout == MoonRow {
		return layoutMoonRow(area, days, first, labels)
	}
	return layoutMoonGrid(area, days, first, labels)
}

// moonMark is the mark of a day's event in labels; "" for none.
func moonMark(e moonEvent, labels PageLabels) string {
	switch e {
	case newMoonEvent:
		return labels.NewMoon
	case fullMoonEvent:
		return labels.FullMoon
	}
	return ""
}

// layoutMoonRow lays out days, the first on weekday first (Monday 0), one
// per row in two columns, days 1–16 left and the rest right: the day
// number and weekday, the moon and the mark, a grey rule under each row.
// Rows are as tall as fit, up to moonMaxRow, and the block is centred
// down area.
func layoutMoonRow(area Box, days []moonDay, first int, labels PageLabels) moonSheet {
	const rows = 16
	colW := (area.W - moonRowGap) / 2
	rowH := min(area.H/rows, moonMaxRow)
	top := area.Y + (area.H-rowH*rows)/2
	d := min(rowH*0.7, moonMaxDisc)
	labelW := colW * 0.4
	var s moonSheet
	for i, day := range days {
		x := area.X + float64(i/rows)*(colW+moonRowGap)
		y := top + float64(i%rows)*rowH
		label := fmt.Sprintf("%d %s", day.Day, labels.Weekdays[(first+i)%7])
		s.Texts = append(s.Texts, moonText{Text: label, Box: Box{X: x, Y: y, W: labelW, H: rowH}, Size: moonLabelSize})
		s.Moons = append(s.Moons, moonCircle{C: Point{X: x + labelW + d/2, Y: y + rowH/2}, R: d / 2, Phase: day.Phase})
		if mark := moonMark(day.Event, labels); mark != "" {
			markX := x + labelW + d + 2
			s.Texts = append(s.Texts, moonText{Text: mark, Box: Box{X: markX, Y: y, W: x + colW - markX, H: rowH}, Size: moonLabelSize})
		}
		s.Rules = append(s.Rules, moonRule{X1: x, X2: x + colW, Y: y + rowH})
	}
	return s
}

// layoutMoonGrid lays out days, the first on weekday first (Monday 0), in
// a 7-column month grid under a weekday header row: a grey rule along the
// top of each day's cell, the day number at its top left, the moon
// centred and the mark below it. Cells are as tall as fit, up to 1.25
// times their width, and the grid is centred down area.
func layoutMoonGrid(area Box, days []moonDay, first int, labels PageLabels) moonSheet {
	cellW := area.W / 7
	headerH := lineHeight(moonHeaderSize) + 2
	weeks := (first + len(days) + 6) / 7
	cellH := min((area.H-headerH)/float64(weeks), cellW*1.25)
	top := area.Y + (area.H-headerH-cellH*float64(weeks))/2
	const pad = 1.0
	numberH := pad + lineHeight(moonLabelSize)
	markH := lineHeight(moonMarkSize) + pad
	space := cellH - numberH - markH
	d := min(cellW*0.6, space*0.9)

	var s moonSheet
	for c, wd := range labels.Weekdays {
		s.Texts = append(s.Texts, moonText{Text: wd, Box: Box{X: area.X + float64(c)*cellW, Y: top, W: cellW, H: headerH}, Size: moonHeaderSize, Centred: true})
	}
	for i, day := range days {
		pos := first + i
		x := area.X + float64(pos%7)*cellW
		y := top + headerH + float64(pos/7)*cellH
		s.Rules = append(s.Rules, moonRule{X1: x + pad, X2: x + cellW - pad, Y: y})
		s.Texts = append(s.Texts, moonText{Text: strconv.Itoa(day.Day), Box: Box{X: x + pad, Y: y + pad, W: cellW - 2*pad, H: lineHeight(moonLabelSize)}, Size: moonLabelSize})
		s.Moons = append(s.Moons, moonCircle{C: Point{X: x + cellW/2, Y: y + numberH + space/2}, R: d / 2, Phase: day.Phase})
		if mark := moonMark(day.Event, labels); mark != "" {
			s.Texts = append(s.Texts, moonText{Text: mark, Box: Box{X: x + pad, Y: y + numberH + space, W: cellW - 2*pad, H: lineHeight(moonMarkSize)}, Size: moonMarkSize, Centred: true})
		}
	}
	return s
}

// layoutMoonYear lays out the 12 months, one row each: the short month
// name (its first three letters) on the left and a moon per day in 31
// columns under a day-number header row, with a grey rule under each row.
// A new or full moon is marked by its mark's initial under the moon.
// Rows are as tall as fit, up to three columns wide, and the block is
// centred down area.
func layoutMoonYear(area Box, months [][]moonDay, labels PageLabels) moonSheet {
	cellW := (area.W - moonYearLabel) / 31
	headerH := lineHeight(moonYearDaySize) + 1
	rowH := min((area.H-headerH)/12, cellW*3)
	top := area.Y + (area.H-headerH-rowH*12)/2
	d := cellW * 0.8
	left := area.X + moonYearLabel

	var s moonSheet
	for day := 0; day < 31; day++ {
		s.Texts = append(s.Texts, moonText{Text: strconv.Itoa(day + 1), Box: Box{X: left + float64(day)*cellW, Y: top, W: cellW, H: headerH}, Size: moonYearDaySize, Centred: true})
	}
	initialH := lineHeight(moonInitialSize)
	for m, days := range months {
		y := top + headerH + float64(m)*rowH
		s.Texts = append(s.Texts, moonText{Text: firstRunes(labels.Months[m], 3), Box: Box{X: area.X, Y: y, W: moonYearLabel, H: rowH}, Size: moonMarkSize})
		// Moon and initial together, centred down the row.
		cy := y + (rowH-initialH)/2
		for i, day := range days {
			x := left + float64(i)*cellW
			s.Moons = append(s.Moons, moonCircle{C: Point{X: x + cellW/2, Y: cy}, R: d / 2, Phase: day.Phase})
			if mark := moonMark(day.Event, labels); mark != "" {
				s.Texts = append(s.Texts, moonText{Text: firstRunes(mark, 1), Box: Box{X: x, Y: cy + d/2, W: cellW, H: initialH}, Size: moonInitialSize, Centred: true})
			}
		}
		s.Rules = append(s.Rules, moonRule{X1: area.X, X2: area.X + area.W, Y: y + rowH})
	}
	return s
}

// firstRunes is the first n runes of s.
func firstRunes(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}

// moonPath is the outline of a moon's lit part, from Start: four cubic
// Bézier curves, each its two control points and its end. The first two
// follow the limb on the lit side from the top to the bottom, through the
// equator; the last two the terminator back up to the top.
type moonPath struct {
	Start  Point
	Curves [4][3]Point
}

// moonLitPath is the outline of the lit part of a moon of radius r centred
// at c, at phase p (see moonDay): a half circle on the lit side, the right
// one for a waxing moon in the north, and a half ellipse for the
// terminator, r·|cos 2πp| wide, bulging towards the lit side for a
// crescent and away from it for a gibbous moon. The south mirrors it.
func moonLitPath(c Point, r, p float64, south bool) moonPath {
	side := 1.0 // the lit limb's side: +1 right, -1 left
	if p >= 0.5 {
		side = -1
	}
	if south {
		side = -side
	}
	// The terminator's signed half-width, from the centre: cos 2πp is
	// positive for a crescent (lit side) and negative for a gibbous moon.
	t := side * r * math.Cos(2*math.Pi*p)
	k := moonKappa
	limb := side * r
	top, bottom := Point{X: c.X, Y: c.Y - r}, Point{X: c.X, Y: c.Y + r}
	return moonPath{
		Start: top,
		Curves: [4][3]Point{
			{{X: c.X + k*limb, Y: top.Y}, {X: c.X + limb, Y: c.Y - k*r}, {X: c.X + limb, Y: c.Y}},
			{{X: c.X + limb, Y: c.Y + k*r}, {X: c.X + k*limb, Y: bottom.Y}, bottom},
			{{X: c.X + k*t, Y: bottom.Y}, {X: c.X + t, Y: c.Y + k*r}, {X: c.X + t, Y: c.Y}},
			{{X: c.X + t, Y: c.Y - k*r}, {X: c.X + k*t, Y: top.Y}, top},
		},
	}
}

// drawMoon draws m: filled, a dark disc with its lit part white and a thin
// dark outline; with colourIn, no fills, only the disc's outline and the
// lit part's (the terminator). It restores the fill and draw colours and
// the line width.
func drawMoon(pdf *pdfDoc, m moonCircle, south, colourIn bool) {
	fr, fg, fb := pdf.GetFillColor()
	dr, dg, db := pdf.GetDrawColor()
	width := pdf.GetLineWidth()
	defer func() {
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(dr, dg, db)
		pdf.SetLineWidth(width)
	}()
	pdf.SetDrawColor(moonDark, moonDarkG, moonDarkB)
	pdf.SetLineWidth(moonLineWidth)
	path := moonLitPath(m.C, m.R, m.Phase, south)
	trace := func() {
		pdf.MoveTo(path.Start.X, path.Start.Y)
		for _, c := range path.Curves {
			pdf.CurveBezierCubicTo(c[0].X, c[0].Y, c[1].X, c[1].Y, c[2].X, c[2].Y)
		}
		pdf.ClosePath()
	}
	if colourIn {
		pdf.Circle(m.C.X, m.C.Y, m.R, "D")
		trace()
		pdf.DrawPath("D")
		return
	}
	pdf.SetFillColor(moonDark, moonDarkG, moonDarkB)
	pdf.Circle(m.C.X, m.C.Y, m.R, "F")
	pdf.SetFillColor(0xff, 0xff, 0xff)
	trace()
	pdf.DrawPath("F")
	pdf.Circle(m.C.X, m.C.Y, m.R, "D")
}
