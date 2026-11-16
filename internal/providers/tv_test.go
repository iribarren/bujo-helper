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
)

func newTestTMDBTV(srv *httptest.Server, key string) *TMDB {
	p := NewTMDBTV(srv.Client(), staticKey(key))
	p.BaseURL = srv.URL + "/3"
	p.ImageBase = "https://img.example/t/p"
	return p
}

func newTestIMDbTV(srv *httptest.Server) *IMDb {
	p := NewIMDbTV(&http.Client{})
	p.BaseURL = srv.URL + "/suggestion"
	return p
}

func TestTMDBTVWithoutKeySkipsWithoutHTTP(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search_tv.json"))
	for name, key := range map[string]func() string{
		"nil key func": nil,
		"empty key":    staticKey(""),
		"blank key":    staticKey("   "),
	} {
		t.Run(name, func(t *testing.T) {
			p := NewTMDBTV(srv.Client(), key)
			p.BaseURL = srv.URL
			_, err := p.Search(context.Background(), media.Item{Title: "The Office", Creator: "2005"}, media.Options{})
			if !errors.Is(err, ErrNoTMDBKey) {
				t.Errorf("err = %v, want ErrNoTMDBKey", err)
			}
		})
	}
	if n := rec.calls.Load(); n != 0 {
		t.Errorf("server calls = %d, want 0", n)
	}
}

func TestTMDBTVSearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search_tv.json"))
	p := newTestTMDBTV(srv, testV3Key)

	if p.Name() != "tmdb" {
		t.Errorf("Name = %q, want tmdb", p.Name())
	}
	got, err := p.Search(context.Background(), media.Item{Title: "The Office", Creator: "2005"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	if rec.last.URL.Path != "/3/search/tv" {
		t.Errorf("path = %q, want /3/search/tv", rec.last.URL.Path)
	}
	q := rec.last.URL.Query()
	for key, want := range map[string]string{
		"query": "The Office", "first_air_date_year": "2005", "include_adult": "false",
		"language": "es-ES", "api_key": testV3Key,
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	for _, key := range []string{"year", "region"} {
		if q.Has(key) {
			t.Errorf("query has %s=%q, want none for a TV search", key, q.Get(key))
		}
	}

	// The posterless "The Office Retrospective" is skipped.
	wantTitles := []string{"The Office", "The Office", "The Office: The Accountants"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:   "The Office",
		Creator: "2005",
		Year:    "2005",
		ImageURLs: []string{
			"https://img.example/t/p/original/7DJKHzAi83BmQrWLrYYOqcoKfhR.jpg",
			"https://img.example/t/p/w780/7DJKHzAi83BmQrWLrYYOqcoKfhR.jpg",
		},
		Source: "tmdb",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
}

func TestTMDBTVRanksByFirstAirYear(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "tmdb_search_tv.json"))
	p := newTestTMDBTV(srv, testV3Key)

	for itemYear, want := range map[string][]string{
		"2005": {"2005", "2001", "2006"},
		"2001": {"2001", "2005", "2006"},
	} {
		got, err := p.Search(context.Background(), media.Item{Title: "The Office", Creator: itemYear}, media.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if years := candidateYears(got); !reflect.DeepEqual(years, want) {
			t.Errorf("The Office, %s: years = %q, want %q", itemYear, years, want)
		}
	}
}

func TestTMDBTVRetriesWithoutYear(t *testing.T) {
	var mu sync.Mutex
	var queries []url.Values
	body := fixture(t, "tmdb_search_tv.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.Query())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Has("first_air_date_year") {
			w.Write([]byte(`{"page":1,"results":[{"name":"The Office","first_air_date":"2005-03-24","poster_path":null}],"total_pages":1,"total_results":1}`))
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	p := newTestTMDBTV(srv, testV3Key)

	got, err := p.Search(context.Background(), media.Item{Title: "The Office", Creator: "2005"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || queries[0].Get("first_air_date_year") != "2005" || queries[1].Has("first_air_date_year") {
		t.Fatalf("queries = %v, want one with first_air_date_year=2005 then one without", queries)
	}
	if len(got) == 0 || got[0].Year != "2005" || got[0].Title != "The Office" {
		t.Errorf("first = %+v, want the 2005 series from the yearless search", got)
	}
}

func TestTMDBTVNoPostersIsNotFound(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", []byte(`{"results":[{"name":"The Office","first_air_date":"2005-03-24","poster_path":null}]}`))
	p := newTestTMDBTV(srv, testV3Key)

	_, err := p.Search(context.Background(), media.Item{Title: "The Office"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestTMDBTVTransportErrorRedactsKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	p := newTestTMDBTV(srv, "secret123")
	srv.Close()

	_, err := p.Search(context.Background(), media.Item{Title: "The Office"}, media.Options{})
	if err == nil || strings.Contains(err.Error(), "secret123") {
		t.Errorf("err = %v, want a transport error without the key", err)
	}
}

func TestIMDbTVSearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "imdb_suggestion_tv.json"))
	p := newTestIMDbTV(srv)

	got, err := p.Search(context.Background(), media.Item{Title: "The Office", Creator: "2005"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rec.last.URL.Path != "/suggestion/t/the_office.json" {
		t.Errorf("path = %q", rec.last.URL.Path)
	}
	// The movie, the imageless series and the name entry are dropped; the
	// mini-series is kept.
	wantTitles := []string{"The Office", "The Office", "The Office: The Accountants"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:     "The Office",
		Creator:   "2005",
		Year:      "2005",
		ImageURLs: []string{"https://m.media-amazon.com/images/M/MV5BMDNkOTE4NDQtMTNmYi00MWE0LWE4ZTktYTc0NzhhNWIzNzJiXkEyXkFqcGc@._V1_.jpg"},
		Source:    "imdb",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}

	got, err = p.Search(context.Background(), media.Item{Title: "The Office", Creator: "2001"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if years := candidateYears(got); !reflect.DeepEqual(years, []string{"2001", "2005", "2006"}) {
		t.Errorf("The Office, 2001: years = %q, want 2001 first", years)
	}
}

func TestIMDbKindsByMode(t *testing.T) {
	body := []byte(`{"d":[
		{"id":"tt1","l":"A movie","qid":"movie","y":2000,"i":{"imageUrl":"https://img/1.jpg"}},
		{"id":"tt2","l":"A TV movie","qid":"tvMovie","y":2001,"i":{"imageUrl":"https://img/2.jpg"}},
		{"id":"tt3","l":"A series","qid":"tvSeries","y":2002,"i":{"imageUrl":"https://img/3.jpg"}},
		{"id":"tt4","l":"A mini-series","qid":"tvMiniSeries","y":2003,"i":{"imageUrl":"https://img/4.jpg"}},
		{"id":"tt5","l":"An episode","qid":"tvEpisode","y":2004,"i":{"imageUrl":"https://img/5.jpg"}},
		{"id":"tt6","l":"A special","qid":"tvSpecial","y":2005,"i":{"imageUrl":"https://img/6.jpg"}}
	]}`)
	tests := []struct {
		name string
		new  func(*httptest.Server) *IMDb
		want []string
	}{
		{"movies drop series", newTestIMDb, []string{"A movie", "A TV movie"}},
		{"tv keeps series only", newTestIMDbTV, []string{"A series", "A mini-series"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rec recorder
			srv := rec.serve(t, 200, "application/json", body)
			got, err := tt.new(srv).Search(context.Background(), media.Item{Title: "zzz"}, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(candidateTitles(got), tt.want) {
				t.Errorf("titles = %q, want %q", candidateTitles(got), tt.want)
			}
		})
	}
}

func TestIMDbNotFoundNamesTheMediaKind(t *testing.T) {
	body := []byte(`{"d":[{"id":"tt1","l":"Imageless","qid":"tvSeries","y":2000}]}`)
	tests := []struct {
		name, want, notWant string
		new                 func(*httptest.Server) *IMDb
	}{
		{"movies", "no movie results with a poster", "TV", newTestIMDb},
		{"tv", "no TV series results with a poster", "movie", newTestIMDbTV},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rec recorder
			srv := rec.serve(t, 200, "application/json", body)
			_, err := tt.new(srv).Search(context.Background(), media.Item{Title: "zzz"}, media.Options{})
			if !errors.Is(err, media.ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
			if !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), tt.notWant) {
				t.Errorf("err = %q, want it to say %q", err, tt.want)
			}
		})
	}
}
