package sheet

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strings"
	"testing"
)

// checklistTop is where the first row of a checklist starts: right below
// the header band if there is a header, else at the top margin.
func checklistTop(header bool) float64 {
	if header {
		return MinMargin + FullPageHeaderHeight
	}
	return MinMargin
}

func TestNewChecklistGridRows(t *testing.T) {
	tests := []struct {
		paper  string
		header bool
		rows   int
	}{
		{"a4", false, 17},
		{"a4", true, 16},
		{"letter", false, 16},
		{"letter", true, 15},
		{"a5", false, 12},
		{"a5", true, 11},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/header=%v", tt.paper, tt.header), func(t *testing.T) {
			p, err := PaperByID(tt.paper)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewChecklistGrid(p.Page, tt.header)
			if err != nil {
				t.Fatal(err)
			}
			if g.Rows != tt.rows || g.Columns != 1 || g.PerPage() != tt.rows {
				t.Errorf("grid %d × %d, %d per page; want 1 × %d", g.Columns, g.Rows, g.PerPage(), tt.rows)
			}
			// Rows per page = floor(room below the header / row height).
			room := p.Page.Height - checklistTop(tt.header) - MinMargin
			if want := int(math.Floor(room / ChecklistRowHeight)); g.Rows != want {
				t.Errorf("%d rows, want floor(%v / %v) = %d", g.Rows, room, ChecklistRowHeight, want)
			}
			if g.Page != p.Page || !near(g.MarginX, MinMargin) || !near(g.CellWidth, p.Page.Width-2*MinMargin) ||
				!near(g.CoverHeight, ChecklistRowHeight) || !near(g.MarginY, checklistTop(tt.header)) {
				t.Errorf("grid %+v: want full-width rows %v mm tall from y = %v", g, ChecklistRowHeight, checklistTop(tt.header))
			}
		})
	}
}

func TestNewChecklistGridRejectsTinyPages(t *testing.T) {
	for _, page := range []Page{{Width: 30, Height: 30}, {Width: 100, Height: 20}} {
		if _, err := NewChecklistGrid(page, true); err == nil {
			t.Errorf("%v × %v mm page: want an error", page.Width, page.Height)
		}
	}
}

func TestOptionsChecklistGrid(t *testing.T) {
	for _, p := range Papers {
		for _, header := range []string{"", "  ", "TBR 2026"} {
			// The size and cut mode do not apply.
			for _, opts := range []Options{
				{Format: "checklist", Paper: p.ID, Header: header},
				{Format: "checklist", Paper: p.ID, Header: header, Size: "mini", Cut: "bleed"},
			} {
				got, err := opts.Grid()
				if err != nil {
					t.Fatal(err)
				}
				want, err := NewChecklistGrid(p.Page, strings.TrimSpace(header) != "")
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("%+v: grid %+v, want %+v", opts, got, want)
				}
				if n, err := opts.PerPage(); err != nil || n != want.Rows {
					t.Errorf("%+v: PerPage() = %d, %v; want %d", opts, n, err, want.Rows)
				}
			}
		}
	}
	if _, err := (Options{Format: "checklist", Paper: "a3"}).Grid(); err == nil {
		t.Error("unknown paper: want an error")
	}
}

// Rows touch, fill the pages top-down within the margins, below the
// header band, and paginate on the same rows.
func TestChecklistRowsPaginateInsideTheMargins(t *testing.T) {
	for _, p := range Papers {
		for _, header := range []bool{false, true} {
			g, err := NewChecklistGrid(p.Page, header)
			if err != nil {
				t.Fatal(err)
			}
			count := 2*g.Rows + 3
			pages, err := g.Cells(count)
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 3 || len(pages[0]) != g.Rows || len(pages[1]) != g.Rows || len(pages[2]) != 3 {
				t.Fatalf("%s, header %v: pages of %d, %d, %d rows; want %d, %d, 3", p.ID, header,
					len(pages[0]), len(pages[1]), len(pages[len(pages)-1]), g.Rows, g.Rows)
			}
			usable := Box{X: MinMargin, Y: checklistTop(header), W: p.Page.Width - 2*MinMargin, H: p.Page.Height - checklistTop(header) - MinMargin}
			for i, page := range pages {
				for j, row := range page {
					if !inside(row, usable) {
						t.Errorf("%s, header %v: page %d row %d %+v outside %+v", p.ID, header, i+1, j, row, usable)
					}
					if row != pages[0][j] {
						t.Errorf("%s, header %v: page %d row %d %+v, want it where page 1's is, %+v", p.ID, header, i+1, j, row, pages[0][j])
					}
					if j > 0 && !near(row.Y, page[j-1].Y+page[j-1].H) {
						t.Errorf("%s, header %v: row %d starts at %v, want right below row %d at %v", p.ID, header, j, row.Y, j-1, page[j-1].Y+page[j-1].H)
					}
				}
			}
			if !near(pages[0][0].Y, checklistTop(header)) {
				t.Errorf("%s, header %v: first row at %v, want %v", p.ID, header, pages[0][0].Y, checklistTop(header))
			}
		}
	}
}

