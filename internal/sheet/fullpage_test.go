package sheet

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strings"
	"testing"
)

// fullPageCounts are the cover counts the full-page grid is checked with:
// one, a few, a full page and more than a page.
var fullPageCounts = []int{1, 2, 3, 4, 5, 7, 12, 25, 59, 60, 61, 100}

// fullPageHeight is the largest cover height n covers can have in c
// columns on page, the header band left out or not: the auto-size rule,
// written out independently of NewFullPageGrid.
func fullPageHeight(page Page, n, c int, header bool) float64 {
	usableW := page.Width - 2*MinMargin
	usableH := page.Height - 2*MinMargin
	if header {
		usableH -= FullPageHeaderHeight
	}
	r := (n + c - 1) / c
	return min(
		(usableW-float64(c-1)*FullPageGap)/float64(c)/minCellRatio,
		(usableH-float64(r-1)*FullPageGap)/float64(r),
		FullPageMaxCover,
	)
}

func TestNewFullPageGridKnownSizes(t *testing.T) {
	tests := []struct {
		paper      string
		count      int
		header     bool
		cols, rows int
		height     float64
	}{
		// Few covers reach the 80 mm cap; ties keep fewer empty cells,
		// then more columns.
		{"a4", 1, false, 1, 1, 80},
		{"a4", 2, true, 2, 1, 80},
		{"a4", 3, false, 3, 1, 80},
		{"a4", 4, true, 2, 2, 80},
		{"a4", 7, true, 3, 3, 80},
		{"a4", 12, false, 4, 3, 69.375},
		{"a4", 60, false, 8, 8, 32.4375},
		{"a4", 60, true, 8, 8, 31},
		// Past the per-page cap, the grid is the one of a full page.
		{"a4", 61, true, 8, 8, 31},
		{"a4", 500, false, 8, 8, 32.4375},
		{"letter", 12, false, 4, 3, 71.5875},
		{"letter", 60, false, 8, 8, 30.3},
		{"letter", 60, true, 9, 7, 29.316666666666666},
		{"a5", 3, false, 2, 2, 80},
		{"a5", 7, false, 3, 3, 62.666666666666664},
		{"a5", 7, true, 3, 3, 58.666666666666664},
		{"a5", 12, true, 4, 3, 46.125},
		{"a5", 60, true, 8, 8, 20.125},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%d/header=%v", tt.paper, tt.count, tt.header), func(t *testing.T) {
			p, err := PaperByID(tt.paper)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewFullPageGrid(p.Page, tt.count, tt.header)
			if err != nil {
				t.Fatal(err)
			}
			if g.Columns != tt.cols || g.Rows != tt.rows || !near(g.CoverHeight, tt.height) {
				t.Errorf("grid %d × %d of %v mm covers, want %d × %d of %v mm", g.Columns, g.Rows, g.CoverHeight, tt.cols, tt.rows, tt.height)
			}
			if want := min(tt.count, FullPageMax); g.PerPage() != want {
				t.Errorf("PerPage() = %d, want %d", g.PerPage(), want)
			}
		})
	}
}

// For every paper, header or not, and count, the grid holds the covers of a
// page in 2:3 cells as large as they can be, inside the usable area and
// below the header band, centred, a tight gap apart.
func TestNewFullPageGridFillsThePage(t *testing.T) {
	for _, p := range Papers {
		for _, header := range []bool{false, true} {
			for _, count := range fullPageCounts {
				name := fmt.Sprintf("%s/%d/header=%v", p.ID, count, header)
				g, err := NewFullPageGrid(p.Page, count, header)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				checkFullPageGrid(t, name, p.Page, g, min(count, FullPageMax), header)
			}
		}
	}
}

