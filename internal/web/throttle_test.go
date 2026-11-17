package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is an injectable clock: sleeping records the duration and
// advances the time instead of waiting. When release is non-nil, a sleep
// first reports itself on sleeping and then blocks until release is closed.
type fakeClock struct {
	mu     sync.Mutex
	cur    time.Time
	sleeps []time.Duration

	sleeping chan time.Duration
	release  chan struct{}
}

func newFakeClock() *fakeClock {
	return &fakeClock{cur: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
}

func (c *fakeClock) sleep(ctx context.Context, d time.Duration) error {
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	c.mu.Unlock()
	if c.release != nil {
		c.sleeping <- d
		select {
		case <-c.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.mu.Lock()
	c.cur = c.cur.Add(d)
	c.mu.Unlock()
	return nil
}

func (c *fakeClock) recorded() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.sleeps...)
}

type waitRecord struct {
	host string
	d    time.Duration
}

type waitRecorder struct {
	mu    sync.Mutex
	waits []waitRecord
}

func (r *waitRecorder) onWait(host string, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waits = append(r.waits, waitRecord{host, d})
}

func (r *waitRecorder) recorded() []waitRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]waitRecord(nil), r.waits...)
}

func newTestThrottle(c *fakeClock, w *waitRecorder) *Throttle {
	return &Throttle{now: c.now, sleep: c.sleep, OnWait: w.onWait}
}

// statusServer answers each request with the next reply in replies; the
// last reply repeats. It counts the requests it served.
type reply struct {
	status     int
	retryAfter string
	body       string
}

func statusServer(t *testing.T, replies ...reply) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1)) - 1
		if n >= len(replies) {
			n = len(replies) - 1
		}
		rp := replies[n]
		if rp.retryAfter != "" {
			w.Header().Set("Retry-After", rp.retryAfter)
		}
		w.WriteHeader(rp.status)
		io.WriteString(w, rp.body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestThrottleRetries(t *testing.T) {
	clockNow := newFakeClock().cur
	tests := []struct {
		name       string
		replies    []reply
		wantStatus int
		wantBody   string
		wantHits   int32
		wantSleeps []time.Duration
	}{
		{
			name:       "429 with Retry-After seconds then 200",
			replies:    []reply{{429, "2", "slow"}, {200, "", "ok"}},
			wantStatus: 200, wantBody: "ok", wantHits: 2,
			wantSleeps: []time.Duration{2 * time.Second},
		},
		{
			name:       "503 with Retry-After HTTP-date then 200",
			replies:    []reply{{503, clockNow.Add(3 * time.Second).Format(http.TimeFormat), "busy"}, {200, "", "ok"}},
			wantStatus: 200, wantBody: "ok", wantHits: 2,
			wantSleeps: []time.Duration{3 * time.Second},
		},
		{
			name:       "no Retry-After backs off exponentially",
			replies:    []reply{{429, "", ""}, {429, "", ""}, {200, "", "ok"}},
			wantStatus: 200, wantBody: "ok", wantHits: 3,
			wantSleeps: []time.Duration{time.Second, 2 * time.Second},
		},
		{
			name:       "Retry-After date in the past waits the backoff step",
			replies:    []reply{{503, clockNow.Add(-10 * time.Second).Format(http.TimeFormat), ""}, {503, clockNow.Add(-10 * time.Second).Format(http.TimeFormat), ""}, {200, "", "ok"}},
			wantStatus: 200, wantBody: "ok", wantHits: 3,
			wantSleeps: []time.Duration{time.Second, 2 * time.Second},
		},
		{
			name:       "Retry-After 0 waits the backoff step",
			replies:    []reply{{429, "0", ""}, {200, "", "ok"}},
			wantStatus: 200, wantBody: "ok", wantHits: 2,
			wantSleeps: []time.Duration{time.Second},
		},
		{
			name:       "negative Retry-After falls back to backoff",
			replies:    []reply{{429, "-5", ""}, {200, "", "ok"}},
			wantStatus: 200, wantBody: "ok", wantHits: 2,
			wantSleeps: []time.Duration{time.Second},
		},
		{
			name:       "unparseable Retry-After falls back to backoff",
			replies:    []reply{{429, "garbage", ""}, {429, "garbage", ""}, {200, "", "ok"}},
			wantStatus: 200, wantBody: "ok", wantHits: 3,
			wantSleeps: []time.Duration{time.Second, 2 * time.Second},
		},
		{
			name:       "always 429 returns the last 429 after MaxRetries retries",
			replies:    []reply{{429, "1", "slow down"}},
			wantStatus: 429, wantBody: "slow down", wantHits: 4,
			wantSleeps: []time.Duration{time.Second, time.Second, time.Second},
		},
		{
			name:       "Retry-After over MaxWait returns at once",
			replies:    []reply{{429, "120", "come back later"}},
			wantStatus: 429, wantBody: "come back later", wantHits: 1,
		},
		{
			// Amazon serves bot-check pages as 503 without Retry-After; those
			// are not rate limits and must reach the provider at once.
			name:       "503 without Retry-After is returned at once",
			replies:    []reply{{503, "", "bot check"}},
			wantStatus: 503, wantBody: "bot check", wantHits: 1,
		},
		{
			name:       "other errors are not retried",
			replies:    []reply{{500, "1", "boom"}},
			wantStatus: 500, wantBody: "boom", wantHits: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, hits := statusServer(t, tt.replies...)
			clock := newFakeClock()
			waits := &waitRecorder{}
			client := &http.Client{Transport: newTestThrottle(clock, waits)}

			resp, err := Get(context.Background(), client, srv.URL, nil)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if resp.Status != tt.wantStatus || string(resp.Body) != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", resp.Status, resp.Body, tt.wantStatus, tt.wantBody)
			}
			if got := hits.Load(); got != tt.wantHits {
				t.Errorf("server hits = %d, want %d", got, tt.wantHits)
			}
			if got := clock.recorded(); !slices.Equal(got, tt.wantSleeps) {
				t.Errorf("sleeps = %v, want %v", got, tt.wantSleeps)
			}
			var wantWaits []waitRecord
			host := strings.TrimPrefix(srv.URL, "http://")
			for _, d := range tt.wantSleeps {
				wantWaits = append(wantWaits, waitRecord{host, d})
			}
			if got := waits.recorded(); !slices.Equal(got, wantWaits) {
				t.Errorf("OnWait calls = %v, want %v", got, wantWaits)
			}
		})
	}
}

