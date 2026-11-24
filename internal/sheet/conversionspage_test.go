package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

// spanishConversionLabels are the conversion chart words the app prints
// for Spanish.
var spanishConversionLabels = ConversionLabels{
	Title:       "Conversiones de cocina",
	Volume:      "Volumen",
	Weight:      "Peso",
	Oven:        "Temperaturas del horno",
	CupsToGrams: "Tazas a gramos",
	Cup:         "taza",
	Tbsp:        "cda.",
	Tsp:         "cdta.",
	GasMark:     "Gas",
	FanOven:     "Horno con ventilador: 20 °C menos",
	Ingredients: []string{"Harina", "Azúcar", "Azúcar moreno", "Azúcar glas", "Mantequilla",
		"Arroz", "Copos de avena", "Miel", "Cacao en polvo", "Leche o agua"},
}

func TestConversionsRegistryEntry(t *testing.T) {
	k, err := PageKindByID(ConversionsKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Kitchen conversions" || k.FileTag != "conversions" || !k.UsesTitle || k.UsesCut {
		t.Errorf("conversions kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{}` {
		t.Errorf("defaults %s", raw)
	}
	n := len(PageKinds)
	if PageKinds[n-1].ID != PlantsKind || PageKinds[n-2].ID != ConversionsKind || PageKinds[n-3].ID != RecipesKind {
		t.Errorf("conversions kind not right after the recipe cards, before the plant care cards")
	}
	for _, p := range Papers {
		if n, err := PageCount(ConversionsKind, PageSettings{Paper: p.ID}, nil); n != 1 || err != nil {
			t.Errorf("%s: PageCount = %d, %v, want 1", p.ID, n, err)
		}
	}
}

func TestDecodeConversionsOptions(t *testing.T) {
	kind, err := PageKindByID(ConversionsKind)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		raw     string
		wantErr string // part of the error; "" for none
	}{
		{"no options", "", ""},
		{"null", "null", ""},
		{"empty object", "{}", ""},
		{"unknown field", `{"units":"metric"}`, `"units"`},
		{"not an object", `[1]`, "options"},
		{"trailing data", `{} {}`, "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := kind.decode(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if doc != (ConversionsOptions{}) || doc.pages() != 1 {
				t.Errorf("decoded %+v", doc)
			}
		})
	}
}

// US customary units in millilitres (the cup is the 240 ml US legal cup
// cookbooks use) and avoirdupois units in grams.
const (
	mlPerCup  = 240.0
	mlPerTbsp = 14.78676
	mlPerTsp  = 4.928922
	mlPerFlOz = 29.57353
	gPerOz    = 28.349523
	gPerLb    = 453.59237
)

