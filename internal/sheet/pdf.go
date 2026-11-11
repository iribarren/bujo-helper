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
	"sync"

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

// Build writes a PDF of the images at paths, laid out by Layout, to w.
// Images are downscaled to 300 dpi at their printed size and re-encoded as
// JPEG to keep the file small. If any image cannot be read, Build returns an
// error naming the file and writes nothing.
func Build(w io.Writer, paths []string) error {
	if len(paths) == 0 {
		return errors.New("no covers selected")
	}
	ratios := make([]float64, len(paths))
	for i, p := range paths {
		r, err := aspectRatio(p)
		if err != nil {
			return err
		}
		ratios[i] = r
	}
	pages, err := Layout(ratios)
	if err != nil {
		return err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetDrawColor(borderGrey, borderGrey, borderGrey)
	pdf.SetLineWidth(borderWidth)
	opts := fpdf.ImageOptions{ImageType: "JPG"}

	var boxes []Box
	for _, page := range pages {
		boxes = append(boxes, page...)
	}
	images, err := printImages(paths, boxes)
	if err != nil {
		return err
	}

	i := 0
	for _, page := range pages {
		pdf.AddPage()
		for _, box := range page {
			name := fmt.Sprintf("cover-%d", i)
			pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(images[i]))
			pdf.ImageOptions(name, box.X, box.Y, box.W, box.H, false, opts, 0, "")
			pdf.Rect(box.X, box.Y, box.W, box.H, "D")
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("%s: %w", filepath.Base(paths[i]), err)
			}
			i++
		}
	}

	// Render in memory first so a failure leaves w untouched.
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return fmt.Errorf("write PDF: %w", err)
	}
	_, err = w.Write(buf.Bytes())
	return err
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

// printImages prepares the image of every cover with printImage, a few at a
// time (decoding and scaling dominate the export time). On failure it
// returns the error of the first failing cover in order.
func printImages(paths []string, boxes []Box) ([][]byte, error) {
	images := make([][]byte, len(paths))
	errs := make([]error, len(paths))
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(maxWorkers, runtime.NumCPU(), len(paths)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				images[i], errs[i] = printImage(paths[i], boxes[i].H)
			}
		}()
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return images, nil
}

// printImage decodes the image at path and returns it as a JPEG sized for a
// printed height of heightMM.
func printImage(path string, heightMM float64) ([]byte, error) {
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
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, fitImage(src, heightMM), &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("%s: encode: %w", name, err)
	}
	return buf.Bytes(), nil
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
