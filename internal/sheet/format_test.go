package sheet

import (
	"strings"
	"testing"
)

func TestFormatByID(t *testing.T) {
	if DefaultFormat != "covers" || Formats[0].ID != DefaultFormat {
		t.Errorf("DefaultFormat = %q, first format %q; want covers first", DefaultFormat, Formats[0].ID)
	}
	tests := []struct {
		id, want                      string
		usesSize, usesCut, usesHeader bool
	}{
		{"", "covers", true, true, false},
		{"covers", "covers", true, true, false},
		{"cards", "cards", false, false, false},
		{"spines", "spines", true, false, false},
		{"stubs", "stubs", false, false, false},
		{"fullpage", "fullpage", false, false, true},
	}
	for _, tt := range tests {
		f, err := FormatByID(tt.id)
		if err != nil || f.ID != tt.want || f.UsesSize != tt.usesSize || f.UsesCut != tt.usesCut || f.UsesHeader != tt.usesHeader {
			t.Errorf("FormatByID(%q) = %+v, %v; want %s (uses size: %v, uses cut: %v, uses header: %v)", tt.id, f, err, tt.want, tt.usesSize, tt.usesCut, tt.usesHeader)
		}
	}
	if _, err := FormatByID("poster"); err == nil || !strings.Contains(err.Error(), "poster") {
		t.Errorf(`FormatByID("poster"): error %v, want one naming the format`, err)
	}
	if _, err := (Options{Format: "poster"}).Grid(); err == nil || !strings.Contains(err.Error(), "poster") {
		t.Errorf(`Options{Format: "poster"}.Grid(): error %v, want one naming the format`, err)
	}
	if _, err := (Options{Format: "poster"}).PerPage(); err == nil {
		t.Error(`Options{Format: "poster"}.PerPage(): want an error`)
	}
}

func TestFormatsHaveLabelsAndFileSafeTags(t *testing.T) {
	const safe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	seen := map[string]bool{}
	for _, f := range Formats {
		if f.ID == "" || f.Label == "" || f.Noun == "" || seen[f.ID] {
			t.Errorf("format %+v: want a unique ID, a label and a noun", f)
		}
		seen[f.ID] = true
		if f.FileTag == "" || strings.Trim(f.FileTag, safe) != "" {
			t.Errorf("format %q: file tag %q, want letters and digits only", f.ID, f.FileTag)
		}
	}
}

// The covers format, named or not, keeps today's grids.
func TestCoversFormatKeepsTheCoverGrid(t *testing.T) {
	for _, p := range Papers {
		for _, s := range Sizes {
			def, err := Options{Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			covers, err := Options{Format: "covers", Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			want, _ := NewGrid(p.Page, s.CoverHeight)
			if def != want || covers != want {
				t.Errorf("%s %s: grids %+v / %+v, want %+v", p.ID, s.ID, def, covers, want)
			}
		}
	}
}

func TestOptionsPerPage(t *testing.T) {
	tests := []struct {
		opts Options
		want int
	}{
		{Options{}, 25},
		{Options{Size: "mini"}, 81},
		{Options{Paper: "letter"}, 20},
		{Options{Paper: "a5", Size: "large"}, 4},
		{Options{Format: "cards"}, 8},
		{Options{Format: "cards", Paper: "a4"}, 8},
		{Options{Format: "cards", Paper: "letter"}, 8},
		{Options{Format: "cards", Paper: "a5"}, 3},
		// The cover size does not apply to cards.
		{Options{Format: "cards", Size: "mini"}, 8},
		{Options{Format: "cards", Size: "large", Paper: "a5"}, 3},
		// Spines follow the cover size.
		{Options{Format: "spines"}, 80},
		{Options{Format: "spines", Size: "mini"}, 144},
		{Options{Format: "spines", Size: "large", Paper: "a5"}, 22},
		{Options{Format: "stubs"}, 12},
		{Options{Format: "stubs", Paper: "letter"}, 12},
		{Options{Format: "stubs", Paper: "a5"}, 4},
		// The cover size does not apply to stubs.
		{Options{Format: "stubs", Size: "mini"}, 12},
		{Options{Format: "stubs", Size: "large", Paper: "a5"}, 4},
		// A full page holds up to 60 covers, whatever the paper, size or
		// header.
		{Options{Format: "fullpage"}, 60},
		{Options{Format: "fullpage", Paper: "letter", Header: "Books of 2026"}, 60},
		{Options{Format: "fullpage", Paper: "a5", Size: "large"}, 60},
	}
	for _, tt := range tests {
		got, err := tt.opts.PerPage()
		if err != nil || got != tt.want {
			t.Errorf("%+v.PerPage() = %d, %v; want %d", tt.opts, got, err, tt.want)
		}
	}
	if _, err := (Options{Paper: "a3"}).PerPage(); err == nil {
		t.Error(`Options{Paper: "a3"}.PerPage(): want an error`)
	}
}
