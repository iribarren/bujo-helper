package downloader

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"bujo-helper/internal/media"
)

const testCredit = "Title: Wheat Field with Cypresses (1889)\nArtist: Vincent van Gogh"

// credited builds a provider whose one candidate carries credit.
func credited(name, credit string, urls ...string) *fakeProvider {
	return &fakeProvider{name: name, search: func(_ context.Context, item media.Item) ([]media.Candidate, error) {
		return []media.Candidate{{Title: item.Title, Creator: item.Creator, ImageURLs: urls, Source: name, Credit: credit}}, nil
	}}
}

// dirNames returns the sorted names in dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestCreditPath(t *testing.T) {
	got := CreditPath(filepath.Join("covers", "Wheat Field with Cypresses - Vincent van Gogh.jpg"))
	want := filepath.Join("covers", "Wheat Field with Cypresses - Vincent van Gogh.jpg.credit.txt")
	if got != want {
		t.Errorf("CreditPath = %q, want %q", got, want)
	}
}

func TestCreditSidecarUsesTheFinalFileName(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, credited("met", testCredit, srv.URL+"/cover.jpg"))
	dir := t.TempDir()
	// An earlier image takes the plain name, so this one is saved as "-2".
	if err := os.WriteFile(filepath.Join(dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg"), jpegBody, 0o644); err != nil {
		t.Fatal(err)
	}
	item := media.Item{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"}

	res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{item, item}, OutputDir: dir, Workers: 1})

	for _, r := range res {
		if r.Status != StatusOK {
			t.Fatalf("item %d: %s %s", r.Index, r.Status, r.Error)
		}
		if got := string(readFile(t, CreditPath(r.Path))); got != testCredit+"\n" {
			t.Errorf("%s: credit = %q, want %q", filepath.Base(r.Path), got, testCredit+"\n")
		}
	}
	want := []string{
		"Wheat Field with Cypresses - Vincent van Gogh-2.jpg",
		"Wheat Field with Cypresses - Vincent van Gogh-2.jpg.credit.txt",
		"Wheat Field with Cypresses - Vincent van Gogh-3.jpg",
		"Wheat Field with Cypresses - Vincent van Gogh-3.jpg.credit.txt",
		"Wheat Field with Cypresses - Vincent van Gogh.jpg",
	}
	if got := dirNames(t, dir); !slices.Equal(got, want) {
		t.Errorf("files = %q, want %q (no sidecar for the earlier image)", got, want)
	}
}

func TestNoCreditSidecarWithoutCredit(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, credited("apple", "", srv.URL+"/cover.jpg"))
	dir := t.TempDir()

	res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Dune", Creator: "Frank Herbert"}}, OutputDir: dir})

	if res[0].Status != StatusOK {
		t.Fatalf("status %s: %s", res[0].Status, res[0].Error)
	}
	if got, want := dirNames(t, dir), []string{"Dune - Frank Herbert.jpg"}; !slices.Equal(got, want) {
		t.Errorf("files = %q, want %q", got, want)
	}
}

func TestCreditSidecarReplacesAStaleOne(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, credited("met", testCredit, srv.URL+"/cover.jpg"))
	dir := t.TempDir()
	// A sidecar left behind by an image deleted outside the app.
	stale := filepath.Join(dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg.credit.txt")
	if err := os.WriteFile(stale, []byte("old credit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"}}, OutputDir: dir})

	if res[0].Status != StatusOK || CreditPath(res[0].Path) != stale {
		t.Fatalf("result = %+v, want the plain name", res[0])
	}
	if got := string(readFile(t, stale)); got != testCredit+"\n" {
		t.Errorf("credit = %q, want the new one", got)
	}
}

func TestCreditSidecarFailureRemovesTheImage(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, credited("met", testCredit, srv.URL+"/cover.jpg"))
	dir := t.TempDir()
	// A directory where the sidecar should go makes writing it fail.
	if err := os.Mkdir(filepath.Join(dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg.credit.txt"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"}}, OutputDir: dir})

	if res[0].Status != StatusFailed {
		t.Fatalf("status = %s, want failed", res[0].Status)
	}
	// No uncredited image is left behind.
	if got, want := dirNames(t, dir), []string{"Wheat Field with Cypresses - Vincent van Gogh.jpg.credit.txt"}; !slices.Equal(got, want) {
		t.Errorf("files = %q, want only the blocking directory", got)
	}
}

func TestCreditSidecarFailureReportsAFailedRemoval(t *testing.T) {
	srv := imageServer(t)
	d := newDownloader(srv, credited("met", testCredit, srv.URL+"/cover.jpg"))
	// Removing a file cannot be made to fail portably, so the removal is
	// replaced; it fails as a locked file would.
	var removed []string
	d.remove = func(path string) error {
		removed = append(removed, path)
		return errors.New("file is locked")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg.credit.txt"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, _ := run(t, d, context.Background(), Request{MediaType: "books", Items: []media.Item{{Title: "Wheat Field with Cypresses", Creator: "Vincent van Gogh"}}, OutputDir: dir})

	if res[0].Status != StatusFailed {
		t.Fatalf("status = %s, want failed", res[0].Status)
	}
	image := filepath.Join(dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg")
	if !slices.Equal(removed, []string{image}) {
		t.Errorf("removed = %q, want only the uncredited image %q", removed, image)
	}
	// Both failures are reported: why the image is not kept, and that it
	// is still there.
	for _, want := range []string{"met: write credit: ", "remove uncredited image: file is locked"} {
		if !strings.Contains(res[0].Error, want) {
			t.Errorf("error = %q, want it to contain %q", res[0].Error, want)
		}
	}
}
