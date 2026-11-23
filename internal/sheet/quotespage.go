package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"unicode/utf8"
)

// QuotesKind is the ID of the quote cards page kind: quotes laid out as
// cut-out cards, two columns of them, in the lettering font.
const QuotesKind = "quotes"

// Quote card styles.
const (
	// QuotesClassic centres the quote in curly quotes, its author below.
	QuotesClassic = "classic"
	// QuotesBold sets the quote left-aligned and larger, its author on the
	// right.
	QuotesBold = "bold"
	// QuotesFramed is Classic inside a thin frame within the cut line.
	QuotesFramed = "framed"
)

// QuoteStyles are the card styles to choose from, the default first.
var QuoteStyles = []string{QuotesClassic, QuotesBold, QuotesFramed}

// QuotesPerPage are the card counts a page may hold, in two columns.
var QuotesPerPage = []int{4, 6, 8}

// SourceWikiquote marks a quote fetched from Wikiquote: the pages holding
// one print the Wikiquote credit.
const SourceWikiquote = "wikiquote"

// Quote list limits.
const (
	MaxQuotes           = 200 // quotes in a document
	MaxQuoteRunes       = 600 // characters in a quote's text
	MaxQuoteAuthorRunes = 100 // characters in a quote's author
)

// Quote card drawing settings: lengths in millimetres, font sizes in
// points.
const (
	quotesPad        = 4.5 // between the cut line and the text
	quotesFrameInset = 2.5 // the framed style's frame, inside the cut line
	quotesFramedPad  = 5.0 // between the cut line and the text, framed
	quotesFrameWidth = 0.3
	quotesCutWidth   = borderWidth
	quotesDash       = 1.5 // dashes of the cut line
	quotesDashSpace  = 1.0
	quotesMinSize    = 8.0 // smallest quote text, cut with "…" below it
	quotesSizeStep   = 0.5
	quotesBoldScale  = 1.2 // the bold style's largest size, of the others'
	quotesAuthorSize = 8.0
	quotesAuthorGap  = 1.5 // between the quote and its author
	quotesCreditSize = 6.5
	quotesCreditGap  = 1.0 // between the cards and the credit caption
	// quotesCutGrey is the cut line colour, the canvas's #8a919c.
	quotesCutR, quotesCutG, quotesCutB = 0x8a, 0x91, 0x9c
	quotesFrameGrey                    = starGrey
)

// quotesMaxSizes are the largest quote text sizes for 4, 6 and 8 cards a
// page.
var quotesMaxSizes = map[int]float64{4: 22, 6: 18, 8: 15}

// QuoteCard is one quote on a card.
type QuoteCard struct {
	Text   string `json:"text"`
	Author string `json:"author"`
	// Source is "" for a typed quote or SourceWikiquote.
	Source string `json:"source"`
}

// QuotesOptions are the options of a quote cards page.
type QuotesOptions struct {
	// Quotes are the cards' quotes, text and author normalised by
	// normalizeHabit, those with blank text dropped: at most MaxQuotes,
	// texts of at most MaxQuoteRunes and authors of at most
	// MaxQuoteAuthorRunes characters. None prints a page of empty cards.
	Quotes  []QuoteCard `json:"quotes"`
	Style   string      `json:"style"`   // one of QuoteStyles
	PerPage int         `json:"perPage"` // one of QuotesPerPage
}

var defaultQuotes = QuotesOptions{Quotes: []QuoteCard{}, Style: QuotesClassic, PerPage: 6}

// decodeQuotes decodes quote card options, like decodeOptions, with the
// quotes normalised and those with blank text dropped.
func decodeQuotes(raw json.RawMessage) (pageDoc, error) {
	doc, err := decodeOptions(defaultQuotes)(raw)
	if err != nil {
		return nil, err
	}
	o := doc.(QuotesOptions)
	o.Quotes = normalizeQuotes(o.Quotes)
	return o, nil
}

// normalizeQuotes returns quotes with text and author normalised by
// normalizeHabit, without those whose text is blank; never nil.
func normalizeQuotes(quotes []QuoteCard) []QuoteCard {
	out := []QuoteCard{}
	for _, q := range quotes {
		q.Text, q.Author = normalizeHabit(q.Text), normalizeHabit(q.Author)
		if q.Text != "" {
			out = append(out, q)
		}
	}
	return out
}

// validate checks the style, the per-page count and the quotes as they
// print: normalised, blank ones dropped.
func (o QuotesOptions) validate() error {
	if !oneOf(o.Style, QuoteStyles) {
		return fmt.Errorf("quote cards style %q: want one of %v", o.Style, QuoteStyles)
	}
	if !slices.Contains(QuotesPerPage, o.PerPage) {
		return fmt.Errorf("quote cards per page %d: want one of %v", o.PerPage, QuotesPerPage)
	}
	for i, q := range o.Quotes {
		if q.Source != "" && q.Source != SourceWikiquote {
			return fmt.Errorf("quote cards quote %d source %q: want \"\" or %q", i+1, q.Source, SourceWikiquote)
		}
	}
	quotes := normalizeQuotes(o.Quotes)
	if len(quotes) > MaxQuotes {
		return fmt.Errorf("quote cards: %d quotes, want at most %d", len(quotes), MaxQuotes)
	}
	for i, q := range quotes {
		if n := utf8.RuneCountInString(q.Text); n > MaxQuoteRunes {
			return fmt.Errorf("quote cards quote %d: %d characters, want at most %d", i+1, n, MaxQuoteRunes)
		}
		if n := utf8.RuneCountInString(q.Author); n > MaxQuoteAuthorRunes {
			return fmt.Errorf("quote cards quote %d author: %d characters, want at most %d", i+1, n, MaxQuoteAuthorRunes)
		}
	}
	return nil
}

