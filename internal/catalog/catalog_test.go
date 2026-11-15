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
	if want := []string{"books", "movies"}; !reflect.DeepEqual(ids, want) {
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
	key = "second"
	if got := tmdb.Key(); got != "second" {
		t.Errorf("TMDB key = %q, want it read live", got)
	}
}
