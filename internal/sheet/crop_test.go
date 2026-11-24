package sheet

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCropModeByID(t *testing.T) {
	if DefaultCrop != "none" || CropModes[0].ID != DefaultCrop {
		t.Errorf("DefaultCrop = %q, first crop mode %q; want none first", DefaultCrop, CropModes[0].ID)
	}
	tests := []struct {
		id, want, label   string
		usesSize, usesCut bool
		frames            int
	}{
		{"", "none", "None", true, true, 1},
		{"none", "none", "None", true, true, 1},
		{"instax", "instax", "Instant mini", false, false, 1},
		{"square", "square", "Square", true, true, 1},
		{"strip3", "strip3", "Photo strip (3 frames)", false, false, 3},
		{"strip4", "strip4", "Photo strip (4 frames)", false, false, 4},
		{"wallet", "wallet", "Wallet", false, true, 1},
	}
	for _, tt := range tests {
		c, err := CropModeByID(tt.id)
		if err != nil || c.ID != tt.want || c.Label != tt.label || c.UsesSize != tt.usesSize || c.UsesCut != tt.usesCut || c.Frames != tt.frames {
			t.Errorf("CropModeByID(%q) = %+v, %v; want %s (%q, uses size: %v, uses cut: %v, %d frames)",
				tt.id, c, err, tt.want, tt.label, tt.usesSize, tt.usesCut, tt.frames)
		}
	}
	if len(CropModes) != len(tests)-1 {
		t.Errorf("%d crop modes, want %d", len(CropModes), len(tests)-1)
	}
	if _, err := CropModeByID("panorama"); err == nil || !strings.Contains(err.Error(), "panorama") {
		t.Errorf(`CropModeByID("panorama"): error %v, want one naming the crop mode`, err)
	}
}

func TestCropModesHaveFileSafeTags(t *testing.T) {
	const safe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	seen := map[string]bool{}
	for _, c := range CropModes {
		if c.ID == "" || c.Label == "" || seen[c.ID] {
			t.Errorf("crop mode %+v: want a unique ID and a label", c)
		}
		seen[c.ID] = true
		if c.FileTag == "" || strings.Trim(c.FileTag, safe) != "" {
			t.Errorf("crop mode %q: file tag %q, want letters and digits only", c.ID, c.FileTag)
		}
	}
}

// Only the cover sheet crops; other formats ignore the crop mode.
func TestFormatUsesCrop(t *testing.T) {
	for _, f := range Formats {
		if want := f.ID == DefaultFormat; f.UsesCrop != want {
			t.Errorf("format %s: UsesCrop %v, want %v", f.ID, f.UsesCrop, want)
		}
	}
}

func TestCentreCrop(t *testing.T) {
	tests := []struct {
		name   string
		bounds image.Rectangle
		ratio  float64
		want   image.Rectangle
	}{
		{"wide to square", image.Rect(0, 0, 300, 100), 1, image.Rect(100, 0, 200, 100)},
		{"tall to square", image.Rect(0, 0, 100, 300), 1, image.Rect(0, 100, 100, 200)},
		{"exact", image.Rect(0, 0, 46, 62), 46.0 / 62, image.Rect(0, 0, 46, 62)},
		{"odd remainder", image.Rect(0, 0, 301, 100), 1, image.Rect(100, 0, 200, 100)},
		{"offset bounds", image.Rect(10, 20, 310, 120), 1, image.Rect(110, 20, 210, 120)},
		{"wide to instant mini", image.Rect(0, 0, 600, 400), 46.0 / 62, image.Rect(151, 0, 448, 400)},
		{"tall to a landscape frame", image.Rect(0, 0, 300, 600), 44.8 / 34.35, image.Rect(0, 185, 300, 415)},
		{"one pixel wide", image.Rect(0, 0, 1, 1000), 3, image.Rect(0, 499, 1, 500)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := centreCrop(tt.bounds, tt.ratio)
			if got != tt.want {
				t.Errorf("centreCrop(%v, %v) = %v, want %v", tt.bounds, tt.ratio, got, tt.want)
			}
		})
	}
}

