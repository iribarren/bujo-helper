// Package web holds the HTTP conventions shared by providers and the
// downloader: request headers, timeouts and bounded body reads.
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// APIUserAgent identifies the app to public APIs (Apple, Open Library).
	APIUserAgent = "bujo-helper/1.0"
	// ContactUserAgent identifies the app with contact information, as
	// MusicBrainz and the Wikimedia User-Agent policy ask.
	ContactUserAgent = "bujo-helper/1.0 ( https://github.com/iribarren/bujo-helper )"
	// MusicBrainzUserAgent is the User-Agent sent to MusicBrainz.
	MusicBrainzUserAgent = ContactUserAgent
	// BrowserUserAgent is a desktop Chrome UA, needed for store pages and
	// image CDNs that reject unknown clients.
	BrowserUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/126.0 Safari/537.36"

	// Timeout is the default per-attempt timeout (see Throttle).
	Timeout = 15 * time.Second
)

// MaxBodySize caps how much of a response body is read into memory.
// Larger bodies fail with ErrBodyTooLarge rather than being truncated, so a
// partial image is never saved. It is a var so tests can lower it.
var MaxBodySize int64 = 32 << 20

// ErrBodyTooLarge is returned by Get when a body exceeds MaxBodySize.
var ErrBodyTooLarge = errors.New("response body too large")

// NewClient returns an HTTP client that goes through DefaultThrottle. It
// has no overall Timeout: the throttle limits each attempt to Timeout, so
// rate-limit waits are not counted against it.
func NewClient() *http.Client {
	return &http.Client{Transport: DefaultThrottle}
}

// SetAPIHeaders marks a request as coming from this app.
func SetAPIHeaders(req *http.Request) {
	req.Header.Set("User-Agent", APIUserAgent)
}

// SetMusicBrainzHeaders marks a request to the MusicBrainz API as coming
// from this app, with the contact MusicBrainz requires.
func SetMusicBrainzHeaders(req *http.Request) {
	req.Header.Set("User-Agent", MusicBrainzUserAgent)
	req.Header.Set("Accept", "application/json")
}

// SetWikimediaHeaders marks a request to a Wikimedia API (Wikidata,
// Commons) as coming from this app, with the contact the Wikimedia
// User-Agent policy requires.
func SetWikimediaHeaders(req *http.Request) {
	req.Header.Set("User-Agent", ContactUserAgent)
}

// SetBHLHeaders marks a request to the Biodiversity Heritage Library API
// as coming from this app, with contact information, and asks for JSON.
func SetBHLHeaders(req *http.Request) {
	req.Header.Set("User-Agent", ContactUserAgent)
	req.Header.Set("Accept", "application/json")
}

// SetOFFHeaders marks a request to an Open Food Facts API as coming from
// this app, with the contact Open Food Facts requires, and asks for JSON.
func SetOFFHeaders(req *http.Request) {
	req.Header.Set("User-Agent", ContactUserAgent)
	req.Header.Set("Accept", "application/json")
}

// contactMediaHosts serve images whose owners require the contact
// User-Agent: Wikimedia Commons files and thumbnails (their robot policy
// rejects generic and fake browser User-Agents) and Open Food Facts
// product images (every request must name the app).
var contactMediaHosts = map[string]bool{
	"upload.wikimedia.org":     true,
	"thumb.wikimedia.org":      true,
	"images.openfoodfacts.org": true,
}

// SetBrowserHeaders makes a request look like a desktop browser. Requests
// to the contactMediaHosts send the contact User-Agent instead, as their
// policies require.
func SetBrowserHeaders(req *http.Request) {
	ua := BrowserUserAgent
	if contactMediaHosts[req.URL.Hostname()] {
		ua = ContactUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

// Response is a fully read HTTP response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// OK reports whether the status code is 2xx.
func (r *Response) OK() bool { return r.Status >= 200 && r.Status < 300 }

// Get performs a GET with the given header setter and reads at most
// MaxBodySize bytes of the body. Non-2xx statuses are not errors.
func Get(ctx context.Context, client *http.Client, url string, setHeaders func(*http.Request)) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if setHeaders != nil {
		setHeaders(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(body)) > MaxBodySize {
		return nil, fmt.Errorf("%w (over %d bytes)", ErrBodyTooLarge, MaxBodySize)
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: body}, nil
}
