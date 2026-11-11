package downloader

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"cover-downloader/internal/media"
)

const (
	englishISBN = "9781101658055"
	spanishISBN = "9788466342667"
)

// editionProvider returns fixed candidates and counts its searches.
type editionProvider struct {
	name  string
	cands []media.Candidate
	err   error
	calls atomic.Int32

	mu       sync.Mutex
	lastOpts media.Options
}

func (p *editionProvider) Name() string { return p.name }

func (p *editionProvider) Search(_ context.Context, _ media.Item, opts media.Options) ([]media.Candidate, error) {
	p.calls.Add(1)
	p.mu.Lock()
	p.lastOpts = opts
	p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return p.cands, nil
}

func edition(title, isbn13 string, urls ...string) media.Candidate {
	return media.Candidate{Title: title, ISBN: isbn13, ImageURLs: urls}
}

func runOne(t *testing.T, d *Downloader, country string) ItemResult {
	t.Helper()
	res, _ := run(t, d, context.Background(), Request{
		MediaType: "books",
		Items:     []media.Item{{Title: "Dune"}},
		OutputDir: t.TempDir(),
		Options:   media.Options{Country: country},
	})
	return res[0]
}

func TestPrefersLocalEditionBeyondMaxCandidates(t *testing.T) {
	srv := imageServer(t)
	p := &editionProvider{name: "apple", cands: []media.Candidate{
		edition("Dune (English)", englishISBN, srv.URL+"/other.jpg"),
		edition("Dune (Debolsillo)", spanishISBN, srv.URL+"/cover.jpg"),
	}}
	d := newDownloader(srv, p)
	d.MaxCandidates = 1

	r := runOne(t, d, "es")
	if r.Status != StatusOK || r.Matched != "Dune (Debolsillo)" || r.Source != "apple" {
		t.Fatalf("result = %+v, want the Spanish edition", r)
	}
	if !bytes.Equal(readFile(t, r.Path), jpegBody) {
		t.Error("saved bytes are not the Spanish cover")
	}
}

func TestFallsBackSilentlyToForeignEdition(t *testing.T) {
	srv := imageServer(t)
	p := &editionProvider{name: "apple", cands: []media.Candidate{
		edition("Sapiens", englishISBN, srv.URL+"/cover.jpg"),
		edition("Sapiens (no ISBN)", "", srv.URL+"/other.jpg"),
	}}
	d := newDownloader(srv, p)

	r := runOne(t, d, "es")
	if r.Status != StatusOK || r.Matched != "Sapiens" || r.Error != "" {
		t.Fatalf("result = %+v, want the foreign edition", r)
	}
}

func TestLocalEditionInLaterProviderWins(t *testing.T) {
	srv := imageServer(t)
	first := &editionProvider{name: "apple", cands: []media.Candidate{edition("Dune (English)", englishISBN, srv.URL+"/other.jpg")}}
	second := &editionProvider{name: "openlibrary", cands: []media.Candidate{edition("Dune (Spanish)", "9788401469312", srv.URL+"/cover.jpg")}}
	d := newDownloader(srv, first, second)

	r := runOne(t, d, "es")
	if r.Status != StatusOK || r.Source != "openlibrary" || r.Matched != "Dune (Spanish)" {
		t.Fatalf("result = %+v, want the Spanish edition from the second provider", r)
	}
	if n := first.calls.Load(); n != 1 {
		t.Errorf("first provider searched %d times, want 1", n)
	}
}

func TestUnknownCountryKeepsProviderOrder(t *testing.T) {
	for _, country := range []string{"zz", ""} {
		srv := imageServer(t)
		first := &editionProvider{name: "apple", cands: []media.Candidate{edition("Dune (English)", englishISBN, srv.URL+"/other.jpg")}}
		second := &editionProvider{name: "openlibrary", cands: []media.Candidate{edition("Dune (Spanish)", spanishISBN, srv.URL+"/cover.jpg")}}
		d := newDownloader(srv, first, second)

		r := runOne(t, d, country)
		if r.Status != StatusOK || r.Source != "apple" {
			t.Errorf("country %q: result = %+v, want the first provider", country, r)
		}
		if n := second.calls.Load(); n != 0 {
			t.Errorf("country %q: second provider searched %d times, want 0", country, n)
		}
	}
}

func TestSearchesEachProviderOncePerItem(t *testing.T) {
	srv := imageServer(t)
	first := &editionProvider{name: "apple", cands: []media.Candidate{edition("Dune", englishISBN, srv.URL+"/tiny.gif")}}
	second := &editionProvider{name: "amazon", err: fmt.Errorf("%w: nothing", media.ErrNotFound)}
	d := newDownloader(srv, first, second)

	res, _ := run(t, d, context.Background(), Request{
		MediaType: "books",
		Items:     []media.Item{{Title: "Dune"}, {Title: "Emma"}},
		OutputDir: t.TempDir(),
		Options:   media.Options{Country: "es"},
	})
	for _, p := range []*editionProvider{first, second} {
		if n := p.calls.Load(); n != 2 {
			t.Errorf("%s searched %d times for 2 items, want 2", p.name, n)
		}
	}
	want := "apple: no downloadable cover image (last: image too small (43 bytes), likely a placeholder); amazon: not found: nothing"
	if res[0].Status != StatusFailed || res[0].Error != want {
		t.Errorf("result = %+v\nwant error %q", res[0], want)
	}
}

func TestFailedImageIsNotFetchedTwice(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	p := &editionProvider{name: "apple", cands: []media.Candidate{edition("Dune", spanishISBN, srv.URL+"/missing.jpg")}}
	d := newDownloader(srv, p)

	r := runOne(t, d, "es")
	if r.Status != StatusFailed || r.Error != "apple: no downloadable cover image (last: HTTP 404)" {
		t.Errorf("result = %+v", r)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("image requested %d times, want 1 (pass 2 reuses the pass 1 failure)", n)
	}
}

func TestLanguageComesFromCountry(t *testing.T) {
	tests := []struct {
		opts media.Options
		want string
	}{
		{media.Options{Country: "es"}, "spa"},
		{media.Options{Country: "FR"}, "fre"},
		{media.Options{Country: "zz"}, ""},
		{media.Options{Country: "es", Language: "cat"}, "cat"},
	}
	for _, tt := range tests {
		srv := imageServer(t)
		p := &editionProvider{name: "apple", cands: []media.Candidate{edition("Dune", "", srv.URL+"/cover.jpg")}}
		d := newDownloader(srv, p)
		run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Dune"}}, OutputDir: t.TempDir(), Options: tt.opts})
		if got := p.lastOpts.Language; got != tt.want {
			t.Errorf("opts %+v: provider got Language %q, want %q", tt.opts, got, tt.want)
		}
		if got := p.lastOpts.Country; got != tt.opts.Country {
			t.Errorf("opts %+v: provider got Country %q, want it unchanged", tt.opts, got)
		}
	}
}
