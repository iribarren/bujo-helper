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
	"golang.org/x/image/font/gofont/goitalic"
)

// PlantsKind is the ID of the plant care cards page kind: a card per
// plant with its plate image, its name in the lettering font and ruled
// fields for light, water, soil and notes. The cards come in the recipe
// card sizes (RecipeSizes) and sit in the same slots.
const PlantsKind = "plants"

// Plant list limits.
const (
	MaxPlants          = 50   // plants in a document
	MaxPlantNameRunes  = 100  // characters in a name
	MaxPlantFieldRunes = 200  // characters in the light, water and soil
	MaxPlantNotesRunes = 600  // characters in the notes
	MaxPlantImageBytes = 4096 // bytes in an image path
)

// Plant care card drawing settings: lengths in millimetres.
const (
	plantPlateShare = 0.4 // of the inner height the plate takes at most
	plantGap        = 1.5 // below the plate and above the fields
	// plantTwoLines is the fewest writing lines that give light, water and
	// soil two each, the notes at least two.
	plantTwoLines = 3*2 + 2
)

// plantStyle are the text sizes of a card size, in points: the largest
// name, the scientific name, the field labels and the typed field text.
type plantStyle struct {
	nameMax, sciSize, labelSize, bodySize float64
}

var plantStyles = map[string]plantStyle{
	RecipesA6:   {nameMax: 16, sciSize: 8, labelSize: 8, bodySize: 8.5},
	Recipes4x6:  {nameMax: 16, sciSize: 8, labelSize: 8, bodySize: 8.5},
	RecipesPage: {nameMax: 22, sciSize: 10, labelSize: 10, bodySize: 11},
}

// Plant is one plant's care card.
type Plant struct {
	// Name is printed as the card's title; "" takes it from the image's
	// file name (see plantNames).
	Name  string `json:"name"`
	Light string `json:"light"`
	Water string `json:"water"`
	Soil  string `json:"soil"`
	Notes string `json:"notes"`
	// Image is the absolute path of a JPEG or PNG plate, or "" for none.
	// It is read when the page is drawn.
	Image string `json:"image"`
}

// PlantsOptions are the options of a plant care cards page.
type PlantsOptions struct {
	// Plants are the plants, normalised by normalizePlants: at most
	// MaxPlants, within the other Max limits. None prints a page of blank
	// cards.
	Plants []Plant `json:"plants"`
	Size   string  `json:"size"` // one of RecipeSizes
}

var defaultPlants = PlantsOptions{Plants: []Plant{}, Size: RecipesA6}

// decodePlants decodes plant care card options, like decodeOptions, with
// the plants normalised.
func decodePlants(raw json.RawMessage) (pageDoc, error) {
	doc, err := decodeOptions(defaultPlants)(raw)
	if err != nil {
		return nil, err
	}
	o := doc.(PlantsOptions)
	o.Plants = normalizePlants(o.Plants)
	return o, nil
}

// normalizePlants returns plants with their text normalised by
// normalizeHabit and the image path trimmed (its inner spaces are part of
// the path). A plant left with nothing is dropped. The result is never
// nil.
func normalizePlants(plants []Plant) []Plant {
	out := []Plant{}
	for _, p := range plants {
		p.Name, p.Light, p.Water = normalizeHabit(p.Name), normalizeHabit(p.Light), normalizeHabit(p.Water)
		p.Soil, p.Notes = normalizeHabit(p.Soil), normalizeHabit(p.Notes)
		p.Image = strings.TrimSpace(p.Image)
		if p != (Plant{}) {
			out = append(out, p)
		}
	}
	return out
}

// validate checks the size and the plants as they print: normalised,
// empty ones dropped. Errors name a plant by its 1-based index, never by
// its text.
func (o PlantsOptions) validate() error {
	if !oneOf(o.Size, RecipeSizes) {
		return fmt.Errorf("plant care cards size %q: want one of %v", o.Size, RecipeSizes)
	}
	plants := normalizePlants(o.Plants)
	if len(plants) > MaxPlants {
		return fmt.Errorf("plant care cards: %d plants, want at most %d", len(plants), MaxPlants)
	}
	for i, p := range plants {
		if err := p.validate(i + 1); err != nil {
			return err
		}
	}
	return nil
}

