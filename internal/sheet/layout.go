// Package sheet lays out cover images on pages for printing and writes them
// to a PDF, in one of several formats: a sheet of covers, review cards, or book spines.
//
// On a cover sheet, covers have a fixed height chosen by a size preset (50 mm by default) and
// keep their aspect ratio, in a centred grid derived from the cover height
// and the paper size (A4 by default). A cover too wide for its column is shrunk to the column
// width. All sizes are in millimetres.
package sheet

import (
	"errors"
	"fmt"
	"math"
	"strings"
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

// Standard paper sizes.
var (
	A4     = Page{Width: 210, Height: 297}
	Letter = Page{Width: 215.9, Height: 279.4} // US Letter, 8.5 × 11 in
	A5     = Page{Width: 148, Height: 210}
)

// Paper is a paper size preset.
type Paper struct {
	ID      string
	Label   string
	FileTag string // short, file-name-safe name, e.g. for "covers_A4_Mini.pdf"
	Page    Page
}

// DefaultPaper is the ID of the paper used when none is chosen.
const DefaultPaper = "a4"

// Papers lists the paper size presets, the default first.
var Papers = []Paper{
	{ID: DefaultPaper, Label: "A4", FileTag: "A4", Page: A4},
	{ID: "letter", Label: "US Letter", FileTag: "Letter", Page: Letter},
	{ID: "a5", Label: "A5", FileTag: "A5", Page: A5},
}

// PaperByID returns the paper with the given ID; "" means DefaultPaper.
func PaperByID(id string) (Paper, error) {
	if id == "" {
		id = DefaultPaper
	}
	for _, p := range Papers {
		if p.ID == id {
			return p, nil
		}
	}
	return Paper{}, fmt.Errorf("unknown paper size %q", id)
}

// Size is a cover size preset.
type Size struct {
	ID          string
	Label       string
	FileTag     string  // short, file-name-safe name, e.g. for "covers_A4_Mini.pdf"
	CoverHeight float64 // mm
}

// DefaultSize is the ID of the preset used when none is chosen.
const DefaultSize = "standard"

// Sizes lists the cover size presets, smallest first.
var Sizes = []Size{
	{ID: "mini", Label: "Mini", FileTag: "Mini", CoverHeight: 25},
	{ID: DefaultSize, Label: "Standard", FileTag: "Standard", CoverHeight: 50},
	{ID: "large", Label: "Large", FileTag: "Large", CoverHeight: 80},
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

// Format is a print format: what is drawn for each image.
type Format struct {
	ID       string
	Label    string
	FileTag  string // short, file-name-safe name, e.g. for "cards_A4.pdf"
	UsesSize bool   // whether the cover size preset applies
}

// DefaultFormat is the ID of the format used when none is chosen.
const DefaultFormat = "covers"

// IDs of the formats other than the cover sheet.
const (
	cardsFormat  = "cards"
	spinesFormat = "spines"
)

// Formats lists the print formats, the default first.
var Formats = []Format{
	{ID: DefaultFormat, Label: "Cover sheet", FileTag: "covers", UsesSize: true},
	{ID: cardsFormat, Label: "Review cards", FileTag: "cards", UsesSize: false},
	{ID: spinesFormat, Label: "Spines", FileTag: "spines", UsesSize: true},
}

// FormatByID returns the format with the given ID; "" means DefaultFormat.
func FormatByID(id string) (Format, error) {
	if id == "" {
		id = DefaultFormat
	}
	for _, f := range Formats {
		if f.ID == id {
			return f, nil
		}
	}
	return Format{}, fmt.Errorf("unknown print format %q", id)
}

// Options are the print settings of a sheet. The zero value gives the
// default sheet: standard covers on A4.
type Options struct {
	Format string `json:"format"` // a Format ID; "" means DefaultFormat
	Size   string `json:"size"`   // a Size ID; "" means DefaultSize. Ignored by formats that do not use it.
	Paper  string `json:"paper"`  // a Paper ID; "" means DefaultPaper
	// DateLabel labels the date field of review cards, e.g. "Read on";
	// "" means "Date".
	DateLabel string `json:"dateLabel"`
}

// Grid returns the grid the options describe: the cover grid of the size
// preset for a cover sheet, the card grid for review cards, the spine grid
// of the size preset (its cover height) for spines.
func (o Options) Grid() (Grid, error) {
	f, err := FormatByID(o.Format)
	if err != nil {
		return Grid{}, err
	}
	if f.ID == cardsFormat {
		p, err := PaperByID(o.Paper)
		if err != nil {
			return Grid{}, err
		}
		return NewCardGrid(p.Page)
	}
	s, err := SizeByID(o.Size)
	if err != nil {
		return Grid{}, err
	}
	p, err := PaperByID(o.Paper)
	if err != nil {
		return Grid{}, err
	}
	if f.ID == spinesFormat {
		return NewSpineGrid(p.Page, s.CoverHeight)
	}
	return NewGrid(p.Page, s.CoverHeight)
}

// PerPage is the number of covers, cards or spines on a full page.
func (o Options) PerPage() (int, error) {
	g, err := o.Grid()
	if err != nil {
		return 0, err
	}
	return g.PerPage(), nil
}

// dateLabel is the text of the date field of a review card, with its colon.
func (o Options) dateLabel() string {
	label := strings.TrimSpace(o.DateLabel)
	if label == "" {
		label = "Date"
	}
	return label + ":"
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
	// Spines makes the columns book spines: their widths follow the
	// spineWidths pattern, spineGap apart, and CellWidth is unused.
	Spines bool
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

// Cells returns the cells of count items on pages: the grid is filled left
// to right, top to bottom, PerPage per page; the last page may be partial.
func (g Grid) Cells(count int) ([][]Box, error) {
	if count <= 0 {
		return nil, errors.New("nothing to lay out")
	}
	perPage := g.PerPage()
	var pages [][]Box
	for i := range count {
		if i%perPage == 0 {
			pages = append(pages, make([]Box, 0, min(perPage, count-i)))
		}
		page := &pages[len(pages)-1]
		*page = append(*page, g.cell(i%perPage))
	}
	return pages, nil
}

// cell returns cell n of a page.
func (g Grid) cell(n int) Box {
	col, row := n%g.Columns, n/g.Columns
	y := g.MarginY + float64(row)*(g.CoverHeight+Gap)
	if g.Spines {
		return spineCell(g.MarginX, y, g.CoverHeight, col)
	}
	return Box{
		X: g.MarginX + float64(col)*(g.CellWidth+Gap),
		Y: y,
		W: g.CellWidth,
		H: g.CoverHeight,
	}
}

// place returns the box of a cover with aspect ratio r in cell n of a page.
func (g Grid) place(n int, r float64) Box {
	w, h := g.CoverHeight*r, g.CoverHeight
	if w > g.CellWidth {
		w, h = g.CellWidth, g.CellWidth/r
	}
	c := g.cell(n)
	return Box{
		X: c.X + (c.W-w)/2,
		Y: c.Y + (c.H-h)/2,
		W: w,
		H: h,
	}
}
