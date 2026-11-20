// Package fonts serves the lettering fonts and their licences to the
// webview, so the font picker can show each font in its own face without
// copying the TTFs into the frontend bundle.
package fonts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"bujo-helper/internal/sheet"
)

// Path is the URL path prefix the handler answers.
const Path = "/fonts/"

// TTFURL returns the URL of the face headers use with the font id.
func TTFURL(id string) string { return Path + id + ".ttf" }

// LicenceURL returns the URL of the licence text of the font id.
func LicenceURL(id string) string { return Path + id + "/licence" }

// Handler serves GET /fonts/<id>.ttf (the font's header face, as font/ttf)
// and GET /fonts/<id>/licence (its licence, as plain text) from the
// embedded fonts. Unknown ids and any other path are 404. The fonts never
// change while the app runs, so responses are cacheable; a content-hash
// ETag lets the webview revalidate once max-age runs out, so an upgrade
// that changes a font or licence is picked up.
type Handler struct{}

func (Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body []byte
	var contentType string
	if id, ok := strings.CutSuffix(rest, ".ttf"); ok {
		if f, found := sheet.LetteringFontByID(id); found {
			body, contentType = f.HeaderTTF(), "font/ttf"
		}
	} else if id, ok := strings.CutSuffix(rest, "/licence"); ok {
		if f, found := sheet.LetteringFontByID(id); found {
			body, contentType = []byte(f.LicenceText), "text/plain; charset=utf-8"
		}
	}
	// Ids hold no "/" or ".", so an exact id match rules out any other
	// path; body stays nil for anything unknown.
	if body == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Header().Set("ETag", etag(body))
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
}

// etag is a strong entity tag for body: a prefix of its SHA-256.
func etag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}
