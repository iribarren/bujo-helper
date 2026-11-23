package sheet

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"

	"github.com/go-pdf/fpdf"
)

// Checklist row sizes, in millimetres.
const (
	ChecklistRowHeight = 16.0 // each row, the rule under it included
	ChecklistBoxSize   = 5.0  // side of the checkbox
	checklistPadding   = 1.5  // above and below the thumbnail
	checklistGap       = 3.0  // between the checkbox, the thumbnail and the text
	checklistMinText   = 20.0 // narrowest text room a row may have
)

// Checklist text sizes, in points, and drawing settings.
const (
	checklistTitleFontSize   = 10.0
	checklistCreatorFontSize = 8.5
	checkboxGrey             = 0x55 // checkbox outline colour
	checkboxLineWidth        = 0.3  // checkbox outline width, mm
)

// NewChecklistGrid derives the rows of a checklist on page: one column
// across the width within MinMargin margins, ChecklistRowHeight rows that
// touch, from the top margin down, or from right below a
// FullPageHeaderHeight header band if header is set. As many rows fit as
// the height left allows (rounded down), and every page has the same rows.
// It returns an error if not even one row fits on the page.
func NewChecklistGrid(page Page, header bool) (Grid, error) {
	top := MinMargin
	if header {
		top += FullPageHeaderHeight
	}
	usableW := page.Width - 2*MinMargin
	room := page.Height - top - MinMargin
	// A small tolerance keeps an exact fit from being lost to rounding.
	const tol = 1e-9
	rows := int(math.Floor(room/ChecklistRowHeight + tol))
	textW := usableW - ChecklistBoxSize - 2*checklistGap - (ChecklistRowHeight-2*checklistPadding)*minCellRatio
	if rows < 1 || textW < checklistMinText-tol {
		return Grid{}, fmt.Errorf("a checklist row does not fit on a %v × %v mm page", page.Width, page.Height)
	}
	return Grid{
		Page:        page,
		CoverHeight: ChecklistRowHeight,
		CellWidth:   usableW,
		Columns:     1,
		Rows:        rows,
		MarginX:     MinMargin,
		MarginY:     top,
		Flush:       true,
	}, nil
}

// checklistRow are the boxes of the parts of one checklist row on its page,
// in mm. The text boxes are the room each line may use.
type checklistRow struct {
	Row     Box // the whole row; its rule runs along the bottom edge
	Check   Box // the checkbox
	Thumb   Box // the 2:3 thumbnail area
	Cover   Box // the image, shrunk to fit Thumb, centred in it
	Title   Box // one line of title
	Creator Box // one line of creator, right under the title; zero if none
}

// layoutChecklistRow places the parts of a checklist row in row, for an
// image with aspect ratio (width / height) ratio. From the left: the
// checkbox, vertically centred; the thumbnail area, 2:3 and as tall as the
// row less checklistPadding above and below, with the image shrunk to fit
// it, keeping its ratio, and centred; then the text up to the right edge:
// the title and, if hasCreator, the creator under it, centred down the row
// together.
func layoutChecklistRow(row Box, ratio float64, hasCreator bool) checklistRow {
	midY := row.Y + row.H/2
	c := checklistRow{
		Row:   row,
		Check: Box{X: row.X, Y: midY - ChecklistBoxSize/2, W: ChecklistBoxSize, H: ChecklistBoxSize},
	}
	thumbH := row.H - 2*checklistPadding
	c.Thumb = Box{X: c.Check.X + c.Check.W + checklistGap, Y: row.Y + checklistPadding, W: thumbH * minCellRatio, H: thumbH}
	w, h := c.Thumb.H*ratio, c.Thumb.H
	if w > c.Thumb.W {
		w, h = c.Thumb.W, c.Thumb.W/ratio
	}
	c.Cover = Box{X: c.Thumb.X + (c.Thumb.W-w)/2, Y: c.Thumb.Y + (c.Thumb.H-h)/2, W: w, H: h}

	textX := c.Thumb.X + c.Thumb.W + checklistGap
	textW := row.X + row.W - textX
	titleH := lineHeight(checklistTitleFontSize)
	if !hasCreator {
		c.Title = Box{X: textX, Y: midY - titleH/2, W: textW, H: titleH}
		return c
	}
	creatorH := lineHeight(checklistCreatorFontSize)
	top := midY - (titleH+creatorH)/2
	c.Title = Box{X: textX, Y: top, W: textW, H: titleH}
	c.Creator = Box{X: textX, Y: top + titleH, W: textW, H: creatorH}
	return c
}

