package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadMissingFileReturnsZeroSettings(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope", "settings.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s != (Settings{}) {
		t.Errorf("Load = %+v, want zero Settings", s)
	}
}

func TestSaveThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover-downloader", "settings.json")
	if err := Save(path, Settings{TMDBAPIKey: "  abc123\n"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.TMDBAPIKey != "abc123" {
		t.Errorf("TMDBAPIKey = %q, want the trimmed key abc123", got.TMDBAPIKey)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("file mode = %o, want 600", mode)
		}
		dirInfo, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if mode := dirInfo.Mode().Perm(); mode != 0o700 {
			t.Errorf("dir mode = %o, want 700", mode)
		}
	}
}

func TestSaveReplacesAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	for _, key := range []string{"first", "second"} {
		if err := Save(path, Settings{TMDBAPIKey: key}); err != nil {
			t.Fatalf("Save(%q): %v", key, err)
		}
	}
	got, err := Load(path)
	if err != nil || got.TMDBAPIKey != "second" {
		t.Fatalf("Load = %+v, %v; want key second", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir holds %v, want only settings.json", names)
	}
}

func TestSaveUsesTheJSONFieldName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, Settings{TMDBAPIKey: "k"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"tmdbApiKey": "k"`; !strings.Contains(string(b), want) {
		t.Errorf("file = %s, want it to contain %s", b, want)
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load of invalid JSON returned no error")
	}
}

func TestPathIsUnderTheUserConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	base, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("no user config dir: %v", err)
	}
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "cover-downloader", "settings.json"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestSaveTrimsAndKeepsTheIGDBCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	in := Settings{TMDBAPIKey: "tmdb", IGDBClientID: " client-id\n", IGDBClientSecret: "\tsecret "}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{TMDBAPIKey: "tmdb", IGDBClientID: "client-id", IGDBClientSecret: "secret"}); got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"igdbClientId": "client-id"`, `"igdbClientSecret": "secret"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("file = %s, want it to contain %s", b, want)
		}
	}
}

func TestLoadKeepsOlderFilesWithoutIGDBCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"tmdbApiKey": "k"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != (Settings{TMDBAPIKey: "k"}) {
		t.Errorf("Load = %+v, want only the TMDB key", got)
	}
}
