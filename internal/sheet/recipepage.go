package sheet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
)

// RecipesKind is the ID of the recipe cards page kind: recipes laid out as
// landscape cut-out cards, or one a page, their titles in the lettering
// font.
const RecipesKind = "recipes"

// Recipe card sizes.
const (
	// RecipesA6 is an A6 card, 148 × 105 mm landscape.
	RecipesA6 = "a6"
	// Recipes4x6 is a 4 × 6 in index card, landscape.
	Recipes4x6 = "4x6"
	// RecipesPage is one recipe a page, in the paper's content area.
	RecipesPage = "page"
)

// RecipeSizes are the card sizes to choose from, the default first.
var RecipeSizes = []string{RecipesA6, Recipes4x6, RecipesPage}

// Recipe list limits.
const (
	MaxRecipes               = 50   // recipes in a document
	MaxRecipeTitleRunes      = 100  // characters in a title
	MaxRecipeMetaRunes       = 40   // characters in the servings and in the time
	MaxRecipeIngredients     = 60   // ingredients in a recipe
	MaxRecipeIngredientRunes = 200  // characters in an ingredient
	MaxRecipeSteps           = 40   // steps in a recipe
	MaxRecipeStepRunes       = 600  // characters in a step
	MaxRecipePhotoBytes      = 4096 // bytes in a photo path
)

// Recipe card drawing settings: lengths in millimetres, font sizes in
// points.
const (
	recipeA6W, recipeA6H   = 148.0, 105.0
	recipe4x6W, recipe4x6H = 6 * mmPerInch, 4 * mmPerInch
	recipePad              = 4.0 // between the cut line and the content
	recipeTitleMinSize     = 9.0 // smallest title, cut with "…" below it
	recipeTitleGap         = 1.0 // below the title band
	recipeMetaGap          = 1.5 // below the meta line
	recipeColumnGap        = 4.0 // between the two columns
	recipeIngredientsShare = 0.4 // of the width left for the columns
	recipeHeadingGap       = 1.0 // below a column heading
	recipeMarkerGap        = 1.2 // between a bullet or number and its text
	recipeItemGap          = 0.8 // between two ingredients or steps
	recipeMinBodySize      = 6.0 // smallest body text, flowing on below it
	recipeSizeStep         = 0.5
	recipePhotoRatio       = 4.0 / 3 // the photo box's width / height at most
	recipePhotoShare       = 0.45    // of the column height the photo takes at most
	recipePhotoGap         = 2.0     // below the photo
	recipeBlankPitch       = 1.6     // blank card writing lines, in body line heights
	recipeLabelRuleGap     = 1.0     // between a blank card label and its rule
)

// recipeStyle are the text sizes of a card size: the largest title and
// body text, the meta line and the column headings.
type recipeStyle struct {
	titleMax, bodyMax, metaSize, headingSize float64
}

var recipeStyles = map[string]recipeStyle{
	RecipesA6:   {titleMax: 16, bodyMax: 9, metaSize: 8, headingSize: 8.5},
	Recipes4x6:  {titleMax: 16, bodyMax: 9, metaSize: 8, headingSize: 8.5},
	RecipesPage: {titleMax: 20, bodyMax: 11, metaSize: 9.5, headingSize: 10},
}

// Recipe is one recipe.
type Recipe struct {
	Title       string   `json:"title"`
	Servings    string   `json:"servings"`
	Time        string   `json:"time"`
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	// Photo is the absolute path of a JPEG or PNG image of the user's, or
	// "" for none. It is read when the page is drawn.
	Photo string `json:"photo"`
}

// RecipesOptions are the options of a recipe cards page.
type RecipesOptions struct {
	// Recipes are the recipes, normalised by normalizeRecipes: at most
	// MaxRecipes, within the other Max limits. None prints a page of
	// blank cards.
	Recipes []Recipe `json:"recipes"`
	Size    string   `json:"size"` // one of RecipeSizes
}

var defaultRecipes = RecipesOptions{Recipes: []Recipe{}, Size: RecipesA6}

