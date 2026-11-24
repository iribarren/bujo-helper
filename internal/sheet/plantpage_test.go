package sheet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// plantsJSON is the JSON of plant care card options.
func plantsJSON(plants []Plant, size string) string {
	raw, _ := json.Marshal(PlantsOptions{Plants: plants, Size: size})
	return string(raw)
}

// shortPlant is a plant whose name and fields fit their lines.
func shortPlant(name string) Plant {
	return Plant{Name: name, Light: "Bright, indirect", Water: "Weekly", Soil: "Peat-free mix", Notes: "Mist in summer."}
}

// longPlant is a plant at every limit.
func longPlant() Plant {
	return Plant{
		Name:  repeatTo("Variegated Swiss cheese plant ", MaxPlantNameRunes),
		Light: repeatTo("Bright but never direct sun ", MaxPlantFieldRunes),
		Water: repeatTo("When the top inch is dry ", MaxPlantFieldRunes),
		Soil:  repeatTo("Chunky aroid mix with bark ", MaxPlantFieldRunes),
		Notes: repeatTo("Wipe the leaves and turn the pot every month. ", MaxPlantNotesRunes),
	}
}

// plantMeasures returns the lettering of font and the measures of a
// plant card: the lettering, the Go font and its italic.
func plantMeasures(t *testing.T, font string, colourIn bool) (lettering, plantMeasure) {
	t.Helper()
	pdf := newPDF(A4)
	if err := addPlantTextFonts(pdf, true); err != nil {
		t.Fatal(err)
	}
	l := newLettering(font, colourIn)
	if err := l.add(pdf); err != nil {
		t.Fatal(err)
	}
	return l, plantMeasure{title: l.measure(pdf), text: pdfMeasure(pdf), italic: pdfItalicMeasure(pdf)}
}

// plantCards are the cards of every sheet of o on page.
func plantCards(t *testing.T, o PlantsOptions, page Page, font string) []plantCard {
	t.Helper()
	l, m := plantMeasures(t, font, false)
	var out []plantCard
	for _, s := range o.sheets(page, l, m, PageLabels{}.resolved()) {
		out = append(out, s.Cards...)
	}
	return out
}

// plantTexts returns the texts of part on cards, in order.
func plantTexts(cards []plantCard, part plantPart) []string {
	var out []string
	for _, c := range cards {
		for _, x := range c.Texts {
			if x.Part == part {
				out = append(out, x.Text)
			}
		}
	}
	return out
}

