package providers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/web"
)

// taxonFixtures are the Wikidata bodies a fake server returns for a taxon
// lookup, recorded live (testdata/nature) and trimmed: the taxon search,
// the hits' labels and aliases, and the chosen hit's claims.
type taxonFixtures struct {
	search, labels, claims []byte
}

func loadTaxonFixtures(t *testing.T, name string) taxonFixtures {
	t.Helper()
	return taxonFixtures{
		search: fixture(t, "nature/"+name+"_wikidata_search.json"),
		labels: fixture(t, "nature/"+name+"_wikidata_labels.json"),
		claims: fixture(t, "nature/"+name+"_wikidata_claims.json"),
	}
}

// route serves the Wikidata requests of a taxon lookup, or reports false.
func (f taxonFixtures) route(r *http.Request) (int, []byte, bool) {
	if r.URL.Path != wikidataTestPath {
		return 0, nil, false
	}
	q := r.URL.Query()
	switch {
	case q.Get("list") == "search":
		return http.StatusOK, f.search, true
	case q.Get("action") == "wbgetentities" && q.Get("props") == "labels|aliases":
		return http.StatusOK, f.labels, true
	case q.Get("action") == "wbgetentities" && q.Get("props") == "claims":
		return http.StatusOK, f.claims, true
	}
	return 0, nil, false
}

func newTaxonTest(t *testing.T, f taxonFixtures) (*TaxonResolver, *routedServer) {
	t.Helper()
	rs := &routedServer{}
	srv := rs.serve(t, func(r *http.Request) (int, []byte) {
		if status, body, ok := f.route(r); ok {
			return status, body
		}
		return http.StatusNotFound, []byte(`{}`)
	})
	tr := NewTaxonResolver(srv.Client())
	tr.WikidataURL = srv.URL + wikidataTestPath
	return tr, rs
}

func TestTaxonResolverDefaults(t *testing.T) {
	if tr := NewTaxonResolver(nil); tr.Client == nil || tr.WikidataURL != "https://www.wikidata.org/w/api.php" {
		t.Errorf("NewTaxonResolver(nil) = %+v", tr)
	}
}

func TestTaxonResolve(t *testing.T) {
	robin := Taxon{ID: "Q25334", Label: "European robin", Name: "Erithacus rubecula", Category: "Erithacus rubecula"}
	monstera := Taxon{ID: "Q161077", Label: "Monstera deliciosa", Name: "Monstera deliciosa", Category: "Monstera deliciosa"}
	tests := []struct {
		name, fixtures, query string
		want                  Taxon
	}{
		// "robin" is an alias of both the European and the American robin:
		// the tie keeps the search order.
		{"common name alias", "robin", "Robin", robin},
		{"scientific name alias", "robin", "erithacus  RUBECULA", robin},
		{"label", "monstera", "Monstera deliciosa", monstera},
		{"common name of a plant", "monstera", "Swiss cheese plant", monstera},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, rs := newTaxonTest(t, loadTaxonFixtures(t, tt.fixtures))
			got, err := tr.Resolve(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %+v, want %+v", tt.query, got, tt.want)
			}
			// Only the chosen hit's claims are fetched.
			reqs := rs.requests()
			if len(reqs) != 3 {
				t.Fatalf("requests = %d, want 3 (search, labels, claims)", len(reqs))
			}
			if ids := reqs[2].URL.Query().Get("ids"); ids != tt.want.ID {
				t.Errorf("claims ids = %q, want %q", ids, tt.want.ID)
			}
		})
	}
}

func TestTaxonResolveRequests(t *testing.T) {
	tr, rs := newTaxonTest(t, loadTaxonFixtures(t, "robin"))
	if _, err := tr.Resolve(context.Background(), " Robin "); err != nil {
		t.Fatal(err)
	}
	reqs := rs.requests()
	if len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3", len(reqs))
	}
	checks := []map[string]string{
		{
			"action": "query", "list": "search", "srsearch": "Robin haswbstatement:P31=Q16521",
			"srnamespace": "0", "srlimit": "10", "srprop": "", "format": "json",
		},
		{
			"action": "wbgetentities", "ids": "Q25334|Q460967|Q266761|Q218448|Q1061885|Q855800|Q607027|Q1051396|Q578756|Q27075670",
			"props": "labels|aliases", "languages": "en", "languagefallback": "1", "format": "json",
		},
		{"action": "wbgetentities", "ids": "Q25334", "props": "claims", "format": "json"},
	}
	for i, want := range checks {
		q := reqs[i].URL.Query()
		for key, v := range want {
			if q.Get(key) != v {
				t.Errorf("request %d: %s = %q, want %q", i, key, q.Get(key), v)
			}
		}
	}
	for i, r := range reqs {
		if ua := r.Header.Get("User-Agent"); ua != web.ContactUserAgent {
			t.Errorf("request %d: User-Agent = %q, want the contact UA", i, ua)
		}
	}
}

