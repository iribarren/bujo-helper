package sheet

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Review card drawing settings.
const (
	textFont      = "go" // Go fonts, embedded so any accented title prints
	titleGrey     = 0x22 // title colour
	textGrey      = 0x55 // creator and date label colour
	starGrey      = 0x77 // star outline colour
	writingGrey   = 0xcc // writing line colour
	ruleWidth     = 0.2  // date and writing line width, mm
	starLineWidth = 0.25 // star outline width, mm
	minDateRule   = 10.0 // shortest rule left for writing the date, mm
	labelRuleGap  = 1.0  // between the date label and its rule, mm
)

// buildCards draws a review card per image on grid: the cut border, the
// cover, its title and creator (from the file name), the date field, empty
// stars and writing lines.
func buildCards(grid Grid, paths []string, ratios []float64, opts Options) (*fpdf.Fpdf, error) {
	pages, err := grid.Cells(len(paths))
	if err != nil {
		return nil, err
	}
	var cards []cardParts
	var covers []Box
	for _, page := range pages {
		for _, cell := range page {
			c := layoutCard(cell, ratios[len(cards)])
			cards = append(cards, c)
			covers = append(covers, c.Cover)
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
	label := opts.dateLabel()

	i := 0
	for _, page := range pages {
		pdf.AddPage()
		for range page {
			name := fmt.Sprintf("cover-%d", i)
			pdf.RegisterImageOptionsReader(name, imgOpts, bytes.NewReader(images[i].jpeg))
			title, creator := ParseName(paths[i])
			drawCard(pdf, cards[i], name, imgOpts, title, creator, label)
			if err := pdf.Error(); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}
	return pdf, nil
}

// addTextFonts embeds the regular and bold text fonts in pdf.
func addTextFonts(pdf *fpdf.Fpdf) error {
	pdf.AddUTF8FontFromBytes(textFont, "", goregular.TTF)
	pdf.AddUTF8FontFromBytes(textFont, "B", gobold.TTF)
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("load fonts: %w", err)
	}
	return nil
}

// drawCard draws one review card with the registered cover image.
func drawCard(pdf *fpdf.Fpdf, c cardParts, image string, imgOpts fpdf.ImageOptions, title, creator, label string) {
	pdf.SetDrawColor(borderGrey, borderGrey, borderGrey)
	pdf.SetLineWidth(borderWidth)
	pdf.Rect(c.Card.X, c.Card.Y, c.Card.W, c.Card.H, "D")
	pdf.ImageOptions(image, c.Cover.X, c.Cover.Y, c.Cover.W, c.Cover.H, false, imgOpts, 0, "")

	// Title, then the creator right under its last line.
	pdf.SetTextColor(titleGrey, titleGrey, titleGrey)
	pdf.SetFont(textFont, "B", titleFontSize)
	y := c.Title.Y
	for _, line := range fitLines(title, c.Title.W, titleLines, pdf.GetStringWidth) {
		pdf.Text(c.Title.X, baseline(y, titleFontSize), line)
		y += lineHeight(titleFontSize)
	}
	pdf.SetTextColor(textGrey, textGrey, textGrey)
	pdf.SetFont(textFont, "", creatorFontSize)
	for _, line := range fitLines(creator, c.Creator.W, 1, pdf.GetStringWidth) {
		pdf.Text(c.Creator.X, baseline(y, creatorFontSize), line)
	}

	// Date label, and a rule to the right edge to write the date on.
	pdf.SetFont(textFont, "", labelFontSize)
	base := baseline(c.Date.Y, labelFontSize)
	ruleX := c.Date.X
	for _, line := range fitLines(label, c.Date.W-minDateRule, 1, pdf.GetStringWidth) {
		pdf.Text(c.Date.X, base, line)
		ruleX += pdf.GetStringWidth(line) + labelRuleGap
	}
	pdf.SetDrawColor(writingGrey, writingGrey, writingGrey)
	pdf.SetLineWidth(ruleWidth)
	pdf.Line(ruleX, base, c.Date.X+c.Date.W, base)

	pdf.SetDrawColor(starGrey, starGrey, starGrey)
	pdf.SetLineWidth(starLineWidth)
	for _, s := range c.Stars {
		pdf.Polygon(starPoints(s), "D")
	}

	pdf.SetDrawColor(writingGrey, writingGrey, writingGrey)
	pdf.SetLineWidth(ruleWidth)
	for _, l := range c.Lines {
		pdf.Line(l.X, l.Y, l.X+l.W, l.Y)
	}
}

// starPoints returns the 10 corners of a five-pointed star, point up, as
// large as fits in b and centred in it, starting at the top point.
func starPoints(b Box) []fpdf.PointType {
	cos36 := math.Cos(math.Pi / 5)
	sin72 := math.Sin(2 * math.Pi / 5)
	// A star of outer radius r is 2r·sin 72° wide and r(1 + cos 36°) tall.
	r := min(b.H/(1+cos36), b.W/(2*sin72))
	inner := r * math.Sin(math.Pi/10) / math.Sin(3*math.Pi/10)
	cx := b.X + b.W/2
	cy := b.Y + (b.H-r*(1+cos36))/2 + r
	pts := make([]fpdf.PointType, 10)
	for i := range pts {
		radius := r
		if i%2 == 1 {
			radius = inner
		}
		a := -math.Pi/2 + float64(i)*math.Pi/5
		pts[i] = fpdf.PointType{X: cx + radius*math.Cos(a), Y: cy + radius*math.Sin(a)}
	}
	return pts
}