// decodeRecipes decodes recipe card options, like decodeOptions, with the
// recipes normalised.
func decodeRecipes(raw json.RawMessage) (pageDoc, error) {
	doc, err := decodeOptions(defaultRecipes)(raw)
	if err != nil {
		return nil, err
	}
	o := doc.(RecipesOptions)
	o.Recipes = normalizeRecipes(o.Recipes)
	return o, nil
}

// normalizeRecipes returns recipes with their title, servings, time,
// ingredients and steps normalised by normalizeHabit, blank ingredients
// and steps dropped, and the photo path trimmed (its inner spaces are
// part of the path). A recipe left with no title, ingredients, steps or
// photo is dropped. The result and its lists are never nil.
func normalizeRecipes(recipes []Recipe) []Recipe {
	out := []Recipe{}
	for _, r := range recipes {
		r.Title, r.Servings, r.Time = normalizeHabit(r.Title), normalizeHabit(r.Servings), normalizeHabit(r.Time)
		r.Ingredients, r.Steps = normalizeHabits(r.Ingredients), normalizeHabits(r.Steps)
		r.Photo = strings.TrimSpace(r.Photo)
		if r.Title != "" || len(r.Ingredients) > 0 || len(r.Steps) > 0 || r.Photo != "" {
			out = append(out, r)
		}
	}
	return out
}

// validate checks the size and the recipes as they print: normalised,
// empty ones dropped. Errors name a recipe by its 1-based index, never by
// its text.
func (o RecipesOptions) validate() error {
	if !oneOf(o.Size, RecipeSizes) {
		return fmt.Errorf("recipe cards size %q: want one of %v", o.Size, RecipeSizes)
	}
	recipes := normalizeRecipes(o.Recipes)
	if len(recipes) > MaxRecipes {
		return fmt.Errorf("recipe cards: %d recipes, want at most %d", len(recipes), MaxRecipes)
	}
	for i, r := range recipes {
		if err := r.validate(i + 1); err != nil {
			return err
		}
	}
	return nil
}

// validate checks recipe n (1-based), normalised, against the limits.
func (r Recipe) validate(n int) error {
	name := fmt.Sprintf("recipe cards recipe %d", n)
	runes := func(part, s string, limit int) error {
		if c := utf8.RuneCountInString(s); c > limit {
			return fmt.Errorf("%s %s: %d characters, want at most %d", name, part, c, limit)
		}
		return nil
	}
	list := func(noun string, items []string, maxItems, maxRunes int) error {
		if len(items) > maxItems {
			return fmt.Errorf("%s: %d %ss, want at most %d", name, len(items), noun, maxItems)
		}
		for i, s := range items {
			if err := runes(fmt.Sprintf("%s %d", noun, i+1), s, maxRunes); err != nil {
				return err
			}
		}
		return nil
	}
	for _, err := range []error{
		runes("title", r.Title, MaxRecipeTitleRunes),
		runes("servings", r.Servings, MaxRecipeMetaRunes),
		runes("time", r.Time, MaxRecipeMetaRunes),
		list("ingredient", r.Ingredients, MaxRecipeIngredients, MaxRecipeIngredientRunes),
		list("step", r.Steps, MaxRecipeSteps, MaxRecipeStepRunes),
	} {
		if err != nil {
			return err
		}
	}
	if b := len(r.Photo); b > MaxRecipePhotoBytes {
		return fmt.Errorf("%s photo: %d bytes, want at most %d", name, b, MaxRecipePhotoBytes)
	}
	if r.Photo != "" && !filepath.IsAbs(r.Photo) {
		return fmt.Errorf("%s photo: want an absolute path", name)
	}
	return nil
}

// pages is the page count on A4; PageCount asks pagesOn.
func (o RecipesOptions) pages() int {
	n, err := o.pagesOn(A4)
	if err != nil {
		return 1
	}
	return max(1, n)
}

// pagesOn is how many pages the cards take on page, continuation cards
// included. The titles never change the count (their band is fixed), so
// only the Go font is measured.
func (o RecipesOptions) pagesOn(page Page) (int, error) {
	pdf := newPDF(page)
	if err := addRegularTextFont(pdf); err != nil {
		return 0, err
	}
	m := pdfMeasure(pdf)
	return len(o.sheets(page, defaultLettering, m, m, PageLabels{}.resolved())), nil
}

