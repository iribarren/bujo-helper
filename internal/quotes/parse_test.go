package quotes

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func page(body string) string {
	return `<div class="mw-content-ltr mw-parser-output" lang="en">` + body + `</div>`
}

func TestParseQuotes(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "bare headings",
			html: page(`<h2>Quotes</h2><ul><li>The first quote of the page.</li></ul>
				<h3>1920s</h3><ul><li>The second quote, from the twenties.</li></ul>`),
			want: []string{"The first quote of the page.", "The second quote, from the twenties."},
		},
		{
			name: "wrapped headings",
			html: page(`<div class="mw-heading mw-heading2"><h2 id="Quotes">Quotes</h2></div>
				<ul><li>A quote under a wrapped heading.</li></ul>
				<div class="mw-heading mw-heading3"><h3>Speeches</h3></div>
				<ul><li>A quote from a speech, wrapped.</li></ul>`),
			want: []string{"A quote under a wrapped heading.", "A quote from a speech, wrapped."},
		},
		{
			name: "text before the first heading and the TOC are skipped",
			html: page(`<ul><li>An intro list item, not a quote.</li></ul>
				<div id="toc" class="toc"><div class="toctitle"><h2>Contents</h2></div><ul><li>Quotes and more quotes</li></ul></div>
				<h2>Quotes</h2><ul><li>The only real quote here.</li></ul>`),
			want: []string{"The only real quote here."},
		},
		{
			name: "excluded sections",
			html: page(`<h2>Quotes</h2><ul><li>Kept quote number one.</li></ul>
				<h3>Attributed</h3><ul><li>Attributed without a source.</li></ul>
				<h3>Interviews</h3><ul><li>Kept quote number two.</li></ul>
				<h2>Disputed</h2><ul><li>A disputed quote text.</li></ul>
				<h2>Misattributed</h2><ul><li>A misattributed quote text.</li></ul>
				<h2>Quotes about Einstein</h2><ul><li>Someone else talking.</li></ul>
				<h2>About Einstein</h2><ul><li>Someone else talking again.</li></ul>
				<h2>See also</h2><ul><li>Another page title</li></ul>
				<h2>External links</h2><ul><li>Some external website</li></ul>
				<h2>Further reading</h2><ul><li>A long book title</li></ul>`),
			want: []string{"Kept quote number one.", "Kept quote number two."},
		},
		{
			name: "spanish excluded sections",
			html: page(`<h2>Citas</h2><ul><li>«Una cita que se queda».</li></ul>
				<h2>Atribuidas</h2><ul><li>Una cita atribuida sin fuente.</li></ul>
				<h2>Erróneamente atribuidas</h2><ul><li>Una cita mal atribuida.</li></ul>
				<h2>Dudosas</h2><ul><li>Una cita dudosa de verdad.</li></ul>
				<h2>Citas sobre Einstein</h2><ul><li>Otra persona hablando.</li></ul>
				<h2>Sobre Einstein</h2><ul><li>Otra persona hablando más.</li></ul>
				<h2>Véase también</h2><ul><li>Otra página del sitio</li></ul>
				<h2>Referencias</h2><ul><li>Un libro de referencia</li></ul>
				<h2>Bibliografía</h2><ul><li>Un libro de la bibliografía</li></ul>
				<h2>Enlaces externos</h2><ul><li>Un sitio web externo</li></ul>`),
			want: []string{"Una cita que se queda."},
		},
		{
			name: "h3 inside an excluded h2 stays excluded",
			html: page(`<h2>Quotes</h2><ul><li>A kept quote at the top.</li></ul>
				<h2>Misattributed</h2><h3>Science</h3><ul><li>Still misattributed though.</li></ul>
				<h2>Later years</h2><h3>1950s</h3><ul><li>Kept again after it.</li></ul>`),
			want: []string{"A kept quote at the top.", "Kept again after it."},
		},
		{
			name: "headings inside a frame div",
			html: page(`<h2>Quotes</h2><ul><li>A kept quote at the top.</li></ul>
				<div class="disputed-begin-frame"><div class="mw-heading mw-heading2"><h2>Disputed</h2></div>
				<ul><li>A disputed quote inside a frame.</li></ul></div>
				<div class="mw-heading mw-heading2"><h2>Late quotes</h2></div><ul><li>A kept quote after the frame.</li></ul>`),
			want: []string{"A kept quote at the top.", "A kept quote after the frame."},
		},
		{
			name: "nested lists, references and edit links removed",
			html: page(`<h2>Quotes<span class="mw-editsection">[edit]</span></h2>
				<ul><li><i>Imagination is more important than knowledge.</i><sup class="reference"><a href="#n1">[1]</a></sup>
				<ul><li>Interview, 1929.</li></ul><dl><dd>A note.</dd></dl><ol><li>Other</li></ol>
				<style>.x{}</style><script>var x;</script></li>
				<li>First line<br/>second line of the quote.</li></ul>`),
			want: []string{"Imagination is more important than knowledge.", "First line second line of the quote."},
		},
		{
			name: "whitespace and control characters collapse",
			html: page("<h2>Quotes</h2><ul><li>  Spaced \t out\u0007 and\n\nbroken   quote. </li></ul>"),
			want: []string{"Spaced out and broken quote."},
		},
		{
			name: "wrapping quotation marks stripped",
			html: page(`<h2>Quotes</h2><ul>
				<li>"Straight quotes around it."</li>
				<li>“Curly quotes around it.”</li>
				<li>«Comillas angulares». </li>
				<li>«¿Una pregunta entre comillas?».</li>
				<li>"Two" and "three" separate quotes.</li>
				<li>He said “yes” to the offer.</li></ul>`),
			want: []string{
				"Straight quotes around it.",
				"Curly quotes around it.",
				"Comillas angulares.",
				"¿Una pregunta entre comillas?",
				`"Two" and "three" separate quotes.`,
				"He said “yes” to the offer.",
			},
		},
		{
			name: "length bounds and dedupe",
			html: page(`<h2>Quotes</h2><ul>
				<li>Too short</li>
				<li>Just right.</li>
				<li>` + strings.Repeat("a", 400) + `</li>
				<li>` + strings.Repeat("b", 401) + `</li>
				<li>Just right.</li></ul>`),
			want: []string{"Just right.", strings.Repeat("a", 400)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQuotes(tt.html, "Someone")
			if err != nil {
				t.Fatal(err)
			}
			if texts := quoteTexts(got); !slices.Equal(texts, tt.want) {
				t.Errorf("quotes = %q\nwant %q", texts, tt.want)
			}
			for _, q := range got {
				if q.Author != "Someone" || q.Source != "wikiquote" {
					t.Errorf("quote %q: author %q, source %q", q.Text, q.Author, q.Source)
				}
			}
		})
	}
}

