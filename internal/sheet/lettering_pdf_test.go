package sheet

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"
	"testing"
)

// trPairs checks the text rendering modes of one uncompressed page content
// stream: every "1 Tr" is followed by exactly one text object and then
// "0 Tr", and the stream ends in mode 0. It returns how many hollow texts
// it found.
func trPairs(t *testing.T, name, stream string) int {
	t.Helper()
	hollow, texts, pairs := false, 0, 0
	for _, line := range strings.Split(stream, "\n") {
		switch {
		case line == "1 Tr":
			if hollow {
				t.Errorf("%s: 1 Tr while already in mode 1", name)
			}
			hollow, texts = true, 0
		case line == "0 Tr":
			if !hollow {
				t.Errorf("%s: 0 Tr without a 1 Tr", name)
			}
			if texts != 1 {
				t.Errorf("%s: %d texts drawn in mode 1, want 1", name, texts)
			}
			hollow = false
			pairs++
		case strings.HasSuffix(line, " Tr"):
			t.Errorf("%s: unexpected %q", name, line)
		case strings.Contains(line, " Tj "):
			if hollow {
				texts++
			}
		}
	}
	if hollow {
		t.Errorf("%s: the stream ends in mode 1", name)
	}
	return pairs
}

// letteringPaths are covers for the lettering tests, one with a long title.
func letteringPaths(t *testing.T, n int) []string {
	t.Helper()
	dir := t.TempDir()
	cover := writeImage(t, dir, "Cien años de soledad - Gabriel García Márquez.jpg", 66, 100, color.RGBA{200, 40, 40, 255})
	paths := make([]string, n)
	for i := range paths {
		paths[i] = cover
	}
	return paths
}

func TestLetteringHollow(t *testing.T) {
	stroke, _ := LetteringFontByID("titan-one")
	none, _ := LetteringFontByID("sacramento")
	native, _ := LetteringFontByID("bungee-outline")
	tests := []struct {
		name string
		l    lettering
		pt   float64
		want bool
	}{
		{"stroke font, colour-in, large", lettering{stroke, true}, 20, true},
		{"stroke font, colour-in, at the threshold", lettering{stroke, true}, minHollowFontSize, true},
		{"stroke font, colour-in, small", lettering{stroke, true}, 13.9, false},
		{"stroke font, colour-in off", lettering{stroke, false}, 20, false},
		{"default font, colour-in", lettering{LetteringFont(""), true}, 20, true},
		{"too thin to outline", lettering{none, true}, 20, false},
		{"hollow by design", lettering{native, true}, 20, false},
	}
	for _, tt := range tests {
		if got := tt.l.hollow(tt.pt); got != tt.want {
			t.Errorf("%s: hollow(%v) = %v, want %v", tt.name, tt.pt, got, tt.want)
		}
	}
}

func TestHollowStrokeWidth(t *testing.T) {
	for _, tt := range []struct{ pt, want float64 }{
		{maxHeaderFontSize, 0.25},
		{14, minHollowStrokeWidth},
		{40, 0.5},
	} {
		if got := hollowStrokeWidth(tt.pt); !near(got, tt.want) {
			t.Errorf("hollowStrokeWidth(%v) = %v, want %v", tt.pt, got, tt.want)
		}
	}
}

// A header or title in a stroke font with Colour-in on is drawn stroke-only
// on every page, and the mode is back to 0 before any other text.
func TestHollowHeadersPairTr(t *testing.T) {
	tests := []struct {
		name  string
		pages int
		pdf   func(t *testing.T) []byte
	}{
		{"full page, two pages", 2, func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 61), Options{Format: "fullpage", Header: "Books of 2026", Font: "titan-one", ColourIn: true})
		}},
		{"checklist", 1, func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 3), Options{Format: "checklist", Header: "TBR 2026", Font: "pacifico", ColourIn: true})
		}},
		{"default font", 1, func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 1), Options{Format: "fullpage", Header: "Books of 2026", ColourIn: true})
		}},
		{"dot grid title", 1, func(t *testing.T) []byte {
			return renderPage(t, DotGridKind, PageSettings{Title: "Ideas 2026", Font: "bungee", ColourIn: true}, "")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streams := pageStreamsEach(tt.pdf(t))
			if len(streams) != tt.pages {
				t.Fatalf("%d page streams, want %d", len(streams), tt.pages)
			}
			for i, s := range streams {
				if n := trPairs(t, fmt.Sprintf("page %d", i+1), s); n != 1 {
					t.Errorf("page %d: %d hollow texts, want 1", i+1, n)
				}
			}
		})
	}
}

