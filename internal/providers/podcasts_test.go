package providers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"bujo-helper/internal/media"
)

func newTestApplePodcasts(rec *recorder, t *testing.T, body []byte) *Apple {
	t.Helper()
	srv := rec.serve(t, 200, "text/javascript; charset=utf-8", body)
	p := NewApplePodcasts(srv.Client())
	p.BaseURL = srv.URL + "/search"
	return p
}

func TestApplePodcastSearch(t *testing.T) {
	var rec recorder
	p := newTestApplePodcasts(&rec, t, fixture(t, "apple_podcast_search.json"))

	if p.Name() != "apple" {
		t.Errorf("Name = %q, want apple", p.Name())
	}
	got, err := p.Search(context.Background(), media.Item{Title: "The Daily", Creator: "The New York Times"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	q := rec.last.URL.Query()
	for key, want := range map[string]string{
		"term": "The Daily The New York Times", "media": "podcast", "country": "es", "limit": "25",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if q.Has("entity") {
		t.Errorf("query entity = %q, want none", q.Get("entity"))
	}

	// The exact show and host come first, then the same show by another
	// publisher, then the publisher's other shows.
	wantTitles := []string{"The Daily", "The Daily", "The Headlines", "The Field"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:   "The Daily",
		Creator: "The New York Times",
		Year:    "2026",
		ImageURLs: []string{
			"https://is1-ssl.mzstatic.com/image/thumb/Podcasts221/v4/ab/64/66/ab6466a9-9a7d-e20e-7a3d-bc5be37d29ce/mza_15084852813176276273.jpg/10000x10000bb.jpg",
		},
		Source: "apple",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
	if got[1].Creator != "The New York Times One" {
		t.Errorf("second creator = %q, want The New York Times One", got[1].Creator)
	}
	// Podcast artwork is not named after an ISBN.
	for _, c := range got {
		if c.ISBN != "" {
			t.Errorf("%q ISBN = %q, want none", c.Title, c.ISBN)
		}
	}
}

func TestApplePodcastTitleIsTheCollectionName(t *testing.T) {
	var rec recorder
	body := []byte(`{"resultCount":1,"results":[{"kind":"podcast","trackName":"Feed title","collectionName":"Radiolab","artistName":"WNYC Studios","artworkUrl100":"https://img.example/a/100x100bb.jpg"}]}`)
	p := newTestApplePodcasts(&rec, t, body)

	got, err := p.Search(context.Background(), media.Item{Title: "Radiolab"}, media.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Radiolab" || got[0].Creator != "WNYC Studios" {
		t.Errorf("candidates = %+v, want Radiolab by WNYC Studios", got)
	}
}

func TestApplePodcastWithoutArtworkIsNotFound(t *testing.T) {
	var rec recorder
	body := []byte(`{"resultCount":1,"results":[{"kind":"podcast","collectionName":"Serial","artistName":"Serial Productions"}]}`)
	p := newTestApplePodcasts(&rec, t, body)

	_, err := p.Search(context.Background(), media.Item{Title: "Serial"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if q := rec.last.URL.Query(); q.Get("country") != "us" {
		t.Errorf("country = %q, want us", q.Get("country"))
	}
}

func TestAppleLabels(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    *Apple
		want string
	}{
		{"books", NewApple(nil), "Apple Books"},
		{"albums", NewAppleAlbums(nil), "Apple Music"},
		{"podcasts", NewApplePodcasts(nil), "Apple Podcasts"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Label(); got != tt.want {
				t.Errorf("Label = %q, want %q", got, tt.want)
			}
			if got := tt.p.Name(); got != "apple" {
				t.Errorf("Name = %q, want apple", got)
			}
		})
	}
}
