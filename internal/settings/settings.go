// Package settings stores the user's app settings (such as API keys) in a
// private JSON file under the user config directory.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
}

// Trimmed returns s with surrounding whitespace removed from every value.
func (s Settings) Trimmed() Settings {
	s.TMDBAPIKey = strings.TrimSpace(s.TMDBAPIKey)
	s.IGDBClientID = strings.TrimSpace(s.IGDBClientID)
	s.IGDBClientSecret = strings.TrimSpace(s.IGDBClientSecret)
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
