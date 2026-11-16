package sheet

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/go-pdf/fpdf"
)

// Ticket stub text and drawing settings.
const (
	admitText = "ADMIT ONE"
	dateText  = "Date:"
	whereText = "Where:"
	perfDash  = 1.0 // length of a perforation dash, mm
	perfSpace = 0.8 // space between perforation dashes, mm
)

// buildStubs draws a ticket stub per image on grid: the notched cut
// outline, the dashed perforation, the poster, its title and creator (from
// the file name), the date and where fields, empty stars and "ADMIT ONE" on
// the stub end.
func buildStubs(grid Grid, paths []string, ratios []float64) (*fpdf.Fpdf, error) {
	pages, err := grid.Cells(len(paths))
	if err != nil {
		return nil, err
	}
	var stubs []stubParts
	var covers []Box
	for _, page := range pages {
		for _, cell := range page {
			s := layoutStub(cell, ratios[len(stubs)])
			stubs = append(stubs, s)
			covers = append(covers, s.Cover)
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
	imgOpts := fpdf.ImageOptions{ImageType: "JPG"}

	i := 0
	for _, page := range pages {
		pdf.AddPage()
		for range page {
			name := fmt.Sprintf("cover-%d", i)
			pdf.RegisterImageOptionsReader(name, imgOpts, bytes.NewReader(images[i].jpeg))
			title, creator := ParseName(paths[i])
			drawStub(pdf, stubs[i], name, imgOpts, title, creator)
			if err := pdf.Error(); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}
	return pdf, nil
}

// drawStub draws one ticket stub with the registered poster image.
func drawStub(pdf *fpdf.Fpdf, s stubParts, image string, imgOpts fpdf.ImageOptions, title, creator string) {
	pdf.SetDrawColor(borderGrey, borderGrey, borderGrey)
	pdf.SetLineWidth(borderWidth)
	drawStubOutline(pdf, s)
	p := s.Perforation
	pdf.SetDashPattern([]float64{perfDash, perfSpace}, 0)
	pdf.Line(p.X, p.Y, p.X, p.Y+p.H)
	pdf.SetDashPattern([]float64{}, 0)

	pdf.ImageOptions(image, s.Cover.X, s.Cover.Y, s.Cover.W, s.Cover.H, false, imgOpts, 0, "")
	drawTitleAndCreator(pdf, s.Title, s.Creator, title, creator)

	// Field labels, each with a rule to the right edge to write on; the
	// rules start together, after the longer label.
	pdf.SetFont(textFont, "", labelFontSize)
	ruleX := s.Date.X + max(pdf.GetStringWidth(dateText), pdf.GetStringWidth(whereText)) + labelRuleGap
	for _, f := range []struct {
		box   Box
		label string
	}{{s.Date, dateText}, {s.Where, whereText}} {
		base := baseline(f.box.Y, labelFontSize)
		pdf.Text(f.box.X, base, f.label)
		pdf.SetDrawColor(writingGrey, writingGrey, writingGrey)
		pdf.SetLineWidth(ruleWidth)
		pdf.Line(ruleX, base, f.box.X+f.box.W, base)
	}

	pdf.SetDrawColor(starGrey, starGrey, starGrey)
	pdf.SetLineWidth(starLineWidth)
	for _, st := range s.Stars {
		pdf.Polygon(starPoints(st), "D")
	}

	// "ADMIT ONE", turned a quarter turn anticlockwise about the centre of
	// its box to read bottom to top, centred along the stub end.
	a := s.Admit
	cx, cy := a.X+a.W/2, a.Y+a.H/2
	pdf.SetTextColor(textGrey, textGrey, textGrey)
	pdf.SetFont(textFont, "B", admitFontSize)
	pdf.TransformBegin()
	pdf.TransformRotate(90, cx, cy)
	for _, line := range fitLines(admitText, a.H, 1, pdf.GetStringWidth) {
		pdf.Text(cx-pdf.GetStringWidth(line)/2, baseline(cy-a.W/2, admitFontSize), line)
	}
	pdf.TransformEnd()
}

// drawStubOutline strokes the cut line of a stub: its rectangle with a
// semicircular notch cut into the top and bottom edges at the perforation.
// Arc angles run anticlockwise from 3 o'clock, as seen on the page.
func drawStubOutline(pdf *fpdf.Fpdf, s stubParts) {
	b, r := s.Stub, s.NotchRadius
	top, bottom := s.Notches[0], s.Notches[1]
	pdf.MoveTo(b.X, b.Y)
	pdf.LineTo(top.X-r, top.Y)
	pdf.ArcTo(top.X, top.Y, r, r, 0, 180, 360) // left, down, right
	pdf.LineTo(b.X+b.W, b.Y)
	pdf.LineTo(b.X+b.W, b.Y+b.H)
	pdf.LineTo(bottom.X+r, bottom.Y)
	pdf.ArcTo(bottom.X, bottom.Y, r, r, 0, 0, 180) // right, up, left
	pdf.LineTo(b.X, b.Y+b.H)
	pdf.ClosePath()
	pdf.DrawPath("D")
}
