// Package sheet lays out cover images on pages for printing and writes them
// to a PDF, in one of several formats: a sheet of covers, review cards, book
// spines, movie ticket stubs, a full page of covers under a header, a
// checklist of rows to tick, each with a thumbnail, title and creator, or
// polaroid frames with the title as a caption.
//
// On a cover sheet, covers have a fixed height chosen by a size preset (50 mm by default) and
// keep their aspect ratio, in a centred grid derived from the cover height
// and the paper size (A4 by default). A square cover gets a square cell, a
// landscape cover a wide cell, and the cells flow in rows (see Grid.Flow);
// a cover too wide for its cell is shrunk to the cell width. A crop mode
// instead centre-crops photos to a standard print shape (see CropMode).
// All sizes are in millimetres.
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
	Noun     string // what one printed item is called, e.g. "card"
	UsesSize bool   // whether the cover size preset applies
	UsesCut  bool   // whether the cut mode applies; other formats keep their borders
	UsesCrop bool   // whether the crop mode applies
	// UsesHeader is whether the format prints the header the user types.
	UsesHeader bool
	// ShapedCells is whether cells follow the shape of the image: square
	// images get square cells and landscape images wide cells (see
	// Grid.Flow). In other formats they shrink to fit like any image.
	ShapedCells bool
}

// DefaultFormat is the ID of the format used when none is chosen.
const DefaultFormat = "covers"

// IDs of the formats other than the cover sheet.
const (
	cardsFormat  = "cards"
	spinesFormat = "spines"
	stubsFormat  = "stubs"
	fullFormat   = "fullpage"
	checkFormat  = "checklist"
	polaFormat   = "polaroid"
)

// Formats lists the print formats, the default first.
var Formats = []Format{
	{ID: DefaultFormat, Label: "Cover sheet", FileTag: "covers", Noun: "cover", UsesSize: true, UsesCut: true, UsesCrop: true, ShapedCells: true},
	{ID: cardsFormat, Label: "Review cards", FileTag: "cards", Noun: "card", UsesSize: false},
	{ID: spinesFormat, Label: "Spines", FileTag: "spines", Noun: "spine", UsesSize: true},
	{ID: stubsFormat, Label: "Ticket stubs", FileTag: "stubs", Noun: "stub", UsesSize: false},
	{ID: fullFormat, Label: "Full page", FileTag: "fullpage", Noun: "cover", UsesHeader: true},
	{ID: checkFormat, Label: "Checklist", FileTag: "checklist", Noun: "row", UsesHeader: true},
	{ID: polaFormat, Label: "Polaroid", FileTag: "polaroid", Noun: "polaroid", UsesSize: true},
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
	Cut    string `json:"cut"`    // a CutMode ID; "" means DefaultCut. Ignored by formats that do not use it.
	// Crop is a CropMode ID; "" means DefaultCrop (no cropping). Ignored by
	// formats that do not use it.
	Crop string `json:"crop"`
	// DateLabel labels the date field of review cards, e.g. "Read on";
	// "" means Labels.CardDate ("Date" by default).
	DateLabel string `json:"dateLabel"`
	// Header is printed above the covers of a full page or the rows of a
	// checklist, e.g. "Books of 2026" or "TBR 2026"; "" (or only spaces)
	// means no header. Ignored by other formats.
	Header string `json:"header"`
	// Credits adds credits pages after the images: one entry per selected
	// image with a credit sidecar (see media.CreditPath), in sheet order.
	// Without such an image it adds nothing.
	Credits bool `json:"credits"`
	// Labels are the fixed words printed on stubs, cards and credits
	// pages, in the UI language. The app fills them in; the zero value
	// prints English.
	Labels Labels `json:"-"`
}

// Labels are the fixed words printed on ticket stubs, review cards and
// credits pages. An empty (or blank) field keeps its English default.
type Labels struct {
	Admit    string // on the stub end; "ADMIT ONE"
	Date     string // the stub's date field, with its colon; "Date:"
	Where    string // the stub's where field, with its colon; "Where:"
	CardDate string // a card's date field when DateLabel is "", without the colon; "Date"
	Credits  string // the heading of credits pages; "Credits"
}

// labels returns o.Labels, trimmed, with every empty field set to its
// English default.
func (o Options) labels() Labels {
	or := func(s, def string) string {
		if s = strings.TrimSpace(s); s == "" {
			return def
		}
		return s
	}
	l := o.Labels
	return Labels{
		Admit:    or(l.Admit, admitText),
		Date:     or(l.Date, dateText),
		Where:    or(l.Where, whereText),
		CardDate: or(l.CardDate, cardDateText),
		Credits:  or(l.Credits, creditsText),
	}
}