// checklistRatios are typical, landscape, square, narrow and very wide
// image shapes.
var checklistRatios = []float64{minCellRatio, 0.66, 1.5, 1, 0.4, 3}

func TestLayoutChecklistRowParts(t *testing.T) {
	row := Box{X: MinMargin, Y: 40, W: A4.Width - 2*MinMargin, H: ChecklistRowHeight}
	for _, r := range checklistRatios {
		for _, hasCreator := range []bool{true, false} {
			name := fmt.Sprintf("ratio %v, creator %v", r, hasCreator)
			c := layoutChecklistRow(row, r, hasCreator)
			if c.Row != row {
				t.Errorf("%s: row %+v, want %+v", name, c.Row, row)
			}
			for part, b := range map[string]Box{"checkbox": c.Check, "thumbnail": c.Thumb, "cover": c.Cover, "title": c.Title} {
				if !inside(b, row) {
					t.Errorf("%s: %s %+v outside the row %+v", name, part, b, row)
				}
			}
			// The checkbox: a square, at the left, vertically centred.
			if !near(c.Check.W, ChecklistBoxSize) || !near(c.Check.H, ChecklistBoxSize) || !near(c.Check.X, row.X) ||
				!near(c.Check.Y+c.Check.H/2, row.Y+row.H/2) {
				t.Errorf("%s: checkbox %+v, want a %v mm square at the left, centred", name, c.Check, ChecklistBoxSize)
			}
			// The thumbnail: a 2:3 box as tall as the row less its padding,
			// right of the checkbox.
			if !near(c.Thumb.H, row.H-2*checklistPadding) || !near(c.Thumb.W, c.Thumb.H*minCellRatio) ||
				!near(c.Thumb.Y, row.Y+checklistPadding) || c.Thumb.X < c.Check.X+c.Check.W+eps {
				t.Errorf("%s: thumbnail %+v, want a 2:3 box %v mm tall right of the checkbox", name, c.Thumb, row.H-2*checklistPadding)
			}
			// The image keeps its ratio, fits the box and fills it one way.
			if !near(c.Cover.W/c.Cover.H, r) || !inside(c.Cover, c.Thumb) ||
				!(near(c.Cover.W, c.Thumb.W) || near(c.Cover.H, c.Thumb.H)) {
				t.Errorf("%s: cover %+v in %+v, want ratio %v shrunk to fit", name, c.Cover, c.Thumb, r)
			}
			if !near(c.Cover.X+c.Cover.W/2, c.Thumb.X+c.Thumb.W/2) || !near(c.Cover.Y+c.Cover.H/2, c.Thumb.Y+c.Thumb.H/2) {
				t.Errorf("%s: cover %+v not centred in %+v", name, c.Cover, c.Thumb)
			}
			// The text runs from right of the thumbnail to the right margin.
			if c.Title.X < c.Thumb.X+c.Thumb.W+eps || !near(c.Title.X+c.Title.W, row.X+row.W) {
				t.Errorf("%s: title %+v, want it from right of %+v to the row's right edge", name, c.Title, c.Thumb)
			}
			if !near(c.Title.H, lineHeight(checklistTitleFontSize)) {
				t.Errorf("%s: title %v mm tall, want one %v pt line", name, c.Title.H, checklistTitleFontSize)
			}
			if hasCreator {
				if !inside(c.Creator, row) || c.Creator.X != c.Title.X || c.Creator.W != c.Title.W ||
					!near(c.Creator.Y, c.Title.Y+c.Title.H) || !near(c.Creator.H, lineHeight(checklistCreatorFontSize)) {
					t.Errorf("%s: creator %+v, want one %v pt line right under the title %+v", name, c.Creator, checklistCreatorFontSize, c.Title)
				}
				// Title and creator together are centred down the row.
				if mid := (c.Title.Y + c.Creator.Y + c.Creator.H) / 2; !near(mid, row.Y+row.H/2) {
					t.Errorf("%s: text centred at %v, want %v", name, mid, row.Y+row.H/2)
				}
			} else {
				if c.Creator != (Box{}) {
					t.Errorf("%s: creator %+v, want none", name, c.Creator)
				}
				if !near(c.Title.Y+c.Title.H/2, row.Y+row.H/2) {
					t.Errorf("%s: title %+v, want it centred down the row", name, c.Title)
				}
			}
		}
	}
}

