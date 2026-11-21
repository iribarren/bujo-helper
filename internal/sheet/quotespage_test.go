package sheet

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// quoteList is n typed quotes "Quote 1" to "Quote n" by "Author i", with
// source src.
func quoteList(n int, src string) []QuoteCard {
	out := make([]QuoteCard, n)
	for i := range out {
		out[i] = QuoteCard{Text: fmt.Sprintf("Quote number %d to keep", i+1), Author: fmt.Sprintf("Author %d", i+1), Source: src}
	}
	return out
}

// quotesJSON is the JSON of quote card options.
func quotesJSON(quotes []QuoteCard, style string, perPage int) string {
	raw, _ := json.Marshal(QuotesOptions{Quotes: quotes, Style: style, PerPage: perPage})
	return string(raw)
}

// quoteMeasures returns the lettering of font with Colour-in colourIn and
// the measures of quote text (in that lettering) and of the Go font.
func quoteMeasures(t *testing.T, font string, colourIn bool) (lettering, measureFunc, measureFunc) {
	t.Helper()
	pdf := newPDF(A4)
	if err := addRegularTextFont(pdf); err != nil {
		t.Fatal(err)
	}
	l := newLettering(font, colourIn)
	if err := l.add(pdf); err != nil {
		t.Fatal(err)
	}
	return l, l.measure(pdf), pdfMeasure(pdf)
}

