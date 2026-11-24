package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// WikidataITunes finds films on Wikidata by title, reads their iTunes movie
// IDs (property P6398) and fetches the posters through the keyless iTunes
// lookup API. It makes at most four requests per item: search, entities,
// lookup and one US lookup retry.
type WikidataITunes struct {
	Client      *http.Client
	WikidataURL string
	ITunesURL   string
}

// NewWikidataITunes returns a Wikidata → iTunes provider using client (or a
// default one).
func NewWikidataITunes(client *http.Client) *WikidataITunes {
	if client == nil {
		client = web.NewClient()
	}
	return &WikidataITunes{
		Client:      client,
		WikidataURL: "https://www.wikidata.org/w/api.php",
		ITunesURL:   "https://itunes.apple.com/lookup",
	}
}

// Name implements media.Provider.
func (p *WikidataITunes) Name() string { return "wikidata-itunes" }

// Label implements media.Labeler.
func (*WikidataITunes) Label() string { return "Wikidata/iTunes" }

// wikidataSearchLimit caps the entities read per search.
const wikidataSearchLimit = 10

// wikidataError is the error object the MediaWiki API returns with HTTP 200.
type wikidataError struct {
	Code string `json:"code"`
	Info string `json:"info"`
}

func (e *wikidataError) err() error {
	if e == nil {
		return nil
	}
	return fmt.Errorf("wikidata API error %s: %s", e.Code, e.Info)
}

type wikidataSearchResponse struct {
	Error *wikidataError `json:"error"`
	Query struct {
		Search []struct {
			Title string `json:"title"`
		} `json:"search"`
	} `json:"query"`
}

// wikidataSnak is a statement's main value; Value is a string (external
// IDs) or an object (times).
type wikidataSnak struct {
	Mainsnak struct {
		Datavalue struct {
			Value any `json:"value"`
		} `json:"datavalue"`
	} `json:"mainsnak"`
}

type wikidataEntitiesResponse struct {
	Error    *wikidataError `json:"error"`
	Entities map[string]struct {
		Labels map[string]struct {
			Value string `json:"value"`
		} `json:"labels"`
		Claims map[string][]wikidataSnak `json:"claims"`
	} `json:"entities"`
}

type itunesLookupResponse struct {
	Results []struct {
		TrackID       int64  `json:"trackId"`
		TrackName     string `json:"trackName"`
		ReleaseDate   string `json:"releaseDate"`
		ArtworkURL100 string `json:"artworkUrl100"`
	} `json:"results"`
}

// wikidataFilm is an entity with at least one iTunes movie ID.
type wikidataFilm struct {
	label     string
	year      string
	itunesIDs []string
}

// Search implements media.Provider. item.Creator holds the release year,
// if known; it breaks ranking ties.
func (p *WikidataITunes) Search(ctx context.Context, item media.Item, opts media.Options) ([]media.Candidate, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}

	ids, err := p.search(ctx, title)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no Wikidata films", media.ErrNotFound)
	}

	films, err := p.films(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(films) == 0 {
		return nil, fmt.Errorf("%w: no Wikidata films with an iTunes ID", media.ErrNotFound)
	}

	hits, err := p.lookup(ctx, films, country(opts))
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no iTunes results with artwork", media.ErrNotFound)
	}
	return rankCandidates(hits, item), nil
}

// search returns the Q-ids of entities matching title that have an iTunes
// movie ID. Full-text search with haswbstatement is used because the
// label-prefix search (wbsearchentities) ranks films below novels, bands
// and places, often past any useful limit. title is searched for
// literally; a title without words finds nothing.
func (p *WikidataITunes) search(ctx context.Context, title string) ([]string, error) {
	return wikidataItemSearch(ctx, p.Client, p.WikidataURL, title, "haswbstatement:P6398")
}

