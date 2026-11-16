package sheet

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/go-pdf/fpdf"
)

// buildPolaroids draws a polaroid per image on grid: the frame's cut
// border, the image in its window and the title (from the file name) as a
// caption in the band below.
func buildPolaroids(grid Grid, paths []string, ratios []float64) (*fpdf.Fpdf, error) {
	pages, err := grid.Cells(len(paths))
	if err != nil {
		return nil, err
	}
	var polaroids []polaroidParts
	var covers []Box
	for _, page := range pages {
		for _, cell := range page {
			p := layoutPolaroid(cell, ratios[len(polaroids)])
			polaroids = append(polaroids, p)
			covers = append(covers, p.Cover)
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
			title, _ := ParseName(paths[i])
			drawPolaroid(pdf, polaroids[i], name, imgOpts, title)
			if err := pdf.Error(); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}
	return pdf, nil
}

// drawPolaroid draws one polaroid with the registered image: the frame's
// grey cut border, the image and the title, regular, centred in up to
// polaroidCaptionLines lines in the caption.
func drawPolaroid(pdf *fpdf.Fpdf, p polaroidParts, image string, imgOpts fpdf.ImageOptions, title string) {
	pdf.SetDrawColor(borderGrey, borderGrey, borderGrey)
	pdf.SetLineWidth(borderWidth)
	pdf.Rect(p.Frame.X, p.Frame.Y, p.Frame.W, p.Frame.H, "D")
	pdf.ImageOptions(image, p.Cover.X, p.Cover.Y, p.Cover.W, p.Cover.H, false, imgOpts, 0, "")

	pdf.SetTextColor(titleGrey, titleGrey, titleGrey)
	pdf.SetFont(textFont, "", p.FontSize)
	for _, l := range layoutPolaroidCaption(p.Caption, title, p.FontSize, pdf.GetStringWidth) {
		pdf.Text(l.X, baseline(l.Y, p.FontSize), l.Text)
	}
}
