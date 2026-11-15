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
}

// Path returns the settings file location:
// <user config dir>/cover-downloader/settings.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("settings: %w", err)
	}
	return filepath.Join(dir, "cover-downloader", "settings.json"), nil
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
// partial file behind. Surrounding whitespace is trimmed from the key.
func Save(path string, s Settings) error {
	s.TMDBAPIKey = strings.TrimSpace(s.TMDBAPIKey)
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
