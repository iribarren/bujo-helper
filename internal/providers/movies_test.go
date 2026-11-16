package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

const (
	testV3Key = "0123456789abcdef0123456789abcdef"
	testV4Key = "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiIwMTIzNDU2Nzg5YWJjZGVmIiwic2NvcGVzIjpbImFwaV9yZWFkIl19.c2lnbmF0dXJl"
)

func staticKey(k string) func() string { return func() string { return k } }

func newTestTMDB(srv *httptest.Server, key string) *TMDB {
	p := NewTMDB(srv.Client(), staticKey(key))
	p.BaseURL = srv.URL + "/3"
	p.ImageBase = "https://img.example/t/p"
	return p
}

func candidateYears(cs []media.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Year
	}
	return out
}

func TestTMDBWithoutKeySkipsWithoutHTTP(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search.json"))
	for name, key := range map[string]func() string{
		"nil key func": nil,
		"empty key":    staticKey(""),
		"blank key":    staticKey("   "),
	} {
		t.Run(name, func(t *testing.T) {
			p := NewTMDB(srv.Client(), key)
			p.BaseURL = srv.URL
			_, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{})
			if err == nil || err.Error() != "no TMDB API key (set it in Settings)" {
				t.Errorf("err = %v, want the missing-key error", err)
			}
			if errors.Is(err, media.ErrNotFound) {
				t.Error("a missing key must not wrap ErrNotFound")
			}
		})
	}
	if n := rec.calls.Load(); n != 0 {
		t.Errorf("server calls = %d, want 0", n)
	}
}

func TestTMDBSearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search.json"))
	p := newTestTMDB(srv, testV3Key)

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	if rec.last.URL.Path != "/3/search/movie" {
		t.Errorf("path = %q", rec.last.URL.Path)
	}
	q := rec.last.URL.Query()
	for key, want := range map[string]string{
		"query": "Dune", "year": "2021", "include_adult": "false",
		"language": "es-ES", "region": "ES", "api_key": testV3Key,
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if auth := rec.last.Header.Get("Authorization"); auth != "" {
		t.Errorf("Authorization = %q, want none for a v3 key", auth)
	}
	if ua := rec.last.Header.Get("User-Agent"); ua != web.APIUserAgent {
		t.Errorf("User-Agent = %q", ua)
	}
	if acc := rec.last.Header.Get("Accept"); acc != "application/json" {
		t.Errorf("Accept = %q", acc)
	}

	// The posterless "Dune: Behind the Scenes" is skipped.
	wantTitles := []string{"Dune", "Dune", "Dune: Part Two"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:   "Dune",
		Creator: "2021",
		Year:    "2021",
		ImageURLs: []string{
			"https://img.example/t/p/original/d5NXSklXo0qyIYkgV94XAgMIckC.jpg",
			"https://img.example/t/p/w780/d5NXSklXo0qyIYkgV94XAgMIckC.jpg",
		},
		Source: "tmdb",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
}

func TestTMDBRanksByYear(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search.json"))
	p := newTestTMDB(srv, testV3Key)

	for itemYear, want := range map[string][]string{
		"2021": {"2021", "1984", "2024"},
		"1984": {"1984", "2021", "2024"},
	} {
		got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: itemYear}, media.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if years := candidateYears(got); !reflect.DeepEqual(years, want) {
			t.Errorf("Dune, %s: years = %q, want %q", itemYear, years, want)
		}
	}
}

func TestTMDBBearerToken(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search.json"))
	p := newTestTMDB(srv, testV4Key)

	if _, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{}); err != nil {
		t.Fatal(err)
	}
	if auth := rec.last.Header.Get("Authorization"); auth != "Bearer "+testV4Key {
		t.Errorf("Authorization = %q", auth)
	}
	q := rec.last.URL.Query()
	if q.Has("api_key") {
		t.Error("api_key must not be sent with a v4 token")
	}
	if q.Has("year") {
		t.Error("year must not be sent without an item year")
	}
	if q.Get("language") != "en-US" || q.Get("region") != "US" {
		t.Errorf("language=%q region=%q, want en-US/US by default", q.Get("language"), q.Get("region"))
	}
}

