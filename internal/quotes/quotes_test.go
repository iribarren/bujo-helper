package quotes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"bujo-helper/internal/web"
)

// fixtureServer serves the recorded Wikiquote responses in testdata. The
// API URL of language lang is <server>/<lang>; a search for "Einstein" and
// a parse of "Albert Einstein" have fixtures, any other search has no hits.
type fixtureServer struct {
	*httptest.Server
	mu       sync.Mutex
	paths    []string // request paths, in order
	agents   []string // User-Agent of each request
	pages    []string // page parameter of each parse request
	override http.HandlerFunc
}

func newFixtureServer(t *testing.T) *fixtureServer {
	t.Helper()
	fs := &fixtureServer{}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		fs.mu.Lock()
		fs.paths = append(fs.paths, r.URL.Path)
		fs.agents = append(fs.agents, r.UserAgent())
		if q.Get("action") == "parse" {
			fs.pages = append(fs.pages, q.Get("page"))
		}
		override := fs.override
		fs.mu.Unlock()
		if override != nil {
			override(w, r)
			return
		}
		lang := strings.TrimPrefix(r.URL.Path, "/")
		var name string
		switch q.Get("action") {
		case "query":
			if q.Get("list") != "search" || q.Get("srnamespace") != "0" || q.Get("format") != "json" {
				http.Error(w, "bad search parameters", http.StatusBadRequest)
				return
			}
			name = lang + "_search_nohits.json"
			if q.Get("srsearch") == "Einstein" {
				name = lang + "_search_einstein.json"
			}
			if lang == "es" && name == "es_search_nohits.json" {
				name = "en_search_nohits.json"
			}
		case "parse":
			if q.Get("redirects") != "1" || q.Get("prop") != "text" || q.Get("formatversion") != "2" {
				http.Error(w, "bad parse parameters", http.StatusBadRequest)
				return
			}
			name = lang + "_parse_" + strings.ToLower(strings.ReplaceAll(q.Get("page"), " ", "_")) + ".json"
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		body, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	t.Cleanup(fs.Close)
	return fs
}

// parsedPages returns the page parameter of each parse request so far.
func (fs *fixtureServer) parsedPages() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return slices.Clone(fs.pages)
}

// requestPaths returns the path of each request so far.
func (fs *fixtureServer) requestPaths() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return slices.Clone(fs.paths)
}

// userAgents returns the User-Agent of each request so far.
func (fs *fixtureServer) userAgents() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return slices.Clone(fs.agents)
}

func (fs *fixtureServer) client() *Client {
	return &Client{HTTP: fs.Server.Client(), APIURL: func(lang string) string { return fs.URL + "/" + lang }}
}

// referenceMark matches a footnote marker such as "[12]".
var referenceMark = regexp.MustCompile(`\[\s*\d+\s*\]`)

func quoteTexts(qs []Quote) []string {
	var out []string
	for _, q := range qs {
		out = append(out, q.Text)
	}
	return out
}

func TestSearchFindsQuotes(t *testing.T) {
	tests := []struct {
		name        string
		lang        string
		wantURL     string
		wantMatch   string
		wantQuote   string
		absentParts []string // text of excluded sections and nested sources
	}{
		{
			name:      "english",
			lang:      "en",
			wantURL:   "https://en.wikiquote.org/wiki/Albert_Einstein",
			wantMatch: "Albert Einstein and politics",
			wantQuote: "Everything should be made as simple as possible, but not simpler.",
			absentParts: []string{
				"There are only two ways to live your life", // Disputed
				"I fear the day that technology",            // Misattributed
				"The astonishing thing about Einstein",      // Quotes about Einstein
				"Repeated throughout his life",              // nested source
				"Collected Papers of Albert Einstein from",  // External links
				"God does not care about our mathematical",  // Attributed in posthumous publications
			},
		},
		{
			name:      "spanish",
			lang:      "es",
			wantURL:   "https://es.wikiquote.org/wiki/Albert_Einstein",
			wantMatch: "Einstein y la religión",
			wantQuote: "Dar ejemplo no es la principal manera de influir sobre los demás; es la única manera.",
			absentParts: []string{
				"Villa Carlotta", // Citas sobre Einstein
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFixtureServer(t)
			got, err := fs.client().Search(context.Background(), "  Einstein ", tt.lang)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if got.Person != "Albert Einstein" || got.URL != tt.wantURL {
				t.Errorf("Person, URL = %q, %q; want Albert Einstein, %q", got.Person, got.URL, tt.wantURL)
			}
			if len(got.Matches) != 5 || slices.Contains(got.Matches, "Albert Einstein") || !slices.Contains(got.Matches, tt.wantMatch) {
				t.Errorf("Matches = %q; want 5 other titles with %q", got.Matches, tt.wantMatch)
			}
			if len(got.Quotes) < 10 || len(got.Quotes) > maxQuotes {
				t.Fatalf("got %d quotes, want 10..%d", len(got.Quotes), maxQuotes)
			}
			texts := quoteTexts(got.Quotes)
			if !slices.Contains(texts, tt.wantQuote) {
				t.Errorf("quotes lack %q; first ones: %q", tt.wantQuote, texts[:3])
			}
			for _, q := range got.Quotes {
				if q.Author != "Albert Einstein" || q.Source != "wikiquote" {
					t.Errorf("quote %q has author %q, source %q", q.Text, q.Author, q.Source)
				}
				if n := len([]rune(q.Text)); n < minRunes || n > maxRunes {
					t.Errorf("quote %q has %d runes", q.Text, n)
				}
				if referenceMark.MatchString(q.Text) {
					t.Errorf("quote %q keeps a reference mark", q.Text)
				}
				for _, part := range tt.absentParts {
					if strings.Contains(q.Text, part) {
						t.Errorf("quote %q contains excluded text %q", q.Text, part)
					}
				}
			}
			if pages := fs.parsedPages(); !slices.Equal(pages, []string{"Albert Einstein"}) {
				t.Errorf("parsed pages %q, want the best search title", pages)
			}
		})
	}
}