// fitChecklistText fits title (bold) and creator (regular) each on one
// line no wider than width, as measured by measure, cut with "…" if too
// long. An empty part, or one too narrow for "…", gives "".
func fitChecklistText(title, creator string, width float64, measure measureFunc) (string, string) {
	line := func(text string, bold bool, pt float64) string {
		lines := fitLines(text, width, 1, func(s string) float64 { return measure(s, bold, pt) })
		if len(lines) == 0 {
			return ""
		}
		return lines[0]
	}
	return line(title, true, checklistTitleFontSize), line(creator, false, checklistCreatorFontSize)
}

// buildChecklist draws a checklist row per image on the rows of grid: the
// header, if not empty, on every page in the band above the rows in the
// lettering l, then per
// row a checkbox, the thumbnail, the title and creator (from the file name)
// and a rule under the row. There are no cut borders.
func buildChecklist(grid Grid, paths []string, ratios []float64, header string, l lettering) (*pdfDoc, error) {
	pages, err := grid.Cells(len(paths))
	if err != nil {
		return nil, err
	}
	var rows []checklistRow
	var covers []Box
	for _, page := range pages {
		for _, cell := range page {
			_, creator := ParseName(paths[len(rows)])
			r := layoutChecklistRow(cell, ratios[len(rows)], creator != "")
			rows = append(rows, r)
			covers = append(covers, r.Cover)
		}
	}
	images, err := printImages(paths, covers, false)
	if err != nil {
		return nil, err
	}

	pdf := newPDF(grid.Page)
	if err := addTextFonts(pdf); err != nil {
		return nil, err
	}
	band, hasHeader := fullPageHeaderBox(grid, header != "")
	var line headerLine
	if hasHeader {
		// Re-adding the default font is a no-op.
		if err := l.add(pdf); err != nil {
			return nil, err
		}
		line = fitHeader(pdf, l, header, band.W)
	}
	measure := pdfMeasure(pdf)
	imgOpts := fpdf.ImageOptions{ImageType: "JPG"}

	i := 0
	for _, page := range pages {
		pdf.AddPage()
		drawHeader(pdf, l, band, line)
		for range page {
			name := fmt.Sprintf("cover-%d", i)
			pdf.RegisterImageOptionsReader(name, imgOpts, bytes.NewReader(images[i].jpeg))
			title, creator := ParseName(paths[i])
			title, creator = fitChecklistText(title, creator, rows[i].Title.W, measure)
			drawChecklistRow(pdf, rows[i], name, imgOpts, title, creator)
			if err := pdf.Error(); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}
	return pdf, nil
}

// drawChecklistRow draws one checklist row with the registered thumbnail
// image and the title and creator lines, already fitted.
func drawChecklistRow(pdf *pdfDoc, r checklistRow, image string, imgOpts fpdf.ImageOptions, title, creator string) {
	pdf.SetDrawColor(checkboxGrey, checkboxGrey, checkboxGrey)
	pdf.SetLineWidth(checkboxLineWidth)
	pdf.Rect(r.Check.X, r.Check.Y, r.Check.W, r.Check.H, "D")

	pdf.ImageOptions(image, r.Cover.X, r.Cover.Y, r.Cover.W, r.Cover.H, false, imgOpts, 0, "")

	if title != "" {
		pdf.SetTextColor(titleGrey, titleGrey, titleGrey)
		pdf.SetFont(textFont, "B", checklistTitleFontSize)
		pdf.Text(r.Title.X, baseline(r.Title.Y, checklistTitleFontSize), title)
	}
	if creator != "" && r.Creator != (Box{}) {
		pdf.SetTextColor(textGrey, textGrey, textGrey)
		pdf.SetFont(textFont, "", checklistCreatorFontSize)
		pdf.Text(r.Creator.X, baseline(r.Creator.Y, checklistCreatorFontSize), creator)
	}

	bottom := r.Row.Y + r.Row.H
	pdf.SetDrawColor(writingGrey, writingGrey, writingGrey)
	pdf.SetLineWidth(ruleWidth)
	pdf.Line(r.Row.X, bottom, r.Row.X+r.Row.W, bottom)
}
