package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"
)

// fixMoonClock pins the moon calendar's clock to now and its time zone to
// UTC for the test.
func fixMoonClock(t *testing.T, now time.Time) {
	t.Helper()
	oldNow, oldLoc := moonNow, moonLocation
	moonNow = func() time.Time { return now }
	moonLocation = time.UTC
	t.Cleanup(func() { moonNow, moonLocation = oldNow, oldLoc })
}

// spanishPageLabels are the Spanish labels the app passes.
var spanishPageLabels = PageLabels{
	Months: []string{"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio",
		"Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"},
	Weekdays: []string{"Lun", "Mar", "Mié", "Jue", "Vie", "Sáb", "Dom"},
	NewMoon:  "Nueva",
	FullMoon: "Llena",
}

func TestDecodeMoonOptions(t *testing.T) {
	fixMoonClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	kind, err := PageKindByID(MoonKind)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		raw     string
		want    MoonOptions
		wantErr string // part of the error; "" for none
	}{
		{"no options: this month, grid, north", "", MoonOptions{11, 2026, "grid", "north"}, ""},
		{"empty object", "{}", MoonOptions{11, 2026, "grid", "north"}, ""},
		{"explicit", `{"month":2,"year":2024,"layout":"row","hemisphere":"south"}`, MoonOptions{2, 2024, "row", "south"}, ""},
		{"year layout", `{"month":0,"year":1900,"layout":"year"}`, MoonOptions{11, 1900, "year", "north"}, ""},
		{"only the year is current", `{"month":3,"year":0}`, MoonOptions{3, 2026, "grid", "north"}, ""},
		{"last year", `{"year":2100,"month":12}`, MoonOptions{12, 2100, "grid", "north"}, ""},
		{"month 13", `{"month":13}`, MoonOptions{}, "13"},
		{"negative month", `{"month":-1}`, MoonOptions{}, "-1"},
		{"year too early", `{"year":1899}`, MoonOptions{}, "1899"},
		{"year too late", `{"year":2101}`, MoonOptions{}, "2101"},
		{"unknown layout", `{"layout":"week"}`, MoonOptions{}, `"week"`},
		{"unknown hemisphere", `{"hemisphere":"east"}`, MoonOptions{}, `"east"`},
		{"unknown field", `{"month":1,"quarters":true}`, MoonOptions{}, `"quarters"`},
		{"month as a string", `{"month":"1"}`, MoonOptions{}, "month"},
		{"trailing data", `{} {}`, MoonOptions{}, "options"},
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
			if got, ok := doc.(MoonOptions); !ok || got != tt.want {
				t.Errorf("decoded %#v, want %#v", doc, tt.want)
			}
		})
	}
}

// The current month and year come from the clock in the calendar's time
// zone: 23:30 UTC on 31 December is already January two hours east.
func TestMoonOptionsCurrentMonthInTheLocalZone(t *testing.T) {
	fixMoonClock(t, time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC))
	moonLocation = time.FixedZone("EET", 2*3600)
	doc, err := decodeMoon(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.(MoonOptions); got.Month != 1 || got.Year != 2027 {
		t.Errorf("current month %d/%d, want 1/2027", got.Month, got.Year)
	}
}

