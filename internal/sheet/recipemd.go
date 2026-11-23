package sheet

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxRecipesMarkdownBytes is the largest recipes Markdown file
// ParseRecipesMarkdown reads: 1 MiB.
const MaxRecipesMarkdownBytes = 1 << 20

// ErrNoRecipes is returned by ParseRecipesMarkdown for a file with no
// recipe in it.
var ErrNoRecipes = errors.New("no recipes found")

// recipeSection is the part of a recipe the lines being read belong to.
type recipeSection int

const (
	sectionNone        recipeSection = iota // before the first section
	sectionIngredients                      // ## Ingredients
	sectionSteps                            // ## Steps and the like
	sectionOther                            // any other section, ignored
)

// recipeSections maps the section headings ParseRecipesMarkdown knows,
// lower case, to their sections. Spanish names are listed with and
// without accents.
var recipeSections = map[string]recipeSection{
	"ingredients":   sectionIngredients,
	"ingredientes":  sectionIngredients,
	"steps":         sectionSteps,
	"method":        sectionSteps,
	"directions":    sectionSteps,
	"instructions":  sectionSteps,
	"instrucciones": sectionSteps,
	"pasos":         sectionSteps,
	"preparación":   sectionSteps,
	"preparacion":   sectionSteps,
	"elaboración":   sectionSteps,
	"elaboracion":   sectionSteps,
}

// recipeKeys maps the lower-case keys of the "Key: value" lines
// ParseRecipesMarkdown knows to the recipe field they set.
var recipeKeys = map[string]string{
	"serves": "servings", "servings": "servings", "raciones": "servings", "porciones": "servings",
	"time": "time", "tiempo": "time",
	"photo": "photo", "foto": "photo",
}

var (
	mdHeading = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t]*$`)
	// mdClosingHashes is the optional closing sequence of an ATX heading.
	mdClosingHashes = regexp.MustCompile(`(?:^|[ \t]+)#+$`)
	mdFence         = regexp.MustCompile("^ {0,3}(?:```|~~~)")
	mdBreak         = regexp.MustCompile(`^ {0,3}(?:(?:-[ \t]*){3,}|(?:\*[ \t]*){3,}|(?:_[ \t]*){3,})$`)
	mdListItem      = regexp.MustCompile(`^(?:[-*+]|\d{1,9}[.)])(?:[ \t]+|$)`)
	mdTaskBox       = regexp.MustCompile(`^\[[ xX]\](?:[ \t]+|$)`)
	// mdKeyLine is a "Key: value" line, the key maybe bold or italic, as
	// in "**Serves:** 4" or "**Serves**: 4".
	mdKeyLine = regexp.MustCompile(`^(\*\*|__|\*|_)?[ \t]*(\pL+)[ \t]*(?:\*\*|__|\*|_)?[ \t]*:[ \t]*(\*\*|__|\*|_)?[ \t]*(.*?)[ \t]*$`)
	mdImage   = regexp.MustCompile(`^!\[[^\]]*\]\(([^)]*)\)$`)
	mdLink    = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	mdStars   = regexp.MustCompile(`\*{1,3}([^\s*](?:[^*]*[^\s*])?)\*{1,3}`)
	// mdUnderscores only strips underscores at word edges, so snake_case
	// stays as it is.
	mdUnderscores = regexp.MustCompile(`(^|[^\pL\pN_])_{1,3}([^\s_](?:[^_]*[^\s_])?)_{1,3}($|[^\pL\pN_])`)
)

// ParseRecipesMarkdown reads recipes from Markdown, at most
// MaxRecipesMarkdownBytes of it, a byte order mark and CRLF line endings
// allowed:
//
//   - a level-1 heading ("# Pancakes") starts a recipe; text before the
//     first one is ignored;
//   - "Serves:", "Servings:", "Raciones:" or "Porciones:" and "Time:" or
//     "Tiempo:" lines set the servings and the time, and a "Photo:" or
//     "Foto:" line (or a lone image, "![](pancakes.jpg)") the photo; keys
//     are case-insensitive and may be bold;
//   - "## Ingredients" or "## Ingredientes" and "## Steps", "## Method",
//     "## Directions", "## Instructions", "## Pasos", "## Preparación" or
//     "## Elaboración" start a list; any other heading ends it and its
//     content is ignored, except that a deeper heading inside a list is
//     kept as an item (an ingredient group, say);
//   - list items are "-", "*" or "+" bullets or "1." or "1)" numbers, a
//     task-list box "[ ]" stripped; a plain line in a list is an item too;
//     an indented line, or a line right after an item, continues it.
//
// Emphasis markers and backticks are stripped and links become their
// text. A relative photo path is joined to dir; "~" is not expanded. The
// recipes are normalised and checked like the page kind's options, so an
// error names a recipe by its 1-based index. No recipe is ErrNoRecipes.
func ParseRecipesMarkdown(r io.Reader, dir string) ([]Recipe, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxRecipesMarkdownBytes+1))
	if err != nil {
		return nil, fmt.Errorf("recipes Markdown: %w", err)
	}
	if len(data) > MaxRecipesMarkdownBytes {
		return nil, fmt.Errorf("recipes Markdown: larger than %d MiB", MaxRecipesMarkdownBytes>>20)
	}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))

	p := recipeParser{dir: dir}
	for _, line := range strings.Split(string(data), "\n") {
		p.line(strings.TrimSuffix(line, "\r"))
	}
	recipes := normalizeRecipes(p.recipes)
	if len(recipes) == 0 {
		return nil, ErrNoRecipes
	}
	if err := (RecipesOptions{Recipes: recipes, Size: RecipesA6}).validate(); err != nil {
		return nil, err
	}
	return recipes, nil
}

