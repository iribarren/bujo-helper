package sheet

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
)

// ConversionsKind is the ID of the kitchen conversions page kind: a chart
// of volume, weight, oven temperature and cups-to-grams conversions.
const ConversionsKind = "conversions"

// Kitchen conversions drawing settings: lengths in millimetres, font sizes
// in points.
const (
	conversionsGap        = 6.0  // between the two columns and the two rows of tables
	conversionsMaxHeading = 16.0 // table headings, shrunk to fit the column
	conversionsMinHeading = 8.0
	conversionsHeadingPad = 1.5  // between a heading and its table
	conversionsMaxText    = 12.0 // table cells, shrunk to fit rows and columns
	conversionsMinText    = 6.0
	conversionsSizeStep   = 0.5
	// conversionsTextShare is the share of a row's height a line of cell
	// text may take.
	conversionsTextShare = 0.75
	conversionsCellPad   = 1.5  // between a cell's text and the column's edges
	conversionsFillGrey  = 0xee // header row fill
)

// ConversionsOptions are the options of a kitchen conversions page: it has
// none, so its JSON is {}.
type ConversionsOptions struct{}

var defaultConversions = ConversionsOptions{}

// ConversionLabels are the words of the kitchen conversions page. Units
// with an international symbol (ml, g, kg, oz, lb, fl oz, °C, °F) are not
// translated.
type ConversionLabels struct {
	Title       string // the default title; "Kitchen conversions"
	Volume      string // table headings; "Volume", "Weight",
	Weight      string // "Oven temperatures" and "Cups to grams"
	Oven        string
	CupsToGrams string
	Cup         string // unit words; "cup", "tbsp" and "tsp"
	Tbsp        string
	Tsp         string
	GasMark     string // the oven table's gas mark column; "Gas mark"
	FanOven     string // the oven table's caption; "Fan oven: 20 °C lower"
	// Ingredients are the 10 ingredients of the cups-to-grams table, in
	// the order of conversionFigures.Cups.
	Ingredients []string
}

// englishIngredients are the default names of the cups-to-grams table.
var englishIngredients = []string{"Flour", "Sugar", "Brown sugar", "Icing sugar", "Butter",
	"Rice", "Rolled oats", "Honey", "Cocoa powder", "Milk or water"}

// resolved returns l trimmed, with every missing label set to its English
// default.
func (l ConversionLabels) resolved() ConversionLabels {
	return ConversionLabels{
		Title:       labelOr(l.Title, "Kitchen conversions"),
		Volume:      labelOr(l.Volume, "Volume"),
		Weight:      labelOr(l.Weight, "Weight"),
		Oven:        labelOr(l.Oven, "Oven temperatures"),
		CupsToGrams: labelOr(l.CupsToGrams, "Cups to grams"),
		Cup:         labelOr(l.Cup, "cup"),
		Tbsp:        labelOr(l.Tbsp, "tbsp"),
		Tsp:         labelOr(l.Tsp, "tsp"),
		GasMark:     labelOr(l.GasMark, "Gas mark"),
		FanOven:     labelOr(l.FanOven, "Fan oven: 20 °C lower"),
		Ingredients: labelList(l.Ingredients, englishIngredients),
	}
}

// Volume units of the conversion chart.
const (
	volumeCup  = "cup"
	volumeTbsp = "tbsp"
	volumeTsp  = "tsp"
	volumeFlOz = "fl oz"
)

// volumeFigure is Num/Den of a volume unit in millilitres.
type volumeFigure struct {
	Num, Den int
	Unit     string // one of the volume units
	ML       int
}

// weightFigure is From FromUnit in ToUnit.
type weightFigure struct {
	From     float64
	FromUnit string
	To       float64
	ToUnit   string
}

// ovenFigure is an oven setting in °C, °F and gas mark.
type ovenFigure struct {
	C, F, Gas int
}

// conversionFigures are the figures of the chart, in the cookbook
// conventions: a US cup of 240 ml, ounces and pounds rounded to the gram,
// and the usual gas mark table (gas mark n is 250 + 25n °F, its °C rounded
// to 10).
var conversionFigures = struct {
	Volume []volumeFigure
	Weight []weightFigure
	Oven   []ovenFigure
	// Cups are the grams in a cup of each ingredient, in the order of
	// ConversionLabels.Ingredients.
	Cups []int
}{
	Volume: []volumeFigure{
		{1, 1, volumeCup, 240},
		{3, 4, volumeCup, 180},
		{2, 3, volumeCup, 160},
		{1, 2, volumeCup, 120},
		{1, 3, volumeCup, 80},
		{1, 4, volumeCup, 60},
		{1, 1, volumeTbsp, 15},
		{1, 1, volumeTsp, 5},
		{1, 1, volumeFlOz, 30},
	},
	Weight: []weightFigure{
		{1, "oz", 28, "g"},
		{4, "oz", 113, "g"},
		{8, "oz", 227, "g"},
		{1, "lb", 454, "g"},
		{1, "kg", 2.2, "lb"},
	},
	Oven: []ovenFigure{
		{140, 275, 1},
		{150, 300, 2},
		{170, 325, 3},
		{180, 350, 4},
		{190, 375, 5},
		{200, 400, 6},
		{220, 425, 7},
		{230, 450, 8},
		{240, 475, 9},
	},
	Cups: []int{125, 200, 220, 120, 227, 185, 90, 340, 85, 240},
}

