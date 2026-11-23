package quotes

import (
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// Limits on the quotes taken from a page.
const (
	minRunes  = 10
	maxRunes  = 400
	maxQuotes = 60
)

// Section headings whose lists are not the person's own sourced quotes.
// prefixes match the start of a heading; exact ones the whole heading, so
// that e.g. "Notes from Underground" stays while "Notes" goes.
var (
	excludedPrefixes = []string{
		// English.
		"disputed", "misattributed", "attributed", "misquoted",
		"quotes about", "quotations about", "about ",
		// Spanish.
		"atribuidas", "erróneamente atribuidas", "atribuidas erróneamente",
		"dudosas", "citas sobre", "frases sobre", "sobre ",
	}
	excludedHeadings = map[string]bool{
		// English.
		"see also": true, "external links": true, "references": true,
		"sources": true, "notes": true, "further reading": true,
		"bibliography": true,
		// Spanish.
		"véase también": true, "enlaces externos": true, "referencias": true,
		"fuentes": true, "notas": true, "bibliografía": true,
	}
)

// excludedSection reports whether a section with heading holds lists that
// are not quotes to offer: disputed or misattributed quotes, quotes about
// the person, or navigation and references.
func excludedSection(heading string) bool {
	h := strings.ToLower(cleanText(heading))
	h = strings.TrimSpace(strings.TrimRight(h, ":"))
	if excludedHeadings[h] {
		return true
	}
	for _, p := range excludedPrefixes {
		if strings.HasPrefix(h, p) {
			return true
		}
	}
	return false
}

// pageParser collects quotes while walking a page's sections in order.
type pageParser struct {
	author  string
	started bool // a heading has been seen
	h2Out   bool // the current h2 section is excluded
	h3Out   bool // the current h3 section is excluded
	seen    map[string]bool
	quotes  []Quote
}

// parseQuotes returns the quotes of a Wikiquote page's rendered HTML: the
// items of the top-level lists under the h2/h3 sections that are not
// excluded, cleaned, 10–400 runes long, deduplicated and capped at 60.
func parseQuotes(html, author string) ([]Quote, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	root := doc.Find(".mw-parser-output").First()
	if root.Length() == 0 {
		root = doc.Find("body")
	}
	p := &pageParser{author: author, seen: map[string]bool{}}
	p.walk(root)
	return p.quotes, nil
}

// full reports whether the quote cap is reached.
func (p *pageParser) full() bool { return len(p.quotes) >= maxQuotes }

// walk visits the children of sel in order. Divs that wrap headings (such
// as the frame around a Disputed section) are walked into, so their
// headings still switch sections.
func (p *pageParser) walk(sel *goquery.Selection) {
	sel.Children().EachWithBreak(func(_ int, c *goquery.Selection) bool {
		switch name := goquery.NodeName(c); {
		case name == "h2" || name == "h3":
			p.heading(name, c)
		case c.HasClass("mw-heading"):
			if h := c.ChildrenFiltered("h2, h3").First(); h.Length() > 0 {
				p.heading(goquery.NodeName(h), h)
			}
		case c.Is("#toc, .toc"):
		case name == "div" && c.Find("h2, h3").Length() > 0:
			p.walk(c)
		case name == "ul":
			if p.started && !p.h2Out && !p.h3Out {
				c.ChildrenFiltered("li").EachWithBreak(func(_ int, li *goquery.Selection) bool {
					p.item(li)
					return !p.full()
				})
			}
		}
		return !p.full()
	})
}

// heading starts a new h2 or h3 section.
func (p *pageParser) heading(level string, h *goquery.Selection) {
	h.Find(".mw-editsection").Remove()
	out := excludedSection(h.Text())
	p.started = true
	if level == "h2" {
		p.h2Out, p.h3Out = out, false
		return
	}
	p.h3Out = out
}

// item adds the quote of one list item, if it is one.
func (p *pageParser) item(li *goquery.Selection) {
	li.Find("ul, ol, dl, sup.reference, .mw-editsection, style, script").Remove()
	li.Find("br").ReplaceWithHtml(" ")
	text := unwrapQuotes(cleanText(li.Text()))
	if n := len([]rune(text)); n < minRunes || n > maxRunes || p.seen[text] {
		return
	}
	p.seen[text] = true
	p.quotes = append(p.quotes, Quote{Text: text, Author: p.author, Source: SourceWikiquote})
}

// cleanText turns control characters and runs of white space into single
// spaces and trims the result.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// quotePairs maps opening quotation marks to their closing ones.
var quotePairs = map[rune]rune{'"': '"', '“': '”', '„': '“', '«': '»', '‘': '’'}

// unwrapQuotes strips quotation marks that wrap the whole of s, keeping a
// full stop that follows the closing mark (as in Spanish «…».). Text with
// more than one quoted part is left alone.
func unwrapQuotes(s string) string {
	r := []rune(s)
	if len(r) < 2 {
		return s
	}
	open := r[0]
	closing, ok := quotePairs[open]
	if !ok {
		return s
	}
	end, stop := len(r)-1, false
	if r[end] == '.' && end > 1 {
		end, stop = end-1, true
	}
	if r[end] != closing || end == 0 {
		return s
	}
	inner := r[1:end]
	for _, c := range inner {
		if c == open || c == closing {
			return s
		}
	}
	text := strings.TrimSpace(string(inner))
	if stop && !strings.ContainsAny(lastRune(text), ".!?…") {
		text += "."
	}
	return text
}

// lastRune returns the last rune of s as a string, or "" when s is empty.
func lastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	return string(r[len(r)-1])
}