func TestSearchSendsContactUserAgent(t *testing.T) {
	fs := newFixtureServer(t)
	if _, err := fs.client().Search(context.Background(), "Einstein", "en"); err != nil {
		t.Fatal(err)
	}
	agents := fs.userAgents()
	if len(agents) != 2 {
		t.Fatalf("got %d requests, want search + parse", len(agents))
	}
	for _, ua := range agents {
		if ua != web.ContactUserAgent {
			t.Errorf("User-Agent = %q, want %q", ua, web.ContactUserAgent)
		}
	}
}

func TestSearchFallsBackToEnglish(t *testing.T) {
	for _, lang := range []string{"fr", "", "EN-gb"} {
		t.Run(lang, func(t *testing.T) {
			fs := newFixtureServer(t)
			got, err := fs.client().Search(context.Background(), "Einstein", lang)
			if err != nil {
				t.Fatal(err)
			}
			paths := fs.requestPaths()
			if !strings.HasPrefix(got.URL, "https://en.wikiquote.org/") || len(paths) == 0 || paths[0] != "/en" {
				t.Errorf("URL %q, request paths %q; want English", got.URL, paths)
			}
		})
	}
}

func TestSearchErrors(t *testing.T) {
	jsonBody := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("action") == "query" && strings.Contains(body, `"parse"`) {
				w.Write([]byte(`{"query":{"search":[{"ns":0,"title":"Somebody"}]}}`))
				return
			}
			w.Write([]byte(body))
		}
	}
	tests := []struct {
		name         string
		query        string
		override     http.HandlerFunc
		wantNotFound bool
		wantText     string
	}{
		{name: "empty query", query: " \t\n", wantText: "empty"},
		{name: "no hits", query: "zzqxv nonexistent person", wantNotFound: true, wantText: `no Wikiquote page for "zzqxv nonexistent person"`},
		{
			name:     "API error object",
			query:    "Einstein",
			override: jsonBody(`{"error":{"code":"internal_api_error","info":"boom"}}`),
			wantText: "boom",
		},
		{
			name:     "parse error object",
			query:    "Einstein",
			override: jsonBody(`{"parse":null,"error":{"code":"missingtitle","info":"The page you specified doesn't exist."}}`),
			wantText: "missingtitle",
		},
		{
			name:  "HTTP 500",
			query: "Einstein",
			override: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "oops", http.StatusInternalServerError)
			},
			wantText: "HTTP 500",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFixtureServer(t)
			fs.override = tt.override
			_, err := fs.client().Search(context.Background(), tt.query, "en")
			if err == nil {
				t.Fatal("Search succeeded, want an error")
			}
			if errors.Is(err, ErrNotFound) != tt.wantNotFound {
				t.Errorf("errors.Is(%v, ErrNotFound) = %v, want %v", err, !tt.wantNotFound, tt.wantNotFound)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("error %q lacks %q", err, tt.wantText)
			}
		})
	}
}

