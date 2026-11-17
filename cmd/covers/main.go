// Command covers batch-downloads cover images from the command line.
//
// Usage:
//
//	covers [-type books|movies|tv|albums|games|artworks|places] [-o dir] [-country es] [-csv file] [-tmdb-key key] [-igdb-id id -igdb-secret secret] ["Title, Author" | "Title, Year" | "Album, Artist" | "Title, Artist" | "Place, Country" ...]
//
// The TMDB API key (used by -type movies and -type tv) comes from -tmdb-key, else the
// TMDB_API_KEY environment variable, else the settings file the GUI writes.
// The IGDB credentials (used by -type games) come from -igdb-id and
// -igdb-secret, else IGDB_CLIENT_ID and IGDB_CLIENT_SECRET, else the
// settings file.
//
// Each positional argument is one "Title, Creator" item (split on the last
// comma): "Title, Author" for books, "Title, Year" for movies, TV series and
// games, "Album, Artist" for albums, "Title, Artist" for artworks and
// "Place, Country" (the country is optional) or "Flag of Country" for
// places. Each artwork and place image gets a "<image file>.credit.txt"
// sidecar with its credit.
// Items from -csv are added after them. Flags must come before the items. The exit code is 1 when any item failed and 2 on usage errors.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"

	"bujo-helper/internal/batch"
	"bujo-helper/internal/catalog"
	"bujo-helper/internal/downloader"
	"bujo-helper/internal/media"
	"bujo-helper/internal/settings"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, env{
		getenv:       os.Getenv,
		loadSettings: loadUserSettings,
		registry:     func(keys catalog.Keys) *media.Registry { return catalog.Default(nil, keys) },
	}))
}

