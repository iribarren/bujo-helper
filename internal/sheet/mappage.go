package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/goregular"
)

// MapKind is the ID of the visited map page kind: a vector map of the
// world or of a continent with the visited countries filled in.
const MapKind = "map"

// Visited map colours: the fill of visited countries.
const (
	MapTeal     = "teal"
	MapCoral    = "coral"
	MapSage     = "sage"
	MapLavender = "lavender"
)

// MapColours are the colours to choose from.
var MapColours = []string{MapTeal, MapCoral, MapSage, MapLavender}

// mapRGB is a colour of the map.
type mapRGB struct{ R, G, B int }

// mapColourRGB are the MapColours: soft tones that print well and keep the
// country borders readable.
var mapColourRGB = map[string]mapRGB{
	MapTeal:     {0x2a, 0x9d, 0x8f}, // #2a9d8f
	MapCoral:    {0xee, 0x84, 0x66}, // #ee8466
	MapSage:     {0x8f, 0xae, 0x7e}, // #8fae7e
	MapLavender: {0xa3, 0x93, 0xc9}, // #a393c9
}

// Visited map drawing settings: lengths in millimetres, font sizes in
// points.
var (
	mapLandGrey    = mapRGB{0xe3, 0xe6, 0xea} // unvisited land
	mapBorder      = mapRGB{0xff, 0xff, 0xff} // country borders, filled
	mapOutlineGrey = mapRGB{0xcc, 0xcc, 0xcc} // the world's frame, filled
	mapMarkerEdge  = mapRGB{0x55, 0x55, 0x55} // small-country marker outline, filled
)

const (
	mapBorderWidth  = 0.12 // country borders, filled
	mapOutlineWidth = 0.2  // the world's frame, filled
	mapLineWidth    = 0.15 // outlines with Colour-in; marker outlines
	mapVisitedWidth = 0.45 // visited countries' outlines with Colour-in
	mapInset        = 0.3  // between the map rectangle (the clip) and the map
	mapMinStep      = 0.08 // closer points of a ring are dropped
	mapSmall        = 1.5  // a visited country smaller both ways gets a marker
	mapMarkerRadius = 0.8
	mapListGap      = 5.0  // between the map and the list
	mapColumnGap    = 4.0  // between list columns
	mapCaptionSize  = 10.0 // "Countries visited: N"
	mapListSize     = 9.0  // country names
	mapListMinSize  = 6.0  // smallest country names
	mapListStep     = 0.5  // font size step when names shrink
	mapMinShare     = 0.6  // smallest share of the area height the map keeps
	mapMinColumns   = 2
	mapMaxColumns   = 4
)

// MapOptions are the options of a visited map page.
type MapOptions struct {
	Region string `json:"region"` // one of MapRegions
	// Countries are the visited countries: English or Spanish names or
	// ISO 3166-1 codes (see matchCountries).
	Countries []string `json:"countries"`
	Colour    string   `json:"colour"` // one of MapColours
	List      bool     `json:"list"`   // print the visited countries under the map
	// visited are the Countries, resolved by decodeMap.
	visited []*mapCountry
}

var defaultMap = MapOptions{Region: MapWorld, Countries: []string{}, Colour: MapTeal, List: true}

// decodeMap decodes visited map options, like decodeOptions, and resolves
// the countries; an unknown or ambiguous country is an error.
func decodeMap(raw json.RawMessage) (pageDoc, error) {
	doc, err := decodeOptions(defaultMap)(raw)
	if err != nil {
		return nil, err
	}
	o := doc.(MapOptions)
	if o.visited, err = matchCountries(o.Countries); err != nil {
		return nil, err
	}
	return o, nil
}

func (o MapOptions) validate() error {
	if !oneOf(o.Region, MapRegions) {
		return fmt.Errorf("visited map region %q: want one of %v", o.Region, MapRegions)
	}
	if !oneOf(o.Colour, MapColours) {
		return fmt.Errorf("visited map colour %q: want one of %v", o.Colour, MapColours)
	}
	return nil
}

// pages is always 1.
func (o MapOptions) pages() int { return 1 }

// defaultTitle is "Places I've been" in the labels' language.
func (o MapOptions) defaultTitle(labels PageLabels) string { return labels.MapTitle }

