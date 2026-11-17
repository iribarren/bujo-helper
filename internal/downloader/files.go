package downloader

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"bujo-helper/internal/media"
)

// maxNameRunes caps each part (title, creator) of a file name.
const maxNameRunes = 120

// maxDuplicates bounds the "-N" suffix search when names collide.
const maxDuplicates = 10000

// windowsReserved are device names Windows refuses as file names.
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// imageExts maps image MIME types to file extensions.
var imageExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/avif": ".avif",
	"image/bmp":  ".bmp",
	"image/tiff": ".tiff",
}

// knownExts are URL extensions trusted as-is.
var knownExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".avif": true, ".bmp": true, ".tif": true, ".tiff": true,
}

// safeName makes s usable as part of a file name on Windows and Linux:
// reserved characters and control characters are removed, whitespace is
// collapsed, trailing dots and spaces are trimmed and the length is capped.
func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case r == utf8.RuneError, unicode.IsControl(r), strings.ContainsRune(`<>:"/\|?*`, r):
			// drop
		default:
			b.WriteRune(r)
		}
	}
	name := strings.Join(strings.Fields(b.String()), " ")
	if runes := []rune(name); len(runes) > maxNameRunes {
		name = string(runes[:maxNameRunes])
	}
	return strings.TrimRight(name, ". ")
}

// fileName builds "Title - Creator.ext", or "Title.ext" without a creator.
func fileName(item media.Item, ext string) string {
	stem := safeName(item.Title)
	if stem == "" {
		stem = "cover"
	}
	if creator := safeName(item.Creator); creator != "" {
		stem += " - " + creator
	}
	if windowsReserved[strings.ToUpper(stem)] {
		stem += "_"
	}
	return stem + ext
}

// imageExt picks the extension from the URL path when it is a known image
// extension, else from the Content-Type, else ".jpg".
func imageExt(rawURL, contentType string) string {
	if u, err := url.Parse(rawURL); err == nil {
		ext := strings.ToLower(path.Ext(u.Path))
		if ext == ".jpeg" {
			ext = ".jpg"
		}
		if knownExts[ext] {
			return ext
		}
	}
	mediaType, _, _ := strings.Cut(contentType, ";")
	if ext, ok := imageExts[strings.ToLower(strings.TrimSpace(mediaType))]; ok {
		return ext
	}
	return ".jpg"
}

// saveUnique writes data to dir/name, or to "stem-2.ext", "stem-3.ext", ...
// when the name is taken. O_EXCL makes the choice safe across goroutines.
func saveUnique(dir, name string, data []byte) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 1; n <= maxDuplicates; n++ {
		candidate := name
		if n > 1 {
			candidate = fmt.Sprintf("%s-%d%s", stem, n, ext)
		}
		p := filepath.Join(dir, candidate)
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create file: %w", err)
		}
		_, werr := f.Write(data)
		cerr := f.Close()
		if err := errors.Join(werr, cerr); err != nil {
			os.Remove(p)
			return "", fmt.Errorf("write file: %w", err)
		}
		return p, nil
	}
	return "", fmt.Errorf("too many files named %q", name)
}

// CreditSuffix ends the name of an image's credit sidecar (see
// media.CreditSuffix).
const CreditSuffix = media.CreditSuffix

// CreditPath returns the path of the credit sidecar of the image at path
// (see media.CreditPath).
func CreditPath(imagePath string) string {
	return media.CreditPath(imagePath)
}

// saveCredit writes credit to the sidecar of the image at imagePath,
// replacing a stale sidecar left by an image deleted outside the app.
func saveCredit(imagePath, credit string) error {
	if err := os.WriteFile(CreditPath(imagePath), []byte(credit+"\n"), 0o644); err != nil {
		return fmt.Errorf("write credit: %w", err)
	}
	return nil
}
