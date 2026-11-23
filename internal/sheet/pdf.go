package sheet

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	// Decoders for the supported cover formats.
	_ "image/gif"
	_ "image/png"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Print settings.
const (
	dpi         = 300  // resolution covers are downscaled to
	jpegQuality = 90   // quality of the re-encoded covers
	borderWidth = 0.2  // cut border line width, mm
	borderGrey  = 0x99 // cut border colour, #999999
	mmPerInch   = 25.4
	// maxWorkers bounds the images decoded at once: each full-size cover
	// takes tens of MB while it is scaled.
	maxWorkers = 4
)

// Build writes a PDF of the images at paths, in the format and on the grid
// of opts, to w: a cover sheet, a review card per image, a book spine per
// image, a ticket stub per image, full pages of covers under a header, a
// checklist row per image under a header, or a polaroid frame per image;
// on a cover sheet with a crop mode, photos centre-cropped to its shape;
// with opts.Credits, followed by credits pages for the images that have a
// credit sidecar. Images are downscaled to 300 dpi at their printed size and re-encoded as
// JPEG to keep the file small. If any image cannot be read or opts are
// invalid, Build returns an error (naming the file, if any) and writes
// nothing.
func Build(w io.Writer, paths []string, opts Options) error {
	pdf, err := build(paths, opts)
	if err != nil {
		return err
	}
	// Render in memory first so a failure leaves w untouched.
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return fmt.Errorf("write PDF: %w", err)
	}
	_, err = w.Write(buf.Bytes())
	return err
}

// build draws the document Build writes.
func build(paths []string, opts Options) (*pdfDoc, error) {
	if len(paths) == 0 {
		return nil, errors.New("no covers selected")
	}
	format, err := FormatByID(opts.Format)
	if err != nil {
		return nil, err
	}
	cut, err := CutModeByID(opts.Cut)
	if err != nil {
		return nil, err
	}
	grid, err := opts.Grid()
	if err != nil {
		return nil, err
	}
	ratios, err := AspectRatios(paths)
	if err != nil {
		return nil, err
	}

	var pdf *pdfDoc
	switch format.ID {
	case cardsFormat:
		pdf, err = buildCards(grid, paths, ratios, opts)
	case spinesFormat:
		pdf, err = buildSpines(grid, paths)
	case stubsFormat:
		pdf, err = buildStubs(grid, paths, ratios, opts.labels())
	case fullFormat:
		pdf, err = buildFullPage(grid.Page, paths, ratios, opts.header(), opts.lettering())
	case checkFormat:
		pdf, err = buildChecklist(grid, paths, ratios, opts.header(), opts.lettering())
	case polaFormat:
		pdf, err = buildPolaroids(grid, paths, ratios)
	default:
		var crop CropMode
		if crop, err = opts.cropMode(); err != nil {
			return nil, err
		}
		if crop.ID != DefaultCrop {
			pdf, err = buildCrops(grid, paths, crop, cut)
		} else {
			pdf, err = buildCovers(grid, paths, ratios, cut)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := addCredits(pdf, grid.Page, paths, opts); err != nil {
		return nil, err
	}
	return pdf, nil
}

// pdfDoc is the document every sheet and page draws on. Text reaches fpdf
// only through its Text and GetStringWidth, which make the text printable
// first, so titles from file names and text the user types cannot carry a
// rune the UTF-8 fonts fail on, whatever format or page kind draws them.
type pdfDoc struct{ *fpdf.Fpdf }

// Text writes s, made printable, with its baseline starting at (x, y).
func (d *pdfDoc) Text(x, y float64, s string) { d.Fpdf.Text(x, y, printable(s)) }

// GetStringWidth measures s, made printable, in the current font, so text
// is measured as it is drawn.
func (d *pdfDoc) GetStringWidth(s string) float64 { return d.Fpdf.GetStringWidth(printable(s)) }

// printable returns s with invalid UTF-8 and runes outside the Basic
// Multilingual Plane (emoji, rare CJK) replaced by "?", one per rune: the
// PDF's UTF-8 fonts index glyph widths by 16-bit rune and fail on anything
// above. Runes in the plane that the font lacks print as empty boxes.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r == utf8.RuneError || r > 0xFFFF {
			return '?'
		}
		return r
	}, s)
}

// newPDF starts a document of page-sized pages, in mm, drawing cut borders.
// Its fonts and images are written in sorted order, not map order, so the
// same inputs (and creation date) give the same bytes.
func newPDF(page Page) *pdfDoc {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           fpdf.SizeType{Wd: page.Width, Ht: page.Height},
	})
	pdf.SetCatalogSort(true)
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetDrawColor(borderGrey, borderGrey, borderGrey)
	pdf.SetLineWidth(borderWidth)
	return &pdfDoc{pdf}
}

// buildCovers draws a cover sheet: each image in its cell as grid.Flow
// lays them out, prepared for cutting as cut says (see drawCovers).
func buildCovers(grid Grid, paths []string, ratios []float64, cut CutMode) (*pdfDoc, error) {
	pages, err := grid.Flow(ratios)
	if err != nil {
		return nil, err
	}
	var boxes []Box
	for _, page := range pages {
		boxes = append(boxes, page...)
	}
	images, err := printImages(paths, boxes, cut.ID == bleedCut)
	if err != nil {
		return nil, err
	}
	return drawCovers(grid.Page, paths, pages, images, cut)
}