// The hollow helper puts back the line width and draw colour it changed, so
// the drawing after it (cut borders, rules) is unaffected.
func TestHollowTextRestoresState(t *testing.T) {
	pdf := newPDF(A4)
	f, _ := LetteringFontByID("titan-one")
	if err := addLetteringFont(pdf, f); err != nil {
		t.Fatal(err)
	}
	pdf.AddPage()
	pdf.SetLineWidth(0.7)
	pdf.SetDrawColor(10, 20, 30)
	l := lettering{f, true}
	l.setFont(pdf, 20)
	l.text(pdf, 10, 30, 20, "Hollow")
	if got := pdf.GetLineWidth(); got != 0.7 {
		t.Errorf("line width = %v, want 0.7", got)
	}
	if r, g, b := pdf.GetDrawColor(); r != 10 || g != 20 || b != 30 {
		t.Errorf("draw colour = %d %d %d, want 10 20 30", r, g, b)
	}
}

// Hollow-by-design and too-thin fonts, Colour-in off, and text below 14 pt
// are always filled.
func TestFilledLetteringHasNoTr(t *testing.T) {
	long := strings.Repeat("Watched in March ", 3)
	tests := []struct {
		name string
		pdf  func(t *testing.T) []byte
	}{
		{"native font", func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 2), Options{Format: "fullpage", Header: "Books", Font: "bungee-outline", ColourIn: true})
		}},
		{"thin font", func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 2), Options{Format: "checklist", Header: "Books", Font: "sacramento", ColourIn: true})
		}},
		{"colour-in off", func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 2), Options{Format: "fullpage", Header: "Books", Font: "titan-one"})
		}},
		{"review card titles are small", func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 2), Options{Format: "cards", Font: "titan-one", ColourIn: true})
		}},
		{"a long header shrinks below 14 pt", func(t *testing.T) []byte {
			return render(t, letteringPaths(t, 2), Options{Format: "fullpage", Paper: "a5", Header: long, Font: "titan-one", ColourIn: true})
		}},
		{"native dot grid title", func(t *testing.T) []byte {
			return renderPage(t, DotGridKind, PageSettings{Title: "Ideas", Font: "londrina-outline", ColourIn: true}, "")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if s := pageStreams(tt.pdf(t)); strings.Contains(s, " Tr") || strings.Contains(s, "\n1 Tr") {
				t.Errorf("filled lettering uses a text rendering mode:\n%s", s)
			}
		})
	}
	// The long header really is below the hollow threshold.
	pdf := newPDF(A5)
	f, _ := LetteringFontByID("titan-one")
	if err := addLetteringFont(pdf, f); err != nil {
		t.Fatal(err)
	}
	if line := fitHeader(pdf, lettering{f, true}, long, A5.Width-2*MinMargin); line.Size >= minHollowFontSize {
		t.Errorf("long header at %v pt, want it below %v pt", line.Size, minHollowFontSize)
	}
}

