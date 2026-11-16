package sheet

import (
	"fmt"
	"math"
	"testing"
)

func TestStubGridPapers(t *testing.T) {
	tests := []struct {
		paper            string
		page             Page
		cols, rows       int
		marginX, marginY float64
	}{
		// (210 - 2·90 - 4) / 2 = 13; (297 - 6·40 - 5·4) / 2 = 18.5.
		{"a4", A4, 2, 6, 13, 18.5},
		// (215.9 - 184) / 2 = 15.95; (279.4 - 260) / 2 = 9.7.
		{"letter", Letter, 2, 6, 15.95, 9.7},
		// (148 - 90) / 2 = 29; (210 - 4·40 - 3·4) / 2 = 19.
		{"a5", A5, 1, 4, 29, 19},
	}
	for _, tt := range tests {
		g, err := Options{Format: "stubs", Paper: tt.paper}.Grid()
		if err != nil {
			t.Fatalf("%s: %v", tt.paper, err)
		}
		if g.Page != tt.page || g.Columns != tt.cols || g.Rows != tt.rows ||
			!near(g.CellWidth, StubWidth) || !near(g.CoverHeight, StubHeight) ||
			!near(g.MarginX, tt.marginX) || !near(g.MarginY, tt.marginY) {
			t.Errorf("%s: grid %+v, want %d x %d stubs, margins %v / %v", tt.paper, g, tt.cols, tt.rows, tt.marginX, tt.marginY)
		}
		if g.MarginX < MinMargin || g.MarginY < MinMargin {
			t.Errorf("%s: margins %v / %v below %v", tt.paper, g.MarginX, g.MarginY, MinMargin)
		}
		// Full: no room for another column or row.
		if float64(g.Columns+1)*StubWidth+float64(g.Columns)*Gap <= g.Page.Width-2*MinMargin {
			t.Errorf("%s: %d columns fit, grid has %d", tt.paper, g.Columns+1, g.Columns)
		}
		if float64(g.Rows+1)*StubHeight+float64(g.Rows)*Gap <= g.Page.Height-2*MinMargin {
			t.Errorf("%s: %d rows fit, grid has %d", tt.paper, g.Rows+1, g.Rows)
		}
		// Centred both ways: the last stub leaves the same margins.
		pages, err := g.Cells(g.PerPage())
		if err != nil {
			t.Fatal(err)
		}
		last := pages[0][len(pages[0])-1]
		if right, bottom := g.Page.Width-(last.X+last.W), g.Page.Height-(last.Y+last.H); !near(right, g.MarginX) || !near(bottom, g.MarginY) {
			t.Errorf("%s: right / bottom margins %v / %v, want %v / %v", tt.paper, right, bottom, g.MarginX, g.MarginY)
		}
	}
}

func TestNewStubGridRejectsTinyPages(t *testing.T) {
	if g, err := NewStubGrid(Page{Width: 100, Height: 297}); err == nil {
		t.Errorf("NewStubGrid on a 100 mm wide page = %+v, want an error", g)
	}
	if g, err := NewStubGrid(Page{Width: 210, Height: 50}); err == nil {
		t.Errorf("NewStubGrid on a 50 mm tall page = %+v, want an error", g)
	}
}

// stubRatios are posters of every shape: very tall, typical, 2:3, square,
// wide and panoramic.
var stubRatios = []float64{0.3, 0.656, 2.0 / 3, 1, 16.0 / 9, 3}

func TestLayoutStubPartsInsideTheStub(t *testing.T) {
	for _, p := range Papers {
		g, err := Options{Format: "stubs", Paper: p.ID}.Grid()
		if err != nil {
			t.Fatal(err)
		}
		pages, err := g.Cells(g.PerPage())
		if err != nil {
			t.Fatal(err)
		}
		pageBox := Box{0, 0, p.Page.Width, p.Page.Height}
		for i, stub := range pages[0] {
			r := stubRatios[i%len(stubRatios)]
			s := layoutStub(stub, r)
			name := fmt.Sprintf("%s stub %d (ratio %v)", p.ID, i, r)
			if s.Stub != stub {
				t.Errorf("%s: outline %+v, want the cell %+v", name, s.Stub, stub)
			}
			if !inside(s.Stub, pageBox) {
				t.Errorf("%s: %+v off the page", name, s.Stub)
			}
			// The perforation splits the stub into the main part and the
			// stub end, stubEndWidth from the right edge.
			if !near(s.PerfX, stub.X+stub.W-stubEndWidth) {
				t.Errorf("%s: perforation at %v, want %v", name, s.PerfX, stub.X+stub.W-stubEndWidth)
			}
			if s.Main != (Box{stub.X, stub.Y, s.PerfX - stub.X, stub.H}) ||
				!near(s.End.X, s.PerfX) || !near(s.End.X+s.End.W, stub.X+stub.W) || s.End.Y != stub.Y || s.End.H != stub.H {
				t.Errorf("%s: main %+v and end %+v do not split %+v at %v", name, s.Main, s.End, stub, s.PerfX)
			}
			// Everything on the main part stays inside its padding, so
			// clear of the cut line, the notches and the perforation.
			mainInner := padded(s.Main, cardPadding)
			parts := map[string]Box{"cover": s.Cover, "text": s.Text, "title": s.Title, "creator": s.Creator, "date": s.Date, "where": s.Where}
			for j, st := range s.Stars {
				parts[fmt.Sprintf("star %d", j+1)] = st
			}
			for part, b := range parts {
				if !inside(b, mainInner) {
					t.Errorf("%s: %s %+v outside the main part's padding %+v", name, part, b, mainInner)
				}
			}
			if endInner := padded(s.End, cardPadding); !inside(s.Admit, endInner) {
				t.Errorf("%s: admit text %+v outside the stub end's padding %+v", name, s.Admit, endInner)
			}
		}
	}
}

