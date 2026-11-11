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

func TestLayoutPageCount(t *testing.T) {
	tests := []struct {
		covers  int
		pages   int
		lastLen int
	}{
		{1, 1, 1},
		{7, 1, 7},
		{25, 1, 25},
		{26, 2, 1},
		{60, 3, 10},
	}
	for _, tt := range tests {
		pages, err := Layout(ratios(tt.covers, 0.656))
		if err != nil {
			t.Fatalf("%d covers: %v", tt.covers, err)
		}
		if len(pages) != tt.pages {
			t.Errorf("%d covers: %d pages, want %d", tt.covers, len(pages), tt.pages)
			continue
		}
		for i, p := range pages[:len(pages)-1] {
			if len(p) != PerPage {
				t.Errorf("%d covers: page %d has %d boxes, want %d", tt.covers, i+1, len(p), PerPage)
			}
		}
		if got := len(pages[len(pages)-1]); got != tt.lastLen {
			t.Errorf("%d covers: last page has %d boxes, want %d", tt.covers, got, tt.lastLen)
		}
	}
}

func TestLayoutRejectsNoCovers(t *testing.T) {
	if _, err := Layout(nil); err == nil {
		t.Fatal("Layout(nil): want an error")
	}
}

func TestLayoutRejectsBadRatio(t *testing.T) {
	for _, r := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := Layout([]float64{0.7, r}); err == nil {
			t.Errorf("ratio %v: want an error", r)
		}
	}
}

func TestLayoutBoxesInsidePageWithoutOverlap(t *testing.T) {
	// Mix of tall, typical, square and very wide covers.
	mix := []float64{0.3, 0.656, 0.66, 1, 1.5, 3}
	var rs []float64
	for i := 0; i < 50; i++ {
		rs = append(rs, mix[i%len(mix)])
	}
	pages, err := Layout(rs)
	if err != nil {
		t.Fatal(err)
	}
	for pi, page := range pages {
		for i, b := range page {
			if b.X < -eps || b.Y < -eps || b.X+b.W > PageWidth+eps || b.Y+b.H > PageHeight+eps {
				t.Errorf("page %d box %d %+v is outside the page", pi+1, i, b)
			}
			if b.W > CellWidth+eps || b.H > CoverHeight+eps {
				t.Errorf("page %d box %d %+v is larger than its cell", pi+1, i, b)
			}
			for j := i + 1; j < len(page); j++ {
				c := page[j]
				if b.X < c.X+c.W && c.X < b.X+b.W && b.Y < c.Y+c.H && c.Y < b.Y+b.H {
					t.Errorf("page %d boxes %d %+v and %d %+v overlap", pi+1, i, b, j, c)
				}
			}
		}
	}
}

func TestLayoutTypicalCoverKeepsHeight(t *testing.T) {
	pages, err := Layout([]float64{0.656})
	if err != nil {
		t.Fatal(err)
	}
	b := pages[0][0]
	if !near(b.H, 50) || !near(b.W, 32.8) {
		t.Errorf("box = %+v, want 32.8 x 50", b)
	}
}

func TestLayoutSquareCoverShrinksToFit(t *testing.T) {
	pages, err := Layout([]float64{1})
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
	pages, err := Layout(ratios(25, 0.656))
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
	// The grid is centred: equal margins left/right and top/bottom.
	if right := PageWidth - (166.4 + CellWidth); !near(right, MarginX) {
		t.Errorf("right margin %v, want %v", right, MarginX)
	}
	if bottom := PageHeight - (last.Y + last.H); !near(bottom, MarginY) {
		t.Errorf("bottom margin %v, want %v", bottom, MarginY)
	}
}

func TestLayoutConstants(t *testing.T) {
	if !near(CellWidth, 35.6) || !near(MarginY, 15.5) || PerPage != 25 {
		t.Errorf("CellWidth=%v MarginY=%v PerPage=%v", CellWidth, MarginY, PerPage)
	}
}