// draw draws the page: the title centred in its band at the top in the
// lettering l, the map below it and, with List, the visited countries
// under the map.
func (o MapOptions) draw(pdf *fpdf.Fpdf, page Page, title string, l lettering, labels PageLabels) error {
	if _, err := mapCountries(); err != nil {
		return fmt.Errorf("map data: %w", err)
	}
	pdf.AddUTF8FontFromBytes(textFont, "", goregular.TTF)
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("load fonts: %w", err)
	}
	pdf.AddPage()
	if title != "" {
		band := dotGridTitleBand(page)
		drawHeader(pdf, l, band, fitHeader(pdf, l, title, band.W))
	}
	s := o.sheet(page, labels, pdfMeasure(pdf))
	drawMapShapes(pdf, s, l.colourIn)
	pdf.SetTextColor(textGrey, textGrey, textGrey)
	for _, t := range s.Texts {
		pdf.SetFont(textFont, "", t.Size)
		pdf.Text(t.Box.X, baseline(t.Box.Y+(t.Box.H-lineHeight(t.Size))/2, t.Size), t.Text)
	}
	return pdf.Error()
}

// mapShape is a country as drawn: its visible rings in page coordinates,
// to fill as one path with the even-odd rule, and its fill when not
// coloured in. Min and Max bound its visible rings.
type mapShape struct {
	Country  *mapCountry
	Rings    [][]Point
	Visited  bool
	Fill     mapRGB
	Min, Max Point
}

// mapMarker is the dot that marks a visited country too small to see.
type mapMarker struct {
	C    Point
	R    float64
	Fill mapRGB
}

// mapSheet is what a visited map page draws below its title: the map
// rectangle, which clips the map, the world's frame, the countries
// (unvisited first), the markers and the list (moonText lines, left
// aligned: the caption, then the names).
type mapSheet struct {
	Rect    Box
	Outline []Point
	Shapes  []mapShape
	Markers []mapMarker
	Texts   []moonText
}

// sheet lays out the map o describes on page, measuring the list in
// measure. With List and at least one country, the map takes the top of
// the area (it shrinks, down to mapMinShare of the area's height, to make
// room for the names) and the list goes below it; otherwise the map is
// centred in the area.
func (o MapOptions) sheet(page Page, labels PageLabels, measure measureFunc) mapSheet {
	area := moonArea(page)
	view := newMapView(o.Region)
	vw, vh := view.MaxX-view.MinX, view.MaxY-view.MinY
	names := o.visitedNames(labels)
	list := o.List && len(names) > 0

	maxH := area.H
	if list {
		rows := math.Ceil(float64(len(names)) / 3)
		need := mapListGap + lineHeight(mapCaptionSize) + rows*lineHeight(mapListSize)
		maxH = max(area.H-need, area.H*mapMinShare)
	}
	scale := min((area.W-2*mapInset)/vw, (maxH-2*mapInset)/vh)
	rect := Box{W: vw*scale + 2*mapInset, H: vh*scale + 2*mapInset}
	rect.X = area.X + (area.W-rect.W)/2
	rect.Y = area.Y + (area.H-rect.H)/2
	if list {
		rect.Y = area.Y
	}
	toPage := func(p lonLat) Point {
		x, y := view.project(p)
		return Point{X: rect.X + mapInset + (x-view.MinX)*scale, Y: rect.Y + mapInset + (view.MaxY-y)*scale}
	}

	s := mapSheet{Rect: rect}
	for _, p := range view.outline() {
		s.Outline = append(s.Outline, toPage(p))
	}
	visited := map[*mapCountry]bool{}
	for _, c := range o.visited {
		visited[c] = true
	}
	countries, _ := mapCountries() // checked by decodeMap
	var unvisitedShapes, visitedShapes []mapShape
	for _, c := range countries {
		sh, largest, ok := projectCountry(c, view, rect, toPage)
		if !ok {
			continue
		}
		sh.Visited = visited[c]
		sh.Fill = mapLandGrey
		if !sh.Visited {
			unvisitedShapes = append(unvisitedShapes, sh)
			continue
		}
		sh.Fill = mapColourRGB[o.Colour]
		visitedShapes = append(visitedShapes, sh)
		if largest.W < mapSmall && largest.H < mapSmall {
			mid := Point{X: largest.X + largest.W/2, Y: largest.Y + largest.H/2}
			if mid.X >= rect.X && mid.X <= rect.X+rect.W && mid.Y >= rect.Y && mid.Y <= rect.Y+rect.H {
				s.Markers = append(s.Markers, mapMarker{C: mid, R: mapMarkerRadius, Fill: sh.Fill})
			}
		}
	}
	s.Shapes = append(unvisitedShapes, visitedShapes...)
	if list {
		top := rect.Y + rect.H + mapListGap
		s.Texts = layoutMapList(Box{X: area.X, Y: top, W: area.W, H: area.Y + area.H - top}, labels, names, measure)
	}
	return s
}

