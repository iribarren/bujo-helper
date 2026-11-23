package sheet

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// recipesJSON is the JSON of recipe card options.
func recipesJSON(recipes []Recipe, size string) string {
	raw, _ := json.Marshal(RecipesOptions{Recipes: recipes, Size: size})
	return string(raw)
}

// shortRecipe is a recipe that fits one card at the largest size.
func shortRecipe(title string) Recipe {
	return Recipe{
		Title:       title,
		Servings:    "4",
		Time:        "30 min",
		Ingredients: []string{"2 eggs", "100 g flour", "1 cup milk"},
		Steps:       []string{"Whisk everything together.", "Fry in a hot pan."},
	}
}

// repeatTo is s repeated and cut to n bytes (ASCII), ending in "x" so
// normalising keeps it whole.
func repeatTo(s string, n int) string {
	return strings.Repeat(s, n/len(s)+1)[:n-1] + "x"
}

// longRecipe is a recipe at every limit: it needs continuation cards.
func longRecipe() Recipe {
	r := Recipe{
		Title:    repeatTo("Grandmother's slow-cooked ", MaxRecipeTitleRunes),
		Servings: repeatTo("6", MaxRecipeMetaRunes),
		Time:     repeatTo("2 h ", MaxRecipeMetaRunes),
	}
	for i := range MaxRecipeIngredients {
		r.Ingredients = append(r.Ingredients, repeatTo(fmt.Sprintf("ingredient %d finely chopped ", i+1), MaxRecipeIngredientRunes))
	}
	for i := range MaxRecipeSteps {
		r.Steps = append(r.Steps, repeatTo(fmt.Sprintf("Step %d stir gently and wait. ", i+1), MaxRecipeStepRunes))
	}
	return r
}

