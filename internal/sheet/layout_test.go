package sheet

import (
	"math"
	"testing"
)

const eps = 1e-9

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func ratios(n int, r float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = r
	}
	return out
}

// standard is today's grid: 50 mm covers on A4.
func standard(t *testing.T) Grid {
	t.Helper()
	g, err := Options{}.Grid()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// presetGrids returns the grid of every size preset, by ID.
func presetGrids(t *testing.T) map[string]Grid {
	t.Helper()
	grids := map[string]Grid{}
	for _, s := range Sizes {
		g, err := Options{Size: s.ID}.Grid()
		if err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
		grids[s.ID] = g
	}
	return grids
}

func TestGridPresets(t *testing.T) {
	tests := []struct {
		size       string
		height     float64
		cols, rows int
		cellWidth  float64
		marginY    float64
		perPage    int
	}{
		// (194 - 8·4) / 9 = 18; (297 - 9·25 - 8·4) / 2 = 20.
		{"mini", 25, 9, 9, 18, 20, 81},
		// Today's layout: (194 - 4·4) / 5 = 35.6; (297 - 5·50 - 4·4) / 2 = 15.5.
		{"standard", 50, 5, 5, 35.6, 15.5, 25},
		// (194 - 2·4) / 3 = 62; (297 - 3·80 - 2·4) / 2 = 24.5.
		{"large", 80, 3, 3, 62, 24.5, 9},
	}
	grids := presetGrids(t)
	if len(grids) != len(tests) {
		t.Fatalf("%d presets, want %d", len(grids), len(tests))
	}
	for _, tt := range tests {
		g, ok := grids[tt.size]
		if !ok {
			t.Errorf("no %q preset", tt.size)
			continue
		}
		if g.Page != A4 || !near(g.CoverHeight, tt.height) || g.Columns != tt.cols || g.Rows != tt.rows ||
			!near(g.CellWidth, tt.cellWidth) || !near(g.MarginX, 8) || !near(g.MarginY, tt.marginY) ||
			g.PerPage() != tt.perPage {
			t.Errorf("%s: grid %+v (%d per page), want %v mm, %d x %d, cell %v, margins 8 / %v, %d per page",
				tt.size, g, g.PerPage(), tt.height, tt.cols, tt.rows, tt.cellWidth, tt.marginY, tt.perPage)
		}
	}
}

func TestGridCellsAreAtLeastTwoByThree(t *testing.T) {
	for id, g := range presetGrids(t) {
		if g.CellWidth < g.CoverHeight*2/3-eps {
			t.Errorf("%s: cell %v x %v is narrower than 2:3", id, g.CellWidth, g.CoverHeight)
		}
		// One more column would make the cells narrower than 2:3.
		more := (g.Page.Width - 2*MinMargin - float64(g.Columns)*Gap) / float64(g.Columns+1)
		if more >= g.CoverHeight*2/3 {
			t.Errorf("%s: %d columns fit, grid has %d", id, g.Columns+1, g.Columns)
		}
	}
}

func TestGridMiniFitsMoreLargeFewer(t *testing.T) {
	grids := presetGrids(t)
	mini, std, large := grids["mini"].PerPage(), grids["standard"].PerPage(), grids["large"].PerPage()
	if !(mini > std && std > large) {
		t.Errorf("per page: mini %d, standard %d, large %d; want mini > standard > large", mini, std, large)
	}
}

func TestSizeByID(t *testing.T) {
	if DefaultSize != "standard" {
		t.Errorf("DefaultSize = %q, want standard", DefaultSize)
	}
	if s, err := SizeByID(""); err != nil || s.ID != "standard" {
		t.Errorf(`SizeByID("") = %+v, %v; want standard`, s, err)
	}
	for _, id := range []string{"mini", "standard", "large"} {
		if s, err := SizeByID(id); err != nil || s.ID != id {
			t.Errorf("SizeByID(%q) = %+v, %v", id, s, err)
		}
	}
	if _, err := SizeByID("huge"); err == nil {
		t.Error(`SizeByID("huge"): want an error`)
	}
	if _, err := (Options{Size: "huge"}).Grid(); err == nil {
		t.Error(`Options{Size: "huge"}.Grid(): want an error`)
	}
}

func TestLayoutPageCount(t *testing.T) {
	tests := []struct {
		size    string
		covers  int
		pages   int
		lastLen int
	}{
		{"standard", 1, 1, 1},
		{"standard", 7, 1, 7},
		{"standard", 25, 1, 25},
		{"standard", 26, 2, 1},
		{"standard", 60, 3, 10},
		{"mini", 81, 1, 81},
		{"mini", 82, 2, 1},
		{"large", 9, 1, 9},
		{"large", 20, 3, 2},
	}
	grids := presetGrids(t)
	for _, tt := range tests {
		g := grids[tt.size]
		pages, err := g.Layout(ratios(tt.covers, 0.656))
		if err != nil {
			t.Fatalf("%s, %d covers: %v", tt.size, tt.covers, err)
		}
		if len(pages) != tt.pages {
			t.Errorf("%s, %d covers: %d pages, want %d", tt.size, tt.covers, len(pages), tt.pages)
			continue
		}
		for i, p := range pages[:len(pages)-1] {
			if len(p) != g.PerPage() {
				t.Errorf("%s, %d covers: page %d has %d boxes, want %d", tt.size, tt.covers, i+1, len(p), g.PerPage())
			}
		}
		if got := len(pages[len(pages)-1]); got != tt.lastLen {
			t.Errorf("%s, %d covers: last page has %d boxes, want %d", tt.size, tt.covers, got, tt.lastLen)
		}
	}
}

func TestLayoutRejectsNoCovers(t *testing.T) {
	if _, err := standard(t).Layout(nil); err == nil {
		t.Fatal("Layout(nil): want an error")
	}
}

func TestLayoutRejectsBadRatio(t *testing.T) {
	for _, r := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := standard(t).Layout([]float64{0.7, r}); err == nil {
			t.Errorf("ratio %v: want an error", r)
		}
	}
}