// env holds run's dependencies on the outside world.
type env struct {
	getenv       func(string) string
	loadSettings func() (settings.Settings, error)
	// registry builds the media types; keys read the resolved API keys.
	registry func(keys catalog.Keys) *media.Registry
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, e env) int {
	// The keys are resolved after the flags are parsed, on first use, so
	// the settings file is read at most once and only when a search needs
	// a key.
	tmdbKey := func() string { return "" }
	igdbCredentials := func() (string, string) { return "", "" }
	reg := e.registry(catalog.Keys{
		TMDB: func() string { return tmdbKey() },
		IGDB: func() (string, string) { return igdbCredentials() },
	})

	fs := flag.NewFlagSet("covers", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mediaType := fs.String("type", "books", "media type: "+typeIDs(reg, ", ")+`; movie, TV and game items are "Title, Year", albums "Album, Artist", artworks "Title, Artist", places "Place, Country" or "Flag of Country"`)
	outDir := fs.String("o", "covers", "output directory")
	country := fs.String("country", "es", "store country code (es, us, gb, de, ...); books prefer local editions")
	csvPath := fs.String("csv", "", "CSV file with title and creator (author, year or artist) columns")
	tmdbFlag := fs.String("tmdb-key", "", "TMDB API key (default: $TMDB_API_KEY, then the saved settings)")
	igdbIDFlag := fs.String("igdb-id", "", "IGDB (Twitch) Client ID (default: $IGDB_CLIENT_ID, then the saved settings)")
	igdbSecretFlag := fs.String("igdb-secret", "", "IGDB (Twitch) Client Secret (default: $IGDB_CLIENT_SECRET, then the saved settings)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, `usage: covers [-type `+typeIDs(reg, "|")+`] [-o dir] [-country es] [-csv file] [-tmdb-key key] [-igdb-id id -igdb-secret secret] ["Title, Author" | "Title, Year" | "Album, Artist" | "Title, Artist" | "Place, Country" ...]`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var items []media.Item
	for _, arg := range fs.Args() {
		items = append(items, batch.ParseLines(arg)...)
	}
	if *csvPath != "" {
		csvItems, err := readCSV(*csvPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
		items = append(items, csvItems...)
	}
	if len(items) == 0 {
		fmt.Fprintln(stderr, "error: no items; pass \"Title, Author\" (or \"Title, Year\", \"Album, Artist\", \"Title, Artist\") arguments or -csv")
		fs.Usage()
		return 2
	}
	if _, ok := reg.Get(*mediaType); !ok {
		fmt.Fprintf(stderr, "error: unknown media type %q (available: %s)\n", *mediaType, typeIDs(reg, ", "))
		return 2
	}
	loadSettings := sync.OnceValues(e.loadSettings)
	tmdbKey = sync.OnceValue(func() string {
		return resolveTMDBKey(*tmdbFlag, e.getenv, loadSettings)
	})
	igdbCredentials = sync.OnceValues(func() (string, string) {
		return resolveIGDBCredentials(*igdbIDFlag, *igdbSecretFlag, e.getenv, loadSettings)
	})

	var mu sync.Mutex
	d := &downloader.Downloader{Registry: reg}
	sum, err := d.Run(ctx, downloader.Request{
		MediaType: *mediaType,
		Items:     items,
		OutputDir: *outDir,
		Options:   media.Options{Country: strings.ToLower(*country)},
	}, func(r downloader.ItemResult) {
		mu.Lock()
		defer mu.Unlock()
		printResult(stdout, r)
	})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}

	fmt.Fprintf(stdout, "\n%d ok, %d failed, %d cancelled\n", sum.OK, sum.Failed, sum.Cancelled)
	if sum.Failed > 0 || sum.Cancelled > 0 {
		return 1
	}
	return 0
}

// resolveTMDBKey returns the TMDB API key: flagVal, else the TMDB_API_KEY
// environment variable, else the saved settings. Blank values are skipped,
// and the settings are read only when needed. An unreadable settings file
// means no key.
func resolveTMDBKey(flagVal string, getenv func(string) string, loadSettings func() (settings.Settings, error)) string {
	if key := strings.TrimSpace(flagVal); key != "" {
		return key
	}
	if key := strings.TrimSpace(getenv("TMDB_API_KEY")); key != "" {
		return key
	}
	s, err := loadSettings()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s.TMDBAPIKey)
}

// resolveIGDBCredentials returns the IGDB (Twitch) Client ID and Client
// Secret. Each comes from its flag, else its environment variable
// (IGDB_CLIENT_ID, IGDB_CLIENT_SECRET), else the saved settings. Blank
// values are skipped, and the settings are read only when needed. An
// unreadable settings file gives no value.
func resolveIGDBCredentials(idFlag, secretFlag string, getenv func(string) string, loadSettings func() (settings.Settings, error)) (clientID, clientSecret string) {
	clientID = firstNonBlank(idFlag, getenv("IGDB_CLIENT_ID"))
	clientSecret = firstNonBlank(secretFlag, getenv("IGDB_CLIENT_SECRET"))
	if clientID != "" && clientSecret != "" {
		return clientID, clientSecret
	}
	s, err := loadSettings()
	if err != nil {
		return clientID, clientSecret
	}
	return firstNonBlank(clientID, s.IGDBClientID), firstNonBlank(clientSecret, s.IGDBClientSecret)
}

// firstNonBlank returns the first value that is not blank, trimmed.
func firstNonBlank(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// loadUserSettings reads the settings file the GUI writes. Warnings go to
// standard error.
func loadUserSettings() (settings.Settings, error) {
	return loadUserSettingsWarning(os.Stderr)
}

// loadUserSettingsWarning reads the settings file the GUI writes, and
// writes a warning to warn when the legacy settings are skipped or cannot
// be copied.
func loadUserSettingsWarning(warn io.Writer) (settings.Settings, error) {
	path, err := settings.Path()
	if err != nil {
		return settings.Settings{}, err
	}
	// Bring over the settings saved before the rename to BuJo Helper. A
	// skipped legacy file or a failed copy is not fatal: Load then reports
	// what is there.
	if legacy, err := settings.LegacyPath(); err == nil {
		if err := settings.Migrate(legacy, path); err != nil {
			fmt.Fprintf(warn, "warning: %v\n", err)
		}
	}
	return settings.Load(path)
}

func printResult(w io.Writer, r downloader.ItemResult) {
	switch r.Status {
	case downloader.StatusOK:
		fmt.Fprintf(w, "OK  %s -> %s (%s)\n", r.Item.Title, r.Path, r.Source)
	case downloader.StatusCancelled:
		fmt.Fprintf(w, "CANCELLED %s\n", r.Item.Title)
	default:
		fmt.Fprintf(w, "FAIL %s: %s\n", r.Item.Title, r.Error)
	}
}

func readCSV(path string) ([]media.Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return batch.ReadCSV(f)
}

// typeIDs lists the registered media type IDs joined by sep.
func typeIDs(reg *media.Registry, sep string) string {
	var ids []string
	for _, t := range reg.List() {
		ids = append(ids, t.ID)
	}
	return strings.Join(ids, sep)
}