func TestSearchEmptyQueryMakesNoRequest(t *testing.T) {
	fs := newFixtureServer(t)
	_, err := fs.client().Search(context.Background(), "   ", "en")
	if !errors.Is(err, ErrEmptyQuery) {
		t.Errorf("err = %v, want ErrEmptyQuery", err)
	}
	if paths := fs.requestPaths(); len(paths) != 0 {
		t.Errorf("made %d requests, want none", len(paths))
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient(nil)
	if c.HTTP == nil {
		t.Error("NewClient(nil) has no HTTP client")
	}
	if got := c.APIURL("es"); got != "https://es.wikiquote.org/w/api.php" {
		t.Errorf("APIURL(es) = %q", got)
	}
}

func TestPageURL(t *testing.T) {
	tests := []struct{ lang, title, want string }{
		{"en", "Albert Einstein", "https://en.wikiquote.org/wiki/Albert_Einstein"},
		{"es", "Jesús de Nazaret", "https://es.wikiquote.org/wiki/Jes%C3%BAs_de_Nazaret"},
		{"en", "What? Me worry", "https://en.wikiquote.org/wiki/What%3F_Me_worry"},
	}
	for _, tt := range tests {
		if got := pageURL(tt.lang, tt.title); got != tt.want {
			t.Errorf("pageURL(%q, %q) = %q, want %q", tt.lang, tt.title, got, tt.want)
		}
	}
}

// fallbackHandler answers a search with titles and a parse of each title
// with a page whose quotes are quotes[title] (none when absent). Titles in
// redirects resolve to the mapped title.
func fallbackHandler(titles []string, quotes map[string][]string, redirects map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("action") == "query" {
			var hits []map[string]string
			for _, t := range titles {
				hits = append(hits, map[string]string{"title": t})
			}
			body, _ := json.Marshal(map[string]any{"query": map[string]any{"search": hits}})
			w.Write(body)
			return
		}
		page := q.Get("page")
		if to, ok := redirects[page]; ok {
			page = to
		}
		html := `<div class="mw-parser-output"><h2>Quotes</h2><ul>`
		for _, text := range quotes[page] {
			html += "<li>" + text + "</li>"
		}
		html += `</ul><h2>See also</h2><ul><li>Some other long page title</li></ul></div>`
		body, _ := json.Marshal(map[string]any{"parse": map[string]string{"title": page, "text": html}})
		w.Write(body)
	}
}

func TestSearchFallsBackToOtherMatches(t *testing.T) {
	const quote = "Imagination is more important than knowledge."
	tests := []struct {
		name        string
		titles      []string
		quotes      map[string][]string
		redirects   map[string]string
		wantPerson  string
		wantMatches []string
		wantQuotes  []string
		wantPages   []string
	}{
		{
			name:        "second title has quotes",
			titles:      []string{"Somebody", "Albert Einstein", "EPR paradox"},
			quotes:      map[string][]string{"Albert Einstein": {quote}},
			wantPerson:  "Albert Einstein",
			wantMatches: []string{"Somebody", "EPR paradox"},
			wantQuotes:  []string{quote},
			wantPages:   []string{"Somebody", "Albert Einstein"},
		},
		{
			name:        "third title through a redirect",
			titles:      []string{"A", "B", "Einstein", "Albert Einstein", "D"},
			quotes:      map[string][]string{"Albert Einstein": {quote}},
			redirects:   map[string]string{"Einstein": "Albert Einstein"},
			wantPerson:  "Albert Einstein",
			wantMatches: []string{"A", "B", "D"},
			wantQuotes:  []string{quote},
			wantPages:   []string{"A", "B", "Einstein"},
		},
		{
			name:        "no title has quotes",
			titles:      []string{"Somebody", "B", "C", "D", "E", "F"},
			quotes:      map[string][]string{"D": {quote}},
			wantPerson:  "Somebody",
			wantMatches: []string{"B", "C", "D", "E", "F"},
			wantQuotes:  []string{},
			wantPages:   []string{"Somebody", "B", "C"},
		},
		{
			name:        "single title without quotes",
			titles:      []string{"Somebody"},
			wantPerson:  "Somebody",
			wantMatches: []string{},
			wantQuotes:  []string{},
			wantPages:   []string{"Somebody"},
		},
		{
			name:        "best title redirect without quotes",
			titles:      []string{"Einstein", "Albert Einstein"},
			redirects:   map[string]string{"Einstein": "Albert Einstein"},
			wantPerson:  "Albert Einstein",
			wantMatches: []string{},
			wantQuotes:  []string{},
			wantPages:   []string{"Einstein"}, // the second title is the page already read
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFixtureServer(t)
			fs.override = fallbackHandler(tt.titles, tt.quotes, tt.redirects)
			got, err := fs.client().Search(context.Background(), "Einstein", "en")
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if got.Person != tt.wantPerson || got.URL != pageURL("en", tt.wantPerson) {
				t.Errorf("Person, URL = %q, %q; want %q", got.Person, got.URL, tt.wantPerson)
			}
			if got.Matches == nil || !slices.Equal(got.Matches, tt.wantMatches) {
				t.Errorf("Matches = %#v, want %q", got.Matches, tt.wantMatches)
			}
			if got.Quotes == nil || !slices.Equal(quoteTexts(got.Quotes), tt.wantQuotes) {
				t.Errorf("Quotes = %#v, want %q", got.Quotes, tt.wantQuotes)
			}
			if pages := fs.parsedPages(); !slices.Equal(pages, tt.wantPages) {
				t.Errorf("parsed pages %q, want %q", pages, tt.wantPages)
			}
		})
	}
}

