package sheet

import (
	"bytes"
	"image/color"
	"testing"
)

// spanishLabels are the labels the app prints for Spanish.
var spanishLabels = Labels{Admit: "ENTRADA", Date: "Fecha:", Where: "Lugar:", CardDate: "Fecha"}

func TestLabelsDefaultToEnglish(t *testing.T) {
	english := Labels{Admit: "ADMIT ONE", Date: "Date:", Where: "Where:", CardDate: "Date"}
	if got := (Options{}).labels(); got != english {
		t.Errorf("zero labels = %+v, want %+v", got, english)
	}
	got := (Options{Labels: Labels{Admit: " ENTRADA ", Where: "  "}}).labels()
	if want := (Labels{Admit: "ENTRADA", Date: "Date:", Where: "Where:", CardDate: "Date"}); got != want {
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

// The English and Spanish labels print whole inside their fixed boxes on
// stubs and cards, measured with the embedded fonts, leaving room for the
// rules to write on.
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
	s := layoutStub(Box{X: 13, Y: 18.5, W: StubWidth, H: StubHeight}, 0.656)
	c := layoutCard(Box{X: 13, Y: 18.5, W: CardWidth, H: CardHeight}, 0.656)
	for name, labels := range map[string]Labels{"English": {}, "Spanish": spanishLabels} {
		l := (Options{Labels: labels}).labels()

		admit := measure(true, admitFontSize)
		if lines := fitLines(l.Admit, s.Admit.H, 1, admit); len(lines) != 1 || lines[0] != l.Admit {
			t.Errorf("%s: admit text fits as %q in %v mm, want %q whole", name, lines, s.Admit.H, l.Admit)
		}

		label := measure(false, labelFontSize)
		if room := s.Text.W - max(label(l.Date), label(l.Where)) - labelRuleGap; room < minDateRule {
			t.Errorf("%s: %v mm left for the stub rules, want at least %v", name, room, minDateRule)
		}
		card := (Options{Labels: labels}).dateLabel()
		if lines := fitLines(card, c.Date.W-minDateRule, 1, label); len(lines) != 1 || lines[0] != card {
			t.Errorf("%s: card date label fits as %q, want %q whole", name, lines, card)
		}
	}
}

func TestStubLabels(t *testing.T) {
	dir := t.TempDir()
	paths := []string{writeImage(t, dir, "Dune - Denis Villeneuve.jpg", 66, 100, color.RGBA{200, 120, 40, 255})}
	english := Labels{Admit: "ADMIT ONE", Date: "Date:", Where: "Where:", CardDate: "Date"}
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
