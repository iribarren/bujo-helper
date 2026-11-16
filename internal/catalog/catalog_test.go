package catalog

import (
	"net/http"
	"reflect"
	"testing"

	"cover-downloader/internal/media"
	"cover-downloader/internal/providers"
)

func providerNames(t media.MediaType) []string {
	var names []string
	for _, p := range t.Providers {
		names = append(names, p.Name())
	}
	return names
}

func TestDefault(t *testing.T) {
	reg := Default(http.DefaultClient, Keys{})

	var ids []string
	for _, mt := range reg.List() {
		ids = append(ids, mt.ID)
	}
	if want := []string{"books", "movies", "tv"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("media types = %q, want %q", ids, want)
	}

	books, _ := reg.Get("books")
	if books.Label != "Books" || books.CreatorLabel != "Author" || !books.LocalEditions {
		t.Errorf("books = %+v", books)
	}
	if want := []string{"apple", "openlibrary", "amazon"}; !reflect.DeepEqual(providerNames(books), want) {
		t.Errorf("books providers = %q, want %q", providerNames(books), want)
	}

	movies, _ := reg.Get("movies")
	if movies.Label != "Movies" || movies.CreatorLabel != "Year" || movies.LocalEditions {
		t.Errorf("movies = %+v", movies)
	}
	if want := []string{"tmdb", "imdb", "wikidata-itunes"}; !reflect.DeepEqual(providerNames(movies), want) {
		t.Errorf("movies providers = %q, want %q", providerNames(movies), want)
	}

	if books.DateLabel != "Read on" || movies.DateLabel != "Watched on" {
		t.Errorf("date labels = %q, %q, want \"Read on\", \"Watched on\"", books.DateLabel, movies.DateLabel)
	}

	tv, _ := reg.Get("tv")
	if tv.Label != "TV series" || tv.CreatorLabel != "Year" || tv.DateLabel != "Watched on" || tv.LocalEditions {
		t.Errorf("tv = %+v", tv)
	}
	if want := []string{"tmdb", "imdb"}; !reflect.DeepEqual(providerNames(tv), want) {
		t.Errorf("tv providers = %q, want %q", providerNames(tv), want)
	}
	if tmdb, ok := tv.Providers[0].(*providers.TMDB); !ok || !tmdb.TV {
		t.Errorf("first tv provider = %#v, want TMDB in TV mode", tv.Providers[0])
	}
	if imdb, ok := tv.Providers[1].(*providers.IMDb); !ok || !imdb.TV {
		t.Errorf("second tv provider = %#v, want IMDb in TV mode", tv.Providers[1])
	}
	if tmdb := movies.Providers[0].(*providers.TMDB); tmdb.TV {
		t.Error("movies TMDB is in TV mode")
	}
	if imdb := movies.Providers[1].(*providers.IMDb); imdb.TV {
		t.Error("movies IMDb is in TV mode")
	}
	if len(tv.Samples) == 0 {
		t.Error("tv has no samples")
	}
	for _, s := range tv.Samples {
		if s.Title == "" || len(s.Creator) != 4 {
			t.Errorf("tv sample %+v, want a title and a year", s)
		}
	}

	if Default(nil, Keys{}) == nil {
		t.Error("Default(nil) should fall back to a default client")
	}
}

func TestDefaultPassesTheTMDBKey(t *testing.T) {
	key := "first"
	reg := Default(http.DefaultClient, Keys{TMDB: func() string { return key }})
	movies, _ := reg.Get("movies")
	tmdb, ok := movies.Providers[0].(*providers.TMDB)
	if !ok {
		t.Fatalf("first movie provider is %T, want *providers.TMDB", movies.Providers[0])
	}
	tv, _ := reg.Get("tv")
	tvTMDB, ok := tv.Providers[0].(*providers.TMDB)
	if !ok {
		t.Fatalf("first tv provider is %T, want *providers.TMDB", tv.Providers[0])
	}
	key = "second"
	if got := tmdb.Key(); got != "second" {
		t.Errorf("TMDB key = %q, want it read live", got)
	}
	if got := tvTMDB.Key(); got != "second" {
		t.Errorf("TV TMDB key = %q, want it read live", got)
	}
}