func TestLayoutBoxesInsidePageWithoutOverlap(t *testing.T) {
	// Mix of tall, typical, square and very wide covers.
	mix := []float64{0.3, 0.656, 0.66, 1, 1.5, 3}
	var rs []float64
	for i := 0; i < 200; i++ {
		rs = append(rs, mix[i%len(mix)])
	}
	for id, g := range presetGrids(t) {
		pages, err := g.Layout(rs)
		if err != nil {
			t.Fatal(err)
		}
		for pi, page := range pages {
			for i, b := range page {
				if b.X < -eps || b.Y < -eps || b.X+b.W > g.Page.Width+eps || b.Y+b.H > g.Page.Height+eps {
					t.Errorf("%s: page %d box %d %+v is outside the page", id, pi+1, i, b)
				}
				if b.W > g.CellWidth+eps || b.H > g.CoverHeight+eps {
					t.Errorf("%s: page %d box %d %+v is larger than its cell", id, pi+1, i, b)
				}
				for j := i + 1; j < len(page); j++ {
					c := page[j]
					if b.X < c.X+c.W && c.X < b.X+b.W && b.Y < c.Y+c.H && c.Y < b.Y+b.H {
						t.Errorf("%s: page %d boxes %d %+v and %d %+v overlap", id, pi+1, i, b, j, c)
					}
				}
			}
		}
	}
}

func TestLayoutKeepsAspectRatio(t *testing.T) {
	for id, g := range presetGrids(t) {
		// A typical cover is full height; a square one is shrunk to the
		// column width. Both keep their aspect ratio.
		pages, err := g.Layout([]float64{0.656, 1})
		if err != nil {
			t.Fatal(err)
		}
		typical, square := pages[0][0], pages[0][1]
		if !near(typical.H, g.CoverHeight) || !near(typical.W, 0.656*g.CoverHeight) {
			t.Errorf("%s: typical cover %+v, want %v x %v", id, typical, 0.656*g.CoverHeight, g.CoverHeight)
		}
		if !near(square.W, g.CellWidth) || !near(square.H, g.CellWidth) {
			t.Errorf("%s: square cover %+v, want %v x %v", id, square, g.CellWidth, g.CellWidth)
		}
	}
}

func TestLayoutTypicalCoverKeepsHeight(t *testing.T) {
	pages, err := standard(t).Layout([]float64{0.656})
	if err != nil {
		t.Fatal(err)
	}
	b := pages[0][0]
	if !near(b.H, 50) || !near(b.W, 32.8) {
		t.Errorf("box = %+v, want 32.8 x 50", b)
	}
}

func TestLayoutSquareCoverShrinksToFit(t *testing.T) {
	pages, err := standard(t).Layout([]float64{1})
	if err != nil {
		t.Fatal(err)
	}
	b := pages[0][0]
	if !near(b.W, 35.6) || !near(b.H, 35.6) {
		t.Errorf("box = %+v, want 35.6 x 35.6", b)
	}
	// Centred in its 35.6 x 50 cell at (8, 15.5).
	if !near(b.X, 8) || !near(b.Y, 15.5+(50-35.6)/2) {
		t.Errorf("box at (%v, %v), want (8, %v)", b.X, b.Y, 15.5+(50-35.6)/2)
	}
}

func TestLayoutGridPositions(t *testing.T) {
	g := standard(t)
	pages, err := g.Layout(ratios(25, 0.656))
	if err != nil {
		t.Fatal(err)
	}
	// Cell 0 is at (8, 15.5); a 32.8 mm wide cover is centred, 1.4 mm in.
	first := pages[0][0]
	if !near(first.X, 9.4) || !near(first.Y, 15.5) {
		t.Errorf("cell 0 at (%v, %v), want (9.4, 15.5)", first.X, first.Y)
	}
	// Cell 1 is one column right: 35.6 + 4 mm.
	if second := pages[0][1]; !near(second.X, 9.4+39.6) || !near(second.Y, 15.5) {
		t.Errorf("cell 1 at (%v, %v), want (49, 15.5)", second.X, second.Y)
	}
	// Cell 5 starts the second row: 50 + 4 mm down.
	if sixth := pages[0][5]; !near(sixth.X, 9.4) || !near(sixth.Y, 69.5) {
		t.Errorf("cell 5 at (%v, %v), want (9.4, 69.5)", sixth.X, sixth.Y)
	}
	// Cell 24 is the bottom-right cell at (166.4, 231.5).
	last := pages[0][24]
	if !near(last.X, 166.4+1.4) || !near(last.Y, 231.5) {
		t.Errorf("cell 24 at (%v, %v), want (167.8, 231.5)", last.X, last.Y)
	}
}

func TestLayoutGridIsCentred(t *testing.T) {
	for id, g := range presetGrids(t) {
		pages, err := g.Layout(ratios(g.PerPage(), 0.656))
		if err != nil {
			t.Fatal(err)
		}
		// Cells span from the first box's cell to the last box's cell.
		last := pages[0][len(pages[0])-1]
		lastCellX := last.X - (g.CellWidth-last.W)/2
		right := g.Page.Width - (lastCellX + g.CellWidth)
		bottom := g.Page.Height - (last.Y + last.H)
		if !near(right, g.MarginX) || !near(bottom, g.MarginY) {
			t.Errorf("%s: right margin %v (want %v), bottom margin %v (want %v)",
				id, right, g.MarginX, bottom, g.MarginY)
		}
	}
}
