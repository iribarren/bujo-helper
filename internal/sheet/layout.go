// Package sheet lays out cover images on pages for printing and writes them
// to a PDF.
//
// Covers have a fixed height chosen by a size preset (50 mm by default) and
// keep their aspect ratio, in a centred grid derived from the cover height
// and the page size. A cover too wide for its column is shrunk to the column
// width. All sizes are in millimetres.
package sheet

import (
	"errors"
	"fmt"
	"math"
)

// Grid spacing, in millimetres.
const (
	Gap       = 4.0 // space between cells, both ways
	MinMargin = 8.0 // smallest margin on each side of the page
)

// minCellRatio is the narrowest cell, as width / height: 2:3, the shape of
// a typical cover. It sets how many columns fit on a page.
const minCellRatio = 2.0 / 3.0

// Page is a paper size, portrait, in millimetres.
type Page struct {
	Width, Height float64
}

// A4 is the paper size of every sheet.
var A4 = Page{Width: 210, Height: 297}

// Size is a cover size preset.
type Size struct {
	ID          string
	Label       string
	CoverHeight float64 // mm
}

// DefaultSize is the ID of the preset used when none is chosen.
const DefaultSize = "standard"

// Sizes lists the cover size presets, smallest first.
var Sizes = []Size{
	{ID: "mini", Label: "Mini", CoverHeight: 25},
	{ID: DefaultSize, Label: "Standard", CoverHeight: 50},
	{ID: "large", Label: "Large", CoverHeight: 80},
}

// SizeByID returns the preset with the given ID; "" means DefaultSize.
func SizeByID(id string) (Size, error) {
	if id == "" {
		id = DefaultSize
	}
	for _, s := range Sizes {
		if s.ID == id {
			return s, nil
		}
	}
	return Size{}, fmt.Errorf("unknown cover size %q", id)
}

// Options are the print settings of a sheet. The zero value gives the
// default sheet: standard covers on A4.
type Options struct {
	Size string `json:"size"` // a Size ID; "" means DefaultSize
}

// Grid returns the grid the options describe.
func (o Options) Grid() (Grid, error) {
	s, err := SizeByID(o.Size)
	if err != nil {
		return Grid{}, err
	}
	return NewGrid(A4, s.CoverHeight)
}

// Grid is the cell layout of a page.
type Grid struct {
	Page        Page
	CoverHeight float64 // cell height: the height of a cover that fits its column
	CellWidth   float64 // column width
	Columns     int
	Rows        int
	MarginX     float64 // left and right margin
	MarginY     float64 // top and bottom margin
}

// NewGrid derives the grid of covers coverHeight tall on page. As many
// columns fit as keep every cell at least 2:3, within MinMargin side margins
// and Gap between cells; the columns then widen to fill the width. As many
// rows fit as the height allows within MinMargin, and the rows are centred
// vertically. It returns an error if not even one cell fits on the page.
func NewGrid(page Page, coverHeight float64) (Grid, error) {
	usableW := page.Width - 2*MinMargin
	usableH := page.Height - 2*MinMargin
	// A small tolerance keeps an exact fit from being lost to rounding.
	const tol = 1e-9
	if !(coverHeight > 0) || coverHeight > usableH+tol || coverHeight*minCellRatio > usableW+tol {
		return Grid{}, fmt.Errorf("a %v mm cover does not fit on a %v × %v mm page", coverHeight, page.Width, page.Height)
	}
	cols := int(math.Floor((usableW+Gap)/(coverHeight*minCellRatio+Gap) + tol))
	rows := int(math.Floor((usableH+Gap)/(coverHeight+Gap) + tol))
	return Grid{
		Page:        page,
		CoverHeight: coverHeight,
		CellWidth:   (usableW - float64(cols-1)*Gap) / float64(cols),
		Columns:     cols,
		Rows:        rows,
		MarginX:     MinMargin,
		MarginY:     (page.Height - float64(rows)*coverHeight - float64(rows-1)*Gap) / 2,
	}, nil
}

// PerPage is the number of covers on a full page.
func (g Grid) PerPage() int {
	return g.Columns * g.Rows
}

// Box is where one cover is drawn on its page: top-left corner, width and
// height, in millimetres.
type Box struct {
	X, Y, W, H float64
}

// Layout places covers with the given aspect ratios (width / height) on
// pages. Covers fill the grid left to right, top to bottom, PerPage per
// page; the last page may be partial. Each cover is centred in its cell.
func (g Grid) Layout(ratios []float64) ([][]Box, error) {
	if len(ratios) == 0 {
		return nil, errors.New("no covers to lay out")
	}
	perPage := g.PerPage()
	var pages [][]Box
	for i, r := range ratios {
		if r <= 0 || math.IsNaN(r) || math.IsInf(r, 0) {
			return nil, fmt.Errorf("cover %d has an invalid aspect ratio %v", i+1, r)
		}
		if i%perPage == 0 {
			pages = append(pages, make([]Box, 0, min(perPage, len(ratios)-i)))
		}
		page := &pages[len(pages)-1]
		*page = append(*page, g.place(i%perPage, r))
	}
	return pages, nil
}

// place returns the box of a cover with aspect ratio r in cell n of a page.
func (g Grid) place(n int, r float64) Box {
	w, h := g.CoverHeight*r, g.CoverHeight
	if w > g.CellWidth {
		w, h = g.CellWidth, g.CellWidth/r
	}
	col, row := n%g.Columns, n/g.Columns
	cellX := g.MarginX + float64(col)*(g.CellWidth+Gap)
	cellY := g.MarginY + float64(row)*(g.CoverHeight+Gap)
	return Box{
		X: cellX + (g.CellWidth-w)/2,
		Y: cellY + (g.CoverHeight-h)/2,
		W: w,
		H: h,
	}
}
