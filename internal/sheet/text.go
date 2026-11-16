package sheet

import (
	"strings"
	"unicode/utf8"
)

// ellipsis ends a line of text that was cut short.
const ellipsis = "…"

// fitTolerance keeps an exact fit from being lost to rounding, in the units
// of the width measure.
const fitTolerance = 1e-9

// fitLines wraps text into at most maxLines lines no wider than maxWidth, as
// measured by measure. Words are separated by single spaces; a word wider
// than a line is broken between runes. If the text needs more lines, the
// last line is cut so that it fits followed by "…". Empty text, no lines
// allowed, or a width too narrow for a single rune give no lines.
func fitLines(text string, maxWidth float64, maxLines int, measure func(string) float64) []string {
	if maxLines <= 0 {
		return nil
	}
	fits := func(s string) bool { return measure(s) <= maxWidth+fitTolerance }

	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if len(lines) > maxLines {
			break // already too long: the rest is cut anyway
		}
		if line != "" {
			if fits(line + " " + word) {
				line += " " + word
				continue
			}
			lines = append(lines, line)
			line = ""
		}
		for !fits(word) {
			n := fittingPrefix(word, fits)
			if n == 0 {
				return nil
			}
			lines = append(lines, word[:n])
			word = word[n:]
		}
		line = word
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) <= maxLines {
		return lines
	}
	lines = lines[:maxLines]
	last, ok := withEllipsis(lines[maxLines-1], fits)
	if !ok {
		return lines[:maxLines-1]
	}
	lines[maxLines-1] = last
	return lines
}

// fittingPrefix returns the length in bytes of the longest prefix of word,
// cut between runes, that fits; 0 if not even the first rune does.
func fittingPrefix(word string, fits func(string) bool) int {
	n := 0
	for i, r := range word {
		end := i + utf8.RuneLen(r)
		if !fits(word[:end]) {
			break
		}
		n = end
	}
	return n
}

// withEllipsis returns line followed by "…", dropping runes from the end of
// line until it fits. It reports false if not even "…" alone fits.
func withEllipsis(line string, fits func(string) bool) (string, bool) {
	runes := []rune(line)
	for n := len(runes); n >= 0; n-- {
		s := strings.TrimRight(string(runes[:n]), " ") + ellipsis
		if fits(s) {
			return s, true
		}
	}
	return "", false
}