func TestDecodeQuotesOptions(t *testing.T) {
	kind, err := PageKindByID(QuotesKind)
	if err != nil {
		t.Fatal(err)
	}
	long := func(n int) string { return strings.Repeat("a", n) }
	one := func(text, author, source string) string {
		raw, _ := json.Marshal([]QuoteCard{{Text: text, Author: author, Source: source}})
		return `{"quotes":` + string(raw) + `}`
	}
	many := func(n int) string { return `{"quotes":` + string(must(json.Marshal(quoteList(n, "")))) + `}` }
	defaults := QuotesOptions{Quotes: []QuoteCard{}, Style: QuotesClassic, PerPage: 6}
	tests := []struct {
		name    string
		raw     string
		want    QuotesOptions
		wantErr string
	}{
		{"no options take the defaults", "", defaults, ""},
		{"null takes the defaults", "null", defaults, ""},
		{"empty object takes the defaults", "{}", defaults, ""},
		{"bold, 4 per page", `{"style":"bold","perPage":4}`, QuotesOptions{Quotes: []QuoteCard{}, Style: QuotesBold, PerPage: 4}, ""},
		{"framed, 8 per page", `{"style":"framed","perPage":8}`, QuotesOptions{Quotes: []QuoteCard{}, Style: QuotesFramed, PerPage: 8}, ""},
		{"a typed and a Wikiquote quote", `{"quotes":[{"text":"Be kind.","author":"Anon","source":""},{"text":"Imagination.","author":"Einstein","source":"wikiquote"}]}`,
			QuotesOptions{Quotes: []QuoteCard{{Text: "Be kind.", Author: "Anon"}, {Text: "Imagination.", Author: "Einstein", Source: "wikiquote"}}, Style: QuotesClassic, PerPage: 6}, ""},
		{"text and author normalised", `{"quotes":[{"text":"  Be\t\tkind,\n\u0007 always. ","author":" Ann  Lee\n"}]}`,
			QuotesOptions{Quotes: []QuoteCard{{Text: "Be kind, always.", Author: "Ann Lee"}}, Style: QuotesClassic, PerPage: 6}, ""},
		{"empty texts dropped", `{"quotes":[{"text":" \t","author":"Nobody"},{"text":"Kept.","author":""},{"text":"","author":""}]}`,
			QuotesOptions{Quotes: []QuoteCard{{Text: "Kept."}}, Style: QuotesClassic, PerPage: 6}, ""},
		{"format verbs kept as text", one("100% %s %d", "%v", ""), QuotesOptions{Quotes: []QuoteCard{{Text: "100% %s %d", Author: "%v"}}, Style: QuotesClassic, PerPage: 6}, ""},
		{"600-rune text", one(long(MaxQuoteRunes), "", ""), QuotesOptions{Quotes: []QuoteCard{{Text: long(MaxQuoteRunes)}}, Style: QuotesClassic, PerPage: 6}, ""},
		{"100-rune author", one("Hi there.", long(MaxQuoteAuthorRunes), ""), QuotesOptions{Quotes: []QuoteCard{{Text: "Hi there.", Author: long(MaxQuoteAuthorRunes)}}, Style: QuotesClassic, PerPage: 6}, ""},
		{"601-rune text", one(long(MaxQuoteRunes+1), "", ""), QuotesOptions{}, "601 characters, want at most 600"},
		{"101-rune author", one("Hi there.", long(MaxQuoteAuthorRunes+1), ""), QuotesOptions{}, "101 characters, want at most 100"},
		{"limits count runes, not bytes", one(strings.Repeat("é", MaxQuoteRunes), "", ""), QuotesOptions{Quotes: []QuoteCard{{Text: strings.Repeat("é", MaxQuoteRunes)}}, Style: QuotesClassic, PerPage: 6}, ""},
		{"201 quotes", many(MaxQuotes + 1), QuotesOptions{}, "201 quotes, want at most 200"},
		{"201 quotes with an empty one", `{"quotes":` + string(must(json.Marshal(append(quoteList(MaxQuotes, ""), QuoteCard{Text: "  "})))) + `}`, QuotesOptions{}, ""},
		{"unknown style", `{"style":"fancy"}`, QuotesOptions{}, `"fancy"`},
		{"empty style", `{"style":""}`, QuotesOptions{}, `style ""`},
		{"unknown per page", `{"perPage":5}`, QuotesOptions{}, "5"},
		{"zero per page", `{"perPage":0}`, QuotesOptions{}, "per page 0"},
		{"unknown source", one("Hi there.", "", "quotable"), QuotesOptions{}, `"quotable"`},
		{"unknown field", `{"quotes":[],"colour":"red"}`, QuotesOptions{}, `"colour"`},
		{"unknown quote field", `{"quotes":[{"text":"Hi","year":1990}]}`, QuotesOptions{}, `"year"`},
		{"quotes not a list", `{"quotes":"Hi"}`, QuotesOptions{}, "quotes"},
		{"trailing data", `{} {}`, QuotesOptions{}, "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := kind.decode(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, ok := doc.(QuotesOptions)
			if tt.name == "201 quotes with an empty one" {
				if !ok || len(got.Quotes) != MaxQuotes {
					t.Errorf("decoded %d quotes, want %d", len(got.Quotes), MaxQuotes)
				}
				return
			}
			if !ok || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %#v, want %#v", doc, tt.want)
			}
		})
	}
	if len(defaultQuotes.Quotes) != 0 {
		t.Errorf("decoding changed the defaults: %v", defaultQuotes.Quotes)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestQuotesRegistryEntry(t *testing.T) {
	k, err := PageKindByID(QuotesKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Quote cards" || k.FileTag != "quotes" || !k.UsesTitle || k.UsesCut {
		t.Errorf("quotes kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{"quotes":[],"style":"classic","perPage":6}` {
		t.Errorf("defaults %s", raw)
	}
	// After the habit tracker, before the map; TestCalendarRegistryEntry
	// checks the full order.
	for i, k := range PageKinds {
		if k.ID == QuotesKind && (i == 0 || PageKinds[i-1].ID != HabitsKind || i+1 >= len(PageKinds) || PageKinds[i+1].ID != MapKind) {
			t.Errorf("quotes kind at %d, want between habits and map", i)
		}
	}
}

// The default title and the credit caption come from the labels, English
// by default; a label is printed as is, never as a format string.
func TestQuotesLabels(t *testing.T) {
	tests := []struct {
		labels      PageLabels
		wantTitle   string
		wantCredit  string
		description string
	}{
		{PageLabels{}, "Words to keep", "Quotes from Wikiquote (CC BY-SA 4.0)", "English"},
		{PageLabels{QuotesTitle: "Palabras para guardar", QuotesCredit: "Citas de Wikiquote (CC BY-SA 4.0)"}, "Palabras para guardar", "Citas de Wikiquote (CC BY-SA 4.0)", "Spanish"},
		{PageLabels{QuotesTitle: "  ", QuotesCredit: "\t"}, "Words to keep", "Quotes from Wikiquote (CC BY-SA 4.0)", "blank"},
		{PageLabels{QuotesTitle: "100% %s"}, "100% %s", "Quotes from Wikiquote (CC BY-SA 4.0)", "percent"},
	}
	for _, tt := range tests {
		labels := tt.labels.resolved()
		if got := defaultQuotes.defaultTitle(labels); got != tt.wantTitle {
			t.Errorf("%s: title %q, want %q", tt.description, got, tt.wantTitle)
		}
		if labels.QuotesCredit != tt.wantCredit {
			t.Errorf("%s: credit %q, want %q", tt.description, labels.QuotesCredit, tt.wantCredit)
		}
	}
}

func TestQuotesPageCount(t *testing.T) {
	for _, perPage := range QuotesPerPage {
		for _, n := range []int{0, 1, 6, 7, 200} {
			t.Run(fmt.Sprintf("%d per page/%d quotes", perPage, n), func(t *testing.T) {
				want := max(1, (n+perPage-1)/perPage)
				opts := quotesJSON(quoteList(n, ""), QuotesClassic, perPage)
				got, err := PageCount(QuotesKind, PageSettings{}, json.RawMessage(opts))
				if err != nil || got != want {
					t.Errorf("PageCount = %d, %v; want %d", got, err, want)
				}
				l, qm, tm := quoteMeasures(t, DefaultLetteringFont, false)
				o := QuotesOptions{Quotes: quoteList(n, ""), Style: QuotesClassic, PerPage: perPage}
				sheets := o.sheets(A5, l, qm, tm, PageLabels{}.resolved())
				if len(sheets) != want {
					t.Fatalf("%d sheets, want %d", len(sheets), want)
				}
				// Every page is a full grid of cards; the quotes fill them
				// in order and the rest are empty frames.
				i := 0
				for p, s := range sheets {
					if len(s.Cards) != perPage {
						t.Errorf("page %d: %d cards, want %d", p, len(s.Cards), perPage)
					}
					for _, c := range s.Cards {
						switch {
						case i < n && (len(c.Lines) == 0 || !strings.Contains(strings.Join(lineTexts(c.Lines), " "), fmt.Sprintf("number %d ", i+1))):
							t.Errorf("page %d: card for quote %d has lines %q", p, i+1, lineTexts(c.Lines))
						case i >= n && (len(c.Lines) != 0 || c.Author.Text != ""):
							t.Errorf("page %d: empty card %d has text", p, i)
						}
						i++
					}
				}
			})
		}
	}
}

func lineTexts(lines []quoteLine) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.Text)
	}
	return out
}

