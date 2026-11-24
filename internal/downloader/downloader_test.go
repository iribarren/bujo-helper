package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/providers"
	"bujo-helper/internal/web"
)

var (
	jpegBody = append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0xAB}, 4096)...)
	pngBody  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0xCD}, 2048)...)
	tinyGIF  = []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
)

// imageServer serves a few fixed responses used as candidate image URLs.
func imageServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	serve := func(path, contentType string, body []byte) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("User-Agent"), "Chrome") {
				http.Error(w, "browser UA required", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Write(body)
		})
	}
	serve("/cover.jpg", "image/jpeg", jpegBody)
	serve("/other.jpg", "image/jpeg", pngBody)
	serve("/noext", "image/png", pngBody)
	serve("/tiny.gif", "image/gif", tinyGIF)
	serve("/page.html", "text/html; charset=utf-8", bytes.Repeat([]byte("<p>not an image</p>"), 200))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// fakeProvider answers searches with a function of the item.
type fakeProvider struct {
	name   string
	search func(ctx context.Context, item media.Item) ([]media.Candidate, error)
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	return f.search(ctx, item)
}

// returns builds a provider that returns one candidate with the given URLs.
func returns(name string, urls ...string) *fakeProvider {
	return &fakeProvider{name: name, search: func(_ context.Context, item media.Item) ([]media.Candidate, error) {
		return []media.Candidate{{Title: item.Title + " (match)", Creator: item.Creator, ImageURLs: urls, Source: name}}, nil
	}}
}

func notFound(name string) *fakeProvider {
	return &fakeProvider{name: name, search: func(context.Context, media.Item) ([]media.Candidate, error) {
		return nil, fmt.Errorf("%w: nothing", media.ErrNotFound)
	}}
}

// skipped builds an optional provider that is not configured (no key).
func skipped(name string) *fakeProvider {
	return &fakeProvider{name: name, search: func(context.Context, media.Item) ([]media.Candidate, error) {
		return nil, fmt.Errorf("no key: %w", media.ErrSkipped)
	}}
}

func newDownloader(srv *httptest.Server, providers ...media.Provider) *Downloader {
	reg := media.NewRegistry()
	if err := reg.Register(media.MediaType{ID: "books", Providers: providers, LocalEditions: true}); err != nil {
		panic(err)
	}
	return &Downloader{Registry: reg, Client: srv.Client()}
}

// run executes a batch and returns the results sorted by index.
func run(t *testing.T, d *Downloader, ctx context.Context, req Request) ([]ItemResult, Summary) {
	t.Helper()
	var mu sync.Mutex
	var results []ItemResult
	sum, err := d.Run(ctx, req, func(r ItemResult) {
		mu.Lock()
		defer mu.Unlock()
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Index < results[j].Index })
	if len(results) != len(req.Items) {
		t.Fatalf("got %d results for %d items", len(results), len(req.Items))
	}
	return results, sum
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFallbackToSecondProvider(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, notFound("first"), returns("second", srv.URL+"/cover.jpg"))
	dir := t.TempDir()

	res, sum := run(t, d, context.Background(), Request{
		MediaType: "books",
		Items:     []media.Item{{Title: "Dune", Creator: "Frank Herbert"}},
		OutputDir: dir,
	})

	r := res[0]
	if r.Status != StatusOK || r.Source != "second" || r.Matched != "Dune (match) - Frank Herbert" {
		t.Fatalf("result = %+v", r)
	}
	if want := filepath.Join(dir, "Dune - Frank Herbert.jpg"); r.Path != want {
		t.Errorf("Path = %q, want %q", r.Path, want)
	}
	if !bytes.Equal(readFile(t, r.Path), jpegBody) {
		t.Error("saved bytes differ from served image")
	}
	if sum != (Summary{OK: 1}) {
		t.Errorf("summary = %+v", sum)
	}
}