func TestThrottleRetriesReplayableBodies(t *testing.T) {
	// IGDB searches are POSTs. A rate-limited request was refused, so a
	// body the client can replay (GetBody) is sent again.
	srv, hits := statusServer(t, reply{429, "1", "slow"}, reply{200, "", "ok"})
	var bodies []string
	var mu sync.Mutex
	inner := srv.Config.Handler
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		inner.ServeHTTP(w, r)
	})
	clock := newFakeClock()
	client := &http.Client{Transport: newTestThrottle(clock, &waitRecorder{})}

	resp, err := client.Post(srv.URL, "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || readAll(t, resp) != "ok" {
		t.Errorf("status = %d, want the retry's 200", resp.StatusCode)
	}
	if hits.Load() != 2 || !slices.Equal(clock.recorded(), []time.Duration{time.Second}) {
		t.Errorf("hits=%d sleeps=%v, want two attempts and one 1s wait", hits.Load(), clock.recorded())
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(bodies, []string{"payload", "payload"}) {
		t.Errorf("bodies = %q, want the payload sent twice", bodies)
	}
}

func TestThrottleDoesNotRetryOneShotBodies(t *testing.T) {
	srv, hits := statusServer(t, reply{429, "1", "slow"})
	clock := newFakeClock()
	client := &http.Client{Transport: newTestThrottle(clock, &waitRecorder{})}

	// A plain io.Reader has no GetBody, so it cannot be sent again.
	req, err := http.NewRequest(http.MethodPost, srv.URL, io.MultiReader(strings.NewReader("payload")))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 429 || readAll(t, resp) != "slow" {
		t.Errorf("status = %d, want the 429 passed through", resp.StatusCode)
	}
	if hits.Load() != 1 || len(clock.recorded()) != 0 {
		t.Errorf("hits=%d sleeps=%v, want one attempt and no sleep", hits.Load(), clock.recorded())
	}
}

