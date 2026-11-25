package sheet

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/go-pdf/fpdf"
)

// spanishLabels are the labels the app prints for Spanish.
var spanishLabels = Labels{Admit: "ENTRADA", Date: "Fecha:", Where: "Lugar:", CardDate: "Fecha", Credits: "Créditos", Fonts: "Fuentes"}

func TestLabelsDefaultToEnglish(t *testing.T) {
	english := Labels{Admit: "ADMIT ONE", Date: "Date:", Where: "Where:", CardDate: "Date", Credits: "Credits", Fonts: "Fonts"}
	if got := (Options{}).labels(); got != english {
		t.Errorf("zero labels = %+v, want %+v", got, english)
	}
	got := (Options{Labels: Labels{Admit: " ENTRADA ", Where: "  "}}).labels()
	if want := (Labels{Admit: "ENTRADA", Date: "Date:", Where: "Where:", CardDate: "Date", Credits: "Credits", Fonts: "Fonts"}); got != want {
		t.Errorf("partial labels = %+v, want %+v", got, want)
	}
	if got := spanishLabels; (Options{Labels: got}).labels() != got {
		t.Errorf("full labels changed to %+v", (Options{Labels: got}).labels())
	}
}

func TestCardDateLabelFallsBackToTheLabels(t *testing.T) {
	tests := []struct {
		opts Options
		want string
	}{
		{Options{}, "Date:"},
		{Options{Labels: spanishLabels}, "Fecha:"},
		{Options{Labels: spanishLabels, DateLabel: "Leído el"}, "Leído el:"},
	}
	for _, tt := range tests {
		if got := tt.opts.dateLabel(); got != tt.want {
			t.Errorf("dateLabel(%+v) = %q, want %q", tt.opts, got, tt.want)
		}
	}
}

// posterRatios span the poster shapes (width / height) a sheet meets, from
// wide landscape to tall and narrow.
var posterRatios = []struct {
	name  string
	ratio float64
}{
	{"wide landscape 16:9", 16.0 / 9},
	{"square", 1},
	{"one-sheet 27x40", 0.675},
	{"poster 0.656", 0.656},
	{"tall 1:2", 0.5},
	{"narrow 3:10", 0.3},
}

// labelTolerance absorbs the 0.01 pt rounding of the drawn coordinates, mm.
const labelTolerance = 0.01

// The English and Spanish labels print whole inside their fixed boxes on
// stubs and cards, measured with the embedded fonts, leaving room for the
// rules to write on, whatever the poster's shape.
func TestLabelsFitTheirBoxes(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := func(bold bool, pt float64) func(string) float64 {
		return func(text string) float64 {
			pdf.SetFont(textFont, fontStyle(bold), pt)
			return pdf.GetStringWidth(text)
		}
	}
	languages := []struct {
		name   string
		labels Labels
	}{
		{"English", Labels{}},
		{"Spanish", spanishLabels},
	}
	for _, r := range posterRatios {
		s := layoutStub(Box{X: 13, Y: 18.5, W: StubWidth, H: StubHeight}, r.ratio)
		c := layoutCard(Box{X: 13, Y: 18.5, W: CardWidth, H: CardHeight}, r.ratio)
		for _, lang := range languages {
			t.Run(r.name+"/"+lang.name, func(t *testing.T) {
				l := (Options{Labels: lang.labels}).labels()

				admit := measure(true, admitFontSize)
				if lines := fitLines(l.Admit, s.Admit.H, 1, admit); len(lines) != 1 || lines[0] != l.Admit {
					t.Errorf("stub admit text fits as %q in %v mm, want %q whole", lines, s.Admit.H, l.Admit)
				}

				label := measure(false, labelFontSize)
				if room := s.Text.W - max(label(l.Date), label(l.Where)) - labelRuleGap; room < minDateRule {
					t.Errorf("stub: %v mm left for the rules, want at least %v", room, minDateRule)
				}
				card := (Options{Labels: lang.labels}).dateLabel()
				if lines := fitLines(card, c.Date.W-minDateRule, 1, label); len(lines) != 1 || lines[0] != card {
					t.Errorf("card date label fits as %q, want %q whole", lines, card)
				}
			})
		}
	}
}

