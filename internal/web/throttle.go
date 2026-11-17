package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Throttle defaults, used when the matching field is zero.
const (
	DefaultMaxRetries  = 3
	DefaultBaseBackoff = time.Second
	DefaultMaxWait     = 60 * time.Second
)

// waitNoticeMin is the shortest wait reported to OnWait, so millisecond
// spacing does not flood the UI.
const waitNoticeMin = time.Second

// drainLimit caps how much of a rejected body is read before closing it,
// so the connection can be reused without reading a huge error page.
const drainLimit = 64 << 10

// Throttle is an http.RoundTripper that waits when a server rate-limits.
//
// On 429 or 503 it waits for Retry-After (seconds or an HTTP-date), or
// backs off exponentially when the header is missing, invalid, zero or in
// the past, and retries. The wait
// blocks the host for every request through the same Throttle, so one
// worker's 429 pauses all workers. Hosts in MinInterval are also spaced
// out. Bodiless GET and HEAD requests are retried, and so are requests
// whose body can be sent again (GetBody is set, as http.NewRequest does
// for in-memory bodies): a rate-limited request was refused, not
// processed. IGDB's searches are such POSTs. A request with a one-shot body
// gets the rate-limit response back at once.
//
// Each attempt gets its own AttemptTimeout, which covers reading the body
// too; use a Client with no Timeout, since that would also count the waits.
type Throttle struct {
	// Base performs the requests. Nil means http.DefaultTransport.
	Base http.RoundTripper
	// MinInterval is the minimum spacing between requests to a host. Keys
	// are matched against the URL host, with and then without the port.
	MinInterval map[string]time.Duration
	// MaxRetries is how many times a rate-limited request is retried.
	// Zero means DefaultMaxRetries; negative means never.
	MaxRetries int
	// BaseBackoff is the first wait when there is no Retry-After; it
	// doubles on each retry, up to MaxWait. Zero means DefaultBaseBackoff.
	BaseBackoff time.Duration
	// MaxWait is the longest wait accepted. A longer Retry-After returns
	// the response at once. Zero means DefaultMaxWait.
	MaxWait time.Duration
	// AttemptTimeout limits each attempt, body included. Zero means
	// Timeout.
	AttemptTimeout time.Duration
	// OnWait, if set, is called before waits of a second or more. Set it
	// with SetOnWait once the Throttle is in use.
	OnWait func(host string, d time.Duration)

	// now and sleep are replaced by tests. Nil means the real clock and a
	// context-aware timer.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error

	mu   sync.Mutex
	next map[string]time.Time // per host: earliest start of the next request
}

// DefaultThrottle is shared by every client from NewClient, so all of them
// see the same per-host blocks and spacing.
var DefaultThrottle = &Throttle{
	MinInterval: map[string]time.Duration{
		"api.themoviedb.org": 25 * time.Millisecond,
		"www.wikidata.org":   200 * time.Millisecond,
		// MusicBrainz allows one request per second per client.
		"musicbrainz.org": time.Second,
		// IGDB allows four requests per second (and eight open ones).
		"api.igdb.com": 250 * time.Millisecond,
		// The Met asks for at most 80 requests per second; a search makes
		// up to 12, so this keeps well under it.
		"collectionapi.metmuseum.org": 25 * time.Millisecond,
		// Wikimedia asks for serial API requests well under five per
		// second, and at most two concurrent media downloads; spacing the
		// image hosts by half a second keeps downloads at about that.
		"commons.wikimedia.org": 200 * time.Millisecond,
		"upload.wikimedia.org":  500 * time.Millisecond,
		"thumb.wikimedia.org":   500 * time.Millisecond,
		// The Biodiversity Heritage Library publishes no rate numbers but
		// has IP limits: its API and page scans (redirected to S3) are
		// spaced half a second apart.
		"www.biodiversitylibrary.org":              500 * time.Millisecond,
		"bhl-open-data.s3.us-east-2.amazonaws.com": 500 * time.Millisecond,
	},
}

// SetOnWait sets the OnWait hook. It is safe to call while requests run.
func (t *Throttle) SetOnWait(f func(host string, d time.Duration)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.OnWait = f
}

