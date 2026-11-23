package sheet

import (
	"embed"
	"fmt"

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
func addLetteringFont(pdf *pdfDoc, f Font) error {
	pdf.AddUTF8FontFromBytes(f.ID, "", f.TTF)
	if f.Bold != nil {
		pdf.AddUTF8FontFromBytes(f.ID, "B", f.Bold)
	}
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("load font %s: %w", f.ID, err)
	}
	return nil
}

// Lettering drawing settings.
const (
	// minHollowFontSize is the smallest size letters are drawn hollow at;
	// smaller hollow letters leave no room to colour in, so they print
	// filled.
	minHollowFontSize = 14.0 // pt
	// refHollowStrokeWidth is the outline width of hollow letters at
	// hollowReferenceSize; it scales with the font size, down to
	// minHollowStrokeWidth.
	refHollowStrokeWidth = 0.25              // mm
	hollowReferenceSize  = maxHeaderFontSize // pt
	minHollowStrokeWidth = 0.2               // mm
	// bundledFitShare is the share of the available width display text in
	// a bundled font is fitted into. fpdf measures advance widths only, and
	// script and display faces draw swashes and overhangs past them, so the
	// rest is a safety margin, split on both sides of centred text. The Go
	// font keeps the whole width, so default headers stay as they were.
	bundledFitShare = 0.96
)

// lettering is how a printable draws its display text (headers and
// titles): in font, and hollow (stroke-only) when Colour-in is on and the
// font can be outlined.
type lettering struct {
	font     Font
	colourIn bool
}

// newLettering is the lettering of the font id (empty or unknown: the
// default font) with Colour-in on or off.
func newLettering(id string, colourIn bool) lettering {
	return lettering{font: LetteringFont(id), colourIn: colourIn}
}

// defaultLettering is today's display text: the Go font, filled.
var defaultLettering = newLettering(DefaultLetteringFont, false)

// bundled reports whether l uses a bundled font rather than the Go font,
// so the PDF embeds a font the credits page lists.
func (l lettering) bundled() bool { return l.font.ID != DefaultLetteringFont }

// hollow reports whether text at pt points is drawn stroke-only: with
// Colour-in on, in a font that can be outlined (fonts hollow by design are
// never stroked, and thin ones stay filled), at minHollowFontSize or more.
func (l lettering) hollow(pt float64) bool {
	return l.colourIn && l.font.Hollow == HollowStroke && pt >= minHollowFontSize
}

// fitWidth is the width text fitted into width may take in l's font.
func (l lettering) fitWidth(width float64) float64 {
	if l.bundled() {
		return width * bundledFitShare
	}
	return width
}

// add registers l's font in pdf.
func (l lettering) add(pdf *pdfDoc) error { return addLetteringFont(pdf, l.font) }

// setFont selects l's font, in its header style, at pt points; it must be
// added.
func (l lettering) setFont(pdf *pdfDoc, pt float64) {
	pdf.SetFont(l.font.ID, l.font.HeaderStyle(), pt)
}

// measure measures text in l's font, which must be added. The bold flag of
// measureFunc is ignored: display text always uses the header style.
func (l lettering) measure(pdf *pdfDoc) measureFunc {
	return func(text string, _ bool, pt float64) float64 {
		l.setFont(pdf, pt)
		return pdf.GetStringWidth(text)
	}
}

// text writes s with its baseline at (x, y) in the current font, which is
// l's at pt points: hollow if l.hollow(pt), else filled.
func (l lettering) text(pdf *pdfDoc, x, y, pt float64, s string) {
	if !l.hollow(pt) {
		pdf.Text(x, y, s)
		return
	}
	hollowText(pdf, x, y, pt, s)
}

// hollowStrokeWidth is the outline width of hollow letters at pt points.
func hollowStrokeWidth(pt float64) float64 {
	return max(minHollowStrokeWidth, refHollowStrokeWidth*pt/hollowReferenceSize)
}

// hollowText writes s stroke-only (text rendering mode 1) in black, its
// outline as wide as hollowStrokeWidth for pt. It always restores mode 0,
// which fpdf does not track, and the line width and draw colour, so the
// text and lines drawn after it are unaffected.
func hollowText(pdf *pdfDoc, x, y, pt float64, s string) {
	width := pdf.GetLineWidth()
	r, g, b := pdf.GetDrawColor()
	defer func() {
		pdf.SetTextRenderingMode(0)
		pdf.SetDrawColor(r, g, b)
		pdf.SetLineWidth(width)
	}()
	pdf.SetLineWidth(hollowStrokeWidth(pt))
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetTextRenderingMode(1)
	pdf.Text(x, y, s)
}
