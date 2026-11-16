package sheet

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSpineGridPapersAndSizes(t *testing.T) {
	tests := []struct {
		paper, size string
		cols, rows  int
		perPage     int
		marginX     float64
	}{
		// A row of 16 spines is 194 mm (the first 16 widths, 15 gaps of 2 mm):
		// exactly the usable width of A4.
		{"a4", "mini", 16, 9, 144, 8},
		{"a4", "standard", 16, 5, 80, 8},
		{"a4", "large", 16, 3, 48, 8},
		// (215.9 - 194) / 2 = 10.95.
		{"letter", "mini", 16, 9, 144, 10.95},
		{"letter", "standard", 16, 4, 64, 10.95},
		{"letter", "large", 16, 3, 48, 10.95},
		// 11 spines are 132 mm: exactly the usable width of A5.
		{"a5", "mini", 11, 6, 66, 8},
		{"a5", "standard", 11, 3, 33, 8},
		{"a5", "large", 11, 2, 22, 8},
	}
	for _, tt := range tests {
		opts := Options{Format: "spines", Paper: tt.paper, Size: tt.size}
		g, err := opts.Grid()
		if err != nil {
			t.Fatalf("%s %s: %v", tt.paper, tt.size, err)
		}
		s, _ := SizeByID(tt.size)
		p, _ := PaperByID(tt.paper)
		if g.Page != p.Page || !g.Spines || !near(g.CoverHeight, s.CoverHeight) ||
			g.Columns != tt.cols || g.Rows != tt.rows || !near(g.MarginX, tt.marginX) {
			t.Errorf("%s %s: grid %+v, want %d x %d spines %v mm tall, margin %v",
				tt.paper, tt.size, g, tt.cols, tt.rows, s.CoverHeight, tt.marginX)
		}
		if n, err := opts.PerPage(); err != nil || n != tt.perPage || g.PerPage() != tt.perPage {
			t.Errorf("%s %s: PerPage %d, %v (grid %d); want %d", tt.paper, tt.size, n, err, g.PerPage(), tt.perPage)
		}
		// Full: no room for another spine in a row, or another row.
		if spineRowWidth(g.Columns+1) <= p.Page.Width-2*MinMargin {
			t.Errorf("%s %s: %d spines fit in a row, grid has %d", tt.paper, tt.size, g.Columns+1, g.Columns)
		}
		if float64(g.Rows+1)*g.CoverHeight+float64(g.Rows)*Gap <= p.Page.Height-2*MinMargin {
			t.Errorf("%s %s: %d rows fit, grid has %d", tt.paper, tt.size, g.Rows+1, g.Rows)
		}
	}
}

func TestSpineWidthPattern(t *testing.T) {
	seen := map[float64]bool{}
	for col := range 40 {
		w := spineWidth(col)
		if w < MinSpineWidth || w > MaxSpineWidth {
			t.Errorf("column %d: width %v, want %v–%v mm", col, w, MinSpineWidth, MaxSpineWidth)
		}
		if w != spineWidth(col+len(spineWidths)) {
			t.Errorf("column %d: the pattern does not repeat", col)
		}
		seen[w] = true
	}
	// Natural-looking: the widths vary.
	if len(seen) < 3 {
		t.Errorf("only %d different widths", len(seen))
	}
	if spineRowWidth(0) != 0 || spineRowWidth(1) != spineWidths[0] || !near(spineRowWidth(2), spineWidths[0]+spineGap+spineWidths[1]) {
		t.Errorf("spineRowWidth: 0 → %v, 1 → %v, 2 → %v", spineRowWidth(0), spineRowWidth(1), spineRowWidth(2))
	}
}

