package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"bujo-helper/internal/match"
	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// Artwork providers only return open-access works: public domain at the
// Met, CC0 at Cleveland. Both licences are CC0 1.0, which needs no
// attribution, but every image still gets a credit text for the user.

// cc0License is the licence line of every artwork credit.
const cc0License = "CC0 1.0 Universal (public domain), https://creativecommons.org/publicdomain/zero/1.0/"

// Museum names used in credits.
const (
	museumMet       = "The Metropolitan Museum of Art"
	museumCleveland = "The Cleveland Museum of Art"
)

// artworkCredit builds the credit text of an open-access artwork. Empty
// fields are left out; the title, museum and licence lines are always
// there.
func artworkCredit(title, date, artist, museum, pageURL string) string {
	head := "Title: " + strings.TrimSpace(title)
	if d := strings.TrimSpace(date); d != "" {
		head += " (" + d + ")"
	}
	lines := []string{head}
	if a := strings.TrimSpace(artist); a != "" {
		lines = append(lines, "Artist: "+a)
	}
	lines = append(lines, "Museum: "+museum, "License: "+cc0License)
	if u := strings.TrimSpace(pageURL); u != "" {
		lines = append(lines, "Object page: "+u)
	}
	return strings.Join(lines, "\n")
}

// errNotOpenAccess reports that n works matched but none has an open-access
// image.
func errNotOpenAccess(n int) error {
	matches := "1 match"
	if n != 1 {
		matches = strconv.Itoa(n) + " matches"
	}
	return fmt.Errorf("%w: %s, none with a public-domain (CC0) image", media.ErrNotFound, matches)
}

// artworkYearPattern finds the first standalone four-digit year in a date
// such as "ca. 1665" or "1889–90".
var artworkYearPattern = regexp.MustCompile(`\b\d{4}\b`)

// rankArtworks keeps the plausible matches (see match.Plausible) of hits,
// ranked best-first, and then only the open-access ones. A museum search
// matches any word of the title, so an implausible hit is usually another
// work; dropping it lets the next provider try. errNotOpenAccess is
// returned when plausible matches exist but none is open access.
func rankArtworks(hits []artworkHit, item media.Item) ([]media.Candidate, error) {
	var plausible []artworkHit
	for _, h := range hits {
		if match.Plausible(item, h.Title, h.Creator) {
			plausible = append(plausible, h)
		}
	}
	if len(plausible) == 0 {
		return nil, fmt.Errorf("%w: no matching artwork among %d results", media.ErrNotFound, len(hits))
	}
	var open []media.Candidate
	for _, h := range plausible {
		if h.open && len(h.ImageURLs) > 0 {
			open = append(open, h.Candidate)
		}
	}
	if len(open) == 0 {
		return nil, errNotOpenAccess(len(plausible))
	}
	return rankCandidates(open, item), nil
}

// artworkHit is a search hit and whether it is open access.
type artworkHit struct {
	media.Candidate
	open bool
}

// --- The Met ---

// metMaxObjects caps the object lookups of one search: the Met search
// returns object IDs only, and every object is one more request.
const metMaxObjects = 10

// Met searches the Metropolitan Museum of Art collection API (keyless).
// Only public-domain works with a primary image are returned; the Met
// publishes those under CC0.
type Met struct {
	Client *http.Client
	// BaseURL is the API root; "/v1.1/search" and "/v1/objects/{id}"
	// follow.
	BaseURL string
}

// NewMet returns a Met provider using client (or a default one).
func NewMet(client *http.Client) *Met {
	if client == nil {
		client = web.NewClient()
	}
	return &Met{Client: client, BaseURL: "https://collectionapi.metmuseum.org/public/collection"}
}

// Name implements media.Provider.
func (p *Met) Name() string { return "met" }

type metObject struct {
	ObjectID          int    `json:"objectID"`
	Title             string `json:"title"`
	ArtistDisplayName string `json:"artistDisplayName"`
	ObjectDate        string `json:"objectDate"`
	IsPublicDomain    bool   `json:"isPublicDomain"`
	PrimaryImage      string `json:"primaryImage"`
	PrimaryImageSmall string `json:"primaryImageSmall"`
	ObjectURL         string `json:"objectURL"`
}

// Search implements media.Provider. item.Creator is the artist.
func (p *Met) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	ids, err := p.search(ctx, title, true)
	if err != nil {
		return nil, err
	}
	// The title search matches any word of the title, in collection order.
	// When it finds nothing, or more than can be looked up and an artist is
	// known, the free-text search with the artist (ranked by relevance)
	// is tried instead.
	if len(ids) == 0 || (strings.TrimSpace(item.Creator) != "" && len(ids) > metMaxObjects) {
		more, err := p.search(ctx, searchTerm(item), false)
		if err != nil && len(ids) == 0 {
			return nil, err
		}
		// On a failed fallback, the title matches already found are used.
		if err == nil && len(more) > 0 {
			ids = more
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no artworks with images", media.ErrNotFound)
	}
	if len(ids) > metMaxObjects {
		ids = ids[:metMaxObjects]
	}

	var hits []artworkHit
	// failure is the first lookup error other than a 404: a listed object
	// can be gone, but an outage must not read as "not found".
	var failure error
	for _, id := range ids {
		var o metObject
		err := getJSON(ctx, p.Client, p.BaseURL+"/v1/objects/"+strconv.Itoa(id), &o)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			var status *httpStatusError
			if failure == nil && !(errors.As(err, &status) && status.Status == http.StatusNotFound) {
				failure = err
			}
			continue
		}
		hits = append(hits, p.hit(o))
	}
	if len(hits) == 0 {
		if failure != nil {
			return nil, failure
		}
		return nil, fmt.Errorf("%w: no artworks with images", media.ErrNotFound)
	}
	return rankArtworks(hits, item)
}

