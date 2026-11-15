// Command covers batch-downloads cover images from the command line.
//
// Usage:
//
//	covers [-type books] [-o dir] [-country es] [-csv file] [-tmdb-key key] ["Title, Author" ...]
//
// The TMDB API key comes from -tmdb-key, else the TMDB_API_KEY environment
// variable, else the settings file the GUI writes.
//
// Each positional argument is one "Title, Creator" item (split on the last
// comma). Items from -csv are added after them. Flags must come before the
// items. The exit code is 1 when any item failed and 2 on usage errors.
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

	"cover-downloader/internal/batch"
	"cover-downloader/internal/catalog"
	"cover-downloader/internal/downloader"
	"cover-downloader/internal/media"
	"cover-downloader/internal/settings"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, catalog.Default(nil)))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, reg *media.Registry) int {
	fs := flag.NewFlagSet("covers", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mediaType := fs.String("type", "books", "media type: "+typeIDs(reg))
	outDir := fs.String("o", "covers", "output directory")
	country := fs.String("country", "es", "store country code (es, us, gb, de, ...); local editions are preferred")
	csvPath := fs.String("csv", "", "CSV file with title and creator columns")
	tmdbFlag := fs.String("tmdb-key", "", "TMDB API key (default: $TMDB_API_KEY, then the saved settings)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, `usage: covers [-type books] [-o dir] [-country es] [-csv file] [-tmdb-key key] ["Title, Author" ...]`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// The movie sources will read the key; the catalog does not take it yet.
	tmdbKey := resolveTMDBKey(*tmdbFlag, os.Getenv, loadUserSettings)
	_ = tmdbKey

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
		fmt.Fprintln(stderr, "error: no items; pass \"Title, Author\" arguments or -csv")
		fs.Usage()
		return 2
	}
	if _, ok := reg.Get(*mediaType); !ok {
		fmt.Fprintf(stderr, "error: unknown media type %q (available: %s)\n", *mediaType, typeIDs(reg))
		return 2
	}

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

// loadUserSettings reads the settings file the GUI writes.
func loadUserSettings() (settings.Settings, error) {
	path, err := settings.Path()
	if err != nil {
		return settings.Settings{}, err
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

func typeIDs(reg *media.Registry) string {
	var ids []string
	for _, t := range reg.List() {
		ids = append(ids, t.ID)
	}
	return strings.Join(ids, ", ")
}
