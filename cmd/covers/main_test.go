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

	"bujo-helper/internal/catalog"
	"bujo-helper/internal/media"
	"bujo-helper/internal/settings"
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

// testEnv runs with reg, an empty environment and no saved settings, so
// tests never read the real settings file.
func testEnv(reg *media.Registry) env {
	return env{
		getenv:       func(string) string { return "" },
		loadSettings: func() (settings.Settings, error) { return settings.Settings{}, nil },
		registry:     func(catalog.Keys) *media.Registry { return reg },
	}
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
			code := run(context.Background(), tt.args, &stdout, &stderr, testEnv(testRegistry(t)))
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
	code := run(ctx, []string{"-o", t.TempDir(), "Dune"}, &stdout, &stderr, testEnv(testRegistry(t)))
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
		run(context.Background(), args, &stdout, &stderr, testEnv(reg))
		if got := <-p.got; got != tt.want {
			t.Errorf("args %q: country = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestUsageShowsDefaultCountry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run(context.Background(), []string{"-o", t.TempDir()}, &stdout, &stderr, testEnv(testRegistry(t)))
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

// keyProvider records the TMDB key it sees at search time.
type keyProvider struct {
	key func() string
	got chan string
}

func (p keyProvider) Name() string { return "key" }

func (p keyProvider) Search(context.Context, media.Item, media.Options) ([]media.Candidate, error) {
	k := ""
	if p.key != nil {
		k = p.key()
	}
	p.got <- k
	return nil, media.ErrNotFound
}

func TestRunPassesTheTMDBKey(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		envKey   string
		fileKey  string
		want     string
		readFile bool
	}{
		{name: "flag", flag: "flag-key", envKey: "env-key", fileKey: "file-key", want: "flag-key"},
		{name: "environment", envKey: "env-key", fileKey: "file-key", want: "env-key"},
		{name: "settings file", fileKey: "file-key", want: "file-key", readFile: true},
		{name: "none", want: "", readFile: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make(chan string, 1)
			fileRead := false
			e := env{
				getenv: func(name string) string {
					if name == "TMDB_API_KEY" {
						return tt.envKey
					}
					return ""
				},
				loadSettings: func() (settings.Settings, error) {
					fileRead = true
					return settings.Settings{TMDBAPIKey: tt.fileKey}, nil
				},
				registry: func(keys catalog.Keys) *media.Registry {
					reg := media.NewRegistry()
					if err := reg.Register(media.MediaType{ID: "movies", Providers: []media.Provider{keyProvider{keys.TMDB, got}}}); err != nil {
						t.Fatal(err)
					}
					return reg
				},
			}
			args := []string{"-type", "movies", "-o", t.TempDir()}
			if tt.flag != "" {
				args = append(args, "-tmdb-key", tt.flag)
			}
			var stdout, stderr bytes.Buffer
			run(context.Background(), append(args, "Dune, 2021"), &stdout, &stderr, e)
			if k := received(t, got); k != tt.want {
				t.Errorf("key = %q, want %q", k, tt.want)
			}
			if fileRead != tt.readFile {
				t.Errorf("settings file read = %v, want %v", fileRead, tt.readFile)
			}
		})
	}
}

func TestUsageListsMediaTypes(t *testing.T) {
	e := testEnv(nil)
	e.registry = func(keys catalog.Keys) *media.Registry { return catalog.Default(nil, keys) }
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-o", t.TempDir()}, &stdout, &stderr, e); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	for _, want := range []string{
		"books, movies, tv, albums, games",
		"books|movies|tv|albums|games",
		`movie, TV and game items are "Title, Year"`,
		`albums "Album, Artist"`,
		"-igdb-id", "-igdb-secret", "$IGDB_CLIENT_ID", "$IGDB_CLIENT_SECRET",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("usage missing %q:\n%s", want, &stderr)
		}
	}
}

