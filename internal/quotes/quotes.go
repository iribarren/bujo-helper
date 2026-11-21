// Package quotes finds a person's quotes on Wikiquote (English or Spanish)
// through the MediaWiki Action API.
package quotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"bujo-helper/internal/web"
)

// SourceWikiquote is the Source of every quote fetched from Wikiquote.
const SourceWikiquote = "wikiquote"

// searchLimit is how many page titles the full-text search asks for: the
// best one plus up to five other matches.
const searchLimit = 6

// maxMatches caps Result.Matches.
const maxMatches = 5

var (
	// ErrNotFound is returned when no page matches the query or the page
	// has no quotes.
	ErrNotFound = errors.New("no quotes found")
	// ErrEmptyQuery is returned for a query with nothing but spaces.
	ErrEmptyQuery = errors.New("empty query")
)

// Quote is one quotation. Source is SourceWikiquote for fetched quotes.
type Quote struct {
	Text   string `json:"text"`
	Author string `json:"author"`
	Source string `json:"source"`
}

// Result is the outcome of a search: the person whose page was read, the
// page URL, the other pages the search found and the page's quotes.
type Result struct {
	Person  string   `json:"person"`
	URL     string   `json:"url"`
	Matches []string `json:"matches"`
	Quotes  []Quote  `json:"quotes"`
}

// Client searches Wikiquote. Requests are made one at a time with the
// contact User-Agent the Wikimedia policy asks for.
type Client struct {
	HTTP *http.Client
	// APIURL returns the Action API endpoint of a Wikiquote language
	// ("en" or "es"); tests point it at a local server.
	APIURL func(lang string) string
}

// NewClient returns a Client using client, or a default one when nil.
func NewClient(client *http.Client) *Client {
	if client == nil {
		client = web.NewClient()
	}
	return &Client{HTTP: client, APIURL: defaultAPIURL}
}

func defaultAPIURL(lang string) string {
	return "https://" + lang + ".wikiquote.org/w/api.php"
}

// normalizeLang returns "es" for Spanish and "en" for anything else.
func normalizeLang(lang string) string {
	if strings.ToLower(strings.TrimSpace(lang)) == "es" {
		return "es"
	}
	return "en"
}

// pageURL returns the public URL of the Wikiquote page title.
func pageURL(lang, title string) string {
	return "https://" + lang + ".wikiquote.org/wiki/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
}

// Search finds the Wikiquote page that best matches query in lang ("en" or
// "es"; anything else means "en") and returns its quotes. A query with no
// page, or a page without quotes, gives an error wrapping ErrNotFound.
func (c *Client) Search(ctx context.Context, query, lang string) (Result, error) {
	query = cleanText(query)
	if query == "" {
		return Result{}, fmt.Errorf("wikiquote: %w", ErrEmptyQuery)
	}
	lang = normalizeLang(lang)
	api := c.APIURL
	if api == nil {
		api = defaultAPIURL
	}
	endpoint := api(lang)

	titles, err := c.search(ctx, endpoint, query)
	if err != nil {
		return Result{}, err
	}
	if len(titles) == 0 {
		return Result{}, fmt.Errorf("%w: no Wikiquote page for %q", ErrNotFound, query)
	}
	best := titles[0]
	person, html, err := c.parse(ctx, endpoint, best)
	if err != nil {
		return Result{}, err
	}
	quotes, err := parseQuotes(html, person)
	if err != nil {
		return Result{}, fmt.Errorf("wikiquote: parse %q: %w", person, err)
	}
	if len(quotes) == 0 {
		return Result{}, fmt.Errorf("%w on %q", ErrNotFound, person)
	}
	matches := []string{}
	for _, t := range titles[1:] {
		if t != person && t != best && len(matches) < maxMatches {
			matches = append(matches, t)
		}
	}
	return Result{Person: person, URL: pageURL(lang, person), Matches: matches, Quotes: quotes}, nil
}

// apiError is the error object MediaWiki returns with HTTP 200.
type apiError struct {
	Code string `json:"code"`
	Info string `json:"info"`
}

func (e *apiError) err() error {
	if e == nil {
		return nil
	}
	return fmt.Errorf("wikiquote API error %s: %s", e.Code, e.Info)
}

type searchResponse struct {
	Error *apiError `json:"error"`
	Query struct {
		Search []struct {
			Title string `json:"title"`
		} `json:"search"`
	} `json:"query"`
}

// search returns the titles the full-text search finds for query in the
// main namespace, best first.
func (c *Client) search(ctx context.Context, endpoint, query string) ([]string, error) {
	v := url.Values{
		"action":      {"query"},
		"list":        {"search"},
		"srsearch":    {query},
		"srnamespace": {"0"},
		"srlimit":     {fmt.Sprint(searchLimit)},
		"srprop":      {""},
		"format":      {"json"},
	}
	var body searchResponse
	if err := c.getJSON(ctx, endpoint+"?"+v.Encode(), &body); err != nil {
		return nil, err
	}
	if err := body.Error.err(); err != nil {
		return nil, err
	}
	var titles []string
	for _, s := range body.Query.Search {
		titles = append(titles, s.Title)
	}
	return titles, nil
}

type parseResponse struct {
	Error *apiError `json:"error"`
	Parse *struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"parse"`
}

// parse returns the title (after redirects) and rendered HTML of page.
func (c *Client) parse(ctx context.Context, endpoint, page string) (title, html string, err error) {
	v := url.Values{
		"action":             {"parse"},
		"page":               {page},
		"prop":               {"text"},
		"redirects":          {"1"},
		"disableeditsection": {"1"},
		"format":             {"json"},
		"formatversion":      {"2"},
	}
	var body parseResponse
	if err := c.getJSON(ctx, endpoint+"?"+v.Encode(), &body); err != nil {
		return "", "", err
	}
	if err := body.Error.err(); err != nil {
		return "", "", err
	}
	if body.Parse == nil {
		return "", "", fmt.Errorf("wikiquote: no parse result for %q", page)
	}
	title = body.Parse.Title
	if title == "" {
		title = page
	}
	return title, body.Parse.Text, nil
}

// getJSON fetches u with the Wikimedia headers and decodes the JSON body
// into v. A non-2xx status is an error.
func (c *Client) getJSON(ctx context.Context, u string, v any) error {
	client := c.HTTP
	if client == nil {
		client = web.NewClient()
	}
	resp, err := web.Get(ctx, client, u, web.SetWikimediaHeaders)
	if err != nil {
		return fmt.Errorf("wikiquote: %w", err)
	}
	if !resp.OK() {
		return fmt.Errorf("wikiquote: HTTP %d", resp.Status)
	}
	if err := json.Unmarshal(resp.Body, v); err != nil {
		return fmt.Errorf("wikiquote: decode response: %w", err)
	}
	return nil
}