// Every spine of a full page lies within the margins, without overlap, in
// rows centred on the page; the spines of a row touch the same top and
// bottom, separated by spineGap.
func TestSpineCellsInsideThePageCentredWithoutOverlap(t *testing.T) {
	for _, p := range Papers {
		for _, s := range Sizes {
			g, err := Options{Format: "spines", Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			name := p.ID + " " + s.ID
			pages, err := g.Cells(g.PerPage() + 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 2 || len(pages[0]) != g.PerPage() || len(pages[1]) != 1 {
				t.Fatalf("%s: pages of %d, %d spines; want %d, 1", name, len(pages[0]), len(pages[len(pages)-1]), g.PerPage())
			}
			usable := Box{X: MinMargin, Y: MinMargin, W: p.Page.Width - 2*MinMargin, H: p.Page.Height - 2*MinMargin}
			boxes := pages[0]
			for i, b := range boxes {
				if !inside(b, usable) {
					t.Errorf("%s: spine %d %+v outside the margins", name, i, b)
				}
				if b.W < MinSpineWidth-eps || b.W > MaxSpineWidth+eps || !near(b.H, s.CoverHeight) {
					t.Errorf("%s: spine %d is %v x %v mm", name, i, b.W, b.H)
				}
				for j := range i {
					if overlap(b, boxes[j]) {
						t.Errorf("%s: spines %d and %d overlap", name, j, i)
					}
				}
			}
			for row := range g.Rows {
				first, last := boxes[row*g.Columns], boxes[(row+1)*g.Columns-1]
				left, right := first.X, p.Page.Width-(last.X+last.W)
				if !near(left, right) {
					t.Errorf("%s: row %d margins %v / %v, want centred", name, row, left, right)
				}
				for col := 1; col < g.Columns; col++ {
					a, b := boxes[row*g.Columns+col-1], boxes[row*g.Columns+col]
					if !near(b.X-(a.X+a.W), spineGap) || !near(a.Y, b.Y) {
						t.Errorf("%s: row %d, spines %d and %d: gap %v, tops %v / %v", name, row, col-1, col, b.X-(a.X+a.W), a.Y, b.Y)
					}
				}
				if row > 0 {
					above := boxes[(row-1)*g.Columns]
					if !near(first.Y-(above.Y+above.H), Gap) {
						t.Errorf("%s: rows %d and %d %v mm apart, want %v", name, row-1, row, first.Y-(above.Y+above.H), Gap)
					}
				}
			}
			top, bottom := boxes[0].Y, p.Page.Height-(boxes[len(boxes)-1].Y+boxes[len(boxes)-1].H)
			if !near(top, bottom) || top < MinMargin-eps {
				t.Errorf("%s: top / bottom margins %v / %v, want centred", name, top, bottom)
			}
			// A new page starts at the top-left spine again.
			if pages[1][0] != boxes[0] {
				t.Errorf("%s: second page starts at %+v, want %+v", name, pages[1][0], boxes[0])
			}
		}
	}
}

func TestNewSpineGridRejectsWhatDoesNotFit(t *testing.T) {
	if g, err := NewSpineGrid(Page{Width: 20, Height: 297}, 50); err == nil {
		t.Errorf("a 20 mm wide page: %+v, want an error", g)
	}
	if g, err := NewSpineGrid(A4, 300); err == nil {
		t.Errorf("300 mm spines on A4: %+v, want an error", g)
	}
	if g, err := NewSpineGrid(A4, 0); err == nil {
		t.Errorf("0 mm spines: %+v, want an error", g)
	}
}

// fakeMeasure measures text as if every rune were 0.5 em wide (bold 0.6 em).
func fakeMeasure(s string, bold bool, pt float64) float64 {
	em := 0.5
	if bold {
		em = 0.6
	}
	return float64(utf8.RuneCountInString(s)) * em * pt * mmPerPoint
}

func TestLayoutSpineTextStaysOnTheSpine(t *testing.T) {
	long := strings.Repeat("The Hitchhiker's Guide to the Galaxy ", 4)
	titles := []string{"It", "Dune", "Cien años de soledad", long, "Supercalifragilisticexpialidocious", ""}
	creators := []string{"", "Stephen King", "Gabriel García Márquez", long}
	for _, s := range Sizes {
		for _, w := range []float64{MinSpineWidth, 9, 10, 11, MaxSpineWidth} {
			spine := Box{X: 10, Y: 20, W: w, H: s.CoverHeight}
			for _, title := range titles {
				for _, creator := range creators {
					lines := layoutSpineText(spine, title, creator, fakeMeasure)
					checkSpineText(t, spine, title, creator, lines)
				}
			}
		}
	}
}

// checkSpineText checks the lines of text laid out on spine: inside it,
// along its length within the end padding, across it without overlap.
func checkSpineText(t *testing.T, spine Box, title, creator string, lines []spineLine) {
	t.Helper()
	name := spine.W
	if title != "" && (len(lines) == 0 || !lines[0].Bold) {
		t.Errorf("%v mm spine, %q: lines %+v, want the title first, bold", name, title, lines)
	}
	length := spine.H - 2*spineEndPadding
	across := 0.0
	for _, l := range lines {
		if l.Text == "" {
			t.Errorf("%v mm spine: empty line %+v", name, l)
		}
		if l.Size < minSpineFontSize-eps || l.Size > maxSpineFontSize+eps {
			t.Errorf("%v mm spine: %q at %v pt, want %v–%v pt", name, l.Text, l.Size, minSpineFontSize, maxSpineFontSize)
		}
		w := fakeMeasure(l.Text, l.Bold, l.Size)
		if w > length+eps {
			t.Errorf("%v x %v mm spine: %q is %v mm long, room for %v", spine.W, spine.H, l.Text, w, length)
		}
		// Offset is where the line starts along the spine, from its top;
		// Across is the top of its line box, from the right edge.
		if l.Offset < spineEndPadding-eps || l.Offset+w > spine.H-spineEndPadding+eps {
			t.Errorf("%v mm spine: %q from %v to %v mm", name, l.Text, l.Offset, l.Offset+w)
		}
		if l.Across < across-eps {
			t.Errorf("%v mm spine: %q overlaps the line before", name, l.Text)
		}
		across = l.Across + lineHeight(l.Size)
		if l.Across < -eps || across > spine.W+eps {
			t.Errorf("%v mm spine: %q from %v to %v mm across", name, l.Text, l.Across, across)
		}
	}
	if title != "" && creator != "" && len(lines) != 2 {
		t.Errorf("%v mm spine, %q / %q: %d lines, want title and creator", name, title, creator, len(lines))
	}
}

func TestLayoutSpineTextShrinksThenTruncates(t *testing.T) {
	spine := Box{W: 10, H: 50}
	short := layoutSpineText(spine, "Dune", "", fakeMeasure)
	if len(short) != 1 || short[0].Size != maxSpineFontSize || short[0].Text != "Dune" {
		t.Errorf("short title: %+v, want Dune at %v pt", short, maxSpineFontSize)
	}
	// Centred along the spine.
	if w := fakeMeasure("Dune", true, short[0].Size); !near(short[0].Offset, (50-w)/2) {
		t.Errorf("short title starts at %v mm, want centred", short[0].Offset)
	}

	medium := layoutSpineText(spine, "The Name of the Wind and the Fire", "", fakeMeasure)
	if len(medium) != 1 || medium[0].Text != "The Name of the Wind and the Fire" || medium[0].Size >= maxSpineFontSize || medium[0].Size < minSpineFontSize {
		t.Errorf("medium title: %+v, want it whole, shrunk", medium)
	}

	long := layoutSpineText(spine, strings.Repeat("Galaxy ", 20), "", fakeMeasure)
	if len(long) != 1 || long[0].Size != minSpineFontSize || !strings.HasSuffix(long[0].Text, ellipsis) {
		t.Errorf("long title: %+v, want it cut at %v pt", long, minSpineFontSize)
	}
}

// A spine too short for even "…" leaves the title out; the creator must not
// size itself from a title line that is not there.
func TestLayoutSpineTextTitleThatCannotFit(t *testing.T) {
	huge := func(string, bool, float64) float64 { return 1000 }
	if lines := layoutSpineText(Box{W: 12, H: 25}, "Dune", "Frank Herbert", huge); len(lines) != 0 {
		t.Errorf("got %+v, want no lines", lines)
	}
}

func TestLayoutSpineTextCreatorIsSmaller(t *testing.T) {
	for _, w := range []float64{MinSpineWidth, MaxSpineWidth} {
		lines := layoutSpineText(Box{W: w, H: 80}, "Dune", "Frank Herbert", fakeMeasure)
		if len(lines) != 2 || lines[1].Bold || lines[1].Text != "Frank Herbert" || lines[1].Size >= lines[0].Size {
			t.Errorf("%v mm: %+v, want the creator smaller under the title", w, lines)
		}
	}
	if lines := layoutSpineText(Box{W: 10, H: 50}, "", "", fakeMeasure); len(lines) != 0 {
		t.Errorf("no title: %+v, want no lines", lines)
	}
}
