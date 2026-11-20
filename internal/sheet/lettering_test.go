package sheet

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestLetteringFontsRegistry(t *testing.T) {
	wantIDs := []string{
		"go", "kaushan-script", "pacifico", "sacramento", "patrick-hand",
		"amatic-sc", "titan-one", "bungee", "luckiest-guy",
		"bungee-outline", "londrina-outline",
	}
	if len(LetteringFonts) != len(wantIDs) {
		t.Fatalf("len(LetteringFonts) = %d, want %d", len(LetteringFonts), len(wantIDs))
	}
	groups := map[string]bool{"sans": true, "brush": true, "monoline": true, "handwritten": true, "display": true, "hollow": true}
	seen := map[string]bool{}
	for i, f := range LetteringFonts {
		if f.ID != wantIDs[i] {
			t.Errorf("font %d id = %q, want %q", i, f.ID, wantIDs[i])
		}
		if seen[f.ID] {
			t.Errorf("duplicate id %q", f.ID)
		}
		seen[f.ID] = true
		if f.Label == "" {
			t.Errorf("%s: empty label", f.ID)
		}
		if !groups[f.Group] {
			t.Errorf("%s: group %q is not a known group", f.ID, f.Group)
		}
		switch f.Hollow {
		case HollowStroke, HollowNone, HollowNative:
		default:
			t.Errorf("%s: hollow %q is not a known capability", f.ID, f.Hollow)
		}
		if len(f.TTF) == 0 {
			t.Errorf("%s: no TTF bytes", f.ID)
		}
		if f.Licence == "" || len(f.LicenceText) < 500 {
			t.Errorf("%s: licence %q with %d bytes of text", f.ID, f.Licence, len(f.LicenceText))
		}
	}
	if LetteringFonts[0].ID != DefaultLetteringFont {
		t.Errorf("first font = %q, want the default %q", LetteringFonts[0].ID, DefaultLetteringFont)
	}
}

func TestLetteringFontValues(t *testing.T) {
	tests := []struct {
		id, label, group string
		hollow           Hollow
		licence          string
	}{
		{"go", "Default", "sans", HollowStroke, "BSD 3-Clause (Go fonts)"},
		{"kaushan-script", "Kaushan Script", "brush", HollowStroke, "SIL OFL 1.1"},
		{"pacifico", "Pacifico", "brush", HollowStroke, "SIL OFL 1.1"},
		{"sacramento", "Sacramento", "monoline", HollowNone, "SIL OFL 1.1"},
		{"patrick-hand", "Patrick Hand", "handwritten", HollowStroke, "SIL OFL 1.1"},
		{"amatic-sc", "Amatic SC", "handwritten", HollowNone, "SIL OFL 1.1"},
		{"titan-one", "Titan One", "display", HollowStroke, "SIL OFL 1.1"},
		{"bungee", "Bungee", "display", HollowStroke, "SIL OFL 1.1"},
		{"luckiest-guy", "Luckiest Guy", "display", HollowStroke, "Apache 2.0"},
		{"bungee-outline", "Bungee Outline", "hollow", HollowNative, "SIL OFL 1.1"},
		{"londrina-outline", "Londrina Outline", "hollow", HollowNative, "SIL OFL 1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			f, ok := LetteringFontByID(tt.id)
			if !ok {
				t.Fatalf("LetteringFontByID(%q) not found", tt.id)
			}
			if f.Label != tt.label || f.Group != tt.group || f.Hollow != tt.hollow || f.Licence != tt.licence {
				t.Errorf("got %q/%q/%q/%q, want %q/%q/%q/%q",
					f.Label, f.Group, f.Hollow, f.Licence, tt.label, tt.group, tt.hollow, tt.licence)
			}
		})
	}
}

func TestLetteringFontLookup(t *testing.T) {
	tests := []struct {
		name, id, want string
	}{
		{"empty is the default", "", "go"},
		{"unknown falls back to the default", "comic-sans", "go"},
		{"case matters", "Pacifico", "go"},
		{"known id", "pacifico", "pacifico"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LetteringFont(tt.id).ID; got != tt.want {
				t.Errorf("LetteringFont(%q).ID = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
	if _, ok := LetteringFontByID(""); ok {
		t.Error(`LetteringFontByID("") found a font`)
	}
	if _, ok := LetteringFontByID("comic-sans"); ok {
		t.Error(`LetteringFontByID("comic-sans") found a font`)
	}
}

func TestLetteringFontHeaderFace(t *testing.T) {
	g := LetteringFont("go")
	if g.HeaderStyle() != "B" {
		t.Errorf("go header style = %q, want B (today's bold header)", g.HeaderStyle())
	}
	if !bytes.Equal(g.HeaderTTF(), g.Bold) || len(g.Bold) == 0 {
		t.Error("go header TTF is not Go Bold")
	}
	p := LetteringFont("pacifico")
	if p.HeaderStyle() != "" || !bytes.Equal(p.HeaderTTF(), p.TTF) {
		t.Errorf("pacifico header style = %q, want the single regular face", p.HeaderStyle())
	}
}

// TestLetteringFontsLoadInPDF registers every font in a fresh document,
// draws accented text with it and checks fpdf reports no error.
func TestLetteringFontsLoadInPDF(t *testing.T) {
	for _, f := range LetteringFonts {
		t.Run(f.ID, func(t *testing.T) {
			pdf := newPDF(A4)
			if err := addLetteringFont(pdf, f); err != nil {
				t.Fatalf("addLetteringFont: %v", err)
			}
			pdf.AddPage()
			pdf.SetFont(f.ID, f.HeaderStyle(), 24)
			pdf.Text(20, 40, "¡Hola! ¿Qué tal? Ñandú, pingüino 0123")
			if w := pdf.GetStringWidth("Lettering"); w <= 0 {
				t.Errorf("string width = %v, want > 0", w)
			}
			var buf bytes.Buffer
			if err := pdf.Output(&buf); err != nil {
				t.Fatalf("output: %v", err)
			}
			if pdf.Err() {
				t.Fatalf("pdf error: %v", pdf.Error())
			}
			if buf.Len() == 0 {
				t.Fatal("empty PDF")
			}
		})
	}
}

// TestLetteringFontsLoadAfterTextFonts checks the default font can be added
// on top of addTextFonts, as every format already loads the Go fonts.
func TestLetteringFontsLoadAfterTextFonts(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	for _, f := range LetteringFonts {
		if err := addLetteringFont(pdf, f); err != nil {
			t.Fatalf("%s: %v", f.ID, err)
		}
	}
}

// esAlphabet is every character the EN and ES alphabets need.
const esAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789" +
	"áéíóúÁÉÍÓÚñÑüÜ¿¡"

func TestLetteringFontsCoverSpanish(t *testing.T) {
	for _, f := range LetteringFonts {
		t.Run(f.ID, func(t *testing.T) {
			faces := [][]byte{f.TTF}
			if f.Bold != nil {
				faces = append(faces, f.Bold)
			}
			for _, ttf := range faces {
				font, err := sfnt.Parse(ttf)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				var buf sfnt.Buffer
				var missing []string
				for _, r := range esAlphabet {
					idx, err := font.GlyphIndex(&buf, r)
					if err != nil {
						t.Fatalf("glyph %q: %v", r, err)
					}
					if idx == 0 {
						missing = append(missing, string(r))
					}
				}
				if len(missing) > 0 {
					t.Errorf("missing glyphs: %s", strings.Join(missing, " "))
				}
			}
		})
	}
}
