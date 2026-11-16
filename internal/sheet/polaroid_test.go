package sheet

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestPolaroidFrameFollowsTheCoverHeight(t *testing.T) {
	tests := []struct {
		size                 string
		coverHeight          float64
		width, height        float64 // the frame
		border, band, window float64 // side and top border, bottom band, window width
	}{
		// Window 2/3 H × H; borders 0.08 H; band 0.28 H.
		{"mini", 25, 50.0/3 + 4, 34, 2, 7, 50.0 / 3},
		{"standard", 50, 100.0/3 + 8, 68, 4, 14, 100.0 / 3},
		{"large", 80, 160.0/3 + 12.8, 108.8, 6.4, 22.4, 160.0 / 3},
	}
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			s, err := SizeByID(tt.size)
			if err != nil || s.CoverHeight != tt.coverHeight {
				t.Fatalf("size %s: %+v, %v; want a %v mm cover", tt.size, s, err, tt.coverHeight)
			}
			w, h := polaroidFrame(s.CoverHeight)
			if !near(w, tt.width) || !near(h, tt.height) {
				t.Errorf("frame %v × %v, want %v × %v", w, h, tt.width, tt.height)
			}
			p := layoutPolaroid(Box{X: 10, Y: 20, W: w, H: h}, minCellRatio)
			if !near(p.Window.W, tt.window) || !near(p.Window.H, tt.coverHeight) {
				t.Errorf("window %+v, want %v × %v", p.Window, tt.window, tt.coverHeight)
			}
			if !near(p.Window.X-10, tt.border) || !near(p.Window.Y-20, tt.border) || !near(10+w-(p.Window.X+p.Window.W), tt.border) {
				t.Errorf("window %+v in frame at (10, 20): want %v mm borders left, top and right", p.Window, tt.border)
			}
			if !near(p.Band.H, tt.band) || !near(p.Band.Y, p.Window.Y+p.Window.H) || !near(p.Band.Y+p.Band.H, 20+h) {
				t.Errorf("band %+v, want %v mm under the window, to the bottom edge", p.Band, tt.band)
			}
		})
	}
}

func TestPolaroidGridPapersAndSizes(t *testing.T) {
	tests := []struct {
		paper, size string
		cols, rows  int
	}{
		{"a4", "mini", 8, 7},
		{"a4", "standard", 4, 3},
		{"a4", "large", 2, 2},
		{"letter", "mini", 8, 7},
		{"letter", "standard", 4, 3},
		{"letter", "large", 2, 2},
		{"a5", "mini", 5, 5},
		{"a5", "standard", 3, 2}, // three frames fill the width exactly
		{"a5", "large", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.paper+"/"+tt.size, func(t *testing.T) {
			g, err := Options{Format: "polaroid", Paper: tt.paper, Size: tt.size}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			p, _ := PaperByID(tt.paper)
			s, _ := SizeByID(tt.size)
			w, h := polaroidFrame(s.CoverHeight)
			if g.Page != p.Page || g.Columns != tt.cols || g.Rows != tt.rows || !near(g.CellWidth, w) || !near(g.CoverHeight, h) {
				t.Errorf("grid %+v, want %d × %d frames of %v × %v", g, tt.cols, tt.rows, w, h)
			}
			if n, err := (Options{Format: "polaroid", Paper: tt.paper, Size: tt.size}).PerPage(); err != nil || n != tt.cols*tt.rows {
				t.Errorf("PerPage() = %d, %v; want %d", n, err, tt.cols*tt.rows)
			}
			if g.MarginX < MinMargin-eps || g.MarginY < MinMargin-eps {
				t.Errorf("margins %v / %v below %v", g.MarginX, g.MarginY, MinMargin)
			}
			// Full: no room for another column or row.
			if float64(g.Columns+1)*w+float64(g.Columns)*Gap <= p.Page.Width-2*MinMargin {
				t.Errorf("%d columns fit, grid has %d", g.Columns+1, g.Columns)
			}
			if float64(g.Rows+1)*h+float64(g.Rows)*Gap <= p.Page.Height-2*MinMargin {
				t.Errorf("%d rows fit, grid has %d", g.Rows+1, g.Rows)
			}
			// Centred both ways: the last frame leaves the same margins.
			pages, err := g.Cells(g.PerPage())
			if err != nil {
				t.Fatal(err)
			}
			last := pages[0][len(pages[0])-1]
			if right, bottom := p.Page.Width-(last.X+last.W), p.Page.Height-(last.Y+last.H); !near(right, g.MarginX) || !near(bottom, g.MarginY) {
				t.Errorf("right / bottom margins %v / %v, want %v / %v", right, bottom, g.MarginX, g.MarginY)
			}
		})
	}
	// No size means the standard one.
	def, err := Options{Format: "polaroid"}.Grid()
	if err != nil {
		t.Fatal(err)
	}
	if std, _ := (Options{Format: "polaroid", Paper: "a4", Size: "standard"}).Grid(); def != std {
		t.Errorf("default grid %+v, want the A4 standard one %+v", def, std)
	}
	if _, err := (Options{Format: "polaroid", Size: "huge"}).Grid(); err == nil {
		t.Error(`polaroid size "huge": want an error`)
	}
}

