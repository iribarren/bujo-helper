package main

import (
	"bytes"
	"context"
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
