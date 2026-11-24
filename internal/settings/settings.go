// Package settings stores the user's app settings (such as API keys) in a
// private JSON file under the user config directory.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Settings are the user's saved preferences.
type Settings struct {
	// TMDBAPIKey is the user's The Movie Database API key (v3 key or v4
	// read access token).
	TMDBAPIKey string `json:"tmdbApiKey"`
	// IGDBClientID and IGDBClientSecret are the user's Twitch app
	// credentials, used for the IGDB games API.
	IGDBClientID     string `json:"igdbClientId"`
	IGDBClientSecret string `json:"igdbClientSecret"`
	// BHLAPIKey is the user's Biodiversity Heritage Library API key, used
	// for nature plates.
	BHLAPIKey string `json:"bhlApiKey"`
	// BGGToken is a BoardGameGeek XML API application token, used for
	// board games. It overrides the token built into release builds.
	BGGToken string `json:"bggToken"`
	// Language is the UI language: "" follows the system, "en" or "es".
	Language string `json:"language"`
	// Print and Pages remember the last Print covers and Make pages
	// options. Files saved before they existed load with zero values,
	// which mean "use the defaults".
	Print PrintPrefs `json:"print,omitzero"`
	Pages PagesPrefs `json:"pages,omitzero"`
	// Theme is the colour theme: "" follows the system, "light" or "dark".
	Theme string `json:"theme,omitempty"`
	// View is which parts of the app are shown. The zero value shows
	// everything.
	View ViewPrefs `json:"view,omitzero"`
}

// AlwaysShownSection is the section that can never be hidden: Print.
const AlwaysShownSection = "print"

// ViewPrefs are the parts of the app the user hides. They list what is
// hidden, not what is shown, so sections and items added later show up.
type ViewPrefs struct {
	// HiddenSections are the IDs of the hidden app sections (tabs), such
	// as "download" or "pages"; AlwaysShownSection is never hidden.
	HiddenSections []string `json:"hiddenSections,omitempty"`
	// Hidden maps a list ID (such as "media" or "paper") to the IDs of its
	// hidden items. The frontend keeps at least one item of each list.
	Hidden map[string][]string `json:"hidden,omitempty"`
}

// Trimmed returns v with IDs trimmed, empty and repeated ones dropped,
// AlwaysShownSection never hidden, and lists with no hidden items left
// out; nothing hidden is the zero value.
func (v ViewPrefs) Trimmed() ViewPrefs {
	var out ViewPrefs
	for _, id := range cleanIDs(v.HiddenSections) {
		if id != AlwaysShownSection {
			out.HiddenSections = append(out.HiddenSections, id)
		}
	}
	// Sorted, so lists whose IDs trim to the same one merge in a set order.
	for _, key := range slices.Sorted(maps.Keys(v.Hidden)) {
		list := strings.TrimSpace(key)
		ids := cleanIDs(v.Hidden[key])
		if list == "" || len(ids) == 0 {
			continue
		}
		if out.Hidden == nil {
			out.Hidden = make(map[string][]string, len(v.Hidden))
		}
		out.Hidden[list] = append(out.Hidden[list], ids...)
		out.Hidden[list] = cleanIDs(out.Hidden[list])
	}
	return out
}

// cleanIDs returns ids trimmed, in order, without empty or repeated ones;
// nil when none are left. It never shares ids' array.
func cleanIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// MaxTextLen is the longest header or title kept, in characters; the
// frontend fields have the same limit.
const MaxTextLen = 120

// PrintPrefs are the last options picked in the Print covers tab. Every
// value is an id offered by the app; "" (or an id no longer offered) means
// the default.
type PrintPrefs struct {
	Format string `json:"format,omitempty"`
	// Media is the review cards' media type, saved only once the user
	// picked one.
	Media string `json:"media,omitempty"`
	Size  string `json:"size,omitempty"`
	Crop  string `json:"crop,omitempty"`
	Paper string `json:"paper,omitempty"`
	Cut   string `json:"cut,omitempty"`
	// Credits is the credits-page checkbox; nil means the default (on).
	Credits *bool  `json:"credits,omitempty"`
	Header  string `json:"header,omitempty"`
	Font    string `json:"font,omitempty"`
	// ColourIn is the user's colour-in choice for fonts that offer it.
	ColourIn bool `json:"colourIn,omitempty"`
}

// Trimmed returns p with surrounding whitespace removed and the header
// cut to MaxTextLen characters.
func (p PrintPrefs) Trimmed() PrintPrefs {
	p.Format = strings.TrimSpace(p.Format)
	p.Media = strings.TrimSpace(p.Media)
	p.Size = strings.TrimSpace(p.Size)
	p.Crop = strings.TrimSpace(p.Crop)
	p.Paper = strings.TrimSpace(p.Paper)
	p.Cut = strings.TrimSpace(p.Cut)
	p.Header = capText(p.Header)
	p.Font = strings.TrimSpace(p.Font)
	if p.Credits != nil {
		// Copy, so the stored settings never share the caller's value.
		c := *p.Credits
		p.Credits = &c
	}
	return p
}