// pages is one page per PerPage quotes, at least 1.
func (o QuotesOptions) pages() int {
	return max(1, (len(o.Quotes)+o.PerPage-1)/o.PerPage)
}

// defaultTitle is the QuotesTitle label.
func (o QuotesOptions) defaultTitle(labels PageLabels) string {
	return labels.QuotesTitle
}

// quotesMaxSize is the largest quote text size for perPage cards a page in
// style: a fifth larger in the bold style.
func quotesMaxSize(perPage int, style string) float64 {
	pt := quotesMaxSizes[perPage]
	if style == QuotesBold {
		pt *= quotesBoldScale
	}
	return pt
}

// draw draws the pages, each with the title centred in its band at the top
// in the lettering l and a full grid of cards below it.
func (o QuotesOptions) draw(pdf *pdfDoc, page Page, title string, l lettering, labels PageLabels) error {
	if err := addRegularTextFont(pdf); err != nil {
		return err
	}
	// The quotes are in the lettering too: buildPage adds its font only
	// for a title. Adding it again is a no-op.
	if len(o.Quotes) > 0 {
		if err := l.add(pdf); err != nil {
			return err
		}
	}
	for _, s := range o.sheets(page, l, l.measure(pdf), pdfMeasure(pdf), labels) {
		pdf.AddPage()
		drawPageTitle(pdf, page, title, l)
		drawQuotesSheet(pdf, s, l)
	}
	return pdf.Error()
}

// quoteLine is a line of text and its box: X and Y its top left, W its
// measured width, H its line height.
type quoteLine struct {
	Text string
	Box  Box
}

// quoteCardSheet is one card: its cut line, the frame of the framed style
// (zero otherwise), the inner box text stays in, and its quote lines at
// Size points and author line (empty text for none).
type quoteCardSheet struct {
	Card   Box
	Frame  Box
	Inner  Box
	Size   float64
	Lines  []quoteLine
	Author quoteLine
}

// quotesSheet is one page of cards below the title, and the credit
// caption at its foot (empty text when no quote on the page is from
// Wikiquote; its box is reserved anyway).
type quotesSheet struct {
	Cards  []quoteCardSheet
	Credit quoteLine
}

// sheets lays out the pages on page: PerPage cards a page in two columns,
// filling the space below the title band within the margins, Gap apart,
// above a band at the foot for the credit caption. The quotes fill the
// cards in order; the cards left over on the last page (or the only page,
// with no quotes) are empty frames to write in by hand. quoteMeasure
// measures text in the lettering l, textMeasure in the Go font.
func (o QuotesOptions) sheets(page Page, l lettering, quoteMeasure, textMeasure measureFunc, labels PageLabels) []quotesSheet {
	area := moonArea(page)
	creditH := lineHeight(quotesCreditSize)
	credit := Box{X: area.X, Y: area.Y + area.H - creditH, W: area.W, H: creditH}
	area.H -= creditH + quotesCreditGap

	rows := o.PerPage / 2
	cardW := (area.W - Gap) / 2
	cardH := (area.H - Gap*float64(rows-1)) / float64(rows)
	out := make([]quotesSheet, o.pages())
	for p := range out {
		s := &out[p]
		wikiquote := false
		for i := 0; i < o.PerPage; i++ {
			card := Box{X: area.X + float64(i%2)*(cardW+Gap), Y: area.Y + float64(i/2)*(cardH+Gap), W: cardW, H: cardH}
			c := quoteCardSheet{Card: card, Inner: inset(card, quotesPad)}
			if o.Style == QuotesFramed {
				c.Frame, c.Inner = inset(card, quotesFrameInset), inset(card, quotesFramedPad)
			}
			if n := p*o.PerPage + i; n < len(o.Quotes) {
				q := o.Quotes[n]
				o.layoutCard(&c, q, l, quoteMeasure, textMeasure)
				wikiquote = wikiquote || q.Source == SourceWikiquote
			}
			s.Cards = append(s.Cards, c)
		}
		s.Credit.Box = credit
		if wikiquote {
			text := fitText(labels.QuotesCredit, credit.W, quotesCreditSize, textMeasure)
			w := textMeasure(text, false, quotesCreditSize)
			s.Credit = quoteLine{Text: text, Box: Box{X: credit.X + (credit.W-w)/2, Y: credit.Y, W: w, H: creditH}}
		}
	}
	return out
}

