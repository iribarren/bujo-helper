// Package catalog wires the built-in media types to their providers.
// Adding a media type means adding its providers and one Register call here.
package catalog

import (
	"net/http"

	"cover-downloader/internal/media"
	"cover-downloader/internal/providers"
	"cover-downloader/internal/web"
)

// Keys supplies the API keys some providers need. Each func is called on
// every search, so a key changed at run time applies at once. A nil func
// means no key; providers that need one are skipped.
type Keys struct {
	TMDB func() string
}

// Default returns the registry of built-in media types. A nil client gets
// a default one with a request timeout.
func Default(client *http.Client, keys Keys) *media.Registry {
	if client == nil {
		client = web.NewClient()
	}
	reg := media.NewRegistry()
	mustRegister(reg, media.MediaType{
		ID:           "books",
		Label:        "Books",
		CreatorLabel: "Author",
		DateLabel:    "Read on",
		Providers: []media.Provider{
			providers.NewApple(client),
			providers.NewOpenLibrary(client),
			providers.NewAmazon(client),
		},
		LocalEditions: true,
		Samples: []media.Item{
			{Title: "Dune", Creator: "Frank Herbert"},
			{Title: "Pride and Prejudice", Creator: "Jane Austen"},
			{Title: "Guns, Germs, and Steel", Creator: "Jared Diamond"},
			{Title: "One Hundred Years of Solitude", Creator: "Gabriel García Márquez"},
			{Title: "The Hobbit", Creator: "J.R.R. Tolkien"},
		},
	})
	mustRegister(reg, media.MediaType{
		ID:           "movies",
		Label:        "Movies",
		CreatorLabel: "Year",
		DateLabel:    "Watched on",
		Providers: []media.Provider{
			providers.NewTMDB(client, keys.TMDB),
			providers.NewIMDb(client),
			providers.NewWikidataITunes(client),
		},
		Samples: []media.Item{
			{Title: "Dune", Creator: "2021"},
			{Title: "The Godfather", Creator: "1972"},
			{Title: "Spirited Away", Creator: "2001"},
			{Title: "Crouching Tiger, Hidden Dragon", Creator: "2000"},
			{Title: "Amélie", Creator: "2001"},
		},
	})
	// No Wikidata → iTunes fallback for TV: Wikidata only links Apple TV
	// movie IDs.
	mustRegister(reg, media.MediaType{
		ID:           "tv",
		Label:        "TV series",
		CreatorLabel: "Year",
		DateLabel:    "Watched on",
		Providers: []media.Provider{
			providers.NewTMDBTV(client, keys.TMDB),
			providers.NewIMDbTV(client),
		},
		Samples: []media.Item{
			{Title: "Breaking Bad", Creator: "2008"},
			{Title: "The Office", Creator: "2005"},
			{Title: "Stranger Things", Creator: "2016"},
			{Title: "Money Heist", Creator: "2017"},
			{Title: "Dark", Creator: "2017"},
		},
	})
	return reg
}

func mustRegister(reg *media.Registry, t media.MediaType) {
	if err := reg.Register(t); err != nil {
		panic(err)
	}
}