// Title and creator are cut with "…" to one line each, never wider than
// the text room, measured with the embedded fonts.
func TestFitChecklistText(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := func(text string, bold bool, pt float64) float64 {
		pdf.SetFont(textFont, fontStyle(bold), pt)
		return pdf.GetStringWidth(text)
	}
	long := "Cien años de soledad, edición conmemorativa de la Real Academia Española y la Asociación de Academias"
	longCreator := strings.Repeat("Ünïcödé Çréâtör Wïth À Vêry Löng Nämé, ", 4) + "And Several Co-Authors"
	tests := []struct {
		name, title, creator   string
		wantTitle, wantCreator string // "" means cut with "…"
	}{
		{"short", "Dune", "Frank Herbert", "Dune", "Frank Herbert"},
		{"movie year", "Blade Runner", "1982", "Blade Runner", "1982"},
		{"no creator", "Square", "", "Square", ""},
		{"long title", long, "Gabriel García Márquez", "", "Gabriel García Márquez"},
		{"long creator", "Dune", longCreator, "Dune", ""},
		{"one long word", strings.Repeat("Supercalifragilistic", 12), "", "", ""},
	}
	for _, p := range Papers {
		row := layoutChecklistRow(Box{X: MinMargin, Y: MinMargin, W: p.Page.Width - 2*MinMargin, H: ChecklistRowHeight}, minCellRatio, true)
		width := row.Title.W
		for _, tt := range tests {
			title, creator := fitChecklistText(tt.title, tt.creator, width, measure)
			if tt.wantTitle != "" && title != tt.wantTitle {
				t.Errorf("%s, %s: title %q, want %q", p.ID, tt.name, title, tt.wantTitle)
			}
			if tt.wantTitle == "" && !strings.HasSuffix(title, ellipsis) {
				t.Errorf("%s, %s: title %q, want it cut with %q", p.ID, tt.name, title, ellipsis)
			}
			if tt.creator == "" && creator != "" {
				t.Errorf("%s, %s: creator %q, want none", p.ID, tt.name, creator)
			}
			if tt.creator != "" && tt.wantCreator != "" && creator != tt.wantCreator {
				t.Errorf("%s, %s: creator %q, want %q", p.ID, tt.name, creator, tt.wantCreator)
			}
			if tt.creator != "" && tt.wantCreator == "" && !strings.HasSuffix(creator, ellipsis) {
				t.Errorf("%s, %s: creator %q, want it cut with %q", p.ID, tt.name, creator, ellipsis)
			}
			if w := measure(title, true, checklistTitleFontSize); w > width+eps {
				t.Errorf("%s, %s: title %q is %v mm wide, room for %v", p.ID, tt.name, title, w, width)
			}
			if w := measure(creator, false, checklistCreatorFontSize); w > width+eps {
				t.Errorf("%s, %s: creator %q is %v mm wide, room for %v", p.ID, tt.name, creator, w, width)
			}
		}
	}
}

// textAt matches the start of a line of text: its x and y, in points.
var textAt = regexp.MustCompile(`BT ([0-9.]+) ([0-9.]+) Td`)