func TestThrottleNegativeMaxRetriesNeverRetries(t *testing.T) {
	srv, hits := statusServer(t, reply{429, "1", "slow"}, reply{200, "", "ok"})
	clock := newFakeClock()
	th := newTestThrottle(clock, &waitRecorder{})
	th.MaxRetries = -1
	client := &http.Client{Transport: th}

	resp, err := Get(context.Background(), client, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 429 || string(resp.Body) != "slow" {
		t.Errorf("got %d %q, want the 429 passed through", resp.Status, resp.Body)
	}
	if hits.Load() != 1 || len(clock.recorded()) != 0 {
		t.Errorf("hits=%d sleeps=%v, want one attempt and no sleep", hits.Load(), clock.recorded())
	}
}

func TestThrottleSharesTheHostBlock(t *testing.T) {
	srvA, hitsA := statusServer(t, reply{429, "2", ""}, reply{200, "", "a"})
	srvB, hitsB := statusServer(t, reply{200, "", "b"})
	clock := newFakeClock()
	clock.sleeping = make(chan time.Duration, 4)
	clock.release = make(chan struct{})
	waits := &waitRecorder{}
	client := &http.Client{Transport: newTestThrottle(clock, waits)}

	type result struct {
		body string
		err  error
	}
	getA := func(out chan<- result) {
		resp, err := Get(context.Background(), client, srvA.URL, nil)
		if err != nil {
			out <- result{err: err}
			return
		}
		out <- result{body: string(resp.Body)}
	}

	first := make(chan result, 1)
	go getA(first)
	select {
	case d := <-clock.sleeping:
		if d != 2*time.Second {
			t.Fatalf("first request sleeps %v, want 2s", d)
		}
	case r := <-first:
		t.Fatalf("first request to A did not wait for its 429: %+v", r)
	case <-time.After(5 * time.Second):
		t.Fatal("first request to A did not wait for its 429")
	}

	// Host B is not blocked: it answers while host A is waiting.
	resp, err := Get(context.Background(), client, srvB.URL, nil)
	if err != nil || string(resp.Body) != "b" {
		t.Fatalf("host B: resp=%v err=%v", resp, err)
	}

	// A second request to host A waits for the same block.
	second := make(chan result, 1)
	go getA(second)
	select {
	case d := <-clock.sleeping:
		if d != 2*time.Second {
			t.Errorf("second request to A sleeps %v, want 2s", d)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second request to A did not wait for the block")
	}
	if hitsA.Load() != 1 {
		t.Errorf("host A hits during the block = %d, want 1", hitsA.Load())
	}

	close(clock.release)
	for _, ch := range []chan result{first, second} {
		if r := <-ch; r.err != nil || r.body != "a" {
			t.Errorf("host A result = %+v, want body a", r)
		}
	}
	if hitsA.Load() != 3 || hitsB.Load() != 1 {
		t.Errorf("hits A=%d B=%d, want 3 and 1", hitsA.Load(), hitsB.Load())
	}
	for _, w := range waits.recorded() {
		if w.host != strings.TrimPrefix(srvA.URL, "http://") {
			t.Errorf("OnWait for host %q, want only host A", w.host)
		}
	}
}

func TestThrottleSpacesRequestsToAHost(t *testing.T) {
	srv, _ := statusServer(t, reply{200, "", "ok"})
	clock := newFakeClock()
	waits := &waitRecorder{}
	th := newTestThrottle(clock, waits)
	th.MinInterval = map[string]time.Duration{strings.TrimPrefix(srv.URL, "http://"): 100 * time.Millisecond}
	client := &http.Client{Transport: th}

	for i := 0; i < 3; i++ {
		if _, err := Get(context.Background(), client, srv.URL, nil); err != nil {
			t.Fatal(err)
		}
	}
	want := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond}
	if got := clock.recorded(); !slices.Equal(got, want) {
		t.Errorf("sleeps = %v, want %v", got, want)
	}
	if got := waits.recorded(); len(got) != 0 {
		t.Errorf("OnWait calls = %v, want none for sub-second spacing", got)
	}
}

func TestThrottleMinIntervalMatchesHostname(t *testing.T) {
	srv, _ := statusServer(t, reply{200, "", "ok"})
	clock := newFakeClock()
	th := newTestThrottle(clock, &waitRecorder{})
	th.MinInterval = map[string]time.Duration{"127.0.0.1": 50 * time.Millisecond}
	client := &http.Client{Transport: th}

	for i := 0; i < 2; i++ {
		if _, err := Get(context.Background(), client, srv.URL, nil); err != nil {
			t.Fatal(err)
		}
	}
	if got := clock.recorded(); !slices.Equal(got, []time.Duration{50 * time.Millisecond}) {
		t.Errorf("sleeps = %v, want [50ms]", got)
	}
}

func TestThrottleWaitHonorsContext(t *testing.T) {
	srv, hits := statusServer(t, reply{429, "30", ""})
	th := &Throttle{} // real clock and sleep
	client := &http.Client{Transport: th}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	_, err := Get(ctx, client, srv.URL, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("returned after %v, want soon after the cancel", elapsed)
	}
	if hits.Load() != 1 {
		t.Errorf("hits = %d, want 1", hits.Load())
	}
}

func TestThrottleAttemptTimeout(t *testing.T) {
	t.Run("body read after RoundTrip is not cut off", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "head-")
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
			io.WriteString(w, "tail")
		}))
		defer srv.Close()
		client := &http.Client{Transport: &Throttle{AttemptTimeout: 2 * time.Second}}

		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
		if got := readAll(t, resp); got != "head-tail" {
			t.Errorf("body = %q, want head-tail", got)
		}
	})
	t.Run("a hanging attempt times out", func(t *testing.T) {
		unblock := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-unblock:
			case <-r.Context().Done():
			}
		}))
		defer srv.Close()
		defer close(unblock)
		client := &http.Client{Transport: &Throttle{AttemptTimeout: 50 * time.Millisecond}}

		_, err := Get(context.Background(), client, srv.URL, nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
	})
}

