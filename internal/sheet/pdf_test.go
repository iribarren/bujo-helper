package sheet

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
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

func TestBuildSpinesPages(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{200, 120, 40, 255})
	tests := []struct {
		paper, size string
		spines      int
		pages       int
		mediaBox    string
	}{
		{"", "", 80, 1, "/MediaBox [0 0 595.28 841.89]"},
		{"a4", "standard", 81, 2, "/MediaBox [0 0 595.28 841.89]"},
		{"a4", "mini", 145, 2, "/MediaBox [0 0 595.28 841.89]"},
		{"a4", "large", 48, 1, "/MediaBox [0 0 595.28 841.89]"},
		{"letter", "standard", 65, 2, "/MediaBox [0 0 612.00 792.00]"},
		{"a5", "standard", 33, 1, "/MediaBox [0 0 419.53 595.28]"},
		{"a5", "large", 45, 3, "/MediaBox [0 0 419.53 595.28]"},
	}
	for _, tt := range tests {
		paths := make([]string, tt.spines)
		for i := range paths {
			paths[i] = cover
		}
		var out bytes.Buffer
		if err := Build(&out, paths, Options{Format: "spines", Paper: tt.paper, Size: tt.size}); err != nil {
			t.Fatalf("%q %q: %v", tt.paper, tt.size, err)
		}
		if !bytes.HasPrefix(out.Bytes(), []byte("%PDF")) {
			t.Fatalf("%q %q: output is not a PDF", tt.paper, tt.size)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != tt.pages {
			t.Errorf("%q %q, %d spines: PDF has %d pages, want %d", tt.paper, tt.size, tt.spines, n, tt.pages)
		}
		if !bytes.Contains(out.Bytes(), []byte(tt.mediaBox)) {
			t.Errorf("%q %q: PDF has no %s", tt.paper, tt.size, tt.mediaBox)
		}
	}
}

func TestBuildSpinesWithAccentsLongTitlesAndOddImages(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "Cien años de soledad edición conmemorativa de la Real Academia Española - Gabriel García Márquez.jpg", 66, 100, color.RGBA{40, 120, 200, 255}),
		writeImage(t, dir, "Dune - 2021.jpg", 66, 100, color.RGBA{200, 160, 40, 255}),
		writeImage(t, dir, "Amélie-2.png", 80, 80, color.NRGBA{0, 200, 0, 120}),
		writeImage(t, dir, "Panorama - Ünïcödé Çréâtör Wïth À Vêry Löng Nämé Indeed.jpg", 300, 100, color.RGBA{90, 90, 90, 255}),
		writeImage(t, dir, "Supercalifragilisticexpialidociousandevenlongerthanthat.jpg", 30, 100, color.RGBA{10, 10, 10, 255}),
		writeImage(t, dir, ".jpg", 66, 100, color.White),
	}
	for _, size := range []string{"mini", "standard", "large"} {
		var out bytes.Buffer
		if err := Build(&out, paths, Options{Format: "spines", Size: size}); err != nil {
			t.Fatalf("%s: %v", size, err)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != 1 {
			t.Errorf("%s: PDF has %d pages, want 1", size, n)
		}
		if !bytes.Contains(out.Bytes(), []byte("/FontFile2")) {
			t.Errorf("%s: PDF embeds no TrueType font", size)
		}
	}
}

// A spine is a coloured strip with text: it embeds no cover image.
func TestBuildSpinesHaveNoImages(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{10, 25, 90, 255})
	var out bytes.Buffer
	if err := Build(&out, []string{cover}, Options{Format: "spines"}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("/Subtype /Image")) {
		t.Error("spines PDF embeds an image")
	}
}

func TestBuildSpinesNamesTheBadFileAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	good := writeImage(t, dir, "Dune.jpg", 66, 100, color.White)
	// A PNG whose header reads but whose pixels do not.
	full := writeImage(t, dir, "full.png", 66, 100, color.White)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "Truncated - Nobody.png")
	if err := os.WriteFile(bad, data[:len(data)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Build(&out, []string{good, bad}, Options{Format: "spines"})
	if err == nil || !strings.Contains(err.Error(), "Truncated - Nobody.png") {
		t.Errorf("error %v, want one naming the bad file", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}

// render builds the PDF of paths with opts, uncompressed and with fixed
// dates, so its content can be inspected and compared.
func render(t *testing.T, paths []string, opts Options) []byte {
	t.Helper()
	pdf, err := build(paths, opts)
	if err != nil {
		t.Fatal(err)
	}
	pdf.SetCompression(false)
	pdf.SetCatalogSort(true)
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pdf.SetCreationDate(when)
	pdf.SetModificationDate(when)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pageContent matches an uncompressed page content stream: its dictionary
// has only a length, unlike images and fonts.
var pageContent = regexp.MustCompile(`(?s)obj\n<</Length \d+>>\nstream\n(.*?)endstream`)

// imageName matches an image reference, named after a hash of its data.
var imageName = regexp.MustCompile(`/I[0-9a-f]+ Do`)

// pageStreams returns the page content streams of an uncompressed PDF,
// joined, with image names replaced by "/I".
func pageStreams(pdf []byte) string {
	var b strings.Builder
	for _, m := range pageContent.FindAllSubmatch(pdf, -1) {
		b.Write(m[1])
	}
	return imageName.ReplaceAllString(b.String(), "/I Do")
}

// cutFixtures writes a typical, a square and a wide cover.
func cutFixtures(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	return []string{
		writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{200, 40, 40, 255}),
		writeImage(t, dir, "Square.png", 100, 100, color.RGBA{40, 200, 40, 255}),
		writeImage(t, dir, "Wide.jpg", 150, 100, color.RGBA{40, 40, 200, 255}),
	}
}

// The default cover sheet, and the Border cut mode, draw exactly what they
// drew before cut modes existed: each image, then its grey 0.2 mm border.
// The square cover has a 50 mm square cell, so the row of a 2:3, a square
// and a 2:3 cell is centred as if padded with another 2:3 cell: 20.6 mm in.
func TestBuildBorderCutKeepsTodaysCoverSheet(t *testing.T) {
	const today = "0 J\n0 j\n0.57 w\n0.600 G\n0.000 g\n" +
		"q 93.54331 0 0 141.73228 62.07874 656.22047 cm /I Do Q\n" +
		"62.08 797.95 93.54 -141.73 re S\n" +
		"q 141.73228 0 0 141.73228 170.64567 656.22047 cm /I Do Q\n" +
		"170.65 797.95 141.73 -141.73 re S\n" +
		"q 100.91339 0 0 67.27559 323.71654 693.44882 cm /I Do Q\n" +
		"323.72 760.72 100.91 -67.28 re S\n\n"
	paths := cutFixtures(t)
	for _, cut := range []string{"", "border"} {
		if got := pageStreams(render(t, paths, Options{Cut: cut})); got != today {
			t.Errorf("cut %q: page content\n%s\nwant\n%s", cut, got, today)
		}
	}
}

func TestBuildNoneCutDrawsImagesOnly(t *testing.T) {
	got := pageStreams(render(t, cutFixtures(t), Options{Cut: "none"}))
	if n := strings.Count(got, " Do Q"); n != 3 {
		t.Errorf("%d images drawn, want 3", n)
	}
	if strings.Contains(got, " re S") || strings.Contains(got, " l S") {
		t.Errorf("cut none draws lines:\n%s", got)
	}
}

func TestBuildBleedCutDrawsBleedAndCropMarks(t *testing.T) {
	paths := cutFixtures(t)
	got := pageStreams(render(t, paths, Options{Cut: "bleed"}))
	if strings.Contains(got, " re S") {
		t.Errorf("cut bleed draws a border:\n%s", got)
	}
	if n := strings.Count(got, " Do Q"); n != 3 {
		t.Errorf("%d images drawn, want 3", n)
	}
	if n := strings.Count(got, " l S"); n != 3*8 {
		t.Errorf("%d crop mark lines, want %d", n, 3*8)
	}
	// Thin black marks: 0.1 mm is 0.28 pt.
	if !strings.Contains(got, "0.000 G\n") || !strings.Contains(got, "0.28 w\n") {
		t.Errorf("crop marks are not thin black lines:\n%s", got)
	}
	// Each image is clipped to its bleed box, 1 mm past its cut line.
	g := standard(t)
	pages, err := g.Flow([]float64{0.66, 1, 1.5})
	if err != nil {
		t.Fatal(err)
	}
	const k = 72 / 25.4
	for _, b := range pages[0] {
		bl := bleedBox(b)
		clip := fmt.Sprintf("q %.2f %.2f %.2f %.2f re W n", bl.X*k, (g.Page.Height-bl.Y)*k, bl.W*k, -bl.H*k)
		if !strings.Contains(got, clip) {
			t.Errorf("no clip %q for box %+v in\n%s", clip, b, got)
		}
	}
}

func TestBuildBleedCutKeepsPageCount(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.RGBA{200, 40, 40, 255})
	paths := make([]string, 26)
	for i := range paths {
		paths[i] = cover
	}
	for _, c := range CutModes {
		var out bytes.Buffer
		if err := Build(&out, paths, Options{Cut: c.ID}); err != nil {
			t.Fatal(err)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != 2 {
			t.Errorf("cut %s: PDF has %d pages, want 2", c.ID, n)
		}
	}
}

// Review cards, spines and ticket stubs keep their borders whatever the
// cut mode.
func TestBuildCardsSpinesAndStubsIgnoreTheCutMode(t *testing.T) {
	paths := cutFixtures(t)
	for _, format := range []string{"cards", "spines", "stubs"} {
		want := pageStreams(render(t, paths, Options{Format: format}))
		// Cards stroke their border (S); spines fill and stroke it (B);
		// stubs close and stroke their notched outline (h S).
		if !strings.Contains(want, " re S") && !strings.Contains(want, " re B") && !strings.Contains(want, "h\nS\n") {
			t.Fatalf("%s: no border drawn", format)
		}
		for _, c := range CutModes {
			if got := pageStreams(render(t, paths, Options{Format: format, Cut: c.ID})); got != want {
				t.Errorf("%s, cut %s: page content differs from the default", format, c.ID)
			}
		}
	}
}

func TestBuildRejectsUnknownCutMode(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "cover.jpg", 66, 100, color.White)
	for _, format := range []string{"covers", "cards", "spines", "stubs"} {
		var out bytes.Buffer
		err := Build(&out, []string{cover}, Options{Format: format, Cut: "kiss"})
		if err == nil || !strings.Contains(err.Error(), "kiss") {
			t.Errorf("%s: error %v, want one naming the cut mode", format, err)
		}
		if out.Len() != 0 {
			t.Errorf("%s: wrote %d bytes, want none", format, out.Len())
		}
	}
}

// In bleed mode the prepared image is padded by its edge pixels; otherwise
// it is the fitted image as before.
func TestPrintImageBleedPadding(t *testing.T) {
	dir := t.TempDir()
	path := writeImage(t, dir, "cover.png", 66, 100, color.RGBA{200, 40, 40, 255})
	for _, bleed := range []bool{false, true} {
		p, err := printImage(path, 50, bleed)
		if err != nil {
			t.Fatal(err)
		}
		wantPad := 0
		if bleed {
			wantPad = bleedPad(100, 50)
		}
		if p.w != 66 || p.h != 100 || p.pad != wantPad {
			t.Errorf("bleed %v: printed %dx%d pad %d, want 66x100 pad %d", bleed, p.w, p.h, p.pad, wantPad)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(p.jpeg))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != 66+2*wantPad || cfg.Height != 100+2*wantPad {
			t.Errorf("bleed %v: JPEG %dx%d, want %dx%d", bleed, cfg.Width, cfg.Height, 66+2*wantPad, 100+2*wantPad)
		}
	}
}

func TestBuildStubsPages(t *testing.T) {
	dir := t.TempDir()
	cover := writeImage(t, dir, "Dune - 2021.jpg", 66, 100, color.RGBA{200, 120, 40, 255})
	tests := []struct {
		paper    string
		stubs    int
		pages    int
		mediaBox string
	}{
		{"", 12, 1, "/MediaBox [0 0 595.28 841.89]"},
		{"a4", 13, 2, "/MediaBox [0 0 595.28 841.89]"},
		{"letter", 12, 1, "/MediaBox [0 0 612.00 792.00]"},
		{"letter", 25, 3, "/MediaBox [0 0 612.00 792.00]"},
		{"a5", 4, 1, "/MediaBox [0 0 419.53 595.28]"},
		{"a5", 5, 2, "/MediaBox [0 0 419.53 595.28]"},
	}
	for _, tt := range tests {
		paths := make([]string, tt.stubs)
		for i := range paths {
			paths[i] = cover
		}
		var out bytes.Buffer
		// The cover size does not apply to stubs.
		if err := Build(&out, paths, Options{Format: "stubs", Paper: tt.paper, Size: "large"}); err != nil {
			t.Fatalf("%q: %v", tt.paper, err)
		}
		if !bytes.HasPrefix(out.Bytes(), []byte("%PDF")) {
			t.Fatalf("%q: output is not a PDF", tt.paper)
		}
		if n := len(pageObject.FindAll(out.Bytes(), -1)); n != tt.pages {
			t.Errorf("%q, %d stubs: PDF has %d pages, want %d", tt.paper, tt.stubs, n, tt.pages)
		}
		if !bytes.Contains(out.Bytes(), []byte(tt.mediaBox)) {
			t.Errorf("%q: PDF has no %s", tt.paper, tt.mediaBox)
		}
	}
}

func TestBuildStubsWithAccentsLongTitlesAndOddShapes(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "Cien años de soledad edición conmemorativa de la Real Academia Española - Gabriel García Márquez.jpg", 66, 100, color.RGBA{40, 120, 200, 255}),
		writeImage(t, dir, "Dune - 2021.jpg", 66, 100, color.RGBA{200, 160, 40, 255}),
		writeImage(t, dir, "Amélie-2.png", 80, 80, color.NRGBA{0, 200, 0, 120}),
		writeImage(t, dir, "Panorama - Ünïcödé Çréâtör Wïth À Vêry Löng Nämé Indeed.jpg", 300, 100, color.RGBA{90, 90, 90, 255}),
		writeImage(t, dir, "Supercalifragilisticexpialidociousandevenlongerthanthat.jpg", 30, 100, color.RGBA{10, 10, 10, 255}),
		writeImage(t, dir, ".jpg", 66, 100, color.White),
	}
	var out bytes.Buffer
	if err := Build(&out, paths, Options{Format: "stubs"}); err != nil {
		t.Fatal(err)
	}
	if n := len(pageObject.FindAll(out.Bytes(), -1)); n != 1 {
		t.Errorf("PDF has %d pages, want 1", n)
	}
	// The text font is embedded, so accents print anywhere.
	if !bytes.Contains(out.Bytes(), []byte("/FontFile2")) {
		t.Error("PDF embeds no TrueType font")
	}
}

