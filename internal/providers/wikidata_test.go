package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

// wikidataServer fakes the Wikidata API (search and entities) and the
// iTunes lookup. lookup returns the lookup body for a country.
type wikidataServer struct {
	search, entities []byte
	lookup           func(country string) []byte

	mu       sync.Mutex
	requests []*http.Request
}

func (s *wikidataServer) start(t *testing.T) (*httptest.Server, *WikidataITunes) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/w/api.php" && q.Get("list") == "search":
			w.Write(s.search)
		case r.URL.Path == "/w/api.php" && q.Get("action") == "wbgetentities":
			w.Write(s.entities)
		case r.URL.Path == "/lookup":
			w.Write(s.lookup(q.Get("country")))
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	p := NewWikidataITunes(srv.Client())
	p.WikidataURL = srv.URL + "/w/api.php"
	p.ITunesURL = srv.URL + "/lookup"
	return srv, p
}

func (s *wikidataServer) recorded() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*http.Request(nil), s.requests...)
}

func newDuneServer(t *testing.T) *wikidataServer {
	lookup := fixture(t, "itunes_lookup.json")
	return &wikidataServer{
		search:   fixture(t, "wikidata_search.json"),
		entities: fixture(t, "wikidata_entities.json"),
		lookup:   func(string) []byte { return lookup },
	}
}

func TestWikidataITunesName(t *testing.T) {
	p := NewWikidataITunes(nil)
	if p.Name() != "wikidata-itunes" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.WikidataURL != "https://www.wikidata.org/w/api.php" || p.ITunesURL != "https://itunes.apple.com/lookup" {
		t.Errorf("default URLs = %q, %q", p.WikidataURL, p.ITunesURL)
	}
}

func TestWikidataITunesRanksByYear(t *testing.T) {
	s := newDuneServer(t)
	_, p := s.start(t)

	for itemYear, want := range map[string][]string{
		"2021": {"2021", "1984"},
		"1984": {"1984", "2021"},
	} {
		got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: itemYear}, media.Options{Country: "es"})
		if err != nil {
			t.Fatal(err)
		}
		if years := candidateYears(got); !reflect.DeepEqual(years, want) {
			t.Errorf("Dune, %s: years = %q, want %q", itemYear, years, want)
		}
	}
}

func TestWikidataITunesCandidate(t *testing.T) {
	s := newDuneServer(t)
	_, p := s.start(t)

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}
	const art = "https://is1-ssl.mzstatic.com/image/thumb/Video116/v4/15/53/b9/1553b93b-25b7-cbed-eff5-34d8efe8e09a/pr_source.lsr/"
	want := media.Candidate{
		Title:   "Dune",
		Creator: "2021",
		Year:    "2021",
		ImageURLs: []string{
			art + "9999x9999bb.jpg",
			art + "600x600bb.jpg",
		},
		Source: "wikidata-itunes",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
	// The 1984 entity's year comes from Wikidata, not the iTunes release date.
	if got[1].Year != "1984" || !strings.HasSuffix(got[1].ImageURLs[0], "/Dune_ENG_1400x2100.jpg/9999x9999bb.jpg") {
		t.Errorf("second = %+v", got[1])
	}
}

func TestWikidataITunesRequests(t *testing.T) {
	s := newDuneServer(t)
	_, p := s.start(t)

	if _, err := p.Search(context.Background(), media.Item{Title: " Dune ", Creator: "2021"}, media.Options{Country: "ES"}); err != nil {
		t.Fatal(err)
	}
	reqs := s.recorded()
	if len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3 (search, entities, lookup)", len(reqs))
	}
	checks := []map[string]string{
		{
			"action": "query", "list": "search", "srsearch": "Dune haswbstatement:P6398",
			"srnamespace": "0", "srlimit": "10", "format": "json",
		},
		{
			"action": "wbgetentities", "ids": "Q190192|Q114819|Q60834962|Q109300883",
			"props": "claims|labels", "languages": "en", "languagefallback": "1", "format": "json",
		},
		{
			// Only the ids of entities with P6398 are looked up, in search order.
			"id": "283648369,826266153,312749847,1585877256", "country": "es", "entity": "movie",
		},
	}
	for i, want := range checks {
		q := reqs[i].URL.Query()
		for key, v := range want {
			if q.Get(key) != v {
				t.Errorf("request %d: %s = %q, want %q", i, key, q.Get(key), v)
			}
		}
		if ua := reqs[i].Header.Get("User-Agent"); ua != web.APIUserAgent {
			t.Errorf("request %d: User-Agent = %q", i, ua)
		}
	}
}

func TestWikidataITunesFiltersEntitiesWithoutITunesID(t *testing.T) {
	s := newDuneServer(t)
	// Only the novel and the film series: neither has P6398.
	s.entities = []byte(`{"entities":{
		"Q190192":{"id":"Q190192","labels":{"en":{"value":"Dune"}},"claims":{}},
		"Q109300883":{"id":"Q109300883","labels":{"en":{"value":"Dune"}},"claims":{}}
	}}`)
	_, p := s.start(t)

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	for _, r := range s.recorded() {
		if r.URL.Path == "/lookup" {
			t.Errorf("unexpected iTunes lookup %q", r.URL.RawQuery)
		}
	}
}