func TestNewPolaroidGridRejectsWhatDoesNotFit(t *testing.T) {
	tests := []struct {
		name        string
		page        Page
		coverHeight float64
	}{
		{"narrow page", Page{Width: 40, Height: 297}, 50},
		{"short page", Page{Width: 210, Height: 60}, 50},
		{"no height", A4, 0},
		{"negative height", A4, -50},
		{"not a number", A4, math.NaN()},
	}
	for _, tt := range tests {
		if g, err := NewPolaroidGrid(tt.page, tt.coverHeight); err == nil {
			t.Errorf("%s: NewPolaroidGrid = %+v, want an error", tt.name, g)
		}
	}
}

// polaroidRatios are covers of every shape: very tall, typical, 2:3,
// square, wide and panoramic.
var polaroidRatios = []float64{0.3, 0.656, 2.0 / 3, 1, 16.0 / 9, 3}

// Every part sits inside its frame, on the page; the window and the band
// do not overlap, and the image keeps its ratio inside the window.
func TestLayoutPolaroidPartsInsideTheFrame(t *testing.T) {
	for _, p := range Papers {
		for _, s := range Sizes {
			g, err := Options{Format: "polaroid", Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			pages, err := g.Cells(g.PerPage())
			if err != nil {
				t.Fatal(err)
			}
			pageBox := Box{0, 0, p.Page.Width, p.Page.Height}
			for i, frame := range pages[0] {
				r := polaroidRatios[i%len(polaroidRatios)]
				pp := layoutPolaroid(frame, r)
				name := fmt.Sprintf("%s %s frame %d (ratio %v)", p.ID, s.ID, i, r)
				if pp.Frame != frame {
					t.Errorf("%s: frame %+v, want the cell %+v", name, pp.Frame, frame)
				}
				if !inside(pp.Frame, pageBox) {
					t.Errorf("%s: %+v off the page", name, pp.Frame)
				}
				for part, b := range map[string]Box{"window": pp.Window, "band": pp.Band, "caption": pp.Caption, "cover": pp.Cover} {
					if !inside(b, frame) {
						t.Errorf("%s: %s %+v outside the frame %+v", name, part, b, frame)
					}
				}
				if overlap(pp.Window, pp.Band) || overlap(pp.Window, pp.Caption) {
					t.Errorf("%s: window %+v overlaps the band %+v or caption %+v", name, pp.Window, pp.Band, pp.Caption)
				}
				if !inside(pp.Caption, pp.Band) || !near(pp.Caption.X, pp.Window.X) || !near(pp.Caption.W, pp.Window.W) {
					t.Errorf("%s: caption %+v, want the band %+v within the window's width %+v", name, pp.Caption, pp.Band, pp.Window)
				}
				if !inside(pp.Cover, pp.Window) {
					t.Errorf("%s: cover %+v outside the window %+v", name, pp.Cover, pp.Window)
				}
				if !near(pp.Window.H, s.CoverHeight) {
					t.Errorf("%s: window %v tall, want the %v mm cover height", name, pp.Window.H, s.CoverHeight)
				}
			}
		}
	}
}

func TestLayoutPolaroidCoverKeepsRatio(t *testing.T) {
	w, h := polaroidFrame(50)
	frame := Box{X: 12, Y: 15, W: w, H: h}
	tests := []struct {
		name      string
		ratio     float64
		fullWidth bool // shrunk to the window width rather than full height
	}{
		{"2:3", 2.0 / 3, true}, // both
		{"typical", 0.656, false},
		{"narrow", 0.3, false},
		{"square", 1, true},
		{"wide", 16.0 / 9, true},
		{"panoramic", 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := layoutPolaroid(frame, tt.ratio)
			if !near(p.Cover.W/p.Cover.H, tt.ratio) {
				t.Errorf("cover %+v has ratio %v, want %v", p.Cover, p.Cover.W/p.Cover.H, tt.ratio)
			}
			if !inside(p.Cover, p.Window) {
				t.Errorf("cover %+v outside the window %+v", p.Cover, p.Window)
			}
			// As large as the window allows.
			if tt.fullWidth && !near(p.Cover.W, p.Window.W) {
				t.Errorf("cover %+v, want the window width %v", p.Cover, p.Window.W)
			}
			if !tt.fullWidth && !near(p.Cover.H, p.Window.H) {
				t.Errorf("cover %+v, want the window height %v", p.Cover, p.Window.H)
			}
			// Centred in the window.
			if !near(p.Cover.X+p.Cover.W/2, p.Window.X+p.Window.W/2) || !near(p.Cover.Y+p.Cover.H/2, p.Window.Y+p.Window.H/2) {
				t.Errorf("cover %+v not centred in the window %+v", p.Cover, p.Window)
			}
		})
	}
}