// A drawn stub prints the app's labels whole and cuts a label too long for
// its field short with an ellipsis, so both rules still start together and
// leave at least minDateRule to write on, whatever the poster's shape.
func TestStubDrawsLabelsThatFit(t *testing.T) {
	const long = "Fecha y hora de la sesión del cine:"
	tests := []struct {
		name   string
		labels Labels
		cut    bool // whether the Date label must be cut short
	}{
		{"English", Labels{}, false},
		{"Spanish", spanishLabels, false},
		{"long date label", Labels{Date: long}, true},
	}
	for _, r := range posterRatios {
		s := layoutStub(Box{X: 13, Y: 18.5, W: StubWidth, H: StubHeight}, r.ratio)
		for _, tt := range tests {
			t.Run(r.name+"/"+tt.name, func(t *testing.T) {
				l := (Options{Labels: tt.labels}).labels()
				texts, lines, width := drawOneStub(t, s, l)

				date := labelAt(t, texts, s.Date.X, baseline(s.Date.Y, labelFontSize))
				where := labelAt(t, texts, s.Where.X, baseline(s.Where.Y, labelFontSize))
				if where != l.Where {
					t.Errorf("where label drawn as %q, want %q whole", where, l.Where)
				}
				if !tt.cut && date != l.Date {
					t.Errorf("date label drawn as %q, want %q whole", date, l.Date)
				}
				if tt.cut {
					if width(l.Date) <= s.Date.W-labelRuleGap-minDateRule {
						t.Fatalf("%q is %v mm wide and fits; the case needs a longer label", l.Date, width(l.Date))
					}
					trimmed := strings.TrimSuffix(date, ellipsis)
					if date == l.Date || trimmed == date || trimmed == "" || !strings.HasPrefix(l.Date, trimmed) {
						t.Errorf("long date label drawn as %q, want a prefix of %q ending in %q", date, l.Date, ellipsis)
					}
				}
				if w, room := width(date), s.Date.W-labelRuleGap-minDateRule; w > room+labelTolerance {
					t.Errorf("date label %q is %v mm wide, want at most %v", date, w, room)
				}

				right := s.Date.X + s.Date.W
				dateRule := ruleAt(t, lines, baseline(s.Date.Y, labelFontSize), right)
				whereRule := ruleAt(t, lines, baseline(s.Where.Y, labelFontSize), right)
				if math.Abs(dateRule-whereRule) > labelTolerance {
					t.Errorf("rules start at %v and %v mm, want together", dateRule, whereRule)
				}
				if want := s.Date.X + max(width(date), width(where)) + labelRuleGap; math.Abs(dateRule-want) > labelTolerance {
					t.Errorf("rules start at %v mm, want %v (after the longer label)", dateRule, want)
				}
				if room := right - dateRule; room < minDateRule-labelTolerance {
					t.Errorf("%v mm left for the rules, want at least %v", room, minDateRule)
				}
			})
		}
	}
}

// drawnText is a string drawn with its baseline starting at (X, Y), mm.
type drawnText struct {
	X, Y float64
	Text string
}

// drawnLine is a straight line stroked from (X1, Y1) to (X2, Y2), mm.
type drawnLine struct {
	X1, Y1, X2, Y2 float64
}

var (
	textOp = regexp.MustCompile(`BT (-?[\d.]+) (-?[\d.]+) Td \(((?:\\.|[^\\)])*)\) Tj ET`)
	lineOp = regexp.MustCompile(`(-?[\d.]+) (-?[\d.]+) m (-?[\d.]+) (-?[\d.]+) l S`)
)