// Whatever the shape of the image, the crop keeps its full width or full
// height, lies inside it, is centred to the pixel and has the target ratio
// to within a pixel.
func TestCentreCropProperties(t *testing.T) {
	sizes := []image.Point{{1, 1}, {66, 100}, {100, 100}, {150, 100}, {4000, 3000}, {3000, 4000}, {1080, 1920}, {7, 1000}}
	for _, m := range CropModes[1:] {
		for _, w := range cropWindows(m, 50) {
			ratio := w.W / w.H
			for _, s := range sizes {
				b := image.Rect(0, 0, s.X, s.Y)
				r := centreCrop(b, ratio)
				name := fmt.Sprintf("%s %v → ratio %.4f", m.ID, s, ratio)
				if !r.In(b) || r.Empty() {
					t.Errorf("%s: crop %v outside %v or empty", name, r, b)
					continue
				}
				if r.Dx() != b.Dx() && r.Dy() != b.Dy() {
					t.Errorf("%s: crop %v keeps neither the width nor the height", name, r)
				}
				if l, rr := r.Min.X-b.Min.X, b.Max.X-r.Max.X; l-rr > 1 || rr-l > 1 {
					t.Errorf("%s: crop %v not centred across", name, r)
				}
				if top, bottom := r.Min.Y-b.Min.Y, b.Max.Y-r.Max.Y; top-bottom > 1 || bottom-top > 1 {
					t.Errorf("%s: crop %v not centred down", name, r)
				}
				// Within a pixel of the ratio, unless the image is a
				// pixel thin.
				if r.Dx() > 1 && r.Dy() > 1 {
					if dw := math.Abs(float64(r.Dx()) - float64(r.Dy())*ratio); dw > 1 && math.Abs(float64(r.Dy())-float64(r.Dx())/ratio) > 1 {
						t.Errorf("%s: crop %v has ratio %v", name, r, float64(r.Dx())/float64(r.Dy()))
					}
				}
			}
		}
	}
}

// cropWindows returns the windows of the piece of crop mode m for a cover
// size of coverHeight, its frame at the origin.
func cropWindows(m CropMode, coverHeight float64) []Box {
	w, h := cropCellSize(m, coverHeight)
	return layoutCrop(m, Box{W: w, H: h}).Windows
}

