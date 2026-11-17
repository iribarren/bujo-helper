package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// fakeAniList serves body with status for every GraphQL POST and records
// the decoded request bodies.
type fakeAniList struct {
	mu       sync.Mutex
	requests []aniListTestRequest
}

type aniListTestRequest struct {
	Method      string
	Path        string
	ContentType string
	Query       string         `json:"query"`
	Variables   map[string]any `json:"variables"`
}

func (f *fakeAniList) serve(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		req := aniListTestRequest{Method: r.Method, Path: r.URL.Path, ContentType: r.Header.Get("Content-Type")}
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("request body %q is not JSON: %v", raw, err)
		}
		f.mu.Lock()
		f.requests = append(f.requests, req)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (f *fakeAniList) calls() []aniListTestRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]aniListTestRequest(nil), f.requests...)
}

func newTestAniList(srv *httptest.Server, mediaType string) *AniList {
	var p *AniList
	if mediaType == "MANGA" {
		p = NewAniListManga(srv.Client())
	} else {
		p = NewAniListAnime(srv.Client())
	}
	p.BaseURL = srv.URL + "/graphql"
	return p
}

func TestAniListSearch(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mediaType string
		fixture   string
		item      media.Item
		want      media.Candidate
		titles    []string
	}{
		{
			name:      "anime",
			mediaType: "ANIME",
			fixture:   "anilist_frieren_anime.json",
			item:      media.Item{Title: "Frieren", Creator: "2023"},
			want: media.Candidate{
				Title:   "Frieren: Beyond Journey’s End",
				Creator: "2023",
				Year:    "2023",
				ImageURLs: []string{
					"https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx154587-qQTzQnEJJ3oB.jpg",
					"https://s4.anilist.co/file/anilistcdn/media/anime/cover/medium/bx154587-qQTzQnEJJ3oB.jpg",
				},
				Source: "anilist",
			},
			// The 3rd season has no English title: the romaji one is used.
			titles: []string{
				"Frieren: Beyond Journey’s End",
				"Sousou no Frieren 3rd Season",
				"Frieren: Beyond Journey’s End Season 2",
			},
		},
		{
			name:      "manga",
			mediaType: "MANGA",
			fixture:   "anilist_berserk_manga.json",
			item:      media.Item{Title: "Berserk", Creator: "1989"},
			want: media.Candidate{
				Title:   "Berserk",
				Creator: "1989",
				Year:    "1989",
				ImageURLs: []string{
					"https://s4.anilist.co/file/anilistcdn/media/manga/cover/large/bx30002-Cul4OeN7bYtn.jpg",
					"https://s4.anilist.co/file/anilistcdn/media/manga/cover/medium/bx30002-Cul4OeN7bYtn.jpg",
				},
				Source: "anilist",
			},
			// Manga have no seasonYear: the start year is used.
			titles: []string{
				"Berserk",
				"Berserk: The Prototype",
				"Berserk of Gluttony",
				"Berserk of Gluttony",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var fake fakeAniList
			srv := fake.serve(t, http.StatusOK, fixture(t, tt.fixture))
			p := newTestAniList(srv, tt.mediaType)
			if p.Name() != "anilist" {
				t.Errorf("Name = %q, want anilist", p.Name())
			}

			got, err := p.Search(context.Background(), tt.item, media.Options{Country: "es"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[0], tt.want) {
				t.Errorf("first = %+v, want %+v", got[0], tt.want)
			}
			if titles := candidateTitles(got); !reflect.DeepEqual(titles, tt.titles) {
				t.Errorf("titles = %q, want %q", titles, tt.titles)
			}

			calls := fake.calls()
			if len(calls) != 1 {
				t.Fatalf("requests = %d, want 1", len(calls))
			}
			req := calls[0]
			if req.Method != http.MethodPost || req.Path != "/graphql" {
				t.Errorf("request = %s %s, want POST /graphql", req.Method, req.Path)
			}
			if req.ContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", req.ContentType)
			}
			for key, want := range map[string]any{
				"search": tt.item.Title, "type": tt.mediaType, "perPage": float64(10), "isAdult": false,
			} {
				if got := req.Variables[key]; got != want {
					t.Errorf("variable %s = %#v, want %#v", key, got, want)
				}
			}
			for _, field := range []string{"sort: SEARCH_MATCH", "isAdult: $isAdult", "extraLarge", "synonyms"} {
				if !strings.Contains(req.Query, field) {
					t.Errorf("query lacks %q:\n%s", field, req.Query)
				}
			}
		})
	}
}