// RoundTrip implements http.RoundTripper.
func (t *Throttle) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	host := req.URL.Host
	retries := t.maxRetries()
	if !retryable(req) {
		retries = 0
	}
	for attempt := 0; ; attempt++ {
		if err := t.wait(ctx, host, t.reserve(req.URL)); err != nil {
			return nil, err
		}
		send, err := rewind(req, attempt)
		if err != nil {
			return nil, err
		}
		resp, err := t.attempt(send)
		if err != nil {
			return nil, err
		}
		if !rateLimited(resp) {
			return resp, nil
		}
		now := t.clock()
		// A missing, invalid, zero or past Retry-After falls back to the
		// backoff step, so retries are never sent back-to-back.
		delay, ok := parseRetryAfter(resp.Header.Get("Retry-After"), now)
		if !ok || delay <= 0 {
			delay = t.backoff(attempt)
		}
		if attempt >= retries || delay > t.maxWait() {
			return resp, nil
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimit))
		resp.Body.Close()
		t.block(host, now.Add(delay))
	}
}

// rateLimited reports whether resp asks the client to slow down: any 429,
// or a 503 that carries Retry-After. A bare 503 is treated as a plain error
// (Amazon serves its bot-check pages that way), so it is not retried and
// does not block the host.
func rateLimited(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusServiceUnavailable:
		return resp.Header.Get("Retry-After") != ""
	}
	return false
}

// retryable reports whether req can be sent again: a GET or HEAD with no
// body, or any request whose body GetBody can recreate.
func retryable(req *http.Request) bool {
	if !hasBody(req) {
		return req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == ""
	}
	return req.GetBody != nil
}

func hasBody(req *http.Request) bool {
	return req.Body != nil && req.Body != http.NoBody
}

// rewind returns the request for the given attempt: req itself the first
// time, then a copy with a fresh body from GetBody, since the transport
// has consumed the previous one.
func rewind(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 || !hasBody(req) {
		return req, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	r := req.Clone(req.Context())
	r.Body = body
	return r, nil
}

// reserve takes the next slot for the URL's host and returns how long to
// wait for it. The slot is taken under the lock; the wait happens outside.
func (t *Throttle) reserve(u *url.URL) time.Duration {
	interval, ok := t.MinInterval[u.Host]
	if !ok {
		interval = t.MinInterval[u.Hostname()]
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.clock()
	start := now
	if next := t.next[u.Host]; next.After(now) {
		start = next
	}
	if interval > 0 {
		if t.next == nil {
			t.next = make(map[string]time.Time)
		}
		t.next[u.Host] = start.Add(interval)
	}
	return start.Sub(now)
}

// attempt sends req once under AttemptTimeout. The timeout ends when the
// returned body is closed, or at once on error.
func (t *Throttle) attempt(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), t.attemptTimeout())
	resp, err := t.base().RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelOnClose ends an attempt's context when its body is closed.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// wait sleeps for d, telling OnWait first when d is long enough to notice.
func (t *Throttle) wait(ctx context.Context, host string, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	if d >= waitNoticeMin {
		t.mu.Lock()
		onWait := t.OnWait
		t.mu.Unlock()
		if onWait != nil {
			onWait(host, d)
		}
	}
	if t.sleep != nil {
		return t.sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// block keeps requests to host from starting before until.
func (t *Throttle) block(host string, until time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.next == nil {
		t.next = make(map[string]time.Time)
	}
	if until.After(t.next[host]) {
		t.next[host] = until
	}
}

// parseRetryAfter reads a Retry-After value: delta seconds or an HTTP-date.
// Zero seconds or a date in the past gives zero; a negative or unparseable
// value is not ok.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs < 0 {
			return 0, false
		}
		if secs > int64(365*24*time.Hour/time.Second) {
			secs = int64(365 * 24 * time.Hour / time.Second) // avoid overflow
		}
		return time.Duration(secs) * time.Second, true
	}
	when, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	if d := when.Sub(now); d > 0 {
		return d, true
	}
	return 0, true
}

// backoff is the wait before retry number attempt+1 when the server gave
// no Retry-After: BaseBackoff doubled per attempt, capped at MaxWait.
func (t *Throttle) backoff(attempt int) time.Duration {
	d := t.baseBackoff()
	limit := t.maxWait()
	for i := 0; i < attempt && d < limit; i++ {
		d *= 2
	}
	return min(d, limit)
}

func (t *Throttle) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

func (t *Throttle) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *Throttle) maxRetries() int {
	switch {
	case t.MaxRetries < 0:
		return 0
	case t.MaxRetries == 0:
		return DefaultMaxRetries
	}
	return t.MaxRetries
}

func (t *Throttle) baseBackoff() time.Duration {
	if t.BaseBackoff > 0 {
		return t.BaseBackoff
	}
	return DefaultBaseBackoff
}

func (t *Throttle) maxWait() time.Duration {
	if t.MaxWait > 0 {
		return t.MaxWait
	}
	return DefaultMaxWait
}

func (t *Throttle) attemptTimeout() time.Duration {
	if t.AttemptTimeout > 0 {
		return t.AttemptTimeout
	}
	return Timeout
}
