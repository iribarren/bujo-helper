package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"cover-downloader/internal/media"
	"cover-downloader/internal/web"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// recorder serves body for every request and remembers the last one.
type recorder struct {
	last  *http.Request
	calls atomic.Int32
}

func (rec *recorder) serve(t *testing.T, status int, contentType string, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.calls.Add(1)
		rec.last = r
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func candidateTitles(cs []media.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Title
	}
	return out
}

func TestAppleSearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "text/javascript; charset=utf-8", fixture(t, "apple_search.json"))
	p := NewApple(srv.Client())
	p.BaseURL = srv.URL + "/search"

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "Frank Herbert"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	q := rec.last.URL.Query()
	for key, want := range map[string]string{"term": "Dune Frank Herbert", "media": "ebook", "country": "es", "limit": "25"} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if ua := rec.last.Header.Get("User-Agent"); ua != web.APIUserAgent {
		t.Errorf("User-Agent = %q", ua)
	}

	wantTitles := []string{"Dune", "Dune: House Corrino", "DUNE: The Graphic Novel,  Book 2: Muad’Dib", "Frank Herbert's Dune Saga Collection: Books 1-3"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	first := got[0]
	wantURL := "https://is1-ssl.mzstatic.com/image/thumb/Publication122/v4/85/70/2f/85702f34-2982-e2ee-d883-b5ffc3ae0897/9781101658055.d.jpg/10000x10000bb.jpg"
	if !reflect.DeepEqual(first.ImageURLs, []string{wantURL}) {
		t.Errorf("ImageURLs = %q", first.ImageURLs)
	}
	if first.Creator != "Frank Herbert" || first.Year != "1975" || first.Source != "apple" {
		t.Errorf("first = %+v", first)
	}
}

func TestAppleDefaultsCountryAndTermWithoutCreator(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", []byte(`{"resultCount":0,"results":[]}`))
	p := NewApple(srv.Client())
	p.BaseURL = srv.URL

	_, err := p.Search(context.Background(), media.Item{Title: " Dune "}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if q := rec.last.URL.Query(); q.Get("country") != "us" || q.Get("term") != "Dune" {
		t.Errorf("query = %v", q)
	}
}

func TestAppleHTTPError(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 500, "text/plain", []byte("boom"))
	p := NewApple(srv.Client())
	p.BaseURL = srv.URL
	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want HTTP 500", err)
	}
}

func TestOpenLibrarySearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", fixture(t, "openlibrary_search.json"))
	p := NewOpenLibrary(srv.Client())
	p.BaseURL = srv.URL + "/search.json"
	p.CoverBaseURL = "https://covers.example"

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "Frank Herbert"}, media.Options{Country: "es"})
	if err != nil {
		t.Fatal(err)
	}

	q := rec.last.URL.Query()
	for key, want := range map[string]string{"title": "Dune", "author": "Frank Herbert", "limit": "25"} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if !strings.HasPrefix(q.Get("fields"), "title,author_name,cover_i") {
		t.Errorf("fields = %q", q.Get("fields"))
	}
	if ua := rec.last.Header.Get("User-Agent"); ua != web.APIUserAgent {
		t.Errorf("User-Agent = %q", ua)
	}

	wantTitles := []string{"Dune", "Dune Messiah", "Children of Dune", "Frank Herbert"}
	if !reflect.DeepEqual(candidateTitles(got), wantTitles) {
		t.Errorf("titles = %q, want %q", candidateTitles(got), wantTitles)
	}
	want := media.Candidate{
		Title:     "Dune",
		Creator:   "Frank Herbert, Brian Herbert",
		Year:      "1965",
		ImageURLs: []string{"https://covers.example/b/id/11481354-L.jpg?default=false"},
		Source:    "openlibrary",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first = %+v, want %+v", got[0], want)
	}
}

func TestOpenLibraryOmitsEmptyAuthor(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "application/json", []byte(`{"docs":[{"title":"Dune"}]}`))
	p := NewOpenLibrary(srv.Client())
	p.BaseURL = srv.URL

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound (no doc has a cover)", err)
	}
	if _, ok := rec.last.URL.Query()["author"]; ok {
		t.Error("author parameter should be omitted when the creator is empty")
	}
}