func TestSearchCapsParseRequests(t *testing.T) {
	fs := newFixtureServer(t)
	fs.override = fallbackHandler([]string{"A", "B", "C", "D", "E", "F"}, nil, nil)
	got, err := fs.client().Search(context.Background(), "nobody", "en")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if n := len(fs.parsedPages()); n != maxParses {
		t.Errorf("made %d parse requests, want %d", n, maxParses)
	}
	if len(got.Quotes) != 0 || got.Person != "A" {
		t.Errorf("got %+v, want no quotes on A", got)
	}
}

func TestSearchFallbackStopsOnError(t *testing.T) {
	tests := []struct {
		name      string
		failPage  string
		wantErr   bool
		wantPages []string
	}{
		{name: "fallback parse fails keeps the best page", failPage: "B", wantPages: []string{"A", "B"}},
		{name: "first parse fails", failPage: "A", wantErr: true, wantPages: []string{"A"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFixtureServer(t)
			ok := fallbackHandler([]string{"A", "B", "C"}, map[string][]string{"C": {"A quote long enough to be kept."}}, nil)
			fs.override = func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") == tt.failPage {
					http.Error(w, "oops", http.StatusInternalServerError)
					return
				}
				ok(w, r)
			}
			got, err := fs.client().Search(context.Background(), "x", "en")
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
					t.Errorf("err = %v, want the HTTP 500 of the first parse", err)
				}
			} else {
				if err != nil {
					t.Fatalf("Search: %v", err)
				}
				if got.Person != "A" || got.Quotes == nil || len(got.Quotes) != 0 || !slices.Equal(got.Matches, []string{"B", "C"}) {
					t.Errorf("got %+v, want A with no quotes and matches B, C", got)
				}
			}
			if pages := fs.parsedPages(); !slices.Equal(pages, tt.wantPages) {
				t.Errorf("parsed pages %q, want %q (no request after the failure)", pages, tt.wantPages)
			}
		})
	}
}

func TestSearchSkipsRepeatedTitles(t *testing.T) {
	const quote = "Imagination is more important than knowledge."
	fs := newFixtureServer(t)
	// "Einstein" redirects, so its resolved title differs from the search
	// title; the repeated hit must not spend a parse.
	fs.override = fallbackHandler(
		[]string{"Einstein", "Einstein", "B", "C"},
		map[string][]string{"C": {quote}},
		map[string]string{"Einstein": "Albert Einstein"},
	)
	got, err := fs.client().Search(context.Background(), "Einstein", "en")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got.Person != "C" || !slices.Equal(quoteTexts(got.Quotes), []string{quote}) {
		t.Errorf("got %+v, want the quotes of C", got)
	}
	if pages := fs.parsedPages(); !slices.Equal(pages, []string{"Einstein", "B", "C"}) {
		t.Errorf("parsed pages %q, want Einstein, B, C", pages)
	}
}

func TestSearchSendsTheNameLiterally(t *testing.T) {
	tests := []struct {
		name, query string
		want        []string // srsearch sent; nil means no request
	}{
		{"leading minus", "-Einstein", []string{"Einstein"}},
		{"quoted phrase", `"Mark Twain"`, []string{"Mark Twain"}},
		{"keyword prefix", "intitle:Gandhi", []string{"intitle Gandhi"}},
		{"operator word", "Simon AND Garfunkel", []string{"Simon and Garfunkel"}},
		{"plain name", "Albert Einstein", []string{"Albert Einstein"}},
		// Nothing is left to search for.
		{"syntax only", `"-*~?"`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFixtureServer(t)
			var mu sync.Mutex
			var sent []string
			fs.override = func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				sent = append(sent, r.URL.Query().Get("srsearch"))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"batchcomplete":"","query":{"search":[]}}`))
			}
			_, err := fs.client().Search(context.Background(), tt.query, "en")
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if !slices.Equal(sent, tt.want) {
				t.Errorf("srsearch = %q, want %q", sent, tt.want)
			}
			if n := len(fs.requestPaths()); n != len(tt.want) {
				t.Errorf("requests = %d, want %d", n, len(tt.want))
			}
		})
	}
}