func TestTaxonResolveCaches(t *testing.T) {
	tr, rs := newTaxonTest(t, loadTaxonFixtures(t, "robin"))
	var wg sync.WaitGroup
	results := make([]Taxon, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = tr.Resolve(context.Background(), "Robin")
		}(i)
	}
	wg.Wait()
	for i, got := range results {
		if got.ID != "Q25334" {
			t.Errorf("result %d = %+v", i, got)
		}
	}
	n := len(rs.requests())
	// The same name in another case and spacing is a cache hit.
	if got, err := tr.Resolve(context.Background(), "  ROBIN "); err != nil || got.ID != "Q25334" {
		t.Errorf("cached Resolve = %+v, %v", got, err)
	}
	if after := len(rs.requests()); after != n {
		t.Errorf("requests after a cached lookup = %d, want %d", after, n)
	}
}

func TestTaxonResolveNotFound(t *testing.T) {
	tests := []struct {
		name, query string
		modify      func(*taxonFixtures)
		wantErr     string
		wantReqs    int
	}{
		{"empty name", "  ", nil, "empty name", 0},
		{"no taxon hits", "Nothingness", func(f *taxonFixtures) {
			f.search = []byte(`{"batchcomplete":"","query":{"search":[]}}`)
		}, `no Wikidata taxon matches "Nothingness"`, 1},
		// The search matched something else (a description, say): no hit
		// is named like the query.
		{"no plausible hit", "Zebra", nil, `no Wikidata taxon named "Zebra"`, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadTaxonFixtures(t, "robin")
			if tt.modify != nil {
				tt.modify(&f)
			}
			tr, rs := newTaxonTest(t, f)
			got, err := tr.Resolve(context.Background(), tt.query)
			if !errors.Is(err, media.ErrNotFound) {
				t.Fatalf("Resolve = %+v, %v; want ErrNotFound", got, err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
			if n := len(rs.requests()); n != tt.wantReqs {
				t.Errorf("requests = %d, want %d", n, tt.wantReqs)
			}
		})
	}
}

func TestTaxonResolveFallsBackToLabel(t *testing.T) {
	// A taxon item without P225 or P373: the label is the taxon name and
	// there is no Commons category.
	f := loadTaxonFixtures(t, "monstera")
	f.claims = []byte(`{"entities":{"Q161077":{"id":"Q161077","claims":{"P225":[{"mainsnak":{"datavalue":{"value":"Old name"}},"rank":"deprecated"}]}}}}`)
	tr, _ := newTaxonTest(t, f)
	got, err := tr.Resolve(context.Background(), "Monstera deliciosa")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Taxon{ID: "Q161077", Label: "Monstera deliciosa", Name: "Monstera deliciosa"}); got != want {
		t.Errorf("Resolve = %+v, want %+v", got, want)
	}
}

func TestTaxonResolveErrors(t *testing.T) {
	apiError := []byte(`{"error":{"code":"maxlag","info":"Waiting for a database server"}}`)
	tests := []struct {
		name   string
		status int
		modify func(*taxonFixtures)
		want   string
	}{
		{"search API error", http.StatusOK, func(f *taxonFixtures) { f.search = apiError }, "maxlag"},
		{"labels API error", http.StatusOK, func(f *taxonFixtures) { f.labels = apiError }, "maxlag"},
		{"claims API error", http.StatusOK, func(f *taxonFixtures) { f.claims = apiError }, "maxlag"},
		{"HTTP error", http.StatusServiceUnavailable, nil, "HTTP 503"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := loadTaxonFixtures(t, "robin")
			if tt.modify != nil {
				tt.modify(&f)
			}
			rs := &routedServer{}
			srv := rs.serve(t, func(r *http.Request) (int, []byte) {
				if tt.status != http.StatusOK {
					return tt.status, []byte(`{}`)
				}
				status, body, _ := f.route(r)
				return status, body
			})
			tr := NewTaxonResolver(srv.Client())
			tr.WikidataURL = srv.URL + wikidataTestPath
			_, err := tr.Resolve(context.Background(), "Robin")
			if err == nil || errors.Is(err, media.ErrNotFound) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want an error with %q, not ErrNotFound", err, tt.want)
			}
			// Failures are not cached: the next lookup asks again.
			n := len(rs.requests())
			_, _ = tr.Resolve(context.Background(), "Robin")
			if len(rs.requests()) == n {
				t.Error("a failed lookup was cached")
			}
		})
	}
}
