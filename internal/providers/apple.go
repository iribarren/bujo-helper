// Package providers implements cover image sources.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cover-downloader/internal/match"
	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// Apple searches Apple Books through the public iTunes Search API. It returns
// the original artwork resolution.
type Apple struct {
	Client  *http.Client
	BaseURL string
	// Media is the iTunes media type to search, e.g. "ebook".
	Media string
}

// NewApple returns an Apple Books provider using client (or a default one).
func NewApple(client *http.Client) *Apple {
	if client == nil {
		client = web.NewClient()
	}
	return &Apple{Client: client, BaseURL: "https://itunes.apple.com/search", Media: "ebook"}
}

// Name implements media.Provider.
func (a *Apple) Name() string { return "apple" }

type appleResponse struct {
	Results []struct {
		TrackName     string `json:"trackName"`
		ArtistName    string `json:"artistName"`
		ReleaseDate   string `json:"releaseDate"`
		ArtworkURL100 string `json:"artworkUrl100"`
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
	var body appleResponse
	if err := getJSON(ctx, a.Client, a.BaseURL+"?"+q.Encode(), &body); err != nil {
		return nil, err
	}

	var hits []media.Candidate
	for _, r := range body.Results {
		if r.ArtworkURL100 == "" {
			continue
		}
		hits = append(hits, media.Candidate{
			Title:   r.TrackName,
			Creator: r.ArtistName,
			Year:    year(r.ReleaseDate),
			// Asking for an oversized box returns the original artwork resolution.
			ImageURLs: []string{strings.Replace(r.ArtworkURL100, "100x100bb", "10000x10000bb", 1)},
			Source:    a.Name(),
		})
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

func getJSON(ctx context.Context, client *http.Client, u string, v any) error {
	resp, err := web.Get(ctx, client, u, web.SetAPIHeaders)
	if err != nil {
		return err
	}
	if !resp.OK() {
		return fmt.Errorf("HTTP %d", resp.Status)
	}
	if err := json.Unmarshal(resp.Body, v); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