// drawOneStub draws stub s with labels on an uncompressed A4 page, as
// buildStubs does, and returns the texts and straight lines drawn, plus a
// measure of label text as drawn.
func drawOneStub(t *testing.T, s stubParts, labels Labels) ([]drawnText, []drawnLine, func(string) float64) {
	t.Helper()
	var img bytes.Buffer
	if err := jpeg.Encode(&img, image.NewRGBA(image.Rect(0, 0, 4, 6)), nil); err != nil {
		t.Fatal(err)
	}
	pdf := newPDF(A4)
	pdf.SetCompression(false)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	opts := fpdf.ImageOptions{ImageType: "JPG"}
	pdf.RegisterImageOptionsReader("cover", opts, &img)
	pdf.AddPage()
	drawStub(pdf, s, "cover", opts, "Dune", "Denis Villeneuve", labels)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatal(err)
	}

	k := 72 / 25.4 // points per mm
	num := func(b []byte) float64 {
		v, err := strconv.ParseFloat(string(b), 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	var texts []drawnText
	for _, m := range textOp.FindAllSubmatch(out.Bytes(), -1) {
		texts = append(texts, drawnText{X: num(m[1]) / k, Y: A4.Height - num(m[2])/k, Text: decodePDFText(t, m[3])})
	}
	var lines []drawnLine
	for _, m := range lineOp.FindAllSubmatch(out.Bytes(), -1) {
		lines = append(lines, drawnLine{num(m[1]) / k, A4.Height - num(m[2])/k, num(m[3]) / k, A4.Height - num(m[4])/k})
	}
	width := func(text string) float64 {
		pdf.SetFont(textFont, "", labelFontSize)
		return pdf.GetStringWidth(text)
	}
	return texts, lines, width
}

// decodePDFText turns an escaped PDF string of UTF-16BE code units, as
// fpdf writes text in UTF-8 fonts, back into UTF-8.
func decodePDFText(t *testing.T, escaped []byte) string {
	t.Helper()
	var raw []byte
	for i := 0; i < len(escaped); i++ {
		b := escaped[i]
		if b == '\\' && i+1 < len(escaped) {
			i++
			b = escaped[i]
			if b == 'r' {
				b = '\r'
			}
		}
		raw = append(raw, b)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("PDF text %q is not UTF-16", escaped)
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = uint16(raw[2*i])<<8 | uint16(raw[2*i+1])
	}
	return string(utf16.Decode(units))
}

// labelAt returns the one text drawn with its baseline starting at (x, y).
func labelAt(t *testing.T, texts []drawnText, x, y float64) string {
	t.Helper()
	var found []string
	for _, d := range texts {
		if math.Abs(d.X-x) <= labelTolerance && math.Abs(d.Y-y) <= labelTolerance {
			found = append(found, d.Text)
		}
	}
	if len(found) != 1 {
		t.Fatalf("texts drawn at (%v, %v) mm: %q, want exactly one", x, y, found)
	}
	return found[0]
}

// ruleAt returns where the one horizontal rule on baseline y that ends at
// right starts, in mm.
func ruleAt(t *testing.T, lines []drawnLine, y, right float64) float64 {
	t.Helper()
	var found []float64
	for _, l := range lines {
		if math.Abs(l.Y1-y) <= labelTolerance && math.Abs(l.Y2-y) <= labelTolerance && math.Abs(l.X2-right) <= labelTolerance {
			found = append(found, l.X1)
		}
	}
	if len(found) != 1 {
		t.Fatalf("rules on y = %v mm ending at %v mm start at %v, want exactly one", y, right, found)
	}
	return found[0]
}

func TestStubLabels(t *testing.T) {
	dir := t.TempDir()
	paths := []string{writeImage(t, dir, "Dune - Denis Villeneuve.jpg", 66, 100, color.RGBA{200, 120, 40, 255})}
	english := Labels{Admit: "ADMIT ONE", Date: "Date:", Where: "Where:", CardDate: "Date", Credits: "Credits", Fonts: "Fonts"}
	for _, format := range []string{"stubs", "cards"} {
		zero := render(t, paths, Options{Format: format})
		if got := render(t, paths, Options{Format: format, Labels: english}); !bytes.Equal(got, zero) {
			t.Errorf("%s: explicit English labels change the PDF", format)
		}
		if got := render(t, paths, Options{Format: format, Labels: spanishLabels}); bytes.Equal(got, zero) {
			t.Errorf("%s: Spanish labels do not change the PDF", format)
		}
	}
}
