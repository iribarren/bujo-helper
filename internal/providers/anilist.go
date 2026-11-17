package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// AniList media types searched by the AniList provider.
const (
	AniListAnime = "ANIME"
	AniListManga = "MANGA"
)

// aniListQuery searches one page of non-adult media of one type, in
// AniList's own relevance order.
const aniListQuery = `query ($search: String, $type: MediaType, $perPage: Int, $isAdult: Boolean) {
  Page(perPage: $perPage) {
    media(search: $search, type: $type, sort: SEARCH_MATCH, isAdult: $isAdult) {
      title { romaji english native }
      synonyms
      startDate { year }
      seasonYear
      coverImage { extraLarge large }
    }
  }
}`

// aniListPerPage is how many results one search asks for.
const aniListPerPage = 10

// AniList searches the AniList GraphQL API for anime or manga. It needs no
// key. Each search is one POST; the shared throttle spaces them and retries
// a 429.
type AniList struct {
	Client *http.Client
	// BaseURL is the GraphQL endpoint.
	BaseURL string
	// Type is AniListAnime or AniListManga.
	Type string
}

// NewAniList returns an AniList provider for mediaType (AniListAnime or
// AniListManga) using client (or a default one).
func NewAniList(client *http.Client, mediaType string) *AniList {
	if client == nil {
		client = web.NewClient()
	}
	return &AniList{
		Client:  client,
		BaseURL: "https://graphql.anilist.co",
		Type:    mediaType,
	}
}

// NewAniListAnime returns an AniList provider that searches anime.
func NewAniListAnime(client *http.Client) *AniList { return NewAniList(client, AniListAnime) }

// NewAniListManga returns an AniList provider that searches manga.
func NewAniListManga(client *http.Client) *AniList { return NewAniList(client, AniListManga) }

// Name implements media.Provider.
func (p *AniList) Name() string { return "anilist" }

type aniListResponse struct {
	Data struct {
		Page struct {
			Media []aniListMedia `json:"media"`
		} `json:"Page"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type aniListMedia struct {
	Title struct {
		Romaji  string `json:"romaji"`
		English string `json:"english"`
		Native  string `json:"native"`
	} `json:"title"`
	Synonyms  []string `json:"synonyms"`
	StartDate struct {
		Year int `json:"year"`
	} `json:"startDate"`
	SeasonYear int `json:"seasonYear"`
	CoverImage struct {
		ExtraLarge string `json:"extraLarge"`
		Large      string `json:"large"`
	} `json:"coverImage"`
}

// aniListHit is a candidate with the title that best matches the query.
type aniListHit struct {
	candidate media.Candidate
	best      string
}

// Search implements media.Provider. item.Creator holds the start year, if
// known; it breaks ranking ties.
func (p *AniList) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	payload, err := json.Marshal(map[string]any{
		"query": aniListQuery,
		"variables": map[string]any{
			"search":  title,
			"type":    p.Type,
			"perPage": aniListPerPage,
			"isAdult": false,
		},
	})
	if err != nil {
		return nil, err
	}
	var body aniListResponse
	// postJSON sends a strings.Reader, so the throttle can resend the body
	// after a 429.
	err = postJSON(ctx, p.Client, p.BaseURL, "application/json", string(payload), func(req *http.Request) {
		web.SetAPIHeaders(req)
		req.Header.Set("Accept", "application/json")
	}, &body)
	if err != nil {
		return nil, err
	}
	if len(body.Errors) > 0 {
		msgs := make([]string, len(body.Errors))
		for i, e := range body.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("AniList: %s", strings.Join(msgs, "; "))
	}

	query := match.Normalize(title)
	var hits []aniListHit
	for _, m := range body.Data.Page.Media {
		urls := uniqueNonEmpty(m.CoverImage.ExtraLarge, m.CoverImage.Large)
		if len(urls) == 0 {
			continue
		}
		name := m.Title.English
		if name == "" {
			name = m.Title.Romaji
		}
		y := m.StartDate.Year
		if y == 0 {
			y = m.SeasonYear
		}
		ys := ""
		if y > 0 {
			ys = strconv.Itoa(y)
		}
		titles := append([]string{m.Title.Romaji, m.Title.English, m.Title.Native}, m.Synonyms...)
		hits = append(hits, aniListHit{
			candidate: media.Candidate{
				Title:     name,
				Creator:   ys,
				Year:      ys,
				ImageURLs: urls,
				Source:    p.Name(),
			},
			best: bestTitle(titles, query),
		})
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no %s results with a cover", media.ErrNotFound, strings.ToLower(p.Type))
	}

	// Only a year breaks ties; any other creator text is ignored.
	rankItem := media.Item{Title: title}
	if y := strings.TrimSpace(item.Creator); isYear(y) {
		rankItem.Creator = y
	}
	ranked := rankGames(hits, rankItem,
		func(h aniListHit) string { return h.best },
		func(h aniListHit) string { return h.candidate.Year })
	out := make([]media.Candidate, len(ranked))
	for i, h := range ranked {
		out[i] = h.candidate
	}
	return out, nil
}

// bestTitle returns the title that matches the normalized query best: an
// exact match if there is one, else the top one by match.Rank (the first on
// ties).
func bestTitle(titles []string, query string) string {
	var nonEmpty []string
	for _, t := range titles {
		if strings.TrimSpace(t) == "" {
			continue
		}
		if match.Normalize(t) == query {
			return t
		}
		nonEmpty = append(nonEmpty, t)
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	ranked := match.Rank(nonEmpty, media.Item{Title: query},
		func(t string) string { return t },
		func(string) string { return "" })
	return ranked[0]
}

// uniqueNonEmpty returns the non-empty values in order, without repeats.
func uniqueNonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