// recipeParser holds ParseRecipesMarkdown's state between lines.
type recipeParser struct {
	dir     string
	recipes []Recipe
	section recipeSection
	fenced  bool // inside a fenced code block
	// listed is whether the section's last item was a list item, which
	// indented lines continue; open is whether the previous line was part
	// of it, which plain lines continue too.
	listed, open bool
}

// line reads one line, its line ending removed.
func (p *recipeParser) line(line string) {
	if mdFence.MatchString(line) {
		p.fenced, p.open = !p.fenced, false
		return
	}
	if p.fenced {
		return
	}
	if m := mdHeading.FindStringSubmatch(line); m != nil {
		p.heading(len(m[1]), mdClosingHashes.ReplaceAllString(m[2], ""))
		return
	}
	if len(p.recipes) == 0 {
		return
	}
	text := strings.TrimSpace(line)
	if text == "" || mdBreak.MatchString(line) {
		p.open = false
		return
	}
	indented := strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "  ")
	item := mdListItem.FindString(text)
	if item == "" && !indented && p.meta(text) {
		p.open = false
		return
	}
	if p.section != sectionIngredients && p.section != sectionSteps {
		if item != "" {
			p.meta(text[len(item):])
		}
		return
	}
	switch {
	case item != "":
		p.add(mdTaskBox.ReplaceAllString(text[len(item):], ""))
		p.listed, p.open = true, true
	case p.listed && (p.open || indented):
		list := p.list()
		(*list)[len(*list)-1] += " " + stripInlineMarkdown(text)
		p.open = true
	default:
		p.add(text)
		p.listed, p.open = false, false
	}
}

// heading reads a heading of the given level and text.
func (p *recipeParser) heading(level int, text string) {
	p.open = false
	if level == 1 {
		p.recipes = append(p.recipes, Recipe{Title: stripInlineMarkdown(text)})
		p.section, p.listed = sectionNone, false
		return
	}
	if len(p.recipes) == 0 {
		return
	}
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(stripInlineMarkdown(text)), ":"))
	if s, ok := recipeSections[strings.TrimSpace(name)]; ok {
		p.section, p.listed = s, false
		return
	}
	if level > 2 && (p.section == sectionIngredients || p.section == sectionSteps) {
		p.add(text)
		p.listed = false
		return
	}
	p.section = sectionOther
}

// meta sets a field of the current recipe from a "Key: value" or lone
// image line, and reports whether text was one.
func (p *recipeParser) meta(text string) bool {
	r := &p.recipes[len(p.recipes)-1]
	if m := mdImage.FindStringSubmatch(text); m != nil {
		if r.Photo == "" {
			r.Photo = p.photoPath(m[1])
		}
		return true
	}
	m := mdKeyLine.FindStringSubmatch(text)
	if m == nil {
		return false
	}
	value := m[4]
	if m[1] == "" {
		// A marker after the colon closes nothing: it is the value's.
		value = m[3] + value
	}
	switch recipeKeys[strings.ToLower(m[2])] {
	case "servings":
		r.Servings = stripInlineMarkdown(value)
	case "time":
		r.Time = stripInlineMarkdown(value)
	case "photo":
		if im := mdImage.FindStringSubmatch(value); im != nil {
			value = im[1]
		}
		r.Photo = p.photoPath(value)
	default:
		return false
	}
	return true
}

// photoPath returns a photo path as written, maybe in backticks or angle
// brackets, joined to the Markdown file's folder when relative.
func (p *recipeParser) photoPath(value string) string {
	value = strings.TrimSpace(value)
	for _, q := range [][2]string{{"`", "`"}, {"<", ">"}} {
		if len(value) >= 2 && strings.HasPrefix(value, q[0]) && strings.HasSuffix(value, q[1]) {
			value = strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(p.dir, filepath.FromSlash(value))
}

// list returns the current recipe's list for the current section, which
// must be the ingredients or the steps.
func (p *recipeParser) list() *[]string {
	r := &p.recipes[len(p.recipes)-1]
	if p.section == sectionIngredients {
		return &r.Ingredients
	}
	return &r.Steps
}

// add appends an item to the current list.
func (p *recipeParser) add(text string) {
	list := p.list()
	*list = append(*list, stripInlineMarkdown(text))
}

// stripInlineMarkdown returns s with links and images replaced by their
// text, backticks removed and emphasis markers stripped.
func stripInlineMarkdown(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "`", "")
	// A replacement consumes the characters around underscores, so nested
	// or adjacent emphasis needs another pass; each pass removes markers,
	// so this ends.
	for {
		t := mdUnderscores.ReplaceAllString(mdStars.ReplaceAllString(s, "$1"), "$1$2$3")
		if t == s {
			return s
		}
		s = t
	}
}