// validate checks plant n (1-based), normalised, against the limits.
func (p Plant) validate(n int) error {
	name := fmt.Sprintf("plant care cards plant %d", n)
	for _, f := range []struct {
		part, text string
		limit      int
	}{
		{"name", p.Name, MaxPlantNameRunes},
		{"light", p.Light, MaxPlantFieldRunes},
		{"water", p.Water, MaxPlantFieldRunes},
		{"soil", p.Soil, MaxPlantFieldRunes},
		{"notes", p.Notes, MaxPlantNotesRunes},
	} {
		if c := utf8.RuneCountInString(f.text); c > f.limit {
			return fmt.Errorf("%s %s: %d characters, want at most %d", name, f.part, c, f.limit)
		}
	}
	if p.Image == "" {
		return nil
	}
	if b := len(p.Image); b > MaxPlantImageBytes {
		return fmt.Errorf("%s image: %d bytes, want at most %d", name, b, MaxPlantImageBytes)
	}
	if !filepath.IsAbs(p.Image) {
		return fmt.Errorf("%s image: want an absolute path", name)
	}
	switch strings.ToLower(filepath.Ext(p.Image)) {
	case ".jpg", ".jpeg", ".png":
		return nil
	}
	return fmt.Errorf("%s image: want a JPEG or PNG file", name)
}

// pages is the page count on A4; PageCount asks pagesOn.
func (o PlantsOptions) pages() int {
	n, _ := o.pagesOn(A4)
	return n
}

// pagesOn is how many pages the cards take on page: one card a plant,
// at least one page.
func (o PlantsOptions) pagesOn(page Page) (int, error) {
	per := len(recipeSlots(page, o.Size))
	return max(1, (len(o.Plants)+per-1)/per), nil
}

// plantNames are the name printed on p's card and the scientific name
// printed under it: the typed name alone, or with none typed, the common
// and scientific names read from the image's file name, "Common name -
// Scientific name.jpg", by ParseName.
func plantNames(p Plant) (name, scientific string) {
	if p.Name != "" || p.Image == "" {
		return p.Name, ""
	}
	return ParseName(p.Image)
}

// plantMeasure measures the texts of a plant card: title in the
// lettering, text in the Go font and italic in its italic.
type plantMeasure struct {
	title, text, italic measureFunc
}

// plantPart is what a text on a plant card is.
type plantPart int

const (
	plantScientific plantPart = iota // the scientific name, in italics
	plantLabel                       // a field label
	plantField                       // a line of typed field text
)

// plantText is a line of text in the Go font at Size points, in italics
// if Italic.
type plantText struct {
	Text   string
	Box    Box
	Size   float64
	Part   plantPart
	Italic bool
}

// plantCard is one card: its box (a dashed cut line when Cut), the inner
// box everything stays in, the plate box (zero for none), the name in the
// lettering at TitleSize (empty text for none), the texts in the Go font
// and the writing lines. Plant is the plant's 1-based index, 0 for a blank
// card.
type plantCard struct {
	Card, Inner Box
	Cut         bool
	Plant       int
	Plate       Box
	Title       quoteLine
	TitleSize   float64
	Texts       []plantText
	Rules       []moonRule
}

