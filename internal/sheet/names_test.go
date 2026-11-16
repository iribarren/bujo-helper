package sheet

import "testing"

func TestParseName(t *testing.T) {
	tests := []struct {
		name, file     string
		title, creator string
	}{
		{"title and creator", "Dune - Frank Herbert.jpg", "Dune", "Frank Herbert"},
		{"no creator", "Dune.jpg", "Dune", ""},
		{"duplicate suffix", "Dune - Frank Herbert-2.jpg", "Dune", "Frank Herbert"},
		{"tenth duplicate", "Dune - Frank Herbert-10.png", "Dune", "Frank Herbert"},
		{"duplicate without creator", "Dune-3.webp", "Dune", ""},
		{"title containing the separator", "Star Wars - A New Hope - George Lucas.jpg", "Star Wars - A New Hope", "George Lucas"},
		{"year as creator", "Dune - 2021.jpg", "Dune", "2021"},
		{"duplicate of a movie", "Dune - 2021-2.jpg", "Dune", "2021"},
		{"number in the title", "Blade Runner 2049 - 2017.jpg", "Blade Runner 2049", "2017"},
		{"path with directories", "/home/me/Covers/Books/Dune - Frank Herbert.jpg", "Dune", "Frank Herbert"},
		{"relative path", "Books/Dune.jpg", "Dune", ""},
		{"accents", "Cien años de soledad - Gabriel García Márquez.jpg", "Cien años de soledad", "Gabriel García Márquez"},
		{"hyphenated creator", "Ubik - Jean-Luc Picard.jpg", "Ubik", "Jean-Luc Picard"},
		{"-1 is not a duplicate suffix", "Apollo-1.jpg", "Apollo-1", ""},
		{"spaced -2 is not a duplicate suffix", "Rocky -2.jpg", "Rocky -2", ""},
		{"hyphen without number", "X-Men - 2000.jpg", "X-Men", "2000"},
		{"no extension", "Dune - Frank Herbert", "Dune", "Frank Herbert"},
		{"extra spaces", " Dune  -  Frank Herbert .jpg", "Dune", "Frank Herbert"},
		{"fallback name", "cover.jpg", "cover", ""},
		{"empty", "", "", ""},
		// Ambiguous by design: a title ending in -N (N ≥ 2) with no creator
		// looks exactly like a duplicate, so the suffix is always stripped.
		{"title ending in -2 without creator", "Rocky-2.jpg", "Rocky", ""},
		{"Catch-22 without creator", "Catch-22.jpg", "Catch", ""},
		{"Catch-22 with creator keeps its title", "Catch-22 - Joseph Heller.jpg", "Catch-22", "Joseph Heller"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, creator := ParseName(tt.file)
			if title != tt.title || creator != tt.creator {
				t.Errorf("ParseName(%q) = %q, %q; want %q, %q", tt.file, title, creator, tt.title, tt.creator)
			}
		})
	}
}