func checkFullPageGrid(t *testing.T, name string, page Page, g Grid, n int, header bool) {
	t.Helper()
	if g.Page != page || g.PerPage() != n {
		t.Errorf("%s: page %+v, %d per page; want %+v, %d", name, g.Page, g.PerPage(), page, n)
	}
	// No row is left empty.
	if g.Columns*g.Rows < n || g.Columns*g.Rows-n >= g.Columns {
		t.Errorf("%s: %d × %d grid for %d covers", name, g.Columns, g.Rows, n)
	}
	if g.CoverHeight > FullPageMaxCover+eps || !near(g.CellWidth, g.CoverHeight*minCellRatio) {
		t.Errorf("%s: cells %v × %v mm, want 2:3 and at most %v mm tall", name, g.CellWidth, g.CoverHeight, FullPageMaxCover)
	}
	// As large as any column count allows.
	for c := 1; c <= n; c++ {
		if h := fullPageHeight(page, n, c, header); h > g.CoverHeight+eps {
			t.Errorf("%s: %d columns give %v mm covers, larger than the grid's %v mm", name, c, h, g.CoverHeight)
		}
	}

	usable := Box{X: MinMargin, Y: MinMargin, W: page.Width - 2*MinMargin, H: page.Height - 2*MinMargin}
	band, hasBand := fullPageHeaderBox(g, header)
	if hasBand != header {
		t.Fatalf("%s: header band %v, want %v", name, hasBand, header)
	}
	top := g.MarginY
	if header {
		if !inside(band, usable) || !near(band.H, FullPageHeaderHeight) || !near(band.W, usable.W) {
			t.Errorf("%s: header band %+v, want %v mm tall across the usable area %+v", name, band, FullPageHeaderHeight, usable)
		}
		if !near(band.Y+band.H, g.MarginY) {
			t.Errorf("%s: header band ends at %v mm, the grid starts at %v mm", name, band.Y+band.H, g.MarginY)
		}
		top = band.Y
	}
	pages, err := g.Cells(n)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || len(pages[0]) != n {
		t.Fatalf("%s: cells %v, want one page of %d", name, pages, n)
	}
	cells := pages[0]
	gridW := float64(g.Columns)*g.CellWidth + float64(g.Columns-1)*FullPageGap
	gridH := float64(g.Rows)*g.CoverHeight + float64(g.Rows-1)*FullPageGap
	grid := Box{X: g.MarginX, Y: g.MarginY, W: gridW, H: gridH}
	for i, c := range cells {
		if !inside(c, usable) {
			t.Errorf("%s: cell %d %+v outside the usable area %+v", name, i, c, usable)
		}
		if header && c.Y < band.Y+band.H-eps {
			t.Errorf("%s: cell %d %+v overlaps the header band %+v", name, i, c, band)
		}
		if !inside(c, grid) {
			t.Errorf("%s: cell %d %+v outside the grid %+v", name, i, c, grid)
		}
		for j := range i {
			if overlap(c, cells[j]) {
				t.Errorf("%s: cells %d and %d overlap", name, j, i)
			}
		}
	}
	// A tight gap between neighbours.
	if g.Columns > 1 && n > 1 && !near(cells[1].X-(cells[0].X+cells[0].W), FullPageGap) {
		t.Errorf("%s: column gap %v mm, want %v", name, cells[1].X-(cells[0].X+cells[0].W), FullPageGap)
	}
	if n > g.Columns && !near(cells[g.Columns].Y-(cells[0].Y+cells[0].H), FullPageGap) {
		t.Errorf("%s: row gap %v mm, want %v", name, cells[g.Columns].Y-(cells[0].Y+cells[0].H), FullPageGap)
	}
	// The full grid is centred across; the header and grid block, down.
	if !inside(grid, usable) || !near(grid.X, page.Width-grid.X-grid.W) {
		t.Errorf("%s: grid %+v not centred across a %v mm page", name, grid, page.Width)
	}
	if !near(top, page.Height-grid.Y-grid.H) {
		t.Errorf("%s: block from %v to %v mm, not centred on a %v mm page", name, top, grid.Y+grid.H, page.Height)
	}
}

func TestNewFullPageGridRejectsNoCovers(t *testing.T) {
	for _, count := range []int{0, -1} {
		if _, err := NewFullPageGrid(A4, count, false); err == nil {
			t.Errorf("NewFullPageGrid(A4, %d): want an error", count)
		}
	}
}

