package sheet

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"path/filepath"

	"github.com/go-pdf/fpdf"
)

// CropMode is a standard print shape the photos of a cover sheet are cropped
// to: each photo is centre-cropped to fill its window exactly.
type CropMode struct {
	ID      string
	Label   string
	FileTag string // short, file-name-safe name, e.g. for "covers_A4_instax.pdf"
	// UsesSize is whether the cover size preset applies: it sets the side
	// of a square, and the cover height when nothing is cropped.
	UsesSize bool
	// UsesCut is whether the cut mode applies: the photo fills its piece,
	// whose cut line follows the cut mode. Framed pieces always draw a grey
	// frame border instead.
	UsesCut bool
	// Frames is how many photos one printed piece holds: the frames of a
	// strip, else 1.
	Frames int
}

// DefaultCrop is the ID of the crop mode used when none is chosen: no
// cropping, today's cover sheet.
const DefaultCrop = "none"

// IDs of the crop modes other than None.
const (
	instaxCrop = "instax"
	squareCrop = "square"
	strip3Crop = "strip3"
	strip4Crop = "strip4"
	walletCrop = "wallet"
)

// CropModes lists the crop modes, the default first.
var CropModes = []CropMode{
	{ID: DefaultCrop, Label: "None", FileTag: "none", UsesSize: true, UsesCut: true, Frames: 1},
	{ID: instaxCrop, Label: "Instant mini", FileTag: "instax", Frames: 1},
	{ID: squareCrop, Label: "Square", FileTag: "square", UsesSize: true, UsesCut: true, Frames: 1},
	{ID: strip3Crop, Label: "Photo strip (3 frames)", FileTag: "strip3", Frames: 3},
	{ID: strip4Crop, Label: "Photo strip (4 frames)", FileTag: "strip4", Frames: 4},
	{ID: walletCrop, Label: "Wallet", FileTag: "wallet", UsesCut: true, Frames: 1},
}

// CropModeByID returns the crop mode with the given ID; "" means
// DefaultCrop.
func CropModeByID(id string) (CropMode, error) {
	if id == "" {
		id = DefaultCrop
	}
	for _, c := range CropModes {
		if c.ID == id {
			return c, nil
		}
	}
	return CropMode{}, fmt.Errorf("unknown crop mode %q", id)
}

// Sizes of the cropped pieces, in mm, portrait.
const (
	// An instant mini print: a white frame around the image window, with
	// a wider empty band under it.
	instaxWidth   = 54.0
	instaxHeight  = 86.0
	instaxWindowW = 46.0
	instaxWindowH = 62.0
	instaxSide    = 4.0 // left and right border
	instaxTop     = 7.0 // top border; the bottom band takes the rest (17 mm)
	// A 2 × 6 in photo strip: its frames are stacked down it, stripBorder
	// from its edges and apart.
	stripWidth  = 2 * mmPerInch
	stripHeight = 6 * mmPerInch
	stripBorder = 3.0
	// A wallet print, filled by the photo.
	walletWidth  = 64.0
	walletHeight = 89.0
)

// cropCellSize returns the width and height of a printed piece of crop mode
// m, in mm; a square's side is coverHeight.
func cropCellSize(m CropMode, coverHeight float64) (w, h float64) {
	switch m.ID {
	case instaxCrop:
		return instaxWidth, instaxHeight
	case strip3Crop, strip4Crop:
		return stripWidth, stripHeight
	case walletCrop:
		return walletWidth, walletHeight
	}
	return coverHeight, coverHeight
}

// NewCropGrid derives the grid of the printed pieces of crop mode m on page,
// a square's side being coverHeight: as many pieces as fit within MinMargin
// margins and Gap between pieces, centred on the page both ways. It returns
// an error if not even one piece fits on the page.
func NewCropGrid(page Page, m CropMode, coverHeight float64) (Grid, error) {
	w, h := cropCellSize(m, coverHeight)
	if !(w > 0 && h > 0) {
		return Grid{}, fmt.Errorf("a %v × %v mm %s print does not fit on a %v × %v mm page", w, h, m.Label, page.Width, page.Height)
	}
	return newFixedGrid(page, w, h, m.Label+" print")
}

// cropParts are the boxes of one printed piece on its page, in mm.
type cropParts struct {
	Frame Box // the whole piece, its cut line
	// Windows are where photos go, in order, each filled exactly: the
	// frames of a strip top to bottom, else one.
	Windows []Box
}

