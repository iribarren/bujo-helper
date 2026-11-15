package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// IMDb searches IMDb's public title suggestion endpoint (keyless and
// unofficial). Its images are full-size posters.
type IMDb struct {
	Client  *http.Client
	BaseURL string
}

// NewIMDb returns an IMDb provider using client (or a default one).
func NewIMDb(client *http.Client) *IMDb {
	if client == nil {
		client = web.NewClient()
	}
	return &IMDb{Client: client, BaseURL: "https://v2.sg.media-imdb.com/suggestion"}
}

// Name implements media.Provider.
func (p *IMDb) Name() string { return "imdb" }

// imdbMovieTypes are the suggestion kinds (qid) kept as movies.
var imdbMovieTypes = map[string]bool{"movie": true, "tvMovie": true, "short": true, "video": true}

type imdbResponse struct {
	D []struct {
		ID    string `json:"id"`
		Title string `json:"l"`
		Year  int    `json:"y"`
		Kind  string `json:"qid"`
		Image struct {
			URL string `json:"imageUrl"`
		} `json:"i"`
	} `json:"d"`
}

// Search implements media.Provider. item.Creator holds the release year,
// if known; it breaks ranking ties.
func (p *IMDb) Search(ctx context.Context, item media.Item, opts media.Options) ([]media.Candidate, error) {
	query := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(item.Title)), " ", "_")
	if query == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	first := "x"
	if c := query[0]; c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
		first = string(c)
	}
	u := p.BaseURL + "/" + first + "/" + url.PathEscape(query) + ".json"

	var body imdbResponse
	if err := getJSONWith(ctx, p.Client, u, web.SetBrowserHeaders, &body); err != nil {
		return nil, err
	}

	var hits []media.Candidate
	for _, d := range body.D {
		if !strings.HasPrefix(d.ID, "tt") || !imdbMovieTypes[d.Kind] || d.Image.URL == "" {
			continue
		}
		y := ""
		if d.Year > 0 {
			y = strconv.Itoa(d.Year)
		}
		hits = append(hits, media.Candidate{
			Title:     d.Title,
			Creator:   y,
			Year:      y,
			ImageURLs: []string{d.Image.URL},
			Source:    p.Name(),
		})
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no movie results with a poster", media.ErrNotFound)
	}
	return rankCandidates(hits, item), nil
}