func TestGridLimit(t *testing.T) {
	tests := []struct {
		limit, want int
	}{
		{0, 64},  // no limit: the whole grid
		{60, 60}, // the per-page cap
		{64, 64},
		{100, 64}, // a limit past the grid changes nothing
	}
	for _, tt := range tests {
		g := Grid{Page: A4, CoverHeight: 30, CellWidth: 20, Columns: 8, Rows: 8, MarginX: 10, MarginY: 10, Limit: tt.limit}
		if got := g.PerPage(); got != tt.want {
			t.Errorf("limit %d: PerPage() = %d, want %d", tt.limit, got, tt.want)
		}
	}
	// Layout and Cells paginate at the limit.
	g := Grid{Page: A4, CoverHeight: 30, CellWidth: 20, Columns: 8, Rows: 8, MarginX: 10, MarginY: 10, Limit: 60}
	boxes, err := g.Layout(ratios(121, minCellRatio))
	if err != nil {
		t.Fatal(err)
	}
	cells, err := g.Cells(121)
	if err != nil {
		t.Fatal(err)
	}
	for _, pages := range [][][]Box{boxes, cells} {
		if len(pages) != 3 || len(pages[0]) != 60 || len(pages[1]) != 60 || len(pages[2]) != 1 {
			t.Errorf("pages of %d, %d, …; want 60, 60, 1", len(pages[0]), len(pages[1]))
		}
		// Every page starts at the same cell.
		if pages[1][0] != pages[0][0] || pages[2][0] != pages[0][0] {
			t.Errorf("first boxes %+v, %+v, %+v; want the same", pages[0][0], pages[1][0], pages[2][0])
		}
	}
}

// The options of a full page give the grid of a full page of covers, with
// a header band only when there is a header, and 60 per page.
func TestOptionsFullPageGrid(t *testing.T) {
	for _, p := range Papers {
		for _, header := range []string{"", "  ", "Books of 2026"} {
			// The size and cut mode do not apply.
			for _, opts := range []Options{
				{Format: "fullpage", Paper: p.ID, Header: header},
				{Format: "fullpage", Paper: p.ID, Header: header, Size: "mini", Cut: "bleed"},
			} {
				got, err := opts.Grid()
				if err != nil {
					t.Fatal(err)
				}
				want, err := NewFullPageGrid(p.Page, FullPageMax, strings.TrimSpace(header) != "")
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("%+v: grid %+v, want %+v", opts, got, want)
				}
				if n, err := opts.PerPage(); err != nil || n != FullPageMax {
					t.Errorf("%+v: PerPage() = %d, %v; want %d", opts, n, err, FullPageMax)
				}
			}
		}
	}
	if _, err := (Options{Format: "fullpage", Paper: "a3"}).Grid(); err == nil {
		t.Error("unknown paper: want an error")
	}
}

// Covers keep their aspect ratio and stay inside their 2:3 cells.
func TestFullPageLayoutKeepsAspectRatio(t *testing.T) {
	rs := []float64{minCellRatio, 1, 1.5, 0.4, 0.7, 2}
	for _, p := range Papers {
		g, err := NewFullPageGrid(p.Page, len(rs), true)
		if err != nil {
			t.Fatal(err)
		}
		pages, err := g.Layout(rs)
		if err != nil {
			t.Fatal(err)
		}
		cells, _ := g.Cells(len(rs))
		if len(pages) != 1 {
			t.Fatalf("%s: %d pages, want 1", p.ID, len(pages))
		}
		for i, b := range pages[0] {
			if !near(b.W/b.H, rs[i]) {
				t.Errorf("%s: cover %d is %v wide, want %v", p.ID, i, b.W/b.H, rs[i])
			}
			if !inside(b, cells[0][i]) {
				t.Errorf("%s: cover %d %+v outside its cell %+v", p.ID, i, b, cells[0][i])
			}
		}
	}
}

