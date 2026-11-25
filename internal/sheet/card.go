package sheet

import (
	"fmt"
	"math"
)

// Review card size, in millimetres.
const (
	CardWidth  = 90.0
	CardHeight = 55.0
)

// Review card spacing, in millimetres.
const (
	cardPadding = 3.0 // inner margin of a card
	cardTextGap = 3.0 // between the cover area and the text column
	blockGap    = 1.5 // between the title block, date, stars and writing lines
	starSize    = 4.2 // side of the square a star is drawn in
	starGap     = 1.2 // between stars
)

// Review card text sizes, in points, and how many there are of each part.
const (
	titleFontSize   = 9.5
	creatorFontSize = 8.0
	labelFontSize   = 7.0
	titleLines      = 2 // the title wraps to at most this many lines
	starCount       = 5
	writingLines    = 3
)

// Text line spacing.
const (
	mmPerPoint  = mmPerInch / 72
	lineSpacing = 1.25 // line height / font size
)

// lineHeight is the height of a line of text in a pt-point font, in mm.
func lineHeight(pt float64) float64 {
	return pt * mmPerPoint * lineSpacing
}

// baseline is the baseline of a line of pt-point text whose line box starts
// at top, in mm: the glyphs are centred in the line box.
func baseline(top, pt float64) float64 {
	size := pt * mmPerPoint
	return top + (size*lineSpacing-size)/2 + 0.8*size
}

// NewCardGrid derives the grid of review cards on page: as many
// CardWidth × CardHeight cards as fit within MinMargin margins and Gap
// between cards, centred on the page both ways. It returns an error if not
// even one card fits on the page.
func NewCardGrid(page Page) (Grid, error) {
	return newFixedGrid(page, CardWidth, CardHeight, "card")
}

// newFixedGrid derives the grid of fixed-size w × h items on page, as many
// as fit within MinMargin margins and Gap between items, centred on the
// page both ways. noun names an item in the error returned if not even one
// fits on the page.
func newFixedGrid(page Page, w, h float64, noun string) (Grid, error) {
	usableW := page.Width - 2*MinMargin
	usableH := page.Height - 2*MinMargin
	// A small tolerance keeps an exact fit from being lost to rounding
	// (TestNewFixedGridToleratesAnExactFit, card_test.go).
	const tol = 1e-9
	if w > usableW+tol || h > usableH+tol {
		return Grid{}, fmt.Errorf("a %v × %v mm %s does not fit on a %v × %v mm page", w, h, noun, page.Width, page.Height)
	}
	cols := int(math.Floor((usableW+Gap)/(w+Gap) + tol))
	rows := int(math.Floor((usableH+Gap)/(h+Gap) + tol))
	return Grid{
		Page:        page,
		CoverHeight: h,
		CellWidth:   w,
		Columns:     cols,
		Rows:        rows,
		MarginX:     (page.Width - float64(cols)*w - float64(cols-1)*Gap) / 2,
		MarginY:     (page.Height - float64(rows)*h - float64(rows-1)*Gap) / 2,
	}, nil
}

// cardParts are the boxes of the parts of one review card on its page, in
// mm. The text boxes are the room each part may use; Lines are horizontal
// rules of zero height.
type cardParts struct {
	Card    Box   // the whole card, outlined as its cut line
	Cover   Box   // the cover image
	Text    Box   // the text column, right of the cover area
	Title   Box   // up to titleLines lines of title
	Creator Box   // one line of creator, drawn right under the title
	Date    Box   // the date label and the rule to write the date on
	Stars   []Box // starCount squares, each holding one star outline
	Lines   []Box // writingLines rules for notes, the last on the bottom padding
}

// layoutCard places the parts of a review card in card, for a cover with
// aspect ratio (width / height) ratio. Inside cardPadding, the cover area on
// the left is full height and 2:3 wide; the cover is shrunk to fit it,
// keeping its ratio, left-aligned and vertically centred. The text column
// fills the rest of the width and stacks, top-down, the title, creator, date
// field and stars, then spreads the writing lines over the remaining height.
func layoutCard(card Box, ratio float64) cardParts {
	inner := Box{
		X: card.X + cardPadding,
		Y: card.Y + cardPadding,
		W: card.W - 2*cardPadding,
		H: card.H - 2*cardPadding,
	}
	areaW := inner.H * minCellRatio
	w, h := inner.H*ratio, inner.H
	if w > areaW {
		w, h = areaW, areaW/ratio
	}
	textX := inner.X + areaW + cardTextGap
	c := cardParts{
		Card:  card,
		Cover: Box{X: inner.X, Y: inner.Y + (inner.H-h)/2, W: w, H: h},
		Text:  Box{X: textX, Y: inner.Y, W: inner.X + inner.W - textX, H: inner.H},
	}

	y := c.Text.Y
	next := func(h float64) Box {
		b := Box{X: c.Text.X, Y: y, W: c.Text.W, H: h}
		y += h
		return b
	}
	c.Title = next(titleLines * lineHeight(titleFontSize))
	c.Creator = next(lineHeight(creatorFontSize))
	y += blockGap
	c.Date = next(lineHeight(labelFontSize))
	y += blockGap
	c.Stars = make([]Box, starCount)
	for i := range c.Stars {
		c.Stars[i] = Box{X: c.Text.X + float64(i)*(starSize+starGap), Y: y, W: starSize, H: starSize}
	}
	y += starSize

	step := (c.Text.Y + c.Text.H - y) / writingLines
	c.Lines = make([]Box, writingLines)
	for i := range c.Lines {
		c.Lines[i] = Box{X: c.Text.X, Y: y + float64(i+1)*step, W: c.Text.W}
	}
	return c
}