func TestPolaroidFontSize(t *testing.T) {
	tests := []struct {
		coverHeight, want float64
	}{
		{10, 6}, // 1.8 pt, raised to the smallest size
		{25, 6}, // mini: 4.5 pt, raised
		{40, 7.2},
		{50, 9}, // standard
		{60, 10.8},
		{80, 12}, // large: 14.4 pt, capped
		{200, 12},
	}
	for _, tt := range tests {
		if got := polaroidFontSize(tt.coverHeight); !near(got, tt.want) {
			t.Errorf("polaroidFontSize(%v) = %v, want %v", tt.coverHeight, got, tt.want)
		}
	}
	// The size a frame carries is that of its cover height.
	for _, s := range Sizes {
		w, h := polaroidFrame(s.CoverHeight)
		if got := layoutPolaroid(Box{W: w, H: h}, 1).FontSize; !near(got, polaroidFontSize(s.CoverHeight)) {
			t.Errorf("%s: frame font size %v, want %v", s.ID, got, polaroidFontSize(s.CoverHeight))
		}
	}
}

// The caption is the title in up to two centred lines, cut with "…",
// its block centred down the band and always inside it.
func TestLayoutPolaroidCaption(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	long := "Cien años de soledad, edición conmemorativa de la Real Academia Española y la Asociación de Academias"
	tests := []struct {
		name  string
		title string
		lines int  // 0: no caption
		cut   bool // the last line ends with "…"
	}{
		{"empty", "", 0, false},
		{"blank", "   ", 0, false},
		{"short", "Dune", 1, false},
		{"long", long, 2, true},
		{"one long word", strings.Repeat("Supercalifragilistic", 12), 2, true},
	}
	for _, s := range Sizes {
		w, h := polaroidFrame(s.CoverHeight)
		p := layoutPolaroid(Box{X: 30, Y: 40, W: w, H: h}, minCellRatio)
		measure := func(text string) float64 {
			pdf.SetFont(textFont, "", p.FontSize)
			return pdf.GetStringWidth(text)
		}
		for _, tt := range tests {
			t.Run(s.ID+"/"+tt.name, func(t *testing.T) {
				lines := layoutPolaroidCaption(p.Caption, tt.title, p.FontSize, measure)
				if len(lines) != tt.lines {
					t.Fatalf("%d lines %+v, want %d", len(lines), lines, tt.lines)
				}
				if tt.lines == 0 {
					return
				}
				if cut := strings.HasSuffix(lines[len(lines)-1].Text, ellipsis); cut != tt.cut {
					t.Errorf("last line %q cut with %q: %v, want %v", lines[len(lines)-1].Text, ellipsis, cut, tt.cut)
				}
				for i, l := range lines {
					if !inside(l.Box, p.Caption) {
						t.Errorf("line %d %+v outside the caption %+v", i, l, p.Caption)
					}
					if tw := measure(l.Text); tw > p.Caption.W+eps || !near(l.W, tw) {
						t.Errorf("line %d %q is %v mm wide (box %v), room for %v", i, l.Text, tw, l.W, p.Caption.W)
					}
					if !near(l.H, lineHeight(p.FontSize)) {
						t.Errorf("line %d %+v, want %v mm tall", i, l, lineHeight(p.FontSize))
					}
					// Centred across, one under the other.
					if !near(l.X+l.W/2, p.Caption.X+p.Caption.W/2) {
						t.Errorf("line %d %+v not centred in %+v", i, l, p.Caption)
					}
					if i > 0 && !near(l.Y, lines[i-1].Y+lines[i-1].H) {
						t.Errorf("line %d at %v, want right under line %d", i, l.Y, i-1)
					}
				}
				// The block is centred down the band.
				top, bottom := lines[0].Y, lines[len(lines)-1].Y+lines[len(lines)-1].H
				if !near(top-p.Caption.Y, p.Caption.Y+p.Caption.H-bottom) {
					t.Errorf("block %v–%v not centred in %+v", top, bottom, p.Caption)
				}
			})
		}
	}
}