func TestLayoutFullPageHeader(t *testing.T) {
	const width = 194.0 // A4 usable width
	short := layoutFullPageHeader("Books of 2026", width, fakeMeasure)
	if short.Text != "Books of 2026" || short.Size != maxHeaderFontSize {
		t.Errorf("short header: %+v, want it whole at %v pt", short, maxHeaderFontSize)
	}
	// Runs of spaces collapse; surrounding space goes.
	if got := layoutFullPageHeader("  Watched \t in  March ", width, fakeMeasure); got.Text != "Watched in March" {
		t.Errorf("spaced header: %+v, want %q", got, "Watched in March")
	}
	medium := strings.Repeat("Books ", 9) + "of 2026"
	if got := layoutFullPageHeader(medium, width, fakeMeasure); got.Text != medium || got.Size >= maxHeaderFontSize || got.Size < minHeaderFontSize {
		t.Errorf("medium header: %+v, want it whole, shrunk", got)
	}
	long := strings.Repeat("Books of 2026 ", 20)
	if got := layoutFullPageHeader(long, width, fakeMeasure); got.Size != minHeaderFontSize || !strings.HasSuffix(got.Text, ellipsis) {
		t.Errorf("long header: %+v, want it cut at %v pt", got, minHeaderFontSize)
	}
	for _, empty := range []string{"", " \t "} {
		if got := layoutFullPageHeader(empty, width, fakeMeasure); got.Text != "" {
			t.Errorf("header %q: %+v, want no text", empty, got)
		}
	}
}

// Whatever the text and page, the header never runs wider than the usable
// width, and stays within its font sizes.
func TestLayoutFullPageHeaderFits(t *testing.T) {
	texts := []string{
		"Books of 2026",
		"Libros leídos en 2026 · Ünïcödé Çréâtör",
		strings.Repeat("Watched in March ", 12),
		"Supercalifragilisticexpialidociousandevenlongerthanthatbyquitealotmoreletters",
	}
	for _, p := range Papers {
		width := p.Page.Width - 2*MinMargin
		for _, text := range texts {
			got := layoutFullPageHeader(text, width, fakeMeasure)
			if got.Text == "" {
				t.Errorf("%s, %q: no header", p.ID, text)
			}
			if w := fakeMeasure(got.Text, true, got.Size); w > width+eps {
				t.Errorf("%s, %q: %q is %v mm wide, room for %v", p.ID, text, got.Text, w, width)
			}
			if got.Size < minHeaderFontSize || got.Size > maxHeaderFontSize {
				t.Errorf("%s, %q: %v pt, want %v–%v", p.ID, text, got.Size, minHeaderFontSize, maxHeaderFontSize)
			}
			if lineHeight(got.Size) > FullPageHeaderHeight {
				t.Errorf("%s, %q: a %v pt line is taller than the header band", p.ID, text, got.Size)
			}
		}
	}
}

// pageStreamsEach returns the content stream of every page of an
// uncompressed PDF, with image names replaced by "/I". The ToUnicode maps
// of embedded fonts, which also have only a length, are left out.
func pageStreamsEach(pdf []byte) []string {
	var out []string
	for _, m := range pageContent.FindAllSubmatch(pdf, -1) {
		if bytes.Contains(m[1], []byte("begincmap")) {
			continue
		}
		out = append(out, imageName.ReplaceAllString(string(m[1]), "/I Do"))
	}
	return out
}

// imagePlacement matches the matrix an image is drawn with.
var imagePlacement = regexp.MustCompile(`q [0-9. ]+ cm /I Do Q`)

