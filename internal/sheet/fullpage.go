package sheet

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"path/filepath"

	"github.com/go-pdf/fpdf"
)

// Full page settings: lengths in millimetres, font sizes in points.
const (
	FullPageMax          = 60   // most covers on a page; more go on further pages
	FullPageMaxCover     = 80.0 // tallest cover, so a few covers do not fill the page
	FullPageGap          = 3.0  // between covers, both ways
	FullPageHeaderHeight = 12.0 // the header band above the covers
	maxHeaderFontSize    = 20.0 // the header's size when it fits
	minHeaderFontSize    = 12.0 // smallest header size, below which it is cut
)

// NewFullPageGrid derives the grid of a full page of count covers on page,
// up to FullPageMax of them, below a FullPageHeaderHeight header band if
// header is set. The cells are 2:3, FullPageGap apart, within MinMargin
// margins, and as tall as possible up to FullPageMaxCover: every column
// count from 1 to the covers on the page is tried, with as many rows as it
// needs, and the tallest cells win; on a tie, the grid with fewer empty
// cells, then the one with more columns. The grid is centred across the
// page, and the header and grid together down it, the header right above
// the grid. Its Limit is the covers on the page, so more than FullPageMax
// covers go on further pages of the same grid. It returns an error for no
// covers, or if not even one fits.
func NewFullPageGrid(page Page, count int, header bool) (Grid, error) {
	if count <= 0 {
		return Grid{}, errors.New("no covers to lay out")
	}
	n := min(count, FullPageMax)
	usableW := page.Width - 2*MinMargin
	usableH := page.Height - 2*MinMargin
	band := 0.0
	if header {
		band = FullPageHeaderHeight
		usableH -= band
	}
	// A small tolerance keeps rounding from breaking a tie.
	const tol = 1e-9
	height, cols, rows := 0.0, 0, 0
	for c := 1; c <= n; c++ {
		r := (n + c - 1) / c
		h := min(
			(usableW-float64(c-1)*FullPageGap)/float64(c)/minCellRatio,
			(usableH-float64(r-1)*FullPageGap)/float64(r),
			FullPageMaxCover,
		)
		if h > height+tol || (h > height-tol && c*r <= cols*rows) {
			height, cols, rows = h, c, r
		}
	}
	if !(height > 0) {
		return Grid{}, fmt.Errorf("covers do not fit on a %v × %v mm page", page.Width, page.Height)
	}
	width := height * minCellRatio
	gridW := float64(cols)*width + float64(cols-1)*FullPageGap
	gridH := float64(rows)*height + float64(rows-1)*FullPageGap
	return Grid{
		Page:        page,
		CoverHeight: height,
		CellWidth:   width,
		Columns:     cols,
		Rows:        rows,
		MarginX:     (page.Width - gridW) / 2,
		MarginY:     (page.Height-band-gridH)/2 + band,
		Spacing:     FullPageGap,
		Limit:       n,
	}, nil
}

// fullPageHeaderBox is the header band of a full page on grid, across the
// usable width right above the grid; ok is false if there is no header.
func fullPageHeaderBox(g Grid, header bool) (b Box, ok bool) {
	if !header {
		return Box{}, false
	}
	return Box{
		X: MinMargin,
		Y: g.MarginY - FullPageHeaderHeight,
		W: g.Page.Width - 2*MinMargin,
		H: FullPageHeaderHeight,
	}, true
}

// headerLine is the header of a full page as printed.
type headerLine struct {
	Text string
	Size float64 // font size, pt
}

// layoutFullPageHeader fits text, bold and its runs of spaces collapsed, on
// one line no wider than width, as measured by measure: at
// maxHeaderFontSize if it fits, else shrunk to fit down to
// minHeaderFontSize, then cut with "…". Empty text, or a width too narrow
// for "…", gives no text.
func layoutFullPageHeader(text string, width float64, measure measureFunc) headerLine {
	text = Options{Header: text}.header()
	if text == "" {
		return headerLine{}
	}
	pt := maxHeaderFontSize
	if w := measure(text, true, pt); w > width+fitTolerance {
		pt = math.Floor(pt*width/w*10) / 10
	}
	if pt < minHeaderFontSize || measure(text, true, pt) > width+fitTolerance {
		pt = max(pt, minHeaderFontSize)
		lines := fitLines(text, width, 1, func(s string) float64 { return measure(s, true, pt) })
		if len(lines) == 0 {
			return headerLine{}
		}
		text = lines[0]
	}
	return headerLine{Text: text, Size: pt}
}

// buildFullPage draws full pages of covers on page: header, if not empty,
// centred on every page, and the covers below it in the grid
// NewFullPageGrid derives for their count, without borders.
func buildFullPage(page Page, paths []string, ratios []float64, header string) (*fpdf.Fpdf, error) {
	grid, err := NewFullPageGrid(page, len(paths), header != "")
	if err != nil {
		return nil, err
	}
	pages, err := grid.Layout(ratios)
	if err != nil {
		return nil, err
	}
	var boxes []Box
	for _, p := range pages {
		boxes = append(boxes, p...)
	}
	images, err := printImages(paths, boxes, false)
	if err != nil {
		return nil, err
	}

	pdf := newPDF(page)
	band, hasHeader := fullPageHeaderBox(grid, header != "")
	var line headerLine
	if hasHeader {
		if err := addTextFonts(pdf); err != nil {
			return nil, err
		}
		line = layoutFullPageHeader(header, band.W, func(text string, bold bool, pt float64) float64 {
			pdf.SetFont(textFont, fontStyle(bold), pt)
			return pdf.GetStringWidth(text)
		})
	}
	imgOpts := fpdf.ImageOptions{ImageType: "JPG"}

	i := 0
	for _, p := range pages {
		pdf.AddPage()
		if line.Text != "" {
			pdf.SetTextColor(titleGrey, titleGrey, titleGrey)
			pdf.SetFont(textFont, "B", line.Size)
			x := band.X + (band.W-pdf.GetStringWidth(line.Text))/2
			top := band.Y + (band.H-lineHeight(line.Size))/2
			pdf.Text(x, baseline(top, line.Size), line.Text)
		}
		for _, box := range p {
			name := fmt.Sprintf("cover-%d", i)
			pdf.RegisterImageOptionsReader(name, imgOpts, bytes.NewReader(images[i].jpeg))
			pdf.ImageOptions(name, box.X, box.Y, box.W, box.H, false, imgOpts, 0, "")
			if err := pdf.Error(); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}
	return pdf, nil
}