// polaroidFixtures writes covers of mixed shapes, with a short, a long and
// no title.
func polaroidFixtures(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	return []string{
		writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{200, 40, 40, 255}),
		writeImage(t, dir, "Square.png", 100, 100, color.RGBA{40, 200, 40, 255}),
		writeImage(t, dir, "Wide.jpg", 150, 100, color.RGBA{40, 40, 200, 255}),
		writeImage(t, dir, "Cien años de soledad edición conmemorativa de la Real Academia Española - Gabriel García Márquez.jpg", 66, 100, color.RGBA{40, 120, 200, 255}),
		writeImage(t, dir, "Narrow.png", 30, 100, color.RGBA{10, 10, 10, 255}),
		writeImage(t, dir, ".jpg", 66, 100, color.RGBA{90, 90, 90, 255}),
	}
}

func TestBuildPolaroidPages(t *testing.T) {
	fixtures := polaroidFixtures(t)
	tests := []struct {
		paper, size string
		items       int
	}{
		{"", "", 1},
		{"a4", "standard", 12},
		{"a4", "standard", 13},
		{"a4", "mini", 60},
		{"a4", "large", 9},
		{"letter", "standard", 25},
		{"a5", "standard", 7},
		{"a5", "mini", 6},
		{"a5", "large", 3},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s/%d", tt.paper, tt.size, tt.items), func(t *testing.T) {
			opts := Options{Format: "polaroid", Paper: tt.paper, Size: tt.size}
			perPage, err := opts.PerPage()
			if err != nil {
				t.Fatal(err)
			}
			paths := make([]string, tt.items)
			for i := range paths {
				paths[i] = fixtures[i%len(fixtures)]
			}
			pdf := render(t, paths, opts)
			want := (tt.items + perPage - 1) / perPage
			if n := len(pageObject.FindAll(pdf, -1)); n != want {
				t.Fatalf("PDF has %d pages, want %d", n, want)
			}
			streams := pageStreamsEach(pdf)
			if len(streams) != want {
				t.Fatalf("%d page streams, want %d", len(streams), want)
			}
			for i, s := range streams {
				items := min(perPage, tt.items-i*perPage)
				if n := strings.Count(s, " Do Q"); n != items {
					t.Errorf("page %d: %d images, want %d", i+1, n, items)
				}
				// One cut border per frame, and no other boxes or lines.
				if n := strings.Count(s, " re S"); n != items {
					t.Errorf("page %d: %d boxes, want %d frames", i+1, n, items)
				}
				if strings.Contains(s, " l S") {
					t.Errorf("page %d draws lines", i+1)
				}
			}
		})
	}
}