// Each stub draws its poster, its notched outline, a dashed perforation and
// the rotated "ADMIT ONE"; the dash pattern is reset after each perforation.
func TestBuildStubsDrawOutlinePerforationAndStubEnd(t *testing.T) {
	paths := cutFixtures(t)
	got := pageStreams(render(t, paths, Options{Format: "stubs"}))
	n := len(paths)
	if c := strings.Count(got, " Do Q"); c != n {
		t.Errorf("%d images drawn, want %d", c, n)
	}
	if c := strings.Count(got, "h\nS\n"); c != n {
		t.Errorf("%d closed outlines, want %d", c, n)
	}
	// Each notch is a semicircle of Bézier curves.
	if c := strings.Count(got, " c\n"); c < 2*n*2 {
		t.Errorf("%d curve segments, want at least %d for the notches", c, 2*n*2)
	}
	if on, off := strings.Count(got, "] 0.00 d\n")-strings.Count(got, "[] 0.00 d\n"), strings.Count(got, "[] 0.00 d\n"); on != n || off != n {
		t.Errorf("%d dash patterns set and %d reset, want %d each", on, off, n)
	}
	if !strings.HasSuffix(strings.TrimSpace(got[:strings.LastIndex(got, "[] 0.00 d")]), "l S") {
		t.Errorf("the last dash pattern is not reset right after the perforation line:\n%s", got)
	}
	// The stub end text is turned a quarter turn: a rotation matrix with
	// cos 90° = 0 and sin 90° = 1.
	if c := strings.Count(got, "0.00000 1.00000 -1.00000 0.00000 "); c != n {
		t.Errorf("%d quarter-turn rotations, want %d", c, n)
	}
	if strings.Count(got, " Tj") < 3*n {
		t.Errorf("want title, creator, labels and admit text on each stub:\n%s", got)
	}
}

func TestBuildStubsNamesTheBadFileAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	good := writeImage(t, dir, "Dune.jpg", 66, 100, color.White)
	bad := filepath.Join(dir, "Broken - Nobody.jpg")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Build(&out, []string{good, bad}, Options{Format: "stubs"})
	if err == nil || !strings.Contains(err.Error(), "Broken - Nobody.jpg") {
		t.Errorf("error %v, want one naming the bad file", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}
