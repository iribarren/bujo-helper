// Package preview keeps rendered PDFs in memory and serves them to the
// webview, so the UI can draw a live preview without a temporary file.
package preview

import (
	"bytes"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Path is the URL path the store answers.
const Path = "/preview"

// Keep is how many renders the store holds; older ones are dropped. A few
// let a preview that is still loading finish after a newer render arrives.
const Keep = 4

// Store is an http.Handler serving GET /preview?id=<id> as a PDF put there
// by Put. It serves only the last Keep renders; anything else is 404. It is
// safe for concurrent use.
type Store struct {
	mu      sync.Mutex
	last    uint64 // id of the newest render; ids start at 1
	renders map[uint64][]byte
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{renders: map[uint64][]byte{}}
}

// Put stores a rendered PDF and returns its URL. It drops the oldest render
// when the store already holds Keep. The store keeps data as is: the caller
// must not change it afterwards.
func (s *Store) Put(data []byte) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last++
	s.renders[s.last] = data
	if s.last > Keep {
		delete(s.renders, s.last-Keep)
	}
	return Path + "?id=" + strconv.FormatUint(s.last, 10)
}

// Len returns how many renders the store holds.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.renders)
}

func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != Path {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	data, ok := s.renders[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// pagesRoot matches the page tree root fpdf writes, capturing its page
// count.
var pagesRoot = regexp.MustCompile(`<</Type /Pages\n/Kids \[[^\]]*\]\n/Count (\d+)\n`)

// PageCount returns the number of pages of a PDF written by fpdf, read from
// its page tree root; 0 if there is none.
func PageCount(pdf []byte) int {
	m := pagesRoot.FindSubmatch(pdf)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0
	}
	return n
}