func TestWikidataITunesRetriesUSWhenLookupEmpty(t *testing.T) {
	s := newDuneServer(t)
	lookup := fixture(t, "itunes_lookup.json")
	s.lookup = func(cc string) []byte {
		if cc == "us" {
			return lookup
		}
		return []byte(`{"resultCount":0,"results":[]}`)
	}
	_, p := s.start(t)

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}
	var countries []string
	for _, r := range s.recorded() {
		if r.URL.Path == "/lookup" {
			countries = append(countries, r.URL.Query().Get("country"))
		}
	}
	if !reflect.DeepEqual(countries, []string{"es", "us"}) {
		t.Errorf("lookup countries = %q, want es then us", countries)
	}
	if len(got) != 2 || got[0].Year != "2021" {
		t.Errorf("got %+v", got)
	}
}

func TestWikidataITunesEmptyUSLookupIsNotFound(t *testing.T) {
	s := newDuneServer(t)
	s.lookup = func(string) []byte { return []byte(`{"resultCount":0,"results":[]}`) }
	_, p := s.start(t)

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{Country: "us"})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	var lookups int
	for _, r := range s.recorded() {
		if r.URL.Path == "/lookup" {
			lookups++
		}
	}
	if lookups != 1 {
		t.Errorf("lookups = %d, want 1 (no retry for us)", lookups)
	}
}

func TestWikidataITunesSkipsResultsWithoutArtwork(t *testing.T) {
	s := newDuneServer(t)
	s.lookup = func(string) []byte {
		return []byte(`{"resultCount":2,"results":[
			{"trackId":1585877256,"trackName":"Dune","releaseDate":"2021-09-15T07:00:00Z"},
			{"trackId":826266153,"trackName":"Dune","releaseDate":"1984-12-14T08:00:00Z","artworkUrl100":"https://img.example/a/100x100bb.jpg"}
		]}`)
	}
	_, p := s.start(t)

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "2021"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Year != "1984" {
		t.Errorf("got %+v, want only the 1984 film", got)
	}
}

func TestWikidataITunesFallsBackToITunesData(t *testing.T) {
	s := newDuneServer(t)
	// No label and no P577: the title and year come from iTunes.
	s.search = []byte(`{"query":{"search":[{"ns":0,"title":"Q1"}]}}`)
	s.entities = []byte(`{"entities":{"Q1":{"id":"Q1","claims":{"P6398":[
		{"mainsnak":{"snaktype":"value","datavalue":{"value":"42","type":"string"}}}
	]}}}}`)
	s.lookup = func(string) []byte {
		return []byte(`{"results":[{"trackId":42,"trackName":"Some Film","releaseDate":"1999-03-31T08:00:00Z","artworkUrl100":"https://img.example/a/100x100bb.jpg"}]}`)
	}
	_, p := s.start(t)

	got, err := p.Search(context.Background(), media.Item{Title: "Some Film"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Some Film" || got[0].Year != "1999" || got[0].Creator != "1999" {
		t.Errorf("got %+v", got)
	}
}

func TestWikidataITunesEmptySearchIsNotFound(t *testing.T) {
	s := newDuneServer(t)
	s.search = []byte(`{"batchcomplete":"","query":{"searchinfo":{"totalhits":0},"search":[]}}`)
	_, p := s.start(t)

	_, err := p.Search(context.Background(), media.Item{Title: "Zzzzzz"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if n := len(s.recorded()); n != 1 {
		t.Errorf("requests = %d, want 1 (search only)", n)
	}
}

func TestWikidataITunesBlankTitleIsNotFoundWithoutHTTP(t *testing.T) {
	s := newDuneServer(t)
	_, p := s.start(t)

	_, err := p.Search(context.Background(), media.Item{Title: "  "}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if n := len(s.recorded()); n != 0 {
		t.Errorf("requests = %d, want 0", n)
	}
}

func TestWikidataITunesAPIError(t *testing.T) {
	s := newDuneServer(t)
	s.search = []byte(`{"error":{"code":"maxlag","info":"Waiting for all: 5 seconds lagged."}}`)
	_, p := s.start(t)

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if err == nil || !strings.Contains(err.Error(), "maxlag") {
		t.Errorf("err = %v, want the Wikidata API error", err)
	}
	if errors.Is(err, media.ErrNotFound) {
		t.Error("an API error must not wrap ErrNotFound")
	}
}

func TestWikidataYear(t *testing.T) {
	tests := map[string]string{
		"+2021-09-03T00:00:00Z": "2021",
		"+1965-00-00T00:00:00Z": "1965",
		"-0044-03-15T00:00:00Z": "",
		"":                      "",
		"+12":                   "",
	}
	for in, want := range tests {
		if got := wikidataYear(in); got != want {
			t.Errorf("wikidataYear(%q) = %q, want %q", in, got, want)
		}
	}
}
