package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cover-downloader/internal/media"
	"cover-downloader/internal/settings"
)

type fakeProvider struct{ imageURL string }

func (f fakeProvider) Name() string { return "fake" }

func (f fakeProvider) Search(_ context.Context, item media.Item, _ media.Options) ([]media.Candidate, error) {
	if strings.HasPrefix(item.Title, "Missing") {
		return nil, media.ErrNotFound
	}
	return []media.Candidate{{Title: item.Title, ImageURLs: []string{f.imageURL}}}, nil
}

func testRegistry(t *testing.T) *media.Registry {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(bytes.Repeat([]byte{0xFF}, 2048))
	}))
	t.Cleanup(srv.Close)
	reg := media.NewRegistry()
	if err := reg.Register(media.MediaType{ID: "books", Providers: []media.Provider{fakeProvider{srv.URL + "/c.jpg"}}}); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "list.csv")
	if err := os.WriteFile(csvPath, []byte("Title;Author\nEmma;Jane Austen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  []string
		wantErr  string
		wantFile string
	}{
		{
			name:     "all succeed",
			args:     []string{"-o", out, "Dune, Frank Herbert"},
			wantCode: 0,
			wantOut:  []string{"OK  Dune -> " + filepath.Join(out, "Dune - Frank Herbert.jpg") + " (fake)", "1 ok, 0 failed"},
			wantFile: "Dune - Frank Herbert.jpg",
		},
		{
			name:     "one failure exits 1 but the batch continues",
			args:     []string{"-o", out, "-csv", csvPath, "Missing book", "Dune"},
			wantCode: 1,
			wantOut:  []string{"FAIL Missing book: fake: not found", "OK  Emma -> ", "OK  Dune -> ", "2 ok, 1 failed"},
			wantFile: "Emma - Jane Austen.jpg",
		},
		{name: "no items", args: []string{"-o", out}, wantCode: 2, wantErr: "no items"},
		{name: "unknown type", args: []string{"-type", "games", "-o", out, "X"}, wantCode: 2, wantErr: `unknown media type "games"`},
		{name: "missing csv", args: []string{"-csv", filepath.Join(dir, "nope.csv"), "X"}, wantCode: 2, wantErr: "nope.csv"},
		{name: "bad flag", args: []string{"-nope"}, wantCode: 2, wantErr: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tt.args, &stdout, &stderr, testRegistry(t))
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.wantCode, &stdout, &stderr)
			}
			for _, want := range tt.wantOut {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout missing %q:\n%s", want, &stdout)
				}
			}
			if tt.wantErr != "" && !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr missing %q:\n%s", tt.wantErr, &stderr)
			}
			if tt.wantFile != "" {
				if _, err := os.Stat(filepath.Join(out, tt.wantFile)); err != nil {
					t.Errorf("expected file: %v", err)
				}
			}
		})
	}
}

func TestRunCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"-o", t.TempDir(), "Dune"}, &stdout, &stderr, testRegistry(t))
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	for _, want := range []string{"CANCELLED Dune", "1 cancelled"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q: %q", want, &stdout)
		}
	}
}

// countryProvider records the country of each search and finds nothing.
type countryProvider struct{ got chan string }

func (p countryProvider) Name() string { return "country" }

func (p countryProvider) Search(_ context.Context, _ media.Item, opts media.Options) ([]media.Candidate, error) {
	p.got <- opts.Country
	return nil, media.ErrNotFound
}

func TestRunCountry(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, "es"},
		{[]string{"-country", "US"}, "us"},
	}
	for _, tt := range tests {
		p := countryProvider{got: make(chan string, 1)}
		reg := media.NewRegistry()
		if err := reg.Register(media.MediaType{ID: "books", Providers: []media.Provider{p}}); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		args := append(append([]string{"-o", t.TempDir()}, tt.args...), "Dune")
		run(context.Background(), args, &stdout, &stderr, reg)
		if got := <-p.got; got != tt.want {
			t.Errorf("args %q: country = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestUsageShowsDefaultCountry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run(context.Background(), []string{"-o", t.TempDir()}, &stdout, &stderr, testRegistry(t))
	if !strings.Contains(stderr.String(), "[-country es]") {
		t.Errorf("usage does not show the es default:\n%s", &stderr)
	}
}

func TestResolveTMDBKey(t *testing.T) {
	fromFile := func(key string, err error) func() (settings.Settings, error) {
		return func() (settings.Settings, error) { return settings.Settings{TMDBAPIKey: key}, err }
	}
	env := func(vals map[string]string) func(string) string {
		return func(k string) string { return vals[k] }
	}
	tests := []struct {
		name string
		flag string
		env  map[string]string
		load func() (settings.Settings, error)
		want string
	}{
		{"flag wins", "flag-key", map[string]string{"TMDB_API_KEY": "env-key"}, fromFile("file-key", nil), "flag-key"},
		{"env beats file", "", map[string]string{"TMDB_API_KEY": "env-key"}, fromFile("file-key", nil), "env-key"},
		{"file when no flag or env", "", nil, fromFile("file-key", nil), "file-key"},
		{"blank flag and env fall through", "  ", map[string]string{"TMDB_API_KEY": " "}, fromFile("file-key", nil), "file-key"},
		{"values are trimmed", " flag-key\n", nil, fromFile("", nil), "flag-key"},
		{"settings error gives no key", "", nil, fromFile("", errors.New("unreadable")), ""},
		{"nothing set", "", nil, fromFile("", nil), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTMDBKey(tt.flag, env(tt.env), tt.load); got != tt.want {
				t.Errorf("resolveTMDBKey = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveTMDBKeySkipsTheFileWhenNotNeeded(t *testing.T) {
	load := func() (settings.Settings, error) {
		t.Error("settings file read although the flag gave a key")
		return settings.Settings{}, nil
	}
	if got := resolveTMDBKey("flag-key", func(string) string { return "" }, load); got != "flag-key" {
		t.Errorf("resolveTMDBKey = %q, want flag-key", got)
	}
}
