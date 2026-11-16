package sheet

import (
	"fmt"
	"image/color"
	"path/filepath"

	"github.com/go-pdf/fpdf"
)

// buildSpines draws a book spine per image on grid: a strip filled with the
// cover's dominant colour, outlined by its cut border, with the title and
// creator (from the file name) reading top to bottom in black or white,
// whichever contrasts more.
func buildSpines(grid Grid, paths []string) (*fpdf.Fpdf, error) {
	pages, err := grid.Cells(len(paths))
	if err != nil {
		return nil, err
	}
	colors, err := spineColors(paths)
	if err != nil {
		return nil, err
	}

	pdf := newPDF(grid.Page)
	if err := addTextFonts(pdf); err != nil {
		return nil, err
	}
	measure := func(text string, bold bool, pt float64) float64 {
		pdf.SetFont(textFont, fontStyle(bold), pt)
		return pdf.GetStringWidth(text)
	}

	i := 0
	for _, page := range pages {
		pdf.AddPage()
		for _, spine := range page {
			title, creator := ParseName(paths[i])
			drawSpine(pdf, spine, colors[i], layoutSpineText(spine, title, creator, measure))
			if err := pdf.Error(); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}
	return pdf, nil
}

// spineColors returns the dominant colour of every image, a few at a time.
// On failure it returns the error of the first failing image in order.
func spineColors(paths []string) ([]color.RGBA, error) {
	colors := make([]color.RGBA, len(paths))
	err := forEachImage(len(paths), func(i int) error {
		img, err := decodeImage(paths[i])
		if err != nil {
			return err
		}
		colors[i] = dominantColor(img)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return colors, nil
}

// fontStyle is the fpdf style of the bold or regular text font.
func fontStyle(bold bool) string {
	if bold {
		return "B"
	}
	return ""
}

// drawSpine fills spine with bg, outlines it with the cut border and writes
// lines on it, rotated a quarter turn clockwise to read top to bottom.
func drawSpine(pdf *fpdf.Fpdf, spine Box, bg color.RGBA, lines []spineLine) {
	pdf.SetFillColor(int(bg.R), int(bg.G), int(bg.B))
	pdf.SetDrawColor(borderGrey, borderGrey, borderGrey)
	pdf.SetLineWidth(borderWidth)
	pdf.Rect(spine.X, spine.Y, spine.W, spine.H, "FD")

	fg := textColorFor(bg)
	pdf.SetTextColor(int(fg.R), int(fg.G), int(fg.B))
	// Turned about the spine's top-right corner, x runs down the spine
	// and y leftward across it, from its right edge.
	x, y := spine.X+spine.W, spine.Y
	pdf.TransformBegin()
	pdf.TransformRotate(-90, x, y)
	for _, l := range lines {
		pdf.SetFont(textFont, fontStyle(l.Bold), l.Size)
		pdf.Text(x+l.Offset, baseline(y+l.Across, l.Size), l.Text)
	}
	pdf.TransformEnd()
}