// recipeCardSize is the landscape width and height of a card of size, in
// mm; size is RecipesA6 or Recipes4x6.
func recipeCardSize(size string) (w, h float64) {
	if size == Recipes4x6 {
		return recipe4x6W, recipe4x6H
	}
	return recipeA6W, recipeA6H
}

// recipeSlots are the cards of a page, in order: for RecipesPage the
// content area within MinMargin margins; otherwise as many cards as fit
// within the margins, Gap apart and centred, or, if not even one fits, a
// single card scaled down to the content area (its aspect kept), centred.
func recipeSlots(page Page, size string) []Box {
	area := Box{X: MinMargin, Y: MinMargin, W: page.Width - 2*MinMargin, H: page.Height - 2*MinMargin}
	if size == RecipesPage {
		return []Box{area}
	}
	w, h := recipeCardSize(size)
	g, err := newFixedGrid(page, w, h, "recipe card")
	if err != nil {
		s := min(area.W/w, area.H/h)
		w, h = w*s, h*s
		return []Box{{X: (page.Width - w) / 2, Y: (page.Height - h) / 2, W: w, H: h}}
	}
	var out []Box
	for r := range g.Rows {
		for c := range g.Columns {
			out = append(out, Box{X: g.MarginX + float64(c)*(w+Gap), Y: g.MarginY + float64(r)*(h+Gap), W: w, H: h})
		}
	}
	return out
}

// recipePart is what a text on a recipe card is.
type recipePart int

const (
	recipeMeta       recipePart = iota // the meta line, or a blank card's labels
	recipeHeading                      // a column heading
	recipeBullet                       // an ingredient's bullet
	recipeIngredient                   // a line of an ingredient
	recipeNumber                       // a step's number
	recipeStep                         // a line of a step
)

// recipeText is a line of text in the Go font at Size points.
type recipeText struct {
	Text string
	Box  Box
	Size float64
	Part recipePart
}

// recipeCard is one card: its box (a dashed cut line when Cut), the inner
// box everything stays in, the title in the lettering at TitleSize (empty
// text for none), the texts in the Go font, the writing lines and the
// photo box (zero for none). Recipe is the recipe's 1-based index, 0 for
// a blank card; Continued marks its continuation cards; BodySize is its
// ingredients' and steps' size.
type recipeCard struct {
	Card, Inner Box
	Cut         bool
	Recipe      int
	Continued   bool
	Title       quoteLine
	TitleSize   float64
	BodySize    float64
	Texts       []recipeText
	Rules       []moonRule
	Photo       Box
}

// moved is c moved by dx, dy.
func (c recipeCard) moved(dx, dy float64) recipeCard {
	mv := func(b Box) Box { return Box{X: b.X + dx, Y: b.Y + dy, W: b.W, H: b.H} }
	c.Card, c.Inner, c.Title.Box = mv(c.Card), mv(c.Inner), mv(c.Title.Box)
	if c.Photo != (Box{}) {
		c.Photo = mv(c.Photo)
	}
	texts := make([]recipeText, len(c.Texts))
	for i, t := range c.Texts {
		t.Box = mv(t.Box)
		texts[i] = t
	}
	rules := make([]moonRule, len(c.Rules))
	for i, r := range c.Rules {
		rules[i] = moonRule{X1: r.X1 + dx, X2: r.X2 + dx, Y: r.Y + dy}
	}
	c.Texts, c.Rules = texts, rules
	return c
}

// recipesSheet is one page of cards.
type recipesSheet struct {
	Cards []recipeCard
}