// drawCovers draws the prepared images, one per box, on pages of page
// size, prepared for cutting as cut says. Border outlines each image with a
// grey line on its cut box; None draws the image alone; Bleed runs the
// image Bleed past its cut box, by repeating its edge pixels, and adds crop
// marks.
func drawCovers(page Page, paths []string, pages [][]Box, images []printed, cut CutMode) (*pdfDoc, error) {
	pdf := newPDF(page)
	if cut.ID == bleedCut {
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(cropMarkWidth)
	}
	imgOpts := fpdf.ImageOptions{ImageType: "JPG"}

	i := 0
	for _, page := range pages {
		pdf.AddPage()
		for _, box := range page {
			name := fmt.Sprintf("cover-%d", i)
			img := images[i]
			pdf.RegisterImageOptionsReader(name, imgOpts, bytes.NewReader(img.jpeg))
			switch cut.ID {
			case bleedCut:
				// Clip to the bleed box: a low-resolution image's padding
				// may reach further.
				bl := bleedBox(box)
				pdf.ClipRect(bl.X, bl.Y, bl.W, bl.H, false)
				at := paddedBox(box, img.w, img.h, img.pad)
				pdf.ImageOptions(name, at.X, at.Y, at.W, at.H, false, imgOpts, 0, "")
				pdf.ClipEnd()
				for _, m := range cropMarks(box) {
					pdf.Line(m.X1, m.Y1, m.X2, m.Y2)
				}
			case noCut:
				pdf.ImageOptions(name, box.X, box.Y, box.W, box.H, false, imgOpts, 0, "")
			default:
				pdf.ImageOptions(name, box.X, box.Y, box.W, box.H, false, imgOpts, 0, "")
				pdf.Rect(box.X, box.Y, box.W, box.H, "D")
			}
			if err := pdf.Error(); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}
	return pdf, nil
}

// aspectRatio reads the image header of path and returns width / height.
func aspectRatio(path string) (float64, error) {
	name := filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, fmt.Errorf("%s: not a readable image: %w", name, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, fmt.Errorf("%s: image has no size", name)
	}
	return float64(cfg.Width) / float64(cfg.Height), nil
}

// printed is a cover image ready to embed: the JPEG of an image w × h
// pixels plus pad pixels of repeated edge on every side.
type printed struct {
	jpeg      []byte
	w, h, pad int
}

// printImages prepares the image of every cover with printImage, a few at a
// time (decoding and scaling dominate the export time). On failure it
// returns the error of the first failing cover in order.
func printImages(paths []string, boxes []Box, bleed bool) ([]printed, error) {
	images := make([]printed, len(paths))
	err := forEachImage(len(paths), func(i int) (err error) {
		images[i], err = printImage(paths[i], boxes[i].H, bleed)
		return err
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}

// forEachImage runs work for 0 … n-1 on at most maxWorkers goroutines (one
// image each), and returns the error of the lowest failing index, if any.
func forEachImage(n int, work func(i int) error) error {
	errs := make([]error, n)
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(maxWorkers, runtime.NumCPU(), n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				errs[i] = work(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// decodeImage decodes the image at path, naming the file in any error.
func decodeImage(path string) (image.Image, error) {
	name := filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: not a readable image: %w", name, err)
	}
	return src, nil
}

// printImage decodes the image at path and returns it as a JPEG sized for a
// printed height of heightMM. With bleed, its edge pixels are repeated
// outward far enough to cover Bleed.
func printImage(path string, heightMM float64, bleed bool) (printed, error) {
	src, err := decodeImage(path)
	if err != nil {
		return printed{}, err
	}
	return encodePrinted(path, src, heightMM, bleed)
}

// encodePrinted returns src, decoded from path, as a JPEG sized for a
// printed height of heightMM, as printImage describes.
func encodePrinted(path string, src image.Image, heightMM float64, bleed bool) (printed, error) {
	img := fitImage(src, heightMM)
	p := printed{w: img.Bounds().Dx(), h: img.Bounds().Dy()}
	if bleed {
		p.pad = bleedPad(p.h, heightMM)
		img = extendEdges(img, p.pad)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return printed{}, fmt.Errorf("%s: encode: %w", filepath.Base(path), err)
	}
	p.jpeg = buf.Bytes()
	return p, nil
}

// fitImage returns src downscaled (never upscaled) to 300 dpi at a printed
// height of heightMM, with any transparency flattened onto white.
func fitImage(src image.Image, heightMM float64) *image.RGBA {
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	if target := int(math.Round(heightMM / mmPerInch * dpi)); h > target {
		w = max(1, int(math.Round(float64(w)*float64(target)/float64(h))))
		h = target
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	if w == sb.Dx() && h == sb.Dy() {
		draw.Draw(dst, dst.Bounds(), src, sb.Min, draw.Over)
	} else {
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, draw.Over, nil)
	}
	return dst
}
