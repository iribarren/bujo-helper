// Package providers implements cover image sources.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"bujo-helper/internal/isbn"
	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// Apple searches the public iTunes Search API: Apple Books by default, or
// music albums. It returns the original artwork resolution.
type Apple struct {
	Client  *http.Client
	BaseURL string
	// Media is the iTunes media type to search, e.g. "ebook".
	Media string
	// Entity narrows the results within Media, e.g. "album". Empty sends
	// no entity parameter.
	Entity string
}

// NewApple returns an Apple Books provider using client (or a default one).
func NewApple(client *http.Client) *Apple {
	if client == nil {
		client = web.NewClient()
	}
	return &Apple{Client: client, BaseURL: "https://itunes.apple.com/search", Media: "ebook"}
}

// NewAppleAlbums returns an Apple provider that searches music albums.
func NewAppleAlbums(client *http.Client) *Apple {
	a := NewApple(client)
	a.Media, a.Entity = "music", "album"
	return a
}

// albums reports whether a searches music albums rather than tracks or books.
func (a *Apple) albums() bool { return a.Entity == "album" }

// Name implements media.Provider.
func (a *Apple) Name() string { return "apple" }

type appleResponse struct {
	Results []struct {
		TrackName string `json:"trackName"`
		// CollectionName is the album title in album searches.
		CollectionName string `json:"collectionName"`
		ArtistName     string `json:"artistName"`
		ReleaseDate    string `json:"releaseDate"`
		ArtworkURL100  string `json:"artworkUrl100"`
	} `json:"results"`
}

// Search implements media.Provider.
func (a *Apple) Search(ctx context.Context, item media.Item, opts media.Options) ([]media.Candidate, error) {
	q := url.Values{
		"term":    {searchTerm(item)},
		"media":   {a.Media},
		"limit":   {"25"},
		"country": {country(opts)},
	}
	if a.Entity != "" {
		q.Set("entity", a.Entity)
	}
	var body appleResponse
	if err := getJSON(ctx, a.Client, a.BaseURL+"?"+q.Encode(), &body); err != nil {
		return nil, err
	}

	var hits []media.Candidate
	for _, r := range body.Results {
		if r.ArtworkURL100 == "" {
			continue
		}
		c := media.Candidate{
			Title:   r.TrackName,
			Creator: r.ArtistName,
			Year:    year(r.ReleaseDate),
			// Asking for an oversized box returns the original artwork resolution.
			ImageURLs: []string{strings.Replace(r.ArtworkURL100, "100x100bb", "10000x10000bb", 1)},
			Source:    a.Name(),
		}
		if a.albums() {
			c.Title = r.CollectionName
		} else {
			// The artwork file is usually named after the edition ISBN-13.
			c.ISBN = isbn.Find(r.ArtworkURL100)
		}
		hits = append(hits, c)
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no results with artwork", media.ErrNotFound)
	}
	return rankCandidates(hits, item), nil
}

// --- helpers shared by the providers ---

// searchTerm joins title and creator for free-text search engines.
func searchTerm(item media.Item) string {
	return strings.TrimSpace(strings.TrimSpace(item.Title) + " " + strings.TrimSpace(item.Creator))
}

// country returns the lowercase store country, defaulting to "us".
func country(opts media.Options) string {
	if c := strings.ToLower(strings.TrimSpace(opts.Country)); c != "" {
		return c
	}
	return "us"
}

// year returns the leading four-digit year of a date string, if any.
func year(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return ""
}

func rankCandidates(cs []media.Candidate, item media.Item) []media.Candidate {
	return match.Rank(cs, item,
		func(c media.Candidate) string { return c.Title },
		func(c media.Candidate) string { return c.Creator })
}

// httpStatusError reports a non-2xx response.
type httpStatusError struct{ Status int }

func (e *httpStatusError) Error() string { return fmt.Sprintf("HTTP %d", e.Status) }

// getJSON fetches u with the API headers and decodes the JSON body into v.
func getJSON(ctx context.Context, client *http.Client, u string, v any) error {
	return getJSONWith(ctx, client, u, web.SetAPIHeaders, v)
}

// getJSONWith fetches u with the given header setter and decodes the JSON
// body into v. A non-2xx status gives an *httpStatusError.
func getJSONWith(ctx context.Context, client *http.Client, u string, setHeaders func(*http.Request), v any) error {
	resp, err := web.Get(ctx, client, u, setHeaders)
	if err != nil {
		return err
	}
	if !resp.OK() {
		return &httpStatusError{Status: resp.Status}
	}
	if err := json.Unmarshal(resp.Body, v); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
