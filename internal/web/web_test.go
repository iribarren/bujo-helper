package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetRejectsBodiesOverTheCap(t *testing.T) {
	old := MaxBodySize
	MaxBodySize = 10
	defer func() { MaxBodySize = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 11)))
	}))
	defer srv.Close()

	_, err := Get(context.Background(), srv.Client(), srv.URL, nil)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("err = %v, want ErrBodyTooLarge", err)
	}
}

func TestGetAcceptsBodiesAtTheCap(t *testing.T) {
	old := MaxBodySize
	MaxBodySize = 10
	defer func() { MaxBodySize = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 10)))
	}))
	defer srv.Close()

	resp, err := Get(context.Background(), srv.Client(), srv.URL, nil)
	if err != nil || len(resp.Body) != 10 {
		t.Fatalf("resp=%v err=%v, want 10-byte body", resp, err)
	}
}

func TestContactUserAgent(t *testing.T) {
	if !strings.Contains(ContactUserAgent, "https://github.com/") {
		t.Errorf("ContactUserAgent = %q, want a contact URL", ContactUserAgent)
	}
	if MusicBrainzUserAgent != ContactUserAgent {
		t.Errorf("MusicBrainzUserAgent = %q, want the contact UA", MusicBrainzUserAgent)
	}
}

func TestSetWikimediaHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://www.wikidata.org/w/api.php", nil)
	SetWikimediaHeaders(req)
	if ua := req.Header.Get("User-Agent"); ua != ContactUserAgent {
		t.Errorf("User-Agent = %q, want %q", ua, ContactUserAgent)
	}
}

// Wikimedia's media servers get the contact UA their robot policy requires,
// even on the downloader's browser-style image requests.
func TestSetBrowserHeadersUsesTheContactUAForWikimediaMedia(t *testing.T) {
	for url, want := range map[string]string{
		"https://upload.wikimedia.org/wikipedia/commons/a/a8/X.jpg":                   ContactUserAgent,
		"https://thumb.wikimedia.org/wikipedia/commons/thumb/a/a8/X.jpg/1920px-X.jpg": ContactUserAgent,
		"https://is1-ssl.mzstatic.com/image/thumb/x/9999x9999bb.jpg":                  BrowserUserAgent,
		"https://upload.wikimedia.org.example.com/x.jpg":                              BrowserUserAgent,
	} {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		SetBrowserHeaders(req)
		if ua := req.Header.Get("User-Agent"); ua != want {
			t.Errorf("%s: User-Agent = %q, want %q", url, ua, want)
		}
		if req.Header.Get("Accept") == "" {
			t.Errorf("%s: no Accept header", url)
		}
	}
}

func TestSetBHLHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://www.biodiversitylibrary.org/api3", nil)
	SetBHLHeaders(req)
	if got := req.Header.Get("User-Agent"); got != ContactUserAgent {
		t.Errorf("User-Agent = %q, want the contact UA", got)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want application/json", got)
	}
}
