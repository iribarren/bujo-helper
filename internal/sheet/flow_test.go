package sheet

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"testing"
)

func TestIsSquare(t *testing.T) {
	tests := []struct {
		name string
		r    float64
		want bool
	}{
		{"exactly square", 1, true},
		{"lower bound", 0.9, true},
		{"upper bound", 1 / 0.9, true},
		{"just below the lower bound", 0.8999, false},
		{"just above the upper bound", 1.1112, false},
		{"slightly tall", 0.95, true},
		{"slightly wide", 1.05, true},
		{"typical cover", 2.0 / 3, false},
		{"wide", 1.5, false},
		{"zero", 0, false},
		{"NaN", math.NaN(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSquare(tt.r); got != tt.want {
				t.Errorf("IsSquare(%v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

// A page of square covers fills every row with square cells: as many as
// fit the usable width, Gap apart, and the same rows as 2:3 covers.
func TestFlowAllSquareCapacity(t *testing.T) {
	tests := []struct {
		paper, size string
		cols, rows  int
	}{
		// A4, 194 mm usable: (194 + 4) / (H + 4).
		{"a4", "mini", 6, 9},
		{"a4", "standard", 3, 5},
		{"a4", "large", 2, 3},
		// Letter, 199.9 mm usable.
		{"letter", "mini", 7, 9},
		{"letter", "standard", 3, 4},
		{"letter", "large", 2, 3},
		// A5, 132 mm usable.
		{"a5", "mini", 4, 6},
		{"a5", "standard", 2, 3},
		{"a5", "large", 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.paper+" "+tt.size, func(t *testing.T) {
			opts := Options{Paper: tt.paper, Size: tt.size}
			g, err := opts.Grid()
			if err != nil {
				t.Fatal(err)
			}
			if g.Rows != tt.rows {
				t.Fatalf("grid has %d rows, want %d", g.Rows, tt.rows)
			}
			per := tt.cols * tt.rows
			pages, err := g.Flow(ratios(per+1, 1))
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 2 || len(pages[0]) != per || len(pages[1]) != 1 {
				t.Fatalf("%d covers on %d pages (%d on the first), want %d on the first and 1 on the second",
					per+1, len(pages), len(pages[0]), per)
			}
			for i, b := range pages[0] {
				if !near(b.W, g.CoverHeight) || !near(b.H, g.CoverHeight) {
					t.Errorf("box %d %+v, want %v mm square", i, b, g.CoverHeight)
				}
			}
			if got, err := opts.SquarePerPage(); err != nil || got != per {
				t.Errorf("SquarePerPage() = %d, %v; want %d", got, err, per)
			}
		})
	}
}

// Covers that are not square flow exactly as the 2:3 grid lays them out,
// partial last row and page included.
func TestFlowNonSquareKeepsTheGrid(t *testing.T) {
	shapes := []float64{0.656, 2.0 / 3, 0.5, 1.5, 0.89, 1.12, 0.3, 2.5}
	for _, p := range Papers {
		for _, s := range Sizes {
			g, err := Options{Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			per := g.PerPage()
			for _, n := range []int{1, g.Columns - 1, g.Columns + 1, per - 1, per, per + 3, 2*per + 1} {
				if n < 1 {
					continue
				}
				rs := make([]float64, n)
				for i := range rs {
					rs[i] = shapes[i%len(shapes)]
				}
				want, err := g.Layout(rs)
				if err != nil {
					t.Fatal(err)
				}
				got, err := g.Flow(rs)
				if err != nil {
					t.Fatal(err)
				}
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("%s %s, %d covers: flow\n%v\nwant the grid\n%v", p.ID, s.ID, n, got, want)
				}
				for pi := range want {
					for i := range want[pi] {
						if got[pi][i] != want[pi][i] {
							t.Errorf("%s %s, %d covers: page %d box %d %+v, want exactly %+v",
								p.ID, s.ID, n, pi+1, i, got[pi][i], want[pi][i])
						}
					}
				}
			}
		}
	}
}

// Boxes on A4 with standard 50 mm covers: 194 mm usable from x = 8,
// 2:3 cells 35.6 mm wide, square cells 50 mm, 4 mm apart; rows from
// y = 15.5, 54 mm apart.
func TestFlowMixedPositions(t *testing.T) {
	sq := Box{W: 50, H: 50}
	tc := Box{W: 32.8, H: 50} // a 0.656 cover in a 35.6 mm cell
	at := func(b Box, x, y float64) Box { b.X, b.Y = x, y; return b }
	tests := []struct {
		name   string
		ratios []float64
		want   [][]Box
	}{
		{
			// 3 squares fill 158 mm, centred 18 mm in; the last row is
			// centred as if it held 3 too, so it lines up.
			name:   "squares wrap and the last row lines up",
			ratios: []float64{1, 1, 1, 1},
			want: [][]Box{{
				at(sq, 26, 15.5), at(sq, 80, 15.5), at(sq, 134, 15.5),
				at(sq, 26, 69.5),
			}},
		},
		{
			// 35.6 + 50 + 35.6 + 8 = 129.2 mm, padded with one more 2:3
			// cell to 168.8 mm: 12.6 mm in.
			name:   "a square between 2:3 covers",
			ratios: []float64{0.656, 1, 0.656},
			want: [][]Box{{
				at(tc, 22, 15.5), at(sq, 60.2, 15.5), at(tc, 115.6, 15.5),
			}},
		},
		{
			// 4 2:3 cells leave no room for a square: the row wraps, and
			// is centred as if padded to 5 cells, the 2:3 grid.
			name:   "a square after 2:3 covers wraps",
			ratios: []float64{0.656, 0.656, 0.656, 0.656, 1},
			want: [][]Box{{
				at(tc, 9.4, 15.5), at(tc, 49, 15.5), at(tc, 88.6, 15.5), at(tc, 128.2, 15.5),
				at(sq, 26, 69.5),
			}},
		},
		{
			// 50 + 3 · 35.6 + 12 = 168.8 mm: nothing more fits; the next
			// row is 2:3 only and keeps the grid.
			name:   "2:3 covers after a square",
			ratios: []float64{1, 0.656, 0.656, 0.656, 0.656, 0.656},
			want: [][]Box{{
				at(sq, 20.6, 15.5), at(tc, 76, 15.5), at(tc, 115.6, 15.5), at(tc, 155.2, 15.5),
				at(tc, 9.4, 69.5), at(tc, 49, 69.5),
			}},
		},
		{
			// A near-square cover keeps its shape, shrunk to fit its cell.
			name:   "near-square covers fit their square cells",
			ratios: []float64{0.95, 1.05},
			want: [][]Box{{
				{X: 26 + 1.25, Y: 15.5, W: 47.5, H: 50},
				{X: 80, Y: 15.5 + (50-50/1.05)/2, W: 50, H: 50 / 1.05},
			}},
		},
	}
	g := standard(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.Flow(tt.ratios)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("%d pages, want %d", len(got), len(tt.want))
			}
			for pi := range tt.want {
				if len(got[pi]) != len(tt.want[pi]) {
					t.Fatalf("page %d: %d boxes, want %d", pi+1, len(got[pi]), len(tt.want[pi]))
				}
				for i, w := range tt.want[pi] {
					b := got[pi][i]
					if !near(b.X, w.X) || !near(b.Y, w.Y) || !near(b.W, w.W) || !near(b.H, w.H) {
						t.Errorf("page %d box %d %+v, want %+v", pi+1, i, b, w)
					}
				}
			}
		})
	}
}

func TestFlowPageBreaks(t *testing.T) {
	g := standard(t)
	pages, err := g.Flow(ratios(16, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || len(pages[0]) != 15 || len(pages[1]) != 1 {
		t.Fatalf("16 squares: pages of %v boxes, want 15 and 1", pageLens(pages))
	}
	if b := pages[1][0]; !near(b.X, 26) || !near(b.Y, 15.5) {
		t.Errorf("first square of page 2 at (%v, %v), want (26, 15.5)", b.X, b.Y)
	}
	// A 2:3 row holds 5 covers, a square row 3: 6 rows alternating take
	// 5 rows on the first page, 1 on the second.
	var rs []float64
	for range 3 {
		rs = append(rs, ratios(5, 0.656)...)
		rs = append(rs, ratios(3, 1)...)
	}
	pages, err = g.Flow(rs)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(pageLens(pages)) != "[21 3]" {
		t.Errorf("alternating rows: pages of %v boxes, want [21 3]", pageLens(pages))
	}
}

func pageLens(pages [][]Box) []int {
	out := make([]int, len(pages))
	for i, p := range pages {
		out[i] = len(p)
	}
	return out
}

// Mixed sequences on every paper and size: boxes stay inside the margins,
// keep their aspect ratio, never overlap, and fill rows in order.
func TestFlowMixedBoxesInsideMarginsWithoutOverlap(t *testing.T) {
	patterns := map[string][]float64{
		"alternating":   {1, 0.656},
		"square runs":   {1, 1, 0.656, 1.5, 1, 0.5, 0.9, 1 / 0.9},
		"mostly 2:3":    {0.656, 0.656, 0.656, 1},
		"mostly square": {1, 1, 1, 0.656},
	}
	for _, p := range Papers {
		for _, s := range Sizes {
			g, err := Options{Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			for name, pat := range patterns {
				rs := make([]float64, 2*g.PerPage()+3)
				for i := range rs {
					rs[i] = pat[i%len(pat)]
				}
				pages, err := g.Flow(rs)
				if err != nil {
					t.Fatal(err)
				}
				label := fmt.Sprintf("%s %s %s", p.ID, s.ID, name)
				i := 0
				for pi, page := range pages {
					if len(page) == 0 {
						t.Errorf("%s: page %d is empty", label, pi+1)
					}
					rows := map[float64]bool{}
					for j, b := range page {
						r := rs[i]
						i++
						if b.X < MinMargin-eps || b.Y < MinMargin-eps ||
							b.X+b.W > g.Page.Width-MinMargin+eps || b.Y+b.H > g.Page.Height-MinMargin+eps {
							t.Errorf("%s: page %d box %d %+v outside the margins", label, pi+1, j, b)
						}
						if !near(b.W/b.H, r) || b.H > g.CoverHeight+eps {
							t.Errorf("%s: page %d box %d %+v, want aspect %v within %v mm", label, pi+1, j, b, r, g.CoverHeight)
						}
						// The row is the cell top: rows are CoverHeight + Gap apart.
						row := math.Round((b.Y - g.MarginY) / (g.CoverHeight + Gap))
						rows[row] = true
						if j > 0 {
							prev := page[j-1]
							prevRow := math.Round((prev.Y - g.MarginY) / (g.CoverHeight + Gap))
							if row < prevRow || (row == prevRow && b.X <= prev.X+prev.W) {
								t.Errorf("%s: page %d box %d %+v does not follow box %+v", label, pi+1, j, b, prev)
							}
						}
						for k := range j {
							c := page[k]
							if b.X < c.X+c.W-eps && c.X < b.X+b.W-eps && b.Y < c.Y+c.H-eps && c.Y < b.Y+b.H-eps {
								t.Errorf("%s: page %d boxes %d %+v and %d %+v overlap", label, pi+1, k, c, j, b)
							}
						}
					}
					if len(rows) > g.Rows {
						t.Errorf("%s: page %d has %d rows, want at most %d", label, pi+1, len(rows), g.Rows)
					}
				}
				if i != len(rs) {
					t.Errorf("%s: %d boxes for %d covers", label, i, len(rs))
				}
			}
		}
	}
}

// A square cell is never wider than the usable width.
func TestFlowSquareCellFitsANarrowPage(t *testing.T) {
	g, err := NewGrid(Page{Width: 60, Height: 297}, 50)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := g.Flow([]float64{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages[0]) != 2 {
		t.Fatalf("%d boxes on the first page, want 2", len(pages[0]))
	}
	for i, b := range pages[0] {
		if !near(b.X, 8) || !near(b.W, 44) || !near(b.H, 44) {
			t.Errorf("box %d %+v, want 44 mm square at x = 8", i, b)
		}
	}
}

func TestFlowRejectsBadRatios(t *testing.T) {
	if _, err := standard(t).Flow(nil); err == nil {
		t.Error("Flow(nil): want an error")
	}
	for _, r := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := standard(t).Flow([]float64{1, r}); err == nil {
			t.Errorf("Flow with ratio %v: want an error", r)
		}
	}
}

func TestOptionsPages(t *testing.T) {
	tests := []struct {
		name   string
		opts   Options
		ratios []float64
		want   int
	}{
		{"no covers", Options{}, nil, 0},
		{"25 2:3 covers", Options{}, ratios(25, 0.656), 1},
		{"26 2:3 covers", Options{}, ratios(26, 0.656), 2},
		{"15 squares", Options{}, ratios(15, 1), 1},
		{"16 squares", Options{}, ratios(16, 1), 2},
		{"6 large squares", Options{Size: "large"}, ratios(6, 1), 1},
		{"7 large squares", Options{Size: "large"}, ratios(7, 1), 2},
		{"squares on A5", Options{Paper: "a5"}, ratios(7, 1), 2},
		{"cards ignore squares", Options{Format: "cards"}, ratios(9, 1), 2},
		{"polaroids ignore squares", Options{Format: "polaroid"}, ratios(1, 1), 1},
		{"full page ignores squares", Options{Format: "fullpage"}, ratios(61, 1), 2},
		{"checklist with a header", Options{Format: "checklist", Header: "TBR"}, ratios(17, 1), 2},
		{"checklist without a header", Options{Format: "checklist"}, ratios(17, 1), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.opts.Pages(tt.ratios)
			if err != nil || got != tt.want {
				t.Errorf("Pages = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
	if _, err := (Options{Format: "poster"}).Pages(ratios(1, 1)); err == nil {
		t.Error("unknown format: want an error")
	}
}

// Formats without square cells have the same capacity for square covers.
func TestSquarePerPageOtherFormats(t *testing.T) {
	for _, f := range Formats {
		if f.SquareCells {
			continue
		}
		opts := Options{Format: f.ID}
		per, err := opts.PerPage()
		if err != nil {
			t.Fatal(err)
		}
		if got, err := opts.SquarePerPage(); err != nil || got != per {
			t.Errorf("%s: SquarePerPage() = %d, %v; want %d", f.ID, got, err, per)
		}
	}
}

func TestOnlyTheCoverSheetHasSquareCells(t *testing.T) {
	for _, f := range Formats {
		if want := f.ID == DefaultFormat; f.SquareCells != want {
			t.Errorf("%s: SquareCells = %v, want %v", f.ID, f.SquareCells, want)
		}
	}
}

func TestAspectRatios(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "a.jpg", 66, 100, color.RGBA{200, 40, 40, 255}),
		writeImage(t, dir, "b.png", 120, 120, color.RGBA{40, 200, 40, 255}),
	}
	got, err := AspectRatios(paths)
	if err != nil || len(got) != 2 || !near(got[0], 0.66) || !near(got[1], 1) {
		t.Errorf("AspectRatios = %v, %v; want [0.66 1]", got, err)
	}
	if _, err := AspectRatios(append(paths, dir+"/missing.jpg")); err == nil {
		t.Error("a missing file: want an error")
	}
}

// A cover sheet of squares gives them square cells: 15 fit on an A4 page
// of standard covers, not 25, each 50 mm square.
func TestBuildSquareCoversGetSquareCells(t *testing.T) {
	dir := t.TempDir()
	square := writeImage(t, dir, "Abbey Road - The Beatles.jpg", 120, 120, color.RGBA{40, 40, 200, 255})
	for _, tt := range []struct{ covers, pages int }{{15, 1}, {16, 2}} {
		paths := make([]string, tt.covers)
		for i := range paths {
			paths[i] = square
		}
		var out bytes.Buffer
		if err := Build(&out, paths, Options{}); err != nil {
			t.Fatal(err)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != tt.pages {
			t.Errorf("%d squares: PDF has %d pages, want %d", tt.covers, n, tt.pages)
		}
	}
	// 50 mm is 141.73 pt.
	got := pageStreams(render(t, []string{square}, Options{}))
	if !bytes.Contains([]byte(got), []byte("q 141.73228 0 0 141.73228 ")) {
		t.Errorf("square cover not drawn 50 mm square:\n%s", got)
	}
}