// In every font, short and long headers fit the usable width of A5 and A4,
// as measured in that font, within the header sizes; text too long even at
// the smallest size is cut with "…".
func TestHeaderFitsInEveryFont(t *testing.T) {
	long := strings.Repeat("Watched in March ", 12)
	word := "Supercalifragilisticexpialidociousandevenlongerthanthatbyquitealotmoreletters"
	texts := map[string]string{ // text: "whole", "cut", or "either" (depends on the font)
		"Books of 2026": "whole",
		"Libros leídos en 2026 · ¿Qué leí? ¡Año!": "whole",
		long: "cut",
		word: "either",
	}
	for _, f := range LetteringFonts {
		for _, page := range []Page{A5, A4} {
			pdf := newPDF(page)
			if err := addLetteringFont(pdf, f); err != nil {
				t.Fatal(err)
			}
			l := lettering{font: f}
			measure := l.measure(pdf)
			width := page.Width - 2*MinMargin
			for text, want := range texts {
				line := fitHeader(pdf, l, text, width)
				name := fmt.Sprintf("%s %v×%v %q", f.ID, page.Width, page.Height, text)
				if line.Text == "" {
					t.Errorf("%s: no header", name)
					continue
				}
				if w := measure(line.Text, true, line.Size); w > width+eps {
					t.Errorf("%s: %q is %v mm wide, room for %v", name, line.Text, w, width)
				}
				if line.Size < minHeaderFontSize || line.Size > maxHeaderFontSize {
					t.Errorf("%s: %v pt, want %v–%v", name, line.Size, minHeaderFontSize, maxHeaderFontSize)
				}
				cut := strings.HasSuffix(line.Text, ellipsis)
				if cut && line.Size != minHeaderFontSize {
					t.Errorf("%s: cut at %v pt, want %v", name, line.Size, minHeaderFontSize)
				}
				if !cut && line.Text != strings.Join(strings.Fields(text), " ") {
					t.Errorf("%s: %q is neither whole nor cut with %q", name, line.Text, ellipsis)
				}
				if (want == "whole" && cut) || (want == "cut" && !cut) {
					t.Errorf("%s: cut %v (%q), want %s", name, cut, line.Text, want)
				}
			}
		}
	}
}

// Every font prints review card titles and headers without an fpdf error.
func TestEveryFontRendersTitlesAndHeaders(t *testing.T) {
	paths := letteringPaths(t, 2)
	for _, f := range LetteringFonts {
		t.Run(f.ID, func(t *testing.T) {
			for _, opts := range []Options{
				{Format: "cards", Font: f.ID, ColourIn: true},
				{Format: "fullpage", Header: "Libros de 2026 ¿ñ? ¡ü!", Font: f.ID, ColourIn: true},
				{Format: "checklist", Header: "Pendientes · Ü", Font: f.ID},
			} {
				pdf, err := build(paths, opts)
				if err != nil {
					t.Fatalf("%s: %v", opts.Format, err)
				}
				var buf bytes.Buffer
				if err := pdf.Output(&buf); err != nil || pdf.Err() {
					t.Fatalf("%s: output %v, pdf error %v", opts.Format, err, pdf.Error())
				}
			}
			out := renderPage(t, DotGridKind, PageSettings{Title: "Ideas ¿ñ?", Font: f.ID, ColourIn: true}, "")
			if !bytes.HasPrefix(out, []byte("%PDF-")) {
				t.Error("dot grid: not a PDF")
			}
		})
	}
}

// The default font, empty, by id, or for an unknown id, with Colour-in off
// prints exactly what every format printed before lettering; formats that
// do not use lettering ignore the font and Colour-in altogether.
func TestDefaultLetteringIsByteIdentical(t *testing.T) {
	paths := creditFixtures(t)
	for _, f := range Formats {
		t.Run(f.ID, func(t *testing.T) {
			base := Options{Format: f.ID, Header: "Places of 2026", Credits: true}
			want := render(t, paths, base)
			for _, font := range []string{"go", "no-such-font"} {
				opts := base
				opts.Font = font
				if !bytes.Equal(render(t, paths, opts), want) {
					t.Errorf("font %q changed the PDF", font)
				}
			}
			if !f.UsesLettering {
				opts := base
				opts.Font, opts.ColourIn = "titan-one", true
				if !bytes.Equal(render(t, paths, opts), want) {
					t.Error("a format without lettering changed with a lettering font")
				}
			}
		})
	}
	for _, title := range []string{"", "Ideas 2026"} {
		want := renderPage(t, DotGridKind, PageSettings{Title: title}, "")
		if got := renderPage(t, DotGridKind, PageSettings{Title: title, Font: "go"}, ""); !bytes.Equal(got, want) {
			t.Errorf("dot grid %q: the explicit default font changed the PDF", title)
		}
	}
	// Without a title, the font and Colour-in have nothing to draw.
	if got := renderPage(t, DotGridKind, PageSettings{Font: "titan-one", ColourIn: true}, ""); !bytes.Equal(got, renderPage(t, DotGridKind, PageSettings{}, "")) {
		t.Error("dot grid without a title: a lettering font changed the PDF")
	}
}