func TestDecodeRecipesOptions(t *testing.T) {
	kind, err := PageKindByID(RecipesKind)
	if err != nil {
		t.Fatal(err)
	}
	long := func(n int) string { return strings.Repeat("a", n) }
	one := func(r Recipe) string { return recipesJSON([]Recipe{r}, RecipesA6) }
	second := func(r Recipe) string { return recipesJSON([]Recipe{shortRecipe("Pancakes"), r}, RecipesA6) }
	norm := func(r Recipe) Recipe {
		if r.Ingredients == nil {
			r.Ingredients = []string{}
		}
		if r.Steps == nil {
			r.Steps = []string{}
		}
		return r
	}
	opts := func(size string, recipes ...Recipe) RecipesOptions {
		out := RecipesOptions{Recipes: []Recipe{}, Size: size}
		for _, r := range recipes {
			out.Recipes = append(out.Recipes, norm(r))
		}
		return out
	}
	many := func(n int) string {
		rs := make([]Recipe, n)
		for i := range rs {
			rs[i] = shortRecipe(fmt.Sprintf("Recipe %d", i+1))
		}
		return recipesJSON(rs, RecipesA6)
	}
	photo := filepath.Join(t.TempDir(), "soup.jpg")
	tests := []struct {
		name    string
		raw     string
		want    RecipesOptions
		wantErr string
	}{
		{"no options take the defaults", "", opts(RecipesA6), ""},
		{"null takes the defaults", "null", opts(RecipesA6), ""},
		{"empty object takes the defaults", "{}", opts(RecipesA6), ""},
		{"4 x 6 in", `{"size":"4x6"}`, opts(Recipes4x6), ""},
		{"full page", `{"size":"page"}`, opts(RecipesPage), ""},
		{"a recipe", one(shortRecipe("Pancakes")), opts(RecipesA6, shortRecipe("Pancakes")), ""},
		{"text normalised, blank lines dropped",
			`{"recipes":[{"title":"  Pan\tcakes\n","servings":" 4 ","time":"30\u0007min","ingredients":["  2  eggs ","","\t"],"steps":[" Whisk.\n\nFry. ",""],"photo":""}]}`,
			opts(RecipesA6, Recipe{Title: "Pan cakes", Servings: "4", Time: "30 min", Ingredients: []string{"2 eggs"}, Steps: []string{"Whisk. Fry."}}), ""},
		{"photo path only trimmed", one(Recipe{Title: "Soup", Photo: "  " + photo + " "}), opts(RecipesA6, Recipe{Title: "Soup", Photo: photo}), ""},
		{"empty recipes dropped", `{"recipes":[{"title":" ","servings":"4","time":"1 h","ingredients":[" "],"steps":[]},{"title":"Kept"},{}]}`,
			opts(RecipesA6, Recipe{Title: "Kept"}), ""},
		{"a recipe of only a photo is kept", one(Recipe{Photo: photo}), opts(RecipesA6, Recipe{Photo: photo}), ""},
		{"a recipe of only steps is kept", one(Recipe{Steps: []string{"Boil."}}), opts(RecipesA6, Recipe{Steps: []string{"Boil."}}), ""},
		{"format verbs kept as text", one(Recipe{Title: "100% %s", Steps: []string{"%v %d"}}), opts(RecipesA6, Recipe{Title: "100% %s", Steps: []string{"%v %d"}}), ""},
		{"at every limit", one(longRecipe()), opts(RecipesA6, longRecipe()), ""},
		{"limits count runes, not bytes", one(Recipe{Title: strings.Repeat("é", MaxRecipeTitleRunes)}), opts(RecipesA6, Recipe{Title: strings.Repeat("é", MaxRecipeTitleRunes)}), ""},
		{"50 recipes", many(MaxRecipes), RecipesOptions{}, ""},
		{"51 recipes", many(MaxRecipes + 1), RecipesOptions{}, "51 recipes, want at most 50"},
		{"title too long", second(Recipe{Title: long(MaxRecipeTitleRunes + 1)}), RecipesOptions{}, "recipe 2 title: 101 characters, want at most 100"},
		{"servings too long", second(Recipe{Title: "Soup", Servings: long(MaxRecipeMetaRunes + 1)}), RecipesOptions{}, "recipe 2 servings: 41 characters, want at most 40"},
		{"time too long", one(Recipe{Title: "Soup", Time: long(MaxRecipeMetaRunes + 1)}), RecipesOptions{}, "recipe 1 time: 41 characters, want at most 40"},
		{"too many ingredients", second(Recipe{Title: "Soup", Ingredients: strings.Split(strings.Repeat("salt,", MaxRecipeIngredients+1), ",")}), RecipesOptions{}, "recipe 2: 61 ingredients, want at most 60"},
		{"ingredient too long", second(Recipe{Title: "Soup", Ingredients: []string{"salt", long(MaxRecipeIngredientRunes + 1)}}), RecipesOptions{}, "recipe 2 ingredient 2: 201 characters, want at most 200"},
		{"too many steps", one(Recipe{Title: "Soup", Steps: strings.Split(strings.Repeat("stir,", MaxRecipeSteps+1), ",")}), RecipesOptions{}, "recipe 1: 41 steps, want at most 40"},
		{"step too long", second(Recipe{Title: "Soup", Steps: []string{long(MaxRecipeStepRunes + 1)}}), RecipesOptions{}, "recipe 2 step 1: 601 characters, want at most 600"},
		{"photo path too long", second(Recipe{Title: "Soup", Photo: "/" + long(MaxRecipePhotoBytes)}), RecipesOptions{}, "recipe 2 photo: 4097 bytes, want at most 4096"},
		{"relative photo path", second(Recipe{Title: "Soup", Photo: "soup.jpg"}), RecipesOptions{}, "recipe 2 photo: want an absolute path"},
		{"unknown size", `{"size":"a5"}`, RecipesOptions{}, `"a5"`},
		{"empty size", `{"size":""}`, RecipesOptions{}, `size ""`},
		{"unknown field", `{"recipes":[],"colour":"red"}`, RecipesOptions{}, `"colour"`},
		{"unknown recipe field", `{"recipes":[{"title":"Soup","rating":5}]}`, RecipesOptions{}, `"rating"`},
		{"recipes not a list", `{"recipes":"Soup"}`, RecipesOptions{}, "recipes"},
		{"trailing data", `{} {}`, RecipesOptions{}, "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := kind.decode(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				// Errors name the recipe, never echo its text.
				if len(err.Error()) > 120 {
					t.Errorf("error of %d bytes: %.120s…", len(err.Error()), err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, ok := doc.(RecipesOptions)
			if tt.name == "50 recipes" {
				if !ok || len(got.Recipes) != MaxRecipes {
					t.Errorf("decoded %d recipes, want %d", len(got.Recipes), MaxRecipes)
				}
				return
			}
			if !ok || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %#v, want %#v", doc, tt.want)
			}
		})
	}
	if len(defaultRecipes.Recipes) != 0 {
		t.Errorf("decoding changed the defaults: %v", defaultRecipes.Recipes)
	}
}