// Grid returns the grid the options describe: the cover grid of the size
// preset for a cover sheet, the card grid for review cards, the stub grid
// for ticket stubs, the spine grid of the size preset (its cover height)
// for spines, the polaroid frame grid of the size preset for polaroids,
// for a cover sheet with a crop mode the grid of its printed pieces (see
// NewCropGrid), for a full page the grid of a page of FullPageMax covers,
// below a header band if there is a header, and for a checklist its rows,
// below the header band if there is a header (so PerPage counts the rows
// that fit with or without one). The cut mode never changes the grid, but
// an unknown one is an error; so is an unknown crop mode, in any format.
func (o Options) Grid() (Grid, error) {
	f, err := FormatByID(o.Format)
	if err != nil {
		return Grid{}, err
	}
	if _, err := CutModeByID(o.Cut); err != nil {
		return Grid{}, err
	}
	crop, err := o.cropMode()
	if err != nil {
		return Grid{}, err
	}
	// Cards and stubs have a fixed size: only the paper applies.
	fixed := func(newGrid func(Page) (Grid, error)) (Grid, error) {
		p, err := PaperByID(o.Paper)
		if err != nil {
			return Grid{}, err
		}
		return newGrid(p.Page)
	}
	switch f.ID {
	case cardsFormat:
		return fixed(NewCardGrid)
	case stubsFormat:
		return fixed(NewStubGrid)
	case fullFormat:
		return fixed(func(page Page) (Grid, error) {
			return NewFullPageGrid(page, FullPageMax, o.header() != "")
		})
	case checkFormat:
		return fixed(func(page Page) (Grid, error) {
			return NewChecklistGrid(page, o.header() != "")
		})
	}
	s, err := SizeByID(o.Size)
	if err != nil {
		return Grid{}, err
	}
	p, err := PaperByID(o.Paper)
	if err != nil {
		return Grid{}, err
	}
	switch f.ID {
	case spinesFormat:
		return NewSpineGrid(p.Page, s.CoverHeight)
	case polaFormat:
		return NewPolaroidGrid(p.Page, s.CoverHeight)
	}
	if crop.ID != DefaultCrop {
		return NewCropGrid(p.Page, crop, s.CoverHeight)
	}
	return NewGrid(p.Page, s.CoverHeight)
}

// cropMode is the crop mode that applies: that of Crop for a format that
// uses one, else DefaultCrop. An unknown Crop is an error in any format.
func (o Options) cropMode() (CropMode, error) {
	c, err := CropModeByID(o.Crop)
	if err != nil {
		return CropMode{}, err
	}
	f, err := FormatByID(o.Format)
	if err != nil {
		return CropMode{}, err
	}
	if !f.UsesCrop {
		return CropModeByID(DefaultCrop)
	}
	return c, nil
}

// PerPage is the number of covers, cards, spines, stubs, checklist rows,
// polaroids or cropped photos on a full page: for a photo strip, the
// strips times their frames.
func (o Options) PerPage() (int, error) {
	g, err := o.Grid()
	if err != nil {
		return 0, err
	}
	crop, err := o.cropMode()
	if err != nil {
		return 0, err
	}
	return g.PerPage() * crop.Frames, nil
}

// header is the header of a full page or checklist, its runs of spaces
// collapsed; "" if there is none.
func (o Options) header() string {
	return strings.Join(strings.Fields(o.Header), " ")
}

// dateLabel is the text of the date field of a review card, with its colon:
// DateLabel, or the card date label when it is empty.
func (o Options) dateLabel() string {
	label := strings.TrimSpace(o.DateLabel)
	if label == "" {
		label = o.labels().CardDate
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
	// Spacing is the space between cells, both ways; 0 means Gap.
	Spacing float64
	// Flush makes the cells touch, with no space between them; Spacing is
	// then unused.
	Flush bool
	// Limit caps the items on a page below Columns × Rows; 0 means no cap.
	Limit int
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

// PerPage is the number of covers on a full page: every cell, or Limit if
// that is fewer.
func (g Grid) PerPage() int {
	n := g.Columns * g.Rows
	if g.Limit > 0 {
		return min(n, g.Limit)
	}
	return n
}

// gap is the space between cells.
func (g Grid) gap() float64 {
	if g.Flush {
		return 0
	}
	if g.Spacing > 0 {
		return g.Spacing
	}
	return Gap
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
	if err := checkRatios(ratios); err != nil {
		return nil, err
	}
	perPage := g.PerPage()
	var pages [][]Box
	for i, r := range ratios {
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
	y := g.MarginY + float64(row)*(g.CoverHeight+g.gap())
	if g.Spines {
		return spineCell(g.MarginX, y, g.CoverHeight, col)
	}
	return Box{
		X: g.MarginX + float64(col)*(g.CellWidth+g.gap()),
		Y: y,
		W: g.CellWidth,
		H: g.CoverHeight,
	}
}

// place returns the box of a cover with aspect ratio r in cell n of a page.
func (g Grid) place(n int, r float64) Box {
	return fit(g.cell(n), r)
}

// fit returns the box of a cover with aspect ratio r in cell c: as tall as
// the cell, or shrunk to its width if that is too wide, and centred.
func fit(c Box, r float64) Box {
	w, h := c.H*r, c.H
	if w > c.W {
		w, h = c.W, c.W/r
	}
	return Box{
		X: c.X + (c.W-w)/2,
		Y: c.Y + (c.H-h)/2,
		W: w,
		H: h,
	}
}