// insideBox reports whether b lies inside outer, with a little tolerance.
func insideBox(b, outer Box) bool {
	const eps = 1e-6
	return b.X >= outer.X-eps && b.Y >= outer.Y-eps && b.X+b.W <= outer.X+outer.W+eps && b.Y+b.H <= outer.Y+outer.H+eps
}

// On every paper and per-page value the cards sit inside the margins,
// below the title band and above the credit caption, two columns wide,
// at least Gap apart; inner boxes and frames sit inside their card.
func TestQuotesCardGeometry(t *testing.T) {
	l, qm, tm := quoteMeasures(t, DefaultLetteringFont, false)
	for _, p := range Papers {
		for _, perPage := range QuotesPerPage {
			for _, style := range QuoteStyles {
				t.Run(fmt.Sprintf("%s/%d/%s", p.ID, perPage, style), func(t *testing.T) {
					o := QuotesOptions{Quotes: quoteList(1, SourceWikiquote), Style: style, PerPage: perPage}
					s := o.sheets(p.Page, l, qm, tm, PageLabels{}.resolved())[0]
					area := Box{X: MinMargin, Y: MinMargin + DotGridTitleHeight, W: p.Page.Width - 2*MinMargin, H: p.Page.Height - 2*MinMargin - DotGridTitleHeight}
					if !insideBox(s.Credit.Box, area) {
						t.Errorf("credit %+v outside %+v", s.Credit.Box, area)
					}
					cols := map[float64]bool{}
					for i, c := range s.Cards {
						if !insideBox(c.Card, area) || c.Card.Y+c.Card.H > s.Credit.Box.Y+1e-6 {
							t.Errorf("card %d %+v outside the area or over the credit", i, c.Card)
						}
						if !insideBox(c.Inner, c.Card) || c.Inner.W <= 0 || c.Inner.H <= 0 {
							t.Errorf("card %d inner box %+v", i, c.Inner)
						}
						if style == QuotesFramed {
							if !insideBox(c.Frame, c.Card) || !insideBox(c.Inner, c.Frame) || !near(c.Frame.X-c.Card.X, quotesFrameInset) {
								t.Errorf("card %d frame %+v in %+v around %+v", i, c.Frame, c.Card, c.Inner)
							}
						} else if c.Frame != (Box{}) {
							t.Errorf("card %d has a frame in style %s", i, style)
						}
						cols[c.Card.X] = true
						for j, d := range s.Cards[:i] {
							apartX := d.Card.X+d.Card.W+Gap <= c.Card.X+1e-6 || c.Card.X+c.Card.W+Gap <= d.Card.X+1e-6
							apartY := d.Card.Y+d.Card.H+Gap <= c.Card.Y+1e-6 || c.Card.Y+c.Card.H+Gap <= d.Card.Y+1e-6
							if !apartX && !apartY {
								t.Errorf("cards %d %+v and %d %+v overlap or are closer than Gap", j, d.Card, i, c.Card)
							}
						}
					}
					if len(cols) != 2 {
						t.Errorf("%d columns, want 2", len(cols))
					}
				})
			}
		}
	}
}

