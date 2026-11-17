package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"bujo-helper/internal/batch"
	"bujo-helper/internal/downloader"
	"bujo-helper/internal/i18n"
	"bujo-helper/internal/media"
	"bujo-helper/internal/providers"
	"bujo-helper/internal/settings"
	"bujo-helper/internal/sheet"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var jpegBody = append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0xAB}, 4096)...)

// fakeProvider answers searches with a function of the item.
type fakeProvider struct {
	search func(ctx context.Context, item media.Item) ([]media.Candidate, error)
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Search(ctx context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	return f.search(ctx, item)
}

// recorder captures emitted events.
type recorder struct {
	mu    sync.Mutex
	items []ItemEvent
	done  chan DoneEvent
}

func (r *recorder) emit(event string, data any) {
	switch event {
	case EventItem:
		r.mu.Lock()
		r.items = append(r.items, data.(ItemEvent))
		r.mu.Unlock()
	case EventDone:
		r.done <- data.(DoneEvent)
	}
}

func (r *recorder) waitDone(t *testing.T) DoneEvent {
	t.Helper()
	select {
	case d := <-r.done:
		return d
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for job:done")
		return DoneEvent{}
	}
}

// testApp returns an App whose only media type, "books", uses p, plus the
// recorder that captures its events.
func testApp(t *testing.T, p media.Provider) (*App, *recorder) {
	t.Helper()
	reg := media.NewRegistry()
	if err := reg.Register(media.MediaType{ID: "books", Label: "Books", CreatorLabel: "Author", DateLabel: "Read on", Providers: []media.Provider{p}}); err != nil {
		t.Fatal(err)
	}
	a := newApp(reg, http.DefaultClient)
	rec := &recorder{done: make(chan DoneEvent, 1)}
	a.emit = rec.emit
	return a, rec
}

// blockingProvider blocks every search until its context is cancelled.
// started is closed when the first search begins.
func blockingProvider() (*fakeProvider, chan struct{}) {
	started := make(chan struct{})
	var once sync.Once
	return &fakeProvider{search: func(ctx context.Context, _ media.Item) ([]media.Candidate, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return nil, ctx.Err()
	}}, started
}

func TestStartEmitsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cover.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegBody)
	}))
	t.Cleanup(srv.Close)
	a, rec := testApp(t, &fakeProvider{search: func(_ context.Context, item media.Item) ([]media.Candidate, error) {
		url := srv.URL + "/cover.jpg"
		if item.Title == "Missing" {
			url = srv.URL + "/missing.jpg"
		}
		return []media.Candidate{{Title: item.Title, Creator: item.Creator, ImageURLs: []string{url}}}, nil
	}})
	dir := filepath.Join(t.TempDir(), "out")

	views, err := a.Start(StartRequest{
		MediaType: "books",
		Text:      "# comment\nDune, Frank Herbert\n\nMissing\n",
		OutputDir: dir,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := []ItemView{{Index: 0, Title: "Dune", Creator: "Frank Herbert"}, {Index: 1, Title: "Missing"}}
	if len(views) != len(want) || views[0] != want[0] || views[1] != want[1] {
		t.Fatalf("views = %+v, want %+v", views, want)
	}

	done := rec.waitDone(t)
	if done != (DoneEvent{OK: 1, Failed: 1}) {
		t.Errorf("done = %+v", done)
	}
	rec.mu.Lock()
	items := append([]ItemEvent(nil), rec.items...)
	rec.items = nil
	rec.mu.Unlock()
	if len(items) != 2 {
		t.Fatalf("got %d item events: %+v", len(items), items)
	}
	for _, ev := range items {
		switch ev.Index {
		case 0:
			if ev.Status != "ok" || ev.Source != "fake" || ev.Path != filepath.Join(dir, "Dune - Frank Herbert.jpg") {
				t.Errorf("item 0 = %+v", ev)
			}
			if _, err := os.Stat(ev.Path); err != nil {
				t.Errorf("saved file: %v", err)
			}
		case 1:
			if ev.Status != "failed" || !strings.Contains(ev.Error, "HTTP 404") {
				t.Errorf("item 1 = %+v", ev)
			}
		}
	}

	// The running state is cleared, so a new job can start.
	if _, err := a.Start(StartRequest{MediaType: "books", Text: "Dune", OutputDir: dir}); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	rec.waitDone(t)
}

func TestStartWhileRunning(t *testing.T) {
	p, started := blockingProvider()
	a, rec := testApp(t, p)
	req := StartRequest{MediaType: "books", Text: "Dune", OutputDir: t.TempDir()}
	if _, err := a.Start(req); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started
	if _, err := a.Start(req); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("second Start err = %v, want already running", err)
	}
	a.Cancel()
	rec.waitDone(t)
}

func TestStartRejectsBadRequests(t *testing.T) {
	a, _ := testApp(t, &fakeProvider{})
	dir := t.TempDir()
	tests := []struct {
		name string
		req  StartRequest
	}{
		{"empty text", StartRequest{MediaType: "books", Text: "", OutputDir: dir}},
		{"only blanks and comments", StartRequest{MediaType: "books", Text: "\n  \n# Dune, Frank Herbert\n", OutputDir: dir}},
		{"unknown media type", StartRequest{MediaType: "games", Text: "Doom", OutputDir: dir}},
		{"no output folder", StartRequest{MediaType: "books", Text: "Dune", OutputDir: "  "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if views, err := a.Start(tt.req); err == nil {
				t.Errorf("Start = %+v, want an error", views)
			}
		})
	}
}

func TestCancel(t *testing.T) {
	p, started := blockingProvider()
	a, rec := testApp(t, p)
	a.Cancel() // no job: a no-op

	if _, err := a.Start(StartRequest{MediaType: "books", Text: "A\nB\nC", OutputDir: t.TempDir()}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started
	a.Cancel()
	if done := rec.waitDone(t); done != (DoneEvent{Cancelled: 3}) {
		t.Errorf("done = %+v, want 3 cancelled", done)
	}
	rec.mu.Lock()
	n := len(rec.items)
	rec.mu.Unlock()
	if n != 3 {
		t.Errorf("got %d item events, want 3", n)
	}
}

func TestMediaTypes(t *testing.T) {
	a, _ := testApp(t, &fakeProvider{})
	got := a.MediaTypes()
	if len(got) != 1 || got[0] != (MediaTypeInfo{ID: "books", Label: "Books", CreatorLabel: "Author", DateLabel: "Read on"}) {
		t.Errorf("MediaTypes = %+v", got)
	}
}

func TestDefaultOutputDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	if got, want := NewApp().DefaultOutputDir(), filepath.Join(home, "Pictures", "Covers"); got != want {
		t.Errorf("DefaultOutputDir = %q, want %q", got, want)
	}
}

func TestImportCSVFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "books.csv")
	if err := os.WriteFile(good, []byte("title;author\n\"Guns, Germs, and Steel\";Jared Diamond\nDune;Frank Herbert\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := importCSVFile(good, i18n.EN)
	if err != nil {
		t.Fatalf("importCSVFile: %v", err)
	}
	if want := "Guns, Germs, and Steel, Jared Diamond\nDune, Frank Herbert\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	empty := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(empty, []byte("title,author\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{empty, filepath.Join(dir, "missing.csv")} {
		if _, err := importCSVFile(path, i18n.EN); err == nil || !strings.HasPrefix(err.Error(), filepath.Base(path)+": ") {
			t.Errorf("importCSVFile(%s) err = %v, want it prefixed with the file name", filepath.Base(path), err)
		}
	}
}

// optsProvider records the options of its searches and finds nothing.
type optsProvider struct {
	mu   sync.Mutex
	opts []media.Options
}

func (p *optsProvider) Name() string { return "opts" }

func (p *optsProvider) Search(_ context.Context, _ media.Item, opts media.Options) ([]media.Candidate, error) {
	p.mu.Lock()
	p.opts = append(p.opts, opts)
	p.mu.Unlock()
	return nil, media.ErrNotFound
}

func TestStartCountry(t *testing.T) {
	tests := map[string]string{"": "es", "  ": "es", " GB ": "gb", "us": "us"}
	for in, want := range tests {
		p := &optsProvider{}
		a, rec := testApp(t, p)
		if _, err := a.Start(StartRequest{MediaType: "books", Text: "Dune", OutputDir: t.TempDir(), Country: in}); err != nil {
			t.Fatalf("Start: %v", err)
		}
		rec.waitDone(t)
		p.mu.Lock()
		if len(p.opts) != 1 || p.opts[0].Country != want {
			t.Errorf("country %q: provider options = %+v, want Country %q", in, p.opts, want)
		}
		p.mu.Unlock()
	}
}

// writeCover saves a w x h grey JPEG to dir/name.
func writeCover(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListCovers(t *testing.T) {
	dir := t.TempDir()
	writeCover(t, dir, "b 10.jpg", 66, 100)
	writeCover(t, dir, "b 2.jpg", 40, 60)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(media.NewRegistry(), http.DefaultClient)

	views, err := a.ListCovers(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].Name != "b 2.jpg" || views[1].Name != "b 10.jpg" {
		t.Fatalf("views = %+v, want b 2.jpg then b 10.jpg", views)
	}
	v := views[1]
	if v.Width != 66 || v.Height != 100 || v.Path != filepath.Join(dir, "b 10.jpg") {
		t.Errorf("view = %+v", v)
	}
	if !strings.HasPrefix(v.ThumbURL, "/thumbs?path=") {
		t.Errorf("thumbUrl = %q", v.ThumbURL)
	}

	// The listed covers are served by the thumbnail handler; others are not.
	rec := httptest.NewRecorder()
	a.thumbs.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, v.ThumbURL, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("thumbnail: status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	other := writeCover(t, t.TempDir(), "other.jpg", 10, 10)
	rec = httptest.NewRecorder()
	a.thumbs.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/thumbs?path="+other, nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unlisted thumbnail: status %d, want 404", rec.Code)
	}

	if _, err := a.ListCovers(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing folder: want an error")
	}
}

// The print sheet lists covers through ListCovers, so a credit sidecar
// never reaches it as an image.
func TestListCoversIgnoresCreditSidecars(t *testing.T) {
	dir := t.TempDir()
	img := writeCover(t, dir, "Twilight in the Wilderness - Frederic Edwin Church.jpg", 100, 62)
	if err := os.WriteFile(downloader.CreditPath(img), []byte("Title: Twilight in the Wilderness\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(media.NewRegistry(), http.DefaultClient)

	views, err := a.ListCovers(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Path != img {
		t.Fatalf("views = %+v, want only the image", views)
	}
}

// The page count includes the credits page when the box is checked and a
// selected image has a credit sidecar.
func TestPrintPagesCountsCreditsPages(t *testing.T) {
	dir := t.TempDir()
	book := writeCover(t, dir, "Dune - Frank Herbert.jpg", 66, 100)
	tower := writeCover(t, dir, "Eiffel Tower - France.jpg", 100, 150)
	if err := os.WriteFile(downloader.CreditPath(tower), []byte("Author: Benh LIEU SONG\nLicence: public domain\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		paths []string
		opts  sheet.Options
		pages int
	}{
		{"credited image, box checked", []string{book, tower}, sheet.Options{Credits: true}, 2},
		{"credited image, box unchecked", []string{book, tower}, sheet.Options{}, 1},
		{"no sidecar, box checked", []string{book}, sheet.Options{Credits: true}, 1},
		{"credited image on review cards", []string{tower}, sheet.Options{Format: "cards", Credits: true}, 2},
	}
	a := newApp(media.NewRegistry(), http.DefaultClient)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.PrintPages(tt.paths, tt.opts)
			if err != nil || got.Pages != tt.pages {
				t.Errorf("PrintPages = %+v, %v; want %d pages", got, err, tt.pages)
			}
		})
	}
}

func TestExportTo(t *testing.T) {
	src := t.TempDir()
	var files []string
	for i := 0; i < 26; i++ {
		files = append(files, writeCover(t, src, fmt.Sprintf("c%02d.jpg", i), 66, 100))
	}
	outDir := t.TempDir()
	out := filepath.Join(outDir, "covers.pdf")
	// An existing file is replaced.
	if err := os.WriteFile(out, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := exportTo(out, files, sheet.Options{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("output starts with %q, want %%PDF", data[:min(8, len(data))])
	}
	if n := bytes.Count(data, []byte("/Type /Page\n")); n != 2 {
		t.Errorf("PDF has %d pages, want 2", n)
	}
	entries, _ := os.ReadDir(outDir)
	if len(entries) != 1 {
		t.Errorf("output folder has %d entries, want only covers.pdf", len(entries))
	}
}

func TestExportToFailureLeavesNothing(t *testing.T) {
	src := t.TempDir()
	good := writeCover(t, src, "good.jpg", 66, 100)
	bad := filepath.Join(src, "bad.jpg")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	out := filepath.Join(outDir, "covers.pdf")

	err := exportTo(out, []string{good, bad}, sheet.Options{})
	if err == nil || !strings.Contains(err.Error(), "bad.jpg") {
		t.Fatalf("err = %v, want an error naming bad.jpg", err)
	}
	if entries, _ := os.ReadDir(outDir); len(entries) != 0 {
		t.Errorf("output folder has %d entries after a failure, want none", len(entries))
	}
}

func TestExportPDFNeedsCovers(t *testing.T) {
	a := newApp(media.NewRegistry(), http.DefaultClient)
	if _, err := a.ExportPDF(nil, "", sheet.Options{}); err == nil {
		t.Error("no covers: want an error")
	}
}

func TestExportPDFRejectsUnknownSettingsBeforeAsking(t *testing.T) {
	tests := []struct {
		name string
		opts sheet.Options
		want string // the error must name the bad value
	}{
		{"unknown size", sheet.Options{Size: "huge"}, `"huge"`},
		{"unknown paper", sheet.Options{Paper: "a3"}, `"a3"`},
		{"unknown paper with a known size", sheet.Options{Size: "mini", Paper: "legal"}, `"legal"`},
		{"unknown format", sheet.Options{Format: "posters"}, `"posters"`},
		{"unknown paper for cards", sheet.Options{Format: "cards", Paper: "a3"}, `"a3"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubDialogs(t, nil, nil) // the save dialog must not open
			cover := writeCover(t, t.TempDir(), "c.jpg", 66, 100)
			a := newApp(media.NewRegistry(), http.DefaultClient)
			_, err := a.ExportPDF([]string{cover}, "", tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want an error naming %s", err, tt.want)
			}
		})
	}
}

func TestExportPDFDefaultFileName(t *testing.T) {
	tests := []struct {
		name string
		opts sheet.Options
		want string
	}{
		{"default cover sheet", sheet.Options{}, "covers_A4_Standard.pdf"},
		{"cover sheet", sheet.Options{Format: "covers", Size: "mini", Paper: "letter"}, "covers_Letter_Mini.pdf"},
		{"review cards", sheet.Options{Format: "cards", Paper: "a5"}, "cards_A5.pdf"},
		{"review cards ignore the size", sheet.Options{Format: "cards", Size: "huge", Paper: "a4"}, "cards_A4.pdf"},
		{"default spines", sheet.Options{Format: "spines"}, "spines_A4_Standard.pdf"},
		{"spines", sheet.Options{Format: "spines", Size: "large", Paper: "a5"}, "spines_A5_Large.pdf"},
		{"border cut is the default", sheet.Options{Cut: "border"}, "covers_A4_Standard.pdf"},
		{"bleed cut", sheet.Options{Cut: "bleed"}, "covers_A4_Standard_bleed.pdf"},
		{"no cut", sheet.Options{Size: "mini", Paper: "a5", Cut: "none"}, "covers_A5_Mini_none.pdf"},
		{"review cards ignore the cut", sheet.Options{Format: "cards", Cut: "bleed"}, "cards_A4.pdf"},
		{"spines ignore the cut", sheet.Options{Format: "spines", Cut: "none"}, "spines_A4_Standard.pdf"},
		{"default ticket stubs", sheet.Options{Format: "stubs"}, "stubs_A4.pdf"},
		{"ticket stubs", sheet.Options{Format: "stubs", Paper: "letter"}, "stubs_Letter.pdf"},
		{"ticket stubs ignore the size and cut", sheet.Options{Format: "stubs", Size: "large", Paper: "a5", Cut: "bleed"}, "stubs_A5.pdf"},
		{"default full page", sheet.Options{Format: "fullpage"}, "fullpage_A4.pdf"},
		{"full page with a header", sheet.Options{Format: "fullpage", Paper: "letter", Header: "Books of 2026"}, "fullpage_Letter.pdf"},
		{"full page ignores the size and cut", sheet.Options{Format: "fullpage", Size: "large", Paper: "a5", Cut: "bleed"}, "fullpage_A5.pdf"},
		{"default checklist", sheet.Options{Format: "checklist"}, "checklist_A4.pdf"},
		{"checklist with a header", sheet.Options{Format: "checklist", Paper: "letter", Header: "TBR 2026"}, "checklist_Letter.pdf"},
		{"checklist ignores the size and cut", sheet.Options{Format: "checklist", Size: "large", Paper: "a5", Cut: "bleed"}, "checklist_A5.pdf"},
		{"default polaroids", sheet.Options{Format: "polaroid"}, "polaroid_A4_Standard.pdf"},
		{"polaroids", sheet.Options{Format: "polaroid", Size: "mini", Paper: "letter"}, "polaroid_Letter_Mini.pdf"},
		{"polaroids ignore the cut and header", sheet.Options{Format: "polaroid", Size: "large", Paper: "a5", Cut: "bleed", Header: "Books of 2026"}, "polaroid_A5_Large.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			stubDialogs(t, func(o runtime.SaveDialogOptions) (string, error) {
				got = o.DefaultFilename
				return "", nil // cancel: nothing is written
			}, nil)
			cover := writeCover(t, t.TempDir(), "c.jpg", 66, 100)
			a := newApp(media.NewRegistry(), http.DefaultClient)
			if _, err := a.ExportPDF([]string{cover}, "", tt.opts); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("default file name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExportPDFUsesTheChosenSize(t *testing.T) {
	src := t.TempDir()
	var files []string
	for i := 0; i < 10; i++ {
		files = append(files, writeCover(t, src, fmt.Sprintf("c%02d.jpg", i), 66, 100))
	}
	out := filepath.Join(t.TempDir(), "covers.pdf")
	stubDialogs(t, func(runtime.SaveDialogOptions) (string, error) { return out, nil }, nil)
	a := newApp(media.NewRegistry(), http.DefaultClient)
	// 10 large covers need 2 pages of 9; standard would fit them on one.
	if _, err := a.ExportPDF(files, "", sheet.Options{Size: "large"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(data, []byte("/Type /Page\n")); n != 2 {
		t.Errorf("PDF has %d pages, want 2", n)
	}
}

func TestPrintSizes(t *testing.T) {
	sizes := NewApp().PrintSizes()
	want := []PrintSizeInfo{
		{ID: "mini", Label: "Mini", CoverHeight: 25},
		{ID: "standard", Label: "Standard", CoverHeight: 50, Default: true},
		{ID: "large", Label: "Large", CoverHeight: 80},
	}
	if len(sizes) != len(want) {
		t.Fatalf("PrintSizes() = %+v, want %+v", sizes, want)
	}
	for i := range want {
		if sizes[i] != want[i] {
			t.Errorf("size %d = %+v, want %+v", i, sizes[i], want[i])
		}
	}
}

func TestPrintFormats(t *testing.T) {
	got := NewApp().PrintFormats()
	want := []PrintFormatInfo{
		{ID: "covers", Label: "Cover sheet", Noun: "cover", UsesSize: true, UsesCut: true, Default: true},
		{ID: "cards", Label: "Review cards", Noun: "card"},
		{ID: "spines", Label: "Spines", Noun: "spine", UsesSize: true},
		{ID: "stubs", Label: "Ticket stubs", Noun: "stub"},
		{ID: "fullpage", Label: "Full page", Noun: "cover", UsesHeader: true},
		{ID: "checklist", Label: "Checklist", Noun: "row", UsesHeader: true},
		{ID: "polaroid", Label: "Polaroid", Noun: "polaroid", UsesSize: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PrintFormats() = %+v, want %+v", got, want)
	}
}

func TestPrintCutModes(t *testing.T) {
	got := NewApp().PrintCutModes()
	want := []PrintCutModeInfo{
		{ID: "border", Label: "Border", Default: true},
		{ID: "bleed", Label: "Bleed + crop marks"},
		{ID: "none", Label: "None"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PrintCutModes() = %+v, want %+v", got, want)
	}
}

func TestPrintPapers(t *testing.T) {
	papers := NewApp().PrintPapers()
	want := []PrintPaperInfo{
		{ID: "a4", Label: "A4", Width: 210, Height: 297, Default: true,
			PerPage: map[string]map[string]int{
				"covers":    {"mini": 81, "standard": 25, "large": 9},
				"cards":     {"": 8},
				"spines":    {"mini": 144, "standard": 80, "large": 48},
				"stubs":     {"": 12},
				"fullpage":  {"": 60},
				"checklist": {"": 17},
				"polaroid":  {"mini": 56, "standard": 12, "large": 4},
			},
			HeaderPerPage: map[string]int{"fullpage": 60, "checklist": 16},
			SquarePerPage: map[string]map[string]int{"covers": {"mini": 54, "standard": 15, "large": 6}}},
		{ID: "letter", Label: "US Letter", Width: 215.9, Height: 279.4,
			PerPage: map[string]map[string]int{
				"covers":    {"mini": 81, "standard": 20, "large": 9},
				"cards":     {"": 8},
				"spines":    {"mini": 144, "standard": 64, "large": 48},
				"stubs":     {"": 12},
				"fullpage":  {"": 60},
				"checklist": {"": 16},
				"polaroid":  {"mini": 56, "standard": 12, "large": 4},
			},
			HeaderPerPage: map[string]int{"fullpage": 60, "checklist": 15},
			SquarePerPage: map[string]map[string]int{"covers": {"mini": 63, "standard": 12, "large": 6}}},
		{ID: "a5", Label: "A5", Width: 148, Height: 210,
			PerPage: map[string]map[string]int{
				"covers":    {"mini": 36, "standard": 9, "large": 4},
				"cards":     {"": 3},
				"spines":    {"mini": 66, "standard": 33, "large": 22},
				"stubs":     {"": 4},
				"fullpage":  {"": 60},
				"checklist": {"": 12},
				"polaroid":  {"mini": 25, "standard": 6, "large": 1},
			},
			HeaderPerPage: map[string]int{"fullpage": 60, "checklist": 11},
			SquarePerPage: map[string]map[string]int{"covers": {"mini": 24, "standard": 6, "large": 2}}},
	}
	if len(papers) != len(want) {
		t.Fatalf("PrintPapers() = %+v, want %+v", papers, want)
	}
	for i, w := range want {
		t.Run(w.ID, func(t *testing.T) {
			if !reflect.DeepEqual(papers[i], w) {
				t.Errorf("paper %d = %+v, want %+v", i, papers[i], w)
			}
		})
	}
}

// The counts PrintPapers sends are the sheet's own, for every format,
// paper and size.
func TestPrintPapersMatchTheSheet(t *testing.T) {
	for _, p := range NewApp().PrintPapers() {
		for _, f := range sheet.Formats {
			sizes := []string{""}
			if f.UsesSize {
				sizes = nil
				for _, s := range sheet.Sizes {
					sizes = append(sizes, s.ID)
				}
			}
			if len(p.PerPage[f.ID]) != len(sizes) {
				t.Errorf("%s %s: counts %v, want one for each of %q", p.ID, f.ID, p.PerPage[f.ID], sizes)
			}
			for _, size := range sizes {
				want, err := sheet.Options{Format: f.ID, Paper: p.ID, Size: size}.PerPage()
				if err != nil {
					t.Fatal(err)
				}
				if got := p.PerPage[f.ID][size]; got != want {
					t.Errorf("%s %s %q: %d per page, want %d", p.ID, f.ID, size, got, want)
				}
			}
			// Formats that print a header also have their count with one.
			got, ok := p.HeaderPerPage[f.ID]
			if ok != f.UsesHeader {
				t.Errorf("%s %s: count with a header listed: %v, want %v", p.ID, f.ID, ok, f.UsesHeader)
			}
			if f.UsesHeader {
				want, err := sheet.Options{Format: f.ID, Paper: p.ID, Header: "TBR 2026"}.PerPage()
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("%s %s with a header: %d per page, want %d", p.ID, f.ID, got, want)
				}
			}
			// Formats with square cells also have their square count.
			squares, ok := p.SquarePerPage[f.ID]
			if ok != f.ShapedCells {
				t.Errorf("%s %s: square counts listed: %v, want %v", p.ID, f.ID, ok, f.ShapedCells)
			}
			for _, size := range sizes {
				if !f.ShapedCells {
					break
				}
				want, err := sheet.Options{Format: f.ID, Paper: p.ID, Size: size}.SquarePerPage()
				if err != nil {
					t.Fatal(err)
				}
				if got := squares[size]; got != want {
					t.Errorf("%s %s %q: %d squares per page, want %d", p.ID, f.ID, size, got, want)
				}
			}
		}
	}
}

func TestPrintPages(t *testing.T) {
	dir := t.TempDir()
	book := writeCover(t, dir, "Dune - Frank Herbert.jpg", 66, 100)
	album := writeCover(t, dir, "Abbey Road - The Beatles.jpg", 120, 120)
	painting := writeCover(t, dir, "Wheat Field with Cypresses - Vincent van Gogh.jpg", 150, 100)
	portrait := writeCover(t, dir, "Portrait.jpg", 80, 100) // 4:5
	panorama := writeCover(t, dir, "Panorama.jpg", 500, 100)
	bad := filepath.Join(dir, "Broken.jpg")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	repeat := func(n int, paths ...string) []string {
		out := make([]string, 0, n*len(paths))
		for range n {
			out = append(out, paths...)
		}
		return out
	}
	tests := []struct {
		name  string
		paths []string
		opts  sheet.Options
		want  PrintPagesInfo
	}{
		{"no images", nil, sheet.Options{}, PrintPagesInfo{Exact: true}},
		{"25 books fill a page", repeat(25, book), sheet.Options{}, PrintPagesInfo{Pages: 1, Exact: true}},
		{"26 books", repeat(26, book), sheet.Options{}, PrintPagesInfo{Pages: 2, Exact: true}},
		{"15 albums fill a page", repeat(15, album), sheet.Options{}, PrintPagesInfo{Pages: 1, AllSquare: true, Exact: true}},
		{"16 albums", repeat(16, album), sheet.Options{}, PrintPagesInfo{Pages: 2, AllSquare: true, Exact: true}},
		{"large albums on A5", repeat(3, album), sheet.Options{Size: "large", Paper: "a5"}, PrintPagesInfo{Pages: 2, AllSquare: true, Exact: true}},
		// A row holds 2 albums and a book (143.6 mm; another album does
		// not fit), and 6 rows take 2 pages.
		{"mixed", repeat(6, album, album, book), sheet.Options{}, PrintPagesInfo{Pages: 2, Exact: true}},
		{"albums on review cards", repeat(9, album), sheet.Options{Format: "cards"}, PrintPagesInfo{Pages: 2, AllSquare: true, Exact: true}},
		{"albums on a checklist with a header", repeat(17, album), sheet.Options{Format: "checklist", Header: "TBR"}, PrintPagesInfo{Pages: 2, AllSquare: true, Exact: true}},
		// An unreadable image counts as a 2:3 cover; the readable albums
		// still get square cells (31 albums need 3 pages, not 32/25 → 2).
		{"unreadable image", append(repeat(31, album), bad), sheet.Options{}, PrintPagesInfo{Pages: 3}},
		{"missing image", []string{filepath.Join(dir, "missing.jpg")}, sheet.Options{}, PrintPagesInfo{Pages: 1}},
		// 3:2 paintings get 75 mm wide cells on A4: 10 a page. The size
		// labels show how many of them fit at each size.
		{"10 paintings fill a page", repeat(10, painting), sheet.Options{},
			PrintPagesInfo{Pages: 1, Exact: true, WidePerPage: map[string]int{"mini": 36, "standard": 10, "large": 3}}},
		{"11 paintings", repeat(11, painting), sheet.Options{},
			PrintPagesInfo{Pages: 2, Exact: true, WidePerPage: map[string]int{"mini": 36, "standard": 10, "large": 3}}},
		{"paintings on Letter", repeat(9, painting), sheet.Options{Paper: "letter"},
			PrintPagesInfo{Pages: 2, Exact: true, WidePerPage: map[string]int{"mini": 36, "standard": 8, "large": 3}}},
		// The labels use the middle shape: 3 paintings and 2 panoramas
		// count as paintings.
		{"paintings and panoramas", append(repeat(3, painting), panorama, panorama), sheet.Options{},
			PrintPagesInfo{Pages: 1, Exact: true, WidePerPage: map[string]int{"mini": 36, "standard": 10, "large": 3}}},
		// A 4:5 portrait fits the 2:3 column of standard and large covers
		// on A5, so only mini has a wide count.
		{"4:5 portraits on A5", repeat(4, portrait), sheet.Options{Paper: "a5"},
			PrintPagesInfo{Pages: 1, Exact: true, WidePerPage: map[string]int{"mini": 30}}},
		// Mixed shapes keep the 2:3 labels, with an exact page count: a
		// row holds an album and a painting (129 mm; a second album does
		// not fit), 5 rows a page.
		{"albums and paintings", repeat(6, album, painting), sheet.Options{}, PrintPagesInfo{Pages: 2, Exact: true}},
		{"paintings on review cards", repeat(9, painting), sheet.Options{Format: "cards"}, PrintPagesInfo{Pages: 2, Exact: true}},
		// The readable paintings keep their wide cells: 6 rows, 2 pages.
		{"unreadable painting", append(repeat(12, painting), bad), sheet.Options{}, PrintPagesInfo{Pages: 2}},
	}
	a := newApp(media.NewRegistry(), http.DefaultClient)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.PrintPages(tt.paths, tt.opts)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrintPages = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
	for _, opts := range []sheet.Options{{Format: "poster"}, {Size: "huge"}, {Paper: "a3"}, {Cut: "laser"}} {
		if _, err := a.PrintPages([]string{album}, opts); err == nil {
			t.Errorf("PrintPages with %+v: want an error", opts)
		}
	}
}

func TestOpenFileRejectsMissingFiles(t *testing.T) {
	a := newApp(media.NewRegistry(), http.DefaultClient)
	for _, p := range []string{"", "  ", filepath.Join(t.TempDir(), "missing.pdf"), t.TempDir()} {
		if err := a.OpenFile(p); err == nil {
			t.Errorf("OpenFile(%q): want an error", p)
		}
	}
}

func TestIsYes(t *testing.T) {
	for answer, want := range map[string]bool{
		"Replace": true, "Yes": true, "Ok": true, "Cancel": false, "No": false, "": false,
		"Reemplazar": true, "reemplazar": true, "Cancelar": false,
	} {
		if got := isYes(answer); got != want {
			t.Errorf("isYes(%q) = %v, want %v", answer, got, want)
		}
	}
}

func TestSaveSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bujo-helper", "settings.json")
	a := newApp(media.NewRegistry(), http.DefaultClient)
	a.loadSettings(path)
	if got := a.GetSettings(); got != (settings.Settings{}) {
		t.Fatalf("GetSettings before saving = %+v, want zero", got)
	}

	if err := a.SaveSettings(settings.Settings{TMDBAPIKey: " key-1 "}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if got := a.GetSettings().TMDBAPIKey; got != "key-1" {
		t.Errorf("GetSettings key = %q, want key-1", got)
	}
	if got := a.tmdbKey(); got != "key-1" {
		t.Errorf("tmdbKey = %q, want key-1 without a restart", got)
	}

	saved, err := settings.Load(path)
	if err != nil || saved.TMDBAPIKey != "key-1" {
		t.Errorf("file holds %+v (err %v), want key-1", saved, err)
	}

	// A new App (next start) reads the saved key.
	b := newApp(media.NewRegistry(), http.DefaultClient)
	b.loadSettings(path)
	if got := b.GetSettings().TMDBAPIKey; got != "key-1" {
		t.Errorf("new App key = %q, want key-1", got)
	}
}

func TestLoadSettingsErrorIsNotFatal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newApp(media.NewRegistry(), http.DefaultClient)
	a.loadSettings(path)
	if got := a.GetSettings(); got != (settings.Settings{}) {
		t.Errorf("GetSettings = %+v, want zero after a bad file", got)
	}
	// Saving still works and replaces the broken file.
	if err := a.SaveSettings(settings.Settings{TMDBAPIKey: "k"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if s, err := settings.Load(path); err != nil || s.TMDBAPIKey != "k" {
		t.Errorf("file holds %+v (err %v), want key k", s, err)
	}
}

// withConfigDir points the user config directory at a temporary folder on
// every OS and returns it.
func withConfigDir(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	base, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("no user config dir: %v", err)
	}
	return base
}

// captureLog sends the standard logger to a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestNewAppMigratesTheLegacySettings(t *testing.T) {
	base := withConfigDir(t)
	legacy := filepath.Join(base, "cover-downloader", "settings.json")
	if err := settings.Save(legacy, settings.Settings{TMDBAPIKey: "old-key", IGDBClientID: "old-id"}); err != nil {
		t.Fatal(err)
	}

	a := NewApp()

	if got, want := a.GetSettings(), (settings.Settings{TMDBAPIKey: "old-key", IGDBClientID: "old-id"}); got != want {
		t.Errorf("GetSettings = %+v, want the legacy settings %+v", got, want)
	}
	if _, err := os.Stat(filepath.Join(base, "bujo-helper", "settings.json")); err != nil {
		t.Errorf("new settings file not written: %v", err)
	}
	if err := a.SaveSettings(settings.Settings{TMDBAPIKey: "new-key"}); err != nil {
		t.Fatalf("SaveSettings after migration: %v", err)
	}
}

func TestNewAppLogsASkippedLegacyFile(t *testing.T) {
	base := withConfigDir(t)
	legacy := filepath.Join(base, "cover-downloader", "settings.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	logs := captureLog(t)

	a := NewApp()

	if got := a.GetSettings(); got != (settings.Settings{}) {
		t.Errorf("GetSettings = %+v, want zero settings", got)
	}
	if out := logs.String(); !strings.Contains(out, "skipped legacy file") || !strings.Contains(out, legacy) {
		t.Errorf("log = %q, want a warning naming the skipped legacy file", out)
	}
	// The app still saves settings normally.
	if err := a.SaveSettings(settings.Settings{TMDBAPIKey: "k"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
}

func TestSaveSettingsWithoutPathFails(t *testing.T) {
	a := newApp(media.NewRegistry(), http.DefaultClient)
	if err := a.SaveSettings(settings.Settings{TMDBAPIKey: "k"}); err == nil {
		t.Fatal("SaveSettings with no settings path returned no error")
	}
	if got := a.tmdbKey(); got != "" {
		t.Errorf("tmdbKey = %q after a failed save, want empty", got)
	}
}

func TestDefaultAppMoviesUseTheSavedTMDBKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(path, settings.Settings{TMDBAPIKey: "saved-key"}); err != nil {
		t.Fatal(err)
	}
	a := newDefaultApp(http.DefaultClient, path)

	var ids []string
	for _, mt := range a.MediaTypes() {
		ids = append(ids, mt.ID)
	}
	if want := []string{"books", "movies", "tv", "albums", "games", "artworks", "places", "nature"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("media types = %q, want %q", ids, want)
	}
	movies, _ := a.registry.Get("movies")
	tmdb, ok := movies.Providers[0].(*providers.TMDB)
	if !ok {
		t.Fatalf("first movie provider is %T, want *providers.TMDB", movies.Providers[0])
	}
	if got := tmdb.Key(); got != "saved-key" {
		t.Errorf("TMDB key = %q, want the saved key", got)
	}
	if err := a.SaveSettings(settings.Settings{TMDBAPIKey: "new-key"}); err != nil {
		t.Fatal(err)
	}
	if got := tmdb.Key(); got != "new-key" {
		t.Errorf("TMDB key after saving = %q, want new-key without a restart", got)
	}
}

func TestSaveSettingsTrimsTheIGDBCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	a := newApp(media.NewRegistry(), http.DefaultClient)
	a.loadSettings(path)
	if err := a.SaveSettings(settings.Settings{IGDBClientID: " id ", IGDBClientSecret: "\tsecret\n"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if id, secret := a.igdbCredentials(); id != "id" || secret != "secret" {
		t.Errorf("igdbCredentials = %q, %q, want trimmed values without a restart", id, secret)
	}
	if s, err := settings.Load(path); err != nil || s.IGDBClientID != "id" || s.IGDBClientSecret != "secret" {
		t.Errorf("file holds %+v (err %v), want the trimmed credentials", s, err)
	}
}

func TestDefaultAppGamesUseTheSavedIGDBCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(path, settings.Settings{IGDBClientID: "saved-id", IGDBClientSecret: "saved-secret"}); err != nil {
		t.Fatal(err)
	}
	a := newDefaultApp(http.DefaultClient, path)
	games, ok := a.registry.Get("games")
	if !ok {
		t.Fatal("no games media type")
	}
	igdb, ok := games.Providers[0].(*providers.IGDB)
	if !ok {
		t.Fatalf("first games provider is %T, want *providers.IGDB", games.Providers[0])
	}
	if id, secret := igdb.Credentials(); id != "saved-id" || secret != "saved-secret" {
		t.Errorf("IGDB credentials = %q, %q, want the saved ones", id, secret)
	}
	if err := a.SaveSettings(settings.Settings{IGDBClientID: "new-id", IGDBClientSecret: "new-secret"}); err != nil {
		t.Fatal(err)
	}
	if id, secret := igdb.Credentials(); id != "new-id" || secret != "new-secret" {
		t.Errorf("IGDB credentials after saving = %q, %q, want the new ones without a restart", id, secret)
	}
}

func TestOnRateLimitWaitEmitsRoundedUpSeconds(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want int
	}{
		{time.Second, 1},
		{1500 * time.Millisecond, 2},
		{30 * time.Second, 30},
		{30*time.Second + time.Millisecond, 31},
		{time.Millisecond, 1},
	}
	for _, tt := range tests {
		a := newApp(media.NewRegistry(), http.DefaultClient)
		var gotEvent string
		var gotData any
		a.emit = func(event string, data any) { gotEvent, gotData = event, data }

		a.onRateLimitWait("api.themoviedb.org", tt.d)

		if gotEvent != EventRateLimitWait {
			t.Errorf("%v: event = %q, want %q", tt.d, gotEvent, EventRateLimitWait)
		}
		want := RateLimitEvent{Host: "api.themoviedb.org", Seconds: tt.want}
		if gotData != want {
			t.Errorf("%v: data = %#v, want %#v", tt.d, gotData, want)
		}
	}
}

func TestOnRateLimitWaitIgnoresNonPositiveWaits(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		a := newApp(media.NewRegistry(), http.DefaultClient)
		sent := 0
		a.emit = func(string, any) { sent++ }

		a.onRateLimitWait("api.themoviedb.org", d)

		if sent != 0 {
			t.Errorf("%v: sent %d events, want none", d, sent)
		}
	}
}

func TestSampleCSVForEachBuiltInType(t *testing.T) {
	a := NewApp()
	types := a.registry.List()
	if len(types) == 0 {
		t.Fatal("no media types")
	}
	for _, mt := range types {
		t.Run(mt.ID, func(t *testing.T) {
			if len(mt.Samples) < 3 {
				t.Fatalf("%d samples, want at least 3", len(mt.Samples))
			}
			name, data, err := a.sampleCSV(mt.ID)
			if err != nil {
				t.Fatalf("sampleCSV: %v", err)
			}
			if want := "sample_" + mt.ID + ".csv"; name != want {
				t.Errorf("name = %q, want %q", name, want)
			}
			// A UTF-8 BOM makes Excel read the accented names correctly.
			header := "\xef\xbb\xbftitle," + strings.ToLower(mt.CreatorLabel) + "\n"
			if !strings.HasPrefix(string(data), header) {
				t.Errorf("sample %q does not start with %q", data, header)
			}
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := importCSVFile(path, i18n.EN)
			if err != nil {
				t.Fatalf("importCSVFile(sample): %v", err)
			}
			// The header is skipped and every sample comes back unchanged.
			if want := batch.FormatLines(mt.Samples); got != want {
				t.Errorf("sample imports as\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestSampleCSVUnknownType(t *testing.T) {
	if _, _, err := NewApp().sampleCSV("comics"); err == nil {
		t.Error("sampleCSV(comics) err = nil, want an error")
	}
}

func TestWriteFileAtomicFailureKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.csv")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeFileAtomic(path, "CSV", func(w io.Writer) error {
		io.WriteString(w, "partial")
		return fmt.Errorf("boom")
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want boom", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("file = %q after a failure, want it unchanged", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("folder has %d entries after a failure, want only the original", len(entries))
	}

	if err := writeFileAtomic(path, "CSV", func(w io.Writer) error {
		_, err := io.WriteString(w, "new")
		return err
	}); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("file = %q, want new", got)
	}
}

func TestWriteFileAtomicLongName(t *testing.T) {
	// A name at the usual 255-byte limit must still save: the temporary
	// file name must not grow with the target's name.
	path := filepath.Join(t.TempDir(), strings.Repeat("a", 251)+".csv")
	if err := writeFileAtomic(path, "CSV", func(w io.Writer) error {
		_, err := io.WriteString(w, "ok")
		return err
	}); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "ok" {
		t.Errorf("file = %q, want ok", got)
	}
}

// stubDialogs replaces the native dialogs for one test. A dialog whose stub
// is nil fails the test if it opens.
func stubDialogs(t *testing.T, save func(runtime.SaveDialogOptions) (string, error), message func(runtime.MessageDialogOptions) (string, error)) {
	t.Helper()
	oldSave, oldMessage := saveFileDialog, messageDialog
	t.Cleanup(func() { saveFileDialog, messageDialog = oldSave, oldMessage })
	saveFileDialog = func(_ context.Context, o runtime.SaveDialogOptions) (string, error) {
		if save == nil {
			t.Fatal("unexpected save dialog")
		}
		return save(o)
	}
	messageDialog = func(_ context.Context, o runtime.MessageDialogOptions) (string, error) {
		if message == nil {
			t.Fatalf("unexpected message dialog: %q", o.Message)
		}
		return message(o)
	}
}

func TestWithExtensionWithoutPrompt(t *testing.T) {
	stubDialogs(t, nil, nil) // none of these paths may ask anything
	dir := t.TempDir()
	tests := []struct {
		name, path, ext, want string
	}{
		{"has the extension", "sample.csv", ".csv", "sample.csv"},
		{"has the extension in upper case", "SAMPLE.CSV", ".csv", "SAMPLE.CSV"},
		{"no extension and no file", "sample", ".csv", "sample.csv"},
		{"another extension", "sample.txt", ".csv", "sample.txt.csv"},
		{"pdf without extension", "covers", ".pdf", "covers.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewApp().withExtension(filepath.Join(dir, tt.path), tt.ext)
			if err != nil {
				t.Fatalf("withExtension: %v", err)
			}
			if want := filepath.Join(dir, tt.want); got != want {
				t.Errorf("withExtension = %q, want %q", got, want)
			}
		})
	}
}

func TestWithExtensionAsksBeforeReplacing(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "sample.csv")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	dialogErr := errors.New("dialog failed")
	tests := []struct {
		name    string
		answer  string
		err     error
		want    string
		wantErr error
	}{
		{"replace", "Replace", nil, existing, nil},
		{"cancel", "Cancel", nil, "", nil},
		{"dialog error", "", dialogErr, "", dialogErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asked := 0
			stubDialogs(t, nil, func(runtime.MessageDialogOptions) (string, error) {
				asked++
				return tt.answer, tt.err
			})
			got, err := NewApp().withExtension(filepath.Join(dir, "sample"), ".csv")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("withExtension = %q, want %q", got, tt.want)
			}
			if asked != 1 {
				t.Errorf("asked %d times, want 1", asked)
			}
		})
	}
}

func TestSaveSampleCSVDialogOutcomes(t *testing.T) {
	a := NewApp()
	id := a.registry.List()[0].ID
	name, data, err := a.sampleCSV(id)
	if err != nil {
		t.Fatal(err)
	}
	dialogErr := errors.New("dialog failed")

	t.Run("cancelled", func(t *testing.T) {
		stubDialogs(t, func(o runtime.SaveDialogOptions) (string, error) {
			if o.DefaultFilename != name {
				t.Errorf("DefaultFilename = %q, want %q", o.DefaultFilename, name)
			}
			return "", nil
		}, nil)
		if got, err := a.SaveSampleCSV(id); got != "" || err != nil {
			t.Errorf("SaveSampleCSV = %q, %v; want \"\", nil", got, err)
		}
	})

	t.Run("dialog error", func(t *testing.T) {
		stubDialogs(t, func(runtime.SaveDialogOptions) (string, error) { return "", dialogErr }, nil)
		if got, err := a.SaveSampleCSV(id); got != "" || !errors.Is(err, dialogErr) {
			t.Errorf("SaveSampleCSV = %q, %v; want \"\", %v", got, err, dialogErr)
		}
	})

	t.Run("replace declined", func(t *testing.T) {
		dir := t.TempDir()
		existing := filepath.Join(dir, "mine.csv")
		if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		stubDialogs(t,
			func(runtime.SaveDialogOptions) (string, error) { return filepath.Join(dir, "mine"), nil },
			func(runtime.MessageDialogOptions) (string, error) { return "Cancel", nil })
		if got, err := a.SaveSampleCSV(id); got != "" || err != nil {
			t.Errorf("SaveSampleCSV = %q, %v; want \"\", nil", got, err)
		}
		if b, _ := os.ReadFile(existing); string(b) != "old" {
			t.Errorf("existing file = %q, want it unchanged", b)
		}
	})

	t.Run("saved with added extension", func(t *testing.T) {
		dir := t.TempDir()
		stubDialogs(t, func(runtime.SaveDialogOptions) (string, error) { return filepath.Join(dir, "mine"), nil }, nil)
		got, err := a.SaveSampleCSV(id)
		if err != nil {
			t.Fatalf("SaveSampleCSV: %v", err)
		}
		if want := filepath.Join(dir, "mine.csv"); got != want {
			t.Errorf("SaveSampleCSV = %q, want %q", got, want)
		}
		if b, _ := os.ReadFile(got); !bytes.Equal(b, data) {
			t.Errorf("saved %q, want the sample %q", b, data)
		}
	})
}

func TestExportPDFRejectsUnknownCutBeforeTheDialog(t *testing.T) {
	stubDialogs(t, func(runtime.SaveDialogOptions) (string, error) {
		t.Error("save dialog shown for an unknown cut mode")
		return "", nil
	}, nil)
	_, err := NewApp().ExportPDF([]string{"cover.jpg"}, "", sheet.Options{Cut: "kiss"})
	if err == nil || !strings.Contains(err.Error(), "kiss") {
		t.Errorf("error %v, want one naming the cut mode", err)
	}
}

func TestExportPDFSuggestsFileNameWithPaperAndSize(t *testing.T) {
	tests := []struct {
		paper, size, want string
	}{
		{"", "", "covers_A4_Standard.pdf"},
		{"a4", "standard", "covers_A4_Standard.pdf"},
		{"a4", "mini", "covers_A4_Mini.pdf"},
		{"letter", "large", "covers_Letter_Large.pdf"},
		{"a5", "mini", "covers_A5_Mini.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.paper+"/"+tt.size, func(t *testing.T) {
			var got string
			stubDialogs(t, func(o runtime.SaveDialogOptions) (string, error) {
				got = o.DefaultFilename
				return "", nil // cancelled
			}, nil)
			opts := sheet.Options{Paper: tt.paper, Size: tt.size}
			if _, err := NewApp().ExportPDF([]string{"cover.jpg"}, "", opts); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("DefaultFilename = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetLanguage(t *testing.T) {
	a := newApp(media.NewRegistry(), http.DefaultClient)
	if got := a.language(); got != i18n.EN {
		t.Errorf("default language = %q, want en", got)
	}
	for in, want := range map[string]i18n.Lang{"es-ES": i18n.ES, "es": i18n.ES, "en-GB": i18n.EN, "": i18n.EN, "fr": i18n.EN} {
		a.SetLanguage(in)
		if got := a.language(); got != want {
			t.Errorf("SetLanguage(%q): language = %q, want %q", in, got, want)
		}
	}
}

func TestBindingErrorsUseTheUILanguage(t *testing.T) {
	a, _ := testApp(t, &fakeProvider{})
	a.SetLanguage("es")
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(empty, []byte("title,author\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	call := func(f func() error) func() error { return f }
	tests := []struct {
		name string
		call func() error
		want string
	}{
		{"no items", call(func() error {
			_, err := a.Start(StartRequest{MediaType: "books", OutputDir: dir})
			return err
		}), `no hay elementos: escriba uno por línea con el formato "Título, Autor"`},
		{"unknown media type", call(func() error {
			_, err := a.Start(StartRequest{MediaType: "games", Text: "Doom", OutputDir: dir})
			return err
		}), `tipo de medio desconocido: "games"`},
		{"no output folder", call(func() error {
			_, err := a.Start(StartRequest{MediaType: "books", Text: "Dune", OutputDir: " "})
			return err
		}), "elija una carpeta de destino"},
		{"unknown sample media type", call(func() error {
			_, err := a.SaveSampleCSV("games")
			return err
		}), `tipo de medio desconocido: "games"`},
		{"no covers selected", call(func() error {
			_, err := a.ExportPDF(nil, dir, sheet.Options{})
			return err
		}), "seleccione al menos una portada"},
		{"no gallery folder", call(func() error {
			_, err := a.ListCovers("  ")
			return err
		}), "elija una carpeta"},
		{"no folder to open", call(func() error { return a.OpenFolder(" ") }), "no hay ninguna carpeta que abrir"},
		{"no file to open", call(func() error { return a.OpenFile(" ") }), "no hay ningún archivo que abrir"},
		{"unknown settings location", call(func() error { return a.SaveSettings(settings.Settings{}) }), "guardar ajustes: se desconoce la ubicación del archivo de ajustes"},
		{"empty CSV", call(func() error {
			_, err := importCSVFile(empty, a.language())
			return err
		}), "empty.csv: no se han encontrado elementos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil || err.Error() != tt.want {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}

	// The output folder error keeps its cause.
	_, err := a.Start(StartRequest{MediaType: "books", Text: "Dune", OutputDir: filepath.Join(file, "sub")})
	var pathErr *os.PathError
	if err == nil || !strings.HasPrefix(err.Error(), "no se ha podido crear la carpeta de destino: ") || !errors.As(err, &pathErr) {
		t.Errorf("Start under a file: err = %v, want a translated message wrapping the cause", err)
	}
}

func TestStartWhileRunningInSpanish(t *testing.T) {
	p, started := blockingProvider()
	a, rec := testApp(t, p)
	a.SetLanguage("es")
	req := StartRequest{MediaType: "books", Text: "Dune", OutputDir: t.TempDir()}
	if _, err := a.Start(req); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started
	if _, err := a.Start(req); err == nil || err.Error() != "ya hay una descarga en curso" {
		t.Errorf("second Start err = %v, want it in Spanish", err)
	}
	a.Cancel()
	rec.waitDone(t)
}

func TestReplacePromptUsesTheUILanguage(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "covers.pdf")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	var asked runtime.MessageDialogOptions
	stubDialogs(t, nil, func(o runtime.MessageDialogOptions) (string, error) {
		asked = o
		return "Reemplazar", nil // GTK returns the label of the clicked button
	})
	a := newApp(media.NewRegistry(), http.DefaultClient)
	a.SetLanguage("es")

	got, err := a.withExtension(filepath.Join(dir, "covers"), ".pdf")
	if err != nil || got != existing {
		t.Fatalf("withExtension = %q, %v, want %q (replace confirmed)", got, err, existing)
	}
	want := runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "¿Reemplazar el archivo?",
		Message:       "covers.pdf ya existe. ¿Desea reemplazarlo?",
		Buttons:       []string{"Reemplazar", "Cancelar"},
		DefaultButton: "Cancelar",
		CancelButton:  "Cancelar",
	}
	if !reflect.DeepEqual(asked, want) {
		t.Errorf("prompt = %+v, want %+v", asked, want)
	}
}

func TestExportPDFDialogUsesTheUILanguage(t *testing.T) {
	var asked runtime.SaveDialogOptions
	stubDialogs(t, func(o runtime.SaveDialogOptions) (string, error) {
		asked = o
		return "", nil // cancelled
	}, nil)
	a := newApp(media.NewRegistry(), http.DefaultClient)
	a.SetLanguage("es")
	if _, err := a.ExportPDF([]string{"x.jpg"}, "", sheet.Options{}); err != nil {
		t.Fatal(err)
	}
	if asked.Title != "Exportar PDF" || len(asked.Filters) != 1 || asked.Filters[0].DisplayName != "Archivos PDF (*.pdf)" {
		t.Errorf("dialog = %+v, want it in Spanish", asked)
	}
}

func TestPDFLabelsFollowTheUILanguage(t *testing.T) {
	a := newApp(media.NewRegistry(), http.DefaultClient)
	opts := sheet.Options{Format: "stubs", DateLabel: "Leído el"}
	if got := a.withLabels(opts); got.Labels != (sheet.Labels{Admit: "ADMIT ONE", Date: "Date:", Where: "Where:", CardDate: "Date", Credits: "Credits"}) {
		t.Errorf("English labels = %+v", got.Labels)
	}
	a.SetLanguage("es")
	got := a.withLabels(opts)
	if got.Labels != (sheet.Labels{Admit: "ENTRADA", Date: "Fecha:", Where: "Lugar:", CardDate: "Fecha", Credits: "Créditos"}) {
		t.Errorf("Spanish labels = %+v", got.Labels)
	}
	if got.Format != opts.Format || got.DateLabel != opts.DateLabel {
		t.Errorf("withLabels changed the other options: %+v", got)
	}
}

func TestItemEventCodes(t *testing.T) {
	a, rec := testApp(t, &fakeProvider{search: func(_ context.Context, item media.Item) ([]media.Candidate, error) {
		if item.Title == "Boom" {
			return nil, errors.New("boom")
		}
		return nil, fmt.Errorf("%w: nothing", media.ErrNotFound)
	}})
	if _, err := a.Start(StartRequest{MediaType: "books", Text: "Missing\nBoom", OutputDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	rec.waitDone(t)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	want := map[string]string{"Missing": downloader.CodeNotFound, "Boom": ""}
	if len(rec.items) != len(want) {
		t.Fatalf("got %d item events", len(rec.items))
	}
	for _, ev := range rec.items {
		if ev.Code != want[ev.Title] || ev.Error == "" {
			t.Errorf("%s: code %q (error %q), want code %q and the English detail", ev.Title, ev.Code, ev.Error, want[ev.Title])
		}
	}
}