func TestDecodePlantsOptions(t *testing.T) {
	kind, err := PageKindByID(PlantsKind)
	if err != nil {
		t.Fatal(err)
	}
	long := func(n int) string { return strings.Repeat("a", n) }
	one := func(p Plant) string { return plantsJSON([]Plant{p}, RecipesA6) }
	second := func(p Plant) string { return plantsJSON([]Plant{shortPlant("Fern"), p}, RecipesA6) }
	opts := func(size string, plants ...Plant) PlantsOptions {
		return PlantsOptions{Plants: append([]Plant{}, plants...), Size: size}
	}
	many := func(n int) string {
		ps := make([]Plant, n)
		for i := range ps {
			ps[i] = shortPlant(fmt.Sprintf("Plant %d", i+1))
		}
		return plantsJSON(ps, RecipesA6)
	}
	image := filepath.Join(t.TempDir(), "Monstera - Monstera deliciosa.jpg")
	tests := []struct {
		name    string
		raw     string
		want    PlantsOptions
		wantErr string
	}{
		{"no options take the defaults", "", opts(RecipesA6), ""},
		{"null takes the defaults", "null", opts(RecipesA6), ""},
		{"empty object takes the defaults", "{}", opts(RecipesA6), ""},
		{"4 x 6 in", `{"size":"4x6"}`, opts(Recipes4x6), ""},
		{"full page", `{"size":"page"}`, opts(RecipesPage), ""},
		{"a plant", one(shortPlant("Fern")), opts(RecipesA6, shortPlant("Fern")), ""},
		{"text normalised",
			`{"plants":[{"name":"  Boston\tfern\n","light":" Shade ","water":"Keep\u0007moist","soil":"","notes":" Mist.\n\nOften. ","image":""}]}`,
			opts(RecipesA6, Plant{Name: "Boston fern", Light: "Shade", Water: "Keep moist", Notes: "Mist. Often."}), ""},
		{"image path only trimmed", one(Plant{Image: "  " + image + " "}), opts(RecipesA6, Plant{Image: image}), ""},
		{"PNG and upper-case extensions", second(Plant{Image: "/plates/Basil.PNG"}), opts(RecipesA6, shortPlant("Fern"), Plant{Image: "/plates/Basil.PNG"}), ""},
		{"JPEG extension", one(Plant{Image: "/plates/Basil.jpeg"}), opts(RecipesA6, Plant{Image: "/plates/Basil.jpeg"}), ""},
		{"empty plants dropped", `{"plants":[{"name":" ","light":"\t"},{"water":"Daily"},{}]}`, opts(RecipesA6, Plant{Water: "Daily"}), ""},
		{"format verbs kept as text", one(Plant{Name: "100% %s", Notes: "%v %d"}), opts(RecipesA6, Plant{Name: "100% %s", Notes: "%v %d"}), ""},
		{"at every limit", one(longPlant()), opts(RecipesA6, longPlant()), ""},
		{"limits count runes, not bytes", one(Plant{Name: strings.Repeat("é", MaxPlantNameRunes)}), opts(RecipesA6, Plant{Name: strings.Repeat("é", MaxPlantNameRunes)}), ""},
		{"50 plants", many(MaxPlants), PlantsOptions{}, ""},
		{"51 plants", many(MaxPlants + 1), PlantsOptions{}, "51 plants, want at most 50"},
		{"name too long", second(Plant{Name: long(MaxPlantNameRunes + 1)}), PlantsOptions{}, "plant 2 name: 101 characters, want at most 100"},
		{"light too long", second(Plant{Light: long(MaxPlantFieldRunes + 1)}), PlantsOptions{}, "plant 2 light: 201 characters, want at most 200"},
		{"water too long", one(Plant{Water: long(MaxPlantFieldRunes + 1)}), PlantsOptions{}, "plant 1 water: 201 characters"},
		{"soil too long", one(Plant{Soil: long(MaxPlantFieldRunes + 1)}), PlantsOptions{}, "plant 1 soil: 201 characters"},
		{"notes too long", one(Plant{Notes: long(MaxPlantNotesRunes + 1)}), PlantsOptions{}, "plant 1 notes: 601 characters, want at most 600"},
		{"image path too long", second(Plant{Image: "/" + long(MaxPlantImageBytes) + ".jpg"}), PlantsOptions{}, "plant 2 image: 4101 bytes, want at most 4096"},
		{"relative image path", second(Plant{Image: "fern.jpg"}), PlantsOptions{}, "plant 2 image: want an absolute path"},
		{"not a JPEG or PNG", second(Plant{Image: "/plates/fern.gif"}), PlantsOptions{}, "plant 2 image: want a JPEG or PNG file"},
		{"no extension", one(Plant{Image: "/plates/fern"}), PlantsOptions{}, "plant 1 image: want a JPEG or PNG file"},
		{"unknown size", `{"size":"a5"}`, PlantsOptions{}, `"a5"`},
		{"empty size", `{"size":""}`, PlantsOptions{}, `size ""`},
		{"unknown field", `{"plants":[],"colour":"red"}`, PlantsOptions{}, `"colour"`},
		{"unknown plant field", `{"plants":[{"name":"Fern","humidity":"high"}]}`, PlantsOptions{}, `"humidity"`},
		{"plants not a list", `{"plants":"Fern"}`, PlantsOptions{}, "plants"},
		{"trailing data", `{} {}`, PlantsOptions{}, "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := kind.decode(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				// Errors name the plant, never echo its text.
				if len(err.Error()) > 120 {
					t.Errorf("error of %d bytes: %.120s…", len(err.Error()), err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, ok := doc.(PlantsOptions)
			if tt.name == "50 plants" {
				if !ok || len(got.Plants) != MaxPlants {
					t.Errorf("decoded %d plants, want %d", len(got.Plants), MaxPlants)
				}
				return
			}
			if !ok || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %#v, want %#v", doc, tt.want)
			}
		})
	}
	if len(defaultPlants.Plants) != 0 {
		t.Errorf("decoding changed the defaults: %v", defaultPlants.Plants)
	}
}