// checkQuoteCard checks that card c's text and author lie inside its inner
// box, as measured, one below the other, the text at a size between the
// floor and maxSize.
func checkQuoteCard(t *testing.T, c quoteCardSheet, qm, tm measureFunc, maxSize float64) {
	t.Helper()
	if c.Size < quotesMinSize-1e-9 || c.Size > maxSize+1e-9 {
		t.Errorf("text at %v pt, want %v to %v", c.Size, quotesMinSize, maxSize)
	}
	bottom := c.Inner.Y
	for i, line := range c.Lines {
		if w := qm(line.Text, false, c.Size); !near(w, line.Box.W) {
			t.Errorf("line %d %q: box %v mm wide, measures %v", i, line.Text, line.Box.W, w)
		}
		if !near(line.Box.H, lineHeight(c.Size)) {
			t.Errorf("line %d: %v mm high, want %v", i, line.Box.H, lineHeight(c.Size))
		}
		if !insideBox(line.Box, c.Inner) {
			t.Errorf("line %d %q %+v leaves the inner box %+v", i, line.Text, line.Box, c.Inner)
		}
		if line.Box.Y < bottom-1e-6 {
			t.Errorf("line %d overlaps the line above", i)
		}
		bottom = line.Box.Y + line.Box.H
	}
	if c.Author.Text != "" {
		if w := tm(c.Author.Text, false, quotesAuthorSize); !near(w, c.Author.Box.W) {
			t.Errorf("author %q: box %v mm wide, measures %v", c.Author.Text, c.Author.Box.W, w)
		}
		if !insideBox(c.Author.Box, c.Inner) || c.Author.Box.Y < bottom-1e-6 {
			t.Errorf("author %+v leaves the inner box %+v or overlaps the text ending at %v", c.Author.Box, c.Inner, bottom)
		}
	}
}