func TestThrottleSetOnWait(t *testing.T) {
	srv, _ := statusServer(t, reply{429, "1", ""}, reply{200, "", "ok"})
	clock := newFakeClock()
	th := &Throttle{now: clock.now, sleep: clock.sleep}
	waits := &waitRecorder{}
	th.SetOnWait(waits.onWait)
	client := &http.Client{Transport: th}

	if _, err := Get(context.Background(), client, srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	if got := waits.recorded(); len(got) != 1 || got[0].d != time.Second {
		t.Errorf("OnWait calls = %v, want one 1s wait", got)
	}
}

func TestNewClientUsesTheSharedThrottle(t *testing.T) {
	c := NewClient()
	if c.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0 (the throttle times each attempt)", c.Timeout)
	}
	if c.Transport != DefaultThrottle {
		t.Errorf("Transport = %T, want DefaultThrottle", c.Transport)
	}
	for host, want := range map[string]time.Duration{
		"api.themoviedb.org": 25 * time.Millisecond,
		"www.wikidata.org":   200 * time.Millisecond,
		// MusicBrainz allows at most one request per second.
		"musicbrainz.org": time.Second,
		// IGDB allows four requests per second.
		"api.igdb.com": 250 * time.Millisecond,
		// The Met asks for at most 80 requests per second.
		"collectionapi.metmuseum.org": 25 * time.Millisecond,
		// Wikimedia: the Commons API like Wikidata; media downloads spaced
		// so that at most two run at once (robot policy).
		"commons.wikimedia.org": 200 * time.Millisecond,
		"upload.wikimedia.org":  500 * time.Millisecond,
		"thumb.wikimedia.org":   500 * time.Millisecond,
		// BHL publishes no rate numbers: its API and page scans (served
		// from S3) are spaced gently.
		"www.biodiversitylibrary.org":              500 * time.Millisecond,
		"bhl-open-data.s3.us-east-2.amazonaws.com": 500 * time.Millisecond,
	} {
		if got := DefaultThrottle.MinInterval[host]; got != want {
			t.Errorf("DefaultThrottle.MinInterval[%q] = %v, want %v", host, got, want)
		}
	}
}