// projectCountry projects the rings of c that view keeps and that reach
// rect, dropping points closer than mapMinStep to the previous one (a
// ring left with fewer than 3 points is kept whole). ok is false when no
// ring is visible; largest bounds the visible ring with the largest
// bounding box, the country's main body.
func projectCountry(c *mapCountry, view mapView, rect Box, toPage func(lonLat) Point) (sh mapShape, largest Box, ok bool) {
	sh = mapShape{Country: c, Min: Point{X: math.Inf(1), Y: math.Inf(1)}, Max: Point{X: math.Inf(-1), Y: math.Inf(-1)}}
	bestArea := -1.0
	for _, ring := range c.Rings {
		if !view.keepRing(ring) {
			continue
		}
		pts := make([]Point, len(ring))
		lo, hi := Point{X: math.Inf(1), Y: math.Inf(1)}, Point{X: math.Inf(-1), Y: math.Inf(-1)}
		for i, p := range ring {
			q := toPage(p)
			pts[i] = q
			lo.X, lo.Y = math.Min(lo.X, q.X), math.Min(lo.Y, q.Y)
			hi.X, hi.Y = math.Max(hi.X, q.X), math.Max(hi.Y, q.Y)
		}
		if hi.X < rect.X || lo.X > rect.X+rect.W || hi.Y < rect.Y || lo.Y > rect.Y+rect.H {
			continue
		}
		ok = true
		sh.Min.X, sh.Min.Y = math.Min(sh.Min.X, lo.X), math.Min(sh.Min.Y, lo.Y)
		sh.Max.X, sh.Max.Y = math.Max(sh.Max.X, hi.X), math.Max(sh.Max.Y, hi.Y)
		if a := (hi.X - lo.X) * (hi.Y - lo.Y); a > bestArea {
			bestArea = a
			largest = Box{X: lo.X, Y: lo.Y, W: hi.X - lo.X, H: hi.Y - lo.Y}
		}
		sh.Rings = append(sh.Rings, thinRing(pts))
	}
	return sh, largest, ok
}

// thinRing drops the points of ring closer than mapMinStep to the last
// point kept, keeping the first; if fewer than 3 points would be left, it
// returns ring unchanged.
func thinRing(ring []Point) []Point {
	out := []Point{ring[0]}
	for _, q := range ring[1:] {
		last := out[len(out)-1]
		if math.Hypot(q.X-last.X, q.Y-last.Y) >= mapMinStep {
			out = append(out, q)
		}
	}
	if len(out) < 3 {
		return ring
	}
	return out
}

// visitedNames are the names of the visited countries in the labels'
// language (English when a country has no Spanish name), sorted.
func (o MapOptions) visitedNames(labels PageLabels) []string {
	names := make([]string, len(o.visited))
	for i, c := range o.visited {
		names[i] = c.NameEN
		if labels.Language == "es" && c.NameES != "" {
			names[i] = c.NameES
		}
	}
	return sortedNames(names)
}

// sortedNames returns names sorted alphabetically ignoring case, accents
// and punctuation (as countries are matched), then as written.
func sortedNames(names []string) []string {
	out := append([]string(nil), names...)
	keys := map[string]string{}
	for _, n := range out {
		keys[n] = normalizeCountry(n)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ki, kj := keys[out[i]], keys[out[j]]; ki != kj {
			return ki < kj
		}
		return out[i] < out[j]
	})
	return out
}

// layoutMapList lays out the caption "<MapVisited>: N" at the top of box
// and the names below it in columns, filled down then across: the fewest
// columns (mapMinColumns to mapMaxColumns) at the largest size (mapListSize
// down to mapListMinSize) at which every name fits. If none does, the
// names take mapMaxColumns at mapListMinSize at most, cut short with an
// ellipsis, and when they still do not all fit, the last cell reads "+K",
// the K names left out.
func layoutMapList(box Box, labels PageLabels, names []string, measure measureFunc) []moonText {
	texts := []moonText{{
		Text: labels.MapVisited + ": " + strconv.Itoa(len(names)),
		Box:  Box{X: box.X, Y: box.Y, W: box.W, H: lineHeight(mapCaptionSize)},
		Size: mapCaptionSize,
	}}
	top := box.Y + lineHeight(mapCaptionSize) + 1
	avail := box.Y + box.H - top
	colW := func(cols int) float64 {
		return (box.W - float64(cols-1)*mapColumnGap) / float64(cols)
	}
	widest := func(size float64) float64 {
		w := 0.0
		for _, n := range names {
			w = max(w, measure(n, false, size))
		}
		return w
	}
	place := func(size float64, cols int, cells []string) []moonText {
		lh := lineHeight(size)
		w := colW(cols)
		rows := (len(cells) + cols - 1) / cols
		for i, n := range cells {
			x := box.X + float64(i/rows)*(w+mapColumnGap)
			y := top + float64(i%rows)*lh
			texts = append(texts, moonText{Text: fitText(n, w, size, measure), Box: Box{X: x, Y: y, W: w, H: lh}, Size: size})
		}
		return texts
	}
	n := len(names)
	for size := mapListSize; size >= mapListMinSize-1e-9; size -= mapListStep {
		rows := int(avail/lineHeight(size) + 1e-9)
		wide := widest(size)
		for cols := mapMinColumns; cols <= mapMaxColumns; cols++ {
			if n <= rows*cols && wide <= colW(cols)+1e-9 {
				return place(size, cols, names)
			}
		}
	}
	rows := max(int(avail/lineHeight(mapListMinSize)+1e-9), 1)
	cols := mapMinColumns
	for cols < mapMaxColumns && n > rows*cols {
		cols++
	}
	if room := rows * cols; n > room {
		cells := append(append([]string(nil), names[:room-1]...), "+"+strconv.Itoa(n-room+1))
		return place(mapListMinSize, cols, cells)
	}
	return place(mapListMinSize, cols, names)
}

