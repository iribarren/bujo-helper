package sheet

import (
	"fmt"
	"math"

	"github.com/go-pdf/fpdf"
)

// DotGridKind is the ID of the dot grid page kind: a plain grid of small
// dots to write and draw on, the basic bullet journal page.
const DotGridKind = "dotgrid"

// Dot grid settings: lengths in millimetres.
const (
	// DefaultDotSpacing is the distance between dot centres used when none
	// is chosen.
	DefaultDotSpacing = 5
	// DotGridTitleHeight is the band at the top of the space between the
	// margins that holds the title, when there is one; the dots start below
	// it.
	DotGridTitleHeight = FullPageHeaderHeight
	dotRadius          = 0.25
	dotGrey            = 0x80 // dot colour, #808080
)

// DotSpacings are the distances between dot centres to choose from.
var DotSpacings = []int{4, DefaultDotSpacing, 6}

// DotGridOptions are the options of a dot grid page.
type DotGridOptions struct {
	Spacing int `json:"spacing"` // mm between dot centres, one of DotSpacings
}

var defaultDotGrid = DotGridOptions{Spacing: DefaultDotSpacing}

func (o DotGridOptions) validate() error {
	for _, s := range DotSpacings {
		if o.Spacing == s {
			return nil
		}
	}
	return fmt.Errorf("dot spacing %d mm: want one of %v", o.Spacing, DotSpacings)
}

func (o DotGridOptions) pages() int { return 1 }

// draw draws one page: the title, if any, centred in its band at the top
// in the lettering l, and the dots below it.
func (o DotGridOptions) draw(pdf *fpdf.Fpdf, page Page, title string, l lettering) error {
	pdf.AddPage()
	if title != "" {
		band := dotGridTitleBand(page)
		drawHeader(pdf, l, band, fitHeader(pdf, l, title, band.W))
	}
	pdf.SetFillColor(dotGrey, dotGrey, dotGrey)
	for _, d := range newDotGrid(page, float64(o.Spacing), title != "").Dots() {
		pdf.Circle(d.X, d.Y, dotRadius, "F")
	}
	return pdf.Error()
}

// dotGridTitleBand is the band of a dot grid's title: across the page
// between the side margins, at the top margin.
func dotGridTitleBand(page Page) Box {
	return Box{X: MinMargin, Y: MinMargin, W: page.Width - 2*MinMargin, H: DotGridTitleHeight}
}

// dotGrid is the layout of a dot grid page: Columns × Rows dots, Pitch
// apart both ways, the first one's centre at (X, Y).
type dotGrid struct {
	X, Y          float64
	Pitch         float64
	Columns, Rows int
}

// Point is a position on the page, in millimetres.
type Point struct {
	X, Y float64
}

// newDotGrid lays out dots pitch apart on page: as many as fit, a whole
// number of pitches apart, within MinMargin margins and below the title
// band if title is set, centred in that space both ways.
func newDotGrid(page Page, pitch float64, title bool) dotGrid {
	top := MinMargin
	if title {
		top += DotGridTitleHeight
	}
	width := page.Width - 2*MinMargin
	height := page.Height - MinMargin - top
	// A small tolerance keeps an exact fit from being lost to rounding.
	steps := func(length float64) int { return int(math.Floor(length/pitch + 1e-9)) }
	nx, ny := steps(width), steps(height)
	return dotGrid{
		X:       MinMargin + (width-float64(nx)*pitch)/2,
		Y:       top + (height-float64(ny)*pitch)/2,
		Pitch:   pitch,
		Columns: nx + 1,
		Rows:    ny + 1,
	}
}

// Dots returns the dot centres, row by row from the top left.
func (g dotGrid) Dots() []Point {
	dots := make([]Point, 0, g.Columns*g.Rows)
	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Columns; c++ {
			dots = append(dots, Point{X: g.X + float64(c)*g.Pitch, Y: g.Y + float64(r)*g.Pitch})
		}
	}
	return dots
}