func TestLayoutStubNotchesAndPerforation(t *testing.T) {
	stub := Box{X: 13, Y: 18.5, W: StubWidth, H: StubHeight}
	s := layoutStub(stub, 0.656)
	if !near(s.NotchRadius, notchRadius) || s.NotchRadius <= 0 {
		t.Fatalf("notch radius %v, want %v", s.NotchRadius, notchRadius)
	}
	// Semicircles centred on the perforation, on the top and bottom edges.
	want := [2]point{{s.PerfX, stub.Y}, {s.PerfX, stub.Y + stub.H}}
	for i, n := range s.Notches {
		if !near(n.X, want[i].X) || !near(n.Y, want[i].Y) {
			t.Errorf("notch %d at %+v, want %+v", i, n, want[i])
		}
	}
	// The notches fit in the edges, clear of the corners.
	if s.PerfX-s.NotchRadius <= stub.X || s.PerfX+s.NotchRadius >= stub.X+stub.W {
		t.Errorf("notches at %v ± %v reach a corner of %+v", s.PerfX, s.NotchRadius, stub)
	}
	// The padding keeps every part clear of the notches.
	if s.NotchRadius >= cardPadding {
		t.Errorf("notch radius %v reaches into the %v mm padding", s.NotchRadius, cardPadding)
	}
	// The dashed line runs from notch to notch, on the perforation.
	if s.Perforation.W != 0 || !near(s.Perforation.X, s.PerfX) ||
		!near(s.Perforation.Y, stub.Y+s.NotchRadius) || !near(s.Perforation.Y+s.Perforation.H, stub.Y+stub.H-s.NotchRadius) {
		t.Errorf("perforation %+v, want x %v from notch to notch", s.Perforation, s.PerfX)
	}
	// The admit text is centred in the stub end and runs along it.
	if !near(s.Admit.X+s.Admit.W/2, s.End.X+s.End.W/2) || !near(s.Admit.Y+s.Admit.H/2, s.End.Y+s.End.H/2) {
		t.Errorf("admit text %+v not centred in the stub end %+v", s.Admit, s.End)
	}
	if s.Admit.H <= s.Admit.W {
		t.Errorf("admit text %+v: want it upright, taller than wide", s.Admit)
	}
}

func TestLayoutStubCoverKeepsRatio(t *testing.T) {
	stub := Box{X: 13, Y: 18.5, W: StubWidth, H: StubHeight}
	innerH := StubHeight - 2*cardPadding
	maxW := innerH * minCellRatio
	tests := []struct {
		name  string
		ratio float64
		fills string // the side the cover fills: "height" or "width"
	}{
		{"very tall", 0.3, "height"},
		{"portrait", 0.656, "height"},
		{"2:3", 2.0 / 3, "height"},
		{"square", 1, "width"},
		{"landscape", 16.0 / 9, "width"},
		{"panoramic", 3, "width"},
	}
	for _, tt := range tests {
		s := layoutStub(stub, tt.ratio)
		c := s.Cover
		if !near(c.W/c.H, tt.ratio) {
			t.Errorf("%s: cover %+v has ratio %v, want %v", tt.name, c, c.W/c.H, tt.ratio)
		}
		if c.W > maxW+eps || c.H > innerH+eps {
			t.Errorf("%s: cover %+v larger than %v x %v", tt.name, c, maxW, innerH)
		}
		if (tt.fills == "height" && !near(c.H, innerH)) || (tt.fills == "width" && !near(c.W, maxW)) {
			t.Errorf("%s: cover %+v does not fill the %s of its area", tt.name, c, tt.fills)
		}
		// Left-aligned and vertically centred.
		if !near(c.X, stub.X+cardPadding) || !near(c.Y+c.H/2, stub.Y+stub.H/2) {
			t.Errorf("%s: cover at (%v, %v), want left-aligned and centred", tt.name, c.X, c.Y)
		}
		// The text column starts right of the widest cover, after a gap,
		// and ends at the main part's padding.
		if s.Text.X < stub.X+cardPadding+maxW+cardTextGap-eps {
			t.Errorf("%s: text column at %v overlaps the cover area", tt.name, s.Text.X)
		}
		if !near(s.Text.X+s.Text.W, s.PerfX-cardPadding) {
			t.Errorf("%s: text column ends at %v, want %v", tt.name, s.Text.X+s.Text.W, s.PerfX-cardPadding)
		}
	}
}

