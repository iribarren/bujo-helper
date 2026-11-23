package providers

import (
	"cmp"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// ErrNoBGGToken reports that no BoardGameGeek token is available: none is
// built into this build and none is saved in Settings (or passed to the
// CLI). The provider is skipped, not searched.
var ErrNoBGGToken = media.Skipped("no BoardGameGeek token in this build (add one in Settings)")

// BoardGameGeek finds the box art of a board game through the
// BoardGameGeek XML API2. Every request needs an application token, sent
// as a bearer token. A search lists matching board games (expansions are
// ignored); the best ones are then read in one "thing" request for their
// names, year, image and number of ratings.
//
// Box art is copyrighted by its publisher, so each candidate carries a
// credit (game, BoardGameGeek, game page) for the sidecar. Requests are
// spaced by web.DefaultThrottle and send the contact User-Agent.
type BoardGameGeek struct {
	Client *http.Client
	// BaseURL is the XML API2 root; "/search" and "/thing" follow. Tests
	// point it at a fake server.
	BaseURL string
	// Token returns the application token. Nil or blank skips the
	// provider.
	Token func() string
}

// NewBoardGameGeek returns a BoardGameGeek provider using client (or a
// default one) and the token source.
func NewBoardGameGeek(client *http.Client, token func() string) *BoardGameGeek {
	if client == nil {
		client = web.NewClient()
	}
	return &BoardGameGeek{
		Client:  client,
		BaseURL: "https://boardgamegeek.com/xmlapi2",
		Token:   token,
	}
}

// Name implements media.Provider.
func (p *BoardGameGeek) Name() string { return "boardgamegeek" }

// Label implements media.Labeler.
func (*BoardGameGeek) Label() string { return "BoardGameGeek" }

const (
	// bggMaxThings caps the search hits read with the thing request.
	bggMaxThings = 20
	// bggMaxCandidates caps the candidates returned.
	bggMaxCandidates = 3
	// bggGamePage is the public game page; the credit links it whatever
	// BaseURL is.
	bggGamePage = "https://boardgamegeek.com/boardgame/"
	// bggBoardGame is the item type of a board game (not an expansion or
	// an accessory).
	bggBoardGame = "boardgame"
)

// bggValue is an element whose value is in its value attribute.
type bggValue struct {
	Value string `xml:"value,attr"`
}

// bggName is a primary or alternate name.
type bggName struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

// bggItem is an item of a search or thing response. Image and Ratings are
// only in thing responses.
type bggItem struct {
	Type    string    `xml:"type,attr"`
	ID      string    `xml:"id,attr"`
	Names   []bggName `xml:"name"`
	Year    bggValue  `xml:"yearpublished"`
	Image   string    `xml:"image"`
	Ratings bggValue  `xml:"statistics>ratings>usersrated"`
}

type bggItems struct {
	Items []bggItem `xml:"item"`
}

// primary is the item's primary name, else its first name.
func (it bggItem) primary() string {
	for _, n := range it.Names {
		if n.Type == "primary" {
			return strings.TrimSpace(n.Value)
		}
	}
	if len(it.Names) > 0 {
		return strings.TrimSpace(it.Names[0].Value)
	}
	return ""
}

// year is the publication year, or 0 when unknown.
func (it bggItem) year() int {
	y, err := strconv.Atoi(strings.TrimSpace(it.Year.Value))
	if err != nil || y <= 0 {
		return 0
	}
	return y
}

// bggScore ranks an item against the request; lower is better. The title
// counts first, then the year.
type bggScore struct{ title, year int }

func (s bggScore) compare(o bggScore) int {
	return cmp.Or(cmp.Compare(s.title, o.title), cmp.Compare(s.year, o.year))
}

// score rates it against the normalized query and the requested year (0
// when none): the best title score of its primary and alternate names,
// then the year score.
func (it bggItem) score(query string, year int) bggScore {
	title := 4
	for _, n := range it.Names {
		title = min(title, bggTitleScore(query, n.Value))
	}
	return bggScore{title: title, year: bggYearScore(year, it.year())}
}

// bggTitleScore is 0 for the same title, 1 when one starts with the
// other, 2 when the title contains the query, 3 otherwise and 4 for a
// blank title.
func bggTitleScore(query, title string) int {
	t := match.Normalize(title)
	switch {
	case t == "":
		return 4
	case t == query:
		return 0
	case strings.HasPrefix(t, query) || strings.HasPrefix(query, t):
		return 1
	case strings.Contains(t, query):
		return 2
	}
	return 3
}

// bggYearScore is 0 for the requested year (or when none is requested), 1
// for a year off by one, 2 for an unknown year and 3 for any other.
func bggYearScore(want, got int) int {
	switch {
	case want == 0 || got == want:
		return 0
	case got == 0:
		return 2
	case got == want-1 || got == want+1:
		return 1
	}
	return 3
}

// bggRequestedYear is the year in item.Creator, or 0.
func bggRequestedYear(item media.Item) int {
	y, err := strconv.Atoi(strings.TrimSpace(item.Creator))
	if err != nil || y <= 0 {
		return 0
	}
	return y
}

// Search implements media.Provider. item.Creator holds the publication
// year, if known.
func (p *BoardGameGeek) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	var token string
	if p.Token != nil {
		token = strings.TrimSpace(p.Token())
	}
	if token == "" {
		return nil, ErrNoBGGToken
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	query := match.Normalize(title)
	year := bggRequestedYear(item)

	hits, err := p.search(ctx, token, title, true)
	if err == nil && len(hits) == 0 {
		hits, err = p.search(ctx, token, title, false)
	}
	if err != nil {
		return nil, err
	}
	what := strconv.Quote(title)
	if year > 0 {
		what += fmt.Sprintf(" (%d)", year)
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no BoardGameGeek board game matches %s", media.ErrNotFound, what)
	}

	slices.SortStableFunc(hits, func(a, b bggItem) int {
		return a.score(query, year).compare(b.score(query, year))
	})
	hits = hits[:min(len(hits), bggMaxThings)]
	things, err := p.things(ctx, token, hits)
	if err != nil {
		return nil, err
	}
	return p.candidates(things, query, year, what)
}

// search returns the board games (not expansions) found for title, with
// exact=1 when exact is set.
func (p *BoardGameGeek) search(ctx context.Context, token, title string, exact bool) ([]bggItem, error) {
	v := url.Values{"query": {title}, "type": {bggBoardGame}}
	if exact {
		v.Set("exact", "1")
	}
	var resp bggItems
	if err := p.get(ctx, token, p.BaseURL+"/search?"+v.Encode(), &resp); err != nil {
		return nil, err
	}
	var out []bggItem
	for _, it := range resp.Items {
		if it.Type == bggBoardGame && bggValidID(it.ID) {
			out = append(out, it)
		}
	}
	return out, nil
}

// things reads the full records (image, names, ratings) of hits in one
// request. Ids go comma-separated, unescaped, as the API documents.
func (p *BoardGameGeek) things(ctx context.Context, token string, hits []bggItem) ([]bggItem, error) {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	var resp bggItems
	if err := p.get(ctx, token, p.BaseURL+"/thing?id="+strings.Join(ids, ",")+"&stats=1", &resp); err != nil {
		return nil, err
	}
	var out []bggItem
	for _, it := range resp.Items {
		if it.Type == bggBoardGame && bggValidID(it.ID) {
			out = append(out, it)
		}
	}
	return out, nil
}

// candidates ranks things by title, year and then number of ratings, and
// returns the best ones that have an image.
func (p *BoardGameGeek) candidates(things []bggItem, query string, year int, what string) ([]media.Candidate, error) {
	slices.SortStableFunc(things, func(a, b bggItem) int {
		if c := a.score(query, year).compare(b.score(query, year)); c != 0 {
			return c
		}
		return cmp.Compare(bggRatings(b), bggRatings(a))
	})
	var out []media.Candidate
	for _, it := range things {
		img := strings.TrimSpace(it.Image)
		if img == "" {
			continue
		}
		name := capField(strings.Join(strings.Fields(it.primary()), " "))
		y := ""
		if it.year() > 0 {
			y = strconv.Itoa(it.year())
		}
		out = append(out, media.Candidate{
			Title:     name,
			Creator:   y,
			Year:      y,
			ImageURLs: []string{img},
			Source:    p.Name(),
			Credit:    bggCredit(name, y, it.ID),
		})
		if len(out) == bggMaxCandidates {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no BoardGameGeek board game with box art matches %s", media.ErrNotFound, what)
	}
	return out, nil
}

// bggRatings is the number of users who rated it, or 0.
func bggRatings(it bggItem) int {
	n, _ := strconv.Atoi(strings.TrimSpace(it.Ratings.Value))
	return n
}

// bggValidID reports whether id is a numeric BoardGameGeek id.
func bggValidID(id string) bool {
	return id != "" && offDigits.MatchString(id)
}

// bggCredit builds the credit text of a game's box art: the game and its
// year, the source, the game page and the copyright note.
func bggCredit(name, year, id string) string {
	head := "Title: " + name
	if year != "" {
		head += " (" + year + ")"
	}
	return strings.Join([]string{
		head,
		"Image: BoardGameGeek",
		"Game page: " + bggGamePage + id,
		"Box art © its publisher; personal use only.",
	}, "\n")
}

// get fetches u with the BoardGameGeek headers and decodes the XML body
// into v. A rejected token and a busy server get errors that say so.
func (p *BoardGameGeek) get(ctx context.Context, token, u string, v any) error {
	resp, err := web.Get(ctx, p.Client, u, func(r *http.Request) { web.SetBGGHeaders(r, token) })
	if err != nil {
		return redactKey(err, token)
	}
	if !resp.OK() {
		return bggStatusError(&httpStatusError{Status: resp.Status})
	}
	if err := xml.Unmarshal(resp.Body, v); err != nil {
		return fmt.Errorf("decode BoardGameGeek response: %w", err)
	}
	return nil
}

// bggStatusError explains a non-2xx BoardGameGeek response.
func bggStatusError(se *httpStatusError) error {
	switch se.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("BoardGameGeek rejected the token (%w); check the BoardGameGeek token in Settings", se)
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable:
		return fmt.Errorf("BoardGameGeek is busy (%w); try again later", se)
	}
	return fmt.Errorf("BoardGameGeek: %w", se)
}
