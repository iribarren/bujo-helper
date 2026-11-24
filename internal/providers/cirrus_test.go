package providers

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"bujo-helper/internal/media"
)

// noSearchHits is a MediaWiki search response without hits.
var noSearchHits = []byte(`{"batchcomplete":"","query":{"search":[]}}`)

// wikidataSearches returns the srsearch of every Wikidata search request.
func wikidataSearches(rs *routedServer) []string {
	var out []string
	for _, r := range rs.requests() {
		if r.URL.Path == wikidataTestPath && r.URL.Query().Get("list") == "search" {
			out = append(out, r.URL.Query().Get("srsearch"))
		}
	}
	return out
}

// emptyWikidataSearch answers every Wikidata search without hits.
func emptyWikidataSearch(r *http.Request) (int, []byte) {
	if r.URL.Path == wikidataTestPath && r.URL.Query().Get("list") == "search" {
		return http.StatusOK, noSearchHits
	}
	return http.StatusNotFound, []byte(`{}`)
}

// cirrusCase is a user title and the search it must send; want nil means
// no search at all (nothing left to search for).
type cirrusCase struct {
	title string
	want  []string
}

func TestCirrusSearchQueries(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		search func(t *testing.T, title string) (*routedServer, error)
		cases  []cirrusCase
	}{
		{
			name:   "wikidata-itunes",
			filter: " haswbstatement:P6398",
			search: func(t *testing.T, title string) (*routedServer, error) {
				rs := &routedServer{}
				srv := rs.serve(t, emptyWikidataSearch)
				p := NewWikidataITunes(srv.Client())
				p.WikidataURL = srv.URL + wikidataTestPath
				p.ITunesURL = srv.URL + "/lookup"
				_, err := p.Search(context.Background(), media.Item{Title: title}, media.Options{})
				return rs, err
			},
		},
		{
			name:   "wikimedia place",
			filter: " haswbstatement:P18",
			search: func(t *testing.T, title string) (*routedServer, error) {
				p, rs := newWikimediaTest(t, emptyWikidataSearch)
				_, err := p.Search(context.Background(), media.Item{Title: title}, media.Options{})
				return rs, err
			},
		},
		{
			name:   "wikimedia flag",
			filter: " haswbstatement:P41",
			search: func(t *testing.T, title string) (*routedServer, error) {
				p, rs := newWikimediaTest(t, emptyWikidataSearch)
				_, err := p.Search(context.Background(), media.Item{Title: "Flag of " + title}, media.Options{})
				return rs, err
			},
		},
		{
			name:   "taxon",
			filter: " haswbstatement:P31=Q16521",
			search: func(t *testing.T, title string) (*routedServer, error) {
				rs := &routedServer{}
				srv := rs.serve(t, emptyWikidataSearch)
				tr := NewTaxonResolver(srv.Client())
				tr.WikidataURL = srv.URL + wikidataTestPath
				_, err := tr.Resolve(context.Background(), title)
				return rs, err
			},
		},
	}
	titles := []struct{ title, terms string }{
		{"-Saw", "Saw"},
		{`"Heat"`, "Heat"},
		{"intitle:Up", "intitle Up"},
		{"Alien: Romulus", "Alien Romulus"},
		{"Who's Afraid of Virginia Woolf?", "Who's Afraid of Virginia Woolf"},
		{"Fast & Furious", "Fast Furious"},
		{"NOT Another Teen Movie", "not Another Teen Movie"},
		{"Léon", "Léon"},
		// Nothing is left to search for: our filter alone would match
		// every entity that has the property.
		{`"-*~?"`, ""},
	}
	for _, tt := range tests {
		for _, ti := range titles {
			t.Run(tt.name+"/"+ti.title, func(t *testing.T) {
				rs, err := tt.search(t, ti.title)
				if !errors.Is(err, media.ErrNotFound) {
					t.Fatalf("err = %v, want ErrNotFound", err)
				}
				var want []string
				if ti.terms != "" {
					want = []string{ti.terms + tt.filter}
				}
				if got := wikidataSearches(rs); !reflect.DeepEqual(got, want) {
					t.Errorf("srsearch = %q, want %q", got, want)
				}
				if n := len(rs.requests()); n != len(want) {
					t.Errorf("requests = %d, want %d", n, len(want))
				}
			})
		}
	}
}

func TestNatureSearchesKeepPhrasesClosed(t *testing.T) {
	// A quote or a backslash (which would escape the closing quote) in a
	// Wikidata name must not break out of the phrase.
	got := natureSearches(Taxon{ID: "Q1", Name: `Rosa "x\`, Category: `Rosa\ "y"`})
	want := []string{
		"haswbstatement:P180=Q1 haswbstatement:P6216=Q19652 filetype:bitmap",
		`incategory:"Rosa y - botanical illustrations"`,
		`"Rosa x" plate filetype:bitmap`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("natureSearches = %q, want %q", got, want)
	}
	for _, q := range got {
		if strings.Contains(q, `\`) {
			t.Errorf("search %q keeps a backslash", q)
		}
	}
}
