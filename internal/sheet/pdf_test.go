package sheet

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// writeImage saves a w x h image filled with c to dir/name, as PNG or JPEG
// depending on the extension.
func writeImage(t *testing.T, dir, name string, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	var err error
	if strings.HasSuffix(name, ".png") {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

var pageObject = regexp.MustCompile(`/Type /Page[^s]`)

func TestBuildWritesOnePagePerTwentyFiveCovers(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.RGBA{200, 40, 40, 255})
	paths := make([]string, 26)
	for i := range paths {
		paths[i] = cover
	}

	var out bytes.Buffer
	if err := Build(&out, paths, Options{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF")) {
		t.Fatalf("output starts with %q, want %%PDF", out.Bytes()[:min(8, out.Len())])
	}
	if n := len(pageObject.FindAll(out.Bytes(), -1)); n != 2 {
		t.Errorf("PDF has %d pages, want 2", n)
	}
	if !bytes.Contains(out.Bytes(), []byte("/MediaBox [0 0 595.28 841.89]")) {
		t.Error("PDF pages are not A4")
	}
}

func TestBuildPagesFollowTheSize(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.RGBA{40, 40, 200, 255})
	tests := []struct {
		size   string
		covers int
		pages  int
	}{
		{"mini", 81, 1},
		{"mini", 82, 2},
		{"standard", 26, 2},
		{"large", 9, 1},
		{"large", 10, 2},
	}
	for _, tt := range tests {
		paths := make([]string, tt.covers)
		for i := range paths {
			paths[i] = cover
		}
		var out bytes.Buffer
		if err := Build(&out, paths, Options{Size: tt.size}); err != nil {
			t.Fatalf("%s: %v", tt.size, err)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != tt.pages {
			t.Errorf("%s, %d covers: PDF has %d pages, want %d", tt.size, tt.covers, n, tt.pages)
		}
	}
}

func TestBuildRejectsUnknownSize(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.White)
	var out bytes.Buffer
	err := Build(&out, []string{cover}, Options{Size: "huge"})
	if err == nil || !strings.Contains(err.Error(), "huge") {
		t.Errorf("error %v, want one naming the size", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}

func TestBuildAcceptsPNGWithAlpha(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "alpha.png", 60, 90, color.NRGBA{0, 0, 255, 100}),
		writeImage(t, dir, "square.png", 50, 50, color.NRGBA{0, 255, 0, 255}),
	}
	var out bytes.Buffer
	if err := Build(&out, paths, Options{}); err != nil {
		t.Fatal(err)
	}
	if n := len(pageObject.FindAll(out.Bytes(), -1)); n != 1 {
		t.Errorf("PDF has %d pages, want 1", n)
	}
}

func TestBuildNamesTheBadFileAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	good := writeImage(t, dir, "good.jpg", 66, 100, color.White)

	corrupt := filepath.Join(dir, "corrupt.jpg")
	if err := os.WriteFile(corrupt, []byte("this is not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A JPEG cut short: its header decodes, its pixels do not.
	full, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	truncated := filepath.Join(dir, "truncated.jpg")
	if err := os.WriteFile(truncated, full[:len(full)/3], 0o644); err != nil {
		t.Fatal(err)
	}

	for _, bad := range []string{corrupt, truncated, filepath.Join(dir, "missing.jpg")} {
		var out bytes.Buffer
		err := Build(&out, []string{good, bad}, Options{})
		if err == nil {
			t.Errorf("%s: want an error", filepath.Base(bad))
			continue
		}
		if !strings.Contains(err.Error(), filepath.Base(bad)) {
			t.Errorf("error %q does not name %s", err, filepath.Base(bad))
		}
		if out.Len() != 0 {
			t.Errorf("%s: wrote %d bytes, want none", filepath.Base(bad), out.Len())
		}
	}
}

func TestBuildRejectsNoCovers(t *testing.T) {
	var out bytes.Buffer
	if err := Build(&out, nil, Options{}); err == nil {
		t.Fatal("want an error")
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}

func TestFitImageDownscalesTo300DPI(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3000, 4574))
	got := fitImage(src, 50)
	b := got.Bounds()
	// 50 mm at 300 dpi is 591 px.
	if b.Dy() != 591 || b.Dy() > 600 {
		t.Errorf("height = %d, want 591", b.Dy())
	}
	if want := 591 * 3000 / 4574; b.Dx() < want-1 || b.Dx() > want+1 {
		t.Errorf("width = %d, want about %d", b.Dx(), want)
	}
}

func TestFitImageNeverUpscales(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 150))
	if b := fitImage(src, 50).Bounds(); b.Dx() != 100 || b.Dy() != 150 {
		t.Errorf("size = %dx%d, want 100x150", b.Dx(), b.Dy())
	}
}

func TestFitImageFlattensAlphaOnWhite(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 10, 10)) // fully transparent
	got := fitImage(src, 50)
	r, g, b, a := got.At(5, 5).RGBA()
	if r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
		t.Errorf("transparent pixel became %v %v %v %v, want opaque white", r, g, b, a)
	}
}