// conversionTable is a table of the chart: its heading, an optional header
// row, the rows and an optional caption below it. Cols are the columns'
// shares of the width; the first column is left-aligned when LeftFirst,
// the others are centred.
type conversionTable struct {
	Heading   string
	Header    []string
	Rows      [][]string
	Caption   string
	Cols      []float64
	LeftFirst bool
}

// units is how many rows of the chart t takes: its header, rows and
// caption.
func (t conversionTable) units() int {
	n := len(t.Rows)
	if len(t.Header) > 0 {
		n++
	}
	if t.Caption != "" {
		n++
	}
	return n
}

// conversionTables are the four tables, volume, weight, oven and cups to
// grams, in the resolved labels. Fractions are written with a slash: the
// Go font has ½ ¼ ¾ but not ⅓ or ⅔, and mixed forms would read unevenly.
// Spanish decimals take a comma. An ingredient list of the wrong length
// prints the English names, so unresolved labels are safe too.
func conversionTables(labels PageLabels) [4]conversionTable {
	c := labels.Conversions
	number := func(v float64) string {
		s := strconv.FormatFloat(v, 'f', -1, 64)
		if labels.Language == "es" {
			s = strings.ReplaceAll(s, ".", ",")
		}
		return s
	}
	unit := map[string]string{volumeCup: c.Cup, volumeTbsp: c.Tbsp, volumeTsp: c.Tsp, volumeFlOz: volumeFlOz}
	f := conversionFigures

	volume := conversionTable{Heading: c.Volume, Cols: []float64{0.5, 0.5}, LeftFirst: true}
	for _, v := range f.Volume {
		amount := strconv.Itoa(v.Num)
		if v.Den != 1 {
			amount += "/" + strconv.Itoa(v.Den)
		}
		volume.Rows = append(volume.Rows, []string{amount + " " + unit[v.Unit], fmt.Sprintf("%d ml", v.ML)})
	}
	weight := conversionTable{Heading: c.Weight, Cols: []float64{0.5, 0.5}, LeftFirst: true}
	for _, w := range f.Weight {
		weight.Rows = append(weight.Rows, []string{number(w.From) + " " + w.FromUnit, number(w.To) + " " + w.ToUnit})
	}
	oven := conversionTable{Heading: c.Oven, Header: []string{"°C", "°F", c.GasMark}, Caption: c.FanOven, Cols: []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}}
	for _, o := range f.Oven {
		oven.Rows = append(oven.Rows, []string{strconv.Itoa(o.C), strconv.Itoa(o.F), strconv.Itoa(o.Gas)})
	}
	cups := conversionTable{Heading: c.CupsToGrams, Header: []string{"", "1 " + c.Cup}, Cols: []float64{0.62, 0.38}, LeftFirst: true}
	ingredients := labelList(c.Ingredients, englishIngredients)
	for i, g := range f.Cups {
		cups.Rows = append(cups.Rows, []string{ingredients[i], fmt.Sprintf("%d g", g)})
	}
	return [4]conversionTable{volume, weight, oven, cups}
}

// validate accepts the options: there are none.
func (o ConversionsOptions) validate() error { return nil }

// pages is 1: the chart takes one page on every paper.
func (o ConversionsOptions) pages() int { return 1 }

// defaultTitle is the Conversions title label, "Kitchen conversions".
func (o ConversionsOptions) defaultTitle(labels PageLabels) string {
	return labels.Conversions.Title
}

// conversionHeading is a table heading in the lettering font, centred at
// the bottom of Box.
type conversionHeading struct {
	Text string
	Box  Box
	Size float64
}

// conversionsSheet is what a kitchen conversions page draws below its
// title: header row fills, the tables' borders, rules and cells, and their
// headings. Tables are the tables' boxes, without their captions.
type conversionsSheet struct {
	Headings []conversionHeading
	Tables   []Box
	Fills    []Box
	Grid     gridSheet
}

