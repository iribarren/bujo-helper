package sheet

import (
	"errors"
	"fmt"
	"math"
)

// squareMin is the narrowest aspect ratio (width / height) that counts as
// square; the widest is its inverse. Album art is square, give or take a
// few pixels, so about ±10 % of 1 is square.
const squareMin = 0.9

// IsSquare reports whether an image of aspect ratio r (width / height) is
// square: between squareMin and 1 / squareMin, both included.
func IsSquare(r float64) bool {
	return r >= squareMin && r <= 1/squareMin
}

// flowTol keeps an exact fit from being lost to rounding, as in NewGrid.
const flowTol = 1e-9

// flowCellWidth is the width of the cell of a cover of aspect ratio r on a
// cover sheet: CoverHeight (a square cell) for a square cover, never wider
// than the usable width, and CellWidth (the 2:3 column) for any other.
// Other cell shapes, such as landscape, would add a case here.
func (g Grid) flowCellWidth(r float64) float64 {
	if IsSquare(r) {
		return min(g.CoverHeight, g.usableWidth())
	}
	return g.CellWidth
}

// usableWidth is the page width inside the side margins.
func (g Grid) usableWidth() float64 {
	return g.Page.Width - 2*g.MarginX
}

// Flow places covers with the given aspect ratios (width / height) on the
// pages of a cover sheet. Each cover gets a cell CoverHeight tall whose
// width follows its shape (see flowCellWidth). Cells flow left to right,
// gap apart, and a row wraps when the next cell does not fit the usable
// width; Rows rows fill a page, then a new page starts. Each row is
// centred as if padded with cells like its last one until no more fit, so
// a short row lines up with full rows of its kind. Each cover keeps its
// aspect ratio, shrunk to fit and centred in its cell.
//
// Covers that are not square flow exactly as Layout places them: a row of
// 2:3 cells, padded, is the grid's row.
func (g Grid) Flow(ratios []float64) ([][]Box, error) {
	if err := checkRatios(ratios); err != nil {
		return nil, err
	}
	gap, usable := g.gap(), g.usableWidth()
	var pages [][]Box
	row := 0 // row index on the current page
	for start := 0; start < len(ratios); {
		// The row is ratios[start:end], width wide.
		end, width := start+1, g.flowCellWidth(ratios[start])
		for end < len(ratios) {
			next := width + gap + g.flowCellWidth(ratios[end])
			if next > usable+flowTol {
				break
			}
			end, width = end+1, next
		}
		if row == 0 || row == g.Rows {
			pages = append(pages, nil)
			row = 0
		}
		page := &pages[len(pages)-1]
		*page = append(*page, g.flowRow(row, ratios[start:end], width)...)
		start = end
		row++
	}
	return pages, nil
}

// flowRow returns the boxes of the covers of aspect ratios rs in row row of
// a page; width is the width of their cells and the gaps between them.
func (g Grid) flowRow(row int, rs []float64, width float64) []Box {
	boxes := make([]Box, 0, len(rs))
	if !hasSquare(rs) {
		// 2:3 cells only: the grid's own row, for the same arithmetic.
		for col, r := range rs {
			boxes = append(boxes, g.place(row*g.Columns+col, r))
		}
		return boxes
	}
	gap, usable := g.gap(), g.usableWidth()
	// Pad with cells like the last one until no more fit.
	last := g.flowCellWidth(rs[len(rs)-1])
	for width+gap+last <= usable+flowTol {
		width += gap + last
	}
	x := g.MarginX + (usable-width)/2
	y := g.MarginY + float64(row)*(g.CoverHeight+gap)
	for _, r := range rs {
		w := g.flowCellWidth(r)
		boxes = append(boxes, fit(Box{X: x, Y: y, W: w, H: g.CoverHeight}, r))
		x += w + gap
	}
	return boxes
}

// hasSquare reports whether any of the aspect ratios rs is square.
func hasSquare(rs []float64) bool {
	for _, r := range rs {
		if IsSquare(r) {
			return true
		}
	}
	return false
}

// checkRatios returns an error if there are no aspect ratios or one is not
// a positive, finite number.
func checkRatios(ratios []float64) error {
	if len(ratios) == 0 {
		return errors.New("no covers to lay out")
	}
	for i, r := range ratios {
		if r <= 0 || math.IsNaN(r) || math.IsInf(r, 0) {
			return fmt.Errorf("cover %d has an invalid aspect ratio %v", i+1, r)
		}
	}
	return nil
}

// Pages is the number of pages images of the given aspect ratios take in
// the format of o: on a cover sheet, as Flow lays them out; in any other
// format, PerPage a page. No images take no pages.
func (o Options) Pages(ratios []float64) (int, error) {
	f, err := FormatByID(o.Format)
	if err != nil {
		return 0, err
	}
	g, err := o.Grid()
	if err != nil {
		return 0, err
	}
	if len(ratios) == 0 {
		return 0, nil
	}
	if f.SquareCells {
		pages, err := g.Flow(ratios)
		return len(pages), err
	}
	per := g.PerPage()
	return (len(ratios) + per - 1) / per, nil
}

// SquarePerPage is the number of square images on a full page: fewer than
// PerPage on a cover sheet, where they get square cells, and PerPage in
// any other format.
func (o Options) SquarePerPage() (int, error) {
	f, err := FormatByID(o.Format)
	if err != nil {
		return 0, err
	}
	g, err := o.Grid()
	if err != nil {
		return 0, err
	}
	per := g.PerPage()
	if !f.SquareCells {
		return per, nil
	}
	// A row never holds more squares than 2:3 covers (NewGrid fits as many
	// 2:3 columns as it can), so a page holds at most PerPage squares.
	square := make([]float64, per)
	for i := range square {
		square[i] = 1
	}
	pages, err := g.Flow(square)
	if err != nil {
		return 0, err
	}
	return len(pages[0]), nil
}

// AspectRatios reads the image header of each path and returns its width /
// height. It returns an error naming the first file that cannot be read.
func AspectRatios(paths []string) ([]float64, error) {
	ratios := make([]float64, len(paths))
	for i, p := range paths {
		r, err := aspectRatio(p)
		if err != nil {
			return nil, err
		}
		ratios[i] = r
	}
	return ratios, nil
}