func TestFormatsUsingLettering(t *testing.T) {
	want := map[string]bool{"fullpage": true, "checklist": true, "cards": true}
	for _, f := range Formats {
		if f.UsesLettering != want[f.ID] {
			t.Errorf("%s: UsesLettering = %v, want %v", f.ID, f.UsesLettering, want[f.ID])
		}
	}
}

func TestCreditFonts(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{"default font", Options{Format: "fullpage", Header: "Books"}, nil},
		{"full page with a header", Options{Format: "fullpage", Header: "Books", Font: "titan-one"}, []string{"titan-one"}},
		{"full page without a header", Options{Format: "fullpage", Header: "  ", Font: "titan-one"}, nil},
		{"checklist with a header", Options{Format: "checklist", Header: "TBR", Font: "pacifico"}, []string{"pacifico"}},
		{"review cards always print titles", Options{Format: "cards", Font: "luckiest-guy"}, []string{"luckiest-guy"}},
		{"a format without lettering", Options{Format: "spines", Header: "Books", Font: "titan-one"}, nil},
		{"an unknown font is the default", Options{Format: "cards", Font: "nope"}, nil},
	}
	for _, tt := range tests {
		var got []string
		for _, f := range tt.opts.creditFonts() {
			got = append(got, f.ID)
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("%s: creditFonts = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// The fonts section follows the credits: a bold heading, then the font and
// its licence, indented.
func TestLayoutCreditsListsFonts(t *testing.T) {
	titan, _ := LetteringFontByID("titan-one")
	credits := []credit{{Name: "Eiffel Tower — France", Text: "Licence: public domain"}}
	pages := layoutCredits(A4, "Credits", credits, creditFontsSection{"Fonts", []Font{titan}}, fakeMeasure)
	if len(pages) != 1 {
		t.Fatalf("%d pages, want 1", len(pages))
	}
	lines := pages[0]
	n := len(lines)
	if n < 2 || lines[n-2].Text != "Fonts" || !lines[n-2].Bold {
		t.Fatalf("no Fonts heading before the last line: %+v", lines)
	}
	if last := lines[n-1]; last.Text != "Titan One — SIL OFL 1.1" || last.Bold || last.X != creditsMargin+creditsIndent {
		t.Errorf("font line %+v", last)
	}
	without := layoutCredits(A4, "Credits", credits, creditFontsSection{}, fakeMeasure)
	if len(without[0]) != n-2 {
		t.Errorf("without fonts: %d lines, want %d", len(without[0]), n-2)
	}
}

// When the fonts section does not fit after the last credit, it starts a
// further credits page, and CreditPages counts it.
func TestCreditsFontsSectionCanAddAPage(t *testing.T) {
	dir := t.TempDir()
	img := writeImage(t, dir, "Eiffel Tower - France.jpg", 100, 150, color.RGBA{120, 120, 160, 255})
	writeCredit(t, img, "Title: Tour Eiffel\nAuthor: Benh LIEU SONG\nLicence: public domain\n")
	for _, paper := range Papers {
		for n := 1; n <= 80; n++ {
			if creditsFontsPage(t, img, paper.ID, n) {
				return
			}
		}
	}
	t.Fatal("no credit count pushes the fonts section onto a further page")
}

// creditsFontsPage checks the credits pages of n credits of img on review
// cards on paper, with and without a bundled font, and reports whether the
// fonts section took a further page.
func creditsFontsPage(t *testing.T, img, paper string, n int) bool {
	t.Helper()
	paths := make([]string, n)
	for i := range paths {
		paths[i] = img
	}
	plain := Options{Format: "cards", Paper: paper, Credits: true}
	lettered := Options{Format: "cards", Paper: paper, Credits: true, Font: "titan-one"}
	without, err := plain.CreditPages(paths)
	if err != nil {
		t.Fatal(err)
	}
	with, err := lettered.CreditPages(paths)
	if err != nil {
		t.Fatal(err)
	}
	if with < without || with > without+1 {
		t.Fatalf("%s, %d credits: %d pages with fonts, %d without", paper, n, with, without)
	}
	if with == without {
		return false
	}
	cardPages, err := Options{Format: "cards", Paper: paper}.PerPage()
	if err != nil {
		t.Fatal(err)
	}
	imagePages := (n + cardPages - 1) / cardPages
	if got := len(pageObject.FindAll(render(t, paths, lettered), -1)); got != imagePages+with {
		t.Errorf("%d credits: %d pages drawn, want %d + %d", n, got, imagePages, with)
	}
	if got := len(pageObject.FindAll(render(t, paths, plain), -1)); got != imagePages+without {
		t.Errorf("%d credits without fonts: %d pages drawn, want %d + %d", n, got, imagePages, without)
	}
	return true
}

// The fonts section never adds a credits page on its own: without Credits,
// or without a credited image, a bundled font adds nothing.
func TestCreditsFontsNeverAddAPage(t *testing.T) {
	credited := creditFixtures(t)
	plain := cutFixtures(t)
	for _, tt := range []struct {
		name  string
		paths []string
		opts  Options
	}{
		{"credits off", credited, Options{Format: "cards", Font: "titan-one"}},
		{"no credited image", plain, Options{Format: "cards", Font: "titan-one", Credits: true}},
	} {
		n, err := tt.opts.CreditPages(tt.paths)
		if err != nil || n != 0 {
			t.Errorf("%s: CreditPages = %d, %v; want 0", tt.name, n, err)
		}
		base := tt.opts
		base.Font = ""
		if got, want := len(pageObject.FindAll(render(t, tt.paths, tt.opts), -1)), len(pageObject.FindAll(render(t, tt.paths, base), -1)); got != want {
			t.Errorf("%s: %d pages, want %d", tt.name, got, want)
		}
	}
	// With a credited image, the section joins the credits page that is
	// printed anyway.
	opts := Options{Format: "fullpage", Header: "Places", Font: "titan-one", Credits: true}
	if n, err := opts.CreditPages(credited); err != nil || n != 1 {
		t.Errorf("CreditPages = %d, %v; want 1", n, err)
	}
	streams := pageStreamsEach(render(t, credited, opts))
	plainStreams := pageStreamsEach(render(t, credited, Options{Format: "fullpage", Header: "Places", Credits: true}))
	if len(streams) != len(plainStreams) {
		t.Fatalf("%d pages with a bundled font, %d without", len(streams), len(plainStreams))
	}
	last := len(streams) - 1
	if got, want := strings.Count(streams[last], " Tj "), strings.Count(plainStreams[last], " Tj ")+2; got != want {
		t.Errorf("credits page: %d texts, want %d (heading and font line added)", got, want)
	}
}

func TestFontsLabel(t *testing.T) {
	if got := (Options{}).labels().Fonts; got != "Fonts" {
		t.Errorf("default fonts label %q", got)
	}
	if got := (Options{Labels: spanishLabels}).labels().Fonts; got != "Fuentes" {
		t.Errorf("Spanish fonts label %q", got)
	}
}