func TestParseQuotesCapsAtMax(t *testing.T) {
	var b strings.Builder
	b.WriteString("<h2>Quotes</h2><ul>")
	for i := range maxQuotes + 15 {
		fmt.Fprintf(&b, "<li>Quote number %03d of many.</li>", i)
	}
	b.WriteString("</ul>")
	got, err := parseQuotes(page(b.String()), "Someone")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxQuotes {
		t.Fatalf("got %d quotes, want %d", len(got), maxQuotes)
	}
	if got[0].Text != "Quote number 000 of many." || got[maxQuotes-1].Text != fmt.Sprintf("Quote number %03d of many.", maxQuotes-1) {
		t.Errorf("quotes not in page order: first %q, last %q", got[0].Text, got[maxQuotes-1].Text)
	}
}

func TestExcludedSection(t *testing.T) {
	tests := []struct {
		heading string
		want    bool
	}{
		{"Quotes", false},
		{"1920s", false},
		{"Quotes from his letters", false},
		{"Notebooks (1914)", false},
		{"Notes from Underground (1864)", false},
		{"Selección de pensamientos", false},
		{"Disputed", true},
		{"MISATTRIBUTED", true},
		{"Attributed", true},
		{"Attributed in posthumous publications", true},
		{"Quotes about Einstein", true},
		{"About Einstein", true},
		{"See also", true},
		{"External links", true},
		{"References", true},
		{"Sources", true},
		{"Notes", true},
		{"Further reading", true},
		{"  See   also: ", true},
		{"Atribuidas", true},
		{"Erróneamente atribuidas", true},
		{"Dudosas", true},
		{"Citas sobre Einstein", true},
		{"Sobre Einstein", true},
		{"Véase también", true},
		{"Enlaces externos", true},
		{"Referencias", true},
		{"Fuentes", true},
		{"Notas", true},
		{"Bibliografía", true},
	}
	for _, tt := range tests {
		if got := excludedSection(tt.heading); got != tt.want {
			t.Errorf("excludedSection(%q) = %v, want %v", tt.heading, got, tt.want)
		}
	}
}
