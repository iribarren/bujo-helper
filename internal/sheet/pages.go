package sheet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/go-pdf/fpdf"
)

// Generated pages are printables drawn without images, such as a dot grid
// page: each page kind draws from its own typed options, decoded from JSON,
// plus the settings every kind shares (PageSettings). Adding a kind only
// takes its options type and an entry in PageKinds.

// PageSettings are the settings all page kinds share: those of the Page
// and Lettering fieldsets. A kind ignores the ones it does not use (see
// PageKind.UsesCut and UsesTitle).
type PageSettings struct {
	Paper string `json:"paper"` // a Papers ID; "" means DefaultPaper
	Cut   string `json:"cut"`   // a CutModes ID; "" means DefaultCut
	Title string `json:"title"` // printed in a header band at the top
}

// PageKind is a kind of generated page.
type PageKind struct {
	ID      string
	Label   string
	FileTag string // short, file-name-safe name, e.g. for "dotgrid_A5.pdf"
	// UsesCut and UsesTitle are whether the shared cut mode and title
	// apply.
	UsesCut   bool
	UsesTitle bool
	// Defaults are the kind's options with their default values; they
	// marshal to the JSON the UI starts its form from.
	Defaults any
	// decode reads the kind's options from JSON over Defaults and
	// validates them.
	decode func(raw json.RawMessage) (pageDoc, error)
}

// pageDoc is a page kind's decoded, valid options: what it draws.
type pageDoc interface {
	// validate reports an option out of range.
	validate() error
	// pages is how many pages the document has.
	pages() int
	// draw adds the pages to pdf, a document of page-sized pages; title is
	// the shared title, "" if none or if the kind does not use it.
	draw(pdf *fpdf.Fpdf, page Page, title string) error
}

// PageKinds lists the page kinds.
var PageKinds = []PageKind{
	{
		ID:        DotGridKind,
		Label:     "Dot grid",
		FileTag:   "dotgrid",
		UsesTitle: true,
		Defaults:  defaultDotGrid,
		decode:    decodeOptions(defaultDotGrid),
	},
}

// PageKindByID returns the page kind with the given ID.
func PageKindByID(id string) (PageKind, error) {
	for _, k := range PageKinds {
		if k.ID == id {
			return k, nil
		}
	}
	return PageKind{}, fmt.Errorf("unknown page kind %q", id)
}

// decodeOptions returns a decoder of options of type T from JSON objects
// over defaults: fields left out keep their default, and no options at all
// (empty or null) give the defaults. Unknown fields, wrong types, anything
// after the object and values that fail validation are errors.
func decodeOptions[T pageDoc](defaults T) func(raw json.RawMessage) (pageDoc, error) {
	return func(raw json.RawMessage) (pageDoc, error) {
		opts := defaults
		if len(bytes.TrimSpace(raw)) > 0 {
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&opts); err != nil {
				return nil, fmt.Errorf("page options: %w", err)
			}
			if _, err := dec.Token(); !errors.Is(err, io.EOF) {
				return nil, errors.New("page options: unexpected data after the options")
			}
		}
		if err := opts.validate(); err != nil {
			return nil, err
		}
		return opts, nil
	}
}

// preparePage looks up kind and checks the shared settings and options,
// returning the paper and the decoded options.
func preparePage(kind string, s PageSettings, options json.RawMessage) (PageKind, Paper, pageDoc, error) {
	k, err := PageKindByID(kind)
	if err != nil {
		return PageKind{}, Paper{}, nil, err
	}
	paper, err := PaperByID(s.Paper)
	if err != nil {
		return PageKind{}, Paper{}, nil, err
	}
	// Like the print formats, every kind rejects an unknown cut mode, even
	// one it ignores.
	if _, err := CutModeByID(s.Cut); err != nil {
		return PageKind{}, Paper{}, nil, err
	}
	doc, err := k.decode(options)
	if err != nil {
		return PageKind{}, Paper{}, nil, err
	}
	return k, paper, doc, nil
}

// PageCount returns how many pages the page kind draws with the shared
// settings s and its options, a JSON object (empty for the defaults). It
// returns an error for an unknown kind, setting or option.
func PageCount(kind string, s PageSettings, options json.RawMessage) (int, error) {
	_, _, doc, err := preparePage(kind, s, options)
	if err != nil {
		return 0, err
	}
	return doc.pages(), nil
}

// BuildPage writes a PDF of the page kind, with the shared settings s and
// its options, a JSON object (empty for the defaults), to w. If the kind,
// a setting or an option is invalid, it returns an error and writes
// nothing.
func BuildPage(w io.Writer, kind string, s PageSettings, options json.RawMessage) error {
	pdf, err := buildPage(kind, s, options)
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

// buildPage draws the document BuildPage writes.
func buildPage(kind string, s PageSettings, options json.RawMessage) (*fpdf.Fpdf, error) {
	k, paper, doc, err := preparePage(kind, s, options)
	if err != nil {
		return nil, err
	}
	title := ""
	if k.UsesTitle {
		// The header's own rules: trimmed, runs of spaces collapsed.
		title = Options{Header: s.Title}.header()
	}
	pdf := newPDF(paper.Page)
	if title != "" {
		if err := addTextFonts(pdf); err != nil {
			return nil, err
		}
	}
	if err := doc.draw(pdf, paper.Page, title); err != nil {
		return nil, err
	}
	if err := pdf.Error(); err != nil {
		return nil, err
	}
	return pdf, nil
}
