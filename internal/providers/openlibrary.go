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

// OpenLibrary searches openlibrary.org and its covers API. Images are
// smaller than Apple's but the catalogue is free and open.
type OpenLibrary struct {
	Client       *http.Client
	BaseURL      string
	CoverBaseURL string
}

// NewOpenLibrary returns an Open Library provider using client (or a default one).
func NewOpenLibrary(client *http.Client) *OpenLibrary {
	if client == nil {
		client = web.NewClient()
	}
	return &OpenLibrary{
		Client:       client,
		BaseURL:      "https://openlibrary.org/search.json",
		CoverBaseURL: "https://covers.openlibrary.org",
	}
}

// Name implements media.Provider.
func (o *OpenLibrary) Name() string { return "openlibrary" }

type openLibraryResponse struct {
	Docs []struct {
		Title            string   `json:"title"`
		AuthorName       []string `json:"author_name"`
		CoverI           int64    `json:"cover_i"`
		FirstPublishYear int      `json:"first_publish_year"`
	} `json:"docs"`
}

// Search implements media.Provider.
func (o *OpenLibrary) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	q := url.Values{
		"title":  {strings.TrimSpace(item.Title)},
		"limit":  {"25"},
		"fields": {"title,author_name,cover_i,first_publish_year"},
	}
	if creator := strings.TrimSpace(item.Creator); creator != "" {
		q.Set("author", creator)
	}
	var body openLibraryResponse
	if err := getJSON(ctx, o.Client, o.BaseURL+"?"+q.Encode(), &body); err != nil {
		return nil, err
	}

	var hits []media.Candidate
	for _, d := range body.Docs {
		if d.CoverI == 0 {
			continue
		}
		authors := d.AuthorName
		if len(authors) > 2 {
			authors = authors[:2]
		}
		var yr string
		if d.FirstPublishYear > 0 {
			yr = strconv.Itoa(d.FirstPublishYear)
		}
		hits = append(hits, media.Candidate{
			Title:   d.Title,
			Creator: strings.Join(authors, ", "),
			Year:    yr,
			// default=false makes a missing cover a 404 instead of a blank image.
			ImageURLs: []string{fmt.Sprintf("%s/b/id/%d-L.jpg?default=false", o.CoverBaseURL, d.CoverI)},
			Source:    o.Name(),
		})
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no results with a cover", media.ErrNotFound)
	}
	return rankCandidates(hits, item), nil
}
