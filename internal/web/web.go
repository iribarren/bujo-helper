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
	APIUserAgent = "cover-downloader/1.0"
	// MusicBrainzUserAgent identifies the app to MusicBrainz, which asks
	// for a meaningful User-Agent with contact information.
	MusicBrainzUserAgent = "cover-downloader/1.0 ( https://github.com/iribarren/bullet-journal-covers )"
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

// SetBrowserHeaders makes a request look like a desktop browser.
func SetBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", BrowserUserAgent)
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
