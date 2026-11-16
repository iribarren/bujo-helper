package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// MusicBrainz searches MusicBrainz release groups (an album across all its
// editions) and takes their front cover from the Cover Art Archive. Neither
// needs a key; MusicBrainz allows one request per second and requires a
// User-Agent with contact information (web.SetMusicBrainzHeaders).
type MusicBrainz struct {
	Client *http.Client
	// BaseURL is the release-group search endpoint.
	BaseURL string
	// CoverBase is the Cover Art Archive release-group root; the group ID
	// and the image name follow.
	CoverBase string
}

// NewMusicBrainz returns a MusicBrainz + Cover Art Archive provider using
// client (or a default one).
func NewMusicBrainz(client *http.Client) *MusicBrainz {
	if client == nil {
		client = web.NewClient()
	}
	return &MusicBrainz{
		Client:    client,
		BaseURL:   "https://musicbrainz.org/ws/2/release-group/",
		CoverBase: "https://coverartarchive.org/release-group",
	}
}

// Name implements media.Provider.
func (p *MusicBrainz) Name() string { return "musicbrainz" }

type musicBrainzResponse struct {
	ReleaseGroups []struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		PrimaryType      string `json:"primary-type"`
		FirstReleaseDate string `json:"first-release-date"`
		ArtistCredit     []struct {
			Name       string `json:"name"`
			JoinPhrase string `json:"joinphrase"`
		} `json:"artist-credit"`
	} `json:"release-groups"`
}

// Search implements media.Provider. item.Creator, if set, is the artist.
func (p *MusicBrainz) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}
	query := `releasegroup:"` + luceneEscape(title) + `"`
	if artist := strings.TrimSpace(item.Creator); artist != "" {
		query += ` AND artist:"` + luceneEscape(artist) + `"`
	}
	q := url.Values{
		"query": {query},
		"fmt":   {"json"},
		"limit": {"25"},
	}
	var body musicBrainzResponse
	if err := getJSONWith(ctx, p.Client, p.BaseURL+"?"+q.Encode(), web.SetMusicBrainzHeaders, &body); err != nil {
		return nil, err
	}

	// Albums go first so they win ranking ties against singles,
	// broadcasts and other release groups with the same title, which are
	// still kept.
	var albums, others []media.Candidate
	for _, rg := range body.ReleaseGroups {
		if rg.ID == "" {
			continue
		}
		var artist strings.Builder
		for _, ac := range rg.ArtistCredit {
			artist.WriteString(ac.Name)
			artist.WriteString(ac.JoinPhrase)
		}
		// The Cover Art Archive answers 404 when a size (or the front
		// image) is missing; the downloader then tries the next URL.
		cover := p.CoverBase + "/" + rg.ID
		c := media.Candidate{
			Title:     rg.Title,
			Creator:   artist.String(),
			Year:      year(rg.FirstReleaseDate),
			ImageURLs: []string{cover + "/front-1200", cover + "/front-500", cover + "/front"},
			Source:    p.Name(),
		}
		if rg.PrimaryType == "Album" {
			albums = append(albums, c)
		} else {
			others = append(others, c)
		}
	}
	hits := append(albums, others...)
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no release groups", media.ErrNotFound)
	}
	return rankCandidates(hits, item), nil
}

// luceneSpecial holds the characters the Lucene query syntax reserves.
const luceneSpecial = `+-&|!(){}[]^"~*?:\/`

// luceneEscape backslash-escapes Lucene's reserved characters so s is
// matched literally inside a quoted phrase.
func luceneEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(luceneSpecial, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