// PagesPrefs are the last kind and shared options picked in the Make
// pages tab; "" means the default.
type PagesPrefs struct {
	Kind     string `json:"kind,omitempty"`
	Paper    string `json:"paper,omitempty"`
	Cut      string `json:"cut,omitempty"`
	Title    string `json:"title,omitempty"`
	Font     string `json:"font,omitempty"`
	ColourIn bool   `json:"colourIn,omitempty"`
}

// Trimmed returns p with surrounding whitespace removed and the title cut
// to MaxTextLen characters.
func (p PagesPrefs) Trimmed() PagesPrefs {
	p.Kind = strings.TrimSpace(p.Kind)
	p.Paper = strings.TrimSpace(p.Paper)
	p.Cut = strings.TrimSpace(p.Cut)
	p.Title = capText(p.Title)
	p.Font = strings.TrimSpace(p.Font)
	return p
}

// capText trims s and keeps at most MaxTextLen characters.
func capText(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > MaxTextLen {
		s = strings.TrimSpace(string(r[:MaxTextLen]))
	}
	return s
}

// Trimmed returns s with surrounding whitespace removed from every value.
// Language and Theme are also lower-cased, and an unsupported one becomes
// "" (follow the system).
func (s Settings) Trimmed() Settings {
	s.TMDBAPIKey = strings.TrimSpace(s.TMDBAPIKey)
	s.IGDBClientID = strings.TrimSpace(s.IGDBClientID)
	s.IGDBClientSecret = strings.TrimSpace(s.IGDBClientSecret)
	s.BHLAPIKey = strings.TrimSpace(s.BHLAPIKey)
	s.BGGToken = strings.TrimSpace(s.BGGToken)
	switch lang := strings.ToLower(strings.TrimSpace(s.Language)); lang {
	case "en", "es":
		s.Language = lang
	default:
		s.Language = ""
	}
	s.Print = s.Print.Trimmed()
	s.Pages = s.Pages.Trimmed()
	switch theme := strings.ToLower(strings.TrimSpace(s.Theme)); theme {
	case "light", "dark":
		s.Theme = theme
	default:
		s.Theme = ""
	}
	s.View = s.View.Trimmed()
	return s
}

const (
	// dirName is the settings folder under the user config directory.
	dirName = "bujo-helper"
	// legacyDirName is the folder used before the app was renamed from
	// Cover Downloader to BuJo Helper.
	legacyDirName = "cover-downloader"
)

// Path returns the settings file location:
// <user config dir>/bujo-helper/settings.json.
func Path() (string, error) {
	return pathIn(dirName)
}

// LegacyPath returns where the app kept its settings before the rename:
// <user config dir>/cover-downloader/settings.json.
func LegacyPath() (string, error) {
	return pathIn(legacyDirName)
}

func pathIn(name string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("settings: %w", err)
	}
	return filepath.Join(dir, name, "settings.json"), nil
}

// ErrLegacySkipped reports that Migrate found a legacy settings file it
// could not read or parse, and skipped it. It is not fatal: callers log it
// and carry on without the old settings.
var ErrLegacySkipped = errors.New("migrate settings: skipped legacy file")

// Migrate copies the settings at legacy to path, once. It does nothing
// when path already exists, so saved settings are never overwritten. A
// missing legacy file is not an error. A legacy file that is unreadable or
// not valid settings JSON is skipped, and Migrate returns an error wrapping
// ErrLegacySkipped and the cause, so callers can warn about it. The legacy
// file is always left in place. Any other error is a failure to write path.
func Migrate(legacy, path string) error {
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		// The new file exists, or its state is unknown: leave it alone.
		return nil
	}
	b, err := os.ReadFile(legacy)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w %s: %w", ErrLegacySkipped, legacy, err)
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w %s: %w", ErrLegacySkipped, legacy, err)
	}
	if err := Save(path, s); err != nil {
		return fmt.Errorf("migrate settings: %w", err)
	}
	return nil
}

// Load reads the settings at path. A missing file gives zero Settings and
// no error.
func Load(path string) (Settings, error) {
	var s Settings
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, fmt.Errorf("read settings %s: %w", filepath.Base(path), err)
	}
	return s, nil
}

// Save writes s to path, readable only by the user. It writes a temporary
// file in the same directory and renames it, so a failure never leaves a
// partial file behind. Surrounding whitespace is trimmed from every value.
func Save(path string, s Settings) error {
	s = s.Trimmed()
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	b = append(b, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	// CreateTemp makes the file with mode 0600.
	tmp, err := os.CreateTemp(dir, ".settings-*.json.tmp")
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(b); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	// Flush to disk before the rename, so a crash cannot leave an empty
	// settings file in place of the old one.
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	ok = true
	return nil
}