// AniList returns romaji, English and native titles and synonyms; an exact
// match on any of them ranks first, whatever AniList's order.
func TestAniListMatchesAnyTitle(t *testing.T) {
	for _, tt := range []struct {
		name    string
		fixture string
		query   string
		want    string
	}{
		{"romaji", "anilist_frieren_anime.json", "Sousou no Frieren 2nd Season", "Frieren: Beyond Journey’s End Season 2"},
		{"romaji of the top hit", "anilist_shingeki_anime.json", "Shingeki no Kyojin", "Attack on Titan"},
		{"native", "anilist_shingeki_anime.json", "進撃の巨人", "Attack on Titan"},
		{"synonym", "anilist_shingeki_anime.json", "Attack on Titan Season 4", "Attack on Titan Final Season"},
		{"short synonym", "anilist_shingeki_anime.json", "AoT", "Attack on Titan"},
		{"English", "anilist_berserk_manga.json", "Berserk: The Prototype", "Berserk: The Prototype"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var fake fakeAniList
			srv := fake.serve(t, http.StatusOK, fixture(t, tt.fixture))
			got, err := newTestAniList(srv, "ANIME").Search(context.Background(), media.Item{Title: tt.query}, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got[0].Title != tt.want {
				t.Errorf("%q: first = %q, want %q (all %q)", tt.query, got[0].Title, tt.want, candidateTitles(got))
			}
		})
	}
}

