package catalog

import (
	"net/http"
	"reflect"
	"testing"

	"bujo-helper/internal/media"
	"bujo-helper/internal/providers"
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
	if want := []string{"books", "movies", "tv", "albums", "games", "anime", "manga", "artworks", "places", "nature", "products"}; !reflect.DeepEqual(ids, want) {
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

	albums, _ := reg.Get("albums")
	if albums.Label != "Albums" || albums.CreatorLabel != "Artist" || albums.DateLabel != "Listened on" || albums.LocalEditions {
		t.Errorf("albums = %+v", albums)
	}
	if want := []string{"musicbrainz", "apple"}; !reflect.DeepEqual(providerNames(albums), want) {
		t.Errorf("albums providers = %q, want %q", providerNames(albums), want)
	}
	if apple, ok := albums.Providers[1].(*providers.Apple); !ok || apple.Media != "music" || apple.Entity != "album" {
		t.Errorf("second albums provider = %#v, want Apple in album mode", albums.Providers[1])
	}
	if apple := books.Providers[0].(*providers.Apple); apple.Media != "ebook" || apple.Entity != "" {
		t.Errorf("books Apple = %#v, want ebook with no entity", apple)
	}
	if len(albums.Samples) != 5 {
		t.Errorf("albums samples = %d, want 5", len(albums.Samples))
	}
	for _, s := range albums.Samples {
		if s.Title == "" || s.Creator == "" {
			t.Errorf("albums sample %+v, want an album and an artist", s)
		}
	}

	games, _ := reg.Get("games")
	if games.Label != "Games" || games.CreatorLabel != "Year" || games.DateLabel != "Played on" || games.LocalEditions {
		t.Errorf("games = %+v", games)
	}
	if want := []string{"igdb", "steam"}; !reflect.DeepEqual(providerNames(games), want) {
		t.Errorf("games providers = %q, want %q", providerNames(games), want)
	}
	wantGames := []media.Item{
		{Title: "Hollow Knight", Creator: "2017"},
		{Title: "Stardew Valley", Creator: "2016"},
		{Title: "The Witcher 3: Wild Hunt", Creator: "2015"},
		{Title: "Celeste", Creator: "2018"},
		{Title: "The Legend of Zelda: Breath of the Wild", Creator: "2017"},
	}
	if !reflect.DeepEqual(games.Samples, wantGames) {
		t.Errorf("games samples = %+v, want %+v", games.Samples, wantGames)
	}

	for _, tt := range []struct {
		id, label, date, aniListType string
	}{
		{"anime", "Anime", "Watched on", "ANIME"},
		{"manga", "Manga", "Read on", "MANGA"},
	} {
		mt, _ := reg.Get(tt.id)
		if mt.Label != tt.label || mt.CreatorLabel != "Year" || mt.DateLabel != tt.date || mt.LocalEditions {
			t.Errorf("%s = %+v", tt.id, mt)
		}
		if want := []string{"anilist"}; !reflect.DeepEqual(providerNames(mt), want) {
			t.Errorf("%s providers = %q, want %q", tt.id, providerNames(mt), want)
			continue
		}
		if p, ok := mt.Providers[0].(*providers.AniList); !ok || p.Type != tt.aniListType {
			t.Errorf("%s provider = %+v, want AniList type %s", tt.id, mt.Providers[0], tt.aniListType)
		}
		if len(mt.Samples) != 5 {
			t.Errorf("%s samples = %d, want 5", tt.id, len(mt.Samples))
		}
		for _, s := range mt.Samples {
			if s.Title == "" || len(s.Creator) != 4 {
				t.Errorf("%s sample %+v, want a title and a year", tt.id, s)
			}
		}
	}

	artworks, _ := reg.Get("artworks")
	if artworks.Label != "Artworks" || artworks.CreatorLabel != "Artist" || artworks.DateLabel != "Seen on" || artworks.LocalEditions {
		t.Errorf("artworks = %+v", artworks)
	}
	if want := []string{"met", "cleveland"}; !reflect.DeepEqual(providerNames(artworks), want) {
		t.Errorf("artworks providers = %q, want %q", providerNames(artworks), want)
	}
	wantArtworks := []media.Item{
		{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"},
		{Title: "The Harvesters", Creator: "Pieter Bruegel the Elder"},
		{Title: "Washington Crossing the Delaware", Creator: "Emanuel Leutze"},
		{Title: "Twilight in the Wilderness", Creator: "Frederic Edwin Church"},
		{Title: "The Burning of the Houses of Lords and Commons", Creator: "Joseph Mallord William Turner"},
	}
	if !reflect.DeepEqual(artworks.Samples, wantArtworks) {
		t.Errorf("artworks samples = %+v, want %+v", artworks.Samples, wantArtworks)
	}

	places, _ := reg.Get("places")
	if places.Label != "Places" || places.CreatorLabel != "Country" || places.DateLabel != "Visited on" || places.LocalEditions {
		t.Errorf("places = %+v", places)
	}
	if want := []string{"wikimedia"}; !reflect.DeepEqual(providerNames(places), want) {
		t.Errorf("places providers = %q, want %q", providerNames(places), want)
	}
	wantPlaces := []media.Item{
		{Title: "Eiffel Tower", Creator: "France"},
		{Title: "Machu Picchu", Creator: "Peru"},
		{Title: "Sydney Opera House", Creator: "Australia"},
		{Title: "Colosseum", Creator: "Italy"},
		// Flag mode: the country column is not used.
		{Title: "Flag of Japan"},
	}
	if !reflect.DeepEqual(places.Samples, wantPlaces) {
		t.Errorf("places samples = %+v, want %+v", places.Samples, wantPlaces)
	}

	nature, _ := reg.Get("nature")
	// One column: the name, common or scientific.
	if nature.Label != "Nature" || nature.CreatorLabel != "" || nature.DateLabel != "Seen on" || nature.LocalEditions {
		t.Errorf("nature = %+v", nature)
	}
	// Commons first (no key), then BHL when a key is set.
	if want := []string{"commons-nature", "bhl"}; !reflect.DeepEqual(providerNames(nature), want) {
		t.Errorf("nature providers = %q, want %q", providerNames(nature), want)
	}
	wantNature := []media.Item{
		{Title: "Robin"},
		{Title: "Monstera deliciosa"},
		{Title: "Barn owl"},
		{Title: "Common sunflower"},
		{Title: "Monarch butterfly"},
	}
	if !reflect.DeepEqual(nature.Samples, wantNature) {
		t.Errorf("nature samples = %+v, want %+v", nature.Samples, wantNature)
	}

	products, _ := reg.Get("products")
	if products.Label != "Products" || products.CreatorLabel != "Brand" || products.DateLabel != "Tried on" || products.LocalEditions {
		t.Errorf("products = %+v", products)
	}
	if want := []string{"openfoodfacts"}; !reflect.DeepEqual(providerNames(products), want) {
		t.Errorf("products providers = %q, want %q", providerNames(products), want)
	}
	wantProducts := []media.Item{
		{Title: "Nutella", Creator: "Ferrero"},
		{Title: "Earl Grey", Creator: "Twinings"},
		{Title: "Coca-Cola Zero", Creator: "Coca-Cola"},
		// Barcode mode: the brand column is not used.
		{Title: "3017620422003"},
	}
	if !reflect.DeepEqual(products.Samples, wantProducts) {
		t.Errorf("products samples = %+v, want %+v", products.Samples, wantProducts)
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

func TestDefaultPassesTheIGDBCredentials(t *testing.T) {
	id, secret := "id-1", "secret-1"
	reg := Default(http.DefaultClient, Keys{IGDB: func() (string, string) { return id, secret }})
	games, _ := reg.Get("games")
	igdb, ok := games.Providers[0].(*providers.IGDB)
	if !ok {
		t.Fatalf("first games provider is %T, want *providers.IGDB", games.Providers[0])
	}
	id, secret = "id-2", "secret-2"
	if gotID, gotSecret := igdb.Credentials(); gotID != "id-2" || gotSecret != "secret-2" {
		t.Errorf("IGDB credentials = %q, %q, want them read live", gotID, gotSecret)
	}
}

func TestDefaultNatureProvidersShareTheResolverAndReadTheBHLKey(t *testing.T) {
	key := "first"
	reg := Default(http.DefaultClient, Keys{BHL: func() string { return key }})
	nature, _ := reg.Get("nature")
	commons, ok := nature.Providers[0].(*providers.CommonsNature)
	if !ok {
		t.Fatalf("first nature provider is %T, want *providers.CommonsNature", nature.Providers[0])
	}
	bhl, ok := nature.Providers[1].(*providers.BHL)
	if !ok {
		t.Fatalf("second nature provider is %T, want *providers.BHL", nature.Providers[1])
	}
	if commons.Taxa == nil || commons.Taxa != bhl.Taxa {
		t.Error("the nature providers do not share one taxon resolver")
	}
	key = "second"
	if got := bhl.Key(); got != "second" {
		t.Errorf("BHL key = %q, want it read live", got)
	}
}