func TestPlantsRegistryEntry(t *testing.T) {
	k, err := PageKindByID(PlantsKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Plant care cards" || k.FileTag != "plants" || k.UsesTitle || k.UsesCut {
		t.Errorf("plants kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{"plants":[],"size":"a6"}` {
		t.Errorf("defaults %s", raw)
	}
	if last := PageKinds[len(PageKinds)-1]; last.ID != PlantsKind || PageKinds[len(PageKinds)-2].ID != ConversionsKind {
		t.Errorf("plants kind not last, right after the kitchen conversions")
	}
}

// The field labels come from the labels, English by default.
func TestPlantsLabels(t *testing.T) {
	en := PageLabels{}.resolved()
	if en.PlantLight != "Light" || en.PlantWater != "Water" || en.PlantSoil != "Soil" || en.PlantNotes != "Notes" {
		t.Errorf("English plant labels %+v", en)
	}
	es := PageLabels{PlantLight: "Luz", PlantWater: " Riego ", PlantSoil: "Sustrato", PlantNotes: "Notas"}.resolved()
	if es.PlantLight != "Luz" || es.PlantWater != "Riego" || es.PlantSoil != "Sustrato" || es.PlantNotes != "Notas" {
		t.Errorf("Spanish plant labels %+v", es)
	}
	if blank := (PageLabels{PlantSoil: "\t"}).resolved(); blank.PlantSoil != "Soil" {
		t.Errorf("blank plant label %q", blank.PlantSoil)
	}
	l, m := plantMeasures(t, DefaultLetteringFont, false)
	for _, p := range Papers {
		for _, size := range RecipeSizes {
			c := PlantsOptions{Plants: []Plant{}, Size: size}.sheets(p.Page, l, m, es)[0].Cards[0]
			if got := plantTexts([]plantCard{c}, plantLabel); !reflect.DeepEqual(got, []string{"Luz", "Riego", "Sustrato", "Notas"}) {
				t.Errorf("%s/%s: Spanish labels print as %q", p.ID, size, got)
			}
		}
	}
}

// Cards sit in the recipe card slots, one plant a card, blank cards
// filling the last page; PageCount matches the sheets and the rendered
// pages.
func TestPlantCardSlots(t *testing.T) {
	plants := make([]Plant, 5)
	for i := range plants {
		plants[i] = shortPlant(fmt.Sprintf("Plant %d", i+1))
	}
	l, m := plantMeasures(t, DefaultLetteringFont, false)
	for _, p := range Papers {
		for _, size := range RecipeSizes {
			for _, n := range []int{0, 1, 5} {
				t.Run(fmt.Sprintf("%s/%s/%d", p.ID, size, n), func(t *testing.T) {
					o := PlantsOptions{Plants: plants[:n], Size: size}
					slots := recipeSlots(p.Page, size)
					sheets := o.sheets(p.Page, l, m, PageLabels{}.resolved())
					want := max(1, (n+len(slots)-1)/len(slots))
					if len(sheets) != want {
						t.Fatalf("%d sheets, want %d", len(sheets), want)
					}
					plant := 0
					for _, s := range sheets {
						if len(s.Cards) != len(slots) {
							t.Fatalf("%d cards on a sheet, want %d", len(s.Cards), len(slots))
						}
						for i, c := range s.Cards {
							if c.Card != slots[i] {
								t.Errorf("card %d at %+v, want slot %+v", i, c.Card, slots[i])
							}
							if c.Cut == (size == RecipesPage) {
								t.Errorf("card %d cut line %v at size %s", i, c.Cut, size)
							}
							if c.Plant != 0 {
								plant++
								if c.Plant != plant {
									t.Errorf("card %d holds plant %d, want %d", i, c.Plant, plant)
								}
							}
						}
					}
					if plant != n {
						t.Errorf("%d plants printed, want %d", plant, n)
					}
					opts := plantsJSON(plants[:n], size)
					page := PageSettings{Paper: p.ID}
					got, err := PageCount(PlantsKind, page, json.RawMessage(opts))
					if err != nil || got != want {
						t.Errorf("PageCount = %d, %v, want %d", got, err, want)
					}
					if r := len(pageStreamsEach(renderPage(t, PlantsKind, page, opts))); r != want {
						t.Errorf("rendered %d pages, want %d", r, want)
					}
				})
			}
		}
	}
}

// checkPlantCard checks that nothing on card c leaves its inner box or
// overlaps anything else, that text boxes are as wide as they measure and
// that every field line sits on a rule.
func checkPlantCard(t *testing.T, c plantCard, m plantMeasure) {
	t.Helper()
	boxes := []Box{}
	if c.Title.Text != "" {
		if w := m.title(c.Title.Text, false, c.TitleSize); !near(w, c.Title.Box.W) {
			t.Errorf("name %q: box %v mm wide, measures %v", c.Title.Text, c.Title.Box.W, w)
		}
		boxes = append(boxes, c.Title.Box)
	}
	if c.Plate != (Box{}) {
		boxes = append(boxes, c.Plate)
	}
	onRule := func(y float64) bool {
		for _, r := range c.Rules {
			if near(r.Y, y) {
				return true
			}
		}
		return false
	}
	for _, r := range c.Rules {
		if r.X1 < c.Inner.X-1e-6 || r.X2 > c.Inner.X+c.Inner.W+1e-6 || r.X2 <= r.X1 || r.Y < c.Inner.Y-1e-6 || r.Y > c.Inner.Y+c.Inner.H+1e-6 {
			t.Errorf("rule %+v leaves the inner box %+v", r, c.Inner)
		}
	}
	for _, x := range c.Texts {
		measure := m.text
		if x.Part == plantScientific {
			measure = m.italic
		}
		if w := measure(x.Text, false, x.Size); !near(w, x.Box.W) {
			t.Errorf("text %q: box %v mm wide, measures %v", x.Text, x.Box.W, w)
		}
		if !near(x.Box.H, lineHeight(x.Size)) {
			t.Errorf("text %q: %v mm high, want %v", x.Text, x.Box.H, lineHeight(x.Size))
		}
		if (x.Part == plantField || x.Part == plantLabel) && !onRule(x.Box.Y+x.Box.H) {
			t.Errorf("text %q at %v mm: not on a rule", x.Text, x.Box.Y+x.Box.H)
		}
		boxes = append(boxes, x.Box)
	}
	for i, b := range boxes {
		if !insideBox(b, c.Inner) {
			t.Errorf("box %d %+v leaves the inner box %+v", i, b, c.Inner)
		}
		for j, d := range boxes[:i] {
			const eps = 1e-6
			if b.X < d.X+d.W-eps && d.X < b.X+b.W-eps && b.Y < d.Y+d.H-eps && d.Y < b.Y+b.H-eps {
				t.Errorf("boxes %d %+v and %d %+v overlap", j, d, i, b)
			}
		}
	}
}

// Text never leaves a card, whatever the font, paper and size: long names
// shrink and are cut with "…", long field text is cut with "…" on its
// last rule, and the plate sits at the top, the name below it.
func TestPlantsTextFits(t *testing.T) {
	long := longPlant()
	plate := longPlant()
	plate.Image = "/plates/" + repeatTo("Common name ", 60) + " - " + repeatTo("Scientificus nominus ", 300) + ".jpg"
	plate.Name = ""
	short := shortPlant("Fern")
	short.Image = "/plates/fern.jpg"
	for _, font := range []string{DefaultLetteringFont, "pacifico", "bungee", "amatic-sc"} {
		_, m := plantMeasures(t, font, false)
		for _, p := range Papers {
			for _, size := range RecipeSizes {
				t.Run(fmt.Sprintf("%s/%s/%s", font, p.ID, size), func(t *testing.T) {
					o := PlantsOptions{Plants: []Plant{short, long, plate}, Size: size}
					cards := plantCards(t, o, p.Page, font)
					for _, c := range cards {
						checkPlantCard(t, c, m)
					}
					first, second, third := cards[0], cards[1], cards[2]
					if first.Title.Text != "Fern" || first.TitleSize != plantStyles[size].nameMax {
						t.Errorf("short name %q at %v pt", first.Title.Text, first.TitleSize)
					}
					if first.Plate == (Box{}) || first.Plate.Y != first.Inner.Y || first.Title.Box.Y < first.Plate.Y+first.Plate.H {
						t.Errorf("plate %+v not above the name %+v", first.Plate, first.Title.Box)
					}
					if got := plantTexts([]plantCard{first}, plantField); !reflect.DeepEqual(got, []string{short.Light, short.Water, short.Soil, short.Notes}) {
						t.Errorf("short fields print as %q", got)
					}
					if second.Plate != (Box{}) || second.Title.Box.Y < second.Inner.Y || second.Title.Box.Y > second.Inner.Y+lineHeight(plantStyles[size].nameMax) {
						t.Errorf("without a plate the name %+v is not at the top of %+v", second.Title.Box, second.Inner)
					}
					// A condensed font may fit the whole name once shrunk.
					if second.Title.Text != long.Name && !strings.HasSuffix(second.Title.Text, ellipsis) {
						t.Errorf("long name %q neither whole nor cut", second.Title.Text)
					}
					if fields := plantTexts([]plantCard{second}, plantField); len(fields) < 4 {
						t.Errorf("long fields print as %q", fields)
					}
					// A long light is cut on its last rule, its start kept.
					light := plantCards(t, PlantsOptions{Plants: []Plant{{Light: long.Light}}, Size: size}, p.Page, font)[0]
					lines := plantTexts([]plantCard{light}, plantField)
					joined := strings.TrimSuffix(noSpaces(strings.Join(lines, "")), ellipsis)
					whole := joined == noSpaces(long.Light)
					if len(lines) == 0 || !whole && !strings.HasSuffix(lines[len(lines)-1], ellipsis) || !strings.HasPrefix(noSpaces(long.Light), joined) {
						t.Errorf("long light prints as %q", lines)
					}
					sci := plantTexts([]plantCard{third}, plantScientific)
					if len(sci) != 1 || !strings.HasPrefix(sci[0], "Scientificus") || !strings.HasSuffix(sci[0], ellipsis) {
						t.Errorf("long scientific name prints as %q", sci)
					}
					if !strings.HasPrefix(third.Title.Text, "Common name") {
						t.Errorf("derived name %q", third.Title.Text)
					}
				})
			}
		}
	}
}

// No plants print a page of blank cards: a writing line for the name, then
// the labels over writing lines, Notes filling the card.
func TestPlantsBlankCards(t *testing.T) {
	_, m := plantMeasures(t, DefaultLetteringFont, false)
	for _, p := range Papers {
		for _, size := range RecipeSizes {
			cards := plantCards(t, PlantsOptions{Plants: []Plant{}, Size: size}, p.Page, DefaultLetteringFont)
			if len(cards) != len(recipeSlots(p.Page, size)) {
				t.Fatalf("%s/%s: %d blank cards", p.ID, size, len(cards))
			}
			for _, c := range cards {
				checkPlantCard(t, c, m)
				if c.Plant != 0 || c.Title.Text != "" || c.Plate != (Box{}) || len(plantTexts([]plantCard{c}, plantField)) != 0 {
					t.Errorf("%s/%s: blank card %+v", p.ID, size, c)
				}
				if got := plantTexts([]plantCard{c}, plantLabel); !reflect.DeepEqual(got, []string{"Light", "Water", "Soil", "Notes"}) {
					t.Errorf("%s/%s: labels %q", p.ID, size, got)
				}
				// The name line, a line each for light, water and soil at
				// least, and several for the notes.
				if len(c.Rules) < 7 {
					t.Errorf("%s/%s: %d writing lines", p.ID, size, len(c.Rules))
				}
				last := c.Rules[len(c.Rules)-1]
				if pitch := lineHeight(plantStyles[size].bodySize) * recipeBlankPitch; c.Inner.Y+c.Inner.H-last.Y > pitch+1e-6 {
					t.Errorf("%s/%s: notes stop %v mm above the card's end", p.ID, size, c.Inner.Y+c.Inner.H-last.Y)
				}
			}
		}
	}
}

// With no name typed, the name comes from the image's file name, "Common
// name - Scientific name.jpg": the common name in the lettering, the
// scientific name in italics below it. A typed name wins.
func TestPlantsNameFromFileName(t *testing.T) {
	tests := []struct {
		name        string
		plant       Plant
		title, sci  string
		wantNameBox bool
	}{
		{"common and scientific", Plant{Image: "/plates/Monstera - Monstera deliciosa.jpg"}, "Monstera", "Monstera deliciosa", true},
		{"duplicate suffix dropped", Plant{Image: "/plates/Sweet basil - Ocimum basilicum-2.png"}, "Sweet basil", "Ocimum basilicum", true},
		{"no scientific name", Plant{Image: "/plates/Basil.png"}, "Basil", "", true},
		{"typed name wins", Plant{Name: "My fern", Image: "/plates/Monstera - Monstera deliciosa.jpg"}, "My fern", "", true},
		{"no name, no image", Plant{Water: "Daily"}, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := plantCards(t, PlantsOptions{Plants: []Plant{tt.plant}, Size: RecipesA6}, A4, DefaultLetteringFont)[0]
			if c.Title.Text != tt.title {
				t.Errorf("name %q, want %q", c.Title.Text, tt.title)
			}
			sci := strings.Join(plantTexts([]plantCard{c}, plantScientific), "|")
			if sci != tt.sci {
				t.Errorf("scientific name %q, want %q", sci, tt.sci)
			}
			for _, x := range c.Texts {
				if x.Part == plantScientific && (!x.Italic || x.Box.Y < c.Title.Box.Y+c.Title.Box.H-1e-6 || x.Size >= c.TitleSize) {
					t.Errorf("scientific name %+v not small and italic below the name %+v", x, c.Title.Box)
				}
			}
			// With nothing to print, the name is a writing line.
			if !tt.wantNameBox && len(c.Rules) == 0 {
				t.Errorf("no writing line for the name")
			}
		})
	}
}