// sheets lays out the pages on page: every recipe's cards in order, then
// blank cards filling the last page (a page of them with no recipes).
// titleMeasure measures text in the lettering l, textMeasure in the Go
// font.
func (o RecipesOptions) sheets(page Page, l lettering, titleMeasure, textMeasure measureFunc, labels PageLabels) []recipesSheet {
	slots := recipeSlots(page, o.Size)
	card := Box{W: slots[0].W, H: slots[0].H}
	var cards []recipeCard
	for i, r := range o.Recipes {
		cards = append(cards, o.layoutRecipe(i+1, r, card, l, titleMeasure, textMeasure, labels)...)
	}
	if len(cards) == 0 || len(cards)%len(slots) != 0 {
		blank := o.blankCard(card, textMeasure, labels)
		for len(cards) == 0 || len(cards)%len(slots) != 0 {
			cards = append(cards, blank)
		}
	}
	var out []recipesSheet
	for i, c := range cards {
		if i%len(slots) == 0 {
			out = append(out, recipesSheet{})
		}
		s := slots[i%len(slots)]
		out[len(out)-1].Cards = append(out[len(out)-1].Cards, c.moved(s.X, s.Y))
	}
	return out
}

// recipeFrame is the geometry of a card's parts: the inner box, the title
// band, the meta line (zero with none) and the two columns below, each
// starting with its heading.
type recipeFrame struct {
	inner, title, meta, left, right Box
}

// frame is the geometry of card, with or without a meta line.
func (o RecipesOptions) frame(card Box, meta bool) recipeFrame {
	st := recipeStyles[o.Size]
	in := inset(card, recipePad)
	f := recipeFrame{inner: in, title: Box{X: in.X, Y: in.Y, W: in.W, H: lineHeight(st.titleMax)}}
	y := f.title.Y + f.title.H + recipeTitleGap
	if meta {
		f.meta = Box{X: in.X, Y: y, W: in.W, H: lineHeight(st.metaSize)}
		y += f.meta.H + recipeMetaGap
	}
	cols := in.W - recipeColumnGap
	h := in.Y + in.H - y
	f.left = Box{X: in.X, Y: y, W: cols * recipeIngredientsShare, H: h}
	f.right = Box{X: f.left.X + f.left.W + recipeColumnGap, Y: y, W: cols - f.left.W, H: h}
	return f
}

// body is the part of column col below its heading.
func (o RecipesOptions) body(col Box) Box {
	top := lineHeight(recipeStyles[o.Size].headingSize) + recipeHeadingGap
	return Box{X: col.X, Y: col.Y + top, W: col.W, H: col.H - top}
}

// heading is the heading label at the top of column col, cut with "…" to
// its width.
func (o RecipesOptions) heading(label string, col Box, measure measureFunc) recipeText {
	pt := recipeStyles[o.Size].headingSize
	text := fitText(label, col.W, pt, measure)
	return recipeText{Text: text, Box: Box{X: col.X, Y: col.Y, W: measure(text, false, pt), H: lineHeight(pt)}, Size: pt, Part: recipeHeading}
}

// recipeMetaText is "Serves X · Time Y", leaving out an empty part; "" if
// both are.
func recipeMetaText(r Recipe, labels PageLabels) string {
	var parts []string
	if r.Servings != "" {
		parts = append(parts, labels.RecipeServes+" "+r.Servings)
	}
	if r.Time != "" {
		parts = append(parts, labels.RecipeTime+" "+r.Time)
	}
	return strings.Join(parts, " · ")
}

// recipeRow is one line of a column: the first line of an item has its
// marker (bullet or number) and, after the first item, a gap above it.
type recipeRow struct {
	marker, text string
	gap          bool
}

// recipeRows wraps items into the lines of a column width wide at pt
// points: bulleted, or numbered from 1. The text is indented by the
// marker's width (the width of "00." when numbered).
func recipeRows(items []string, numbered bool, width, pt float64, measure measureFunc) ([]recipeRow, float64) {
	m := func(s string) float64 { return measure(s, false, pt) }
	marker := func(int) string { return "•" }
	indent := m("•") + recipeMarkerGap
	if numbered {
		marker = func(i int) string { return fmt.Sprintf("%d.", i+1) }
		indent = m("00.") + recipeMarkerGap
	}
	var rows []recipeRow
	for i, item := range items {
		for j, s := range fitLines(item, width-indent, math.MaxInt, m) {
			row := recipeRow{text: s}
			if j == 0 {
				row.marker, row.gap = marker(i), i > 0
			}
			rows = append(rows, row)
		}
	}
	return rows, indent
}

