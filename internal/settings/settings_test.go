package settings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
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
	path := filepath.Join(t.TempDir(), "bujo-helper", "settings.json")
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
	if want := filepath.Join(base, "bujo-helper", "settings.json"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	legacy, err := LegacyPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "cover-downloader", "settings.json"); legacy != want {
		t.Errorf("LegacyPath = %q, want %q", legacy, want)
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

func TestMigrate(t *testing.T) {
	const legacyJSON = `{"tmdbApiKey": "old-key", "igdbClientId": "old-id"}`
	tests := []struct {
		name string
		// legacy is written to the legacy path; nil means no legacy file
		// and "dir" makes the legacy path a directory (unreadable as a file).
		legacy *string
		// existing is written to the new path first; nil means none.
		existing *string
		// want is the expected new file content as Settings; wantFile
		// reports whether the new file must exist afterwards.
		wantFile bool
		want     Settings
		// wantSkipped reports whether Migrate must return ErrLegacySkipped.
		wantSkipped bool
	}{
		{
			name:     "copies the legacy settings",
			legacy:   ptr(legacyJSON),
			wantFile: true,
			want:     Settings{TMDBAPIKey: "old-key", IGDBClientID: "old-id"},
		},
		{
			name:     "never overwrites an existing new file",
			legacy:   ptr(legacyJSON),
			existing: ptr(`{"tmdbApiKey": "new-key"}`),
			wantFile: true,
			want:     Settings{TMDBAPIKey: "new-key"},
		},
		{
			name: "no legacy file creates nothing",
		},
		{
			name:        "invalid legacy JSON is skipped and reported",
			legacy:      ptr("{not json"),
			wantSkipped: true,
		},
		{
			name:        "unreadable legacy is skipped and reported",
			legacy:      ptr("dir"),
			wantSkipped: true,
		},
		{
			name:     "a bad legacy file is ignored once the new file exists",
			legacy:   ptr("{not json"),
			existing: ptr(`{"tmdbApiKey": "new-key"}`),
			wantFile: true,
			want:     Settings{TMDBAPIKey: "new-key"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			legacy := filepath.Join(base, "cover-downloader", "settings.json")
			path := filepath.Join(base, "bujo-helper", "settings.json")
			if tt.legacy != nil {
				if *tt.legacy == "dir" {
					if err := os.MkdirAll(legacy, 0o700); err != nil {
						t.Fatal(err)
					}
				} else {
					writeFile(t, legacy, *tt.legacy)
				}
			}
			if tt.existing != nil {
				writeFile(t, path, *tt.existing)
			}

			err := Migrate(legacy, path)
			if tt.wantSkipped {
				if !errors.Is(err, ErrLegacySkipped) {
					t.Fatalf("Migrate = %v, want ErrLegacySkipped", err)
				}
				if !strings.Contains(err.Error(), legacy) {
					t.Errorf("Migrate error %q does not name the legacy file", err)
				}
			} else if err != nil {
				t.Fatalf("Migrate: %v", err)
			}

			_, statErr := os.Stat(path)
			if exists := statErr == nil; exists != tt.wantFile {
				t.Fatalf("new file exists = %v, want %v (stat err %v)", exists, tt.wantFile, statErr)
			}
			got, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got != tt.want {
				t.Errorf("Load = %+v, want %+v", got, tt.want)
			}
			if tt.legacy != nil {
				if _, err := os.Stat(legacy); err != nil {
					t.Errorf("legacy file was removed: %v", err)
				}
			}
			if tt.wantFile && tt.existing == nil && runtime.GOOS != "windows" {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if mode := info.Mode().Perm(); mode != 0o600 {
					t.Errorf("file mode = %o, want 600", mode)
				}
			}
		})
	}
}

func ptr(s string) *string { return &s }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTrimmedNormalizesTheLanguage(t *testing.T) {
	for in, want := range map[string]string{
		"":       "",
		"en":     "en",
		" es\n":  "es",
		"ES":     "es",
		"fr":     "",
		"system": "",
		"es-ES":  "",
	} {
		if got := (Settings{Language: in}).Trimmed().Language; got != want {
			t.Errorf("Trimmed Language %q = %q, want %q", in, got, want)
		}
	}
}

func TestSaveAndLoadTheLanguage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, Settings{Language: "es"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"language": "es"`) {
		t.Errorf("file = %s, want a language field", b)
	}
	got, err := Load(path)
	if err != nil || got.Language != "es" {
		t.Errorf("Load = %+v (err %v), want language es", got, err)
	}
}

func TestSaveTrimsAndKeepsTheBHLKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, Settings{TMDBAPIKey: "tmdb", BHLAPIKey: " bhl-key\n"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{TMDBAPIKey: "tmdb", BHLAPIKey: "bhl-key"}); got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"bhlApiKey": "bhl-key"`; !strings.Contains(string(b), want) {
		t.Errorf("file = %s, want it to contain %s", b, want)
	}
	// Files saved before the BHL key existed still load.
	if err := os.WriteFile(path, []byte(`{"tmdbApiKey": "k"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got != (Settings{TMDBAPIKey: "k"}) {
		t.Errorf("Load(old file) = %+v, %v", got, err)
	}
}

func TestSaveTrimsAndKeepsTheBGGToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, Settings{BHLAPIKey: "bhl", BGGToken: " bgg-token\n"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Settings{BHLAPIKey: "bhl", BGGToken: "bgg-token"}); got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"bggToken": "bgg-token"`; !strings.Contains(string(b), want) {
		t.Errorf("file = %s, want it to contain %s", b, want)
	}
	// Files saved before the BoardGameGeek token existed still load.
	if err := os.WriteFile(path, []byte(`{"bhlApiKey": "k"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got != (Settings{BHLAPIKey: "k"}) {
		t.Errorf("Load(old file) = %+v, %v", got, err)
	}
}

func TestTrimmedTrimsTheBGGToken(t *testing.T) {
	if got := (Settings{BGGToken: "\t tok \n"}).Trimmed().BGGToken; got != "tok" {
		t.Errorf("Trimmed BGGToken = %q, want tok", got)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestSaveThenLoadThePrintAndPagesPrefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	in := Settings{
		Language: "es",
		Print: PrintPrefs{
			Format: " book\n", Media: "games", Size: "a5", Crop: "fill", Paper: "a4", Cut: "corners",
			Credits: boolPtr(false), Header: "  My shelf ", Font: "caveat", ColourIn: true,
		},
		Pages: PagesPrefs{Kind: "calendar", Paper: "letter", Cut: "none", Title: " June ", Font: "amatic", ColourIn: true},
	}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{
		Language: "es",
		Print: PrintPrefs{
			Format: "book", Media: "games", Size: "a5", Crop: "fill", Paper: "a4", Cut: "corners",
			Credits: boolPtr(false), Header: "My shelf", Font: "caveat", ColourIn: true,
		},
		Pages: PagesPrefs{Kind: "calendar", Paper: "letter", Cut: "none", Title: "June", Font: "amatic", ColourIn: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"print": {`, `"pages": {`, `"credits": false`, `"colourIn": true`} {
		if !strings.Contains(string(b), s) {
			t.Errorf("file = %s, want it to contain %s", b, s)
		}
	}
}

func TestSaveOmitsEmptyPrefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, Settings{TMDBAPIKey: "k"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"print"`, `"pages"`} {
		if strings.Contains(string(b), s) {
			t.Errorf("file = %s, want no %s object when nothing is remembered", b, s)
		}
	}
}

func TestLoadKeepsOlderFilesWithoutPrefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"tmdbApiKey": "k", "language": "en"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != (Settings{TMDBAPIKey: "k", Language: "en"}) {
		t.Errorf("Load = %+v, want only the key and language", got)
	}
	// A missing credits value means "use the default" (on).
	if got.Print.Credits != nil {
		t.Errorf("Print.Credits = %v, want nil (default)", *got.Print.Credits)
	}
}

func TestTrimmedCapsTheHeaderAndTitle(t *testing.T) {
	long := strings.Repeat("é", 130)
	got := Settings{Print: PrintPrefs{Header: long}, Pages: PagesPrefs{Title: " " + long}}.Trimmed()
	if n := utf8.RuneCountInString(got.Print.Header); n != MaxTextLen {
		t.Errorf("header has %d runes, want %d", n, MaxTextLen)
	}
	if n := utf8.RuneCountInString(got.Pages.Title); n != MaxTextLen {
		t.Errorf("title has %d runes, want %d", n, MaxTextLen)
	}
	if short := (PrintPrefs{Header: " hi "}).Trimmed().Header; short != "hi" {
		t.Errorf("short header = %q, want hi", short)
	}
}
