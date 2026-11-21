package sheet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

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
	// Font is the lettering font id of the title (see LetteringFonts); ""
	// or an unknown id means DefaultLetteringFont.
	Font string `json:"font"`
	// ColourIn draws hollow letters (and, for kinds that have them,
	// outlines without fills) to colour in by hand.
	ColourIn bool `json:"colourIn"`
	// Labels are the fixed words kinds print, in the UI language. The app
	// fills them in; the zero value prints English.
	Labels PageLabels `json:"-"`
}

// PageLabels are the fixed words generated pages print, such as month
// names. An empty (or blank) entry, or a list of the wrong length, keeps
// its English default.
type PageLabels struct {
	Months   []string // the 12 month names, January first
	Weekdays []string // the 7 short weekday names, Monday first
	NewMoon  string   // the new moon mark; "New"
	FullMoon string   // the full moon mark; "Full"
	// MapTitle is the visited map's default title; "Places I've been".
	MapTitle string
	// MapVisited is the caption of the visited map's country list;
	// "Countries visited".
	MapVisited string
	// Language is the language of country names: "es" prints the Spanish
	// ones, anything else the English ones ("en").
	Language string
}

// englishWeekdays are the default short weekday names, Monday first.
var englishWeekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// resolved returns l trimmed, with every missing label set to its English
// default.
func (l PageLabels) resolved() PageLabels {
	or := func(s, def string) string {
		if s = strings.TrimSpace(s); s == "" {
			return def
		}
		return s
	}
	list := func(in, defaults []string) []string {
		out := make([]string, len(defaults))
		for i, def := range defaults {
			out[i] = def
			if len(in) == len(defaults) {
				out[i] = or(in[i], def)
			}
		}
		return out
	}
	months := make([]string, 12)
	for i := range months {
		months[i] = time.Month(i + 1).String()
	}
	return PageLabels{
		Months:     list(l.Months, months),
		Weekdays:   list(l.Weekdays, englishWeekdays),
		NewMoon:    or(l.NewMoon, "New"),
		FullMoon:   or(l.FullMoon, "Full"),
		MapTitle:   or(l.MapTitle, "Places I've been"),
		MapVisited: or(l.MapVisited, "Countries visited"),
		Language:   nameLanguage(l.Language),
	}
}

// nameLanguage is "es" for Spanish (any case, spaces trimmed) and "en" for
// anything else.
func nameLanguage(s string) string {
	if strings.EqualFold(strings.TrimSpace(s), "es") {
		return "es"
	}
	return "en"
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
	// the shared title (or the kind's default title), "" if none or if the
	// kind does not use it, l the lettering of its title, whose font is
	// added when there is one, and labels the resolved fixed words.
	draw(pdf *fpdf.Fpdf, page Page, title string, l lettering, labels PageLabels) error
}

// defaultTitler is a pageDoc that prints a title of its own when the
// shared title is empty, such as a moon calendar's month and year.
type defaultTitler interface {
	// defaultTitle is the title in the resolved labels.
	defaultTitle(labels PageLabels) string
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
	{
		ID:        MoonKind,
		Label:     "Moon calendar",
		FileTag:   "moon",
		UsesTitle: true,
		Defaults:  defaultMoon,
		decode:    decodeMoon,
	},
	{
		ID:        CalendarKind,
		Label:     "Month calendar",
		FileTag:   "calendar",
		UsesTitle: true,
		Defaults:  defaultCalendar,
		decode:    decodeCalendar,
	},
	{
		ID:        MapKind,
		Label:     "Visited map",
		FileTag:   "map",
		UsesTitle: true,
		Defaults:  defaultMap,
		decode:    decodeMap,
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
	labels := s.Labels.resolved()
	title := ""
	if k.UsesTitle {
		// The header's own rules: trimmed, runs of spaces collapsed.
		title = Options{Header: s.Title}.header()
		if d, ok := doc.(defaultTitler); ok && title == "" {
			title = Options{Header: d.defaultTitle(labels)}.header()
		}
	}
	l := newLettering(s.Font, s.ColourIn)
	pdf := newPDF(paper.Page)
	if title != "" {
		if err := l.add(pdf); err != nil {
			return nil, err
		}
	}
	if err := doc.draw(pdf, paper.Page, title, l, labels); err != nil {
		return nil, err
	}
	if err := pdf.Error(); err != nil {
		return nil, err
	}
	return pdf, nil
}
