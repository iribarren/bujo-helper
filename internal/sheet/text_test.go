package sheet

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// runeWidth measures 1 unit per rune.
func runeWidth(s string) float64 { return float64(utf8.RuneCountInString(s)) }

func TestFitLines(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		width    float64
		maxLines int
		want     []string
	}{
		{"fits on one line", "Dune", 10, 2, []string{"Dune"}},
		{"wraps to two lines", "The Left Hand", 9, 2, []string{"The Left", "Hand"}},
		{"wraps exactly into the lines", "Brave New World", 9, 2, []string{"Brave New", "World"}},
		{"truncates with an ellipsis", "The Left Hand of Darkness", 12, 2, []string{"The Left", "Hand of…"}},
		{"cuts the last line to fit the ellipsis", "abcdef ghijkl mnopqr", 6, 2, []string{"abcdef", "ghijk…"}},
		{"long single word is broken by runes", "Supercalifragilistic", 8, 3, []string{"Supercal", "ifragili", "stic"}},
		{"long word after a short one", "A Supercalifragilistic", 8, 4, []string{"A", "Supercal", "ifragili", "stic"}},
		{"long word truncated", "Supercalifragilistic", 8, 2, []string{"Supercal", "ifragil…"}},
		{"exactly fitting line", "abcde fghij", 11, 1, []string{"abcde fghij"}},
		{"one unit too narrow", "abcde fghij", 10, 1, []string{"abcde…"}},
		{"max one line", "Gabriel García Márquez", 12, 1, []string{"Gabriel…"}},
		{"multibyte runes wrap", "Amélie Poulain", 7, 2, []string{"Amélie", "Poulain"}},
		{"multibyte runes truncate", "García Márquez", 8, 1, []string{"García…"}},
		{"multibyte word broken by runes", "Éééééééé", 4, 2, []string{"Éééé", "éééé"}},
		{"collapses spaces", "  Dune   Messiah  ", 20, 2, []string{"Dune Messiah"}},
		{"empty text", "", 10, 2, nil},
		{"blank text", "   ", 10, 2, nil},
		{"no lines allowed", "Dune", 10, 0, nil},
		{"nothing fits", "Dune", 0.5, 2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fitLines(tt.text, tt.width, tt.maxLines, runeWidth)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("fitLines(%q, %v, %d) = %q, want %q", tt.text, tt.width, tt.maxLines, got, tt.want)
			}
			for _, line := range got {
				if runeWidth(line) > tt.width {
					t.Errorf("line %q is wider than %v", line, tt.width)
				}
				if !utf8.ValidString(line) {
					t.Errorf("line %q is not valid UTF-8", line)
				}
			}
		})
	}
}

// fitLines never returns more lines than allowed nor a line wider than the
// width, for any width, line count and text.
func TestFitLinesStaysWithinBounds(t *testing.T) {
	texts := []string{
		"Cien años de soledad edición conmemorativa de la Real Academia Española",
		"Supercalifragilisticexpialidocious and more",
		"a b c d e f g h i j k l m n o p",
		"日本語のタイトルはスペースがない",
	}
	for _, text := range texts {
		for width := 1.0; width <= 30; width++ {
			for maxLines := 1; maxLines <= 4; maxLines++ {
				got := fitLines(text, width, maxLines, runeWidth)
				if len(got) == 0 || len(got) > maxLines {
					t.Errorf("%q, width %v, %d lines: got %d lines", text, width, maxLines, len(got))
				}
				for _, line := range got {
					if runeWidth(line) > width || line == "" {
						t.Errorf("%q, width %v, %d lines: bad line %q", text, width, maxLines, line)
					}
				}
				// Text is cut only when it does not fit, and then ends with "…".
				joined := strings.Join(got, " ")
				if strings.HasSuffix(joined, "…") == (strings.ReplaceAll(joined, " ", "") == strings.ReplaceAll(text, " ", "")) {
					t.Errorf("%q, width %v, %d lines: %q", text, width, maxLines, got)
				}
			}
		}
	}
}