func TestCropGeometry(t *testing.T) {
	stripW := 50.8 - 6
	tests := []struct {
		crop, size string
		w, h       float64 // the frame
		windows    []Box   // relative to the frame
	}{
		{"instax", "standard", 54, 86, []Box{{4, 7, 46, 62}}},
		{"instax", "large", 54, 86, []Box{{4, 7, 46, 62}}}, // size does not apply
		{"square", "mini", 25, 25, []Box{{0, 0, 25, 25}}},
		{"square", "standard", 50, 50, []Box{{0, 0, 50, 50}}},
		{"square", "large", 80, 80, []Box{{0, 0, 80, 80}}},
		{"wallet", "standard", 64, 89, []Box{{0, 0, 64, 89}}},
		{"wallet", "mini", 64, 89, []Box{{0, 0, 64, 89}}},
		{"strip3", "standard", 50.8, 152.4, []Box{
			{3, 3, stripW, 46.8}, {3, 52.8, stripW, 46.8}, {3, 102.6, stripW, 46.8}}},
		{"strip4", "standard", 50.8, 152.4, []Box{
			{3, 3, stripW, 34.35}, {3, 40.35, stripW, 34.35}, {3, 77.7, stripW, 34.35}, {3, 115.05, stripW, 34.35}}},
	}
	for _, tt := range tests {
		t.Run(tt.crop+"/"+tt.size, func(t *testing.T) {
			m, err := CropModeByID(tt.crop)
			if err != nil {
				t.Fatal(err)
			}
			s, err := SizeByID(tt.size)
			if err != nil {
				t.Fatal(err)
			}
			w, h := cropCellSize(m, s.CoverHeight)
			if !near(w, tt.w) || !near(h, tt.h) {
				t.Errorf("frame %v × %v, want %v × %v", w, h, tt.w, tt.h)
			}
			frame := Box{X: 10, Y: 20, W: w, H: h}
			p := layoutCrop(m, frame)
			if p.Frame != frame {
				t.Errorf("frame %+v, want the cell %+v", p.Frame, frame)
			}
			if len(p.Windows) != len(tt.windows) || len(p.Windows) != m.Frames {
				t.Fatalf("%d windows %+v, want %d", len(p.Windows), p.Windows, len(tt.windows))
			}
			for i, want := range tt.windows {
				want.X += 10
				want.Y += 20
				if got := p.Windows[i]; !near(got.X, want.X) || !near(got.Y, want.Y) || !near(got.W, want.W) || !near(got.H, want.H) {
					t.Errorf("window %d %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

// The frames of a strip are equal, inside it, 3 mm from its edges and 3 mm
// apart, and fill it from top to bottom.
func TestStripFramesInsideTheStrip(t *testing.T) {
	for _, id := range []string{"strip3", "strip4"} {
		m, _ := CropModeByID(id)
		w, h := cropCellSize(m, 50)
		strip := Box{X: 7, Y: 11, W: w, H: h}
		ws := layoutCrop(m, strip).Windows
		for i, b := range ws {
			if !inside(b, strip) {
				t.Errorf("%s: frame %d %+v outside the strip %+v", id, i, b, strip)
			}
			if !near(b.W, ws[0].W) || !near(b.H, ws[0].H) {
				t.Errorf("%s: frame %d %+v, want the size of frame 0 %+v", id, i, b, ws[0])
			}
			if !near(b.X-strip.X, 3) || !near(strip.X+strip.W-(b.X+b.W), 3) {
				t.Errorf("%s: frame %d %+v, want 3 mm side borders", id, i, b)
			}
			if i > 0 {
				if overlap(b, ws[i-1]) || !near(b.Y-(ws[i-1].Y+ws[i-1].H), 3) {
					t.Errorf("%s: frame %d %+v, want 3 mm under frame %d %+v", id, i, b, i-1, ws[i-1])
				}
			}
		}
		last := ws[len(ws)-1]
		if !near(ws[0].Y-strip.Y, 3) || !near(strip.Y+strip.H-(last.Y+last.H), 3) {
			t.Errorf("%s: frames %+v, want 3 mm top and bottom borders", id, ws)
		}
	}
}

func TestCropGridPapers(t *testing.T) {
	tests := []struct {
		crop, paper, size string
		cols, rows        int
		perPage           int // photos
	}{
		{"instax", "a4", "", 3, 3, 9},
		{"instax", "letter", "", 3, 2, 6},
		{"instax", "a5", "", 2, 2, 4},
		{"wallet", "a4", "", 2, 3, 6},
		{"wallet", "letter", "", 2, 2, 4},
		{"wallet", "a5", "", 2, 2, 4}, // two wallets fill the width exactly
		{"strip3", "a4", "", 3, 1, 9},
		{"strip3", "letter", "", 3, 1, 9},
		{"strip3", "a5", "", 2, 1, 6},
		{"strip4", "a4", "", 3, 1, 12},
		{"strip4", "letter", "", 3, 1, 12},
		{"strip4", "a5", "", 2, 1, 8},
		{"square", "a4", "mini", 6, 9, 54},
		{"square", "a4", "standard", 3, 5, 15},
		{"square", "a4", "", 3, 5, 15},
		{"square", "a4", "large", 2, 3, 6},
		{"square", "letter", "mini", 7, 9, 63},
		{"square", "letter", "standard", 3, 4, 12},
		{"square", "letter", "large", 2, 3, 6},
		{"square", "a5", "mini", 4, 6, 24},
		{"square", "a5", "standard", 2, 3, 6},
		{"square", "a5", "large", 1, 2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.crop+"/"+tt.paper+"/"+tt.size, func(t *testing.T) {
			opts := Options{Crop: tt.crop, Paper: tt.paper, Size: tt.size}
			g, err := opts.Grid()
			if err != nil {
				t.Fatal(err)
			}
			m, _ := CropModeByID(tt.crop)
			s, _ := SizeByID(tt.size)
			p, _ := PaperByID(tt.paper)
			w, h := cropCellSize(m, s.CoverHeight)
			if g.Page != p.Page || g.Columns != tt.cols || g.Rows != tt.rows || !near(g.CellWidth, w) || !near(g.CoverHeight, h) {
				t.Errorf("grid %+v, want %d × %d of %v × %v", g, tt.cols, tt.rows, w, h)
			}
			if n, err := opts.PerPage(); err != nil || n != tt.perPage {
				t.Errorf("PerPage() = %d, %v; want %d", n, err, tt.perPage)
			}
			for _, r := range []float64{0.3, 2.0 / 3, 1, 3} {
				if n, err := opts.PerPageFor(r); err != nil || n != tt.perPage {
					t.Errorf("PerPageFor(%v) = %d, %v; want %d whatever the shape", r, n, err, tt.perPage)
				}
			}
			if g.MarginX < MinMargin-eps || g.MarginY < MinMargin-eps {
				t.Errorf("margins %v / %v below %v", g.MarginX, g.MarginY, MinMargin)
			}
			pages, err := g.Cells(g.PerPage())
			if err != nil {
				t.Fatal(err)
			}
			last := pages[0][len(pages[0])-1]
			if right, bottom := p.Page.Width-(last.X+last.W), p.Page.Height-(last.Y+last.H); !near(right, g.MarginX) || !near(bottom, g.MarginY) {
				t.Errorf("right / bottom margins %v / %v, want %v / %v", right, bottom, g.MarginX, g.MarginY)
			}
		})
	}
}

// Page counts follow the photos per page, whatever their shape; a strip
// mode fills strips with consecutive photos.
func TestCropPages(t *testing.T) {
	tests := []struct {
		crop   string
		photos int
		pages  int
	}{
		{"instax", 9, 1},
		{"instax", 10, 2},
		{"strip3", 9, 1},
		{"strip3", 10, 2},
		{"strip4", 13, 2},
		{"square", 15, 1},
		{"square", 16, 2},
		{"wallet", 13, 3},
	}
	for _, tt := range tests {
		ratios := make([]float64, tt.photos)
		for i := range ratios {
			ratios[i] = []float64{0.3, 1, 3, 2.0 / 3}[i%4]
		}
		if n, err := (Options{Crop: tt.crop}).Pages(ratios); err != nil || n != tt.pages {
			t.Errorf("%s, %d photos: Pages = %d, %v; want %d", tt.crop, tt.photos, n, err, tt.pages)
		}
	}
}

// Other formats ignore the crop mode, and None keeps the cover sheet grid.
func TestCropModeIgnoredWhereItDoesNotApply(t *testing.T) {
	for _, f := range Formats {
		for _, p := range Papers {
			base := Options{Format: f.ID, Paper: p.ID}
			want, err := base.Grid()
			if err != nil {
				t.Fatal(err)
			}
			wantN, _ := base.PerPage()
			for _, c := range CropModes {
				if f.UsesCrop && c.ID != DefaultCrop {
					continue
				}
				opts := base
				opts.Crop = c.ID
				got, err := opts.Grid()
				n, _ := opts.PerPage()
				if err != nil || got != want || n != wantN {
					t.Errorf("%s %s, crop %s: grid %+v (%d per page), %v; want %+v (%d)", f.ID, p.ID, c.ID, got, n, err, want, wantN)
				}
			}
		}
	}
}

func TestOptionsRejectUnknownCropMode(t *testing.T) {
	for _, f := range Formats {
		if _, err := (Options{Format: f.ID, Crop: "panorama"}).Grid(); err == nil || !strings.Contains(err.Error(), "panorama") {
			t.Errorf(`%s: Grid() with crop "panorama": error %v, want one naming the crop mode`, f.ID, err)
		}
	}
	if _, err := (Options{Crop: "panorama"}).PerPage(); err == nil {
		t.Error(`PerPage() with crop "panorama": want an error`)
	}
	var out bytes.Buffer
	if err := Build(&out, cutFixtures(t), Options{Crop: "panorama"}); err == nil || out.Len() != 0 {
		t.Errorf(`Build with crop "panorama": error %v, %d bytes; want an error and nothing written`, err, out.Len())
	}
}

// writeThirds saves a w × h JPEG split in three bands, left to right if
// across, else top to bottom: red, green and blue.
func writeThirds(t *testing.T, dir, name string, w, h int, across bool) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bands := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}}
	for y := range h {
		for x := range w {
			band := 3 * y / h
			if across {
				band = 3 * x / w
			}
			img.SetRGBA(x, y, bands[band])
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// greenish reports whether c is mostly green.
func greenish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return g > 0xa000 && r < 0x6000 && b < 0x6000
}

// A cropped image has the window's shape, so it fills it with no
// distortion, and it is the middle of the source.
func TestPrintCroppedImage(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name       string
		path       string
		window     Box
		w, h       int  // the printed pixels
		middleOnly bool // only the green middle band is left
	}{
		{"wide to square", writeThirds(t, dir, "wide.jpg", 300, 100, true), Box{W: 10, H: 10}, 100, 100, true},
		{"tall to square", writeThirds(t, dir, "tall.jpg", 100, 300, false), Box{W: 10, H: 10}, 100, 100, true},
		{"square to instant mini", writeThirds(t, dir, "square.jpg", 400, 400, true), Box{W: 46, H: 62}, 297, 400, false},
		{"exact", writeThirds(t, dir, "exact.jpg", 460, 620, true), Box{W: 46, H: 62}, 460, 620, false},
		// 1000 × 1000 crop, downscaled to 10 mm at 300 dpi.
		{"downscaled", writeThirds(t, dir, "big.jpg", 3000, 1000, true), Box{W: 10, H: 10}, 118, 118, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := printCroppedImage(tt.path, tt.window, false)
			if err != nil {
				t.Fatal(err)
			}
			if p.w != tt.w || p.h != tt.h || p.pad != 0 {
				t.Errorf("printed %d × %d (pad %d), want %d × %d", p.w, p.h, p.pad, tt.w, tt.h)
			}
			if ratio := tt.window.W / tt.window.H; math.Abs(float64(p.w)-float64(p.h)*ratio) > 1 {
				t.Errorf("printed %d × %d, not the window's %v ratio", p.w, p.h, ratio)
			}
			img, err := jpeg.Decode(bytes.NewReader(p.jpeg))
			if err != nil {
				t.Fatal(err)
			}
			if b := img.Bounds(); b.Dx() != p.w || b.Dy() != p.h {
				t.Errorf("JPEG %v, want %d × %d", b, p.w, p.h)
			}
			b := img.Bounds()
			if !greenish(img.At(b.Dx()/2, b.Dy()/2)) {
				t.Errorf("centre %v, want the green middle band", img.At(b.Dx()/2, b.Dy()/2))
			}
			corners := []image.Point{{2, 2}, {b.Dx() - 3, 2}, {2, b.Dy() - 3}, {b.Dx() - 3, b.Dy() - 3}}
			for _, c := range corners {
				if got := greenish(img.At(c.X, c.Y)); got != tt.middleOnly {
					t.Errorf("corner %v %v green: %v, want %v", c, img.At(c.X, c.Y), got, tt.middleOnly)
				}
			}
		})
	}
	// With bleed, the edge is repeated past the window.
	p, err := printCroppedImage(tests[0].path, Box{W: 10, H: 10}, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.w != 100 || p.h != 100 || p.pad != bleedPad(100, 10) {
		t.Errorf("with bleed: %d × %d, pad %d; want 100 × 100, pad %d", p.w, p.h, p.pad, bleedPad(100, 10))
	}
	// A crop window whose rounded width has more pixels per mm across than
	// down still gets its full bleed at the sides.
	window := Box{X: 10, Y: 10, W: 33.25, H: 50}
	p, err = printCroppedImage(writeThirds(t, dir, "rounded.jpg", 100, 100, true), window, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.w != 67 || p.h != 100 {
		t.Errorf("rounded window: %d × %d, want 67 × 100", p.w, p.h)
	}
	if at, bl := paddedBox(window, p.w, p.h, p.pad), bleedBox(window); at.X > bl.X+eps || at.Y > bl.Y+eps || at.X+at.W < bl.X+bl.W-eps || at.Y+at.H < bl.Y+bl.H-eps {
		t.Errorf("rounded window, pad %d: image drawn on %+v does not cover the bleed box %+v", p.pad, at, bl)
	}
	if _, err := printCroppedImage(filepath.Join(dir, "missing.jpg"), Box{W: 10, H: 10}, false); err == nil || !strings.Contains(err.Error(), "missing.jpg") {
		t.Errorf("missing file: error %v, want one naming it", err)
	}
}

// cropFixtures writes photos of every shape: portrait, square, landscape,
// panoramic and very tall.
func cropFixtures(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	return []string{
		writeImage(t, dir, "Portrait.jpg", 66, 100, color.RGBA{200, 40, 40, 255}),
		writeImage(t, dir, "Square.png", 100, 100, color.RGBA{40, 200, 40, 255}),
		writeImage(t, dir, "Wide.jpg", 150, 100, color.RGBA{40, 40, 200, 255}),
		writeImage(t, dir, "Panorama.jpg", 300, 100, color.RGBA{40, 120, 200, 255}),
		writeImage(t, dir, "Tall.png", 30, 100, color.RGBA{10, 10, 10, 255}),
	}
}

// mmToPt converts mm to PDF points.
const mmToPt = 72 / 25.4

// imageAt is how fpdf draws an image in box b on page.
func imageAt(page Page, b Box) string {
	return fmt.Sprintf("q %.5f 0 0 %.5f %.5f %.5f cm /I Do Q", b.W*mmToPt, b.H*mmToPt, b.X*mmToPt, (page.Height-b.Y-b.H)*mmToPt)
}

// rectAt is how fpdf outlines box b on page.
func rectAt(page Page, b Box) string {
	return fmt.Sprintf("%.2f %.2f %.2f %.2f re S", b.X*mmToPt, (page.Height-b.Y)*mmToPt, b.W*mmToPt, -b.H*mmToPt)
}

// Each photo fills its window exactly; framed pieces get a grey frame
// border and pieces that fill their cell follow the cut mode. A strip
// takes consecutive photos, the last one maybe partial.
func TestBuildCropDrawsTheLayout(t *testing.T) {
	fixtures := cropFixtures(t)
	tests := []struct {
		crop, paper, size string
		photos            int
	}{
		{"instax", "a4", "", 9},
		{"instax", "letter", "", 5},
		{"square", "a4", "standard", 15},
		{"square", "a5", "large", 2},
		{"square", "letter", "mini", 11},
		{"wallet", "a4", "", 6},
		{"wallet", "a5", "", 3},
		{"strip3", "a4", "", 7}, // the third strip holds one photo
		{"strip4", "a5", "", 8},
		{"strip4", "letter", "", 6},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s/%s/%d", tt.crop, tt.paper, tt.size, tt.photos), func(t *testing.T) {
			opts := Options{Crop: tt.crop, Paper: tt.paper, Size: tt.size}
			paths := make([]string, tt.photos)
			for i := range paths {
				paths[i] = fixtures[i%len(fixtures)]
			}
			got := pageStreams(render(t, paths, opts))
			g, err := opts.Grid()
			if err != nil {
				t.Fatal(err)
			}
			m, _ := CropModeByID(tt.crop)
			pieces := (tt.photos + m.Frames - 1) / m.Frames
			cells, err := g.Cells(pieces)
			if err != nil || len(cells) != 1 {
				t.Fatalf("%d pages of cells, %v; want one", len(cells), err)
			}
			var windows []Box
			for _, c := range cells[0] {
				p := layoutCrop(m, c)
				if m.UsesCut {
					continue
				}
				if r := rectAt(g.Page, p.Frame); !strings.Contains(got, r) {
					t.Errorf("no frame %q", r)
				}
				windows = append(windows, p.Windows...)
			}
			if m.UsesCut {
				for _, c := range cells[0] {
					windows = append(windows, layoutCrop(m, c).Windows...)
					if r := rectAt(g.Page, c); !strings.Contains(got, r) {
						t.Errorf("no cut border %q", r)
					}
				}
			}
			windows = windows[:tt.photos]
			for _, w := range windows {
				if img := imageAt(g.Page, w); !strings.Contains(got, img) {
					t.Errorf("no image filling the window: %q", img)
				}
			}
			if n := strings.Count(got, " Do Q"); n != tt.photos {
				t.Errorf("%d images, want %d", n, tt.photos)
			}
			boxes := pieces
			if m.UsesCut {
				boxes = tt.photos
			}
			if n := strings.Count(got, " re S"); n != boxes {
				t.Errorf("%d boxes, want %d", n, boxes)
			}
			if strings.Contains(got, " l S") || strings.Contains(got, " Tj") {
				t.Errorf("draws lines or text:\n%s", got)
			}
			if !strings.Contains(got, "0.600 G\n") || !strings.Contains(got, "0.57 w\n") {
				t.Errorf("borders are not the grey 0.2 mm cut line:\n%s", got)
			}
		})
	}
}

func TestBuildCropPages(t *testing.T) {
	fixtures := cropFixtures(t)
	for _, m := range CropModes[1:] {
		for _, p := range Papers {
			opts := Options{Crop: m.ID, Paper: p.ID}
			perPage, err := opts.PerPage()
			if err != nil {
				t.Fatal(err)
			}
			for _, photos := range []int{1, perPage, perPage + 1, 2*perPage + m.Frames + 1} {
				paths := make([]string, photos)
				for i := range paths {
					paths[i] = fixtures[i%len(fixtures)]
				}
				pdf := render(t, paths, opts)
				want := (photos + perPage - 1) / perPage
				if n := len(pageObject.FindAll(pdf, -1)); n != want {
					t.Errorf("%s %s, %d photos: %d pages, want %d", m.ID, p.ID, photos, n, want)
				}
				streams := pageStreamsEach(pdf)
				for i, s := range streams {
					if n, want := strings.Count(s, " Do Q"), min(perPage, photos-i*perPage); n != want {
						t.Errorf("%s %s, %d photos, page %d: %d images, want %d", m.ID, p.ID, photos, i+1, n, want)
					}
				}
			}
		}
	}
}

// The framed modes ignore the cut mode and the size; the wallet ignores
// the size.
func TestBuildCropIgnoresWhatDoesNotApply(t *testing.T) {
	paths := cropFixtures(t)
	for _, m := range CropModes[1:] {
		want := pageStreams(render(t, paths, Options{Crop: m.ID}))
		for _, c := range CutModes {
			for _, s := range Sizes {
				if m.UsesCut && c.ID != DefaultCut || m.UsesSize && s.ID != DefaultSize {
					continue
				}
				opts := Options{Crop: m.ID, Cut: c.ID, Size: s.ID, Header: "Photos"}
				if got := pageStreams(render(t, paths, opts)); got != want {
					t.Errorf("%+v: page content differs from the default", opts)
				}
			}
		}
	}
}

// With no cut line, square and wallet photos are drawn alone; with bleed,
// each runs Bleed past its cell, clipped there, with crop marks.
func TestBuildCropCutModes(t *testing.T) {
	paths := cropFixtures(t)
	for _, id := range []string{"square", "wallet"} {
		none := pageStreams(render(t, paths, Options{Crop: id, Cut: "none"}))
		if n := strings.Count(none, " Do Q"); n != len(paths) || strings.Contains(none, " re S") || strings.Contains(none, " l S") {
			t.Errorf("%s, cut none: %d images, want %d and no lines:\n%s", id, n, len(paths), none)
		}
		opts := Options{Crop: id, Cut: "bleed"}
		bleed := pageStreams(render(t, paths, opts))
		if strings.Contains(bleed, " re S") {
			t.Errorf("%s, bleed: draws a border", id)
		}
		if n := strings.Count(bleed, " l S"); n != 8*len(paths) {
			t.Errorf("%s, bleed: %d crop mark lines, want %d", id, n, 8*len(paths))
		}
		g, _ := opts.Grid()
		cells, _ := g.Cells(len(paths))
		for _, c := range cells[0] {
			bl := bleedBox(c)
			clip := fmt.Sprintf("%.2f %.2f %.2f %.2f re W n", bl.X*mmToPt, (g.Page.Height-bl.Y)*mmToPt, bl.W*mmToPt, -bl.H*mmToPt)
			if !strings.Contains(bleed, clip) {
				t.Errorf("%s, bleed: no clip %q", id, clip)
			}
		}
	}
}

// Crop None is today's cover sheet, for every paper, size and cut mode.
func TestBuildCropNoneKeepsTheCoverSheet(t *testing.T) {
	paths := cutFixtures(t)
	for _, p := range Papers {
		for _, s := range Sizes {
			for _, c := range CutModes {
				base := Options{Paper: p.ID, Size: s.ID, Cut: c.ID}
				want := pageStreams(render(t, paths, base))
				opts := base
				opts.Crop = "none"
				if got := pageStreams(render(t, paths, opts)); got != want {
					t.Errorf("%+v: page content differs from no crop", opts)
				}
			}
		}
	}
}

// Credits pages still follow cropped photos.
func TestBuildCropKeepsCredits(t *testing.T) {
	dir := t.TempDir()
	photo := writeImage(t, dir, "Photo.jpg", 150, 100, color.RGBA{40, 40, 200, 255})
	if err := os.WriteFile(photo+".credit.txt", []byte("Photo by someone, CC BY 4.0"), 0o644); err != nil {
		t.Fatal(err)
	}
	pdf := render(t, []string{photo}, Options{Crop: "instax", Credits: true})
	if n := len(pageObject.FindAll(pdf, -1)); n != 2 {
		t.Errorf("%d pages, want the photo page and a credits page", n)
	}
}