func TestBuildFullPagePages(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{200, 120, 40, 255})
	tests := []struct {
		paper  string
		covers int
		header string
		pages  []int // covers on each page
	}{
		{"", 1, "", []int{1}},
		{"a4", 7, "Books of 2026", []int{7}},
		{"a4", 60, "Books of 2026", []int{60}},
		{"a4", 61, "Books of 2026", []int{60, 1}},
		{"letter", 125, "Watched in March", []int{60, 60, 5}},
		{"a5", 65, "", []int{60, 5}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%d/%q", tt.paper, tt.covers, tt.header), func(t *testing.T) {
			paths := make([]string, tt.covers)
			for i := range paths {
				paths[i] = cover
			}
			pdf := render(t, paths, Options{Format: "fullpage", Paper: tt.paper, Header: tt.header})
			if n := len(pageObject.FindAll(pdf, -1)); n != len(tt.pages) {
				t.Fatalf("PDF has %d pages, want %d", n, len(tt.pages))
			}
			streams := pageStreamsEach(pdf)
			if len(streams) != len(tt.pages) {
				t.Fatalf("%d page streams, want %d", len(streams), len(tt.pages))
			}
			first := ""
			for i, s := range streams {
				if n := strings.Count(s, " Do Q"); n != tt.pages[i] {
					t.Errorf("page %d: %d covers, want %d", i+1, n, tt.pages[i])
				}
				// The header, on every page, once.
				want := 0
				if tt.header != "" {
					want = 1
				}
				if n := strings.Count(s, " Tj"); n != want {
					t.Errorf("page %d: %d texts, want %d", i+1, n, want)
				}
				// No cut borders, crop marks or frame.
				if strings.Contains(s, " re S") || strings.Contains(s, " l S") || strings.Contains(s, " re W") {
					t.Errorf("page %d draws lines or clips:\n%s", i+1, s)
				}
				// Every page uses the same grid: its first cover sits
				// where the first page's does, at the same size.
				at := imagePlacement.FindString(s)
				if i == 0 {
					first = at
				} else if at != first {
					t.Errorf("page %d: first cover %q, want %q", i+1, at, first)
				}
			}
			if hasFont := bytes.Contains(pdf, []byte("/FontFile2")); hasFont != (tt.header != "") {
				t.Errorf("font embedded: %v, want %v", hasFont, tt.header != "")
			}
		})
	}
}

// One page of a few mixed covers matches the grid NewFullPageGrid derives
// for that count, not the full-page one.
func TestBuildFullPageSizesCoversToTheirCount(t *testing.T) {
	paths := cutFixtures(t)
	got := pageStreams(render(t, paths, Options{Format: "fullpage", Header: "Books of 2026"}))
	g, err := NewFullPageGrid(A4, len(paths), true)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := g.Layout([]float64{0.66, 1, 1.5})
	if err != nil {
		t.Fatal(err)
	}
	const k = 72 / 25.4
	for _, b := range pages[0] {
		at := fmt.Sprintf("q %.5f 0 0 %.5f %.5f %.5f cm /I Do Q", b.W*k, b.H*k, b.X*k, (A4.Height-b.Y-b.H)*k)
		if !strings.Contains(got, at) {
			t.Errorf("no cover at %q in\n%s", at, got)
		}
	}
	if math.Abs(g.CoverHeight-FullPageMaxCover) > eps {
		t.Errorf("three covers are %v mm tall, want the %v mm cap", g.CoverHeight, FullPageMaxCover)
	}
}

// The size preset and cut mode do not apply to a full page.
func TestBuildFullPageIgnoresSizeAndCut(t *testing.T) {
	paths := cutFixtures(t)
	want := pageStreams(render(t, paths, Options{Format: "fullpage", Header: "Books of 2026"}))
	for _, s := range Sizes {
		for _, c := range CutModes {
			opts := Options{Format: "fullpage", Header: "Books of 2026", Size: s.ID, Cut: c.ID}
			if got := pageStreams(render(t, paths, opts)); got != want {
				t.Errorf("%+v: page content differs from the default", opts)
			}
		}
	}
}

// A long header is shrunk or cut on the page, never wider than the usable
// width: the text starts inside the left margin.
func TestBuildFullPageLongHeaderStaysInside(t *testing.T) {
	paths := cutFixtures(t)
	long := strings.Repeat("Libros leídos en 2026 ", 15)
	got := pageStreams(render(t, paths, Options{Format: "fullpage", Paper: "a5", Header: long}))
	m := regexp.MustCompile(`BT ([0-9.]+) [0-9.]+ Td`).FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("no header text in\n%s", got)
	}
	var x float64
	fmt.Sscan(m[1], &x)
	if x < MinMargin*72/25.4-0.01 {
		t.Errorf("header starts at %v pt, inside the %v mm margin", x, MinMargin)
	}
}
