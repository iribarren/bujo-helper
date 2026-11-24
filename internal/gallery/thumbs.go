package gallery

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/image/draw"
)

// ThumbPath is the URL path the thumbnail handler answers.
const ThumbPath = "/thumbs"

// ThumbHeight is the height of a thumbnail in pixels. Smaller images are
// not upscaled.
const ThumbHeight = 240

const (
	thumbQuality = 80
	maxDecodes   = 4 // concurrent full-size decodes, to bound memory
)

// defaultMaxThumbPixels is the largest image, in pixels, that gets a
// thumbnail: decoding takes about 4 bytes a pixel, so a larger one (or a
// file whose header claims one) is refused before it is decoded.
const defaultMaxThumbPixels = 50_000_000

// URL returns the thumbnail URL of the image at path.
func URL(path string) string {
	return ThumbPath + "?path=" + url.QueryEscape(path)
}

// Thumbs is an http.Handler serving GET /thumbs?path=<path> as a small
// JPEG. It only serves the paths of the most recent Allow call, so the UI
// cannot read arbitrary files. Anything else is 404. Thumbnails are cached
// in memory until the file changes or leaves the allowed list. It is safe
// for concurrent use.
type Thumbs struct {
	mu      sync.Mutex
	allowed map[string]bool
	cache   map[string]thumb

	decodes chan struct{}
	// maxPixels is the largest image that gets a thumbnail
	// (defaultMaxThumbPixels; tests lower it).
	maxPixels int64
}

type thumb struct {
	modTime time.Time
	size    int64
	data    []byte
}

// NewThumbs returns a handler with nothing allowed yet.
func NewThumbs() *Thumbs {
	return &Thumbs{
		allowed:   map[string]bool{},
		cache:     map[string]thumb{},
		decodes:   make(chan struct{}, maxDecodes),
		maxPixels: defaultMaxThumbPixels,
	}
}

// Allow adds the paths of covers to the allowed set. Earlier listings stay
// allowed for the session: a slower, older listing may reach the UI after a
// newer one, and its thumbnails must still load. Only listed images are
// ever served.
func (t *Thumbs) Allow(covers []Cover) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, c := range covers {
		t.allowed[c.Path] = true
	}
}

func (t *Thumbs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != ThumbPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// The path must match a listed path exactly: no cleaning, so no
	// "../" spelling can reach another file.
	path := r.URL.Query().Get("path")
	if !t.isAllowed(path) || !isImageName(path) {
		http.NotFound(w, r)
		return
	}
	data, ok := t.thumbnail(path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

func (t *Thumbs) isAllowed(path string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return path != "" && t.allowed[path]
}

// thumbnail returns the cached thumbnail of path, making it if the file is
// new or has changed since.
func (t *Thumbs) thumbnail(path string) ([]byte, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	t.mu.Lock()
	c, ok := t.cache[path]
	t.mu.Unlock()
	if ok && c.modTime.Equal(info.ModTime()) && c.size == info.Size() {
		return c.data, true
	}

	t.decodes <- struct{}{}
	data, err := makeThumb(path, t.maxPixels)
	<-t.decodes
	if err != nil {
		return nil, false
	}

	t.mu.Lock()
	// Cache only if the path is still allowed; Allow may have run meanwhile.
	if t.allowed[path] {
		t.cache[path] = thumb{modTime: info.ModTime(), size: info.Size(), data: data}
	}
	t.mu.Unlock()
	return data, true
}

// makeThumb decodes the image at path and returns it as a JPEG at most
// ThumbHeight pixels tall, with transparency flattened onto white. An
// image over maxPixels is an error, read from its header alone.
func makeThumb(path string, maxPixels int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, fmt.Errorf("%d x %d image is over %d pixels", cfg.Width, cfg.Height, maxPixels)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	if h > ThumbHeight {
		w = max(1, w*ThumbHeight/h)
		h = ThumbHeight
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.BiLinear.Scale(dst, dst.Bounds(), src, sb, draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
