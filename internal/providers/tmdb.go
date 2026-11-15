package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// ErrNoTMDBKey reports that no TMDB API key is configured. It does not wrap
// media.ErrNotFound: the provider is skipped, not searched.
var ErrNoTMDBKey = errors.New("no TMDB API key (set it in Settings)")

// TMDB searches The Movie Database (API v3). It needs an API key: a v3 key
// or a v4 read access token.
type TMDB struct {
	Client  *http.Client
	BaseURL string
	// ImageBase is the image CDN root; a size and the poster path follow.
	ImageBase string
	// Key returns the current API key. Nil or blank skips the provider.
	Key func() string
}

// NewTMDB returns a TMDB provider using client (or a default one) and the
// key source.
func NewTMDB(client *http.Client, key func() string) *TMDB {
	if client == nil {
		client = web.NewClient()
	}
	return &TMDB{
		Client:    client,
		BaseURL:   "https://api.themoviedb.org/3",
		ImageBase: "https://image.tmdb.org/t/p",
		Key:       key,
	}
}

// Name implements media.Provider.
func (p *TMDB) Name() string { return "tmdb" }

type tmdbResponse struct {
	Results []struct {
		Title         string `json:"title"`
		OriginalTitle string `json:"original_title"`
		ReleaseDate   string `json:"release_date"`
		PosterPath    string `json:"poster_path"`
	} `json:"results"`
}

// Search implements media.Provider. item.Creator holds the release year,
// if known; it filters the search and breaks ranking ties.
func (p *TMDB) Search(ctx context.Context, item media.Item, opts media.Options) ([]media.Candidate, error) {
	key := ""
	if p.Key != nil {
		key = strings.TrimSpace(p.Key())
	}
	if key == "" {
		return nil, ErrNoTMDBKey
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}

	cc := country(opts)
	q := url.Values{
		"query":         {title},
		"include_adult": {"false"},
		"language":      {tmdbLanguage(opts)},
		"region":        {strings.ToUpper(cc)},
	}
	bearer := isTMDBToken(key)
	if !bearer {
		q.Set("api_key", key)
	}
	setHeaders := func(req *http.Request) {
		web.SetAPIHeaders(req)
		req.Header.Set("Accept", "application/json")
		if bearer {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}

	itemYear := strings.TrimSpace(item.Creator)
	if !isYear(itemYear) {
		itemYear = ""
	}
	if itemYear != "" {
		q.Set("year", itemYear)
	}
	body, err := p.search(ctx, q, setHeaders, key)
	if err != nil {
		return nil, err
	}
	hits := p.posterCandidates(body)
	if len(hits) == 0 && itemYear != "" {
		// TMDB's year can be a year off (festival releases), and a match in
		// that year may lack a poster; the ranking still prefers the
		// requested year.
		q.Del("year")
		if body, err = p.search(ctx, q, setHeaders, key); err != nil {
			return nil, err
		}
		hits = p.posterCandidates(body)
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no results with a poster", media.ErrNotFound)
	}
	return rankCandidates(hits, item), nil
}

// posterCandidates converts the results that have a poster.
func (p *TMDB) posterCandidates(body *tmdbResponse) []media.Candidate {
	var hits []media.Candidate
	for _, r := range body.Results {
		if r.PosterPath == "" {
			continue
		}
		y := year(r.ReleaseDate)
		hits = append(hits, media.Candidate{
			Title:   r.Title,
			Creator: y,
			Year:    y,
			ImageURLs: []string{
				p.ImageBase + "/original" + r.PosterPath,
				p.ImageBase + "/w780" + r.PosterPath,
			},
			Source: p.Name(),
		})
	}
	return hits
}

func (p *TMDB) search(ctx context.Context, q url.Values, setHeaders func(*http.Request), key string) (*tmdbResponse, error) {
	var body tmdbResponse
	err := getJSONWith(ctx, p.Client, p.BaseURL+"/search/movie?"+q.Encode(), setHeaders, &body)
	// A v3 key travels in the URL, and transport errors quote the URL.
	err = redactKey(err, key)
	var status *httpStatusError
	if errors.As(err, &status) && status.Status == http.StatusUnauthorized {
		return nil, errors.New("TMDB rejected the API key (HTTP 401)")
	}
	if err != nil {
		return nil, err
	}
	return &body, nil
}

// redactedError is an error whose text had the API key replaced.
type redactedError struct {
	msg   string
	cause error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.cause }

// redactKey replaces key in err's text with "REDACTED". The result unwraps
// to the cause behind a *url.Error (which holds the URL), so errors.Is
// still finds context.Canceled or a timeout without exposing the key.
func redactKey(err error, key string) error {
	if err == nil || key == "" {
		return err
	}
	msg := err.Error()
	if !strings.Contains(msg, key) && !strings.Contains(msg, url.QueryEscape(key)) {
		return err
	}
	cause := err
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		cause = urlErr.Err
	}
	if cause != nil && strings.Contains(cause.Error(), key) {
		cause = nil
	}
	msg = strings.ReplaceAll(msg, key, "REDACTED")
	msg = strings.ReplaceAll(msg, url.QueryEscape(key), "REDACTED")
	return &redactedError{msg: msg, cause: cause}
}

// isTMDBToken reports whether key looks like a v4 read access token (a
// JWT) rather than a 32-character v3 key.
func isTMDBToken(key string) bool {
	return strings.Contains(key, ".") && len(key) > 40
}

// isYear reports whether s is a four-digit year.
func isYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// tmdbLanguages maps a store country to the TMDB language tag used for
// localized titles and posters. Unlisted countries use en-US.
var tmdbLanguages = map[string]string{
	"es": "es-ES", "mx": "es-MX", "ar": "es-AR", "co": "es-CO", "cl": "es-CL", "pe": "es-PE",
	"fr": "fr-FR",
	"de": "de-DE", "at": "de-DE",
	"it": "it-IT",
	"pt": "pt-PT", "br": "pt-BR",
}

// tmdbLanguage returns the TMDB language tag for opts.Country.
func tmdbLanguage(opts media.Options) string {
	if lang, ok := tmdbLanguages[country(opts)]; ok {
		return lang
	}
	return "en-US"
}