// Every figure is checked against an independent computation where there
// is one, and against the fixed reference values otherwise.
func TestConversionFigures(t *testing.T) {
	f := conversionFigures

	perUnit := map[string]float64{volumeCup: mlPerCup, volumeTbsp: mlPerTbsp, volumeTsp: mlPerTsp, volumeFlOz: mlPerFlOz}
	wantVolume := []string{"1 cup", "3/4 cup", "2/3 cup", "1/2 cup", "1/3 cup", "1/4 cup", "1 tbsp", "1 tsp", "1 fl oz"}
	if len(f.Volume) != len(wantVolume) {
		t.Fatalf("%d volume rows, want %d", len(f.Volume), len(wantVolume))
	}
	for i, v := range f.Volume {
		amount := fmt.Sprintf("%d %s", v.Num, v.Unit)
		if v.Den != 1 {
			amount = fmt.Sprintf("%d/%d %s", v.Num, v.Den, v.Unit)
		}
		if amount != wantVolume[i] {
			t.Errorf("volume row %d is %s, want %s", i, amount, wantVolume[i])
		}
		exact := float64(v.Num) / float64(v.Den) * perUnit[v.Unit]
		if want := int(math.Round(exact)); v.ML != want {
			t.Errorf("%s = %d ml, want %d (%.2f)", amount, v.ML, want, exact)
		}
	}

	toGrams := map[string]float64{"oz": gPerOz, "lb": gPerLb}
	wantWeight := []struct {
		from     float64
		fromUnit string
		to       float64
		toUnit   string
	}{{1, "oz", 28, "g"}, {4, "oz", 113, "g"}, {8, "oz", 227, "g"}, {1, "lb", 454, "g"}, {1, "kg", 2.2, "lb"}}
	if len(f.Weight) != len(wantWeight) {
		t.Fatalf("%d weight rows, want %d", len(f.Weight), len(wantWeight))
	}
	for i, w := range f.Weight {
		want := wantWeight[i]
		if w.From != want.from || w.FromUnit != want.fromUnit || w.To != want.to || w.ToUnit != want.toUnit {
			t.Errorf("weight row %d = %+v, want %+v", i, w, want)
		}
		var exact float64
		if w.ToUnit == "g" {
			exact = math.Round(w.From * toGrams[w.FromUnit])
		} else { // kg to lb, to one decimal
			exact = math.Round(w.From*1000/gPerLb*10) / 10
		}
		if w.To != exact {
			t.Errorf("%v %s = %v %s, want %v", w.From, w.FromUnit, w.To, w.ToUnit, exact)
		}
	}

	wantOven := [][3]int{{140, 275, 1}, {150, 300, 2}, {170, 325, 3}, {180, 350, 4}, {190, 375, 5}, {200, 400, 6}, {220, 425, 7}, {230, 450, 8}, {240, 475, 9}}
	if len(f.Oven) != len(wantOven) {
		t.Fatalf("%d oven rows, want %d", len(f.Oven), len(wantOven))
	}
	for i, o := range f.Oven {
		if [3]int{o.C, o.F, o.Gas} != wantOven[i] {
			t.Errorf("oven row %d = %+v, want %v", i, o, wantOven[i])
		}
		// Gas mark n is 250 + 25n °F; the °C column is that converted and
		// rounded the way cookbook tables do, to a round 10 °C, which is
		// within 8 °C of the exact figure.
		if o.F != 250+25*o.Gas {
			t.Errorf("gas mark %d is %d °F, want %d", o.Gas, o.F, 250+25*o.Gas)
		}
		if exact := float64(o.F-32) * 5 / 9; math.Abs(exact-float64(o.C)) > 8 || o.C%10 != 0 {
			t.Errorf("%d °F is %.1f °C, printed %d", o.F, exact, o.C)
		}
	}

	wantCups := []int{125, 200, 220, 120, 227, 185, 90, 340, 85, 240}
	if len(f.Cups) != len(wantCups) {
		t.Fatalf("%d cups rows, want %d", len(f.Cups), len(wantCups))
	}
	for i, g := range f.Cups {
		if g != wantCups[i] {
			t.Errorf("cup of ingredient %d = %d g, want %d", i, g, wantCups[i])
		}
	}
	// A cup of butter is 8 oz; a cup of milk or water weighs as much as its
	// 240 ml.
	if butter := int(math.Round(8 * gPerOz)); f.Cups[4] != butter {
		t.Errorf("a cup of butter = %d g, want %d", f.Cups[4], butter)
	}
	if f.Cups[9] != int(mlPerCup) {
		t.Errorf("a cup of milk = %d g, want %v", f.Cups[9], mlPerCup)
	}
}