// search returns the object IDs the Met search finds for q, only for works
// with images. titleOnly restricts it to titles.
func (p *Met) search(ctx context.Context, q string, titleOnly bool) ([]int, error) {
	v := url.Values{"q": {q}, "hasImages": {"true"}}
	if titleOnly {
		v.Set("title", "true")
	}
	var body struct {
		// ObjectIDs is null when nothing matches.
		ObjectIDs []int `json:"objectIDs"`
	}
	if err := getJSON(ctx, p.Client, p.BaseURL+"/v1.1/search?"+v.Encode(), &body); err != nil {
		return nil, err
	}
	return body.ObjectIDs, nil
}

// hit turns a Met object into a hit. Only the primary image is used, never
// the additional (detail) images.
func (p *Met) hit(o metObject) artworkHit {
	c := media.Candidate{
		Title:   o.Title,
		Creator: o.ArtistDisplayName,
		Year:    artworkYearPattern.FindString(o.ObjectDate),
		Source:  p.Name(),
		Credit:  artworkCredit(o.Title, o.ObjectDate, o.ArtistDisplayName, museumMet, o.ObjectURL),
	}
	for _, u := range []string{o.PrimaryImage, o.PrimaryImageSmall} {
		if u != "" {
			c.ImageURLs = append(c.ImageURLs, u)
		}
	}
	// primaryImage is only set for open-access works.
	return artworkHit{Candidate: c, open: o.IsPublicDomain && o.PrimaryImage != ""}
}

// --- The Cleveland Museum of Art ---

// clevelandFields are the artwork fields Cleveland returns.
const clevelandFields = "id,title,creators,creation_date,share_license_status,images,url"

// Cleveland searches the Cleveland Museum of Art Open Access API
// (keyless). Only CC0 works are returned.
type Cleveland struct {
	Client *http.Client
	// BaseURL is the artworks endpoint.
	BaseURL string
}

// NewCleveland returns a Cleveland provider using client (or a default
// one).
func NewCleveland(client *http.Client) *Cleveland {
	if client == nil {
		client = web.NewClient()
	}
	return &Cleveland{Client: client, BaseURL: "https://openaccess-api.clevelandart.org/api/artworks/"}
}

// Name implements media.Provider.
func (p *Cleveland) Name() string { return "cleveland" }

type clevelandImage struct {
	URL string `json:"url"`
}

type clevelandArtwork struct {
	Title              string `json:"title"`
	ShareLicenseStatus string `json:"share_license_status"`
	CreationDate       string `json:"creation_date"`
	URL                string `json:"url"`
	Creators           []struct {
		Description string `json:"description"`
	} `json:"creators"`
	// Images and each size can be null. The "full" TIFF is never used.
	Images *struct {
		Web   *clevelandImage `json:"web"`
		Print *clevelandImage `json:"print"`
	} `json:"images"`
}

// Search implements media.Provider. item.Creator is the artist.
func (p *Cleveland) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	v := url.Values{"title": {title}, "has_image": {"1"}, "limit": {"20"}, "fields": {clevelandFields}}
	artist := strings.TrimSpace(item.Creator)
	if artist != "" {
		v.Set("artists", artist)
	}
	works, err := p.search(ctx, v)
	if err != nil {
		return nil, err
	}
	// The artist filter needs the museum's spelling; retry without it.
	if len(works) == 0 && artist != "" {
		v.Del("artists")
		if works, err = p.search(ctx, v); err != nil {
			return nil, err
		}
	}
	if len(works) == 0 {
		return nil, fmt.Errorf("%w: no CC0 artworks with images", media.ErrNotFound)
	}

	hits := make([]artworkHit, 0, len(works))
	for _, w := range works {
		hits = append(hits, p.hit(w))
	}
	return rankArtworks(hits, item)
}

func (p *Cleveland) search(ctx context.Context, v url.Values) ([]clevelandArtwork, error) {
	var body struct {
		Data []clevelandArtwork `json:"data"`
	}
	if err := getJSON(ctx, p.Client, p.BaseURL+"?"+clevelandQuery(v), &body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

// clevelandQuery encodes v and adds the bare cc0 flag (it takes no value,
// which url.Values cannot express).
func clevelandQuery(v url.Values) string {
	return v.Encode() + "&cc0"
}

func (p *Cleveland) hit(w clevelandArtwork) artworkHit {
	var described string
	if len(w.Creators) > 0 {
		described = w.Creators[0].Description
	}
	c := media.Candidate{
		Title:   w.Title,
		Creator: stripLifeDates(described),
		Year:    artworkYearPattern.FindString(w.CreationDate),
		Source:  p.Name(),
		Credit:  artworkCredit(w.Title, w.CreationDate, described, museumCleveland, w.URL),
	}
	if w.Images != nil {
		for _, img := range []*clevelandImage{w.Images.Print, w.Images.Web} {
			if img != nil && img.URL != "" {
				c.ImageURLs = append(c.ImageURLs, img.URL)
			}
		}
	}
	return artworkHit{Candidate: c, open: w.ShareLicenseStatus == "CC0"}
}

// trailingParens matches a trailing parenthesized part, such as the
// "(American, 1826–1900)" of a Cleveland creator.
var trailingParens = regexp.MustCompile(`\s*\([^()]*\)\s*$`)

// stripLifeDates drops the trailing nationality and life dates from a
// Cleveland creator description.
func stripLifeDates(s string) string {
	return strings.TrimSpace(trailingParens.ReplaceAllString(s, ""))
}
