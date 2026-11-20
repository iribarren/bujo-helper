package preview

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// get serves a request for target from s and returns the response.
func get(t *testing.T, s http.Handler, method, target string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec.Result()
}

func body(t *testing.T, res *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPutServesTheRender(t *testing.T) {
	s := NewStore()
	url := s.Put([]byte("%PDF-one"))
	if !strings.HasPrefix(url, Path+"?id=") {
		t.Fatalf("Put = %q, want a %s?id= URL", url, Path)
	}
	res := get(t, s, http.MethodGet, url)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", url, res.StatusCode)
	}
	if got := body(t, res); got != "%PDF-one" {
		t.Errorf("body = %q, want the render", got)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	head := get(t, s, http.MethodHead, url)
	if head.StatusCode != http.StatusOK || body(t, head) != "" {
		t.Errorf("HEAD %s: status %d with a body; want 200 and none", url, head.StatusCode)
	}
}

func TestEachPutGetsANewURL(t *testing.T) {
	s := NewStore()
	a, b := s.Put([]byte("a")), s.Put([]byte("b"))
	if a == b {
		t.Fatalf("two renders share the URL %q", a)
	}
	if got := body(t, get(t, s, http.MethodGet, a)); got != "a" {
		t.Errorf("first render = %q, want a", got)
	}
	if got := body(t, get(t, s, http.MethodGet, b)); got != "b" {
		t.Errorf("second render = %q, want b", got)
	}
}

func TestStoreKeepsOnlyTheLastRenders(t *testing.T) {
	s := NewStore()
	var urls []string
	for i := range Keep + 2 {
		urls = append(urls, s.Put([]byte(fmt.Sprint(i))))
	}
	for i, url := range urls {
		res := get(t, s, http.MethodGet, url)
		evicted := i < len(urls)-Keep
		switch {
		case evicted && res.StatusCode != http.StatusNotFound:
			t.Errorf("render %d: status %d, want 404 (evicted)", i, res.StatusCode)
		case !evicted && res.StatusCode != http.StatusOK:
			t.Errorf("render %d: status %d, want 200 (one of the last %d)", i, res.StatusCode, Keep)
		}
	}
	if n := s.Len(); n != Keep {
		t.Errorf("store holds %d renders, want %d", n, Keep)
	}
}

func TestServeRejectsOtherRequests(t *testing.T) {
	s := NewStore()
	url := s.Put([]byte("%PDF"))
	tests := []struct {
		name, method, target string
		status               int
	}{
		{"unknown id", http.MethodGet, Path + "?id=999", http.StatusNotFound},
		{"no id", http.MethodGet, Path, http.StatusNotFound},
		{"bad id", http.MethodGet, Path + "?id=one", http.StatusNotFound},
		{"negative id", http.MethodGet, Path + "?id=-1", http.StatusNotFound},
		{"other path", http.MethodGet, "/previews?id=1", http.StatusNotFound},
		{"sub path", http.MethodGet, Path + "/x?id=1", http.StatusNotFound},
		{"post", http.MethodPost, url, http.StatusMethodNotAllowed},
		{"delete", http.MethodDelete, url, http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := get(t, s, tt.method, tt.target)
			if res.StatusCode != tt.status {
				t.Errorf("%s %s: status %d, want %d", tt.method, tt.target, res.StatusCode, tt.status)
			}
			if tt.status == http.StatusMethodNotAllowed && res.Header.Get("Allow") != "GET, HEAD" {
				t.Errorf("Allow = %q, want GET, HEAD", res.Header.Get("Allow"))
			}
		})
	}
}

func TestStoreIsSafeForConcurrentUse(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			url := s.Put([]byte(fmt.Sprint(i)))
			res := get(t, s, http.MethodGet, url)
			// A concurrent Put may already have evicted it.
			if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
				t.Errorf("status %d", res.StatusCode)
			}
		})
	}
	wg.Wait()
	if n := s.Len(); n != Keep {
		t.Errorf("store holds %d renders, want %d", n, Keep)
	}
}

func TestPageCount(t *testing.T) {
	tests := []struct {
		name string
		pdf  string
		want int
	}{
		{"one page", "%PDF-1.3\n1 0 obj\n<</Type /Pages\n/Kids [3 0 R ]\n/Count 1\n/MediaBox [0 0 595.28 841.89]\n>>\nendobj\n", 1},
		{"three pages", "%PDF-1.3\n1 0 obj\n<</Type /Pages\n/Kids [3 0 R 5 0 R 7 0 R ]\n/Count 3\n>>\nendobj\n", 3},
		{"outline counts are not pages", "%PDF-1.3\n<</Type /Outlines /Count 0>>\n", 0},
		{"not a PDF", "hello", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PageCount([]byte(tt.pdf)); got != tt.want {
				t.Errorf("PageCount = %d, want %d", got, tt.want)
			}
		})
	}
}