func TestBuildPageFollowsThePaper(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.RGBA{40, 200, 40, 255})
	tests := []struct {
		paper    string
		covers   int
		pages    int
		mediaBox string
	}{
		{"", 25, 1, "/MediaBox [0 0 595.28 841.89]"},
		{"a4", 26, 2, "/MediaBox [0 0 595.28 841.89]"},
		{"letter", 20, 1, "/MediaBox [0 0 612.00 792.00]"},
		{"letter", 21, 2, "/MediaBox [0 0 612.00 792.00]"},
		{"a5", 9, 1, "/MediaBox [0 0 419.53 595.28]"},
		{"a5", 10, 2, "/MediaBox [0 0 419.53 595.28]"},
	}
	for _, tt := range tests {
		paths := make([]string, tt.covers)
		for i := range paths {
			paths[i] = cover
		}
		var out bytes.Buffer
		if err := Build(&out, paths, Options{Paper: tt.paper}); err != nil {
			t.Fatalf("%q: %v", tt.paper, err)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != tt.pages {
			t.Errorf("%q, %d covers: PDF has %d pages, want %d", tt.paper, tt.covers, n, tt.pages)
		}
		if !bytes.Contains(out.Bytes(), []byte(tt.mediaBox)) {
			t.Errorf("%q: PDF has no %s", tt.paper, tt.mediaBox)
		}
	}
}

func TestBuildRejectsUnknownPaper(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.White)
	var out bytes.Buffer
	err := Build(&out, []string{cover}, Options{Paper: "a3"})
	if err == nil || !strings.Contains(err.Error(), "a3") {
		t.Errorf("error %v, want one naming the paper", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}

func TestBuildCardsPages(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{200, 120, 40, 255})
	tests := []struct {
		paper    string
		cards    int
		pages    int
		mediaBox string
	}{
		{"", 8, 1, "/MediaBox [0 0 595.28 841.89]"},
		{"a4", 9, 2, "/MediaBox [0 0 595.28 841.89]"},
		{"letter", 8, 1, "/MediaBox [0 0 612.00 792.00]"},
		{"letter", 17, 3, "/MediaBox [0 0 612.00 792.00]"},
		{"a5", 3, 1, "/MediaBox [0 0 419.53 595.28]"},
		{"a5", 4, 2, "/MediaBox [0 0 419.53 595.28]"},
	}
	for _, tt := range tests {
		paths := make([]string, tt.cards)
		for i := range paths {
			paths[i] = cover
		}
		var out bytes.Buffer
		// The cover size does not apply to cards.
		if err := Build(&out, paths, Options{Format: "cards", Paper: tt.paper, Size: "large", DateLabel: "Read on"}); err != nil {
			t.Fatalf("%q: %v", tt.paper, err)
		}
		if !bytes.HasPrefix(out.Bytes(), []byte("%PDF")) {
			t.Fatalf("%q: output is not a PDF", tt.paper)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != tt.pages {
			t.Errorf("%q, %d cards: PDF has %d pages, want %d", tt.paper, tt.cards, n, tt.pages)
		}
		if !bytes.Contains(out.Bytes(), []byte(tt.mediaBox)) {
			t.Errorf("%q: PDF has no %s", tt.paper, tt.mediaBox)
		}
	}
}

func TestBuildCardsWithAccentsLongTitlesAndOddShapes(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "Cien años de soledad edición conmemorativa de la Real Academia Española - Gabriel García Márquez.jpg", 66, 100, color.RGBA{40, 120, 200, 255}),
		writeImage(t, dir, "Dune - 2021.jpg", 66, 100, color.RGBA{200, 160, 40, 255}),
		writeImage(t, dir, "Amélie-2.png", 80, 80, color.NRGBA{0, 200, 0, 120}),
		writeImage(t, dir, "Panorama - Ünïcödé Çréâtör Wïth À Vêry Löng Nämé Indeed.jpg", 300, 100, color.RGBA{90, 90, 90, 255}),
		writeImage(t, dir, "Supercalifragilisticexpialidociousandevenlongerthanthat.jpg", 30, 100, color.RGBA{10, 10, 10, 255}),
		writeImage(t, dir, ".jpg", 66, 100, color.White),
	}
	for _, label := range []string{"", "Watched on", "Leído el"} {
		var out bytes.Buffer
		if err := Build(&out, paths, Options{Format: "cards", DateLabel: label}); err != nil {
			t.Fatalf("label %q: %v", label, err)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != 1 {
			t.Errorf("label %q: PDF has %d pages, want 1", label, n)
		}
		// The text font is embedded, so accents print anywhere.
		if !bytes.Contains(out.Bytes(), []byte("/FontFile2")) {
			t.Errorf("label %q: PDF embeds no TrueType font", label)
		}
	}
}

func TestBuildCoverSheetHasNoText(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.White)
	for _, format := range []string{"", "covers"} {
		var out bytes.Buffer
		if err := Build(&out, []string{cover}, Options{Format: format}); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(out.Bytes(), []byte("/BaseFont")) {
			t.Errorf("format %q: cover sheet has fonts", format)
		}
	}
}

func TestBuildRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.White)
	var out bytes.Buffer
	err := Build(&out, []string{cover}, Options{Format: "poster"})
	if err == nil || !strings.Contains(err.Error(), "poster") {
		t.Errorf("error %v, want one naming the format", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}

func TestBuildCardsNamesTheBadFileAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	good := writeImage(t, dir, "Dune.jpg", 66, 100, color.White)
	bad := filepath.Join(dir, "Broken - Nobody.jpg")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Build(&out, []string{good, bad}, Options{Format: "cards"})
	if err == nil || !strings.Contains(err.Error(), "Broken - Nobody.jpg") {
		t.Errorf("error %v, want one naming the bad file", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}

func TestDateLabel(t *testing.T) {
	tests := map[string]string{"": "Date:", "  ": "Date:", "Read on": "Read on:", " Watched on ": "Watched on:"}
	for in, want := range tests {
		if got := (Options{DateLabel: in}).dateLabel(); got != want {
			t.Errorf("dateLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