func TestImageValidation(t *testing.T) {
	srv := imageServer(t)
	tests := []struct {
		name       string
		providers  []media.Provider
		wantStatus string
		wantSource string
		wantFile   string
		wantErr    []string
	}{
		{
			name:       "tiny GIF placeholder is skipped for the next URL",
			providers:  []media.Provider{returns("p", srv.URL+"/tiny.gif", srv.URL+"/cover.jpg")},
			wantStatus: StatusOK, wantSource: "p", wantFile: "Emma.jpg",
		},
		{
			name:       "tiny GIF only falls through to the next provider",
			providers:  []media.Provider{returns("p1", srv.URL+"/tiny.gif"), returns("p2", srv.URL+"/noext")},
			wantStatus: StatusOK, wantSource: "p2", wantFile: "Emma.png",
		},
		{
			name:       "non-image and missing URLs fail",
			providers:  []media.Provider{returns("p1", srv.URL+"/page.html", srv.URL+"/missing.jpg"), notFound("p2")},
			wantStatus: StatusFailed,
			wantErr:    []string{"p1: no downloadable cover image", "p2: not found: nothing"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			d := newDownloader(srv, tt.providers...)
			res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Emma"}}, OutputDir: dir})
			r := res[0]
			if r.Status != tt.wantStatus || r.Source != tt.wantSource {
				t.Fatalf("result = %+v", r)
			}
			if tt.wantFile != "" && r.Path != filepath.Join(dir, tt.wantFile) {
				t.Errorf("Path = %q, want %s", r.Path, tt.wantFile)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(r.Error, want) {
					t.Errorf("Error = %q, want it to contain %q", r.Error, want)
				}
			}
			if tt.wantStatus == StatusFailed {
				if entries, _ := os.ReadDir(dir); len(entries) != 0 {
					t.Errorf("failed item left files: %v", entries)
				}
			}
		})
	}
}

func TestDedupeIsRaceSafe(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, returns("p", srv.URL+"/cover.jpg"))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dune - Frank Herbert.jpg"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := make([]media.Item, 5)
	for i := range items {
		items[i] = media.Item{Title: "Dune", Creator: "Frank Herbert"}
	}
	res, sum := run(t, d, context.Background(), Request{MediaType: "books", Items: items, OutputDir: dir, Workers: 5})
	if sum.OK != 5 {
		t.Fatalf("summary = %+v, results = %+v", sum, res)
	}

	var got []string
	for _, r := range res {
		got = append(got, filepath.Base(r.Path))
	}
	sort.Strings(got)
	want := []string{
		"Dune - Frank Herbert-2.jpg", "Dune - Frank Herbert-3.jpg", "Dune - Frank Herbert-4.jpg",
		"Dune - Frank Herbert-5.jpg", "Dune - Frank Herbert-6.jpg",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("files = %q, want %q", got, want)
	}
	if string(readFile(t, filepath.Join(dir, "Dune - Frank Herbert.jpg"))) != "existing" {
		t.Error("existing file was overwritten")
	}
}

func TestWindowsUnsafeNames(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, returns("p", srv.URL+"/cover.jpg"))
	dir := t.TempDir()
	res, _ := run(t, d, context.Background(), Request{
		MediaType: "books",
		Items:     []media.Item{{Title: `AC/DC: "Live"  <1990>?`, Creator: `Who|*\Else...`}},
		OutputDir: dir,
	})
	if want := filepath.Join(dir, "ACDC Live 1990 - WhoElse.jpg"); res[0].Path != want {
		t.Errorf("Path = %q, want %q", res[0].Path, want)
	}
}