func TestAmazonSearch(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "text/html;charset=UTF-8", fixture(t, "amazon_search.html"))
	p := NewAmazon(srv.Client())
	p.BaseURL = srv.URL
	p.ImageBaseURL = "https://img.example"

	got, err := p.Search(context.Background(), media.Item{Title: "Dune", Creator: "Frank Herbert"}, media.Options{Country: "us"})
	if err != nil {
		t.Fatal(err)
	}

	if rec.last.URL.Path != "/s" {
		t.Errorf("path = %q", rec.last.URL.Path)
	}
	q := rec.last.URL.Query()
	if q.Get("k") != "Dune Frank Herbert" || q.Get("i") != "stripbooks" {
		t.Errorf("query = %v", q)
	}
	if !strings.Contains(rec.last.URL.RawQuery, "k=Dune+Frank+Herbert") {
		t.Errorf("raw query = %q, want plus-encoded spaces", rec.last.URL.RawQuery)
	}
	if ua := rec.last.Header.Get("User-Agent"); !strings.Contains(ua, "Chrome") {
		t.Errorf("User-Agent = %q, want a browser UA", ua)
	}
	if rec.last.Header.Get("Accept-Language") == "" || rec.last.Header.Get("Accept") == "" {
		t.Error("browser Accept headers missing")
	}

	if len(got) != 3 {
		t.Fatalf("got %d candidates, want 3 (the ASIN-less card is skipped): %+v", len(got), got)
	}
	// Audiobook-first card: print/Kindle ASIN images first, then the search image.
	audio := got[0]
	wantAudio := []string{
		"https://img.example/images/P/0441013597.01._SCRMZZZZZZ_.jpg",
		"https://img.example/images/P/B00B7NPRY8.01._SCRMZZZZZZ_.jpg",
		"https://m.media-amazon.com/images/I/71-1WBgjGoL.jpg",
	}
	if !reflect.DeepEqual(audio.ImageURLs, wantAudio) {
		t.Errorf("audio ImageURLs = %q, want %q", audio.ImageURLs, wantAudio)
	}
	if audio.Title != "Dune: Book One in the Dune Chronicles" || audio.Creator != "Frank Herbert, Scott Brick, et al." || audio.Source != "amazon" {
		t.Errorf("audio candidate = %+v", audio)
	}

	// Print-first card: only the full-size search image.
	printCard := got[1]
	if !reflect.DeepEqual(printCard.ImageURLs, []string{"https://m.media-amazon.com/images/I/71EJeWqNPLL.jpg"}) {
		t.Errorf("print ImageURLs = %q", printCard.ImageURLs)
	}
	if printCard.Creator != "Frank Herbert" || printCard.Year != "2020" {
		t.Errorf("print candidate = %+v", printCard)
	}
}

func TestAmazonBotCheck(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "text/html", fixture(t, "amazon_botcheck.html"))
	p := NewAmazon(srv.Client())
	p.BaseURL = srv.URL
	p.RetryDelay = 0

	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if !errors.Is(err, ErrBotCheck) {
		t.Fatalf("err = %v, want ErrBotCheck", err)
	}
	if !strings.Contains(err.Error(), "blocked by bot check") {
		t.Errorf("err text = %q", err)
	}
	if n := rec.calls.Load(); n != 2 {
		t.Errorf("requests = %d, want 2 (one retry)", n)
	}
}

func TestAmazonHTTPErrorIsBotCheck(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 503, "text/html", []byte("<html>s-main-slot</html>"))
	p := NewAmazon(srv.Client())
	p.BaseURL = srv.URL
	p.RetryDelay = 0
	_, err := p.Search(context.Background(), media.Item{Title: "Dune"}, media.Options{})
	if !errors.Is(err, ErrBotCheck) || !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v", err)
	}
}

func TestAmazonNoResults(t *testing.T) {
	var rec recorder
	srv := rec.serve(t, 200, "text/html", []byte(`<html><body><div class="s-main-slot"></div></body></html>`))
	p := NewAmazon(srv.Client())
	p.BaseURL = srv.URL
	_, err := p.Search(context.Background(), media.Item{Title: "zzqq"}, media.Options{})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestAmazonDomain(t *testing.T) {
	tests := map[string]string{
		"us": "com", "": "com", "gb": "co.uk", "uk": "co.uk", "jp": "co.jp",
		"br": "com.br", "mx": "com.mx", "au": "com.au", "es": "es", "DE": "de",
	}
	for country, want := range tests {
		if got := AmazonDomain(country); got != want {
			t.Errorf("AmazonDomain(%q) = %q, want %q", country, got, want)
		}
	}
}

func TestFullSizeImage(t *testing.T) {
	tests := map[string]string{
		"https://m.media-amazon.com/images/I/71-1WBgjGoL._AC_UY218_.jpg": "https://m.media-amazon.com/images/I/71-1WBgjGoL.jpg",
		"https://m.media-amazon.com/images/I/81abc._AC_UY327_QL65_.png":  "https://m.media-amazon.com/images/I/81abc.png",
		"https://m.media-amazon.com/images/I/81abc.jpg":                  "https://m.media-amazon.com/images/I/81abc.jpg",
		"https://m.media-amazon.com/images/I/a._X_/b._SX1_.jpg":          "https://m.media-amazon.com/images/I/a._X_/b.jpg",
	}
	for in, want := range tests {
		if got := fullSizeImage(in); got != want {
			t.Errorf("fullSizeImage(%q) = %q, want %q", in, got, want)
		}
	}
}