func TestLayoutStubTextStack(t *testing.T) {
	stub := Box{X: 29, Y: 19, W: StubWidth, H: StubHeight}
	s := layoutStub(stub, 0.656)
	if len(s.Stars) != starCount {
		t.Fatalf("%d stars, want %d", len(s.Stars), starCount)
	}
	// Top-down: title, creator, date, where, stars, none overlapping.
	stack := []struct {
		name string
		box  Box
	}{{"title", s.Title}, {"creator", s.Creator}, {"date", s.Date}, {"where", s.Where}, {"stars", s.Stars[0]}}
	for i := 1; i < len(stack); i++ {
		prev, cur := stack[i-1], stack[i]
		if cur.box.Y < prev.box.Y+prev.box.H-eps {
			t.Errorf("%s %+v starts above the end of %s %+v", cur.name, cur.box, prev.name, prev.box)
		}
	}
	// The title starts at the top padding, the stars end on the bottom one.
	if !near(s.Title.Y, stub.Y+cardPadding) {
		t.Errorf("title at %v, want %v", s.Title.Y, stub.Y+cardPadding)
	}
	if bottom := s.Stars[0].Y + s.Stars[0].H; !near(bottom, stub.Y+stub.H-cardPadding) {
		t.Errorf("stars end at %v, want %v", bottom, stub.Y+stub.H-cardPadding)
	}
	// The fields leave room to write: at least blockGap between parts.
	for i := 2; i < len(stack); i++ {
		prev, cur := stack[i-1], stack[i]
		if gap := cur.box.Y - (prev.box.Y + prev.box.H); gap < blockGap-eps {
			t.Errorf("%v mm between %s and %s, want at least %v", gap, prev.name, cur.name, blockGap)
		}
	}
	// Every text part spans the text column.
	for name, b := range map[string]Box{"title": s.Title, "creator": s.Creator, "date": s.Date, "where": s.Where} {
		if !inside(b, s.Text) || !near(b.W, s.Text.W) {
			t.Errorf("%s %+v does not span the text column %+v", name, b, s.Text)
		}
	}
	// Title room for two lines, creator for one.
	if s.Title.H < 2*s.Creator.H {
		t.Errorf("title %v mm tall, creator %v mm: want room for two title lines", s.Title.H, s.Creator.H)
	}
	// Stars in a row, not overlapping, all the same size, in the column.
	for i, st := range s.Stars {
		if !inside(st, s.Text) || !near(st.Y, s.Stars[0].Y) || !near(st.W, s.Stars[0].W) {
			t.Errorf("star %d %+v out of the row", i, st)
		}
		if i > 0 && overlap(s.Stars[i-1], st) {
			t.Errorf("stars %d and %d overlap", i-1, i)
		}
	}
}

// The text drawn on a stub never runs past the room the layout gives it,
// measured with the embedded fonts.
func TestStubTextFits(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	s := layoutStub(Box{X: 13, Y: 18.5, W: StubWidth, H: StubHeight}, 0.656)
	measure := func(bold bool, pt float64) func(string) float64 {
		return func(text string) float64 {
			pdf.SetFont(textFont, fontStyle(bold), pt)
			return pdf.GetStringWidth(text)
		}
	}
	tests := []struct {
		name     string
		text     string
		bold     bool
		pt       float64
		width    float64
		maxLines int
	}{
		{"long accented title", "Cien años de soledad edición conmemorativa de la Real Academia Española", true, titleFontSize, s.Title.W, titleLines},
		{"one long word title", "Supercalifragilisticexpialidociousandevenlongerthanthat", true, titleFontSize, s.Title.W, titleLines},
		{"long accented creator", "Ünïcödé Çréâtör Wïth À Vêry Löng Nämé Indeed", false, creatorFontSize, s.Creator.W, 1},
	}
	for _, tt := range tests {
		m := measure(tt.bold, tt.pt)
		lines := fitLines(tt.text, tt.width, tt.maxLines, m)
		if len(lines) == 0 || len(lines) > tt.maxLines {
			t.Errorf("%s: %d lines, want 1 to %d", tt.name, len(lines), tt.maxLines)
		}
		for _, l := range lines {
			if w := m(l); w > tt.width+eps {
				t.Errorf("%s: line %q is %v mm wide, room for %v", tt.name, l, w, tt.width)
			}
		}
	}

	// "ADMIT ONE" prints whole along the stub end, within its line box.
	m := measure(true, admitFontSize)
	if lines := fitLines(admitText, s.Admit.H, 1, m); len(lines) != 1 || lines[0] != admitText {
		t.Errorf("admit text fits as %q in %v mm, want %q whole", lines, s.Admit.H, admitText)
	}
	if !near(s.Admit.W, lineHeight(admitFontSize)) {
		t.Errorf("admit text box %v mm across, want one %v pt line", s.Admit.W, admitFontSize)
	}

	// Both field labels leave a rule to write on.
	labels := measure(false, labelFontSize)
	if room := s.Text.W - math.Max(labels(dateText), labels(whereText)) - labelRuleGap; room < minDateRule {
		t.Errorf("%v mm left for the rules, want at least %v", room, minDateRule)
	}
}