func TestSafeName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Dune", "Dune"},
		{`a<b>c:d"e/f\g|h?i*j`, "abcdefghij"},
		{"tab\there\nnew\x00line", "tab here newline"},
		{"  spaced   out  ", "spaced out"},
		{"trailing dots... . ", "trailing dots"},
		{"???", ""},
		{"Cien años de soledad", "Cien años de soledad"},
		{strings.Repeat("é", 130), strings.Repeat("é", 120)},
	}
	for _, tt := range tests {
		if got := safeName(tt.in); got != tt.want {
			t.Errorf("safeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFileName(t *testing.T) {
	tests := []struct {
		item media.Item
		ext  string
		want string
	}{
		{media.Item{Title: "Dune", Creator: "Frank Herbert"}, ".jpg", "Dune - Frank Herbert.jpg"},
		{media.Item{Title: "Dune"}, ".png", "Dune.png"},
		{media.Item{Title: "Dune", Creator: "???"}, ".jpg", "Dune.jpg"},
		{media.Item{Title: "???"}, ".jpg", "cover.jpg"},
		{media.Item{Title: "CON"}, ".jpg", "CON_.jpg"},
		{media.Item{Title: "lpt1"}, ".png", "lpt1_.png"},
		{media.Item{Title: "CON", Creator: "X"}, ".jpg", "CON - X.jpg"},
	}
	for _, tt := range tests {
		if got := fileName(tt.item, tt.ext); got != tt.want {
			t.Errorf("fileName(%+v) = %q, want %q", tt.item, got, tt.want)
		}
	}
}

func TestImageExt(t *testing.T) {
	tests := []struct{ url, contentType, want string }{
		{"https://x/a/10000x10000bb.jpg", "image/jpeg", ".jpg"},
		{"https://x/b/id/1-L.jpg?default=false", "image/jpeg", ".jpg"},
		{"https://x/img.PNG", "image/png", ".png"},
		{"https://x/noext", "image/webp", ".webp"},
		{"https://x/noext", "image/png; charset=binary", ".png"},
		{"https://x/thing.php", "image/gif", ".gif"},
		{"https://x/noext", "image/x-unknown", ".jpg"},
	}
	for _, tt := range tests {
		if got := imageExt(tt.url, tt.contentType); got != tt.want {
			t.Errorf("imageExt(%q, %q) = %q, want %q", tt.url, tt.contentType, got, tt.want)
		}
	}
}

func TestOneFailureDoesNotStopBatch(t *testing.T) {
	srv := imageServer(t)
	p := &fakeProvider{name: "p", search: func(_ context.Context, item media.Item) ([]media.Candidate, error) {
		if item.Title == "Bad" {
			return nil, errors.New("boom")
		}
		return []media.Candidate{{Title: item.Title, ImageURLs: []string{srv.URL + "/cover.jpg"}}}, nil
	}}
	d := newDownloader(srv, p)
	items := []media.Item{{Title: "A"}, {Title: "Bad"}, {Title: "C"}, {Title: "   "}}
	res, sum := run(t, d, context.Background(), Request{MediaType: "books", Items: items, OutputDir: filepath.Join(t.TempDir(), "nested", "out")})

	wantStatus := []string{StatusOK, StatusFailed, StatusOK, StatusFailed}
	for i, r := range res {
		if r.Index != i || r.Item != items[i] || r.Status != wantStatus[i] {
			t.Errorf("result %d = %+v, want status %s", i, r, wantStatus[i])
		}
	}
	if res[1].Error != "p: boom" {
		t.Errorf("Error = %q", res[1].Error)
	}
	if sum != (Summary{OK: 2, Failed: 2}) {
		t.Errorf("summary = %+v", sum)
	}
}

func TestCancellation(t *testing.T) {
	srv := imageServer(t)
	started := make(chan struct{})
	var once sync.Once
	p := &fakeProvider{name: "p", search: func(ctx context.Context, item media.Item) ([]media.Candidate, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	d := newDownloader(srv, p)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	items := []media.Item{{Title: "A"}, {Title: "B"}, {Title: "C"}, {Title: "D"}}
	res, sum := run(t, d, ctx, Request{MediaType: "books", Items: items, OutputDir: t.TempDir(), Workers: 1})
	for _, r := range res {
		if r.Status != StatusCancelled {
			t.Errorf("result = %+v, want cancelled", r)
		}
	}
	if sum != (Summary{Cancelled: 4}) {
		t.Errorf("summary = %+v", sum)
	}
}

func TestRunErrors(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, returns("p", srv.URL+"/cover.jpg"))
	if _, err := d.Run(context.Background(), Request{MediaType: "games", OutputDir: t.TempDir()}, nil); err == nil {
		t.Error("unknown media type should fail")
	}
	if _, err := d.Run(context.Background(), Request{MediaType: "books"}, nil); err == nil {
		t.Error("empty output dir should fail")
	}
	// A nil callback is allowed.
	sum, err := d.Run(context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "X"}}, OutputDir: t.TempDir()}, nil)
	if err != nil || sum.OK != 1 {
		t.Errorf("nil callback run = %+v, %v", sum, err)
	}
}

