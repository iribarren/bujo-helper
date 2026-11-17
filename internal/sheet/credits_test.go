package sheet

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"bujo-helper/internal/media"
)

// writeCredit saves credit as the credit sidecar of the image at path.
func writeCredit(t *testing.T, path, credit string) {
	t.Helper()
	if err := os.WriteFile(media.CreditPath(path), []byte(credit), 0o644); err != nil {
		t.Fatal(err)
	}
}

// creditFixtures writes a book cover without a sidecar and two place photos
// with one, in that order.
func creditFixtures(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "Eiffel Tower - France.jpg", 100, 150, color.RGBA{120, 120, 160, 255}),
		writeImage(t, dir, "Dune - Frank Herbert.jpg", 66, 100, color.RGBA{200, 40, 40, 255}),
		writeImage(t, dir, "Flag of Japan.png", 150, 100, color.White),
	}
	writeCredit(t, paths[0], "Title: Tour Eiffel\nAuthor: Benh LIEU SONG\nLicence: public domain\n")
	writeCredit(t, paths[2], "Title: Flag of Japan\nAuthor: various\nLicence: public domain\nFile: https://commons.wikimedia.org/wiki/File:Flag_of_Japan.svg\n")
	return paths
}

func TestReadCredits(t *testing.T) {
	dir := t.TempDir()
	tower := writeImage(t, dir, "Eiffel Tower - France.jpg", 10, 15, color.White)
	book := writeImage(t, dir, "Dune - Frank Herbert.jpg", 10, 15, color.White)
	flag := writeImage(t, dir, "Flag of Japan-2.png", 15, 10, color.White)
	blank := writeImage(t, dir, "Blank.jpg", 10, 15, color.White)
	writeCredit(t, tower, "Author: Benh LIEU SONG\r\nLicence: public domain\n\n")
	writeCredit(t, flag, "Title: 日本の国旗\nAuthor: various")
	writeCredit(t, blank, " \n\t\n")

	tests := []struct {
		name  string
		paths []string
		want  []credit
	}{
		{"no sidecars", []string{book}, nil},
		{"only images with a sidecar, in sheet order", []string{flag, book, tower}, []credit{
			{Name: "Flag of Japan", Text: "Title: 日本の国旗\nAuthor: various"},
			{Name: "Eiffel Tower — France", Text: "Author: Benh LIEU SONG\nLicence: public domain"},
		}},
		{"a blank sidecar is no credit", []string{blank}, nil},
		{"an image selected twice is credited twice", []string{tower, tower}, []credit{
			{Name: "Eiffel Tower — France", Text: "Author: Benh LIEU SONG\nLicence: public domain"},
			{Name: "Eiffel Tower — France", Text: "Author: Benh LIEU SONG\nLicence: public domain"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readCredits(tt.paths)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readCredits = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestReadCreditsNamesAnUnreadableSidecar(t *testing.T) {
	dir := t.TempDir()
	img := writeImage(t, dir, "Colosseum - Italy.jpg", 10, 15, color.White)
	// A directory in place of the sidecar cannot be read as a file.
	if err := os.Mkdir(media.CreditPath(img), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := readCredits([]string{img})
	if err == nil || !strings.Contains(err.Error(), "Colosseum - Italy.jpg.credit.txt") {
		t.Errorf("err = %v, want one naming the sidecar", err)
	}
}

func TestLayoutCredits(t *testing.T) {
	a4 := Page{Width: 210, Height: 297}
	long := strings.Repeat("word ", 200)
	many := make([]credit, 40)
	for i := range many {
		many[i] = credit{Name: "Place", Text: "Author: someone\nLicence: CC BY-SA 4.0\nFile: https://commons.wikimedia.org/wiki/File:x.jpg"}
	}
	tests := []struct {
		name    string
		page    Page
		credits []credit
		pages   int
	}{
		{"one credit", a4, []credit{{Name: "Eiffel Tower — France", Text: "Licence: public domain"}}, 1},
		{"a long credit wraps", a4, []credit{{Name: "Long", Text: long}}, 1},
		{"many credits continue on more pages", a4, many, 3},
		{"many credits on A5", Page{Width: 148, Height: 210}, many, 5},
		{"a credit taller than a page splits", a4, []credit{{Name: "Huge", Text: strings.Repeat("line\n", 120)}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages := layoutCredits(tt.page, "Credits", tt.credits, fakeMeasure)
			if len(pages) != tt.pages {
				t.Fatalf("got %d pages, want %d", len(pages), tt.pages)
			}
			width := tt.page.Width - 2*creditsMargin
			bottom := tt.page.Height - creditsMargin
			var names []string
			for i, page := range pages {
				if len(page) < 2 || page[0].Text != "Credits" || !page[0].Bold {
					t.Fatalf("page %d does not start with the Credits heading: %+v", i+1, page)
				}
				for _, l := range page {
					if l.X < creditsMargin || l.X+fakeMeasure(l.Text, l.Bold, l.Size) > creditsMargin+width+fitTolerance {
						t.Errorf("page %d: line %q overflows the width", i+1, l.Text)
					}
					if l.Y < creditsMargin || l.Y+lineHeight(l.Size) > bottom+fitTolerance {
						t.Errorf("page %d: line %q at %.1f mm overflows the page", i+1, l.Text, l.Y)
					}
				}
				for _, l := range page[1:] {
					if l.Bold {
						names = append(names, l.Text)
					}
				}
			}
			// Every credit is numbered, in order.
			if len(names) != len(tt.credits) {
				t.Fatalf("got %d numbered names, want %d: %q", len(names), len(tt.credits), names)
			}
			for i, name := range names {
				if want := strings.TrimSpace(strings.SplitN(name, ".", 2)[0]); want != strconv.Itoa(i+1) {
					t.Errorf("name %d is %q, want it numbered %d", i+1, name, i+1)
				}
			}
		})
	}
}

func TestBuildAddsCreditsPages(t *testing.T) {
	paths := creditFixtures(t)
	for _, f := range Formats {
		t.Run(f.ID, func(t *testing.T) {
			opts := Options{Format: f.ID, Header: "Places"}
			off := len(pageObject.FindAll(render(t, paths, opts), -1))
			opts.Credits = true
			on := len(pageObject.FindAll(render(t, paths, opts), -1))
			if on != off+1 {
				t.Errorf("credits on: %d pages, want %d (one credits page after %d)", on, off+1, off)
			}
			if n, err := opts.CreditPages(paths); err != nil || n != 1 {
				t.Errorf("CreditPages = %d, %v; want 1", n, err)
			}
		})
	}
}

// With the box unchecked, or no sidecar among the selected images, every
// format renders exactly what it rendered before credits pages existed.
func TestBuildWithoutCreditsIsUnchanged(t *testing.T) {
	credited := creditFixtures(t)
	plain := cutFixtures(t)
	for _, f := range Formats {
		t.Run(f.ID, func(t *testing.T) {
			opts := Options{Format: f.ID, Header: "Books"}
			before := render(t, plain, opts)
			opts.Credits = true
			if !bytes.Equal(render(t, plain, opts), before) {
				t.Error("no sidecars: credits on changed the PDF")
			}
			if n, err := opts.CreditPages(plain); err != nil || n != 0 {
				t.Errorf("no sidecars: CreditPages = %d, %v; want 0", n, err)
			}
			opts.Credits = false
			if n, err := opts.CreditPages(credited); err != nil || n != 0 {
				t.Errorf("credits off: CreditPages = %d, %v; want 0", n, err)
			}
			want := len(pageObject.FindAll(render(t, credited[1:2], opts), -1))
			if got := len(pageObject.FindAll(render(t, credited[1:2], Options{Format: f.ID, Header: "Books", Credits: true}), -1)); got != want {
				t.Errorf("only an uncredited image: %d pages, want %d", got, want)
			}
		})
	}
}

func TestBuildCreditsPaginatesNonLatinAndLongCredits(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := range 30 {
		p := writeImage(t, dir, "Place "+strconv.Itoa(i+1)+" - Country.jpg", 30, 20, color.RGBA{uint8(i * 8), 90, 90, 255})
		paths = append(paths, p)
	}
	writeCredit(t, paths[0], "Title: 東京タワー\nAuthor: 山田太郎 😀 Ærøskøbing Łódź Ελλάδα Россия\nLicence: CC BY-SA 4.0")
	writeCredit(t, paths[1], "Credit: "+strings.Repeat("Supercalifragilistic ", 80)+strings.Repeat("x", 400))
	for _, p := range paths[2:] {
		writeCredit(t, p, "Author: someone\nLicence: CC BY 4.0\nLicence URL: https://creativecommons.org/licenses/by/4.0\nFile: https://commons.wikimedia.org/wiki/File:Example.jpg\nItem: https://www.wikidata.org/wiki/Q42")
	}
	opts := Options{Credits: true}
	n, err := opts.CreditPages(paths)
	if err != nil || n < 2 {
		t.Fatalf("CreditPages = %d, %v; want at least 2", n, err)
	}
	var out bytes.Buffer
	if err := Build(&out, paths, opts); err != nil {
		t.Fatal(err)
	}
	// 30 wide images take 3 cover sheet pages (10 a page).
	if got := len(pageObject.FindAll(out.Bytes(), -1)); got != 3+n {
		t.Errorf("PDF has %d pages, want 3 + %d credits pages", got, n)
	}
}

func TestBuildCreditsNamesAnUnreadableSidecarAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	img := writeImage(t, dir, "Colosseum - Italy.jpg", 66, 100, color.White)
	if err := os.Mkdir(filepath.Join(dir, "Colosseum - Italy.jpg"+media.CreditSuffix), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Build(&out, []string{img}, Options{Credits: true})
	if err == nil || !strings.Contains(err.Error(), "Colosseum - Italy.jpg.credit.txt") {
		t.Errorf("err = %v, want one naming the sidecar", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes, want none", out.Len())
	}
}

// Credits hold long unbroken URLs: they are split across lines within the
// width, and no character of an attribution is lost.
func TestLayoutCreditsSplitsUnbrokenTokens(t *testing.T) {
	a4 := Page{Width: 210, Height: 297}
	url := "https://commons.wikimedia.org/wiki/File:" + strings.Repeat("Very_long_file_name_", 20) + ".jpg"
	pages := layoutCredits(a4, "Credits", []credit{{Name: "Place", Text: "File page: " + url}}, fakeMeasure)
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	right := a4.Width - creditsMargin
	var text strings.Builder
	for _, l := range pages[0][1:] {
		if l.X+fakeMeasure(l.Text, l.Bold, l.Size) > right+fitTolerance {
			t.Errorf("line %q overflows the width", l.Text)
		}
		if !l.Bold {
			text.WriteString(l.Text)
		}
	}
	if got, want := text.String(), "File page:"+url; got != want {
		t.Errorf("credit text = %q, want %q", got, want)
	}
}

// A single credit line longer than maxCreditLines wrapped lines (only a
// hand-edited sidecar can be) is cut with an ellipsis, not dropped silently.
func TestLayoutCreditsMarksAnOverlongLine(t *testing.T) {
	a4 := Page{Width: 210, Height: 297}
	huge := strings.Repeat("word ", 200*maxCreditLines)
	pages := layoutCredits(a4, "Credits", []credit{{Name: "Place", Text: huge}}, fakeMeasure)
	var lines []creditLine
	for _, p := range pages {
		for _, l := range p[1:] {
			if !l.Bold {
				lines = append(lines, l)
			}
		}
	}
	if len(lines) != maxCreditLines {
		t.Fatalf("got %d credit lines, want %d", len(lines), maxCreditLines)
	}
	if last := lines[len(lines)-1].Text; !strings.HasSuffix(last, ellipsis) {
		t.Errorf("last line %q does not end with %q", last, ellipsis)
	}
}
