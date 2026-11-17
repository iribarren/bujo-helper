package sheet

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"

	"bujo-helper/internal/media"
)

// Credits page settings. Some sources (Wikimedia Commons CC BY and BY-SA
// images, for example) require the author and licence wherever the image
// is reproduced, so a sheet can end with the credits the downloader saved
// next to its images.
const (
	creditsText      = "Credits" // default heading; Options.Labels can replace it
	creditsMargin    = 15.0      // page margin, mm
	creditsTitleSize = 16.0      // heading, pt
	creditsTitleGap  = 4.0       // below the heading, mm
	creditsNameSize  = 10.0      // an image's number and name, pt
	creditsTextSize  = 8.0       // credit text, pt
	creditsIndent    = 6.0       // credit text indent, mm
	creditsEntryGap  = 2.5       // between two images' credits, mm
	// maxCreditLines bounds the lines one line of a credit wraps to; the
	// downloader caps credit fields far below it.
	maxCreditLines = 100
)

// credit is the credit of one selected image.
type credit struct {
	Name string // the image's title and creator, from its file name
	Text string // its sidecar's text, trimmed, lines separated by "\n"
}

// readCredits reads the credit sidecar of each image at paths, in order,
// skipping images without one (or with a blank one). An image selected
// twice is credited twice. A sidecar that exists but cannot be read is an
// error naming it.
func readCredits(paths []string) ([]credit, error) {
	var credits []credit
	for _, p := range paths {
		sidecar := media.CreditPath(p)
		data, err := os.ReadFile(sidecar)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(sidecar), err)
		}
		text := strings.TrimSpace(strings.ReplaceAll(printable(string(data)), "\r\n", "\n"))
		if text == "" {
			continue
		}
		credits = append(credits, credit{Name: printable(creditName(p)), Text: text})
	}
	return credits, nil
}

// printable returns s with invalid UTF-8 and runes outside the Basic
// Multilingual Plane (emoji, rare CJK) replaced by "?": the PDF's UTF-8
// fonts index glyph widths by 16-bit rune and fail on anything above.
// Runes in the plane that the text font lacks print as empty boxes.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r == utf8.RuneError || r > 0xFFFF {
			return '?'
		}
		return r
	}, s)
}

// creditName is how the credits page names the image at path: "Title —
// Creator" from its file name, the title alone without a creator, or the
// file name when the title is empty.
func creditName(path string) string {
	title, creator := ParseName(path)
	switch {
	case title == "":
		return filepath.Base(path)
	case creator == "":
		return title
	}
	return title + " — " + creator
}

// creditLine is a line of text on a credits page.
type creditLine struct {
	X, Y float64 // left end and top of the line box, mm
	Text string
	Bold bool
	Size float64 // pt
}

// layoutCredits lays credits out on pages of page size, as measured by
// measure: every page starts with title, bold; then each credit, in order,
// as its number and name, bold, and its text, indented, each line wrapped
// to the page width. A credit that does not fit in what is left of a page
// starts the next one; a credit taller than a page continues on the next.
func layoutCredits(page Page, title string, credits []credit, measure measureFunc) [][]creditLine {
	left := creditsMargin
	width := page.Width - 2*creditsMargin
	bottom := page.Height - creditsMargin
	sized := func(bold bool, pt float64) func(string) float64 {
		return func(s string) float64 { return measure(s, bold, pt) }
	}
	heading := fitLines(title, width, 1, sized(true, creditsTitleSize))

	var pages [][]creditLine
	var lines []creditLine
	y, used := 0.0, 0 // next line top; lines below the heading on the page
	newPage := func() {
		if lines != nil {
			pages = append(pages, lines)
		}
		lines = []creditLine{}
		for _, h := range heading {
			lines = append(lines, creditLine{X: left, Y: creditsMargin, Text: h, Bold: true, Size: creditsTitleSize})
		}
		y = creditsMargin + lineHeight(creditsTitleSize) + creditsTitleGap
		used = 0
	}
	newPage()

	for i, c := range credits {
		var entry []creditLine
		name := strconv.Itoa(i+1) + ". " + c.Name
		for _, l := range fitLines(name, width, maxCreditLines, sized(true, creditsNameSize)) {
			entry = append(entry, creditLine{X: left, Text: l, Bold: true, Size: creditsNameSize})
		}
		for _, text := range strings.Split(c.Text, "\n") {
			for _, l := range fitLines(text, width-creditsIndent, maxCreditLines, sized(false, creditsTextSize)) {
				entry = append(entry, creditLine{X: left + creditsIndent, Text: l, Size: creditsTextSize})
			}
		}
		height := 0.0
		for _, l := range entry {
			height += lineHeight(l.Size)
		}
		if used > 0 && y+height > bottom+fitTolerance {
			newPage()
		}
		for _, l := range entry {
			if used > 0 && y+lineHeight(l.Size) > bottom+fitTolerance {
				newPage()
			}
			l.Y = y
			lines = append(lines, l)
			y += lineHeight(l.Size)
			used++
		}
		y += creditsEntryGap
	}
	return append(pages, lines)
}

// CreditPages is the number of credits pages Build adds after the pages of
// the images at paths: 0 when Credits is off or no image has a credit
// sidecar.
func (o Options) CreditPages(paths []string) (int, error) {
	if !o.Credits {
		return 0, nil
	}
	grid, err := o.Grid()
	if err != nil {
		return 0, err
	}
	credits, err := readCredits(paths)
	if err != nil || len(credits) == 0 {
		return 0, err
	}
	pdf := newPDF(grid.Page)
	if err := addTextFonts(pdf); err != nil {
		return 0, err
	}
	return len(layoutCredits(grid.Page, o.labels().Credits, credits, pdfMeasure(pdf))), nil
}

// addCredits adds the credits pages of the images at paths to pdf, on
// pages of page size, when opts.Credits is on and an image has a credit
// sidecar; otherwise it leaves pdf untouched, so the sheet is exactly what
// it was without credits.
func addCredits(pdf *fpdf.Fpdf, page Page, paths []string, opts Options) error {
	if !opts.Credits {
		return nil
	}
	credits, err := readCredits(paths)
	if err != nil || len(credits) == 0 {
		return err
	}
	if err := addTextFonts(pdf); err != nil {
		return err
	}
	for _, lines := range layoutCredits(page, opts.labels().Credits, credits, pdfMeasure(pdf)) {
		pdf.AddPage()
		for _, l := range lines {
			grey := textGrey
			if l.Bold {
				grey = titleGrey
			}
			pdf.SetTextColor(grey, grey, grey)
			pdf.SetFont(textFont, fontStyle(l.Bold), l.Size)
			pdf.Text(l.X, baseline(l.Y, l.Size), l.Text)
		}
	}
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("credits page: %w", err)
	}
	return nil
}
