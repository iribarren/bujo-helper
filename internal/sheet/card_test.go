package sheet

import (
	"strings"
	"testing"
)

func TestCardGridPapers(t *testing.T) {
	tests := []struct {
		paper            string
		page             Page
		cols, rows       int
		marginX, marginY float64
	}{
		// (210 - 2·90 - 4) / 2 = 13; (297 - 4·55 - 3·4) / 2 = 32.5.
		{"a4", A4, 2, 4, 13, 32.5},
		// (215.9 - 184) / 2 = 15.95; (279.4 - 232) / 2 = 23.7.
		{"letter", Letter, 2, 4, 15.95, 23.7},
		// (148 - 90) / 2 = 29; (210 - 3·55 - 2·4) / 2 = 18.5.
		{"a5", A5, 1, 3, 29, 18.5},
	}
	for _, tt := range tests {
		g, err := Options{Format: "cards", Paper: tt.paper}.Grid()
		if err != nil {
			t.Fatalf("%s: %v", tt.paper, err)
		}
		if g.Page != tt.page || g.Columns != tt.cols || g.Rows != tt.rows ||
			!near(g.CellWidth, CardWidth) || !near(g.CoverHeight, CardHeight) ||
			!near(g.MarginX, tt.marginX) || !near(g.MarginY, tt.marginY) {
			t.Errorf("%s: grid %+v, want %d x %d cards, margins %v / %v", tt.paper, g, tt.cols, tt.rows, tt.marginX, tt.marginY)
		}
		if g.MarginX < MinMargin || g.MarginY < MinMargin {
			t.Errorf("%s: margins %v / %v below %v", tt.paper, g.MarginX, g.MarginY, MinMargin)
		}
		// Full: no room for another column or row.
		if float64(g.Columns+1)*CardWidth+float64(g.Columns)*Gap <= g.Page.Width-2*MinMargin {
			t.Errorf("%s: %d columns fit, grid has %d", tt.paper, g.Columns+1, g.Columns)
		}
		if float64(g.Rows+1)*CardHeight+float64(g.Rows)*Gap <= g.Page.Height-2*MinMargin {
			t.Errorf("%s: %d rows fit, grid has %d", tt.paper, g.Rows+1, g.Rows)
		}
		// Centred both ways: the last card leaves the same margins.
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

func TestNewCardGridRejectsTinyPages(t *testing.T) {
	if g, err := NewCardGrid(Page{Width: 100, Height: 297}); err == nil {
		t.Errorf("NewCardGrid on a 100 mm wide page = %+v, want an error", g)
	}
	if g, err := NewCardGrid(Page{Width: 210, Height: 60}); err == nil {
		t.Errorf("NewCardGrid on a 60 mm tall page = %+v, want an error", g)
	}
}

// An item exactly as wide (or tall) as n items fill the usable page gives n
// columns (rows) even when the float quotient lands just below n, as it
// does for 7 columns across A5 and 11 across Letter; a micrometre wider
// gives n-1 (or no grid, for one item). Three A5 Standard polaroid frames
// are such an exact fit (TestPolaroidGridPapersAndSizes).
func TestNewFixedGridToleratesAnExactFit(t *testing.T) {
	const wider = 1e-6 // mm, far above the tolerance, far below any real size
	shortfall := false
	for _, paper := range []string{"a4", "letter", "a5"} {
		p, err := PaperByID(paper)
		if err != nil {
			t.Fatal(err)
		}
		usableW, usableH := p.Page.Width-2*MinMargin, p.Page.Height-2*MinMargin
		for n := 1; n <= 12; n++ {
			w := (usableW - float64(n-1)*Gap) / float64(n)
			h := (usableH - float64(n-1)*Gap) / float64(n)
			if (usableW+Gap)/(w+Gap) < float64(n) || (usableH+Gap)/(h+Gap) < float64(n) {
				shortfall = true
			}

			g, err := newFixedGrid(p.Page, w, h, "item")
			if err != nil {
				t.Fatalf("%s, %d × %d exact fit: %v", paper, n, n, err)
			}
			if g.Columns != n || g.Rows != n {
				t.Errorf("%s, %d × %d exact fit: grid is %d × %d", paper, n, n, g.Columns, g.Rows)
			}

			g, err = newFixedGrid(p.Page, w+wider, h+wider, "item")
			if n == 1 {
				if err == nil {
					t.Errorf("%s: an item wider and taller than the usable page = %+v, want an error", paper, g)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s, %d × %d fit made wider: %v", paper, n, n, err)
			}
			if g.Columns != n-1 || g.Rows != n-1 {
				t.Errorf("%s, %d × %d fit made %v mm wider and taller: grid is %d × %d, want %d × %d",
					paper, n, n, wider, g.Columns, g.Rows, n-1, n-1)
			}
		}
	}
	if !shortfall {
		t.Error("no exact fit's quotient fell short of n, so the tolerance went untested")
	}
}

func TestCellsPaginate(t *testing.T) {
	g, err := NewCardGrid(A4)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ cards, pages, lastLen int }{
		{1, 1, 1}, {8, 1, 8}, {9, 2, 1}, {17, 3, 1},
	}
	for _, tt := range tests {
		pages, err := g.Cells(tt.cards)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != tt.pages || len(pages[len(pages)-1]) != tt.lastLen {
			t.Errorf("%d cards: %d pages, last has %d; want %d, %d", tt.cards, len(pages), len(pages[len(pages)-1]), tt.pages, tt.lastLen)
		}
		for pi, page := range pages {
			for i, b := range page {
				if !near(b.W, CardWidth) || !near(b.H, CardHeight) || !inside(b, Box{0, 0, A4.Width, A4.Height}) {
					t.Errorf("%d cards: page %d card %d %+v", tt.cards, pi+1, i, b)
				}
				for j := i + 1; j < len(page); j++ {
					if overlap(b, page[j]) {
						t.Errorf("%d cards: page %d cards %d and %d overlap", tt.cards, pi+1, i, j)
					}
				}
			}
		}
	}
	if _, err := g.Cells(0); err == nil {
		t.Error("Cells(0): want an error")
	}
}

// inside reports whether b lies within outer.
func inside(b, outer Box) bool {
	return b.W >= 0 && b.H >= 0 && b.X >= outer.X-eps && b.Y >= outer.Y-eps &&
		b.X+b.W <= outer.X+outer.W+eps && b.Y+b.H <= outer.Y+outer.H+eps
}

// overlap reports whether a and b share any area.
func overlap(a, b Box) bool {
	return a.X < b.X+b.W-eps && b.X < a.X+a.W-eps && a.Y < b.Y+b.H-eps && b.Y < a.Y+a.H-eps
}

// cardRatios are covers of every shape: very tall, typical, 2:3, square,
// wide and panoramic.
var cardRatios = []float64{0.3, 0.656, 2.0 / 3, 1, 16.0 / 9, 3}

func TestLayoutCardPartsInsideTheCard(t *testing.T) {
	for _, p := range Papers {
		g, err := Options{Format: "cards", Paper: p.ID}.Grid()
		if err != nil {
			t.Fatal(err)
		}
		pages, err := g.Cells(g.PerPage())
		if err != nil {
			t.Fatal(err)
		}
		pageBox := Box{0, 0, p.Page.Width, p.Page.Height}
		for i, card := range pages[0] {
			r := cardRatios[i%len(cardRatios)]
			c := layoutCard(card, r)
			if c.Card != card {
				t.Errorf("%s card %d: border %+v, want the cell %+v", p.ID, i, c.Card, card)
			}
			inner := Box{card.X + cardPadding, card.Y + cardPadding, card.W - 2*cardPadding, card.H - 2*cardPadding}
			parts := map[string]Box{"cover": c.Cover, "text": c.Text, "title": c.Title, "creator": c.Creator, "date": c.Date}
			for j, s := range c.Stars {
				parts["star "+string(rune('1'+j))] = s
			}
			for j, l := range c.Lines {
				parts["line "+string(rune('1'+j))] = l
			}
			for name, b := range parts {
				if !inside(b, inner) || !inside(b, pageBox) {
					t.Errorf("%s card %d (ratio %v): %s %+v outside the card's padding %+v", p.ID, i, r, name, b, inner)
				}
			}
		}
	}
}

func TestLayoutCardCoverKeepsRatio(t *testing.T) {
	card := Box{X: 13, Y: 32.5, W: CardWidth, H: CardHeight}
	innerH := CardHeight - 2*cardPadding
	maxW := innerH * minCellRatio
	for _, r := range cardRatios {
		c := layoutCard(card, r)
		if !near(c.Cover.W/c.Cover.H, r) {
			t.Errorf("ratio %v: cover %+v has ratio %v", r, c.Cover, c.Cover.W/c.Cover.H)
		}
		if c.Cover.W > maxW+eps || c.Cover.H > innerH+eps {
			t.Errorf("ratio %v: cover %+v larger than %v x %v", r, c.Cover, maxW, innerH)
		}
		// As large as the area allows: full height, or full width.
		if !near(c.Cover.H, innerH) && !near(c.Cover.W, maxW) {
			t.Errorf("ratio %v: cover %+v does not fill its area", r, c.Cover)
		}
		// Left-aligned and vertically centred.
		if !near(c.Cover.X, card.X+cardPadding) || !near(c.Cover.Y+c.Cover.H/2, card.Y+card.H/2) {
			t.Errorf("ratio %v: cover at (%v, %v), want left-aligned and centred", r, c.Cover.X, c.Cover.Y)
		}
		// The text column starts right of the widest cover, after a gap.
		if c.Text.X < card.X+cardPadding+maxW+eps {
			t.Errorf("ratio %v: text column at %v overlaps the cover area", r, c.Text.X)
		}
	}
	// Covers taller than 2:3 keep the full height; wider ones the full width.
	if c := layoutCard(card, 0.5); !near(c.Cover.H, innerH) {
		t.Errorf("tall cover %+v, want full height %v", c.Cover, innerH)
	}
	if c := layoutCard(card, 1); !near(c.Cover.W, maxW) || !near(c.Cover.H, maxW) {
		t.Errorf("square cover %+v, want %v x %v", c.Cover, maxW, maxW)
	}
}

func TestLayoutCardTextStack(t *testing.T) {
	card := Box{X: 29, Y: 18.5, W: CardWidth, H: CardHeight}
	c := layoutCard(card, 0.656)
	if len(c.Stars) != 5 {
		t.Fatalf("%d stars, want 5", len(c.Stars))
	}
	if len(c.Lines) != 3 {
		t.Fatalf("%d writing lines, want 3", len(c.Lines))
	}
	// Top-down: title, creator, date, stars, writing lines, none overlapping.
	stack := []struct {
		name string
		box  Box
	}{{"title", c.Title}, {"creator", c.Creator}, {"date", c.Date}, {"stars", c.Stars[0]}}
	for _, l := range c.Lines {
		stack = append(stack, struct {
			name string
			box  Box
		}{"line", l})
	}
	for i := 1; i < len(stack); i++ {
		prev, cur := stack[i-1], stack[i]
		if cur.box.Y < prev.box.Y+prev.box.H-eps {
			t.Errorf("%s %+v starts above the end of %s %+v", cur.name, cur.box, prev.name, prev.box)
		}
	}
	// Every part sits in the text column.
	for name, b := range map[string]Box{"title": c.Title, "creator": c.Creator, "date": c.Date, "first line": c.Lines[0]} {
		if !inside(b, c.Text) {
			t.Errorf("%s %+v outside the text column %+v", name, b, c.Text)
		}
		if strings.HasPrefix(name, "first") && !near(b.W, c.Text.W) {
			t.Errorf("writing line %+v does not span the text column %+v", b, c.Text)
		}
	}
	// Stars in a row, not overlapping, all the same size.
	for i, s := range c.Stars {
		if !inside(s, c.Text) || !near(s.Y, c.Stars[0].Y) || !near(s.W, c.Stars[0].W) {
			t.Errorf("star %d %+v out of the row", i, s)
		}
		if i > 0 && overlap(c.Stars[i-1], s) {
			t.Errorf("stars %d and %d overlap", i-1, i)
		}
	}
	// The last writing line sits on the bottom padding.
	if last := c.Lines[len(c.Lines)-1]; !near(last.Y, card.Y+card.H-cardPadding) {
		t.Errorf("last line at %v, want %v", last.Y, card.Y+card.H-cardPadding)
	}
	// Title room for two lines, creator for one.
	if c.Title.H < 2*c.Creator.H {
		t.Errorf("title %v mm tall, creator %v mm: want room for two title lines", c.Title.H, c.Creator.H)
	}
}

func TestStarPointsInsideBox(t *testing.T) {
	b := Box{X: 50, Y: 30, W: 4.5, H: 4.5}
	pts := starPoints(b)
	if len(pts) != 10 {
		t.Fatalf("%d points, want 10", len(pts))
	}
	for _, p := range pts {
		if p.X < b.X-eps || p.X > b.X+b.W+eps || p.Y < b.Y-eps || p.Y > b.Y+b.H+eps {
			t.Errorf("point %+v outside %+v", p, b)
		}
	}
	// Point up: the first point is the top centre.
	top := pts[0].Y
	for _, p := range pts {
		top = min(top, p.Y)
	}
	if !near(pts[0].X, b.X+b.W/2) || !near(pts[0].Y, top) {
		t.Errorf("first point %+v, want the top centre", pts[0])
	}
}