// fitText is text, cut short with an ellipsis if it is wider than w at
// size pt.
func fitText(text string, w, size float64, measure measureFunc) string {
	if measure(text, false, size) <= w {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if cut := string(runes) + "…"; measure(cut, false, size) <= w {
			return cut
		}
	}
	return ""
}

// drawMapShapes draws the map of s inside its rectangle: filled, the world's
// frame in light grey, the countries in their fill with thin white borders
// and the markers in the visited colour; with colourIn, no fills: thin
// black outlines, the visited countries' and markers' heavier, drawn last.
// It restores the fill and draw colours and the line width.
func drawMapShapes(pdf *fpdf.Fpdf, s mapSheet, colourIn bool) {
	fr, fg, fb := pdf.GetFillColor()
	dr, dg, db := pdf.GetDrawColor()
	width := pdf.GetLineWidth()
	r := s.Rect
	pdf.ClipRect(r.X, r.Y, r.W, r.H, false)
	defer func() {
		// Restore inside the clip so fpdf's state matches the PDF's after
		// the clip's graphics state is popped.
		pdf.SetLineJoinStyle("miter")
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(dr, dg, db)
		pdf.SetLineWidth(width)
		pdf.ClipEnd()
	}()
	pdf.SetLineJoinStyle("round")
	lineWidth := -1.0
	setWidth := func(w float64) {
		if w != lineWidth {
			pdf.SetLineWidth(w)
			lineWidth = w
		}
	}
	setDraw := func(c mapRGB) { pdf.SetDrawColor(c.R, c.G, c.B) }

	if len(s.Outline) > 0 {
		setDraw(mapOutlineGrey)
		setWidth(mapOutlineWidth)
		if colourIn {
			setDraw(mapRGB{})
			setWidth(mapLineWidth)
		}
		tracePath(pdf, [][]Point{s.Outline})
		pdf.DrawPath("D")
	}
	if colourIn {
		setDraw(mapRGB{})
	} else {
		setDraw(mapBorder)
		setWidth(mapBorderWidth)
	}
	fill := mapRGB{-1, -1, -1}
	for _, sh := range s.Shapes {
		if colourIn {
			setWidth(mapLineWidth)
			if sh.Visited {
				setWidth(mapVisitedWidth)
			}
			tracePath(pdf, sh.Rings)
			pdf.DrawPath("D")
			continue
		}
		if sh.Fill != fill {
			pdf.SetFillColor(sh.Fill.R, sh.Fill.G, sh.Fill.B)
			fill = sh.Fill
		}
		tracePath(pdf, sh.Rings)
		pdf.DrawPath("DF*")
	}
	for _, m := range s.Markers {
		if colourIn {
			setWidth(mapVisitedWidth)
			pdf.Circle(m.C.X, m.C.Y, m.R, "D")
			continue
		}
		setDraw(mapMarkerEdge)
		setWidth(mapLineWidth)
		pdf.SetFillColor(m.Fill.R, m.Fill.G, m.Fill.B)
		pdf.Circle(m.C.X, m.C.Y, m.R, "DF")
	}
}

// tracePath adds rings to the current path, each closed.
func tracePath(pdf *fpdf.Fpdf, rings [][]Point) {
	for _, ring := range rings {
		pdf.MoveTo(ring[0].X, ring[0].Y)
		for _, q := range ring[1:] {
			pdf.LineTo(q.X, q.Y)
		}
		pdf.ClosePath()
	}
}
