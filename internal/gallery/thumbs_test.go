package gallery

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// pngHeader returns a PNG that holds only a header claiming w x h pixels:
// enough for image.DecodeConfig, no pixel data.
func pngHeader(w, h uint32) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(typ string, data []byte) {
		binary.Write(&b, binary.BigEndian, uint32(len(data)))
		body := append([]byte(typ), data...)
		b.Write(body)
		binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(body))
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	chunk("IHDR", ihdr)
	chunk("IEND", nil)
	return b.Bytes()
}

// Images over maxThumbPixels get no thumbnail, like unreadable files;
// those at the limit still do.
func TestThumbsRefuseOversizedImages(t *testing.T) {
	orig := maxThumbPixels
	maxThumbPixels = 100 * 100
	t.Cleanup(func() { maxThumbPixels = orig })

	dir := t.TempDir()
	writeImage(t, dir, "at-limit.png", 100, 100)
	writeImage(t, dir, "wide.jpg", 101, 100)
	writeImage(t, dir, "tall.png", 50, 201)
	covers, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	th := NewThumbs()
	th.Allow(covers)
	want := map[string]int{"at-limit.png": http.StatusOK, "wide.jpg": http.StatusNotFound, "tall.png": http.StatusNotFound}
	for _, c := range covers {
		if rec := get(th, c.Path); rec.Code != want[c.Name] {
			t.Errorf("%s (%d x %d): status %d, want %d", c.Name, c.Width, c.Height, rec.Code, want[c.Name])
		}
	}
}

// A file whose header claims a huge image is refused from the header
// alone, with the real limit.
func TestThumbsRefuseHugeHeaders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.png")
	if err := os.WriteFile(path, pngHeader(100_000, 100_000), 0o644); err != nil {
		t.Fatal(err)
	}
	covers, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(covers) != 1 {
		t.Fatalf("listed %d covers, want the huge one", len(covers))
	}
	th := NewThumbs()
	th.Allow(covers)
	if rec := get(th, path); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}