// sheet lays out the four tables two by two in the space below the title
// band, on every paper: each column half the width less conversionsGap,
// the first row of tables above the second. Every table row (header and
// caption included) is the same height, as tall as the taller table of
// each row of tables needs to fill the height. Headings are in the
// lettering l (measured by lm) and cells in the Go font (measured by tm),
// all cells at one size that fits rows and columns; the caption shrinks on
// its own. Header rows get a light fill, left off with Colour-in.
func (o ConversionsOptions) sheet(page Page, l lettering, lm, tm measureFunc, labels PageLabels) conversionsSheet {
	tables := conversionTables(labels)
	area := moonArea(page)
	colW := (area.W - conversionsGap) / 2
	headingH := lineHeight(conversionsMaxHeading)
	bandH := headingH + conversionsHeadingPad
	top := max(tables[0].units(), tables[1].units())
	bottom := max(tables[2].units(), tables[3].units())
	rowH := (area.H - conversionsGap - 2*bandH) / float64(top+bottom)

	// The column widths of each table, and the one cell size that fits
	// them all.
	cols := make([][]float64, len(tables))
	for i, t := range tables {
		for _, share := range t.Cols {
			cols[i] = append(cols[i], colW*share)
		}
	}
	fits := func(pt float64) bool {
		if lineHeight(pt) > rowH*conversionsTextShare {
			return false
		}
		for i, t := range tables {
			for _, row := range append([][]string{t.Header}, t.Rows...) {
				for c, cell := range row {
					if tm(cell, false, pt) > cols[i][c]-2*conversionsCellPad+fitTolerance {
						return false
					}
				}
			}
		}
		return true
	}
	size := conversionsMaxText
	for size > conversionsMinText && !fits(size) {
		size -= conversionsSizeStep
	}

	var s conversionsSheet
	for i, t := range tables {
		x := area.X + float64(i%2)*(colW+conversionsGap)
		y := area.Y
		if i >= 2 {
			y += bandH + float64(top)*rowH + conversionsGap
		}
		hs, ht := fitConversionText(t.Heading, l.fitWidth(colW), conversionsMaxHeading, conversionsMinHeading, lm)
		s.Headings = append(s.Headings, conversionHeading{Text: ht, Box: Box{X: x, Y: y, W: colW, H: headingH}, Size: hs})

		rows := t.Rows
		if len(t.Header) > 0 {
			rows = append([][]string{t.Header}, rows...)
		}
		table := Box{X: x, Y: y + bandH, W: colW, H: float64(len(rows)) * rowH}
		s.Tables = append(s.Tables, table)
		s.Grid.Boxes = append(s.Grid.Boxes, table)
		if len(t.Header) > 0 && !l.colourIn {
			s.Fills = append(s.Fills, Box{X: x, Y: table.Y, W: colW, H: rowH})
		}
		for r, row := range rows {
			rowY := table.Y + float64(r)*rowH
			if r > 0 {
				s.Grid.Lines = append(s.Grid.Lines, moonRule{X1: x, X2: x + colW, Y: rowY})
			}
			cellX := x
			for c, cell := range row {
				w := cols[i][c]
				if cell != "" {
					s.Grid.Texts = append(s.Grid.Texts, moonText{
						Text:    fitText(cell, w-2*conversionsCellPad, size, tm),
						Box:     Box{X: cellX + conversionsCellPad, Y: rowY, W: w - 2*conversionsCellPad, H: rowH},
						Size:    size,
						Centred: c > 0 || !t.LeftFirst,
					})
				}
				cellX += w
			}
		}
		if t.Caption != "" {
			w := colW - 2*conversionsCellPad
			cs, ct := fitConversionText(t.Caption, w, size, conversionsMinText, tm)
			s.Grid.Texts = append(s.Grid.Texts, moonText{Text: ct, Box: Box{X: x + conversionsCellPad, Y: table.Y + table.H, W: w, H: rowH}, Size: cs})
		}
	}
	return s
}

// fitConversionText is the largest size from maxPt down to minPt, in
// conversionsSizeStep steps, that text fits width at, measured by measure;
// at minPt the text is cut with "…" if it is still too wide.
func fitConversionText(text string, width, maxPt, minPt float64, measure measureFunc) (float64, string) {
	pt := maxPt
	for pt > minPt && measure(text, false, pt) > width+fitTolerance {
		pt = max(minPt, pt-conversionsSizeStep)
	}
	return pt, fitText(text, width, pt, measure)
}

// draw draws the page: the title centred in its band at the top in the
// lettering l, then the tables below it, their headings in l too.
func (o ConversionsOptions) draw(pdf *fpdf.Fpdf, page Page, title string, l lettering, labels PageLabels) error {
	if err := addRegularTextFont(pdf); err != nil {
		return err
	}
	if err := l.add(pdf); err != nil {
		return err
	}
	s := o.sheet(page, l, l.measure(pdf), pdfMeasure(pdf), labels)
	pdf.AddPage()
	drawPageTitle(pdf, page, title, l)
	pdf.SetFillColor(conversionsFillGrey, conversionsFillGrey, conversionsFillGrey)
	for _, b := range s.Fills {
		pdf.Rect(b.X, b.Y, b.W, b.H, "F")
	}
	drawGridSheet(pdf, s.Grid)
	pdf.SetTextColor(titleGrey, titleGrey, titleGrey)
	for _, h := range s.Headings {
		l.setFont(pdf, h.Size)
		x := h.Box.X + (h.Box.W-pdf.GetStringWidth(h.Text))/2
		l.text(pdf, x, baseline(h.Box.Y+h.Box.H-lineHeight(h.Size), h.Size), h.Size, h.Text)
	}
	return pdf.Error()
}