// flowRows places rows top-down in the column bodies of successive cards,
// body(0) first, starting a card's column whenever the next line does not
// fit, and returns each card's texts (at least one card, perhaps empty).
func flowRows(rows []recipeRow, indent, pt float64, body func(card int) Box, markerPart, textPart recipePart, measure measureFunc) [][]recipeText {
	lh := lineHeight(pt)
	out := [][]recipeText{nil}
	b := body(0)
	y := b.Y
	for _, row := range rows {
		gap := 0.0
		if row.gap && y > b.Y {
			gap = recipeItemGap
		}
		// A line that does not fit starts the next card, unless it is
		// already first in its column: then nothing would ever fit, except
		// on the first card, which may be shorter than the rest.
		if y+gap+lh > b.Y+b.H+fitTolerance && (y > b.Y || len(out) == 1) {
			b = body(len(out))
			out = append(out, nil)
			y, gap = b.Y, 0
		}
		y += gap
		texts := &out[len(out)-1]
		if row.marker != "" {
			*texts = append(*texts, recipeText{Text: row.marker, Box: Box{X: b.X, Y: y, W: measure(row.marker, false, pt), H: lh}, Size: pt, Part: markerPart})
		}
		*texts = append(*texts, recipeText{Text: row.text, Box: Box{X: b.X + indent, Y: y, W: measure(row.text, false, pt), H: lh}, Size: pt, Part: textPart})
		y += lh
	}
	return out
}

// fitRecipeTitle fits title, followed by suffix if any, on one line of
// band in the lettering l, centred: from maxPt down by recipeSizeStep to
// recipeTitleMinSize, then cutting the title (never the suffix) with "…".
func fitRecipeTitle(title, suffix string, band Box, maxPt float64, l lettering, measure measureFunc) (quoteLine, float64) {
	text := strings.TrimSpace(title + " " + suffix)
	if text == "" {
		return quoteLine{}, 0
	}
	width := l.fitWidth(band.W)
	pt := maxPt
	for pt > recipeTitleMinSize && measure(text, false, pt) > width+fitTolerance {
		pt = max(recipeTitleMinSize, pt-recipeSizeStep)
	}
	if measure(text, false, pt) > width+fitTolerance {
		room := width
		if suffix != "" {
			room -= measure(" "+suffix, false, pt)
		}
		text = strings.TrimSpace(fitText(title, room, pt, measure) + " " + suffix)
	}
	w, lh := measure(text, false, pt), lineHeight(pt)
	return quoteLine{Text: text, Box: Box{X: band.X + (band.W-w)/2, Y: band.Y + (band.H-lh)/2, W: w, H: lh}}, pt
}