// The tables print the figures with the unit words of the labels, in
// table order: volume, weight, oven, cups to grams.
func TestConversionTables(t *testing.T) {
	en := conversionTables(PageLabels{}.resolved())
	cell := func(tb conversionTable, r, c int) string { return tb.Rows[r][c] }
	tests := []struct {
		got, want string
	}{
		{en[0].Heading, "Volume"},
		{cell(en[0], 0, 0), "1 cup"},
		{cell(en[0], 0, 1), "240 ml"},
		{cell(en[0], 4, 0), "1/3 cup"},
		{cell(en[0], 6, 0), "1 tbsp"},
		{cell(en[0], 7, 1), "5 ml"},
		{cell(en[0], 8, 0), "1 fl oz"},
		{en[1].Heading, "Weight"},
		{cell(en[1], 1, 0), "4 oz"},
		{cell(en[1], 1, 1), "113 g"},
		{cell(en[1], 4, 0), "1 kg"},
		{cell(en[1], 4, 1), "2.2 lb"},
		{en[2].Heading, "Oven temperatures"},
		{strings.Join(en[2].Header, "|"), "°C|°F|Gas mark"},
		{strings.Join(en[2].Rows[3], "|"), "180|350|4"},
		{en[2].Caption, "Fan oven: 20 °C lower"},
		{en[3].Heading, "Cups to grams"},
		{strings.Join(en[3].Header, "|"), "|1 cup"},
		{strings.Join(en[3].Rows[0], "|"), "Flour|125 g"},
		{strings.Join(en[3].Rows[9], "|"), "Milk or water|240 g"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
	es := conversionTables(PageLabels{Conversions: spanishConversionLabels, Language: "es"}.resolved())
	for _, tt := range []struct{ got, want string }{
		{es[0].Heading, "Volumen"},
		{cell(es[0], 1, 0), "3/4 taza"},
		{cell(es[0], 6, 0), "1 cda."},
		{cell(es[0], 7, 0), "1 cdta."},
		{cell(es[1], 4, 1), "2,2 lb"},
		{strings.Join(es[2].Header, "|"), "°C|°F|Gas"},
		{es[3].Heading, "Tazas a gramos"},
		{strings.Join(es[3].Header, "|"), "|1 taza"},
		{strings.Join(es[3].Rows[6], "|"), "Copos de avena|90 g"},
	} {
		if tt.got != tt.want {
			t.Errorf("Spanish: got %q, want %q", tt.got, tt.want)
		}
	}
}

// The tables do not rely on resolved labels: an ingredient list of the
// wrong length (none, or short) prints the English names instead of
// indexing past its end.
func TestConversionTablesUnresolvedIngredients(t *testing.T) {
	short := spanishConversionLabels
	short.Ingredients = []string{"Harina"}
	for name, labels := range map[string]PageLabels{
		"no ingredients":    {},
		"short ingredients": {Conversions: short},
	} {
		t.Run(name, func(t *testing.T) {
			cups := conversionTables(labels)[3]
			if len(cups.Rows) != len(conversionFigures.Cups) {
				t.Fatalf("%d rows, want %d", len(cups.Rows), len(conversionFigures.Cups))
			}
			for i, row := range cups.Rows {
				if row[0] != englishIngredients[i] {
					t.Errorf("row %d names %q, want %q", i, row[0], englishIngredients[i])
				}
			}
		})
	}
}

// The conversion chart words come from the labels, English by default; a
// list of ingredients of the wrong length keeps the English names.
func TestConversionLabels(t *testing.T) {
	en := PageLabels{}.resolved().Conversions
	if en.Title != "Kitchen conversions" || en.Volume != "Volume" || en.Weight != "Weight" || en.Oven != "Oven temperatures" || en.CupsToGrams != "Cups to grams" ||
		en.Cup != "cup" || en.Tbsp != "tbsp" || en.Tsp != "tsp" || en.GasMark != "Gas mark" || en.FanOven != "Fan oven: 20 °C lower" {
		t.Errorf("English conversion labels %+v", en)
	}
	wantIngredients := "Flour,Sugar,Brown sugar,Icing sugar,Butter,Rice,Rolled oats,Honey,Cocoa powder,Milk or water"
	if got := strings.Join(en.Ingredients, ","); got != wantIngredients {
		t.Errorf("English ingredients %s", got)
	}
	padded := spanishConversionLabels
	padded.Volume = " Volumen "
	if es := (PageLabels{Conversions: padded}).resolved().Conversions; es.Volume != "Volumen" || es.Ingredients[9] != "Leche o agua" || es.GasMark != "Gas" {
		t.Errorf("Spanish conversion labels %+v", es)
	}
	short := spanishConversionLabels
	short.Ingredients = []string{"Harina"}
	if got := strings.Join(PageLabels{Conversions: short}.resolved().Conversions.Ingredients, ","); got != wantIngredients {
		t.Errorf("a short ingredient list gives %s", got)
	}
}

func TestConversionsDefaultTitle(t *testing.T) {
	o := ConversionsOptions{}
	if got := o.defaultTitle(PageLabels{}.resolved()); got != "Kitchen conversions" {
		t.Errorf("English title %q", got)
	}
	if got := o.defaultTitle(PageLabels{Conversions: spanishConversionLabels}.resolved()); got != "Conversiones de cocina" {
		t.Errorf("Spanish title %q", got)
	}
}

// Every word printed is in the Go font; it has no ⅓ or ⅔ glyph, so all
// fractions are written with a slash, ½ ¼ ¾ included, to match.
func TestConversionsGlyphs(t *testing.T) {
	f, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	has := func(r rune) bool {
		i, err := f.GlyphIndex(&buf, r)
		return err == nil && i != 0
	}
	if has('⅓') || has('⅔') {
		t.Errorf("the Go font has ⅓ and ⅔: fractions could use glyphs")
	}
	for name, labels := range map[string]PageLabels{"English": {}, "Spanish": {Conversions: spanishConversionLabels, Language: "es"}} {
		for _, tb := range conversionTables(labels.resolved()) {
			texts := append([]string{tb.Heading, tb.Caption}, tb.Header...)
			for _, row := range tb.Rows {
				texts = append(texts, row...)
			}
			for _, s := range texts {
				if strings.ContainsAny(s, "½¼¾⅓⅔") {
					t.Errorf("%s: %q uses a fraction glyph", name, s)
				}
				for _, r := range s {
					if !has(r) {
						t.Errorf("%s: %q: no glyph for %q", name, s, r)
					}
				}
			}
		}
	}
}

// On every paper, in any lettering font and in both languages, the
// headings and every cell fit their boxes, the tables stay in the space
// below the title band without overlapping, and they fill most of it.
func TestConversionsLayoutFits(t *testing.T) {
	for _, font := range []string{DefaultLetteringFont, "pacifico", "bungee", "amatic-sc"} {
		l, lm, tm := quoteMeasures(t, font, false)
		for _, p := range Papers {
			for name, labels := range map[string]PageLabels{"en": {}, "es": {Conversions: spanishConversionLabels, Language: "es"}} {
				t.Run(fmt.Sprintf("%s/%s/%s", font, p.ID, name), func(t *testing.T) {
					s := ConversionsOptions{}.sheet(p.Page, l, lm, tm, labels.resolved())
					area := moonArea(p.Page)
					inside := func(what string, b Box) {
						const eps = 1e-6
						if b.X < area.X-eps || b.Y < area.Y-eps || b.X+b.W > area.X+area.W+eps || b.Y+b.H > area.Y+area.H+eps {
							t.Errorf("%s %+v outside the area %+v", what, b, area)
						}
					}
					if len(s.Headings) != 4 || len(s.Tables) != 4 {
						t.Fatalf("%d headings, %d tables, want 4 each", len(s.Headings), len(s.Tables))
					}
					for i, h := range s.Headings {
						inside("heading "+h.Text, h.Box)
						if w := lm(h.Text, false, h.Size); w > l.fitWidth(h.Box.W)+fitTolerance {
							t.Errorf("heading %q %.1f mm wide in %.1f mm", h.Text, w, h.Box.W)
						}
						if lineHeight(h.Size) > h.Box.H+fitTolerance || h.Size < conversionsMinHeading {
							t.Errorf("heading %q at %v pt in a %.1f mm band", h.Text, h.Size, h.Box.H)
						}
						if tb := s.Tables[i]; h.Box.Y+h.Box.H > tb.Y+fitTolerance || h.Box.X != tb.X || h.Box.W != tb.W {
							t.Errorf("heading %q %+v not just above its table %+v", h.Text, h.Box, tb)
						}
					}
					for i, tb := range s.Tables {
						inside("table", tb)
						for j, other := range s.Tables[i+1:] {
							if tb.X < other.X+other.W && other.X < tb.X+tb.W && tb.Y < other.Y+other.H && other.Y < tb.Y+tb.H {
								t.Errorf("tables %d and %d overlap", i, i+1+j)
							}
						}
					}
					for _, txt := range s.Grid.Texts {
						inside("text "+txt.Text, txt.Box)
						if w := tm(txt.Text, false, txt.Size); w > txt.Box.W+fitTolerance {
							t.Errorf("%q %.1f mm wide in %.1f mm", txt.Text, w, txt.Box.W)
						}
						if lineHeight(txt.Size) > txt.Box.H+fitTolerance || txt.Size < conversionsMinText {
							t.Errorf("%q at %v pt in a %.1f mm row", txt.Text, txt.Size, txt.Box.H)
						}
						for _, h := range s.Headings {
							if txt.Box.Y < h.Box.Y+h.Box.H-fitTolerance && h.Box.Y < txt.Box.Y+txt.Box.H-fitTolerance && txt.Box.X < h.Box.X+h.Box.W && h.Box.X < txt.Box.X+txt.Box.W {
								t.Errorf("%q overlaps heading %q", txt.Text, h.Text)
							}
						}
					}
					// Every cell of the 9, 5, 10 and 11 rows but the blank
					// corner of the cups header, plus the oven caption.
					if got, want := len(s.Grid.Texts), 9*2+5*2+10*3+11*2-1+1; got != want {
						t.Errorf("%d texts, want %d", got, want)
					}
					top, bottom := area.Y+area.H, area.Y
					for _, h := range s.Headings {
						top = min(top, h.Box.Y)
					}
					for _, txt := range s.Grid.Texts {
						bottom = max(bottom, txt.Box.Y+txt.Box.H)
					}
					if used := (bottom - top) / area.H; used < 0.9 {
						t.Errorf("the tables use %.0f%% of the height", used*100)
					}
				})
			}
		}
	}
}

// Header rows have a light fill, left off with Colour-in; the headings are
// hollow with Colour-in in a font that can be outlined, at 14 pt or more.
func TestConversionsColourIn(t *testing.T) {
	for _, p := range Papers {
		filled := ConversionsOptions{}.sheet(p.Page, newLettering(DefaultLetteringFont, false), measureStub, measureStub, PageLabels{}.resolved())
		if len(filled.Fills) != 2 {
			t.Errorf("%s: %d fills, want the 2 header rows", p.ID, len(filled.Fills))
		}
		hollow := ConversionsOptions{}.sheet(p.Page, newLettering(DefaultLetteringFont, true), measureStub, measureStub, PageLabels{}.resolved())
		if len(hollow.Fills) != 0 {
			t.Errorf("%s: %d fills with Colour-in", p.ID, len(hollow.Fills))
		}
	}
	page := PageSettings{Paper: "a4", Font: "pacifico", ColourIn: true}
	stream := pageStreams(renderPage(t, ConversionsKind, page, ""))
	l, lm, tm := quoteMeasures(t, "pacifico", true)
	want := 1 // the title
	for _, h := range (ConversionsOptions{}).sheet(A4, l, lm, tm, PageLabels{}.resolved()).Headings {
		if l.hollow(h.Size) {
			want++
		}
	}
	if want < 2 {
		t.Errorf("no hollow heading on A4")
	}
	if got := trPairs(t, "colour-in", stream); got != want {
		t.Errorf("%d hollow texts, want %d", got, want)
	}
	if strings.Count(stream, " re f") != 0 {
		t.Errorf("header fills with Colour-in")
	}
	page.ColourIn = false
	stream = pageStreams(renderPage(t, ConversionsKind, page, ""))
	if strings.Contains(stream, " Tr") {
		t.Errorf("filled headings use a text rendering mode")
	}
	if got := strings.Count(stream, " re f"); got != 2 {
		t.Errorf("%d filled rectangles, want the 2 header rows", got)
	}
}

// measureStub measures text as 1 mm per rune per 10 pt.
func measureStub(text string, _ bool, pt float64) float64 {
	return float64(len([]rune(text))) * pt / 10
}

// The page builds on every paper with and without a title, in English by
// default and differently with the Spanish labels.
func TestBuildConversionsEveryPaper(t *testing.T) {
	for _, p := range Papers {
		for _, title := range []string{"", "In the kitchen"} {
			pdf, err := buildPage(ConversionsKind, PageSettings{Paper: p.ID, Title: title}, nil)
			if err != nil {
				t.Fatalf("%s %q: %v", p.ID, title, err)
			}
			if n := pdf.PageCount(); n != 1 {
				t.Errorf("%s %q: %d pages", p.ID, title, n)
			}
		}
	}
	en := renderPage(t, ConversionsKind, PageSettings{Paper: "a5"}, "")
	if got := renderPage(t, ConversionsKind, PageSettings{Paper: "a5", Labels: PageLabels{Conversions: ConversionLabels{Title: "Kitchen conversions"}}}, ""); string(got) != string(en) {
		t.Errorf("the explicit English title changes the PDF")
	}
	if got := renderPage(t, ConversionsKind, PageSettings{Paper: "a5", Labels: PageLabels{Conversions: spanishConversionLabels}}, ""); string(got) == string(en) {
		t.Errorf("the Spanish labels do not change the PDF")
	}
	// No title argument: draw still adds the lettering font for the
	// headings.
	pdf := newPDF(A5)
	if err := (ConversionsOptions{}).draw(pdf, A5, "", newLettering("pacifico", false), PageLabels{}.resolved()); err != nil {
		t.Fatalf("draw without a title: %v", err)
	}
}
