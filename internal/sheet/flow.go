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
// cover sheet (see Grid.Flow): CoverHeight (a square cell) for a square
// cover; CellWidth (the 2:3 column) for a cover that fits it at full
// height; and for any other cover, a wide cell as wide as the cover at full
// height, which is wider than the column. No cell is wider than the usable
// width; a cover too wide for it shrinks to fit.
func (g Grid) flowCellWidth(r float64) float64 {
	switch {
	case IsSquare(r):
		return min(g.CoverHeight, g.usableWidth())
	case g.Wide(r):
		return min(g.CoverHeight*r, g.usableWidth())
	}
	return g.CellWidth
}

// Wide reports whether a cover of aspect ratio r gets a wide cell on a
// cover sheet of this grid: it is not square, and at full height it is
// wider than the 2:3 column. 4:5 portraits and landscape images are wide;
// how wide a portrait must be depends on the grid's column.
func (g Grid) Wide(r float64) bool {
	return !IsSquare(r) && g.CoverHeight*r > g.CellWidth+flowTol
}

// inColumn reports whether a cover of aspect ratio r gets the 2:3 column
// on a cover sheet: it is neither square nor wide.
func (g Grid) inColumn(r float64) bool {
	return !IsSquare(r) && !g.Wide(r)
}

// usableWidth is the page width inside the side margins.
func (g Grid) usableWidth() float64 {
	return g.Page.Width - 2*g.MarginX
}

// Flow places covers with the given aspect ratios (width / height) on the
// pages of a cover sheet. Each cover gets a cell CoverHeight tall whose
// width follows its shape (see flowCellWidth): the 2:3 column, a square, or
// a wide cell for 4:5 portraits and landscape images. Cells flow left to
// right, gap apart, and a row wraps when the next cell does not fit the
// usable width; Rows rows fill a page, then a new page starts. Each row is
// centred as if padded with cells like its last one until no more fit, so
// a short row lines up with full rows of its kind. Each cover keeps its
// aspect ratio, shrunk to fit and centred in its cell: a panorama wider
// than the usable width is shorter than its row.
//
// Covers that fit the 2:3 column flow exactly as Layout places them: a row
// of 2:3 cells, padded, is the grid's row.
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
	if g.allInColumn(rs) {
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

// allInColumn reports whether every one of the aspect ratios rs gets the
// 2:3 column (see inColumn).
func (g Grid) allInColumn(rs []float64) bool {
	for _, r := range rs {
		if !g.inColumn(r) {
			return false
		}
	}
	return true
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
// the format of o: on a cover sheet with no crop mode, as Flow lays them
// out; otherwise PerPage a page. No images take no pages.
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
	if o.shapedCells(f) {
		pages, err := g.Flow(ratios)
		return len(pages), err
	}
	per, err := o.PerPage()
	if err != nil {
		return 0, err
	}
	return (len(ratios) + per - 1) / per, nil
}

// SquarePerPage is the number of square images on a full page: fewer than
// PerPage on a cover sheet, where they get square cells, and PerPage in
// any other format.
func (o Options) SquarePerPage() (int, error) {
	return o.PerPageFor(1)
}

// PerPageFor is the number of images of aspect ratio r on a full page: on
// a cover sheet with no crop mode, as Flow lays them out, so fewer when
// they get square or wide cells; otherwise PerPage.
func (o Options) PerPageFor(r float64) (int, error) {
	f, err := FormatByID(o.Format)
	if err != nil {
		return 0, err
	}
	g, err := o.Grid()
	if err != nil {
		return 0, err
	}
	if !o.shapedCells(f) {
		return o.PerPage()
	}
	per := g.PerPage()
	// A row never holds more cells than the grid has columns: a wide cell
	// is wider than the column, and NewGrid fits as many 2:3 columns as it
	// can, so a row of squares never holds more. One cover more than
	// PerPage therefore starts a second page.
	pages, err := g.Flow(sameRatios(per+1, r))
	if err != nil {
		return 0, err
	}
	return len(pages[0]), nil
}

// sameRatios returns n copies of the aspect ratio r.
func sameRatios(n int, r float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = r
	}
	return out
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

// shapedCells reports whether cells follow the shape of each image with
// the options o of format f: in a format with shaped cells, unless a crop
// mode gives every photo the same shape.
func (o Options) shapedCells(f Format) bool {
	crop, err := o.cropMode()
	return f.ShapedCells && err == nil && crop.ID == DefaultCrop
}
