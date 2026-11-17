// Package catalog wires the built-in media types to their providers.
// Adding a media type means adding its providers and one Register call here.
package catalog

import (
	"net/http"

	"bujo-helper/internal/media"
	"bujo-helper/internal/providers"
	"bujo-helper/internal/web"
)

// Keys supplies the API keys some providers need. Each func is called on
// every search, so a key changed at run time applies at once. A nil func
// means no key; providers that need one are skipped.
type Keys struct {
	TMDB func() string
	// IGDB returns the Twitch Client ID and Client Secret.
	IGDB func() (clientID, clientSecret string)
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
	// Album art is square. MusicBrainz + Cover Art Archive first (the
	// user's choice), then the iTunes album search.
	mustRegister(reg, media.MediaType{
		ID:           "albums",
		Label:        "Albums",
		CreatorLabel: "Artist",
		DateLabel:    "Listened on",
		Providers: []media.Provider{
			providers.NewMusicBrainz(client),
			providers.NewAppleAlbums(client),
		},
		Samples: []media.Item{
			{Title: "Abbey Road", Creator: "The Beatles"},
			{Title: "Rumours", Creator: "Fleetwood Mac"},
			{Title: "Kind of Blue", Creator: "Miles Davis"},
			{Title: "Buena Vista Social Club", Creator: "Buena Vista Social Club"},
			{Title: "Random Access Memories", Creator: "Daft Punk"},
		},
	})
	// Game box art: IGDB when the user has Twitch credentials (the user's
	// choice), then the keyless Steam store, which only knows PC games.
	mustRegister(reg, media.MediaType{
		ID:           "games",
		Label:        "Games",
		CreatorLabel: "Year",
		DateLabel:    "Played on",
		Providers: []media.Provider{
			providers.NewIGDB(client, keys.IGDB),
			providers.NewSteam(client),
		},
		Samples: []media.Item{
			{Title: "Hollow Knight", Creator: "2017"},
			{Title: "Stardew Valley", Creator: "2016"},
			{Title: "The Witcher 3: Wild Hunt", Creator: "2015"},
			{Title: "Celeste", Creator: "2018"},
			{Title: "The Legend of Zelda: Breath of the Wild", Creator: "2017"},
		},
	})
	// Public-domain artworks from museum open-access APIs: the Met first,
	// then Cleveland (the user's choice). Images come with a credit
	// sidecar. The Art Institute of Chicago is not used: its image server
	// rejects every non-browser request.
	mustRegister(reg, media.MediaType{
		ID:           "artworks",
		Label:        "Artworks",
		CreatorLabel: "Artist",
		DateLabel:    "Seen on",
		Providers: []media.Provider{
			providers.NewMet(client),
			providers.NewCleveland(client),
		},
		Samples: []media.Item{
			{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"},
			{Title: "The Harvesters", Creator: "Pieter Bruegel the Elder"},
			{Title: "Washington Crossing the Delaware", Creator: "Emanuel Leutze"},
			{Title: "Twilight in the Wilderness", Creator: "Frederic Edwin Church"},
			{Title: "The Burning of the Houses of Lords and Commons", Creator: "Joseph Mallord William Turner"},
		},
	})
	// Landmark and city photos from Wikimedia Commons, found through
	// Wikidata. Every Commons file keeps its own licence, so each image
	// gets a credit sidecar (author, licence, file page). A title such as
	// "Flag of Japan" or "Japan flag" downloads the country's flag instead.
	mustRegister(reg, media.MediaType{
		ID:           "places",
		Label:        "Places",
		CreatorLabel: "Country",
		DateLabel:    "Visited on",
		Providers: []media.Provider{
			providers.NewWikimedia(client),
		},
		Samples: []media.Item{
			{Title: "Eiffel Tower", Creator: "France"},
			{Title: "Machu Picchu", Creator: "Peru"},
			{Title: "Sydney Opera House", Creator: "Australia"},
			{Title: "Colosseum", Creator: "Italy"},
			{Title: "Flag of Japan"},
		},
	})
	return reg
}

func mustRegister(reg *media.Registry, t media.MediaType) {
	if err := reg.Register(t); err != nil {
		panic(err)
	}
}