// The longest quote fits every card, style and paper in the Go font and in
// bundled script and display fonts: shrunk to the floor and cut with "…"
// when it still does not fit. A short quote prints at the style's largest
// size. The author line is "— Author", cut with "…" to one line.
func TestQuotesTextFits(t *testing.T) {
	longText := strings.TrimSpace(strings.Repeat("Imagination is more important than knowledge. ", 14))[:MaxQuoteRunes]
	longAuthor := strings.Repeat("Wolfgang ", 12)[:MaxQuoteAuthorRunes]
	for _, font := range []string{DefaultLetteringFont, "pacifico", "bungee", "amatic-sc"} {
		l, qm, tm := quoteMeasures(t, font, false)
		for _, p := range Papers {
			for _, perPage := range QuotesPerPage {
				for _, style := range QuoteStyles {
					t.Run(fmt.Sprintf("%s/%s/%d/%s", font, p.ID, perPage, style), func(t *testing.T) {
						o := QuotesOptions{Quotes: []QuoteCard{
							{Text: longText, Author: longAuthor},
							{Text: "Be kind.", Author: "Anon"},
							{Text: longText},
							{Text: strings.Repeat("W", MaxQuoteRunes)},
						}, Style: style, PerPage: perPage}
						maxSize := quotesMaxSize(perPage, style)
						cards := o.sheets(p.Page, l, qm, tm, PageLabels{}.resolved())[0].Cards
						for i, c := range cards[:len(o.Quotes)] {
							checkQuoteCard(t, c, qm, tm, maxSize)
							if len(c.Lines) == 0 {
								t.Fatalf("card %d has no lines", i)
							}
							last := c.Lines[len(c.Lines)-1].Text
							cut := strings.Contains(last, ellipsis)
							if cut && !near(c.Size, quotesMinSize) {
								t.Errorf("card %d cut at %v pt, above the floor", i, c.Size)
							}
							if style != QuotesBold {
								if !strings.HasPrefix(c.Lines[0].Text, "“") || !strings.HasSuffix(last, "”") {
									t.Errorf("card %d: %q … %q not in curly quotes", i, c.Lines[0].Text, last)
								}
							}
							if style == QuotesBold && strings.ContainsAny(c.Lines[0].Text, "“”") {
								t.Errorf("card %d: bold text %q in quotes", i, c.Lines[0].Text)
							}
						}
						short := cards[1]
						if !near(short.Size, maxSize) || len(short.Lines) != 1 {
							t.Errorf("short quote at %v pt in %d lines, want %v pt on one", short.Size, len(short.Lines), maxSize)
						}
						if short.Author.Text != "— Anon" {
							t.Errorf("author %q, want \"— Anon\"", short.Author.Text)
						}
						if a := cards[0].Author.Text; !strings.HasPrefix(a, "— ") || utf8.RuneCountInString(a) > MaxQuoteAuthorRunes+2 {
							t.Errorf("long author %q", a)
						}
						if cards[2].Author.Text != "" {
							t.Errorf("author %q for a quote without one", cards[2].Author.Text)
						}
						// Alignment: centred in classic and framed; bold text
						// on the left and its author on the right.
						line, inner := short.Lines[0].Box, short.Inner
						left, right := line.X-inner.X, inner.X+inner.W-line.X-line.W
						if style == QuotesBold {
							if left > right || !near(short.Author.Box.X+short.Author.Box.W, inner.X+inner.W) {
								t.Errorf("bold card not left-aligned with the author on the right")
							}
						} else if !near(left, right) || !near(short.Author.Box.X-inner.X, inner.X+inner.W-short.Author.Box.X-short.Author.Box.W) {
							t.Errorf("%s card not centred", style)
						}
					})
				}
			}
		}
	}
}

// The largest sizes: 22, 18 and 15 pt for 4, 6 and 8 cards a page; bold a
// fifth larger.
func TestQuotesMaxSize(t *testing.T) {
	for _, tt := range []struct {
		perPage int
		style   string
		want    float64
	}{
		{4, QuotesClassic, 22}, {6, QuotesFramed, 18}, {8, QuotesClassic, 15},
		{4, QuotesBold, 26.4}, {6, QuotesBold, 21.6}, {8, QuotesBold, 18},
	} {
		if got := quotesMaxSize(tt.perPage, tt.style); !near(got, tt.want) {
			t.Errorf("quotesMaxSize(%d, %s) = %v, want %v", tt.perPage, tt.style, got, tt.want)
		}
	}
}