// Two anime are titled Berserk: the 1997 series (English "Berserk") and the
// 2016 one (romaji "Berserk"). The year picks one; without it AniList's
// order stands. The Golden Age Arc film (2012) matches on its main title
// and so outranks "Berserk 2", a prefix match. A year only that film has
// lifts it above the exact matches, as for games.
func TestAniListYearBreaksTies(t *testing.T) {
	for _, tt := range []struct {
		creator string
		want    []string
	}{
		{"2016", []string{"2016", "1997", "2012", "2017"}},
		{"1997", []string{"1997", "2016", "2012", "2017"}},
		{"2012", []string{"2012", "1997", "2016", "2017"}},
		{"", []string{"1997", "2016", "2012", "2017"}},
		// Not a year: ignored, although its last word is one.
		{"c. 2016", []string{"1997", "2016", "2012", "2017"}},
	} {
		t.Run("year "+tt.creator, func(t *testing.T) {
			var fake fakeAniList
			srv := fake.serve(t, http.StatusOK, fixture(t, "anilist_berserk_anime.json"))
			got, err := newTestAniList(srv, "ANIME").Search(context.Background(), media.Item{Title: "Berserk", Creator: tt.creator}, media.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if years := candidateYears(got); !reflect.DeepEqual(years, tt.want) {
				t.Errorf("years = %q, want %q (titles %q)", years, tt.want, candidateTitles(got))
			}
		})
	}
}

func TestAniListCoversAndYears(t *testing.T) {
	body := `{"data":{"Page":{"media":[
		{"title":{"romaji":"No Cover"},"synonyms":[],"startDate":{"year":2020},"coverImage":{"extraLarge":null,"large":null}},
		{"title":{"romaji":"Blank Cover"},"synonyms":[],"startDate":{"year":2020},"coverImage":{"extraLarge":"","large":""}},
		{"title":{"romaji":"Season Only","english":null},"synonyms":[],"startDate":{"year":null},"seasonYear":2019,"coverImage":{"extraLarge":"https://img.example/xl.jpg","large":"https://img.example/xl.jpg"}},
		{"title":{"romaji":"Large Only","english":""},"synonyms":[],"startDate":{},"coverImage":{"large":"https://img.example/l.jpg"}}
	]}}}`
	var fake fakeAniList
	srv := fake.serve(t, http.StatusOK, []byte(body))
	got, err := newTestAniList(srv, "ANIME").Search(context.Background(), media.Item{Title: "Something"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []media.Candidate{
		{Title: "Season Only", Creator: "2019", Year: "2019", ImageURLs: []string{"https://img.example/xl.jpg"}, Source: "anilist"},
		{Title: "Large Only", ImageURLs: []string{"https://img.example/l.jpg"}, Source: "anilist"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates = %+v, want %+v", got, want)
	}
}

func TestAniListErrors(t *testing.T) {
	for _, tt := range []struct {
		name     string
		title    string
		status   int
		body     string
		notFound bool
		calls    int
		contains string
	}{
		{name: "empty title", title: "  ", status: 200, body: `{}`, notFound: true, calls: 0},
		{name: "no results", title: "zzzz", status: 200, body: `{"data":{"Page":{"media":[]}}}`, notFound: true, calls: 1},
		{
			name: "no results with a cover", title: "zzzz", status: 200, notFound: true, calls: 1,
			body: `{"data":{"Page":{"media":[{"title":{"romaji":"X"},"coverImage":{"extraLarge":null,"large":null}}]}}}`,
		},
		{
			name: "GraphQL error", title: "Berserk", status: 200, calls: 1, contains: "Too Many Requests.",
			body: `{"data":null,"errors":[{"message":"Too Many Requests.","status":429}]}`,
		},
		{name: "server error", title: "Berserk", status: 500, body: `{}`, calls: 1, contains: "HTTP 500"},
		{name: "bad JSON", title: "Berserk", status: 200, body: `<html>`, calls: 1, contains: "decode"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var fake fakeAniList
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fake.mu.Lock()
				fake.requests = append(fake.requests, aniListTestRequest{})
				fake.mu.Unlock()
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			_, err := newTestAniList(srv, "MANGA").Search(context.Background(), media.Item{Title: tt.title}, media.Options{})
			if err == nil {
				t.Fatal("err = nil, want an error")
			}
			if got := errors.Is(err, media.ErrNotFound); got != tt.notFound {
				t.Errorf("errors.Is(%v, ErrNotFound) = %v, want %v", err, got, tt.notFound)
			}
			if tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("err = %q, want it to contain %q", err, tt.contains)
			}
			if n := len(fake.calls()); n != tt.calls {
				t.Errorf("requests = %d, want %d", n, tt.calls)
			}
		})
	}
}

// A 429 is retried by the throttle with the same GraphQL body.
func TestAniListRetriesARateLimitedSearch(t *testing.T) {
	var (
		mu     sync.Mutex
		bodies []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		first := len(bodies) == 1
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if first {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"data":null,"errors":[{"message":"Too Many Requests.","status":429}]}`))
			return
		}
		w.Write(fixture(t, "anilist_berserk_manga.json"))
	}))
	t.Cleanup(srv.Close)

	client := &http.Client{Transport: &web.Throttle{BaseBackoff: time.Millisecond}}
	p := NewAniListManga(client)
	p.BaseURL = srv.URL
	got, err := p.Search(context.Background(), media.Item{Title: "Berserk"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "Berserk" {
		t.Errorf("first = %q, want Berserk", got[0].Title)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 || bodies[0] != bodies[1] || bodies[0] == "" {
		t.Errorf("request bodies = %q, want the same body twice", bodies)
	}
}