// The plate keeps its aspect, as large as fits, centred in its box.
func TestPlantPlateFitsItsBox(t *testing.T) {
	box := Box{X: 10, Y: 20, W: 100, H: 40}
	tests := []struct {
		ratio float64
		want  Box
	}{
		{0.5, Box{X: 50, Y: 20, W: 20, H: 40}},
		{2.5, Box{X: 10, Y: 20, W: 100, H: 40}},
		{5, Box{X: 10, Y: 30, W: 100, H: 20}},
	}
	for _, tt := range tests {
		if got := containBox(box, tt.ratio); !near(got.X, tt.want.X) || !near(got.Y, tt.want.Y) || !near(got.W, tt.want.W) || !near(got.H, tt.want.H) {
			t.Errorf("containBox(%v) = %+v, want %+v", tt.ratio, got, tt.want)
		}
	}
}

// Every size builds on every paper with JPEG and PNG plates: dashed cut
// lines around cards (none at page size), the plates drawn, hollow text
// paired.
func TestBuildPlantsEverySize(t *testing.T) {
	dir := t.TempDir()
	jpg := writeImage(t, dir, "Monstera - Monstera deliciosa.jpg", 60, 120, color.RGBA{40, 140, 60, 255})
	png := writeImage(t, dir, "Basil.png", 90, 40, color.RGBA{60, 160, 40, 255})
	plants := []Plant{{Image: jpg, Water: "Weekly"}, {Image: png}, shortPlant("Fern")}
	for _, p := range Papers {
		for _, size := range RecipeSizes {
			t.Run(p.ID+"/"+size, func(t *testing.T) {
				opts := plantsJSON(plants, size)
				page := PageSettings{Paper: p.ID, Font: "kaushan-script", ColourIn: true}
				pages := pageStreamsEach(renderPage(t, PlantsKind, page, opts))
				want, err := PageCount(PlantsKind, page, json.RawMessage(opts))
				if err != nil || len(pages) != want {
					t.Fatalf("%d pages, PageCount %d, %v", len(pages), want, err)
				}
				all := strings.Join(pages, "\n")
				if n := strings.Count(all, "/I Do"); n != 2 {
					t.Errorf("%d images drawn, want 2", n)
				}
				dashed := strings.Count(all, "] 0.00 d") - strings.Count(all, "[] 0.00 d")
				if size == RecipesPage && dashed != 0 {
					t.Errorf("page size draws %d dashed cut lines", dashed)
				}
				if size != RecipesPage && dashed == 0 {
					t.Errorf("no dashed cut line")
				}
				for i, s := range pages {
					trPairs(t, fmt.Sprintf("page %d", i), s)
				}
			})
		}
	}
}

