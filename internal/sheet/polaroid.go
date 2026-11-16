package sheet

import (
	"fmt"
	"math"
)

// Polaroid frame proportions, as shares of the cover height H: the image
// window is 2:3, H tall; a border runs along its top and sides, and a
// thicker band under it holds the caption.
const (
	polaroidBorder = 0.08 // side and top border (4 mm for a 50 mm cover)
	polaroidBand   = 0.28 // bottom caption band (14 mm for a 50 mm cover)
)

// Polaroid caption settings: its size follows the cover height, in points
// per millimetre, within these bounds.
const (
	polaroidFontScale    = 0.18
	minPolaroidFontSize  = 6.0
	maxPolaroidFontSize  = 12.0
	polaroidCaptionLines = 2 // the title wraps to at most this many lines
)

// polaroidFrame returns the width and height of the frame of a cover
// coverHeight tall, in mm.
func polaroidFrame(coverHeight float64) (w, h float64) {
	return coverHeight * (minCellRatio + 2*polaroidBorder), coverHeight * (1 + polaroidBorder + polaroidBand)
}

// polaroidFontSize is the caption size of the frame of a cover coverHeight
// tall: polaroidFontScale points per mm, kept within minPolaroidFontSize
// and maxPolaroidFontSize, rounded to a tenth of a point.
func polaroidFontSize(coverHeight float64) float64 {
	pt := min(max(coverHeight*polaroidFontScale, minPolaroidFontSize), maxPolaroidFontSize)
	return math.Round(pt*10) / 10
}

// NewPolaroidGrid derives the grid of polaroid frames for covers
// coverHeight tall on page: as many frames as fit within MinMargin margins
// and Gap between frames, centred on the page both ways. It returns an
// error if not even one frame fits on the page.
func NewPolaroidGrid(page Page, coverHeight float64) (Grid, error) {
	if !(coverHeight > 0) {
		return Grid{}, fmt.Errorf("a %v mm polaroid cover does not fit on a %v × %v mm page", coverHeight, page.Width, page.Height)
	}
	w, h := polaroidFrame(coverHeight)
	return newFixedGrid(page, w, h, "polaroid")
}

// polaroidParts are the boxes of the parts of one polaroid on its page, in
// mm.
type polaroidParts struct {
	Frame    Box     // the whole frame, outlined as its cut line
	Window   Box     // the 2:3 image window, inside the top and side borders
	Cover    Box     // the image, shrunk to fit the window, centred in it
	Band     Box     // the bottom band, the frame's full width under the window
	Caption  Box     // the room of the caption: the band, as wide as the window
	FontSize float64 // the caption size, pt
}

// layoutPolaroid places the parts of a polaroid in frame, for an image with
// aspect ratio (width / height) ratio. The cover height follows from the
// frame's height. The window sits polaroidBorder of it from the top and
// sides, and the band under it fills the rest; the image is shrunk to fit
// the window, keeping its ratio, and centred in it.
func layoutPolaroid(frame Box, ratio float64) polaroidParts {
	_, perCover := polaroidFrame(1)
	coverHeight := frame.H / perCover
	border := coverHeight * polaroidBorder
	p := polaroidParts{
		Frame:    frame,
		Window:   Box{X: frame.X + border, Y: frame.Y + border, W: frame.W - 2*border, H: coverHeight},
		FontSize: polaroidFontSize(coverHeight),
	}
	bandY := p.Window.Y + p.Window.H
	p.Band = Box{X: frame.X, Y: bandY, W: frame.W, H: frame.Y + frame.H - bandY}
	p.Caption = Box{X: p.Window.X, Y: p.Band.Y, W: p.Window.W, H: p.Band.H}

	w, h := p.Window.H*ratio, p.Window.H
	if w > p.Window.W {
		w, h = p.Window.W, p.Window.W/ratio
	}
	p.Cover = Box{X: p.Window.X + (p.Window.W-w)/2, Y: p.Window.Y + (p.Window.H-h)/2, W: w, H: h}
	return p
}

// captionLine is a line of caption text and its line box: as wide as the
// text and one line tall.
type captionLine struct {
	Text string
	Box
}

// layoutPolaroidCaption fits title in up to polaroidCaptionLines lines of
// pt-point text no wider than caption, as measured by measure, cut with "…"
// if too long. Each line is centred across caption, and the block of lines
// down it. Empty title, or a caption too narrow, gives no lines.
func layoutPolaroidCaption(caption Box, title string, pt float64, measure func(string) float64) []captionLine {
	maxLines := min(polaroidCaptionLines, int(math.Floor(caption.H/lineHeight(pt)+fitTolerance)))
	texts := fitLines(title, caption.W, maxLines, measure)
	lh := lineHeight(pt)
	y := caption.Y + (caption.H-float64(len(texts))*lh)/2
	lines := make([]captionLine, len(texts))
	for i, text := range texts {
		w := measure(text)
		lines[i] = captionLine{Text: text, Box: Box{X: caption.X + (caption.W-w)/2, Y: y, W: w, H: lh}}
		y += lh
	}
	return lines
}
