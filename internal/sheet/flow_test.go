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

// Covers that fit the 2:3 column flow exactly as the 2:3 grid lays them
// out, partial last row and page included.
func TestFlowColumnShapesKeepTheGrid(t *testing.T) {
	shapes := []float64{0.656, 2.0 / 3, 0.5, 0.3, 0.6, 0.66}
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
		"landscape":     {1.5, 4.0 / 3, 3, 0.656, 1, 0.8, 5, 1.12},
		"panoramas":     {5, 3, 5, 0.656},
		"4:5 portraits": {0.8, 0.8, 0.656, 0.8},
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
		// 3:2 cells are 75 mm wide on A4: 2 a row, 10 a page.
		{"10 landscapes", Options{}, ratios(10, 1.5), 1},
		{"11 landscapes", Options{}, ratios(11, 1.5), 2},
		{"36 mini landscapes", Options{Size: "mini"}, ratios(36, 1.5), 1},
		{"37 mini landscapes", Options{Size: "mini"}, ratios(37, 1.5), 2},
		{"large landscapes on A5", Options{Size: "large", Paper: "a5"}, ratios(3, 1.5), 2},
		// A 5:1 panorama takes a row of its own: 5 a page.
		{"5 panoramas", Options{}, ratios(5, 5), 1},
		{"6 panoramas", Options{}, ratios(6, 5), 2},
		// 4:5 cells are 40 mm wide: 4 a row, not 5.
		{"20 4:5 portraits", Options{}, ratios(20, 0.8), 1},
		{"21 4:5 portraits", Options{}, ratios(21, 0.8), 2},
		{"cards ignore landscapes", Options{Format: "cards"}, ratios(8, 1.5), 1},
		{"full page ignores landscapes", Options{Format: "fullpage"}, ratios(60, 3), 1},
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
		if f.ShapedCells {
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

func TestOnlyTheCoverSheetHasShapedCells(t *testing.T) {
	for _, f := range Formats {
		if want := f.ID == DefaultFormat; f.ShapedCells != want {
			t.Errorf("%s: ShapedCells = %v, want %v", f.ID, f.ShapedCells, want)
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

// Shapes used by the wide cell tests, as width / height.
const (
	landscape32 = 1.5     // a 3:2 photo
	landscape43 = 4.0 / 3 // a 4:3 painting
	panorama    = 3.0     // a 3:1 panorama
	portrait45  = 0.8     // a 4:5 portrait, wider than the 2:3 column
)

// On every paper and size, a cover that fits the 2:3 column keeps it, a
// square cover keeps its square cell, and any other cover gets a cell as
// wide as the cover at full height, at least the column and at most the
// usable width.
func TestFlowCellWidths(t *testing.T) {
	for _, p := range Papers {
		for _, s := range Sizes {
			g, err := Options{Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			h, usable := g.CoverHeight, p.Page.Width-2*MinMargin
			tests := []struct {
				name string
				r    float64
				want float64
			}{
				{"2:3", 2.0 / 3, g.CellWidth},
				{"typical cover", 0.656, g.CellWidth},
				{"narrow", 0.5, g.CellWidth},
				{"square", 1, h},
				{"near square", 1.05, h},
				{"4:5", portrait45, max(g.CellWidth, portrait45*h)},
				{"3:2", landscape32, landscape32 * h},
				{"4:3", landscape43, landscape43 * h},
				{"3:1", panorama, min(panorama*h, usable)},
				{"10:1", 10, usable},
			}
			for _, tt := range tests {
				t.Run(fmt.Sprintf("%s %s %s", p.ID, s.ID, tt.name), func(t *testing.T) {
					if got := g.flowCellWidth(tt.r); !near(got, tt.want) {
						t.Errorf("cell width %v, want %v", got, tt.want)
					}
					// A single cover gets its cell: in the grid's first
					// column, or centred as if its row were full of cells
					// like it.
					pages, err := g.Flow([]float64{tt.r})
					if err != nil {
						t.Fatal(err)
					}
					cell := Box{X: g.MarginX, Y: g.MarginY, W: tt.want, H: h}
					if !near(tt.want, g.CellWidth) {
						n := max(1, math.Floor((usable+Gap+eps)/(tt.want+Gap)))
						cell.X += (usable - (n*tt.want + (n-1)*Gap)) / 2
					}
					if b, want := pages[0][0], fit(cell, tt.r); !near(b.X, want.X) || !near(b.Y, want.Y) || !near(b.W, want.W) || !near(b.H, want.H) {
						t.Errorf("box %+v, want %+v", b, want)
					}
				})
			}
		}
	}
}

func TestGridWide(t *testing.T) {
	g := standard(t) // 35.6 mm columns, 50 mm covers: the column fits r ≤ 0.712
	tests := []struct {
		name string
		r    float64
		want bool
	}{
		{"typical cover", 0.656, false},
		{"fits the column", 0.7, false},
		{"just fits the column", 35.6 / 50, false},
		{"4:5", portrait45, true},
		{"square", 1, false},
		{"just wider than square", 1.12, true},
		{"3:2", landscape32, true},
		{"panorama", panorama, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Wide(tt.r); got != tt.want {
				t.Errorf("Wide(%v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

// Boxes on A4 with standard 50 mm covers: 194 mm usable from x = 8, 2:3
// cells 35.6 mm wide, 4 mm apart; rows from y = 15.5, 54 mm apart.
func TestFlowLandscapePositions(t *testing.T) {
	l32 := Box{W: 75, H: 50}
	l43 := Box{W: 200.0 / 3, H: 50}
	p45 := Box{W: 40, H: 50}
	sq := Box{W: 50, H: 50}
	tc := Box{W: 32.8, H: 50} // a 0.656 cover in a 35.6 mm cell
	at := func(b Box, x, y float64) Box { b.X, b.Y = x, y; return b }
	tests := []struct {
		name   string
		ratios []float64
		want   [][]Box
	}{
		{
			// 2 × 75 + 4 = 154 mm, 20 mm in; a third does not fit, and
			// the last row lines up as if it held 2.
			name:   "3:2 covers, two a row",
			ratios: []float64{landscape32, landscape32, landscape32},
			want: [][]Box{{
				at(l32, 28, 15.5), at(l32, 107, 15.5),
				at(l32, 28, 69.5),
			}},
		},
		{
			// 2 × 66.67 + 4 = 137.33 mm, 28.33 mm in.
			name:   "4:3 covers, two a row",
			ratios: []float64{landscape43, landscape43},
			want:   [][]Box{{at(l43, 8+(194-(400.0/3+4))/2, 15.5), at(l43, 8+(194-(400.0/3+4))/2+200.0/3+4, 15.5)}},
		},
		{
			// 4 × 40 + 12 = 172 mm, 11 mm in; the fifth wraps.
			name:   "4:5 covers, four a row",
			ratios: []float64{portrait45, portrait45, portrait45, portrait45, portrait45},
			want: [][]Box{{
				at(p45, 19, 15.5), at(p45, 63, 15.5), at(p45, 107, 15.5), at(p45, 151, 15.5),
				at(p45, 19, 69.5),
			}},
		},
		{
			// 150 + 4 + 35.6 = 189.6 mm, 2.2 mm in.
			name:   "a panorama and a 2:3 cover share a row",
			ratios: []float64{panorama, 0.656},
			want: [][]Box{{
				{X: 10.2, Y: 15.5, W: 150, H: 50},
				at(tc, 164.2+1.4, 15.5),
			}},
		},
		{
			// A 5:1 panorama would be 250 mm: its cell is the usable
			// width, and it shrinks to 194 × 38.8 mm, centred in its row.
			name:   "a panorama wider than the page",
			ratios: []float64{5, 5},
			want: [][]Box{{
				{X: 8, Y: 15.5 + 5.6, W: 194, H: 38.8},
				{X: 8, Y: 69.5 + 5.6, W: 194, H: 38.8},
			}},
		},
		{
			// 35.6 + 50 + 75 + 8 = 168.6 mm, 12.7 mm in.
			name:   "a 2:3, a square and a 3:2 cover",
			ratios: []float64{0.656, 1, landscape32},
			want: [][]Box{{
				at(tc, 20.7+1.4, 15.5), at(sq, 60.3, 15.5), at(l32, 114.3, 15.5),
			}},
		},
		{
			// A row of 2:3 covers keeps the grid; a 3:2 cover after 5 of
			// them wraps.
			name:   "a 3:2 cover after a full 2:3 row",
			ratios: []float64{0.656, 0.656, 0.656, 0.656, 0.656, landscape32},
			want: [][]Box{{
				at(tc, 9.4, 15.5), at(tc, 49, 15.5), at(tc, 88.6, 15.5), at(tc, 128.2, 15.5), at(tc, 167.8, 15.5),
				at(l32, 28, 69.5),
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

// PerPageFor counts the covers of one shape on a full page: on a cover
// sheet, as Flow lays them out; in other formats, PerPage.
func TestPerPageFor(t *testing.T) {
	tests := []struct {
		paper, size string
		r           float64
		want        int
	}{
		// 3:2 cells: (usable + 4) / (1.5 H + 4) a row, the grid's rows.
		{"a4", "mini", landscape32, 36},
		{"a4", "standard", landscape32, 10},
		{"a4", "large", landscape32, 3},
		{"letter", "mini", landscape32, 36},
		{"letter", "standard", landscape32, 8},
		{"letter", "large", landscape32, 3},
		{"a5", "mini", landscape32, 18},
		{"a5", "standard", landscape32, 3},
		{"a5", "large", landscape32, 2},
		// One panorama a row.
		{"a4", "standard", 5, 5},
		{"a4", "standard", portrait45, 20},
		// Squares and 2:3 covers keep their counts.
		{"a4", "standard", 1, 15},
		{"a4", "standard", 0.656, 25},
		{"a5", "large", 1, 2},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s %v", tt.paper, tt.size, tt.r), func(t *testing.T) {
			got, err := Options{Paper: tt.paper, Size: tt.size}.PerPageFor(tt.r)
			if err != nil || got != tt.want {
				t.Errorf("PerPageFor(%v) = %d, %v; want %d", tt.r, got, err, tt.want)
			}
		})
	}
	for _, f := range Formats {
		if f.ShapedCells {
			continue
		}
		opts := Options{Format: f.ID}
		per, err := opts.PerPage()
		if err != nil {
			t.Fatal(err)
		}
		if got, err := opts.PerPageFor(landscape32); err != nil || got != per {
			t.Errorf("%s: PerPageFor(1.5) = %d, %v; want %d", f.ID, got, err, per)
		}
	}
	if _, err := (Options{Format: "poster"}).PerPageFor(1); err == nil {
		t.Error("unknown format: want an error")
	}
}

// A cover sheet of 3:2 images gives them wide cells: 10 fit on an A4 page
// of standard covers, each 75 × 50 mm.
func TestBuildLandscapeCoversGetWideCells(t *testing.T) {
	dir := t.TempDir()
	wide := writeImage(t, dir, "Wheat Field - Van Gogh.jpg", 150, 100, color.RGBA{200, 160, 40, 255})
	for _, tt := range []struct{ covers, pages int }{{10, 1}, {11, 2}} {
		paths := make([]string, tt.covers)
		for i := range paths {
			paths[i] = wide
		}
		var out bytes.Buffer
		if err := Build(&out, paths, Options{}); err != nil {
			t.Fatal(err)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != tt.pages {
			t.Errorf("%d landscapes: PDF has %d pages, want %d", tt.covers, n, tt.pages)
		}
	}
	// 75 × 50 mm is 212.6 × 141.73 pt.
	got := pageStreams(render(t, []string{wide}, Options{}))
	if !bytes.Contains([]byte(got), []byte("q 212.59843 0 0 141.73228 ")) {
		t.Errorf("landscape cover not drawn 75 × 50 mm:\n%s", got)
	}
}