// layoutRecipe lays out recipe n, r, on cards the size of card at its
// origin. The first card has the title, the meta line, the photo
// (centre-cropped into the top of the ingredients column) and the two
// columns: bulleted ingredients and numbered steps. Their text shrinks
// from the size's largest by recipeSizeStep until both fit the first
// card; at recipeMinBodySize what is left flows onto continuation cards,
// titled "Title (continued)", the steps numbered on.
func (o RecipesOptions) layoutRecipe(n int, r Recipe, card Box, l lettering, titleMeasure, textMeasure measureFunc, labels PageLabels) []recipeCard {
	st := recipeStyles[o.Size]
	meta := recipeMetaText(r, labels)
	first, rest := o.frame(card, meta != ""), o.frame(card, false)
	var photo Box
	firstLeft := first.left
	if r.Photo != "" {
		h := min(first.left.W/recipePhotoRatio, first.left.H*recipePhotoShare)
		photo = Box{X: first.left.X, Y: first.left.Y, W: first.left.W, H: h}
		firstLeft.Y += h + recipePhotoGap
		firstLeft.H -= h + recipePhotoGap
	}
	columns := func(firstCol, restCol Box) func(int) Box {
		return func(k int) Box {
			if k == 0 {
				return o.body(firstCol)
			}
			return o.body(restCol)
		}
	}
	leftBody, rightBody := columns(firstLeft, rest.left), columns(first.right, rest.right)

	var ingredients, steps [][]recipeText
	pt := st.bodyMax
	for {
		rows, indent := recipeRows(r.Ingredients, false, first.left.W, pt, textMeasure)
		ingredients = flowRows(rows, indent, pt, leftBody, recipeBullet, recipeIngredient, textMeasure)
		rows, indent = recipeRows(r.Steps, true, first.right.W, pt, textMeasure)
		steps = flowRows(rows, indent, pt, rightBody, recipeNumber, recipeStep, textMeasure)
		if len(ingredients) == 1 && len(steps) == 1 || pt <= recipeMinBodySize {
			break
		}
		pt = max(recipeMinBodySize, pt-recipeSizeStep)
	}

	suffix := "(" + labels.RecipeContinued + ")"
	out := make([]recipeCard, max(len(ingredients), len(steps)))
	for k := range out {
		f, left, suffixK := rest, rest.left, suffix
		if k == 0 {
			f, left, suffixK = first, firstLeft, ""
		}
		c := recipeCard{Card: card, Inner: f.inner, Cut: o.Size != RecipesPage, Recipe: n, Continued: k > 0, BodySize: pt}
		c.Title, c.TitleSize = fitRecipeTitle(r.Title, suffixK, f.title, st.titleMax, l, titleMeasure)
		if k == 0 {
			if meta != "" {
				text := fitText(meta, f.meta.W, st.metaSize, textMeasure)
				w := textMeasure(text, false, st.metaSize)
				c.Texts = append(c.Texts, recipeText{Text: text, Box: Box{X: f.meta.X + (f.meta.W-w)/2, Y: f.meta.Y, W: w, H: f.meta.H}, Size: st.metaSize, Part: recipeMeta})
			}
			c.Photo = photo
		}
		for _, col := range []struct {
			texts [][]recipeText
			box   Box
			label string
		}{{ingredients, left, labels.RecipeIngredients}, {steps, f.right, labels.RecipeSteps}} {
			if k < len(col.texts) && len(col.texts[k]) > 0 {
				c.Texts = append(c.Texts, o.heading(col.label, col.box, textMeasure))
				c.Texts = append(c.Texts, col.texts[k]...)
			}
		}
		out[k] = c
	}
	return out
}

// blankCard is a card to fill in by hand at the origin: a writing line
// for the title, the Serves and Time labels each followed by a line, and
// the two column headings over writing lines.
func (o RecipesOptions) blankCard(card Box, measure measureFunc, labels PageLabels) recipeCard {
	st := recipeStyles[o.Size]
	f := o.frame(card, true)
	c := recipeCard{Card: card, Inner: f.inner, Cut: o.Size != RecipesPage}
	c.Rules = append(c.Rules, moonRule{X1: f.title.X, X2: f.title.X + f.title.W, Y: f.title.Y + f.title.H})
	half := (f.meta.W - recipeColumnGap) / 2
	for i, label := range []string{labels.RecipeServes, labels.RecipeTime} {
		x := f.meta.X + float64(i)*(half+recipeColumnGap)
		text := fitText(label, half/2, st.metaSize, measure)
		w := measure(text, false, st.metaSize)
		c.Texts = append(c.Texts, recipeText{Text: text, Box: Box{X: x, Y: f.meta.Y, W: w, H: f.meta.H}, Size: st.metaSize, Part: recipeMeta})
		c.Rules = append(c.Rules, moonRule{X1: x + w + recipeLabelRuleGap, X2: x + half, Y: f.meta.Y + f.meta.H})
	}
	pitch := lineHeight(st.bodyMax) * recipeBlankPitch
	for _, col := range []struct {
		box   Box
		label string
	}{{f.left, labels.RecipeIngredients}, {f.right, labels.RecipeSteps}} {
		c.Texts = append(c.Texts, o.heading(col.label, col.box, measure))
		body := o.body(col.box)
		for y := body.Y + pitch; y <= body.Y+body.H+fitTolerance; y += pitch {
			c.Rules = append(c.Rules, moonRule{X1: body.X, X2: body.X + body.W, Y: y})
		}
	}
	return c
}