func TestResolveIGDBCredentials(t *testing.T) {
	fromFile := func(id, secret string, err error) func() (settings.Settings, error) {
		return func() (settings.Settings, error) {
			return settings.Settings{IGDBClientID: id, IGDBClientSecret: secret}, err
		}
	}
	env := func(vals map[string]string) func(string) string {
		return func(k string) string { return vals[k] }
	}
	bothEnv := map[string]string{"IGDB_CLIENT_ID": "env-id", "IGDB_CLIENT_SECRET": "env-secret"}
	tests := []struct {
		name               string
		idFlag, secretFlag string
		env                map[string]string
		load               func() (settings.Settings, error)
		wantID, wantSecret string
	}{
		{"flags win", "flag-id", "flag-secret", bothEnv, fromFile("file-id", "file-secret", nil), "flag-id", "flag-secret"},
		{"env beats file", "", "", bothEnv, fromFile("file-id", "file-secret", nil), "env-id", "env-secret"},
		{"file when no flags or env", "", "", nil, fromFile("file-id", "file-secret", nil), "file-id", "file-secret"},
		{"each value falls through on its own", "flag-id", "", map[string]string{"IGDB_CLIENT_SECRET": " "}, fromFile("file-id", "file-secret", nil), "flag-id", "file-secret"},
		{"values are trimmed", " flag-id\n", "", map[string]string{"IGDB_CLIENT_SECRET": " env-secret "}, fromFile("", "", nil), "flag-id", "env-secret"},
		{"settings error gives no credentials", "", "", nil, fromFile("file-id", "file-secret", errors.New("unreadable")), "", ""},
		{"nothing set", "", "", nil, fromFile("", "", nil), "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, secret := resolveIGDBCredentials(tt.idFlag, tt.secretFlag, env(tt.env), tt.load)
			if id != tt.wantID || secret != tt.wantSecret {
				t.Errorf("resolveIGDBCredentials = %q, %q, want %q, %q", id, secret, tt.wantID, tt.wantSecret)
			}
		})
	}
}

func TestResolveIGDBCredentialsSkipsTheFileWhenNotNeeded(t *testing.T) {
	load := func() (settings.Settings, error) {
		t.Error("settings file read although the flags gave both values")
		return settings.Settings{}, nil
	}
	if id, secret := resolveIGDBCredentials("id", "secret", func(string) string { return "" }, load); id != "id" || secret != "secret" {
		t.Errorf("resolveIGDBCredentials = %q, %q, want id, secret", id, secret)
	}
}

// received returns the value a provider sent on ch during run. run is
// synchronous, so an empty channel means the provider was never searched;
// the test fails at once instead of blocking.
func received[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	default:
		t.Fatal("the provider was never searched")
		var zero T
		return zero
	}
}

// credsProvider records the IGDB credentials it sees at search time.
type credsProvider struct {
	creds func() (string, string)
	got   chan [2]string
}

func (p credsProvider) Name() string { return "creds" }

func (p credsProvider) Search(context.Context, media.Item, media.Options) ([]media.Candidate, error) {
	var id, secret string
	if p.creds != nil {
		id, secret = p.creds()
	}
	p.got <- [2]string{id, secret}
	return nil, media.ErrNotFound
}

func TestRunPassesTheIGDBCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		file settings.Settings
		want [2]string
	}{
		{
			name: "flags",
			args: []string{"-igdb-id", "flag-id", "-igdb-secret", "flag-secret"},
			env:  map[string]string{"IGDB_CLIENT_ID": "env-id", "IGDB_CLIENT_SECRET": "env-secret"},
			want: [2]string{"flag-id", "flag-secret"},
		},
		{
			name: "environment",
			env:  map[string]string{"IGDB_CLIENT_ID": "env-id", "IGDB_CLIENT_SECRET": "env-secret"},
			file: settings.Settings{IGDBClientID: "file-id", IGDBClientSecret: "file-secret"},
			want: [2]string{"env-id", "env-secret"},
		},
		{
			name: "settings file",
			file: settings.Settings{TMDBAPIKey: "tmdb", IGDBClientID: "file-id", IGDBClientSecret: "file-secret"},
			want: [2]string{"file-id", "file-secret"},
		},
		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make(chan [2]string, 1)
			e := env{
				getenv:       func(name string) string { return tt.env[name] },
				loadSettings: func() (settings.Settings, error) { return tt.file, nil },
				registry: func(keys catalog.Keys) *media.Registry {
					reg := media.NewRegistry()
					if err := reg.Register(media.MediaType{ID: "games", Providers: []media.Provider{credsProvider{keys.IGDB, got}}}); err != nil {
						t.Fatal(err)
					}
					return reg
				},
			}
			args := append([]string{"-type", "games", "-o", t.TempDir()}, tt.args...)
			var stdout, stderr bytes.Buffer
			run(context.Background(), append(args, "Hollow Knight, 2017"), &stdout, &stderr, e)
			if c := received(t, got); c != tt.want {
				t.Errorf("credentials = %q, want %q", c, tt.want)
			}
		})
	}
}

