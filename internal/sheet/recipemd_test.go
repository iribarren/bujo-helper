package sheet

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseRecipesMarkdown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recipes")
	abs := filepath.Join(t.TempDir(), "photos", "soup.png")
	tests := []struct {
		name string
		in   string
		want []Recipe
	}{
		{
			name: "english headings with bullets and numbers",
			in: `# Pancakes
Serves: 4
Time: 20 min

## Ingredients
- 200 g flour
- 2 eggs

## Steps
1. Mix everything.
2. Fry.
`,
			want: []Recipe{{
				Title: "Pancakes", Servings: "4", Time: "20 min",
				Ingredients: []string{"200 g flour", "2 eggs"},
				Steps:       []string{"Mix everything.", "Fry."},
			}},
		},
		{
			name: "spanish headings and keys",
			in: `# Tortilla de patatas
Raciones: 6
Tiempo: 45 min

## Ingredientes
* 5 huevos
* 1 kg de patatas

## Preparación
1) Pelar las patatas.
2) Freír y cuajar.
`,
			want: []Recipe{{
				Title: "Tortilla de patatas", Servings: "6", Time: "45 min",
				Ingredients: []string{"5 huevos", "1 kg de patatas"},
				Steps:       []string{"Pelar las patatas.", "Freír y cuajar."},
			}},
		},
		{
			name: "section and key names in other cases and spellings",
			in: `# Salsa
PORCIONES: 2
tiempo: 5 min

## INGREDIENTES:
+ tomate

## Elaboracion
- Triturar.
`,
			want: []Recipe{{
				Title: "Salsa", Servings: "2", Time: "5 min",
				Ingredients: []string{"tomate"}, Steps: []string{"Triturar."},
			}},
		},
		{
			name: "step heading synonyms",
			in:   "# A\n## Method\n- a\n# B\n## Directions\n- b\n# C\n## Instructions\n- c\n# D\n## Pasos\n- d\n# E\n## Elaboración\n- e\n# F\n## preparacion\n- f\n",
			want: []Recipe{
				{Title: "A", Ingredients: []string{}, Steps: []string{"a"}},
				{Title: "B", Ingredients: []string{}, Steps: []string{"b"}},
				{Title: "C", Ingredients: []string{}, Steps: []string{"c"}},
				{Title: "D", Ingredients: []string{}, Steps: []string{"d"}},
				{Title: "E", Ingredients: []string{}, Steps: []string{"e"}},
				{Title: "F", Ingredients: []string{}, Steps: []string{"f"}},
			},
		},
		{
			name: "several recipes, servings key synonyms and bold keys",
			in: `Intro text that is ignored.
- not an ingredient

# Toast
**Servings:** 1
**Time**: 3 min
## Ingredients
- bread

# Tea
__Serves:__ 2
## Ingredients
- water
- tea
`,
			want: []Recipe{
				{Title: "Toast", Servings: "1", Time: "3 min", Ingredients: []string{"bread"}, Steps: []string{}},
				{Title: "Tea", Servings: "2", Ingredients: []string{"water", "tea"}, Steps: []string{}},
			},
		},
		{
			name: "task list boxes, plain lines and continuation lines",
			in: `# Bread
## Ingredients
- [ ] 500 g flour
- [x] 10 g salt
* [X] water

plain yeast

## Steps
1. Knead the dough
   until smooth.
2. Let it rise
for an hour.

3. Bake.
    Cool on a rack.
Shape the loaf.
`,
			want: []Recipe{{
				Title:       "Bread",
				Ingredients: []string{"500 g flour", "10 g salt", "water", "plain yeast"},
				Steps: []string{
					"Knead the dough until smooth.",
					"Let it rise for an hour.",
					"Bake. Cool on a rack. Shape the loaf.",
				},
			}},
		},
		{
			name: "plain lines with no list markers are one item each",
			in:   "# Rice\n## Ingredients\nrice\nwater\n\nsalt\n",
			want: []Recipe{{Title: "Rice", Ingredients: []string{"rice", "water", "salt"}, Steps: []string{}}},
		},
		{
			name: "unknown sections are ignored and end the current one",
			in: `# Soup
## Ingredients
- leek
## Notes
- this is not an ingredient
Keeps for 3 days.
## Steps
- Simmer.
## Tips
1. not a step
`,
			want: []Recipe{{Title: "Soup", Ingredients: []string{"leek"}, Steps: []string{"Simmer."}}},
		},
		{
			name: "relative, nested, absolute and home photo paths",
			in:   "# A\nPhoto: a.jpg\n- x\n# B\nFoto: img/../photos/b.png\n# C\nphoto: " + abs + "\n# D\nPhoto: ~/d.jpg\n# E\nPhoto: `e e.jpg`\n",
			want: []Recipe{
				{Title: "A", Photo: filepath.Join(dir, "a.jpg"), Ingredients: []string{}, Steps: []string{}},
				{Title: "B", Photo: filepath.Join(dir, "photos", "b.png"), Ingredients: []string{}, Steps: []string{}},
				{Title: "C", Photo: abs, Ingredients: []string{}, Steps: []string{}},
				{Title: "D", Photo: filepath.Join(dir, "~", "d.jpg"), Ingredients: []string{}, Steps: []string{}},
				{Title: "E", Photo: filepath.Join(dir, "e e.jpg"), Ingredients: []string{}, Steps: []string{}},
			},
		},
		{
			name: "photo file names keep their underscores and asterisks",
			in:   "# A\nPhoto: my_best_photo.jpg\n",
			want: []Recipe{{Title: "A", Photo: filepath.Join(dir, "my_best_photo.jpg"), Ingredients: []string{}, Steps: []string{}}},
		},
		{
			name: "a lone image, image syntax after the key and angle brackets",
			in:   "# A\n![A plate](plates/a.jpg)\n# B\nFoto: ![](b.jpg)\n# C\n**Photo:** <c c.jpg>\n# D\nPhoto: _d.jpg\n",
			want: []Recipe{
				{Title: "A", Photo: filepath.Join(dir, "plates", "a.jpg"), Ingredients: []string{}, Steps: []string{}},
				{Title: "B", Photo: filepath.Join(dir, "b.jpg"), Ingredients: []string{}, Steps: []string{}},
				{Title: "C", Photo: filepath.Join(dir, "c c.jpg"), Ingredients: []string{}, Steps: []string{}},
				{Title: "D", Photo: filepath.Join(dir, "_d.jpg"), Ingredients: []string{}, Steps: []string{}},
			},
		},
		{
			name: "a deeper heading inside a list is kept as an item",
			in:   "# Pie\n## Ingredients\n### For the *dough*\n- flour\n### Filling\n- apples\n#### Steps\n- Bake.\n",
			want: []Recipe{{Title: "Pie", Ingredients: []string{"For the dough", "flour", "Filling", "apples"}, Steps: []string{"Bake."}}},
		},
		{
			name: "emphasis, code and links are stripped",
			in: `# **Best** _ever_ ` + "`chili`" + ` #
Serves: *4*
## Ingredients
- __1__ can of [beans](https://example.com/beans)
- 2 * 3 small *chillies*, snake_case_name
- ` + "`salt`" + ` to taste
## Steps
1. Cook ***slowly*** for an hour.
`,
			want: []Recipe{{
				Title: "Best ever chili", Servings: "4",
				Ingredients: []string{"1 can of beans", "2 * 3 small chillies, snake_case_name", "salt to taste"},
				Steps:       []string{"Cook slowly for an hour."},
			}},
		},
		{
			name: "CRLF line endings and a byte order mark",
			in:   "\ufeff# Eggs\r\nServes: 1\r\n\r\n## Ingredients\r\n- egg\r\n## Steps\r\n1. Boil\r\n   for 8 min.\r\n",
			want: []Recipe{{Title: "Eggs", Servings: "1", Ingredients: []string{"egg"}, Steps: []string{"Boil for 8 min."}}},
		},
		{
			name: "space runs collapse and blank recipes are dropped",
			in:   "#   Spaced\t  title  \n## Ingredients\n-    a   lot\n-\n- [ ]\n#\n# \n",
			want: []Recipe{{Title: "Spaced title", Ingredients: []string{"a lot"}, Steps: []string{}}},
		},
		{
			name: "headings inside a fenced code block are not recipes",
			in:   "# Real\n## Steps\n- a\n```\n# not a recipe\n```\n- b\n",
			want: []Recipe{{Title: "Real", Ingredients: []string{}, Steps: []string{"a", "b"}}},
		},
		{
			name: "a fence closes only on its own character, at least as long",
			in:   "# Real\n## Steps\n- a\n```\n~~~\n# not a recipe\n```\n- b\n````md\n```\n# nor this\n````\n- c\n~~~~\n# nor this one\n~~~ x\n~~~\n~~~~~\n- d\n",
			want: []Recipe{{Title: "Real", Ingredients: []string{}, Steps: []string{"a", "b", "c", "d"}}},
		},
		{
			name: "a key whose emphasis closes before the colon",
			in:   "# A\n**Photo**: _d.jpg\n**Serves**: *about* 4\n__Time__: _1 h_\n# B\n**Photo:** b_1.jpg\n**Serves:** 2\n# C\nPhoto: _c.jpg\n*Time*:  20 min\n",
			want: []Recipe{
				{Title: "A", Servings: "about 4", Time: "1 h", Photo: filepath.Join(dir, "_d.jpg"), Ingredients: []string{}, Steps: []string{}},
				{Title: "B", Servings: "2", Photo: filepath.Join(dir, "b_1.jpg"), Ingredients: []string{}, Steps: []string{}},
				{Title: "C", Time: "20 min", Photo: filepath.Join(dir, "_c.jpg"), Ingredients: []string{}, Steps: []string{}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRecipesMarkdown(strings.NewReader(tt.in), dir)
			if err != nil {
				t.Fatalf("ParseRecipesMarkdown: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

func TestParseRecipesMarkdownErrors(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		in      string
		wantNo  bool   // want ErrNoRecipes
		wantErr string // else a substring of the error
	}{
		{name: "empty input", in: "", wantNo: true},
		{name: "text with no level-1 heading", in: "Just some notes.\n## Ingredients\n- flour\n", wantNo: true},
		{name: "only blank recipes", in: "#\n# \n## Steps\n", wantNo: true},
		{name: "a heading in a fence only", in: "```\n# Fenced\n```\n", wantNo: true},
		{
			name:    "title over the limit",
			in:      "# ok\n# " + strings.Repeat("t", MaxRecipeTitleRunes+1) + "\n",
			wantErr: "recipe cards recipe 2 title",
		},
		{
			name:    "too many ingredients",
			in:      "# A\n## Ingredients\n" + strings.Repeat("- x\n", MaxRecipeIngredients+1),
			wantErr: "recipe cards recipe 1",
		},
		{
			name:    "too many recipes",
			in:      strings.Repeat("# A\n", MaxRecipes+1),
			wantErr: "recipes, want at most",
		},
		{
			name:    "relative photo with a relative directory",
			in:      "# A\nPhoto: a.jpg\n",
			wantErr: "absolute path",
		},
		{
			name:    "input over 1 MiB",
			in:      "# A\n## Steps\n- " + strings.Repeat("x", MaxRecipesMarkdownBytes) + "\n",
			wantErr: "MiB",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dir
			if tt.name == "relative photo with a relative directory" {
				d = "recipes"
			}
			got, err := ParseRecipesMarkdown(strings.NewReader(tt.in), d)
			if err == nil {
				t.Fatalf("got %#v, want an error", got)
			}
			if tt.wantNo != errors.Is(err, ErrNoRecipes) {
				t.Errorf("err = %v, errors.Is ErrNoRecipes = %v, want %v", err, !tt.wantNo, tt.wantNo)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// TestParseRecipesMarkdownMatchesTheKind checks a parsed file passes the
// kind's own option validation, so an import never yields options the page
// rejects.
func TestParseRecipesMarkdownMatchesTheKind(t *testing.T) {
	in := "# A\nServes: 2\n## Ingredients\n- " + strings.Repeat("i", MaxRecipeIngredientRunes) + "\n## Steps\n1. " + strings.Repeat("s", MaxRecipeStepRunes) + "\n"
	got, err := ParseRecipesMarkdown(strings.NewReader(in), t.TempDir())
	if err != nil {
		t.Fatalf("ParseRecipesMarkdown: %v", err)
	}
	if err := (RecipesOptions{Recipes: got, Size: RecipesA6}).validate(); err != nil {
		t.Errorf("validate: %v", err)
	}
}