func TestRecipesRegistryEntry(t *testing.T) {
	k, err := PageKindByID(RecipesKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Recipe cards" || k.FileTag != "recipes" || k.UsesTitle || k.UsesCut {
		t.Errorf("recipes kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{"recipes":[],"size":"a6"}` {
		t.Errorf("defaults %s", raw)
	}
	// TestCalendarRegistryEntry checks the full order.
	for i, k := range PageKinds {
		if k.ID == RecipesKind && (i == 0 || PageKinds[i-1].ID != MapKind) {
			t.Errorf("recipes kind at %d, want right after the map", i)
		}
	}
}

// The recipe card words come from the labels, English by default.
func TestRecipesLabels(t *testing.T) {
	en := PageLabels{}.resolved()
	if en.RecipeIngredients != "Ingredients" || en.RecipeSteps != "Steps" || en.RecipeServes != "Serves" || en.RecipeTime != "Time" || en.RecipeContinued != "continued" {
		t.Errorf("English recipe labels %+v", en)
	}
	es := PageLabels{RecipeIngredients: "Ingredientes", RecipeSteps: "Pasos", RecipeServes: "Raciones", RecipeTime: " Tiempo ", RecipeContinued: "continuación"}.resolved()
	if es.RecipeIngredients != "Ingredientes" || es.RecipeSteps != "Pasos" || es.RecipeServes != "Raciones" || es.RecipeTime != "Tiempo" || es.RecipeContinued != "continuación" {
		t.Errorf("Spanish recipe labels %+v", es)
	}
	blank := PageLabels{RecipeSteps: "  ", RecipeContinued: "\t"}.resolved()
	if blank.RecipeSteps != "Steps" || blank.RecipeContinued != "continued" {
		t.Errorf("blank recipe labels %+v", blank)
	}
}

// Cards are landscape, tiled within the margins Gap apart and centred; a
// card too big for the paper is scaled down, aspect kept, one a page; the
// page size is the whole content area.
func TestRecipeCardSlots(t *testing.T) {
	tests := []struct {
		paper, size string
		perPage     int
		scaled      bool
	}{
		{"a4", RecipesA6, 2, false},
		{"letter", RecipesA6, 2, false},
		{"b5", RecipesA6, 2, false},
		{"a5", RecipesA6, 1, true},
		{"a4", Recipes4x6, 2, false},
		{"a5", Recipes4x6, 1, true},
		{"a4", RecipesPage, 1, false},
		{"a5", RecipesPage, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.paper+"/"+tt.size, func(t *testing.T) {
			p, err := PaperByID(tt.paper)
			if err != nil {
				t.Fatal(err)
			}
			page := p.Page
			slots := recipeSlots(page, tt.size)
			if len(slots) != tt.perPage {
				t.Fatalf("%d cards a page, want %d", len(slots), tt.perPage)
			}
			area := Box{X: MinMargin, Y: MinMargin, W: page.Width - 2*MinMargin, H: page.Height - 2*MinMargin}
			if tt.size == RecipesPage {
				if slots[0] != area {
					t.Errorf("page card %+v, want the content area %+v", slots[0], area)
				}
				return
			}
			w, h := recipeCardSize(tt.size)
			for i, s := range slots {
				if !insideBox(s, area) {
					t.Errorf("card %d %+v outside %+v", i, s, area)
				}
				if !near(s.W/s.H, w/h) {
					t.Errorf("card %d aspect %v, want %v", i, s.W/s.H, w/h)
				}
				if tt.scaled == near(s.W, w) {
					t.Errorf("card %d %v mm wide; scaled %v", i, s.W, tt.scaled)
				}
				if i > 0 && slots[i-1].Y+slots[i-1].H+Gap > s.Y+1e-6 {
					t.Errorf("cards %d and %d closer than Gap", i-1, i)
				}
			}
			first, last := slots[0], slots[len(slots)-1]
			if !near(first.X, page.Width-first.X-first.W) || !near(first.Y, page.Height-last.Y-last.H) {
				t.Errorf("cards %+v … %+v not centred", first, last)
			}
		})
	}
}

// checkRecipeCard checks that nothing on card c leaves its inner box or
// overlaps anything else, and that text boxes are as wide as they measure.
func checkRecipeCard(t *testing.T, c recipeCard, tm, lm measureFunc) {
	t.Helper()
	boxes := []Box{}
	if c.Title.Text != "" {
		if w := lm(c.Title.Text, false, c.TitleSize); !near(w, c.Title.Box.W) {
			t.Errorf("title %q: box %v mm wide, measures %v", c.Title.Text, c.Title.Box.W, w)
		}
		boxes = append(boxes, c.Title.Box)
	}
	if c.Photo != (Box{}) {
		boxes = append(boxes, c.Photo)
	}
	for _, x := range c.Texts {
		if w := tm(x.Text, false, x.Size); !near(w, x.Box.W) {
			t.Errorf("text %q: box %v mm wide, measures %v", x.Text, x.Box.W, w)
		}
		if !near(x.Box.H, lineHeight(x.Size)) {
			t.Errorf("text %q: %v mm high, want %v", x.Text, x.Box.H, lineHeight(x.Size))
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
	for _, r := range c.Rules {
		if r.X1 < c.Inner.X-1e-6 || r.X2 > c.Inner.X+c.Inner.W+1e-6 || r.Y < c.Inner.Y-1e-6 || r.Y > c.Inner.Y+c.Inner.H+1e-6 {
			t.Errorf("rule %+v leaves the inner box %+v", r, c.Inner)
		}
	}
}

// partTexts returns the texts of part on cards, in order.
func partTexts(cards []recipeCard, part recipePart) []string {
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

// noSpaces is s without spaces: wrapping only moves line breaks.
func noSpaces(s string) string { return strings.ReplaceAll(s, " ", "") }

// Text never leaves a card: a short recipe prints on one card at the
// largest size; a recipe at every limit shrinks to the floor and flows
// onto continuation cards titled "… (continued)", its ingredients whole
// and in order and its step numbers running on.
func TestRecipesTextFits(t *testing.T) {
	long := longRecipe()
	long.Photo = "/photo.jpg"
	for _, font := range []string{DefaultLetteringFont, "pacifico", "bungee", "amatic-sc"} {
		l, lm, tm := quoteMeasures(t, font, false)
		for _, p := range Papers {
			for _, size := range RecipeSizes {
				t.Run(fmt.Sprintf("%s/%s/%s", font, p.ID, size), func(t *testing.T) {
					o := RecipesOptions{Recipes: []Recipe{shortRecipe("Pancakes"), long}, Size: size}
					var cards []recipeCard
					for _, s := range o.sheets(p.Page, l, lm, tm, PageLabels{}.resolved()) {
						cards = append(cards, s.Cards...)
					}
					style := recipeStyles[size]
					for i, c := range cards {
						checkRecipeCard(t, c, tm, lm)
						if c.Recipe == 0 {
							continue
						}
						if c.BodySize < recipeMinBodySize-1e-9 || c.BodySize > style.bodyMax+1e-9 {
							t.Errorf("card %d body at %v pt", i, c.BodySize)
						}
						if c.Cut == (size == RecipesPage) {
							t.Errorf("card %d cut line %v at size %s", i, c.Cut, size)
						}
					}
					if cards[0].Recipe != 1 || cards[0].Continued || cards[0].BodySize != style.bodyMax || cards[0].Title.Text != "Pancakes" {
						t.Errorf("short recipe card %+v", cards[0])
					}
					if cards[1].Recipe != 2 || cards[1].Photo == (Box{}) || cards[1].BodySize != recipeMinBodySize {
						t.Errorf("long recipe's first card %+v", cards[1])
					}
					var longCards []recipeCard
					for _, c := range cards {
						if c.Recipe == 2 {
							longCards = append(longCards, c)
						}
					}
					if len(longCards) < 2 {
						t.Fatalf("long recipe on %d cards", len(longCards))
					}
					for i, c := range longCards[1:] {
						if !c.Continued || !strings.HasSuffix(c.Title.Text, "(continued)") || c.Photo != (Box{}) {
							t.Errorf("continuation card %d title %q photo %+v", i+1, c.Title.Text, c.Photo)
						}
					}
					if got, want := noSpaces(strings.Join(partTexts(longCards, recipeIngredient), "")), noSpaces(strings.Join(long.Ingredients, "")); got != want {
						t.Errorf("ingredients printed as %.80q…, want %.80q…", got, want)
					}
					if got, want := noSpaces(strings.Join(partTexts(longCards, recipeStep), "")), noSpaces(strings.Join(long.Steps, "")); got != want {
						t.Errorf("steps printed as %.80q…, want %.80q…", got, want)
					}
					var numbers []string
					for i := range long.Steps {
						numbers = append(numbers, fmt.Sprintf("%d.", i+1))
					}
					if got := partTexts(longCards, recipeNumber); !reflect.DeepEqual(got, numbers) {
						t.Errorf("step numbers %v", got)
					}
					if got := partTexts(longCards, recipeBullet); len(got) != len(long.Ingredients) {
						t.Errorf("%d bullets, want %d", len(got), len(long.Ingredients))
					}
					// Every card after the recipes' own is blank, filling the
					// last page.
					for i, c := range cards[1+len(longCards):] {
						if c.Recipe != 0 {
							t.Errorf("card %d after the recipes belongs to recipe %d", i, c.Recipe)
						}
					}
				})
			}
		}
	}
}

// The meta line is "Serves X · Time Y", leaving out the empty parts, and
// nothing at all (no room taken) when both are empty.
func TestRecipesMetaLine(t *testing.T) {
	l, lm, tm := quoteMeasures(t, DefaultLetteringFont, false)
	tests := []struct {
		servings, time, want string
	}{
		{"4", "30 min", "Serves 4 · Time 30 min"},
		{"4", "", "Serves 4"},
		{"", "1 h", "Time 1 h"},
		{"", "", ""},
	}
	for _, tt := range tests {
		r := Recipe{Title: "Soup", Servings: tt.servings, Time: tt.time, Ingredients: []string{"water"}, Steps: []string{"Boil."}}
		c := RecipesOptions{Recipes: []Recipe{r}, Size: RecipesA6}.sheets(A4, l, lm, tm, PageLabels{}.resolved())[0].Cards[0]
		got := strings.Join(partTexts([]recipeCard{c}, recipeMeta), "|")
		if got != tt.want {
			t.Errorf("meta %q, want %q", got, tt.want)
		}
	}
	es := PageLabels{RecipeServes: "Raciones", RecipeTime: "Tiempo"}.resolved()
	c := RecipesOptions{Recipes: []Recipe{shortRecipe("Sopa")}, Size: RecipesA6}.sheets(A4, l, lm, tm, es)[0].Cards[0]
	if got := partTexts([]recipeCard{c}, recipeMeta); !reflect.DeepEqual(got, []string{"Raciones 4 · Tiempo 30 min"}) {
		t.Errorf("Spanish meta %q", got)
	}
}

// No recipes print a full page of blank cards: the headings and writing
// lines to fill in by hand.
func TestRecipesBlankCards(t *testing.T) {
	l, lm, tm := quoteMeasures(t, DefaultLetteringFont, false)
	for _, p := range Papers {
		for _, size := range RecipeSizes {
			sheets := RecipesOptions{Recipes: []Recipe{}, Size: size}.sheets(p.Page, l, lm, tm, PageLabels{}.resolved())
			if len(sheets) != 1 || len(sheets[0].Cards) != len(recipeSlots(p.Page, size)) {
				t.Fatalf("%s/%s: %d sheets", p.ID, size, len(sheets))
			}
			for _, c := range sheets[0].Cards {
				checkRecipeCard(t, c, tm, lm)
				if c.Recipe != 0 || c.Title.Text != "" || c.Photo != (Box{}) {
					t.Errorf("%s/%s: blank card %+v", p.ID, size, c)
				}
				if got := partTexts([]recipeCard{c}, recipeHeading); !reflect.DeepEqual(got, []string{"Ingredients", "Steps"}) {
					t.Errorf("%s/%s: headings %q", p.ID, size, got)
				}
				if got := partTexts([]recipeCard{c}, recipeMeta); !reflect.DeepEqual(got, []string{"Serves", "Time"}) {
					t.Errorf("%s/%s: meta labels %q", p.ID, size, got)
				}
				if len(c.Rules) < 10 {
					t.Errorf("%s/%s: %d writing lines", p.ID, size, len(c.Rules))
				}
			}
		}
	}
}

// PageCount counts continuation cards: it matches the sheets and the
// rendered pages on every paper and size, whatever the lettering font.
func TestRecipesPageCount(t *testing.T) {
	long := longRecipe()
	tests := []struct {
		name    string
		recipes []Recipe
	}{
		{"none", nil},
		{"one", []Recipe{shortRecipe("Soup")}},
		{"three", []Recipe{shortRecipe("Soup"), shortRecipe("Bread"), shortRecipe("Salad")}},
		{"long", []Recipe{shortRecipe("Soup"), long, long}},
	}
	for _, p := range Papers {
		for _, size := range RecipeSizes {
			for _, tt := range tests {
				t.Run(fmt.Sprintf("%s/%s/%s", p.ID, size, tt.name), func(t *testing.T) {
					opts := recipesJSON(tt.recipes, size)
					page := PageSettings{Paper: p.ID, Font: "bungee"}
					got, err := PageCount(RecipesKind, page, json.RawMessage(opts))
					if err != nil {
						t.Fatal(err)
					}
					l, lm, tm := quoteMeasures(t, "bungee", false)
					o := RecipesOptions{Recipes: tt.recipes, Size: size}
					if want := len(o.sheets(p.Page, l, lm, tm, PageLabels{}.resolved())); got != want {
						t.Errorf("PageCount = %d, want %d sheets", got, want)
					}
					if n := len(pageStreamsEach(renderPage(t, RecipesKind, page, opts))); n != got {
						t.Errorf("rendered %d pages, PageCount %d", n, got)
					}
					if tt.name == "long" && got < 3 {
						t.Errorf("long recipes on %d pages", got)
					}
				})
			}
		}
	}
}

// Every size builds on every paper, with JPEG and PNG photos: dashed cut
// lines around cards (none at page size), the photos drawn, hollow text
// paired.
func TestBuildRecipesEverySize(t *testing.T) {
	dir := t.TempDir()
	jpg := writeImage(t, dir, "soup.jpg", 120, 60, color.RGBA{200, 120, 40, 255})
	png := writeImage(t, dir, "bread.png", 40, 90, color.RGBA{40, 120, 200, 255})
	soup, bread := shortRecipe("Soup"), shortRecipe("Bread")
	soup.Photo, bread.Photo = jpg, png
	for _, p := range Papers {
		for _, size := range RecipeSizes {
			t.Run(p.ID+"/"+size, func(t *testing.T) {
				opts := recipesJSON([]Recipe{soup, bread, shortRecipe("Salad")}, size)
				page := PageSettings{Paper: p.ID, Font: "kaushan-script", ColourIn: true}
				pages := pageStreamsEach(renderPage(t, RecipesKind, page, opts))
				want, err := PageCount(RecipesKind, page, json.RawMessage(opts))
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

// A photo that is missing or not an image fails the build with an error
// naming the recipe.
func TestRecipesBadPhoto(t *testing.T) {
	dir := t.TempDir()
	notImage := filepath.Join(dir, "notes.jpg")
	if err := os.WriteFile(notImage, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"missing": filepath.Join(dir, "gone.jpg"), "not an image": notImage} {
		t.Run(name, func(t *testing.T) {
			r := shortRecipe("Soup")
			r.Photo = path
			opts := recipesJSON([]Recipe{shortRecipe("Bread"), r}, RecipesA6)
			_, err := buildPage(RecipesKind, PageSettings{}, json.RawMessage(opts))
			if err == nil || !strings.Contains(err.Error(), "recipe 2") || !strings.Contains(err.Error(), filepath.Base(path)) {
				t.Errorf("err = %v, want one naming recipe 2 and its photo", err)
			}
		})
	}
}

// Titles print in the lettering, hollow with Colour-in at 14 pt or more,
// even though the page has no shared title.
func TestRecipesHollowTitle(t *testing.T) {
	opts := recipesJSON([]Recipe{shortRecipe("Soup"), shortRecipe("Bread")}, RecipesA6)
	stream := pageStreams(renderPage(t, RecipesKind, PageSettings{Font: "pacifico", ColourIn: true}, opts))
	if got := trPairs(t, "colour-in", stream); got != 2 {
		t.Errorf("%d hollow titles, want 2", got)
	}
	if strings.Contains(pageStreams(renderPage(t, RecipesKind, PageSettings{Font: "pacifico"}, opts)), " Tr") {
		t.Errorf("filled titles use a text rendering mode")
	}
	o := RecipesOptions{Recipes: []Recipe{shortRecipe("Soup")}, Size: RecipesA6}
	pdf := newPDF(A5)
	if err := o.draw(pdf, A5, "", newLettering("pacifico", false), PageLabels{}.resolved()); err != nil {
		t.Fatalf("draw: %v", err)
	}
}
