package sheet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"testing"
)

// emoji is text with a rune outside the Basic Multilingual Plane, which the
// PDF's UTF-8 fonts cannot index.
const emoji = "Café 🎉"

// noPanic runs build, turning a panic into an error.
func noPanic(build func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return build()
}

// Every cover format exports with an emoji in a title, a creator, the
// header, the card date label and a credit.
func TestBuildEveryFormatWithEmoji(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, emoji+" - Ana 🎉.jpg", 60, 90, color.Gray{0x80}),
		writeImage(t, dir, "Wide 🌊 - 2026.png", 120, 60, color.Gray{0x40}),
	}
	writeCredit(t, paths[0], "Photo 📷 by Ana")
	type variant struct {
		name string
		opts Options
	}
	var variants []variant
	for _, f := range Formats {
		variants = append(variants, variant{f.ID, Options{Format: f.ID}})
	}
	for _, font := range LetteringFonts {
		for _, f := range []string{cardsFormat, fullFormat, checkFormat} {
			variants = append(variants,
				variant{f + "/" + font.ID, Options{Format: f, Font: font.ID}},
				variant{f + "/" + font.ID + "/colour in", Options{Format: f, Font: font.ID, ColourIn: true}})
		}
	}
	for _, c := range CropModes {
		variants = append(variants, variant{"crop " + c.ID, Options{Crop: c.ID}})
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			opts := v.opts
			opts.Header = emoji
			opts.DateLabel = emoji
			opts.Credits = true
			err := noPanic(func() error { return Build(io.Discard, paths, opts) })
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Every page kind exports with an emoji in its title and in the text the
// user types into its options.
func TestBuildPageEveryKindWithEmoji(t *testing.T) {
	options := map[string]string{
		HabitsKind: `{"habits":["Run 🏃","Read 📚"]}`,
		QuotesKind: `{"quotes":[{"text":"Smile 😀 always","author":"Ana 🎉"}]}`,
		RecipesKind: `{"recipes":[{"title":"Tarta 🍰","servings":"4 🎉","time":"30 min ⏱",` +
			`"ingredients":["Azúcar 🍬"],"steps":["Mix 🥣 well"]}]}`,
	}
	titleLabels := PageLabels{
		MapTitle: emoji, HabitsTitle: emoji + " {month}", QuotesTitle: emoji,
		Conversions: ConversionLabels{Title: emoji},
	}
	for _, k := range PageKinds {
		cases := []struct {
			name    string
			s       PageSettings
			options string
		}{
			{"typed title", PageSettings{Title: emoji}, ""},
			{"default title", PageSettings{Labels: titleLabels}, ""},
		}
		if o, ok := options[k.ID]; ok {
			cases = append(cases, struct {
				name    string
				s       PageSettings
				options string
			}{"typed text", PageSettings{}, o})
		}
		for _, c := range cases {
			for _, colourIn := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/colour in %v", k.ID, c.name, colourIn), func(t *testing.T) {
					s := c.s
					s.ColourIn = colourIn
					raw := json.RawMessage(c.options)
					err := noPanic(func() error { return BuildPage(io.Discard, k.ID, s, raw) })
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// Drawn text replaces runes outside the Basic Multilingual Plane with "?",
// one per rune, and keeps the rest.
func TestEmojiPrintsAsQuestionMark(t *testing.T) {
	pdf := renderPage(t, DotGridKind, PageSettings{Title: "A🎉B"}, "")
	if !bytes.Contains(pdf, []byte("BT")) {
		t.Fatal("no text drawn")
	}
	want := pageStreams(renderPage(t, DotGridKind, PageSettings{Title: "A?B"}, ""))
	if got := pageStreams(pdf); got != want {
		t.Errorf("page with %q differs from the one with %q", "A🎉B", "A?B")
	}
}