func TestTMDBLanguage(t *testing.T) {
	tests := map[string]string{
		"es": "es-ES", "mx": "es-MX", "ar": "es-AR", "co": "es-CO", "cl": "es-CL", "pe": "es-PE",
		"fr": "fr-FR", "de": "de-DE", "at": "de-DE", "it": "it-IT",
		"pt": "pt-PT", "br": "pt-BR", "gb": "en-US", "us": "en-US", "jp": "en-US", "": "en-US",
	}
	for cc, want := range tests {
		if got := tmdbLanguage(media.Options{Country: cc}); got != want {
			t.Errorf("tmdbLanguage(%q) = %q, want %q", cc, got, want)
		}
	}
}

func TestTMDBRetriesWithoutYear(t *testing.T) {
	var mu sync.Mutex
	var queries []url.Values
	body := fixture(t, "tmdb_search.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.Query())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Has("year") {
			w.Write([]byte(`{"page":1,"results":[],"total_pages":0,"total_results":0}`))
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	p := newTestTMDB(srv, testV3Key)

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2020"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || queries[0].Get("year") != "2020" || queries[1].Has("year") {
		t.Fatalf("queries = %v, want one with year=2020 then one without", queries)
	}
	if len(got) != 3 {
		t.Errorf("got %d candidates, want 3", len(got))
	}
}

func TestTMDBNoPostersIsNotFound(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", []byte(`{"results":[{"title":"Dune","release_date":"2021-09-15","poster_path":null}]}`))
	p := newTestTMDB(srv, testV3Key)

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestTMDBRejectedKey(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 401, "application/json", []byte(`{"status_code":7,"status_message":"Invalid API key: You must be granted a valid key.","success":false}`))
	p := newTestTMDB(srv, testV3Key)

	_, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{})
	if err == nil || err.Error() != "TMDB rejected the API key (HTTP 401)" {
		t.Errorf("err = %v", err)
	}
	if n := rec.calls.Load(); n != 1 {
		t.Errorf("server calls = %d, want 1 (no retry without year after an error)", n)
	}
}

func TestTMDBHTTPError(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 500, "text/plain", []byte("boom"))
	p := newTestTMDB(srv, testV3Key)

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want HTTP 500", err)
	}
}

func newTestIMDb(srv *httptest.Server) *IMDb {
	p := NewIMDb(&http.Client{})
	p.BaseURL = srv.URL + "/suggestion"
	return p
}

func TestIMDbURL(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", []byte(`{"d":[]}`))
	p := newTestIMDb(srv)

	// wantURI is the raw request target, so path escaping is checked too.
	tests := []struct {
		title, wantURI string
	}{
		{"The Matrix", "/suggestion/t/the_matrix.json"},
		{" 2001: A Space Odyssey ", "/suggestion/2/2001:_a_space_odyssey.json"},
		{"Face/Off", "/suggestion/f/face%2Foff.json"},
		{"¡Three Amigos!", "/suggestion/x/%C2%A1three_amigos%21.json"},
		{"Amélie", "/suggestion/a/am%C3%A9lie.json"},
	}
	for _, tt := range tests {
		_, err := p.Search(context.Background(), media.Item{Title: tt.title}, media.Options{})
		if !errors.Is(err, media.ErrNotFound) {
			t.Errorf("%q: err = %v, want ErrNotFound", tt.title, err)
		}
		if got := rec.last.RequestURI; got != tt.wantURI {
			t.Errorf("%q: request URI = %q, want %q", tt.title, got, tt.wantURI)
		}
	}
	if ua := rec.last.Header.Get("User-Agent"); ua != web.BrowserUserAgent {
		t.Errorf("User-Agent = %q, want the browser UA", ua)
	}
}

func TestIMDbSearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "imdb_suggestion.json"))
	p := newTestIMDb(srv)

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The TV series, the name entry and the imageless entry are dropped.
	wantTitles := []string{"Dune: Part One", "Dune"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:     "Dune: Part One",
		Creator:   "2021",
		Year:      "2021",
		ImageURLs: []string{"https://m.media-amazon.com/images/M/MV5BMDQ0NjgyN2YtNWViNS00YjA3LTkxNDktYzFkZTExZGMxZDkxXkEyXkFqcGdeQXVyODE5NzE3OTE@._V1_.jpg"},
		Source:    "imdb",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}

	got, err = p.Search(context.Background(), media.Item{Title: "Dune", Creator: "1984"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if years := candidateYears(got); !reflect.DeepEqual(years, []string{"1984", "2021"}) {
		t.Errorf("Dune, 1984: years = %q, want 1984 first", years)
	}
}

func TestIMDbKeepsMovieLikeTypes(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", []byte(`{"d":[
		{"id":"tt1","l":"A movie","qid":"movie","y":2000,"i":{"imageUrl":"https://img/1.jpg"}},
		{"id":"tt2","l":"A TV movie","qid":"tvMovie","y":2001,"i":{"imageUrl":"https://img/2.jpg"}},
		{"id":"tt3","l":"A short","qid":"short","i":{"imageUrl":"https://img/3.jpg"}},
		{"id":"tt4","l":"A video","qid":"video","y":2003,"i":{"imageUrl":"https://img/4.jpg"}},
		{"id":"tt5","l":"A game","qid":"videoGame","y":2004,"i":{"imageUrl":"https://img/5.jpg"}},
		{"id":"tt6","l":"An episode","qid":"tvEpisode","y":2005,"i":{"imageUrl":"https://img/6.jpg"}}
	]}`))
	p := newTestIMDb(srv)

	got, err := p.Search(context.Background(), media.Item{Title: "zzz"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantTitles := []string{"A movie", "A TV movie", "A short", "A video"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	if got[2].Year != "" || got[2].Creator != "" {
		t.Errorf("an entry without a year has Year=%q Creator=%q, want empty", got[2].Year, got[2].Creator)
	}
}

func TestIMDbHTTPError(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 503, "text/html", []byte("unavailable"))
	p := newTestIMDb(srv)

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want HTTP 503", err)
	}
	if errors.Is(err, media.ErrNotFound) {
		t.Error("an HTTP error must not wrap ErrNotFound")
	}
}

func TestTMDBBlankTitleIsNotFoundWithoutHTTP(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search.json"))
	p := newTestTMDB(srv, testV3Key)

	for _, title := range []string{"", "   "} {
		_, err := p.Search(context.Background(), media.Item{Title: title, Creator: "2021"}, media.Options{})
		if !errors.Is(err, media.ErrNotFound) {
			t.Errorf("title %q: err = %v, want ErrNotFound", title, err)
		}
	}
	if n := rec.calls.Load(); n != 0 {
		t.Errorf("server calls = %d, want 0", n)
	}
}

func TestTMDBRetriesWithoutYearWhenNoPosters(t *testing.T) {
	var mu sync.Mutex
	var queries []url.Values
	body := fixture(t, "tmdb_search.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.Query())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Has("year") {
			w.Write([]byte(`{"page":1,"results":[{"title":"Dune","release_date":"2021-09-15","poster_path":null}],"total_pages":1,"total_results":1}`))
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	p := newTestTMDB(srv, testV3Key)

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || queries[0].Get("year") != "2021" || queries[1].Has("year") {
		t.Fatalf("queries = %v, want one with year=2021 then one without", queries)
	}
	if len(got) == 0 || got[0].Year != "2021" || got[0].ImageURLs[0] != "https://img.example/t/p/original/d5NXSklXo0qyIYkgV94XAgMIckC.jpg" {
		t.Errorf("first = %+v, want the 2021 poster from the yearless search", got)
	}
}

func TestTMDBTransportErrorRedactsKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	p := newTestTMDB(srv, "secret123")
	srv.Close()

	_, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{})
	if err == nil {
		t.Fatal("want a transport error from a closed server")
	}
	if strings.Contains(err.Error(), "secret123") {
		t.Errorf("error leaks the API key: %v", err)
	}
	if !strings.Contains(err.Error(), "REDACTED") {
		t.Errorf("err = %v, want the key replaced by REDACTED", err)
	}
}

func TestTMDBRedactedErrorKeepsContextErrors(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search.json"))
	p := newTestTMDB(srv, "secret123")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Search(ctx, media.Item{Title: "Dune"}, media.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if err != nil && strings.Contains(err.Error(), "secret123") {
		t.Errorf("error leaks the API key: %v", err)
	}
}
