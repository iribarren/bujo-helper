package sheet

import (
	"fmt"
	"math"
	"strings"
)

// Spine widths and spacing, in millimetres.
const (
	MinSpineWidth = 8.0
	MaxSpineWidth = 12.0
	spineGap      = 2.0 // between the spines of a row
)

// spineWidths are the widths of the spines of a row, by column, repeated
// along longer rows: a fixed pattern looks like a real shelf, yet every
// full row holds the same number of spines.
var spineWidths = []float64{10, 12, 9, 11, 8, 12, 10, 9, 11, 10}

// spineWidth is the width of the spine in column col of a row.
func spineWidth(col int) float64 {
	return spineWidths[col%len(spineWidths)]
}

// spineRowWidth is the width of a row of n spines, gaps included.
func spineRowWidth(n int) float64 {
	w := 0.0
	for col := range n {
		if col > 0 {
			w += spineGap
		}
		w += spineWidth(col)
	}
	return w
}

// NewSpineGrid derives the grid of book spines height tall on page. Each
// row holds as many spines, in the widths of spineWidths separated by
// spineGap, as fit within MinMargin side margins, and is centred
// horizontally; as many rows fit as the height allows within MinMargin,
// Gap apart, centred vertically. It returns an error if not even one spine
// fits on the page.
func NewSpineGrid(page Page, height float64) (Grid, error) {
	usableW := page.Width - 2*MinMargin
	usableH := page.Height - 2*MinMargin
	// A small tolerance keeps an exact fit from being lost to rounding.
	const tol = 1e-9
	if !(height > 0) || height > usableH+tol || spineWidth(0) > usableW+tol {
		return Grid{}, fmt.Errorf("a %v mm spine does not fit on a %v × %v mm page", height, page.Width, page.Height)
	}
	cols := 1
	for spineRowWidth(cols+1) <= usableW+tol {
		cols++
	}
	rows := int(math.Floor((usableH+Gap)/(height+Gap) + tol))
	return Grid{
		Page:        page,
		CoverHeight: height,
		Columns:     cols,
		Rows:        rows,
		MarginX:     (page.Width - spineRowWidth(cols)) / 2,
		MarginY:     (page.Height - float64(rows)*height - float64(rows-1)*Gap) / 2,
		Spines:      true,
	}, nil
}

// spineCell returns the box of the spine in column col, in a row starting
// at x and y.
func spineCell(x, y, height float64, col int) Box {
	return Box{X: x + spineRowWidth(col) + spineGap*float64(min(col, 1)), Y: y, W: spineWidth(col), H: height}
}

// Spine text settings: lengths in millimetres, font sizes in points.
const (
	spineEndPadding   = 2.0  // between the text and each end of a spine
	spineSidePadding  = 1.0  // between the text and each side of a spine
	maxSpineFontSize  = 10.0 // largest title size
	minSpineFontSize  = 5.0  // smallest text size, below which text is cut
	spineCreatorScale = 0.8  // creator size / title size
)

// spineLine is a line of text on a spine, laid out along its length. The
// text reads top to bottom, its letters' tops toward the right edge.
type spineLine struct {
	Text   string
	Bold   bool
	Size   float64 // font size, pt
	Offset float64 // from the top of the spine to the start of the text, mm
	Across float64 // from the right edge of the spine to the top of the line box, mm
}

// measureFunc returns the width of text in mm, in the bold or regular text
// font at pt points.
type measureFunc func(text string, bold bool, pt float64) float64

// layoutSpineText lays out the title (bold) and creator (regular, smaller)
// on spine, as measured by measure; empty parts are left out. The lines are
// stacked across the spine, title first, as large as the spine's width
// allows (up to maxSpineFontSize for the title, spineCreatorScale of the
// title's size for the creator); if the creator would be smaller than
// minSpineFontSize, only the title is shown. Along the spine, within
// spineEndPadding of each end, a line too long shrinks down to
// minSpineFontSize, then is cut with "…". Each line is centred along the
// spine, and the stack across it.
func layoutSpineText(spine Box, title, creator string, measure measureFunc) []spineLine {
	type part struct {
		text string
		bold bool
	}
	var parts []part
	if t := strings.Join(strings.Fields(title), " "); t != "" {
		parts = append(parts, part{t, true})
	}
	if c := strings.Join(strings.Fields(creator), " "); c != "" {
		parts = append(parts, part{c, false})
	}
	cross := spine.W - 2*spineSidePadding
	length := spine.H - 2*spineEndPadding
	if len(parts) == 0 || cross < lineHeight(minSpineFontSize) || length <= 0 {
		return nil
	}

	// The largest sizes the width allows: the first line's size, and the
	// second's as a share of the first's.
	perPoint := lineHeight(1)
	maxSize := min(maxSpineFontSize, cross/perPoint)
	if len(parts) == 2 {
		maxSize = min(maxSpineFontSize, cross/((1+spineCreatorScale)*perPoint))
		if maxSize*spineCreatorScale < minSpineFontSize {
			parts, maxSize = parts[:1], min(maxSpineFontSize, cross/perPoint)
		}
	}

	// fit sizes p to the length of the spine, from maxPt down.
	fit := func(p part, maxPt float64) (spineLine, bool) {
		pt, text := maxPt, p.text
		if w := measure(text, p.bold, pt); w > length+fitTolerance {
			pt = math.Floor(pt*length/w*10) / 10
		}
		if pt < minSpineFontSize {
			pt = minSpineFontSize
			lines := fitLines(text, length, 1, func(s string) float64 { return measure(s, p.bold, pt) })
			if len(lines) == 0 {
				return spineLine{}, false
			}
			text = lines[0]
		}
		w := measure(text, p.bold, pt)
		return spineLine{Text: text, Bold: p.bold, Size: pt, Offset: (spine.H - w) / 2}, true
	}

	var lines []spineLine
	for i, p := range parts {
		maxPt := maxSize
		if i > 0 {
			if len(lines) == 0 {
				// The title did not fit; a creator alone would read as a title.
				break
			}
			// Smaller than the line above, even when that one shrank.
			maxPt = max(minSpineFontSize, min(maxSize, lines[0].Size)*spineCreatorScale)
		}
		if l, ok := fit(p, maxPt); ok {
			lines = append(lines, l)
		}
	}

	block := 0.0
	for _, l := range lines {
		block += lineHeight(l.Size)
	}
	across := (spine.W - block) / 2
	for i := range lines {
		lines[i].Across = across
		across += lineHeight(lines[i].Size)
	}
	return lines
}