// films reads the entities and keeps, in ids order, those with an iTunes
// movie ID.
func (p *WikidataITunes) films(ctx context.Context, ids []string) ([]wikidataFilm, error) {
	q := url.Values{
		"action":    {"wbgetentities"},
		"ids":       {strings.Join(ids, "|")},
		"props":     {"claims|labels"},
		"languages": {"en"},
		// Many films only have a language-neutral ("mul") label.
		"languagefallback": {"1"},
		"format":           {"json"},
	}
	var body wikidataEntitiesResponse
	if err := getJSONWith(ctx, p.Client, p.WikidataURL+"?"+q.Encode(), web.SetWikimediaHeaders, &body); err != nil {
		return nil, err
	}
	if err := body.Error.err(); err != nil {
		return nil, err
	}

	var films []wikidataFilm
	for _, id := range ids {
		e, ok := body.Entities[id]
		if !ok {
			continue
		}
		f := wikidataFilm{label: e.Labels["en"].Value}
		for _, c := range e.Claims["P6398"] {
			if v, ok := c.Mainsnak.Datavalue.Value.(string); ok && v != "" {
				f.itunesIDs = append(f.itunesIDs, v)
			}
		}
		if len(f.itunesIDs) == 0 {
			continue
		}
		for _, c := range e.Claims["P577"] {
			v, _ := c.Mainsnak.Datavalue.Value.(map[string]any)
			t, _ := v["time"].(string)
			if y := wikidataYear(t); y != "" && (f.year == "" || y < f.year) {
				f.year = y
			}
		}
		films = append(films, f)
	}
	return films, nil
}

// lookup fetches all the films' iTunes IDs in one call, retrying the US
// store once when another store knows none of them. Each film yields at
// most one candidate: its first ID with artwork.
func (p *WikidataITunes) lookup(ctx context.Context, films []wikidataFilm, cc string) ([]media.Candidate, error) {
	var ids []string
	seen := map[string]bool{}
	for _, f := range films {
		for _, id := range f.itunesIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}

	body, err := p.lookupIDs(ctx, ids, cc)
	if err != nil {
		return nil, err
	}
	if len(body.Results) == 0 && cc != "us" {
		if body, err = p.lookupIDs(ctx, ids, "us"); err != nil {
			return nil, err
		}
	}

	type track struct{ name, released, artwork string }
	tracks := map[string]track{}
	for _, r := range body.Results {
		if r.ArtworkURL100 == "" {
			continue
		}
		tracks[strconv.FormatInt(r.TrackID, 10)] = track{r.TrackName, r.ReleaseDate, r.ArtworkURL100}
	}

	var hits []media.Candidate
	for _, f := range films {
		for _, id := range f.itunesIDs {
			t, ok := tracks[id]
			if !ok {
				continue
			}
			title, y := f.label, f.year
			if title == "" {
				title = t.name
			}
			if y == "" {
				y = year(t.released)
			}
			hits = append(hits, media.Candidate{
				Title:   title,
				Creator: y,
				Year:    y,
				// An oversized box returns the original artwork (usually
				// 2000×3000); 600x600bb is a smaller fallback.
				ImageURLs: []string{
					strings.Replace(t.artwork, "100x100bb", "9999x9999bb", 1),
					strings.Replace(t.artwork, "100x100bb", "600x600bb", 1),
				},
				Source: p.Name(),
			})
			break
		}
	}
	return hits, nil
}

func (p *WikidataITunes) lookupIDs(ctx context.Context, ids []string, cc string) (*itunesLookupResponse, error) {
	q := url.Values{
		"id":      {strings.Join(ids, ",")},
		"country": {cc},
		"entity":  {"movie"},
	}
	var body itunesLookupResponse
	if err := getJSON(ctx, p.Client, p.ITunesURL+"?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	return &body, nil
}

// wikidataYear returns the year of a Wikidata time value such as
// "+2021-09-03T00:00:00Z". Years before the common era give "".
func wikidataYear(t string) string {
	y, ok := strings.CutPrefix(t, "+")
	if !ok || len(y) < 5 || y[4] != '-' || !isYear(y[:4]) {
		return ""
	}
	return y[:4]
}