func TestRunReadsTheSettingsFileOnlyWhenNeeded(t *testing.T) {
	for _, tt := range []struct {
		name      string
		withKeys  bool
		wantReads int
	}{
		{"once for both keys", true, 1},
		{"never without key lookups", false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reads := 0
			tmdb := make(chan string, 1)
			igdb := make(chan [2]string, 1)
			e := testEnv(nil)
			e.loadSettings = func() (settings.Settings, error) {
				reads++
				return settings.Settings{TMDBAPIKey: "tmdb", IGDBClientID: "id", IGDBClientSecret: "secret"}, nil
			}
			e.registry = func(keys catalog.Keys) *media.Registry {
				if !tt.withKeys {
					return testRegistry(t)
				}
				reg := media.NewRegistry()
				p := []media.Provider{keyProvider{keys.TMDB, tmdb}, credsProvider{keys.IGDB, igdb}}
				if err := reg.Register(media.MediaType{ID: "books", Providers: p}); err != nil {
					t.Fatal(err)
				}
				return reg
			}
			var stdout, stderr bytes.Buffer
			run(context.Background(), []string{"-o", t.TempDir(), "Dune, Frank Herbert"}, &stdout, &stderr, e)
			if reads != tt.wantReads {
				t.Errorf("settings file reads = %d, want %d", reads, tt.wantReads)
			}
			if tt.withKeys && (received(t, tmdb) != "tmdb" || received(t, igdb) != [2]string{"id", "secret"}) {
				t.Error("keys not read from the settings file")
			}
		})
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

func TestLoadUserSettingsMigratesTheLegacySettings(t *testing.T) {
	base := withConfigDir(t)
	legacy := filepath.Join(base, "cover-downloader", "settings.json")
	if err := settings.Save(legacy, settings.Settings{TMDBAPIKey: "old-key", IGDBClientSecret: "old-secret"}); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	got, err := loadUserSettingsWarning(&warn)
	if err != nil {
		t.Fatalf("loadUserSettings: %v", err)
	}
	if want := (settings.Settings{TMDBAPIKey: "old-key", IGDBClientSecret: "old-secret"}); got != want {
		t.Errorf("settings = %+v, want the legacy settings %+v", got, want)
	}
	if _, err := os.Stat(filepath.Join(base, "bujo-helper", "settings.json")); err != nil {
		t.Errorf("new settings file not written: %v", err)
	}
	if warn.Len() != 0 {
		t.Errorf("warnings = %q, want none", warn.String())
	}
}

func TestLoadUserSettingsWarnsAboutASkippedLegacyFile(t *testing.T) {
	base := withConfigDir(t)
	legacy := filepath.Join(base, "cover-downloader", "settings.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	got, err := loadUserSettingsWarning(&warn)
	if err != nil {
		t.Fatalf("loadUserSettings: %v, want the CLI to carry on", err)
	}
	if got != (settings.Settings{}) {
		t.Errorf("settings = %+v, want zero settings", got)
	}
	if out := warn.String(); !strings.HasPrefix(out, "warning: ") || !strings.Contains(out, legacy) {
		t.Errorf("warnings = %q, want one naming the skipped legacy file", out)
	}
}