// moved is c moved by dx, dy.
func (c plantCard) moved(dx, dy float64) plantCard {
	mv := func(b Box) Box { return Box{X: b.X + dx, Y: b.Y + dy, W: b.W, H: b.H} }
	c.Card, c.Inner, c.Title.Box = mv(c.Card), mv(c.Inner), mv(c.Title.Box)
	if c.Plate != (Box{}) {
		c.Plate = mv(c.Plate)
	}
	texts := make([]plantText, len(c.Texts))
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

// plantsSheet is one page of cards.
type plantsSheet struct {
	Cards []plantCard
}

// sheets lays out the pages on page: a card per plant in order, then
// blank cards filling the last page (a page of them with no plants).
func (o PlantsOptions) sheets(page Page, l lettering, m plantMeasure, labels PageLabels) []plantsSheet {
	slots := recipeSlots(page, o.Size)
	card := Box{W: slots[0].W, H: slots[0].H}
	var cards []plantCard
	for i, p := range o.Plants {
		cards = append(cards, o.layoutPlant(i+1, p, card, l, m, labels))
	}
	if len(cards) == 0 || len(cards)%len(slots) != 0 {
		blank := o.layoutPlant(0, Plant{}, card, l, m, labels)
		for len(cards) == 0 || len(cards)%len(slots) != 0 {
			cards = append(cards, blank)
		}
	}
	var out []plantsSheet
	for i, c := range cards {
		if i%len(slots) == 0 {
			out = append(out, plantsSheet{})
		}
		s := slots[i%len(slots)]
		out[len(out)-1].Cards = append(out[len(out)-1].Cards, c.moved(s.X, s.Y))
	}
	return out
}

// plantFieldLines shares n writing lines between light, water, soil and
// notes: two each when there is room for two notes lines too, else one,
// the notes taking the rest. With fewer than four, the first fields get
// one each.
func plantFieldLines(n int) [4]int {
	per := 1
	if n >= plantTwoLines {
		per = 2
	}
	var out [4]int
	for i := range out {
		k := per
		if i == len(out)-1 {
			k = n
		}
		out[i] = max(0, min(k, n))
		n -= out[i]
	}
	return out
}

// layoutPlant lays out plant n (0 for a blank card), p, on a card the size
// of card at its origin: the plate at the top, when there is one; the
// name centred in the lettering (shrunk, then cut with "…"; a writing
// line when there is none) and the scientific name, small and in italics,
// under it; then the field labels in a column on the left, each over
// writing lines, its typed text printed on them and cut with "…" on the
// last. The notes fill the rest of the card.
func (o PlantsOptions) layoutPlant(n int, p Plant, card Box, l lettering, m plantMeasure, labels PageLabels) plantCard {
	st := plantStyles[o.Size]
	in := inset(card, recipePad)
	c := plantCard{Card: card, Inner: in, Cut: o.Size != RecipesPage, Plant: n}
	name, sci := plantNames(p)
	pitch := lineHeight(st.bodySize) * recipeBlankPitch

	y := in.Y
	if p.Image != "" {
		// The text keeps room for four writing lines below the names.
		text := lineHeight(st.nameMax) + lineHeight(st.sciSize) + 2*plantGap + 4*pitch
		if h := min(in.H*plantPlateShare, in.H-text); h > 0 {
			c.Plate = Box{X: in.X, Y: y, W: in.W, H: h}
			y += h + plantGap
		}
	}
	band := Box{X: in.X, Y: y, W: in.W, H: lineHeight(st.nameMax)}
	c.Title, c.TitleSize = fitRecipeTitle(name, "", band, st.nameMax, l, m.title)
	if c.Title.Text == "" {
		c.Rules = append(c.Rules, moonRule{X1: band.X, X2: band.X + band.W, Y: band.Y + band.H})
	}
	y += band.H
	if sci != "" {
		text := fitText(sci, in.W, st.sciSize, m.italic)
		w := m.italic(text, false, st.sciSize)
		c.Texts = append(c.Texts, plantText{Text: text, Box: Box{X: in.X + (in.W-w)/2, Y: y, W: w, H: lineHeight(st.sciSize)}, Size: st.sciSize, Part: plantScientific, Italic: true})
		y += lineHeight(st.sciSize)
	}
	y += plantGap

	fields := []struct{ label, text string }{
		{labels.PlantLight, p.Light}, {labels.PlantWater, p.Water}, {labels.PlantSoil, p.Soil}, {labels.PlantNotes, p.Notes},
	}
	// The label column is as wide as the widest label, a third of the
	// card at most (longer labels are cut with "…").
	labelW := 0.0
	for _, f := range fields {
		labelW = max(labelW, m.text(f.label, false, st.labelSize))
	}
	labelW = min(labelW, in.W/3)
	x := in.X + labelW + recipeLabelRuleGap
	lines := plantFieldLines(int(math.Floor((in.Y + in.H - y + fitTolerance) / pitch)))
	for i, f := range fields {
		if lines[i] == 0 {
			continue
		}
		label := fitText(f.label, labelW, st.labelSize, m.text)
		c.Texts = append(c.Texts, plantText{Text: label, Box: Box{X: in.X, Y: y + pitch - lineHeight(st.labelSize), W: m.text(label, false, st.labelSize), H: lineHeight(st.labelSize)}, Size: st.labelSize, Part: plantLabel})
		body := func(s string) float64 { return m.text(s, false, st.bodySize) }
		typed := fitLines(f.text, in.X+in.W-x, lines[i], body)
		for k := range lines[i] {
			y += pitch
			c.Rules = append(c.Rules, moonRule{X1: x, X2: in.X + in.W, Y: y})
			if k < len(typed) {
				c.Texts = append(c.Texts, plantText{Text: typed[k], Box: Box{X: x, Y: y - lineHeight(st.bodySize), W: body(typed[k]), H: lineHeight(st.bodySize)}, Size: st.bodySize, Part: plantField})
			}
		}
	}
	return c
}

// addPlantTextFonts embeds the Go font in pdf, with its italic face when
// italic (only scientific names use it).
func addPlantTextFonts(pdf *pdfDoc, italic bool) error {
	if err := addRegularTextFont(pdf); err != nil {
		return err
	}
	if italic {
		pdf.AddUTF8FontFromBytes(textFont, "I", goitalic.TTF)
		if err := pdf.Error(); err != nil {
			return fmt.Errorf("load fonts: %w", err)
		}
	}
	return nil
}

// pdfItalicMeasure measures text in the italic Go font of pdf, which must
// be added. The bold flag of measureFunc is ignored.
func pdfItalicMeasure(pdf *pdfDoc) measureFunc {
	return func(text string, _ bool, pt float64) float64 {
		pdf.SetFont(textFont, "I", pt)
		return pdf.GetStringWidth(text)
	}
}

// draw draws the pages of cards. The page has no shared title (the kind
// does not use it), so the lettering's font is added here for the names,
// and the italic face only when a scientific name prints.
func (o PlantsOptions) draw(pdf *pdfDoc, page Page, _ string, l lettering, labels PageLabels) error {
	italic := false
	for _, p := range o.Plants {
		if _, sci := plantNames(p); sci != "" {
			italic = true
		}
	}
	if err := addPlantTextFonts(pdf, italic); err != nil {
		return err
	}
	if len(o.Plants) > 0 {
		if err := l.add(pdf); err != nil {
			return err
		}
	}
	m := plantMeasure{title: l.measure(pdf), text: pdfMeasure(pdf), italic: pdfItalicMeasure(pdf)}
	for _, s := range o.sheets(page, l, m, labels) {
		pdf.AddPage()
		if err := o.drawSheet(pdf, s, l); err != nil {
			return err
		}
	}
	return pdf.Error()
}

// drawSheet draws s on the current page: the dashed cut lines, then each
// card's plate, writing lines, name (in the lettering l, hollow at 14 pt
// or more with Colour-in) and texts. User text is drawn as text only. A
// plate that cannot be read is an error naming its plant.
func (o PlantsOptions) drawSheet(pdf *pdfDoc, s plantsSheet, l lettering) error {
	var cuts []Box
	for _, c := range s.Cards {
		if c.Cut {
			cuts = append(cuts, c.Card)
		}
	}
	drawCutLines(pdf, cuts)
	for _, c := range s.Cards {
		if c.Plate != (Box{}) {
			if err := o.drawPlate(pdf, c); err != nil {
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
			grey, style := titleGrey, ""
			if t.Part != plantField {
				grey = textGrey
			}
			if t.Italic {
				style = "I"
			}
			pdf.SetTextColor(grey, grey, grey)
			pdf.SetFont(textFont, style, t.Size)
			pdf.Text(t.Box.X, baseline(t.Box.Y, t.Size), t.Text)
		}
	}
	return nil
}

// containBox is the largest box of width / height ratio within b, centred
// in it.
func containBox(b Box, ratio float64) Box {
	w, h := b.W, b.W/ratio
	if h > b.H {
		w, h = b.H*ratio, b.H
	}
	return Box{X: b.X + (b.W-w)/2, Y: b.Y + (b.H-h)/2, W: w, H: h}
}

// drawPlate draws card c's plate, read now, whole: as large as fits its
// box with its aspect kept, centred (a plate is never cropped).
func (o PlantsOptions) drawPlate(pdf *pdfDoc, c plantCard) error {
	path := o.Plants[c.Plant-1].Image
	fail := func(err error) error {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return fmt.Errorf("plant care cards plant %d image %s: %w", c.Plant, filepath.Base(path), pe.Err)
		}
		return fmt.Errorf("plant care cards plant %d image %s: not a readable JPEG or PNG image", c.Plant, filepath.Base(path))
	}
	src, err := decodeImage(path)
	if err != nil {
		return fail(err)
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return fail(errors.New("image has no size"))
	}
	box := containBox(c.Plate, float64(b.Dx())/float64(b.Dy()))
	img, err := encodePrinted(path, src, box.H, false)
	if err != nil {
		return fail(err)
	}
	name := fmt.Sprintf("plant-image-%d", c.Plant)
	opts := fpdf.ImageOptions{ImageType: "JPG"}
	pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(img.jpeg))
	pdf.ImageOptions(name, box.X, box.Y, box.W, box.H, false, opts, 0, "")
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("plant care cards plant %d image %s: %w", c.Plant, filepath.Base(path), err)
	}
	return nil
}