// draw draws the pages of cards. The page has no shared title (the kind
// does not use it), so the lettering's font is added here for the recipe
// titles.
func (o RecipesOptions) draw(pdf *pdfDoc, page Page, _ string, l lettering, labels PageLabels) error {
	if err := addRegularTextFont(pdf); err != nil {
		return err
	}
	if len(o.Recipes) > 0 {
		if err := l.add(pdf); err != nil {
			return err
		}
	}
	for _, s := range o.sheets(page, l, l.measure(pdf), pdfMeasure(pdf), labels) {
		pdf.AddPage()
		if err := o.drawSheet(pdf, s, l); err != nil {
			return err
		}
	}
	return pdf.Error()
}

// drawSheet draws s on the current page: the dashed cut lines, then each
// card's photo, writing lines, title (in the lettering l, hollow at 14 pt
// or more with Colour-in) and texts. User text is drawn as text only. A
// photo that cannot be read is an error naming its recipe.
func (o RecipesOptions) drawSheet(pdf *pdfDoc, s recipesSheet, l lettering) error {
	var cuts []Box
	for _, c := range s.Cards {
		if c.Cut {
			cuts = append(cuts, c.Card)
		}
	}
	drawCutLines(pdf, cuts)
	for _, c := range s.Cards {
		if c.Photo != (Box{}) {
			if err := o.drawPhoto(pdf, c); err != nil {
				return err
			}
		}
		pdf.SetLineWidth(ruleWidth)
		pdf.SetDrawColor(writingGrey, writingGrey, writingGrey)
		for _, r := range c.Rules {
			pdf.Line(r.X1, r.Y, r.X2, r.Y)
		}
		if c.Title.Text != "" {
			pdf.SetTextColor(titleGrey, titleGrey, titleGrey)
			l.setFont(pdf, c.TitleSize)
			l.text(pdf, c.Title.Box.X, baseline(c.Title.Box.Y, c.TitleSize), c.TitleSize, c.Title.Text)
		}
		for _, t := range c.Texts {
			grey := titleGrey
			if t.Part == recipeMeta || t.Part == recipeBullet || t.Part == recipeNumber {
				grey = textGrey
			}
			pdf.SetTextColor(grey, grey, grey)
			pdf.SetFont(textFont, "", t.Size)
			pdf.Text(t.Box.X, baseline(t.Box.Y, t.Size), t.Text)
		}
	}
	return nil
}

// drawCutLines draws a dashed cut line around each of cards, in the quote
// cards' cut line style, and restores solid lines; none draws nothing.
func drawCutLines(pdf *pdfDoc, cards []Box) {
	if len(cards) == 0 {
		return
	}
	pdf.SetLineWidth(quotesCutWidth)
	pdf.SetDrawColor(quotesCutR, quotesCutG, quotesCutB)
	pdf.SetDashPattern([]float64{quotesDash, quotesDashSpace}, 0)
	for _, b := range cards {
		pdf.Rect(b.X, b.Y, b.W, b.H, "D")
	}
	pdf.SetDashPattern([]float64{}, 0)
}

// drawPhoto draws card c's photo, read now and centre-cropped to fill its
// box.
func (o RecipesOptions) drawPhoto(pdf *pdfDoc, c recipeCard) error {
	path := o.Recipes[c.Recipe-1].Photo
	img, err := printCroppedImage(path, c.Photo, false)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return fmt.Errorf("recipe cards recipe %d photo %s: %w", c.Recipe, filepath.Base(path), pe.Err)
		}
		return fmt.Errorf("recipe cards recipe %d photo %s: not a readable JPEG or PNG image", c.Recipe, filepath.Base(path))
	}
	name := fmt.Sprintf("recipe-photo-%d", c.Recipe)
	opts := fpdf.ImageOptions{ImageType: "JPG"}
	pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(img.jpeg))
	pdf.ImageOptions(name, c.Photo.X, c.Photo.Y, c.Photo.W, c.Photo.H, false, opts, 0, "")
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("recipe cards recipe %d photo %s: %w", c.Recipe, filepath.Base(path), err)
	}
	return nil
}
