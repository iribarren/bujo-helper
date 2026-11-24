package sheet

import (
	"fmt"
	"image"
	"math"
)

// CutMode is how the covers of a cover sheet are prepared for cutting.
type CutMode struct {
	ID      string
	Label   string
	FileTag string // short, file-name-safe name, e.g. for "covers_A4_Standard_bleed.pdf"
}

// DefaultCut is the ID of the cut mode used when none is chosen: a grey
// border on the cut line.
const DefaultCut = "border"

// IDs of the cut modes other than the border.
const (
	bleedCut = "bleed" // images extend past the cut line, with crop marks
	noCut    = "none"  // images only
)

// CutModes lists the cut modes, the default first.
var CutModes = []CutMode{
	{ID: DefaultCut, Label: "Border", FileTag: "border"},
	{ID: bleedCut, Label: "Bleed + crop marks", FileTag: "bleed"},
	{ID: noCut, Label: "None", FileTag: "none"},
}

// CutModeByID returns the cut mode with the given ID; "" means DefaultCut.
func CutModeByID(id string) (CutMode, error) {
	if id == "" {
		id = DefaultCut
	}
	for _, c := range CutModes {
		if c.ID == id {
			return c, nil
		}
	}
	return CutMode{}, fmt.Errorf("unknown cut mode %q", id)
}

// Bleed is how far, in mm, a cover's image extends past its cut line in
// the bleed cut mode. It is less than Gap/2, so neighbours never overlap.
const Bleed = 1.0

// cropMarkWidth is the line width of crop marks, in mm.
const cropMarkWidth = 0.1

// Line is a straight line from (X1, Y1) to (X2, Y2), in millimetres.
type Line struct {
	X1, Y1, X2, Y2 float64
}

// bleedBox returns cut box b grown by Bleed on every side: the area the
// image covers in the bleed cut mode.
func bleedBox(b Box) Box {
	return Box{X: b.X - Bleed, Y: b.Y - Bleed, W: b.W + 2*Bleed, H: b.H + 2*Bleed}
}

// cropMarks returns the crop marks of cut box b: at each corner, two lines
// continuing its cut edges outward, from Bleed to Gap/2 past the cut line.
// They stay clear of the bleed and within the cover's half of the gap.
func cropMarks(b Box) []Line {
	left, right := b.X, b.X+b.W
	top, bottom := b.Y, b.Y+b.H
	marks := make([]Line, 0, 8)
	for _, y := range []float64{top, bottom} {
		marks = append(marks,
			Line{X1: left - Bleed, Y1: y, X2: left - Gap/2, Y2: y},
			Line{X1: right + Bleed, Y1: y, X2: right + Gap/2, Y2: y})
	}
	for _, x := range []float64{left, right} {
		marks = append(marks,
			Line{X1: x, Y1: top - Bleed, X2: x, Y2: top - Gap/2},
			Line{X1: x, Y1: bottom + Bleed, X2: x, Y2: bottom + Gap/2})
	}
	return marks
}

// bleedPad is how many pixels an image px pixels tall, printed mm tall,
// needs on each side to cover Bleed: at least one.
func bleedPad(px int, mm float64) int {
	// The tolerance keeps an exact fit from rounding up a pixel.
	return max(1, int(math.Ceil(Bleed*float64(px)/mm-1e-9)))
}

// boxBleedPad is how many pixels an image w × h pixels, printed on box b,
// needs on each side to cover Bleed on both axes. Rounding can leave the
// image with more pixels per mm across than down, or the other way round,
// so the larger of the two pads is taken.
func boxBleedPad(w, h int, b Box) int {
	return max(bleedPad(w, b.W), bleedPad(h, b.H))
}

// extendEdges returns src with pad pixels added on every side, each a copy
// of the nearest edge pixel, so the image runs on past its edges without
// zooming or cropping it.
func extendEdges(src *image.RGBA, pad int) *image.RGBA {
	if pad <= 0 {
		return src
	}
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w+2*pad, h+2*pad))
	for y := range h + 2*pad {
		sy := sb.Min.Y + max(0, min(y-pad, h-1))
		for x := range w + 2*pad {
			sx := sb.Min.X + max(0, min(x-pad, w-1))
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}

// paddedBox returns where to draw an image w × h pixels plus pad pixels on
// every side so that its w × h pixels land exactly on cut box b, as they do
// without padding.
func paddedBox(b Box, w, h, pad int) Box {
	dx := float64(pad) * b.W / float64(w)
	dy := float64(pad) * b.H / float64(h)
	return Box{X: b.X - dx, Y: b.Y - dy, W: b.W + 2*dx, H: b.H + 2*dy}
}
