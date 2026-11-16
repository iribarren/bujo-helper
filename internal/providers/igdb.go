package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// ErrNoIGDBCredentials reports that no Twitch Client ID and Secret are
// configured. It does not wrap media.ErrNotFound: the provider is skipped,
// not searched.
var ErrNoIGDBCredentials = errors.New("no IGDB credentials (set the Twitch Client ID and Secret in Settings)")

// igdbTokenMargin is how long before its expiry a token is replaced, so a
// search never starts with a token about to lapse.
const igdbTokenMargin = time.Minute

// IGDB searches the IGDB games API (owned by Twitch). It needs the user's
// Twitch app credentials: a Client ID and a Client Secret, exchanged for an
// app access token (client credentials grant) that is kept in memory until
// shortly before it expires. Searches are POSTs with an Apicalypse body.
type IGDB struct {
	Client *http.Client
	// BaseURL is the API root; "/games" follows.
	BaseURL string
	// TokenURL is the Twitch OAuth token endpoint.
	TokenURL string
	// ImageBase is the image CDN root; a size and the image ID follow.
	ImageBase string
	// Credentials returns the current Client ID and Client Secret. Nil or
	// a blank value skips the provider.
	Credentials func() (clientID, clientSecret string)

	// now is replaced by tests. Nil means time.Now.
	now func() time.Time

	// mu guards token and serializes token requests, so concurrent
	// searches share one token instead of each fetching their own.
	mu    sync.Mutex
	token igdbToken
}

// igdbToken is an app access token and the credentials it was issued for.
type igdbToken struct {
	clientID, secret string
	access           string
	expires          time.Time
}

// NewIGDB returns an IGDB provider using client (or a default one) and the
// credentials source.
func NewIGDB(client *http.Client, creds func() (clientID, clientSecret string)) *IGDB {
	if client == nil {
		client = web.NewClient()
	}
	return &IGDB{
		Client:      client,
		BaseURL:     "https://api.igdb.com/v4",
		TokenURL:    "https://id.twitch.tv/oauth2/token",
		ImageBase:   "https://images.igdb.com/igdb/image/upload",
		Credentials: creds,
	}
}

// Name implements media.Provider.
func (p *IGDB) Name() string { return "igdb" }

type igdbGame struct {
	Name             string `json:"name"`
	FirstReleaseDate int64  `json:"first_release_date"`
	Cover            struct {
		ImageID string `json:"image_id"`
	} `json:"cover"`
}

// igdbEscaper escapes a title for a quoted Apicalypse string.
var igdbEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// Search implements media.Provider. item.Creator holds the release year,
// if known; it breaks ranking ties.
func (p *IGDB) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	var id, secret string
	if p.Credentials != nil {
		id, secret = p.Credentials()
	}
	id, secret = strings.TrimSpace(id), strings.TrimSpace(secret)
	if id == "" || secret == "" {
		return nil, ErrNoIGDBCredentials
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: empty title", media.ErrNotFound)
	}

	query := `search "` + igdbEscaper.Replace(title) + `"; fields name,first_release_date,cover.image_id; where cover != null; limit 20;`
	var games []igdbGame
	if err := p.games(ctx, id, secret, query, &games); err != nil {
		return nil, redactKey(err, secret)
	}

	var hits []media.Candidate
	for _, g := range games {
		if g.Cover.ImageID == "" {
			continue
		}
		y := ""
		if g.FirstReleaseDate > 0 {
			y = strconv.Itoa(time.Unix(g.FirstReleaseDate, 0).UTC().Year())
		}
		hits = append(hits, media.Candidate{
			Title:     g.Name,
			Creator:   y,
			Year:      y,
			ImageURLs: p.imageURLs(g.Cover.ImageID),
			Source:    p.Name(),
		})
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("%w: no games with a cover", media.ErrNotFound)
	}
	return rankGames(hits, item,
		func(c media.Candidate) string { return c.Title },
		func(c media.Candidate) string { return c.Creator }), nil
}

// imageURLs returns the cover URLs for imageID, largest first. IGDB scales
// the cover to fit each size, so the aspect ratio is kept.
func (p *IGDB) imageURLs(imageID string) []string {
	var urls []string
	for _, size := range []string{"t_1080p", "t_720p", "t_cover_big_2x"} {
		urls = append(urls, p.ImageBase+"/"+size+"/"+imageID+".jpg")
	}
	return urls
}

// games posts query to the games endpoint and decodes the result into v.
// A 401 means the token was revoked or expired early: it is dropped and the
// query is sent once more with a new one.
func (p *IGDB) games(ctx context.Context, id, secret, query string, v any) error {
	for attempt := 0; ; attempt++ {
		token, err := p.accessToken(ctx, id, secret)
		if err != nil {
			return err
		}
		err = postJSON(ctx, p.Client, p.BaseURL+"/games", "text/plain", query, func(req *http.Request) {
			web.SetAPIHeaders(req)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Client-ID", id)
			req.Header.Set("Authorization", "Bearer "+token)
		}, v)
		var status *httpStatusError
		if !errors.As(err, &status) || status.Status != http.StatusUnauthorized {
			return err
		}
		if attempt > 0 {
			return errors.New("IGDB rejected the credentials (HTTP 401)")
		}
		p.dropToken(token)
	}
}

// accessToken returns a valid app access token for the credentials,
// fetching a new one when there is none, it is about to expire, or it was
// issued for other credentials.
func (p *IGDB) accessToken(ctx context.Context, id, secret string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.clock()
	t := p.token
	if t.access != "" && t.clientID == id && t.secret == secret && now.Before(t.expires) {
		return t.access, nil
	}

	form := url.Values{
		"client_id":     {id},
		"client_secret": {secret},
		"grant_type":    {"client_credentials"},
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	// The credentials travel in the form body, never in the URL.
	err := postJSON(ctx, p.Client, p.TokenURL, "application/x-www-form-urlencoded", form.Encode(), web.SetAPIHeaders, &body)
	var status *httpStatusError
	if errors.As(err, &status) && status.Status >= 400 && status.Status < 500 {
		return "", fmt.Errorf("Twitch rejected the IGDB credentials (HTTP %d)", status.Status)
	}
	if err != nil {
		return "", fmt.Errorf("IGDB token: %w", err)
	}
	if body.AccessToken == "" {
		return "", errors.New("IGDB token: the response has no access token")
	}
	lifetime := time.Duration(body.ExpiresIn) * time.Second
	p.token = igdbToken{
		clientID: id,
		secret:   secret,
		access:   body.AccessToken,
		expires:  now.Add(lifetime - min(igdbTokenMargin, lifetime/2)),
	}
	return body.AccessToken, nil
}

// dropToken forgets token, unless another search already replaced it.
func (p *IGDB) dropToken(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token.access == token {
		p.token = igdbToken{}
	}
}

func (p *IGDB) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// postJSON posts body to u with the given content type and header setter
// and decodes the JSON response into v. It reads at most web.MaxBodySize
// bytes. A non-2xx status gives an *httpStatusError.
func postJSON(ctx context.Context, client *http.Client, u, contentType, body string, setHeaders func(*http.Request), v any) error {
	// strings.Reader lets the client (and the throttle) resend the body.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(body))
	if err != nil {
		return err
	}
	if setHeaders != nil {
		setHeaders(req)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, web.MaxBodySize+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if int64(len(b)) > web.MaxBodySize {
		return fmt.Errorf("%w (over %d bytes)", web.ErrBodyTooLarge, web.MaxBodySize)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &httpStatusError{Status: resp.StatusCode}
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
