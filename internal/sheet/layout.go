// Package sheet lays out cover images on A4 pages for printing and writes
// them to a PDF.
//
// Covers are 50 mm tall and keep their aspect ratio, in a centred grid of
// 5 columns by 5 rows (25 per page). A cover too wide for its column is
// shrunk to the column width. All sizes are in millimetres.
package sheet

import (
	"errors"
	"fmt"
	"math"
)

// Page and grid geometry, in millimetres.
const (
	PageWidth   = 210.0 // A4 portrait
	PageHeight  = 297.0
	CoverHeight = 50.0 // height of a cover that fits its column
	Columns     = 5
	Rows        = 5
	Gap         = 4.0 // space between cells, both ways
	MarginX     = 8.0 // left and right margin

	// CellWidth is the column width: (210 - 2·8 - 4·4) / 5 = 35.6.
	CellWidth = (PageWidth - 2*MarginX - (Columns-1)*Gap) / Columns
	// MarginY centres the rows vertically: (297 - 5·50 - 4·4) / 2 = 15.5.
	MarginY = (PageHeight - Rows*CoverHeight - (Rows-1)*Gap) / 2

	// PerPage is the number of covers on a full page.
	PerPage = Columns * Rows
)

// Box is where one cover is drawn on its page: top-left corner, width and
// height, in millimetres.
type Box struct {
	X, Y, W, H float64
}

// Layout places covers with the given aspect ratios (width / height) on
// pages. Covers fill the grid left to right, top to bottom, 25 per page;
// the last page may be partial. Each cover is centred in its cell.
func Layout(ratios []float64) ([][]Box, error) {
	if len(ratios) == 0 {
		return nil, errors.New("no covers to lay out")
	}
	var pages [][]Box
	for i, r := range ratios {
		if r <= 0 || math.IsNaN(r) || math.IsInf(r, 0) {
			return nil, fmt.Errorf("cover %d has an invalid aspect ratio %v", i+1, r)
		}
		if i%PerPage == 0 {
			pages = append(pages, make([]Box, 0, min(PerPage, len(ratios)-i)))
		}
		page := &pages[len(pages)-1]
		*page = append(*page, place(i%PerPage, r))
	}
	return pages, nil
}

// place returns the box of a cover with aspect ratio r in cell n of a page.
func place(n int, r float64) Box {
	w, h := CoverHeight*r, CoverHeight
	if w > CellWidth {
		w, h = CellWidth, CellWidth/r
	}
	col, row := n%Columns, n/Columns
	cellX := MarginX + float64(col)*(CellWidth+Gap)
	cellY := MarginY + float64(row)*(CoverHeight+Gap)
	return Box{
		X: cellX + (CellWidth-w)/2,
		Y: cellY + (CoverHeight-h)/2,
		W: w,
		H: h,
	}
}