// The credit caption prints on the pages, and only the pages, that hold a
// Wikiquote quote.
func TestQuotesCreditPerPage(t *testing.T) {
	quotes := append(append(quoteList(4, ""), quoteList(1, SourceWikiquote)...), quoteList(4, "")...)
	quotes = append(quotes, quoteList(4, SourceWikiquote)...)
	o := QuotesOptions{Quotes: quotes, Style: QuotesClassic, PerPage: 4}
	l, qm, tm := quoteMeasures(t, DefaultLetteringFont, false)
	sheets := o.sheets(A4, l, qm, tm, PageLabels{}.resolved())
	want := []bool{false, true, true, true}
	if len(sheets) != len(want) {
		t.Fatalf("%d pages, want %d", len(sheets), len(want))
	}
	for i, s := range sheets {
		if got := s.Credit.Text != ""; got != want[i] {
			t.Errorf("page %d: credit %q, want one: %v", i, s.Credit.Text, want[i])
		}
		if want[i] {
			if s.Credit.Text != "Quotes from Wikiquote (CC BY-SA 4.0)" || !near(s.Credit.Box.W, tm(s.Credit.Text, false, quotesCreditSize)) {
				t.Errorf("page %d: credit %+v", i, s.Credit)
			}
			if !near(s.Credit.Box.X-MinMargin, A4.Width-MinMargin-s.Credit.Box.X-s.Credit.Box.W) {
				t.Errorf("page %d: credit not centred", i)
			}
		}
	}
	// In the PDF, a page with the caption draws one more text.
	pages := pageStreamsEach(renderPage(t, QuotesKind, PageSettings{}, quotesJSON(quotes[:8], QuotesClassic, 4)))
	if len(pages) != 2 {
		t.Fatalf("%d pages", len(pages))
	}
	if a, b := strings.Count(pages[0], " Tj"), strings.Count(pages[1], " Tj"); b != a+1 {
		t.Errorf("texts per page %d and %d, want one more on the second (the credit)", a, b)
	}
	es := pageStreamsEach(renderPage(t, QuotesKind, PageSettings{Labels: PageLabels{QuotesCredit: "Citas de Wikiquote (CC BY-SA 4.0)"}}, quotesJSON(quotes[:8], QuotesClassic, 4)))
	if len(es) != 2 || es[0] != pages[0] || es[1] == pages[1] {
		t.Error("the credit label is not used, or changes a page without the credit")
	}
}

// Quote text is hollow with Colour-in only at 14 pt or more, in fonts that
// can be outlined; every hollow text returns to mode 0.
func TestQuotesHollowText(t *testing.T) {
	longText := strings.TrimSpace(strings.Repeat("Imagination is more important than knowledge. ", 14))[:MaxQuoteRunes]
	tests := []struct {
		name     string
		font     string
		colourIn bool
		text     string
		perPage  int
		hollow   bool // quote lines are hollow
	}{
		{"short quote with colour-in", "pacifico", true, "Be kind.", 6, true},
		{"short quote in the Go font", DefaultLetteringFont, true, "Be kind.", 4, true},
		{"long quote shrinks below 14 pt", "pacifico", true, longText, 6, false},
		{"colour-in off", "pacifico", false, "Be kind.", 6, false},
		{"thin font", "sacramento", true, "Be kind.", 6, false},
		{"native hollow font", "bungee-outline", true, "Be kind.", 6, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quotes := []QuoteCard{{Text: tt.text, Author: "Anon", Source: SourceWikiquote}}
			page := PageSettings{Font: tt.font, ColourIn: tt.colourIn}
			stream := pageStreams(renderPage(t, QuotesKind, page, quotesJSON(quotes, QuotesClassic, tt.perPage)))
			l, qm, tm := quoteMeasures(t, tt.font, tt.colourIn)
			card := QuotesOptions{Quotes: quotes, Style: QuotesClassic, PerPage: tt.perPage}.sheets(A4, l, qm, tm, PageLabels{}.resolved())[0].Cards[0]
			if tt.hollow != l.hollow(card.Size) {
				t.Fatalf("quote at %v pt; hollow want %v", card.Size, tt.hollow)
			}
			want := 0
			if l.hollow(maxHeaderFontSize) {
				want = 1 // the title "Words to keep" fits at the largest header size
			}
			if tt.hollow {
				want += len(card.Lines)
			}
			if want == 0 {
				if strings.Contains(stream, " Tr") {
					t.Errorf("filled text uses a text rendering mode")
				}
				return
			}
			if got := trPairs(t, tt.name, stream); got != want {
				t.Errorf("%d hollow texts, want %d", got, want)
			}
		})
	}
}