// An image that is missing or not an image fails the build with an error
// naming the plant.
func TestPlantsBadImage(t *testing.T) {
	dir := t.TempDir()
	notImage := filepath.Join(dir, "notes.jpg")
	if err := os.WriteFile(notImage, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"missing": filepath.Join(dir, "gone.jpg"), "not an image": notImage} {
		t.Run(name, func(t *testing.T) {
			opts := plantsJSON([]Plant{shortPlant("Fern"), {Image: path}}, RecipesA6)
			_, err := buildPage(PlantsKind, PageSettings{}, json.RawMessage(opts))
			if err == nil || !strings.Contains(err.Error(), "plant 2") || !strings.Contains(err.Error(), filepath.Base(path)) {
				t.Errorf("err = %v, want one naming plant 2 and its image", err)
			}
		})
	}
}

// Names print in the lettering, hollow with Colour-in at 14 pt or more,
// even though the page has no shared title; without Colour-in nothing is
// hollow.
func TestPlantsHollowName(t *testing.T) {
	opts := plantsJSON([]Plant{shortPlant("Fern"), shortPlant("Ivy")}, RecipesA6)
	stream := pageStreams(renderPage(t, PlantsKind, PageSettings{Font: "pacifico", ColourIn: true}, opts))
	if !strings.Contains(stream, "1 Tr") {
		t.Errorf("no hollow name with Colour-in")
	}
	if got := trPairs(t, "colour-in", stream); got != 2 {
		t.Errorf("%d hollow names, want 2", got)
	}
	if strings.Contains(pageStreams(renderPage(t, PlantsKind, PageSettings{Font: "pacifico"}, opts)), " Tr") {
		t.Errorf("filled names use a text rendering mode")
	}
	o := PlantsOptions{Plants: []Plant{shortPlant("Fern")}, Size: RecipesA6}
	if err := o.draw(newPDF(A5), A5, "", newLettering("pacifico", false), PageLabels{}.resolved()); err != nil {
		t.Fatalf("draw: %v", err)
	}
}

// The same plants give the same PDF bytes, plates included.
func TestBuildPlantsIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	jpg := writeImage(t, dir, "Monstera - Monstera deliciosa.jpg", 60, 120, color.RGBA{40, 140, 60, 255})
	opts := json.RawMessage(plantsJSON([]Plant{{Image: jpg}, shortPlant("Fern")}, RecipesA6))
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var outs [2][]byte
	for i := range outs {
		pdf, err := buildPage(PlantsKind, PageSettings{Font: "pacifico", ColourIn: true}, opts)
		if err != nil {
			t.Fatal(err)
		}
		pdf.SetCreationDate(when)
		pdf.SetModificationDate(when)
		var buf bytes.Buffer
		if err := pdf.Output(&buf); err != nil {
			t.Fatal(err)
		}
		outs[i] = buf.Bytes()
	}
	if !bytes.Equal(outs[0], outs[1]) {
		t.Errorf("two builds of the same plant cards differ")
	}
}
