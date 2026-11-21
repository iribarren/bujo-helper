package sheet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// On every paper, spacing and with or without a title, the dots are a
// whole number of pitches apart, the first and last dots stay inside the
// margins, no further row or column would fit, and the grid is centred in
// the space between the margins (below the title band, if any).
func TestDotGridLayoutEveryPaper(t *testing.T) {
	for _, p := range Papers {
		for _, spacing := range DotSpacings {
			for _, title := range []bool{false, true} {
				name := fmt.Sprintf("%s/%dmm/title=%v", p.ID, spacing, title)
				t.Run(name, func(t *testing.T) {
					g := newDotGrid(p.Page, float64(spacing), title)
					pitch := float64(spacing)
					if g.Pitch != pitch {
						t.Fatalf("pitch %v, want %v", g.Pitch, pitch)
					}
					top := MinMargin
					if title {
						top += DotGridTitleHeight
					}
					left, right := MinMargin, p.Page.Width-MinMargin
					bottom := p.Page.Height - MinMargin

					lastX := g.X + float64(g.Columns-1)*pitch
					lastY := g.Y + float64(g.Rows-1)*pitch
					if g.X < left || lastX > right+1e-9 || g.Y < top || lastY > bottom+1e-9 {
						t.Errorf("dots %v..%v × %v..%v, want within %v..%v × %v..%v", g.X, lastX, g.Y, lastY, left, right, top, bottom)
					}
					// One more pitch would not fit.
					if lastX-g.X+pitch <= right-left+1e-9 || lastY-g.Y+pitch <= bottom-top+1e-9 {
						t.Errorf("%d × %d dots: another column or row fits", g.Columns, g.Rows)
					}
					// Centred: equal space on both sides, both ways.
					if !near(g.X-left, right-lastX) || !near(g.Y-top, bottom-lastY) {
						t.Errorf("not centred: x %v | %v, y %v | %v", g.X-left, right-lastX, g.Y-top, bottom-lastY)
					}
					dots := g.Dots()
					if len(dots) != g.Columns*g.Rows {
						t.Fatalf("%d dots, want %d × %d", len(dots), g.Columns, g.Rows)
					}
					if first, last := dots[0], dots[len(dots)-1]; !near(first.X, g.X) || !near(first.Y, g.Y) || !near(last.X, lastX) || !near(last.Y, lastY) {
						t.Errorf("dots run %v to %v, want (%v, %v) to (%v, %v)", first, last, g.X, g.Y, lastX, lastY)
					}
				})
			}
		}
	}
}

func TestDotGridLayoutA4(t *testing.T) {
	tests := []struct {
		spacing    int
		title      bool
		cols, rows int
		x, y       float64
	}{
		// 194 mm between the side margins: 38 pitches of 5 mm = 190, 2 mm each side.
		// 281 mm down: 56 pitches = 280, 0.5 mm top and bottom.
		{5, false, 39, 57, 10, 8.5},
		// A 12 mm title band leaves 269 mm: 53 pitches = 265, 2 mm each end.
		{5, true, 39, 54, 10, 22},
		{4, false, 49, 71, 9, 8.5},
		{6, false, 33, 47, 9, 10.5},
	}
	for _, tt := range tests {
		g := newDotGrid(A4, float64(tt.spacing), tt.title)
		if g.Columns != tt.cols || g.Rows != tt.rows || !near(g.X, tt.x) || !near(g.Y, tt.y) {
			t.Errorf("%d mm, title %v: %d × %d dots from (%v, %v), want %d × %d from (%v, %v)",
				tt.spacing, tt.title, g.Columns, g.Rows, g.X, g.Y, tt.cols, tt.rows, tt.x, tt.y)
		}
	}
}

// The title band takes the top of the space between the margins and
// pushes the grid down: fewer rows, same columns.
func TestDotGridTitleShiftsTheGrid(t *testing.T) {
	for _, p := range Papers {
		plain := newDotGrid(p.Page, 5, false)
		titled := newDotGrid(p.Page, 5, true)
		if titled.Columns != plain.Columns || titled.X != plain.X {
			t.Errorf("%s: the title changed the columns", p.ID)
		}
		if titled.Rows >= plain.Rows || titled.Y < MinMargin+DotGridTitleHeight {
			t.Errorf("%s: with a title %d rows from y %v, without %d from %v", p.ID, titled.Rows, titled.Y, plain.Rows, plain.Y)
		}
		band := dotGridTitleBand(p.Page)
		if band.X != MinMargin || band.Y != MinMargin || band.W != p.Page.Width-2*MinMargin || band.H != DotGridTitleHeight {
			t.Errorf("%s: title band %+v", p.ID, band)
		}
	}
}

// renderPage draws a page kind uncompressed, with fixed dates.
func renderPage(t *testing.T, kind string, page PageSettings, options string) []byte {
	t.Helper()
	pdf, err := buildPage(kind, page, json.RawMessage(options))
	if err != nil {
		t.Fatal(err)
	}
	pdf.SetCompression(false)
	// Fonts are written in map order unless sorted.
	pdf.SetCatalogSort(true)
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pdf.SetCreationDate(when)
	pdf.SetModificationDate(when)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fillOp is the end of a filled path: each dot is one.
var fillOp = regexp.MustCompile(`(?m) f$|^f$`)

// A dot grid PDF builds on every paper, one page of paper size with one
// filled circle per dot, and the title when there is one.
func TestBuildDotGridEveryPaper(t *testing.T) {
	for _, p := range Papers {
		for _, title := range []string{"", "Ideas 2026"} {
			t.Run(p.ID+"/"+title, func(t *testing.T) {
				var buf bytes.Buffer
				if err := BuildPage(&buf, DotGridKind, PageSettings{Paper: p.ID, Title: title}, nil); err != nil {
					t.Fatal(err)
				}
				if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
					t.Fatal("not a PDF")
				}
				out := renderPage(t, DotGridKind, PageSettings{Paper: p.ID, Title: title}, "")
				if n := bytes.Count(out, []byte("/Type /Page\n")); n != 1 {
					t.Errorf("%d pages, want 1", n)
				}
				box := fmt.Sprintf("/MediaBox [0 0 %.2f %.2f]", p.Page.Width/mmPerInch*72, p.Page.Height/mmPerInch*72)
				if !bytes.Contains(out, []byte(box)) {
					t.Errorf("no %s", box)
				}
				g := newDotGrid(p.Page, DefaultDotSpacing, title != "")
				stream := pageStreams(out)
				if n := len(fillOp.FindAllString(stream, -1)); n != g.Columns*g.Rows {
					t.Errorf("%d filled shapes, want %d dots", n, g.Columns*g.Rows)
				}
				if hasText := strings.Contains(stream, " Tj"); hasText != (title != "") {
					t.Errorf("text drawn: %v, want %v", hasText, title != "")
				}
			})
		}
	}
}

// A blank title is no title: no band, no text.
func TestBuildDotGridBlankTitle(t *testing.T) {
	blank := pageStreams(renderPage(t, DotGridKind, PageSettings{Title: "   "}, ""))
	none := pageStreams(renderPage(t, DotGridKind, PageSettings{}, ""))
	if blank != none {
		t.Error("a blank title changed the page")
	}
}

// The cut mode does not apply to a dot grid.
func TestBuildDotGridIgnoresTheCut(t *testing.T) {
	border := pageStreams(renderPage(t, DotGridKind, PageSettings{Cut: "border"}, ""))
	bleed := pageStreams(renderPage(t, DotGridKind, PageSettings{Cut: "bleed"}, ""))
	if border != bleed {
		t.Error("the cut mode changed a dot grid")
	}
}
