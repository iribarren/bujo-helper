package sheet

import (
	"embed"
	"fmt"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// fontFiles holds the bundled lettering fonts and their licences, one
// directory per font id. The TTFs are the original files, unmodified: their
// licences reserve the font names for unmodified fonts. fpdf subsets them
// inside each PDF.
//
//go:embed fonts
var fontFiles embed.FS

// Hollow is how a lettering font draws hollow (colour-in) letters.
type Hollow string

const (
	// HollowStroke letters are drawn stroke-only (PDF text rendering mode 1).
	HollowStroke Hollow = "stroke"
	// HollowNone fonts are too thin to outline: their letters stay filled.
	HollowNone Hollow = "none"
	// HollowNative fonts are outlined by design and are never stroked.
	HollowNative Hollow = "native"
)

// Licence names of the lettering fonts.
const (
	licenceOFL    = "SIL OFL 1.1"
	licenceApache = "Apache 2.0"
	licenceGo     = "BSD 3-Clause (Go fonts)"
)

// Font is a lettering font for display text (headers and titles).
type Font struct {
	ID    string // also the fpdf family name
	Label string // display name
	// Group is the style group: sans, brush, monoline, handwritten,
	// display or hollow.
	Group       string
	TTF         []byte // the regular face, registered with style ""
	Bold        []byte // a bold face registered with style "B"; only the Go font has one
	Licence     string // licence name, e.g. "SIL OFL 1.1"
	LicenceText string
	Hollow      Hollow
}

// HeaderStyle is the fpdf style headers use with f: "B" when f has a bold
// face (the Go font keeps today's bold header), else "".
func (f Font) HeaderStyle() string {
	if f.Bold != nil {
		return "B"
	}
	return ""
}

// HeaderTTF is the face headers are drawn in, matching HeaderStyle.
func (f Font) HeaderTTF() []byte {
	if f.Bold != nil {
		return f.Bold
	}
	return f.TTF
}

// DefaultLetteringFont is the id of the default lettering font, the Go font
// every format already uses.
const DefaultLetteringFont = textFont

// LetteringFonts lists the lettering fonts, the default first.
var LetteringFonts = []Font{
	{
		ID: DefaultLetteringFont, Label: "Default", Group: "sans",
		TTF: goregular.TTF, Bold: gobold.TTF,
		Licence: licenceGo, LicenceText: fontFile("go/LICENSE"),
		Hollow: HollowStroke,
	},
	bundledFont("kaushan-script", "Kaushan Script", "brush", "KaushanScript-Regular.ttf", HollowStroke),
	bundledFont("pacifico", "Pacifico", "brush", "Pacifico-Regular.ttf", HollowStroke),
	bundledFont("sacramento", "Sacramento", "monoline", "Sacramento-Regular.ttf", HollowNone),
	bundledFont("patrick-hand", "Patrick Hand", "handwritten", "PatrickHand-Regular.ttf", HollowStroke),
	bundledFont("amatic-sc", "Amatic SC", "handwritten", "AmaticSC-Bold.ttf", HollowNone),
	bundledFont("titan-one", "Titan One", "display", "TitanOne-Regular.ttf", HollowStroke),
	bundledFont("bungee", "Bungee", "display", "Bungee-Regular.ttf", HollowStroke),
	bundledFont("luckiest-guy", "Luckiest Guy", "display", "LuckiestGuy-Regular.ttf", HollowStroke),
	bundledFont("bungee-outline", "Bungee Outline", "hollow", "BungeeOutline-Regular.ttf", HollowNative),
	bundledFont("londrina-outline", "Londrina Outline", "hollow", "LondrinaOutline-Regular.ttf", HollowNative),
}

// bundledFont describes the embedded font in fonts/<id>/: the TTF named
// file and its licence, OFL.txt or (Apache 2.0) LICENSE.txt.
func bundledFont(id, label, group, file string, hollow Hollow) Font {
	licence, text := licenceOFL, ""
	if _, err := fontFiles.Open("fonts/" + id + "/LICENSE.txt"); err == nil {
		licence, text = licenceApache, fontFile(id+"/LICENSE.txt")
	} else {
		text = fontFile(id + "/OFL.txt")
	}
	return Font{
		ID: id, Label: label, Group: group,
		TTF:     []byte(fontFile(id + "/" + file)),
		Licence: licence, LicenceText: text,
		Hollow: hollow,
	}
}

// fontFile returns the embedded file at fonts/name. A missing file is a
// build mistake, so it panics at start-up rather than shipping without it.
func fontFile(name string) string {
	b, err := fontFiles.ReadFile("fonts/" + name)
	if err != nil {
		panic(fmt.Sprintf("lettering font file: %v", err))
	}
	return string(b)
}

// LetteringFontByID returns the lettering font with exactly this id.
func LetteringFontByID(id string) (Font, bool) {
	for _, f := range LetteringFonts {
		if f.ID == id {
			return f, true
		}
	}
	return Font{}, false
}

// LetteringFont returns the lettering font with this id. Empty or unknown
// ids give the default font, so a stale or hand-edited option never fails a
// print.
func LetteringFont(id string) Font {
	if f, ok := LetteringFontByID(id); ok {
		return f
	}
	return LetteringFonts[0]
}

// addLetteringFont registers f in pdf under the family f.ID: its regular
// face with style "" and its bold face, if any, with style "B". Adding a
// face that is already registered is a no-op, so the default font can be
// added on top of addTextFonts.
func addLetteringFont(pdf *fpdf.Fpdf, f Font) error {
	pdf.AddUTF8FontFromBytes(f.ID, "", f.TTF)
	if f.Bold != nil {
		pdf.AddUTF8FontFromBytes(f.ID, "B", f.Bold)
	}
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("load font %s: %w", f.ID, err)
	}
	return nil
}
