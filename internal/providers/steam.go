package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"cover-downloader/internal/match"
	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// steamMaxTieLookups caps the release-date lookups made to break a title
// tie, so a search costs at most this many extra requests.
const steamMaxTieLookups = 3

// Steam searches the Steam store (keyless and undocumented) and takes the
// portrait library capsule from the Steam CDN. It only knows PC games sold
// on Steam. Its search has no release dates, so when a year is given and
// several results share the best title, their release dates are looked up
// to pick the requested year.
type Steam struct {
	Client *http.Client
	// BaseURL is the store API root; "/storesearch/" and "/appdetails"
	// follow.
	BaseURL string
	// ImageBase is the CDN root for app images; the app ID and the image
	// name follow.
	ImageBase string
}

// NewSteam returns a Steam store provider using client (or a default one).
func NewSteam(client *http.Client) *Steam {
	if client == nil {
		client = web.NewClient()
	}
	return &Steam{
		Client:    client,
		BaseURL:   "https://store.steampowered.com/api",
		ImageBase: "https://cdn.cloudflare.steamstatic.com/steam/apps",
	}
}

// Name implements media.Provider.
func (p *Steam) Name() string { return "steam" }

type steamSearchResponse struct {
	Items []struct {
		Type string `json:"type"`
		Name string `json:"name"`
		ID   int64  `json:"id"`
	} `json:"items"`
}

// steamHit is a candidate and its app ID, kept for the release-date lookup.
type steamHit struct {
	media.Candidate
	appID string
}

// Search implements media.Provider. item.Creator holds the release year,
// if known; it breaks title ties.
func (p *Steam) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	// The US store has the widest catalog and English titles.
	q := url.Values{"term": {title}, "l": {"english"}, "cc": {"US"}}
	var body steamSearchResponse
	if err := getJSON(ctx, p.Client, p.BaseURL+"/storesearch/?"+q.Encode(), &body); err != nil {
		return nil, err
	}

	var hits []steamHit
	for _, it := range body.Items {
		// Packages ("sub") and bundles have no library capsule.
		if it.Type != "app" || it.ID <= 0 {
			continue
		}
		id := strconv.FormatInt(it.ID, 10)
		// The 2x capsule is 1200 × 1800; older games only have the 600 × 900
		// one, and the downloader moves on after a 404.
		base := p.ImageBase + "/" + id
		hits = append(hits, steamHit{
			Candidate: media.Candidate{
				Title:     it.Name,
				ImageURLs: []string{base + "/library_600x900_2x.jpg", base + "/library_600x900.jpg"},
				Source:    p.Name(),
			},
			appID: id,
		})
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no games", media.ErrNotFound)
	}
	// Search results have no creator, so only the title ranks them here.
	hits = rankGames(hits, media.Item{Title: title},
		func(h steamHit) string { return h.Title },
		func(steamHit) string { return "" })
	if y := strings.TrimSpace(item.Creator); isYear(y) {
		p.breakTies(ctx, hits, y)
	}

	out := make([]media.Candidate, len(hits))
	for i, h := range hits {
		out[i] = h.Candidate
	}
	return out, nil
}

// breakTies moves the first of the leading results that share the top
// title and were released in year to the front. It looks up the release
// dates of at most steamMaxTieLookups of them; a failed lookup only leaves
// that result without a year.
func (p *Steam) breakTies(ctx context.Context, hits []steamHit, year string) {
	top := match.Normalize(hits[0].Title)
	n := 1
	for n < len(hits) && n < steamMaxTieLookups && match.Normalize(hits[n].Title) == top {
		n++
	}
	if n < 2 {
		return
	}
	for i := range n {
		y := p.releaseYear(ctx, hits[i].appID)
		hits[i].Year, hits[i].Creator = y, y
	}
	for i := range n {
		if hits[i].Year == year {
			h := hits[i]
			copy(hits[1:i+1], hits[:i])
			hits[0] = h
			return
		}
	}
}

// releaseYear returns an app's release year from the store's app details,
// or "" when it is unknown or the lookup fails.
func (p *Steam) releaseYear(ctx context.Context, appID string) string {
	q := url.Values{"appids": {appID}, "filters": {"release_date"}}
	// An unknown app has "data": [] instead of an object, so data is
	// decoded only on success.
	var body map[string]struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := getJSON(ctx, p.Client, p.BaseURL+"/appdetails?"+q.Encode(), &body); err != nil {
		return ""
	}
	app, ok := body[appID]
	if !ok || !app.Success {
		return ""
	}
	var data struct {
		ReleaseDate struct {
			Date string `json:"date"`
		} `json:"release_date"`
	}
	if err := json.Unmarshal(app.Data, &data); err != nil {
		return ""
	}
	return steamYear(data.ReleaseDate.Date)
}

// rankGames ranks game hits like match.Rank, but an exact full-title match
// beats a sequel or edition whose main title matches: "Hollow Knight"
// comes before "Hollow Knight: Silksong", which match.Rank scores the
// same.
func rankGames[T any](hits []T, item media.Item, title, creator func(T) string) []T {
	query := match.Normalize(item.Title)
	exact := make([]T, 0, len(hits))
	var rest []T
	for _, h := range hits {
		if match.Normalize(title(h)) == query {
			exact = append(exact, h)
		} else {
			rest = append(rest, h)
		}
	}
	// match.Rank is a stable sort, so exact matches stay ahead of equally
	// scored ones.
	return match.Rank(append(exact, rest...), item, title, creator)
}

// steamYearPattern finds a standalone four-digit year.
var steamYearPattern = regexp.MustCompile(`\b\d{4}\b`)

// steamYear returns the year in a store release date such as
// "24 Feb, 2017", or "" when there is none ("Coming soon").
func steamYear(date string) string {
	return steamYearPattern.FindString(date)
}
