package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bujo-helper/internal/gallery"
	"bujo-helper/internal/media"
	"bujo-helper/internal/sheet"

	"github.com/go-pdf/fpdf"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// pinPDFOutput fixes the creation and modification dates fpdf stamps on new
// documents, and the order of their resource catalogs (map order, which
// varies for documents with text fonts), so two renders of the same options
// give the same bytes.
func pinPDFOutput(t *testing.T) {
	t.Helper()
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fpdf.SetDefaultCreationDate(when)
	fpdf.SetDefaultModificationDate(when)
	fpdf.SetDefaultCatalogSort(true)
	t.Cleanup(func() {
		fpdf.SetDefaultCreationDate(time.Time{})
		fpdf.SetDefaultModificationDate(time.Time{})
		fpdf.SetDefaultCatalogSort(false)
	})
}

// fetch GETs url from the app's asset handler and returns the status, the
// content type and the body.
func fetch(t *testing.T, a *App, url string) (int, string, []byte) {
	t.Helper()
	rec := httptest.NewRecorder()
	a.assetHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	res := rec.Result()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, res.Header.Get("Content-Type"), data
}

// exportBytes saves through export, with the save dialog answering a file
// in a temporary folder, and returns the written bytes.
func exportBytes(t *testing.T, export func() (string, error)) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.pdf")
	stubDialogs(t, func(runtime.SaveDialogOptions) (string, error) { return out, nil }, nil)
	path, err := export()
	if err != nil || path != out {
		t.Fatalf("export = %q, %v; want %q", path, err, out)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPreviewPageMatchesExport(t *testing.T) {
	pinPDFOutput(t)
	tests := []struct {
		name    string
		page    sheet.PageSettings
		options string
	}{
		{"defaults", sheet.PageSettings{}, ""},
		{"titled A5 page", sheet.PageSettings{Paper: "a5", Title: "Ideas"}, `{"spacing":4}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(media.NewRegistry(), http.DefaultClient)
			want := exportBytes(t, func() (string, error) {
				return a.ExportPage("dotgrid", "", tt.page, tt.options)
			})
			info, err := a.PreviewPage("dotgrid", tt.page, tt.options)
			if err != nil {
				t.Fatal(err)
			}
			if info.Pages != 1 {
				t.Errorf("Pages = %d, want 1", info.Pages)
			}
			status, ct, got := fetch(t, a, info.URL)
			if status != http.StatusOK || ct != "application/pdf" {
				t.Fatalf("GET %s: %d %q, want 200 application/pdf", info.URL, status, ct)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("preview (%d bytes) differs from the export (%d bytes)", len(got), len(want))
			}
		})
	}
}

func TestPreviewPDFMatchesExport(t *testing.T) {
	pinPDFOutput(t)
	src := t.TempDir()
	var covers []string
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"} {
		covers = append(covers, writeCover(t, src, name, 66, 100))
	}
	tests := []struct {
		name  string
		lang  string
		opts  sheet.Options
		pages int
	}{
		{"default cover sheet", "en", sheet.Options{}, 1},
		{"large covers on two pages", "en", sheet.Options{Size: "large", Paper: "a5"}, 2},
		// Stubs print labels in the UI language.
		{"Spanish ticket stubs", "es", sheet.Options{Format: "stubs"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(media.NewRegistry(), http.DefaultClient)
			a.SetLanguage(tt.lang)
			want := exportBytes(t, func() (string, error) {
				return a.ExportPDF(covers, "", tt.opts)
			})
			info, err := a.PreviewPDF(covers, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if info.Pages != tt.pages {
				t.Errorf("Pages = %d, want %d", info.Pages, tt.pages)
			}
			if count, err := a.PrintPages(covers, tt.opts); err != nil || count.Pages != info.Pages {
				t.Errorf("PrintPages = %+v, %v; want the preview's %d pages", count, err, info.Pages)
			}
			status, _, got := fetch(t, a, info.URL)
			if status != http.StatusOK {
				t.Fatalf("GET %s: %d, want 200", info.URL, status)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("preview (%d bytes) differs from the export (%d bytes)", len(got), len(want))
			}
		})
	}
}

func TestPreviewPageRejectsBadSettings(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		page    sheet.PageSettings
		options string
		want    string
	}{
		{"unknown kind", "starmap", sheet.PageSettings{}, "", `"starmap"`},
		{"unknown paper", "dotgrid", sheet.PageSettings{Paper: "a3"}, "", `"a3"`},
		{"unknown cut", "dotgrid", sheet.PageSettings{Cut: "zigzag"}, "", `"zigzag"`},
		{"unknown option", "dotgrid", sheet.PageSettings{}, `{"dots":"round"}`, `"dots"`},
		{"bad spacing", "dotgrid", sheet.PageSettings{}, `{"spacing":8}`, "8"},
		{"not JSON", "dotgrid", sheet.PageSettings{}, `{`, "page options"},
		{"unknown country", "map", sheet.PageSettings{}, `{"countries":["Atlantis"]}`, `"Atlantis"`},
		{"unknown map region", "map", sheet.PageSettings{}, `{"region":"mars"}`, `"mars"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(media.NewRegistry(), http.DefaultClient)
			info, err := a.PreviewPage(tt.kind, tt.page, tt.options)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want an error naming %s", err, tt.want)
			}
			if info != (PreviewInfo{}) || a.previews.Len() != 0 {
				t.Errorf("a failed preview gave %+v and stored %d renders", info, a.previews.Len())
			}
		})
	}
}

func TestPreviewPDFRejectsBadInput(t *testing.T) {
	src := t.TempDir()
	cover := writeCover(t, src, "c.jpg", 66, 100)
	bad := filepath.Join(src, "bad.jpg")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		paths []string
		opts  sheet.Options
		want  string
	}{
		{"no covers", nil, sheet.Options{}, "select at least one cover"},
		{"unknown paper", []string{cover}, sheet.Options{Paper: "a3"}, `"a3"`},
		{"unknown format", []string{cover}, sheet.Options{Format: "posters"}, `"posters"`},
		{"unreadable image", []string{cover, bad}, sheet.Options{}, "bad.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newApp(media.NewRegistry(), http.DefaultClient)
			info, err := a.PreviewPDF(tt.paths, tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want an error containing %s", err, tt.want)
			}
			if info != (PreviewInfo{}) || a.previews.Len() != 0 {
				t.Errorf("a failed preview gave %+v and stored %d renders", info, a.previews.Len())
			}
		})
	}
}

func TestAssetHandlerRoutesThumbsAndPreviews(t *testing.T) {
	a := newApp(media.NewRegistry(), http.DefaultClient)
	cover := writeCover(t, t.TempDir(), "c.jpg", 66, 100)
	a.thumbs.Allow([]gallery.Cover{{Path: cover}})
	info, err := a.PreviewPage("dotgrid", sheet.PageSettings{}, "")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, url string
		status    int
		ct        string
	}{
		{"thumbnail", gallery.URL(cover), http.StatusOK, "image/jpeg"},
		{"preview", info.URL, http.StatusOK, "application/pdf"},
		{"unknown preview", "/preview?id=99", http.StatusNotFound, ""},
		{"other path", "/other", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ct, _ := fetch(t, a, tt.url)
			if status != tt.status || tt.ct != "" && ct != tt.ct {
				t.Errorf("GET %s: %d %q, want %d %q", tt.url, status, ct, tt.status, tt.ct)
			}
		})
	}
}