// Every style, per-page value and paper builds, with or without quotes and
// Colour-in: the page count PageCount gives, a dashed cut border per card
// and, framed, a solid frame inside each.
func TestBuildQuotesEveryStyle(t *testing.T) {
	for _, p := range Papers {
		for _, style := range QuoteStyles {
			for _, perPage := range QuotesPerPage {
				for _, n := range []int{0, 9} {
					t.Run(fmt.Sprintf("%s/%s/%d/%d", p.ID, style, perPage, n), func(t *testing.T) {
						opts := quotesJSON(quoteList(n, SourceWikiquote), style, perPage)
						page := PageSettings{Paper: p.ID, Font: "kaushan-script", ColourIn: n > 0}
						want, err := PageCount(QuotesKind, page, json.RawMessage(opts))
						if err != nil {
							t.Fatal(err)
						}
						pages := pageStreamsEach(renderPage(t, QuotesKind, page, opts))
						if len(pages) != want {
							t.Fatalf("%d pages, want %d", len(pages), want)
						}
						for i, s := range pages {
							rects := strings.Count(s, " re S")
							wantRects := perPage
							if style == QuotesFramed {
								wantRects *= 2
							}
							if rects != wantRects {
								t.Errorf("page %d: %d stroked rectangles, want %d", i, rects, wantRects)
							}
							if !strings.Contains(s, "] 0.00 d") {
								t.Errorf("page %d: no dashed cut line", i)
							}
							if strings.Count(s, "] 0.00 d") != strings.Count(s, "[] 0.00 d")*2 {
								t.Errorf("page %d: dash pattern not reset", i)
							}
							trPairs(t, fmt.Sprintf("page %d", i), s)
						}
					})
				}
			}
		}
	}
}

// The quote text draws in the lettering even when there is no title, so
// the lettering font is added for it.
func TestQuotesDrawWithoutTitle(t *testing.T) {
	o := QuotesOptions{Quotes: []QuoteCard{{Text: "Be kind."}}, Style: QuotesClassic, PerPage: 6}
	pdf := newPDF(A5)
	if err := o.draw(pdf, A5, "", newLettering("pacifico", false), PageLabels{}.resolved()); err != nil {
		t.Fatalf("draw: %v", err)
	}
	if pdf.PageNo() != 1 {
		t.Errorf("%d pages", pdf.PageNo())
	}
}

// The quotes page prints "Words to keep" (or the Spanish label) when the
// title is empty.
func TestBuildQuotesTitle(t *testing.T) {
	opts := quotesJSON(quoteList(2, ""), QuotesBold, 4)
	none := pageStreams(renderPage(t, QuotesKind, PageSettings{}, opts))
	explicit := pageStreams(renderPage(t, QuotesKind, PageSettings{Title: "Words to keep"}, opts))
	other := pageStreams(renderPage(t, QuotesKind, PageSettings{Title: "Mantras"}, opts))
	if none != explicit || none == other {
		t.Error("the default title is not \"Words to keep\"")
	}
	es := PageLabels{QuotesTitle: "Palabras para guardar"}
	esNone := pageStreams(renderPage(t, QuotesKind, PageSettings{Labels: es}, opts))
	esExplicit := pageStreams(renderPage(t, QuotesKind, PageSettings{Title: "Palabras para guardar"}, opts))
	if esNone != esExplicit || esNone == none {
		t.Error("the Spanish default title is not \"Palabras para guardar\"")
	}
}
