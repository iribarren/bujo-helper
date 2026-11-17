package downloader

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "golang.org/x/image/webp"

	"bujo-helper/internal/media"
)

// webpServer serves the WebP fixtures: a lossy opaque plate (made with
// cwebp from a 96×64 plasma) and a lossless 48×32 image with alpha.
func webpServer(t *testing.T) *httptest.Server {
	t.Helper()
	plate := readFile(t, filepath.Join("testdata", "plate.webp"))
	alpha := readFile(t, filepath.Join("testdata", "alpha.webp"))
	broken := append([]byte("RIFF\x00\x10\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0x11}, 2048)...)
	mux := http.NewServeMux()
	serve := func(path, contentType string, body []byte) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.Write(body)
		})
	}
	// BHL's /pageimage/<id> redirects to an S3 ".webp" URL.
	mux.HandleFunc("/pageimage/42", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/s3/page_0042_full.webp", http.StatusFound)
	})
	serve("/s3/page_0042_full.webp", "image/webp", plate)
	serve("/noext", "image/webp", plate)
	// A WebP sent with a generic image type is still converted.
	serve("/sniffed", "image/x-unknown", plate)
	serve("/alpha.webp", "image/webp", alpha)
	serve("/broken.webp", "image/webp", broken)
	serve("/cover.jpg", "image/jpeg", jpegBody)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestWebPIsSavedAsJPEGOrPNG(t *testing.T) {
	srv := webpServer(t)
	tests := []struct {
		name, path         string
		wantExt, wantFmt   string
		wantW, wantH       int
		wantTranslucentPix bool
	}{
		{"redirect to an S3 WebP", "/pageimage/42", ".jpg", "jpeg", 96, 64, false},
		{"WebP without an extension", "/noext", ".jpg", "jpeg", 96, 64, false},
		{"WebP sniffed from the bytes", "/sniffed", ".jpg", "jpeg", 96, 64, false},
		{"WebP with alpha keeps it as PNG", "/alpha.webp", ".png", "png", 48, 32, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDownloader(srv, returns("p", srv.URL+tt.path))
			dir := t.TempDir()
			res, _ := run(t, d, context.Background(), Request{
				MediaType: "books",
				Items:     []media.Item{{Title: "Robin"}},
				OutputDir: dir,
			})
			r := res[0]
			if r.Status != StatusOK {
				t.Fatalf("result = %+v", r)
			}
			if want := filepath.Join(dir, "Robin"+tt.wantExt); r.Path != want {
				t.Errorf("Path = %q, want %q", r.Path, want)
			}
			data := readFile(t, r.Path)
			img, format, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("saved image does not decode: %v", err)
			}
			if format != tt.wantFmt {
				t.Errorf("saved format = %q, want %q", format, tt.wantFmt)
			}
			if b := img.Bounds(); b.Dx() != tt.wantW || b.Dy() != tt.wantH {
				t.Errorf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), tt.wantW, tt.wantH)
			}
			if got := hasTranslucentPixel(img); got != tt.wantTranslucentPix {
				t.Errorf("translucent pixels = %v, want %v", got, tt.wantTranslucentPix)
			}
		})
	}
}

func hasTranslucentPixel(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return true
			}
		}
	}
	return false
}

func TestBrokenWebPFallsBackToTheNextURL(t *testing.T) {
	srv := webpServer(t)
	d := newDownloader(srv, returns("p", srv.URL+"/broken.webp", srv.URL+"/cover.jpg"))
	dir := t.TempDir()
	res, _ := run(t, d, context.Background(), Request{
		MediaType: "books",
		Items:     []media.Item{{Title: "Robin"}},
		OutputDir: dir,
	})
	if r := res[0]; r.Status != StatusOK || r.Path != filepath.Join(dir, "Robin.jpg") || !bytes.Equal(readFile(t, r.Path), jpegBody) {
		t.Fatalf("result = %+v, want the JPEG from the second URL", r)
	}

	d = newDownloader(srv, returns("p", srv.URL+"/broken.webp"))
	res, _ = run(t, d, context.Background(), Request{
		MediaType: "books",
		Items:     []media.Item{{Title: "Robin"}},
		OutputDir: t.TempDir(),
	})
	if r := res[0]; r.Status != StatusFailed || !strings.Contains(r.Error, "WebP") {
		t.Errorf("result = %+v, want a WebP decode failure", r)
	}
}

func TestConvertWebP(t *testing.T) {
	plate := readFile(t, filepath.Join("testdata", "plate.webp"))
	out, ext, err := convertWebP(plate)
	if err != nil || ext != ".jpg" {
		t.Fatalf("convertWebP = %d bytes, %q, %v", len(out), ext, err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("not a JPEG: %v", err)
	}
	alpha := readFile(t, filepath.Join("testdata", "alpha.webp"))
	out, ext, err = convertWebP(alpha)
	if err != nil || ext != ".png" {
		t.Fatalf("convertWebP(alpha) = %d bytes, %q, %v", len(out), ext, err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("not a PNG: %v", err)
	}
	if !isWebP(plate) || isWebP(jpegBody) || isWebP([]byte("RIFF")) {
		t.Error("isWebP misdetects")
	}
}
