package sheet

import (
	"math"
	"strings"
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

func TestNewGridRejectsHeightsThatDoNotFit(t *testing.T) {
	for _, h := range []float64{0, -1, math.NaN(), math.Inf(1), 282, 500} {
		if g, err := NewGrid(A4, h); err == nil {
			t.Errorf("NewGrid(A4, %v) = %+v, want an error", h, g)
		}
	}
	// The tallest cover that fits: the page height minus both margins.
	if g, err := NewGrid(A4, 281); err != nil || g.PerPage() != 1 || g.MarginY < MinMargin-eps {
		t.Errorf("NewGrid(A4, 281) = %+v, %v; want one cell within the margins", g, err)
	}
}

func TestGridPapers(t *testing.T) {
	tests := []struct {
		paper, size string
		page        Page
		cols, rows  int
		cellWidth   float64
		marginY     float64
	}{
		// A4 keeps today's grids.
		{"a4", "mini", A4, 9, 9, 18, 20},
		{"a4", "standard", A4, 5, 5, 35.6, 15.5},
		{"a4", "large", A4, 3, 3, 62, 24.5},
		// Letter: usable 199.9 × 263.4.
		// (199.9 - 8·4) / 9; (279.4 - 9·25 - 8·4) / 2 = 11.2.
		{"letter", "mini", Letter, 9, 9, 167.9 / 9, 11.2},
		// (199.9 - 4·4) / 5 = 36.78; (279.4 - 4·50 - 3·4) / 2 = 33.7.
		{"letter", "standard", Letter, 5, 4, 36.78, 33.7},
		// (199.9 - 2·4) / 3; (279.4 - 3·80 - 2·4) / 2 = 15.7.
		{"letter", "large", Letter, 3, 3, 191.9 / 3, 15.7},
		// A5: usable 132 × 194.
		// (132 - 5·4) / 6; (210 - 6·25 - 5·4) / 2 = 20.
		{"a5", "mini", A5, 6, 6, 112.0 / 6, 20},
		// (132 - 2·4) / 3; (210 - 3·50 - 2·4) / 2 = 26.
		{"a5", "standard", A5, 3, 3, 124.0 / 3, 26},
		// (132 - 4) / 2 = 64; (210 - 2·80 - 4) / 2 = 23.
		{"a5", "large", A5, 2, 2, 64, 23},
	}
	for _, tt := range tests {
		g, err := Options{Paper: tt.paper, Size: tt.size}.Grid()
		if err != nil {
			t.Errorf("%s %s: %v", tt.paper, tt.size, err)
			continue
		}
		if g.Page != tt.page || g.Columns != tt.cols || g.Rows != tt.rows ||
			!near(g.CellWidth, tt.cellWidth) || !near(g.MarginX, MinMargin) || !near(g.MarginY, tt.marginY) {
			t.Errorf("%s %s: grid %+v, want %v, %d x %d, cell %v, margins 8 / %v",
				tt.paper, tt.size, g, tt.page, tt.cols, tt.rows, tt.cellWidth, tt.marginY)
		}
		// Full and centred: no room for another row or column, and covers
		// stay inside the margins.
		usableW, usableH := g.Page.Width-2*MinMargin, g.Page.Height-2*MinMargin
		if more := (usableW - float64(g.Columns)*Gap) / float64(g.Columns+1); more >= g.CoverHeight*2/3 {
			t.Errorf("%s %s: %d columns fit, grid has %d", tt.paper, tt.size, g.Columns+1, g.Columns)
		}
		if float64(g.Rows+1)*g.CoverHeight+float64(g.Rows)*Gap <= usableH {
			t.Errorf("%s %s: %d rows fit, grid has %d", tt.paper, tt.size, g.Rows+1, g.Rows)
		}
		if g.MarginY < MinMargin-eps {
			t.Errorf("%s %s: top margin %v below %v", tt.paper, tt.size, g.MarginY, MinMargin)
		}
	}
}

func TestGridDefaultPaperIsA4(t *testing.T) {
	def, err := Options{}.Grid()
	if err != nil {
		t.Fatal(err)
	}
	a4, err := Options{Paper: "a4"}.Grid()
	if err != nil {
		t.Fatal(err)
	}
	if def != a4 || def.Page != A4 {
		t.Errorf("default grid %+v, want the A4 grid %+v", def, a4)
	}
}

func TestPaperByID(t *testing.T) {
	tests := []struct {
		id   string
		want Page
	}{
		{"", A4},
		{"a4", A4},
		{"letter", Page{Width: 215.9, Height: 279.4}},
		{"a5", Page{Width: 148, Height: 210}},
		{"b5", Page{Width: 176, Height: 250}},
	}
	for _, tt := range tests {
		p, err := PaperByID(tt.id)
		if err != nil || p.Page != tt.want {
			t.Errorf("PaperByID(%q) = %+v, %v; want %v", tt.id, p, err, tt.want)
		}
	}
	if _, err := PaperByID("a3"); err == nil {
		t.Error(`PaperByID("a3"): want an error`)
	}
	if _, err := (Options{Paper: "a3"}).Grid(); err == nil || !strings.Contains(err.Error(), "a3") {
		t.Errorf(`Options{Paper: "a3"}.Grid(): error %v, want one naming the paper`, err)
	}
}

// Every paper and size gives a grid whose boxes stay on the page without
// overlapping, so no cover is clipped.
func TestLayoutEveryPaperBoxesInsidePage(t *testing.T) {
	for _, p := range Papers {
		for _, s := range Sizes {
			g, err := Options{Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatalf("%s %s: %v", p.ID, s.ID, err)
			}
			ratios := make([]float64, g.PerPage())
			for i := range ratios {
				ratios[i] = []float64{2.0 / 3, 1, 0.5, 1.5}[i%4]
			}
			pages, err := g.Layout(ratios)
			if err != nil {
				t.Fatal(err)
			}
			for i, b := range pages[0] {
				if b.X < MinMargin-eps || b.Y < MinMargin-eps ||
					b.X+b.W > g.Page.Width-MinMargin+eps || b.Y+b.H > g.Page.Height-MinMargin+eps {
					t.Errorf("%s %s: box %d %+v outside the margins", p.ID, s.ID, i, b)
				}
			}
		}
	}
}

func TestPresetsHaveFileSafeTags(t *testing.T) {
	const safe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	for _, p := range Papers {
		if p.FileTag == "" || strings.Trim(p.FileTag, safe) != "" {
			t.Errorf("paper %q: file tag %q, want letters and digits only", p.ID, p.FileTag)
		}
	}
	for _, s := range Sizes {
		if s.FileTag == "" || strings.Trim(s.FileTag, safe) != "" {
			t.Errorf("size %q: file tag %q, want letters and digits only", s.ID, s.FileTag)
		}
	}
}
