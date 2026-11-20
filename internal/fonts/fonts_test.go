package fonts

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"bujo-helper/internal/sheet"
)

func serve(method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	Handler{}.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestServesEveryFontTTF(t *testing.T) {
	for _, f := range sheet.LetteringFonts {
		t.Run(f.ID, func(t *testing.T) {
			rec := serve(http.MethodGet, TTFURL(f.ID))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "font/ttf" {
				t.Errorf("content type = %q, want font/ttf", got)
			}
			if got := rec.Header().Get("Cache-Control"); got == "" || got == "no-store" || got == "no-cache" {
				t.Errorf("cache control = %q, want cacheable", got)
			}
			if !bytes.Equal(rec.Body.Bytes(), f.HeaderTTF()) {
				t.Error("body is not the font's header TTF")
			}
		})
	}
}

func TestServesEveryLicence(t *testing.T) {
	for _, f := range sheet.LetteringFonts {
		t.Run(f.ID, func(t *testing.T) {
			rec := serve(http.MethodGet, LicenceURL(f.ID))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
				t.Errorf("content type = %q", got)
			}
			if rec.Body.String() != f.LicenceText {
				t.Error("body is not the font's licence text")
			}
		})
	}
}

func TestURLs(t *testing.T) {
	if got := TTFURL("pacifico"); got != "/fonts/pacifico.ttf" {
		t.Errorf("TTFURL = %q", got)
	}
	if got := LicenceURL("pacifico"); got != "/fonts/pacifico/licence" {
		t.Errorf("LicenceURL = %q", got)
	}
}

func TestHeadServesNoBody(t *testing.T) {
	rec := serve(http.MethodHead, "/fonts/bungee.ttf")
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("HEAD: status %d, %d body bytes", rec.Code, rec.Body.Len())
	}
}

func TestRejectsOtherRequests(t *testing.T) {
	tests := []struct {
		name, method, target string
		want                 int
	}{
		{"unknown font", http.MethodGet, "/fonts/comic-sans.ttf", http.StatusNotFound},
		{"unknown licence", http.MethodGet, "/fonts/comic-sans/licence", http.StatusNotFound},
		{"empty id", http.MethodGet, "/fonts/.ttf", http.StatusNotFound},
		{"no extension", http.MethodGet, "/fonts/pacifico", http.StatusNotFound},
		{"wrong extension", http.MethodGet, "/fonts/pacifico.otf", http.StatusNotFound},
		{"directory", http.MethodGet, "/fonts/", http.StatusNotFound},
		{"raw embedded file", http.MethodGet, "/fonts/pacifico/OFL.txt", http.StatusNotFound},
		{"nested path", http.MethodGet, "/fonts/pacifico/licence/x", http.StatusNotFound},
		{"traversal", http.MethodGet, "/fonts/../pacifico.ttf", http.StatusNotFound},
		{"case matters", http.MethodGet, "/fonts/Pacifico.ttf", http.StatusNotFound},
		{"outside the prefix", http.MethodGet, "/other/pacifico.ttf", http.StatusNotFound},
		{"post", http.MethodPost, "/fonts/pacifico.ttf", http.StatusMethodNotAllowed},
		{"delete licence", http.MethodDelete, "/fonts/pacifico/licence", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.method, tt.target)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusMethodNotAllowed && rec.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("Allow = %q", rec.Header().Get("Allow"))
			}
		})
	}
}

// Responses carry a content-hash ETag, different per body, so a cached font
// or licence revalidates after an upgrade changes it.
func TestETagRevalidates(t *testing.T) {
	seen := map[string]string{}
	for _, f := range sheet.LetteringFonts {
		for _, target := range []string{TTFURL(f.ID), LicenceURL(f.ID)} {
			rec := serve(http.MethodGet, target)
			etag := rec.Header().Get("ETag")
			if len(etag) < 3 || etag[0] != '"' || etag[len(etag)-1] != '"' {
				t.Errorf("%s: ETag %q, want a quoted tag", target, etag)
				continue
			}
			if other, dup := seen[etag]; dup && !bytes.Equal(serve(http.MethodGet, other).Body.Bytes(), rec.Body.Bytes()) {
				t.Errorf("%s and %s share ETag %s with different bodies", target, other, etag)
			}
			seen[etag] = target
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.Header.Set("If-None-Match", etag)
			cached := httptest.NewRecorder()
			Handler{}.ServeHTTP(cached, req)
			if cached.Code != http.StatusNotModified {
				t.Errorf("%s: If-None-Match gave %d, want 304", target, cached.Code)
			}
		}
	}
}
