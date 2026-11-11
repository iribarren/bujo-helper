package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cover-downloader/internal/media"
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
	if err := reg.Register(media.MediaType{ID: "books", Label: "Books", CreatorLabel: "Author", Providers: []media.Provider{p}}); err != nil {
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
	if len(got) != 1 || got[0] != (MediaTypeInfo{ID: "books", Label: "Books", CreatorLabel: "Author"}) {
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
	got, err := importCSVFile(good)
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
		if _, err := importCSVFile(path); err == nil || !strings.HasPrefix(err.Error(), filepath.Base(path)+": ") {
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

	if err := exportTo(out, files); err != nil {
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

	err := exportTo(out, []string{good, bad})
	if err == nil || !strings.Contains(err.Error(), "bad.jpg") {
		t.Fatalf("err = %v, want an error naming bad.jpg", err)
	}
	if entries, _ := os.ReadDir(outDir); len(entries) != 0 {
		t.Errorf("output folder has %d entries after a failure, want none", len(entries))
	}
}

func TestExportPDFNeedsCovers(t *testing.T) {
	a := newApp(media.NewRegistry(), http.DefaultClient)
	if _, err := a.ExportPDF(nil, ""); err == nil {
		t.Error("no covers: want an error")
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
