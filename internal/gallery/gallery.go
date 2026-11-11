// Package gallery lists the cover images in a folder and serves small
// thumbnails of them to the UI.
package gallery

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	// Decoders for the supported cover formats.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// Cover is one image file in a folder.
type Cover struct {
	Name   string // file name
	Path   string // absolute path
	Width  int    // pixels
	Height int    // pixels
}

// imageExts are the file extensions List considers, lower case.
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// isImageName reports whether name has a supported image extension.
func isImageName(name string) bool {
	return imageExts[strings.ToLower(filepath.Ext(name))]
}

// List returns the images directly inside dir, sorted by name
// (case-insensitive, with numbers in natural order). Files that are not
// images, or that cannot be decoded, are skipped.
func List(dir string) ([]Cover, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("choose a folder")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read folder: %w", err)
	}
	covers := []Cover{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !isImageName(e.Name()) {
			continue
		}
		path := filepath.Join(abs, e.Name())
		w, h, ok := imageSize(path)
		if !ok {
			continue
		}
		covers = append(covers, Cover{Name: e.Name(), Path: path, Width: w, Height: h})
	}
	sort.SliceStable(covers, func(i, j int) bool {
		return lessNatural(covers[i].Name, covers[j].Name)
	})
	return covers, nil
}

// imageSize reads the dimensions from the image header of path.
func imageSize(path string) (w, h int, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// lessNatural orders names case-insensitively, comparing runs of digits by
// their numeric value ("2" before "10"). Names that compare equal that way
// are ordered by their exact text, so the order is total.
func lessNatural(a, b string) bool {
	if c := compareNatural(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c < 0
	}
	return a < b
}

func compareNatural(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			ni, nj := i, j
			for ni < len(ar) && unicode.IsDigit(ar[ni]) {
				ni++
			}
			for nj < len(br) && unicode.IsDigit(br[nj]) {
				nj++
			}
			if c := compareNumbers(string(ar[i:ni]), string(br[j:nj])); c != 0 {
				return c
			}
			i, j = ni, nj
			continue
		}
		if ar[i] != br[j] {
			if ar[i] < br[j] {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case len(ar)-i < len(br)-j:
		return -1
	case len(ar)-i > len(br)-j:
		return 1
	}
	return 0
}

// compareNumbers compares two runs of ASCII digits by value; equal values
// with fewer leading zeros come first.
func compareNumbers(a, b string) int {
	ta, tb := strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	switch {
	case len(ta) != len(tb):
		if len(ta) < len(tb) {
			return -1
		}
		return 1
	case ta != tb:
		if ta < tb {
			return -1
		}
		return 1
	case len(a) != len(b):
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return 0
}