// Images, frames and captions are drawn where the layout puts them, the
// frame border in the cut border's grey.
func TestBuildPolaroidDrawsTheLayout(t *testing.T) {
	fixtures := polaroidFixtures(t)
	fixtureRatios := []float64{0.66, 1, 1.5, 0.66, 0.3, 0.66}
	for _, s := range Sizes {
		t.Run(s.ID, func(t *testing.T) {
			g, err := NewPolaroidGrid(A4, s.CoverHeight)
			if err != nil {
				t.Fatal(err)
			}
			// One page: as many fixtures as fit (the long title among them).
			n := min(len(fixtures), g.PerPage())
			paths, ratios := fixtures[:n], fixtureRatios[:n]
			got := pageStreams(render(t, paths, Options{Format: "polaroid", Paper: "a4", Size: s.ID}))
			cells, err := g.Cells(len(paths))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, "0.600 G\n") || !strings.Contains(got, "0.57 w\n") {
				t.Errorf("frames are not drawn in the cut border's grey 0.2 mm line:\n%s", got)
			}
			const k = 72 / 25.4
			texts := 0
			for i, r := range ratios {
				p := layoutPolaroid(cells[0][i], r)
				b := p.Cover
				img := fmt.Sprintf("q %.5f 0 0 %.5f %.5f %.5f cm /I Do Q", b.W*k, b.H*k, b.X*k, (A4.Height-b.Y-b.H)*k)
				if !strings.Contains(got, img) {
					t.Errorf("no image at %q", img)
				}
				f := p.Frame
				frame := fmt.Sprintf("%.2f %.2f %.2f %.2f re S", f.X*k, (A4.Height-f.Y)*k, f.W*k, -f.H*k)
				if !strings.Contains(got, frame) {
					t.Errorf("no frame %q", frame)
				}
				// A line or two of title, but none for an empty one.
				title, _ := ParseName(paths[i])
				if title != "" {
					texts++
				}
				if strings.Contains(title, "Real Academia") {
					texts++
				}
			}
			if n := strings.Count(got, " Tj"); n != texts {
				t.Errorf("%d lines of text, want %d", n, texts)
			}
			// Every line starts inside a caption.
			for _, m := range textAt.FindAllStringSubmatch(got, -1) {
				var x, y float64
				fmt.Sscan(m[1], &x)
				fmt.Sscan(m[2], &y)
				x, y = x/k, A4.Height-y/k
				in := false
				for i, r := range ratios {
					c := layoutPolaroid(cells[0][i], r).Caption
					if x >= c.X-0.01 && x <= c.X+c.W && y >= c.Y && y <= c.Y+c.H {
						in = true
					}
				}
				if !in {
					t.Errorf("text at (%v, %v) mm, outside every caption", x, y)
				}
			}
		})
	}
}

// The cut mode and the header do not apply to polaroids.
func TestBuildPolaroidIgnoresCutAndHeader(t *testing.T) {
	paths := cutFixtures(t)
	want := pageStreams(render(t, paths, Options{Format: "polaroid"}))
	for _, c := range CutModes {
		for _, h := range []string{"", "Books of 2026"} {
			opts := Options{Format: "polaroid", Cut: c.ID, Header: h}
			if got := pageStreams(render(t, paths, opts)); got != want {
				t.Errorf("%+v: page content differs from the default", opts)
			}
		}
	}
}