func TestFetchImageReasons(t *testing.T) {
	srv := imageServer(t)
	tests := []struct {
		path    string
		wantErr string // empty means success
	}{
		{"/cover.jpg", ""},
		{"/missing.jpg", "HTTP 404"},
		{"/page.html", "not an image (text/html; charset=utf-8)"},
		{"/tiny.gif", "image too small"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			data, _, err := fetchImage(context.Background(), srv.Client(), srv.URL+tt.path)
			if tt.wantErr == "" {
				if err != nil || len(data) == 0 {
					t.Fatalf("fetchImage = %d bytes, %v", len(data), err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestFetchImageBodyTooLarge(t *testing.T) {
	srv := imageServer(t)
	old := web.MaxBodySize
	web.MaxBodySize = 2048
	t.Cleanup(func() { web.MaxBodySize = old })

	_, _, err := fetchImage(context.Background(), srv.Client(), srv.URL+"/cover.jpg")
	if !errors.Is(err, web.ErrBodyTooLarge) {
		t.Fatalf("err = %v, want ErrBodyTooLarge", err)
	}
}

func TestProviderErrorIncludesLastReason(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, returns("amazon", srv.URL+"/tiny.gif", srv.URL+"/missing.jpg"))
	res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Emma"}}, OutputDir: t.TempDir()})
	if want := "amazon: no downloadable cover image (last: HTTP 404)"; res[0].Error != want {
		t.Errorf("Error = %q, want %q", res[0].Error, want)
	}
}

func TestNoLocalEditionPassWithoutLocalEditions(t *testing.T) {
	srv := imageServer(t)
	var secondCalls atomic.Int32
	second := &fakeProvider{name: "second", search: func(context.Context, media.Item) ([]media.Candidate, error) {
		secondCalls.Add(1)
		return nil, fmt.Errorf("%w: nothing", media.ErrNotFound)
	}}
	reg := media.NewRegistry()
	if err := reg.Register(media.MediaType{
		ID:            "movies",
		LocalEditions: false,
		Providers:     []media.Provider{returns("first", srv.URL+"/cover.jpg"), second},
	}); err != nil {
		t.Fatal(err)
	}
	d := &Downloader{Registry: reg, Client: srv.Client()}

	// Spain has ISBN prefixes, so a books-like type would run pass 1 and
	// search every provider before downloading.
	res, _ := run(t, d, context.Background(), Request{
		MediaType: "movies",
		Items:     []media.Item{{Title: "Dune", Creator: "2021"}},
		OutputDir: t.TempDir(),
		Options:   media.Options{Country: "es"},
	})
	if res[0].Status != StatusOK || res[0].Source != "first" {
		t.Fatalf("result = %+v", res[0])
	}
	if n := secondCalls.Load(); n != 0 {
		t.Errorf("second provider searched %d times, want 0", n)
	}
}

func TestItemResultCodes(t *testing.T) {
	srv := imageServer(t)
	boom := &fakeProvider{name: "boom", search: func(context.Context, media.Item) ([]media.Candidate, error) {
		return nil, errors.New("boom")
	}}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name      string
		providers []media.Provider
		ctx       context.Context
		title     string
		wantCode  string
		wantError string
	}{
		{name: "ok", providers: []media.Provider{returns("p", srv.URL+"/cover.jpg")}, wantCode: ""},
		{name: "every provider not found", providers: []media.Provider{notFound("a"), notFound("b")}, wantCode: CodeNotFound, wantError: "a: not found: nothing; b: not found: nothing"},
		{name: "not found and another failure", providers: []media.Provider{notFound("a"), boom}, wantCode: "", wantError: "a: not found: nothing; boom: boom"},
		{name: "generic failure", providers: []media.Provider{boom}, wantCode: "", wantError: "boom: boom"},
		// A skipped optional provider (no key) neither prevents not found
		// nor appears among the reasons, unless every provider was skipped.
		{name: "not found and an optional provider skipped", providers: []media.Provider{notFound("commons-nature"), skipped("bhl")}, wantCode: CodeNotFound, wantError: "commons-nature: not found: nothing"},
		{name: "skipped before not found", providers: []media.Provider{skipped("a"), notFound("b")}, wantCode: CodeNotFound, wantError: "b: not found: nothing"},
		{name: "another failure and a skipped provider", providers: []media.Provider{boom, skipped("bhl")}, wantCode: "", wantError: "boom: boom"},
		{name: "not found, skipped and another failure", providers: []media.Provider{notFound("a"), skipped("bhl"), boom}, wantCode: "", wantError: "a: not found: nothing; boom: boom"},
		{name: "every provider skipped", providers: []media.Provider{skipped("a"), skipped("b")}, wantCode: "", wantError: "a: no key: provider skipped; b: no key: provider skipped"},
		{name: "empty title", providers: []media.Provider{boom}, title: "  ", wantCode: CodeEmptyTitle, wantError: "empty title"},
		{name: "no providers", wantCode: CodeNoProviders, wantError: "no providers for media type books"},
		{name: "cancelled", providers: []media.Provider{boom}, ctx: cancelled, wantCode: CodeCancelled, wantError: "cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			title := tt.title
			if title == "" {
				title = "Dune"
			}
			d := newDownloader(srv, tt.providers...)
			res, _ := run(t, d, ctx, Request{MediaType: "books", Items: []media.Item{{Title: title}}, OutputDir: t.TempDir()})
			r := res[0]
			if got := Code(r.Err); got != tt.wantCode {
				t.Errorf("Code = %q, want %q (result %+v)", got, tt.wantCode, r)
			}
			if r.Error != tt.wantError {
				t.Errorf("Error = %q, want %q", r.Error, tt.wantError)
			}
			if (r.Err == nil) != (tt.wantError == "") {
				t.Errorf("Err = %v, want it set only for a failure", r.Err)
			}
		})
	}
}

func TestEverySkippedProviderIsTheCause(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, skipped("bhl"))
	res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Robin"}}, OutputDir: t.TempDir()})
	if r := res[0]; r.Status != StatusFailed || !errors.Is(r.Err, media.ErrSkipped) || errors.Is(r.Err, media.ErrNotFound) {
		t.Errorf("result = %+v, want a failure caused by the skip, not not found", r)
	}
}

func TestCodeOfSentinels(t *testing.T) {
	for err, want := range map[error]string{
		nil:                                    "",
		errors.New("boom"):                     "",
		ErrCancelled:                           CodeCancelled,
		ErrEmptyTitle:                          CodeEmptyTitle,
		fmt.Errorf("x: %w", ErrNoProviders):    CodeNoProviders,
		fmt.Errorf("x: %w", media.ErrNotFound): CodeNotFound,
	} {
		if got := Code(err); got != want {
			t.Errorf("Code(%v) = %q, want %q", err, got, want)
		}
	}
}

// A movie searched without a TMDB key that no other provider finds: the
// key message leads the reason (TMDB comes first), and the item is a
// failure, not a not-found, since TMDB was never searched.
func TestKeylessMovieNotFoundElsewhere(t *testing.T) {
	srv := imageServer(t)
	reg := media.NewRegistry()
	if err := reg.Register(media.MediaType{
		ID:        "movies",
		Providers: []media.Provider{providers.NewTMDB(srv.Client(), func() string { return "" }), notFound("imdb"), notFound("wikidata-itunes")},
	}); err != nil {
		t.Fatal(err)
	}
	d := &Downloader{Registry: reg, Client: srv.Client()}
	res, _ := run(t, d, context.Background(), Request{MediaType: "movies", Items: []media.Item{{Title: "Dune"}}, OutputDir: t.TempDir()})
	r := res[0]
	want := "tmdb: no TMDB API key (set it in Settings); imdb: not found: nothing; wikidata-itunes: not found: nothing"
	if r.Status != StatusFailed || r.Error != want {
		t.Errorf("result = %+v, want a failure %q", r, want)
	}
	if got := Code(r.Err); got != "" {
		t.Errorf("Code = %q, want none (TMDB was not searched)", got)
	}
}