// A current year outside 1900–2100 (a wrong system clock) is rejected like
// an explicit one; an explicit year does not depend on the clock.
func TestMoonOptionsCurrentYearOutOfRange(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixMoonClock(t, tt.now)
			_, err := decodeMoon(json.RawMessage(tt.raw))
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

func TestMoonRegistryEntry(t *testing.T) {
	k, err := PageKindByID(MoonKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Moon calendar" || k.FileTag != "moon" || !k.UsesTitle || k.UsesCut {
		t.Errorf("moon kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{"month":0,"year":0,"layout":"grid","hemisphere":"north"}` {
		t.Errorf("defaults %s", raw)
	}
}

func TestMoonDefaultTitle(t *testing.T) {
	tests := []struct {
		layout string
		labels PageLabels
		want   string
	}{
		{"grid", PageLabels{}, "February 2024"},
		{"row", PageLabels{}, "February 2024"},
		{"year", PageLabels{}, "2024"},
		{"grid", spanishPageLabels, "Febrero 2024"},
		{"row", spanishPageLabels, "Febrero 2024"},
		{"year", spanishPageLabels, "2024"},
	}
	for _, tt := range tests {
		o := MoonOptions{Month: 2, Year: 2024, Layout: tt.layout, Hemisphere: "north"}
		if got := o.defaultTitle(tt.labels.resolved()); got != tt.want {
			t.Errorf("%s, %v: title %q, want %q", tt.layout, tt.labels.Months != nil, got, tt.want)
		}
	}
}

// Missing, blank or incomplete labels fall back to English.
func TestPageLabelsResolved(t *testing.T) {
	got := PageLabels{
		Months:   []string{"Enero"}, // not 12: all English
		Weekdays: []string{"Lun", " ", "Mié", "Jue", "Vie", "Sáb", "Dom"},
		FullMoon: " Llena ",
	}.resolved()
	if got.Months[0] != "January" || got.Months[11] != "December" || len(got.Months) != 12 {
		t.Errorf("months %v", got.Months)
	}
	if strings.Join(got.Weekdays, ",") != "Lun,Tue,Mié,Jue,Vie,Sáb,Dom" {
		t.Errorf("weekdays %v", got.Weekdays)
	}
	if got.NewMoon != "New" || got.FullMoon != "Llena" {
		t.Errorf("marks %q, %q", got.NewMoon, got.FullMoon)
	}
	if es := spanishPageLabels.resolved(); es.Months[8] != "Septiembre" || es.Weekdays[2] != "Mié" || es.NewMoon != "Nueva" {
		t.Errorf("Spanish labels changed: %+v", es)
	}
}

// moonLayouts are the layouts in test order.
var moonLayouts = []string{"row", "grid", "year"}

// On every paper and layout, in English and Spanish, every moon and label
// lies inside the margins below the title band, moons do not overlap, and
// each label fits its box.
func TestMoonSheetGeometry(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	// May 2026 starts on a Friday: six week rows in the grid.
	months := []time.Month{time.February, time.May}
	for _, p := range Papers {
		for _, layout := range moonLayouts {
			for _, labels := range []PageLabels{{}, spanishPageLabels} {
				for _, month := range months {
					name := fmt.Sprintf("%s/%s/%s/%v", p.ID, layout, month, labels.NewMoon)
					t.Run(name, func(t *testing.T) {
						o := MoonOptions{Month: int(month), Year: 2026, Layout: layout, Hemisphere: "north"}
						s := o.sheet(p.Page, labels.resolved(), time.UTC)
						area := moonArea(p.Page)
						checkMoonSheet(t, s, area, measure)
						wantMoons := daysIn(2026, month)
						if layout == "year" {
							wantMoons = 365
						}
						if len(s.Moons) != wantMoons {
							t.Errorf("%d moons, want %d", len(s.Moons), wantMoons)
						}
					})
				}
			}
		}
	}
}

// checkMoonSheet checks that everything on s lies within area, that moons
// do not overlap and that each text fits its box.
func checkMoonSheet(t *testing.T, s moonSheet, area Box, measure measureFunc) {
	t.Helper()
	const eps = 1e-9
	inside := func(what string, b Box) {
		if b.X < area.X-eps || b.Y < area.Y-eps || b.X+b.W > area.X+area.W+eps || b.Y+b.H > area.Y+area.H+eps {
			t.Errorf("%s %+v leaves the area %+v", what, b, area)
		}
	}
	for i, m := range s.Moons {
		if m.R <= 0.5 {
			t.Errorf("moon %d: radius %v mm", i, m.R)
		}
		inside(fmt.Sprintf("moon %d", i), Box{X: m.C.X - m.R, Y: m.C.Y - m.R, W: 2 * m.R, H: 2 * m.R})
		for j := i + 1; j < len(s.Moons); j++ {
			n := s.Moons[j]
			if math.Hypot(m.C.X-n.C.X, m.C.Y-n.C.Y) < m.R+n.R-eps {
				t.Errorf("moons %d and %d overlap", i, j)
			}
		}
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
	for _, r := range s.Rules {
		inside("rule", Box{X: r.X1, Y: r.Y, W: r.X2 - r.X1, H: 0})
	}
}

// The layouts print the labels the drawing promises: weekday headers in
// the grid, a weekday per day in the row, month names in the year, and
// one mark per new or full moon.
func TestMoonSheetLabels(t *testing.T) {
	es := spanishPageLabels.resolved()
	texts := func(s moonSheet) map[string]int {
		out := map[string]int{}
		for _, tx := range s.Texts {
			out[tx.Text]++
		}
		return out
	}
	// February 2024 (UTC): new moon on the 9th, full moon on the 24th.
	grid := texts(MoonOptions{Month: 2, Year: 2024, Layout: "grid"}.sheet(A4, es, time.UTC))
	for _, wd := range es.Weekdays {
		if grid[wd] != 1 {
			t.Errorf("grid: weekday %q printed %d times", wd, grid[wd])
		}
	}
	if grid["Nueva"] != 1 || grid["Llena"] != 1 || grid["29"] != 1 {
		t.Errorf("grid texts %v", grid)
	}
	row := texts(MoonOptions{Month: 2, Year: 2024, Layout: "row"}.sheet(A4, es, time.UTC))
	// 1 February 2024 was a Thursday.
	if row["1 Jue"] != 1 || row["29 Jue"] != 1 || row["Nueva"] != 1 || row["Llena"] != 1 {
		t.Errorf("row texts %v", row)
	}
	year := texts(MoonOptions{Month: 2, Year: 2024, Layout: "year"}.sheet(A4, es, time.UTC))
	if year["Ene"] != 1 || year["Sep"] != 1 || year["Dic"] != 1 || year["31"] != 1 {
		t.Errorf("year texts %v", year)
	}
	// 2024: 13 new and 12 full moons, marked by their initials.
	if year["N"] != 13 || year["L"] != 12 {
		t.Errorf("year marks: %d N, %d L", year["N"], year["L"])
	}
}

// The grid starts weeks on Monday: 1 February 2024, a Thursday, is in the
// fourth column, and 1 April 2024, a Monday, in the first.
func TestMoonGridStartsOnMonday(t *testing.T) {
	for _, tt := range []struct {
		month time.Month
		col   int
	}{{time.February, 3}, {time.April, 0}, {time.September, 6}} {
		s := MoonOptions{Month: int(tt.month), Year: 2024, Layout: "grid"}.sheet(A4, PageLabels{}.resolved(), time.UTC)
		area := moonArea(A4)
		cellW := area.W / 7
		if got := int((s.Moons[0].C.X - area.X) / cellW); got != tt.col {
			t.Errorf("%v: day 1 in column %d, want %d", tt.month, got, tt.col)
		}
	}
}

// The lit part of a waxing moon is on the right in the north and on the
// left in the south; a waning one the other way round. The south path is
// the north one mirrored.
func TestMoonLitPathHemisphere(t *testing.T) {
	c := Point{X: 50, Y: 50}
	const r = 10.0
	side := func(p moonPath) (minX, maxX float64) {
		minX, maxX = p.Start.X, p.Start.X
		for _, seg := range p.Curves {
			for _, q := range seg {
				minX, maxX = math.Min(minX, q.X), math.Max(maxX, q.X)
			}
		}
		return minX, maxX
	}
	for _, tt := range []struct {
		name  string
		phase float64
		south bool
		right bool // lit part entirely right of the centre, else left
	}{
		{"waxing crescent north", 0.1, false, true},
		{"waxing crescent south", 0.1, true, false},
		{"waning crescent north", 0.9, false, false},
		{"waning crescent south", 0.9, true, true},
	} {
		minX, maxX := side(moonLitPath(c, r, tt.phase, tt.south))
		if tt.right && minX < c.X-1e-9 || !tt.right && maxX > c.X+1e-9 {
			t.Errorf("%s: lit part spans x %v..%v around %v", tt.name, minX, maxX, c.X)
		}
	}
	for _, phase := range []float64{0, 0.1, 0.25, 0.4, 0.5, 0.6, 0.75, 0.9} {
		n, s := moonLitPath(c, r, phase, false), moonLitPath(c, r, phase, true)
		mirror := func(q Point) Point { return Point{X: 2*c.X - q.X, Y: q.Y} }
		ok := pointsNear(mirror(n.Start), s.Start)
		for i := range n.Curves {
			for j := range n.Curves[i] {
				ok = ok && pointsNear(mirror(n.Curves[i][j]), s.Curves[i][j])
			}
		}
		if !ok {
			t.Errorf("phase %v: south path is not the north one mirrored", phase)
		}
	}
}

// The terminator crosses the equator r·|cos 2πp| from the centre: on the
// lit side for a crescent, on the dark side for a gibbous moon, through
// the centre at the quarters and along the limb at new and full moon.
func TestMoonLitPathTerminator(t *testing.T) {
	c := Point{X: 0, Y: 0}
	const r = 10.0
	for _, tt := range []struct {
		phase float64
		want  float64 // x where the terminator crosses y = 0, north
	}{
		{0.1, r * math.Cos(2*math.Pi*0.1)},  // waxing crescent: right
		{0.25, 0},                           // first quarter
		{0.4, -r * math.Cos(2*math.Pi*0.1)}, // waxing gibbous: left
		{0.5, r},                            // full: from 0.5 the lit limb is the left one and the terminator runs along the right limb
		{0.6, r * math.Cos(2*math.Pi*0.1)},  // waning gibbous: right
		{0.9, -r * math.Cos(2*math.Pi*0.1)}, // waning crescent: left
		{0.0, r},                            // new: on the lit limb
	} {
		p := moonLitPath(c, r, tt.phase, false)
		// Curves: limb to the equator, limb to the bottom, terminator to
		// the equator, terminator back to the top.
		if got := p.Curves[2][2]; !near(got.X, tt.want) || !near(got.Y, 0) {
			t.Errorf("phase %v: terminator at %+v, want x %v", tt.phase, got, tt.want)
		}
	}
}

func pointsNear(a, b Point) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

// paintOps matches PDF path-painting operators that fill.
var paintOps = regexp.MustCompile(`(?m)(?:^| )(f\*?|F|B\*?|b\*?)$`)

// The moon calendar renders on every paper and layout, filled or for
// colouring in, north or south: one page, two fills per moon (dark disc,
// lit part) when filled and none at all with Colour-in.
func TestBuildMoonEveryPaperAndLayout(t *testing.T) {
	fixMoonClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	for _, p := range Papers {
		for _, layout := range moonLayouts {
			for _, colourIn := range []bool{false, true} {
				for _, hemi := range []string{"north", "south"} {
					name := fmt.Sprintf("%s/%s/colourIn=%v/%s", p.ID, layout, colourIn, hemi)
					t.Run(name, func(t *testing.T) {
						opts := fmt.Sprintf(`{"month":2,"year":2024,"layout":%q,"hemisphere":%q}`, layout, hemi)
						out := renderPage(t, MoonKind, PageSettings{Paper: p.ID, ColourIn: colourIn}, opts)
						if n := strings.Count(string(out), "/Type /Page\n"); n != 1 {
							t.Errorf("%d pages, want 1", n)
						}
						stream := pageStreams(out)
						moons := 29
						if layout == "year" {
							moons = 366
						}
						fills := len(paintOps.FindAllString(stream, -1))
						if colourIn && fills != 0 {
							t.Errorf("Colour-in: %d fills, want none", fills)
						}
						if !colourIn && fills != 2*moons {
							t.Errorf("filled: %d fills, want %d", fills, 2*moons)
						}
						if !strings.Contains(stream, " Tj") {
							t.Error("no text drawn")
						}
					})
				}
			}
		}
	}
}

// The moon kind always prints a title: the shared one, else its default.
// The default is drawn like a set title, and a set title replaces it.
func TestBuildMoonTitle(t *testing.T) {
	fixMoonClock(t, time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC))
	opts := `{"month":2,"year":2024}`
	none := pageStreams(renderPage(t, MoonKind, PageSettings{}, opts))
	blank := pageStreams(renderPage(t, MoonKind, PageSettings{Title: "  "}, opts))
	explicit := pageStreams(renderPage(t, MoonKind, PageSettings{Title: "February 2024"}, opts))
	other := pageStreams(renderPage(t, MoonKind, PageSettings{Title: "Moon log"}, opts))
	if none != blank || none != explicit {
		t.Error("the default title is not the localized \"February 2024\"")
	}
	if none == other {
		t.Error("a set title did not replace the default")
	}
	es := pageStreams(renderPage(t, MoonKind, PageSettings{Labels: spanishPageLabels}, opts))
	esExplicit := pageStreams(renderPage(t, MoonKind, PageSettings{Labels: spanishPageLabels, Title: "Febrero 2024"}, opts))
	if es != esExplicit || es == none {
		t.Error("the Spanish default title is not \"Febrero 2024\"")
	}
	// The title is in the lettering font, here Pacifico, and the labels in
	// the regular Go font only.
	out := string(renderPage(t, MoonKind, PageSettings{Font: "pacifico"}, opts))
	if !strings.Contains(out, "/BaseFont /utf8pacifico") || !strings.Contains(out, "/BaseFont /utf8go\n") {
		t.Error("the default title is not in the lettering font")
	}
	if strings.Contains(out, "/BaseFont /utf8goB") {
		t.Error("the bold Go font is embedded for regular labels")
	}
}

// Labels change only the words: the dot grid ignores them.
func TestBuildDotGridIgnoresLabels(t *testing.T) {
	plain := pageStreams(renderPage(t, DotGridKind, PageSettings{Title: "Ideas"}, ""))
	es := pageStreams(renderPage(t, DotGridKind, PageSettings{Title: "Ideas", Labels: spanishPageLabels}, ""))
	if plain != es {
		t.Error("labels changed a dot grid")
	}
}