// inset is b shrunk by d on every side.
func inset(b Box, d float64) Box {
	return Box{X: b.X + d, Y: b.Y + d, W: b.W - 2*d, H: b.H - 2*d}
}

// layoutCard lays out quote q in card c's inner box. The text, in curly
// quotes except in the bold style, wraps to the inner width (less the
// lettering's safety margin) at the style's largest size, shrinking by
// quotesSizeStep until it fits above the author line; at quotesMinSize the
// last line is cut with "…" (before the closing quote). The author, "— "
// and the name in the Go font, is cut with "…" to one line. The text and
// the author are centred down the inner box together: centred across it,
// or in the bold style the text on the left and the author on the right.
func (o QuotesOptions) layoutCard(c *quoteCardSheet, q QuoteCard, l lettering, quoteMeasure, textMeasure measureFunc) {
	inner := c.Inner
	authorH := 0.0
	if q.Author != "" {
		text := fitText("— "+q.Author, inner.W, quotesAuthorSize, textMeasure)
		c.Author = quoteLine{Text: text, Box: Box{W: textMeasure(text, false, quotesAuthorSize), H: lineHeight(quotesAuthorSize)}}
		authorH = quotesAuthorGap + c.Author.Box.H
	}

	text, openQ, closeQ := q.Text, "", ""
	if o.Style != QuotesBold {
		openQ, closeQ = "“", "”"
	}
	width := l.fitWidth(inner.W)
	margin := (inner.W - width) / 2
	textH := inner.H - authorH
	var lines []string
	pt := quotesMaxSize(o.PerPage, o.Style)
	for {
		measure := func(s string) float64 { return quoteMeasure(s, false, pt) }
		// Every line keeps room for the closing quote, which goes on the
		// last one.
		room := width - measure(closeQ)
		maxLines := int(math.Floor(textH/lineHeight(pt) + fitTolerance))
		if pt <= quotesMinSize {
			lines = fitLines(openQ+text, room, maxLines, measure)
			break
		}
		if lines = fitLines(openQ+text, room, maxLines+1, measure); len(lines) <= maxLines {
			break
		}
		pt = max(quotesMinSize, pt-quotesSizeStep)
	}
	if len(lines) > 0 {
		lines[len(lines)-1] += closeQ
	}

	lh := lineHeight(pt)
	top := inner.Y + (textH-float64(len(lines))*lh)/2
	c.Size = pt
	for i, s := range lines {
		w := quoteMeasure(s, false, pt)
		x := inner.X + (inner.W-w)/2
		if o.Style == QuotesBold {
			x = inner.X + margin
		}
		c.Lines = append(c.Lines, quoteLine{Text: s, Box: Box{X: x, Y: top + float64(i)*lh, W: w, H: lh}})
	}
	if c.Author.Text != "" {
		c.Author.Box.Y = top + float64(len(lines))*lh + quotesAuthorGap
		c.Author.Box.X = inner.X + (inner.W-c.Author.Box.W)/2
		if o.Style == QuotesBold {
			c.Author.Box.X = inner.X + inner.W - c.Author.Box.W
		}
	}
}

// drawQuotesSheet draws s on the current page: each card's dashed cut
// line and frame, its quote in the lettering l (hollow at 14 pt or more
// with Colour-in) and its author in the Go font, then the credit caption.
// User text is drawn as text only.
func drawQuotesSheet(pdf *pdfDoc, s quotesSheet, l lettering) {
	pdf.SetLineWidth(quotesCutWidth)
	pdf.SetDrawColor(quotesCutR, quotesCutG, quotesCutB)
	pdf.SetDashPattern([]float64{quotesDash, quotesDashSpace}, 0)
	for _, c := range s.Cards {
		pdf.Rect(c.Card.X, c.Card.Y, c.Card.W, c.Card.H, "D")
	}
	pdf.SetDashPattern([]float64{}, 0)
	pdf.SetLineWidth(quotesFrameWidth)
	pdf.SetDrawColor(quotesFrameGrey, quotesFrameGrey, quotesFrameGrey)
	for _, c := range s.Cards {
		if c.Frame != (Box{}) {
			pdf.Rect(c.Frame.X, c.Frame.Y, c.Frame.W, c.Frame.H, "D")
		}
	}
	for _, c := range s.Cards {
		pdf.SetTextColor(titleGrey, titleGrey, titleGrey)
		for _, line := range c.Lines {
			l.setFont(pdf, c.Size)
			l.text(pdf, line.Box.X, baseline(line.Box.Y, c.Size), c.Size, line.Text)
		}
		if c.Author.Text != "" {
			pdf.SetTextColor(textGrey, textGrey, textGrey)
			pdf.SetFont(textFont, "", quotesAuthorSize)
			pdf.Text(c.Author.Box.X, baseline(c.Author.Box.Y, quotesAuthorSize), c.Author.Text)
		}
	}
	if s.Credit.Text != "" {
		pdf.SetTextColor(textGrey, textGrey, textGrey)
		pdf.SetFont(textFont, "", quotesCreditSize)
		pdf.Text(s.Credit.Box.X, baseline(s.Credit.Box.Y, quotesCreditSize), s.Credit.Text)
	}
}