// layoutCrop places the windows of a piece of crop mode m in cell: an
// instant mini's window instaxSide from the sides and instaxTop from the
// top, a strip's m.Frames equal frames stacked stripBorder from its edges
// and apart, and otherwise the whole cell.
func layoutCrop(m CropMode, cell Box) cropParts {
	p := cropParts{Frame: cell}
	switch m.ID {
	case instaxCrop:
		p.Windows = []Box{{X: cell.X + instaxSide, Y: cell.Y + instaxTop, W: instaxWindowW, H: instaxWindowH}}
	case strip3Crop, strip4Crop:
		n := float64(m.Frames)
		h := (cell.H - (n+1)*stripBorder) / n
		for i := range m.Frames {
			y := cell.Y + stripBorder + float64(i)*(h+stripBorder)
			p.Windows = append(p.Windows, Box{X: cell.X + stripBorder, Y: y, W: cell.W - 2*stripBorder, H: h})
		}
	default:
		p.Windows = []Box{cell}
	}
	return p
}

// centreCrop returns the middle part of an image with bounds b that has
// aspect ratio (width / height) ratio: its full height if the image is
// wider than ratio, else its full width, and at least one pixel either way.
// Any odd pixel left over goes to the right or bottom.
func centreCrop(b image.Rectangle, ratio float64) image.Rectangle {
	w, h := b.Dx(), b.Dy()
	if float64(w) > float64(h)*ratio {
		cw := min(w, max(1, int(math.Round(float64(h)*ratio))))
		x := b.Min.X + (w-cw)/2
		return image.Rect(x, b.Min.Y, x+cw, b.Max.Y)
	}
	ch := min(h, max(1, int(math.Round(float64(w)/ratio))))
	y := b.Min.Y + (h-ch)/2
	return image.Rect(b.Min.X, y, b.Max.X, y+ch)
}

// subImager is an image whose part can be taken without copying it, as
// every decoded image is.
type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

// cropImage returns the part r of src.
func cropImage(src image.Image, r image.Rectangle) image.Image {
	if s, ok := src.(subImager); ok {
		return s.SubImage(r)
	}
	return croppedImage{src, r}
}

// croppedImage is the part r of an image that cannot take a part itself.
type croppedImage struct {
	image.Image
	r image.Rectangle
}

func (c croppedImage) Bounds() image.Rectangle { return c.r }

// printCroppedImage decodes the image at path, crops its middle to the
// shape of window and returns it as printImage does for a printed height of
// window.H, so it fills window exactly with no distortion. The file is
// never changed.
func printCroppedImage(path string, window Box, bleed bool) (printed, error) {
	src, err := decodeImage(path)
	if err != nil {
		return printed{}, err
	}
	return encodePrinted(path, cropImage(src, centreCrop(src.Bounds(), window.W/window.H)), window.H, bleed)
}

// printCroppedImages prepares the image of every photo with
// printCroppedImage for its window, as printImages does.
func printCroppedImages(paths []string, windows []Box, bleed bool) ([]printed, error) {
	images := make([]printed, len(paths))
	err := forEachImage(len(paths), func(i int) (err error) {
		images[i], err = printCroppedImage(paths[i], windows[i], bleed)
		return err
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}

// buildCrops draws a cover sheet of photos cropped to crop's shape, one
// piece per grid cell and m.Frames photos per piece, in order; the last
// piece may be partial. Pieces the photo fills are prepared for cutting as
// cut says, like the covers of a cover sheet; framed pieces get the grey
// cut border around the frame.
func buildCrops(grid Grid, paths []string, crop CropMode, cut CutMode) (*pdfDoc, error) {
	pieces := (len(paths) + crop.Frames - 1) / crop.Frames
	cells, err := grid.Cells(pieces)
	if err != nil {
		return nil, err
	}
	var parts [][]cropParts // by page
	var windows []Box       // by photo
	pages := make([][]Box, len(cells))
	for i, page := range cells {
		var pp []cropParts
		for _, cell := range page {
			p := layoutCrop(crop, cell)
			p.Windows = p.Windows[:min(len(p.Windows), len(paths)-len(windows))]
			windows = append(windows, p.Windows...)
			pages[i] = append(pages[i], p.Windows...)
			pp = append(pp, p)
		}
		parts = append(parts, pp)
	}
	bleed := crop.UsesCut && cut.ID == bleedCut
	images, err := printCroppedImages(paths, windows, bleed)
	if err != nil {
		return nil, err
	}
	if crop.UsesCut {
		return drawCovers(grid.Page, paths, pages, images, cut)
	}

	pdf := newPDF(grid.Page)
	imgOpts := fpdf.ImageOptions{ImageType: "JPG"}
	i := 0
	for _, page := range parts {
		pdf.AddPage()
		for _, p := range page {
			pdf.Rect(p.Frame.X, p.Frame.Y, p.Frame.W, p.Frame.H, "D")
			for _, w := range p.Windows {
				name := fmt.Sprintf("cover-%d", i)
				pdf.RegisterImageOptionsReader(name, imgOpts, bytes.NewReader(images[i].jpeg))
				pdf.ImageOptions(name, w.X, w.Y, w.W, w.H, false, imgOpts, 0, "")
				if err := pdf.Error(); err != nil {
					return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
				}
				i++
			}
		}
	}
	return pdf, nil
}