func TestBuildChecklistPages(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{200, 120, 40, 255})
	tests := []struct {
		paper  string
		items  int
		header string
		pages  []int // rows on each page
	}{
		{"", 1, "", []int{1}},
		{"a4", 17, "", []int{17}},
		{"a4", 18, "", []int{17, 1}},
		{"a4", 16, "TBR 2026", []int{16}},
		{"a4", 40, "TBR 2026", []int{16, 16, 8}},
		{"letter", 31, "Watchlist", []int{15, 15, 1}},
		{"a5", 12, "", []int{12}},
		{"a5", 23, "Leídos 2026", []int{11, 11, 1}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%d/%q", tt.paper, tt.items, tt.header), func(t *testing.T) {
			paths := make([]string, tt.items)
			for i := range paths {
				paths[i] = cover
			}
			pdf := render(t, paths, Options{Format: "checklist", Paper: tt.paper, Header: tt.header})
			if n := len(pageObject.FindAll(pdf, -1)); n != len(tt.pages) {
				t.Fatalf("PDF has %d pages, want %d", n, len(tt.pages))
			}
			streams := pageStreamsEach(pdf)
			if len(streams) != len(tt.pages) {
				t.Fatalf("%d page streams, want %d", len(streams), len(tt.pages))
			}
			header := 0
			if tt.header != "" {
				header = 1
			}
			firstText := ""
			for i, s := range streams {
				rows := tt.pages[i]
				if n := strings.Count(s, " Do Q"); n != rows {
					t.Errorf("page %d: %d thumbnails, want %d", i+1, n, rows)
				}
				// A checkbox and a rule per row; no other boxes or lines.
				if n := strings.Count(s, " re S"); n != rows {
					t.Errorf("page %d: %d boxes, want %d checkboxes", i+1, n, rows)
				}
				if n := strings.Count(s, " l S"); n != rows {
					t.Errorf("page %d: %d lines, want %d rules", i+1, n, rows)
				}
				// The header on every page, then a title and a creator per
				// row.
				if n := strings.Count(s, " Tj"); n != header+2*rows {
					t.Errorf("page %d: %d texts, want %d", i+1, n, header+2*rows)
				}
				// Every page starts the same way: the header, or the first
				// title, at the same place.
				at := textAt.FindString(s)
				if i == 0 {
					firstText = at
				} else if at != firstText {
					t.Errorf("page %d: first text %q, want %q", i+1, at, firstText)
				}
			}
		})
	}
}

// An item without a creator prints its title alone; a long title is cut,
// and no text starts outside the margins.
func TestBuildChecklistTextStaysInside(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "Square.png", 100, 100, color.RGBA{40, 200, 40, 255}),
		writeImage(t, dir, strings.Repeat("A Very Long Title ", 12)+"- Somebody.jpg", 150, 100, color.RGBA{40, 40, 200, 255}),
	}
	for _, p := range Papers {
		s := pageStreams(render(t, paths, Options{Format: "checklist", Paper: p.ID, Header: strings.Repeat("Libros leídos en 2026 ", 15)}))
		// Header, title alone, then title and creator.
		if n := strings.Count(s, " Tj"); n != 4 {
			t.Errorf("%s: %d texts, want 4", p.ID, n)
		}
		const k = 72 / 25.4
		for _, m := range textAt.FindAllStringSubmatch(s, -1) {
			var x, y float64
			fmt.Sscan(m[1], &x)
			fmt.Sscan(m[2], &y)
			if x < MinMargin*k-0.01 || x > (p.Page.Width-MinMargin)*k || y < MinMargin*k || y > (p.Page.Height-MinMargin)*k {
				t.Errorf("%s: text at (%v, %v) pt, outside the margins", p.ID, x, y)
			}
		}
	}
}

// The size preset and cut mode do not apply to a checklist.
func TestBuildChecklistIgnoresSizeAndCut(t *testing.T) {
	paths := cutFixtures(t)
	want := pageStreams(render(t, paths, Options{Format: "checklist", Header: "TBR 2026"}))
	for _, s := range Sizes {
		for _, c := range CutModes {
			opts := Options{Format: "checklist", Header: "TBR 2026", Size: s.ID, Cut: c.ID}
			if got := pageStreams(render(t, paths, opts)); got != want {
				t.Errorf("%+v: page content differs from the default", opts)
			}
		}
	}
}

// Thumbnails are drawn where the row layout puts them, mixed shapes
// included.
func TestBuildChecklistPlacesThumbnails(t *testing.T) {
	paths := cutFixtures(t)
	got := pageStreams(render(t, paths, Options{Format: "checklist", Paper: "a5"}))
	g, err := NewChecklistGrid(A5, false)
	if err != nil {
		t.Fatal(err)
	}
	cells, err := g.Cells(len(paths))
	if err != nil {
		t.Fatal(err)
	}
	const k = 72 / 25.4
	for i, r := range []float64{0.66, 1, 1.5} {
		_, creator := ParseName(paths[i])
		b := layoutChecklistRow(cells[0][i], r, creator != "").Cover
		at := fmt.Sprintf("q %.5f 0 0 %.5f %.5f %.5f cm /I Do Q", b.W*k, b.H*k, b.X*k, (A5.Height-b.Y-b.H)*k)
		if !strings.Contains(got, at) {
			t.Errorf("no thumbnail at %q in\n%s", at, got)
		}
	}
}
