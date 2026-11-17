package gallery

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func writeImage(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}
	var buf bytes.Buffer
	var err error
	if filepath.Ext(name) == ".png" {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListFiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	writeImage(t, dir, "cover 10.jpg", 30, 45)
	writeImage(t, dir, "Cover 2.PNG", 20, 30)
	writeImage(t, dir, "apple.jpeg", 10, 15)
	writeImage(t, dir, "cover 1.jpg", 10, 15)
	writeFile(t, dir, "notes.txt", "hello")
	writeFile(t, dir, "broken.jpg", "not an image")
	if err := os.Mkdir(filepath.Join(dir, "sub.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}

	covers, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range covers {
		names = append(names, c.Name)
	}
	want := []string{"apple.jpeg", "cover 1.jpg", "Cover 2.PNG", "cover 10.jpg"}
	if len(names) != len(want) {
		t.Fatalf("names = %q, want %q", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %q, want %q", names, want)
		}
	}
	last := covers[3]
	if last.Width != 30 || last.Height != 45 {
		t.Errorf("size = %dx%d, want 30x45", last.Width, last.Height)
	}
	if !filepath.IsAbs(last.Path) || filepath.Base(last.Path) != "cover 10.jpg" {
		t.Errorf("path = %q, want an absolute path to cover 10.jpg", last.Path)
	}
}

func TestListErrors(t *testing.T) {
	if _, err := List(""); err == nil {
		t.Error("List(\"\"): want an error")
	}
	if _, err := List(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("missing folder: want an error")
	}
}

func TestListEmptyFolder(t *testing.T) {
	covers, err := List(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(covers) != 0 {
		t.Errorf("covers = %v, want none", covers)
	}
}

func TestLessNatural(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"a2", "a10", true},
		{"a10", "a2", false},
		{"B", "a", false},
		{"a", "B", true},
		{"x 9 y", "x 10 a", true},
		{"a01", "a1", false},
		{"a1", "a01", true},
		{"same", "same", false},
	}
	for _, tt := range tests {
		if got := lessNatural(tt.a, tt.b); got != tt.want {
			t.Errorf("lessNatural(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, URL(path), nil))
	return rec
}

func TestThumbsServesListedImages(t *testing.T) {
	dir := t.TempDir()
	writeImage(t, dir, "big.png", 400, 600)
	writeImage(t, dir, "small.jpg", 50, 80)
	covers, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	th := NewThumbs()
	th.Allow(covers)

	for _, c := range covers {
		rec := get(th, c.Path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d, want 200", c.Name, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("%s: Content-Type %q, want image/jpeg", c.Name, ct)
		}
		cfg, err := jpeg.DecodeConfig(rec.Body)
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		wantH := min(c.Height, ThumbHeight)
		if cfg.Height != wantH {
			t.Errorf("%s: thumbnail height %d, want %d", c.Name, cfg.Height, wantH)
		}
	}
}

func TestThumbsRejectsUnlistedPaths(t *testing.T) {
	dir := t.TempDir()
	writeImage(t, dir, "listed.jpg", 20, 30)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	unlisted := writeImage(t, sub, "unlisted.jpg", 20, 30)
	notes := writeFile(t, dir, "notes.txt", "secret")
	broken := writeFile(t, dir, "broken.jpg", "not an image")

	covers, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	th := NewThumbs()
	th.Allow(covers)

	for _, p := range []string{
		unlisted,
		notes,
		broken,
		// filepath.Join would clean these, so they are spelled out.
		sub + string(filepath.Separator) + ".." + string(filepath.Separator) + "listed.jpg",
		dir + string(filepath.Separator) + ".." + string(filepath.Separator) + "etc" + string(filepath.Separator) + "passwd.jpg",
		"",
	} {
		if rec := get(th, p); rec.Code != http.StatusNotFound {
			t.Errorf("%q: status %d, want 404", p, rec.Code)
		}
	}

	// Allow-listed but not an image: still 404.
	th.Allow([]Cover{{Name: "notes.txt", Path: notes}})
	if rec := get(th, notes); rec.Code != http.StatusNotFound {
		t.Errorf("non-image: status %d, want 404", rec.Code)
	}
}

// Artworks get a "<image>.credit.txt" sidecar from the downloader; it is
// never listed or served as an image.
func TestCreditSidecarsAreNotImages(t *testing.T) {
	dir := t.TempDir()
	img := writeImage(t, dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg", 30, 24)
	credit := writeFile(t, dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg.credit.txt", "Title: Wheat Field with Cypresses\n")

	covers, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(covers) != 1 || covers[0].Path != img {
		t.Fatalf("covers = %+v, want only the image", covers)
	}

	th := NewThumbs()
	th.Allow(append(covers, Cover{Name: filepath.Base(credit), Path: credit}))
	if rec := get(th, credit); rec.Code != http.StatusNotFound {
		t.Errorf("sidecar thumbnail: status %d, want 404", rec.Code)
	}
}

func TestThumbsAllowKeepsEarlierListings(t *testing.T) {
	// A slower, older listing can reach the UI after a newer one, so paths
	// allowed by any listing in the session keep being served.
	dir := t.TempDir()
	a := writeImage(t, dir, "a.jpg", 20, 30)
	b := writeImage(t, dir, "b.jpg", 20, 30)
	unlisted := writeImage(t, dir, "c.jpg", 20, 30)
	th := NewThumbs()
	th.Allow([]Cover{{Name: "a.jpg", Path: a}})
	th.Allow([]Cover{{Name: "b.jpg", Path: b}})
	for _, p := range []string{a, b} {
		if rec := get(th, p); rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, want 200", filepath.Base(p), rec.Code)
		}
	}
	if rec := get(th, unlisted); rec.Code != http.StatusNotFound {
		t.Errorf("unlisted path: status %d, want 404", rec.Code)
	}
}

func TestThumbsOtherRoutes(t *testing.T) {
	dir := t.TempDir()
	a := writeImage(t, dir, "a.jpg", 20, 30)
	th := NewThumbs()
	th.Allow([]Cover{{Name: "a.jpg", Path: a}})

	rec := httptest.NewRecorder()
	th.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/other?path="+url.QueryEscape(a), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("other route: status %d, want 404", rec.Code)
	}
	rec = httptest.NewRecorder()
	th.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, URL(a), nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d, want 405", rec.Code)
	}
}

func TestThumbsRefreshesChangedFiles(t *testing.T) {
	dir := t.TempDir()
	p := writeImage(t, dir, "a.png", 20, 30)
	th := NewThumbs()
	th.Allow([]Cover{{Name: "a.png", Path: p}})
	if rec := get(th, p); rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	// Replace the file with a different size and a later mtime.
	writeImage(t, dir, "a.png", 40, 60)
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	later := info.ModTime().Add(2e9)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	rec := get(th, p)
	cfg, err := jpeg.DecodeConfig(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Height != 60 {
		t.Errorf("thumbnail height %d after change, want 60", cfg.Height)
	}
}

func TestThumbsConcurrent(t *testing.T) {
	dir := t.TempDir()
	var covers []Cover
	for _, n := range []string{"a.jpg", "b.jpg", "c.png"} {
		covers = append(covers, Cover{Name: n, Path: writeImage(t, dir, n, 300, 450)})
	}
	th := NewThumbs()
	th.Allow(covers)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%6 == 0 {
				th.Allow(covers)
			}
			if rec := get(th, covers[i%len(covers)].Path); rec.Code != http.StatusOK {
				t.Errorf("status %d", rec.Code)
			}
		}(i)
	}
	wg.Wait()
}
