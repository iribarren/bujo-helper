package sheet

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// spanishMapLabels are the Spanish labels the app passes, with the map's.
var spanishMapLabels = func() PageLabels {
	l := spanishPageLabels
	l.MapTitle = "Lugares que he visitado"
	l.MapVisited = "Países visitados"
	l.Language = "es"
	return l
}()

// visitedNames are the English names of the countries o resolved.
func visitedNames(o MapOptions) []string {
	var out []string
	for _, c := range o.visited {
		out = append(out, c.NameEN)
	}
	return out
}

func TestDecodeMapOptions(t *testing.T) {
	kind, err := PageKindByID(MapKind)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		raw         string
		region      string
		colour      string
		list        bool
		visited     []string
		wantErr     string // part of the error; "" for none
		wantErrExtr string // another part of the error
	}{
		{"no options: world, teal, list", "", "world", "teal", true, nil, "", ""},
		{"empty object", "{}", "world", "teal", true, nil, "", ""},
		{"null countries", `{"countries":null}`, "world", "teal", true, nil, "", ""},
		{"explicit with an unknown name", `{"region":"europe","countries":["France","es","Deutschland "," ","fra"],"colour":"coral","list":false}`,
			"europe", "coral", false, nil, `unknown country "Deutschland"`, ""},
		{"names, codes and duplicates", `{"region":"oceania","countries":["Japón","es","FRA","france",""],"colour":"lavender"}`,
			"oceania", "lavender", true, []string{"Japan", "Spain", "France"}, "", ""},
		{"every region", `{"region":"south-america","colour":"sage"}`, "south-america", "sage", true, nil, "", ""},
		{"unknown region", `{"region":"antarctica"}`, "", "", false, nil, `"antarctica"`, "region"},
		{"unknown colour", `{"colour":"red"}`, "", "", false, nil, `"red"`, "colour"},
		{"unknown country", `{"countries":["Atlantis"]}`, "", "", false, nil, `unknown country "Atlantis"`, ""},
		{"ambiguous country", `{"countries":["San Martín"]}`, "", "", false, nil, `country "San Martín" matches`, " and "},
		{"unknown field", `{"region":"world","zoom":2}`, "", "", false, nil, `"zoom"`, ""},
		{"countries as a string", `{"countries":"France"}`, "", "", false, nil, "countries", ""},
		{"trailing data", `{} {}`, "", "", false, nil, "options", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := kind.decode(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), tt.wantErrExtr) {
					t.Fatalf("err = %v, want one containing %q and %q", err, tt.wantErr, tt.wantErrExtr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, ok := doc.(MapOptions)
			if !ok || got.Region != tt.region || got.Colour != tt.colour || got.List != tt.list {
				t.Fatalf("decoded %#v", doc)
			}
			if names := visitedNames(got); !reflect.DeepEqual(names, tt.visited) {
				t.Errorf("visited %v, want %v", names, tt.visited)
			}
		})
	}
}

func TestMapRegistryEntry(t *testing.T) {
	k, err := PageKindByID(MapKind)
	if err != nil {
		t.Fatal(err)
	}
	if k.Label != "Visited map" || k.FileTag != "map" || !k.UsesTitle || k.UsesCut {
		t.Errorf("map kind %+v", k)
	}
	raw, _ := json.Marshal(k.Defaults)
	if string(raw) != `{"region":"world","countries":[],"colour":"teal","list":true}` {
		t.Errorf("defaults %s", raw)
	}
	if n, err := PageCount(MapKind, PageSettings{}, nil); n != 1 || err != nil {
		t.Errorf("PageCount = %d, %v", n, err)
	}
	// After the moon calendar.
	if i := len(PageKinds) - 1; PageKinds[i].ID != MapKind || PageKinds[i-1].ID != MoonKind {
		t.Errorf("map kind not listed after the moon calendar")
	}
}

func TestMapDefaultTitle(t *testing.T) {
	o := defaultMap
	if got := o.defaultTitle(PageLabels{}.resolved()); got != "Places I've been" {
		t.Errorf("English title %q", got)
	}
	if got := o.defaultTitle(spanishMapLabels.resolved()); got != "Lugares que he visitado" {
		t.Errorf("Spanish title %q", got)
	}
}

// The map labels default to English; the language is "en" unless "es".
func TestPageLabelsResolvedMap(t *testing.T) {
	en := PageLabels{}.resolved()
	if en.MapTitle != "Places I've been" || en.MapVisited != "Countries visited" || en.Language != "en" {
		t.Errorf("English map labels %+v", en)
	}
	es := spanishMapLabels.resolved()
	if es.MapTitle != "Lugares que he visitado" || es.MapVisited != "Países visitados" || es.Language != "es" {
		t.Errorf("Spanish map labels %+v", es)
	}
	for in, want := range map[string]string{" ES ": "es", "fr": "en", "": "en", "en": "en"} {
		if got := (PageLabels{Language: in}).resolved().Language; got != want {
			t.Errorf("language %q resolved to %q, want %q", in, got, want)
		}
	}
}

// decodeMapDoc decodes map options or fails the test.
func decodeMapDoc(t *testing.T, raw string) MapOptions {
	t.Helper()
	doc, err := decodeMap(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return doc.(MapOptions)
}

// manyCountries is a JSON list of the ISO3 codes of the first n countries.
func manyCountries(t *testing.T, n int) string {
	t.Helper()
	cs, err := mapCountries()
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, c := range cs {
		if c.ISO3 != "" && len(codes) < n {
			codes = append(codes, c.ISO3)
		}
	}
	raw, _ := json.Marshal(codes)
	return string(raw)
}

// On every region and paper, with and without the list and with no, a few
// or very many countries, the map and every text lie inside the margins
// below the title band, texts fit their boxes, and no shape strays far
// outside the map (no antipode or wrap-around artefacts).
func TestMapSheetGeometry(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	sets := map[string]string{
		"none": `[]`,
		"few":  `["France","Japan","Brazil","Malta","Kenya","Fiji"]`,
		"many": manyCountries(t, 200),
	}
	for _, p := range Papers {
		for _, region := range MapRegions {
			for _, list := range []bool{true, false} {
				for set, countries := range sets {
					for _, labels := range []PageLabels{{}, spanishMapLabels} {
						name := fmt.Sprintf("%s/%s/list=%v/%s/%s", p.ID, region, list, set, labels.resolved().Language)
						t.Run(name, func(t *testing.T) {
							o := decodeMapDoc(t, fmt.Sprintf(`{"region":%q,"countries":%s,"list":%v}`, region, countries, list))
							s := o.sheet(p.Page, labels.resolved(), measure)
							area := moonArea(p.Page)
							checkMapSheet(t, s, area, measure)
							if !list || set == "none" {
								if len(s.Texts) != 0 {
									t.Errorf("%d texts, want none", len(s.Texts))
								}
								// Centred down the area.
								top, bottom := s.Rect.Y-area.Y, area.Y+area.H-s.Rect.Y-s.Rect.H
								if !nearTol(top, bottom, 1e-6) {
									t.Errorf("map not centred: %.2f above, %.2f below", top, bottom)
								}
							} else if len(s.Texts) < 2 {
								t.Errorf("%d texts, want a caption and names", len(s.Texts))
							}
						})
					}
				}
			}
		}
	}
}

// checkMapSheet checks that the map and texts of s lie within area, that
// texts fit their boxes, and that no shape point lies far outside the map.
func checkMapSheet(t *testing.T, s mapSheet, area Box, measure measureFunc) {
	t.Helper()
	const eps = 1e-9
	inside := func(what string, b Box) {
		if b.X < area.X-eps || b.Y < area.Y-eps || b.X+b.W > area.X+area.W+eps || b.Y+b.H > area.Y+area.H+eps {
			t.Errorf("%s %+v leaves the area %+v", what, b, area)
		}
	}
	inside("map", s.Rect)
	if s.Rect.W < 50 || s.Rect.H < 30 {
		t.Errorf("map %+v too small", s.Rect)
	}
	r := s.Rect
	// Kept rings are within mapMaxDistance of a continent's centre, so
	// within about twice the map's size; in the world view, inside the
	// map, and no ring spans most of its width (a wrap-around streak).
	reach := 2 * max(r.W, r.H)
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	for _, sh := range s.Shapes {
		for _, ring := range sh.Rings {
			minX, maxX := ring[0].X, ring[0].X
			for _, q := range ring {
				if math.Abs(q.X-cx) > reach || math.Abs(q.Y-cy) > reach {
					t.Fatalf("%s: point %+v far outside the map %+v", sh.Country.NameEN, q, r)
				}
				minX, maxX = math.Min(minX, q.X), math.Max(maxX, q.X)
			}
			if s.Outline != nil && maxX-minX > 0.6*r.W {
				t.Errorf("%s: a ring spans %.0f of the world's %.0f mm", sh.Country.NameEN, maxX-minX, r.W)
			}
		}
	}
	for i, tx := range s.Texts {
		inside(fmt.Sprintf("text %q", tx.Text), tx.Box)
		if tx.Box.Y < r.Y+r.H-eps {
			t.Errorf("text %q overlaps the map", tx.Text)
		}
		if w := measure(tx.Text, false, tx.Size); w > tx.Box.W+eps {
			t.Errorf("text %q is %.2f mm wide in a %.2f mm box", tx.Text, w, tx.Box.W)
		}
		if h := lineHeight(tx.Size); h > tx.Box.H+eps {
			t.Errorf("text %q needs %.2f mm in a %.2f mm box", tx.Text, h, tx.Box.H)
		}
		if i > 0 && tx.Size < mapListMinSize-eps {
			t.Errorf("text %q at %v pt, below the floor", tx.Text, tx.Size)
		}
	}
}

// shapeOf returns the shape of the country with ISO3 code iso, or nil.
func shapeOf(s mapSheet, iso string) *mapShape {
	for i := range s.Shapes {
		if s.Shapes[i].Country.ISO3 == iso {
			return &s.Shapes[i]
		}
	}
	return nil
}

// Visited countries are filled with the chosen colour, the others light
// grey, the visited ones after the others.
func TestMapSheetFillsVisitedCountries(t *testing.T) {
	for _, colour := range MapColours {
		o := decodeMapDoc(t, fmt.Sprintf(`{"countries":["France","Japan","Peru"],"colour":%q}`, colour))
		s := o.sheet(A4, PageLabels{}.resolved(), fixedMeasure)
		want := mapColourRGB[colour]
		seenVisited := false
		for _, sh := range s.Shapes {
			visited := sh.Country.ISO3 == "FRA" || sh.Country.ISO3 == "JPN" || sh.Country.ISO3 == "PER"
			if sh.Visited != visited {
				t.Errorf("%s: visited %v", sh.Country.NameEN, sh.Visited)
			}
			if visited && sh.Fill != want || !visited && sh.Fill != mapLandGrey {
				t.Errorf("%s (%s): fill %v", sh.Country.NameEN, colour, sh.Fill)
			}
			if !visited && seenVisited {
				t.Errorf("%s: unvisited shape drawn after a visited one", sh.Country.NameEN)
			}
			seenVisited = seenVisited || visited
		}
		for _, iso := range []string{"FRA", "JPN", "PER", "DEU", "CHN"} {
			if shapeOf(s, iso) == nil {
				t.Errorf("no shape for %s on the world map", iso)
			}
		}
	}
	// Visible distinct colours.
	seen := map[mapRGB]bool{mapLandGrey: true}
	for _, c := range MapColours {
		if seen[mapColourRGB[c]] {
			t.Errorf("colour %s repeats another", c)
		}
		seen[mapColourRGB[c]] = true
	}
}

// fixedMeasure measures every rune 1.5 mm wide at 9 pt, scaled with the size.
func fixedMeasure(text string, _ bool, pt float64) float64 {
	return float64(len([]rune(text))) * 1.5 * pt / 9
}

// Visited countries too small to see get a marker at their centre, inside
// the map; large ones do not.
func TestMapSheetMarksSmallCountries(t *testing.T) {
	tests := []struct {
		region    string
		countries string
		markers   int
	}{
		{"world", `["Malta"]`, 1},
		{"world", `["Singapore"]`, 1},
		{"world", `["France"]`, 0},
		{"world", `["Malta","Singapore","France","Vatican"]`, 3},
		{"europe", `["Vatican","Monaco"]`, 2},
		{"europe", `["Japan"]`, 0}, // not in view: no shape, no marker
		{"south-america", `["Malta"]`, 0},
	}
	for _, tt := range tests {
		o := decodeMapDoc(t, fmt.Sprintf(`{"region":%q,"countries":%s}`, tt.region, tt.countries))
		s := o.sheet(A4, PageLabels{}.resolved(), fixedMeasure)
		if len(s.Markers) != tt.markers {
			t.Errorf("%s %s: %d markers, want %d", tt.region, tt.countries, len(s.Markers), tt.markers)
		}
		for _, m := range s.Markers {
			r := s.Rect
			if m.C.X < r.X || m.C.X > r.X+r.W || m.C.Y < r.Y || m.C.Y > r.Y+r.H || m.R <= 0 {
				t.Errorf("%s: marker %+v outside the map %+v", tt.region, m, r)
			}
		}
	}
	// Malta's marker sits on Malta.
	o := decodeMapDoc(t, `{"countries":["Malta"]}`)
	s := o.sheet(A4, PageLabels{}.resolved(), fixedMeasure)
	malta := shapeOf(s, "MLT")
	if malta == nil || len(s.Markers) != 1 {
		t.Fatal("no Malta shape or marker")
	}
	if c := s.Markers[0].C; c.X < malta.Min.X-1 || c.X > malta.Max.X+1 || c.Y < malta.Min.Y-1 || c.Y > malta.Max.Y+1 {
		t.Errorf("marker %+v not on Malta %+v..%+v", c, malta.Min, malta.Max)
	}
}

// texts are the strings of the sheet's texts.
func texts(s mapSheet) []string {
	var out []string
	for _, tx := range s.Texts {
		out = append(out, tx.Text)
	}
	return out
}

// The list prints the caption with the count and the names in the label
// language, sorted ignoring accents.
func TestMapSheetList(t *testing.T) {
	pdf := newPDF(A4)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	o := decodeMapDoc(t, `{"countries":["Japan","es","Germany","Austria","Ireland"]}`)
	es := texts(o.sheet(A4, spanishMapLabels.resolved(), measure))
	wantES := []string{"Países visitados: 5", "Alemania", "Austria", "España", "Irlanda", "Japón"}
	if !reflect.DeepEqual(es, wantES) {
		t.Errorf("Spanish list %q, want %q", es, wantES)
	}
	en := texts(o.sheet(A4, PageLabels{}.resolved(), measure))
	wantEN := []string{"Countries visited: 5", "Austria", "Germany", "Ireland", "Japan", "Spain"}
	if !reflect.DeepEqual(en, wantEN) {
		t.Errorf("English list %q, want %q", en, wantEN)
	}
	// Accented initials sort with their base letter.
	if got := sortedNames([]string{"Zambia", "Égypte", "Ecuador", "Fiji"}); !reflect.DeepEqual(got, []string{"Ecuador", "Égypte", "Fiji", "Zambia"}) {
		t.Errorf("sorted %q", got)
	}
}

// moreText matches the last cell of a truncated list: "+N" more countries.
var moreText = regexp.MustCompile(`^\+(\d+)$`)

// Very many countries never overflow: the names shrink, then the last
// cell counts the ones left out.
func TestMapSheetListOverflow(t *testing.T) {
	pdf := newPDF(A5)
	if err := addTextFonts(pdf); err != nil {
		t.Fatal(err)
	}
	measure := pdfMeasure(pdf)
	for _, p := range Papers {
		for _, region := range []string{MapWorld, MapNorthAmerica} {
			o := decodeMapDoc(t, fmt.Sprintf(`{"region":%q,"countries":%s}`, region, manyCountries(t, 240)))
			s := o.sheet(p.Page, PageLabels{}.resolved(), measure)
			checkMapSheet(t, s, moonArea(p.Page), measure)
			count := 0
			for _, tx := range s.Texts[1:] {
				if m := moreText.FindStringSubmatch(tx.Text); m != nil {
					var n int
					fmt.Sscan(m[1], &n)
					count += n
				} else {
					count++
				}
			}
			if count != len(o.visited) {
				t.Errorf("%s/%s: the list accounts for %d of %d countries", p.ID, region, count, len(o.visited))
			}
			if s.Texts[0].Text != fmt.Sprintf("Countries visited: %d", len(o.visited)) {
				t.Errorf("caption %q", s.Texts[0].Text)
			}
		}
	}
}

// When no size and column count fit, layoutMapList cuts names too wide for
// their column with an ellipsis and, when they still do not all fit, ends
// with "+K", the K names left out; a box too short for one row of names
// holds the caption alone. No text ever leaves the box or its own width.
func TestLayoutMapListTruncates(t *testing.T) {
	labels := PageLabels{}.resolved()
	numbered := func(n, runes int) []string {
		var out []string
		for i := range n {
			out = append(out, fmt.Sprintf("%02d", i)+strings.Repeat("x", runes-2))
		}
		return out
	}
	head := lineHeight(mapCaptionSize) + 1
	row := lineHeight(mapListMinSize)
	tests := []struct {
		name  string
		names []string
		rows  float64 // box height in rows of names at the floor size
		want  []string
	}{
		// 60 runes are 60 mm at 6 pt: wider than a 48 mm column.
		{"too wide: ellipsis", numbered(4, 60), 10, []string{"00x…", "01x…", "02x…", "03x…"}},
		{"too many: +K", numbered(20, 6), 3, append(numbered(11, 6), "+9")},
		{"too wide and too many", numbered(9, 60), 2, []string{"00x…", "01x…", "02x…", "03x…", "04x…", "05x…", "06x…", "+2"}},
		{"no room for a row", numbered(5, 6), 0.5, nil},
		{"no room at all", numbered(5, 6), 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			box := Box{X: 10, Y: 20, W: 100, H: head + tt.rows*row}
			got := layoutMapList(box, labels, tt.names, fixedMeasure)
			if len(got) == 0 || got[0].Text != fmt.Sprintf("Countries visited: %d", len(tt.names)) {
				t.Fatalf("no caption: %+v", got)
			}
			var cells []string
			for _, tx := range got[1:] {
				// Compare ellipsized names by their number and ending.
				c := tx.Text
				if strings.HasSuffix(c, "…") {
					c = c[:3] + "…"
				}
				cells = append(cells, c)
				if tx.Size != mapListMinSize {
					t.Errorf("%q at %v pt, want the floor %v pt", tx.Text, tx.Size, mapListMinSize)
				}
				if w := fixedMeasure(tx.Text, false, tx.Size); w > tx.Box.W+1e-9 {
					t.Errorf("%q is %.1f mm wide in a %.1f mm column", tx.Text, w, tx.Box.W)
				}
			}
			if !reflect.DeepEqual(cells, tt.want) {
				t.Errorf("cells %q, want %q", cells, tt.want)
			}
			for _, tx := range got[1:] {
				b := tx.Box
				if b.X < box.X-1e-9 || b.X+b.W > box.X+box.W+1e-9 || b.Y < box.Y || b.Y+b.H > box.Y+box.H+1e-9 {
					t.Errorf("%q at %+v leaves the box %+v", tx.Text, b, box)
				}
			}
		})
	}
}

// fillOps matches PDF path-painting operators that fill.
var fillOps = paintOps

// The map renders on every region and paper, filled or for colouring in:
// one page; filled, the visited colour is set iff a country is visited;
// Colour-in, no fills at all and the heavier visited outline iff a country
// is visited.
func TestBuildMapEveryRegionAndPaper(t *testing.T) {
	fillOp := func(c mapRGB) string {
		return fmt.Sprintf("%.3f %.3f %.3f rg", float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
	}
	heavy := fmt.Sprintf("%.2f w", mapVisitedWidth*72/25.4)
	for _, p := range Papers {
		for _, region := range MapRegions {
			for _, colourIn := range []bool{false, true} {
				for _, countries := range []string{`[]`, `["Malta","France","Japan","Brazil","Australia","Kenya","Mexico"]`} {
					visited := countries != `[]`
					name := fmt.Sprintf("%s/%s/colourIn=%v/visited=%v", p.ID, region, colourIn, visited)
					t.Run(name, func(t *testing.T) {
						opts := fmt.Sprintf(`{"region":%q,"countries":%s,"colour":"coral"}`, region, countries)
						out := renderPage(t, MapKind, PageSettings{Paper: p.ID, ColourIn: colourIn}, opts)
						if n := strings.Count(string(out), "/Type /Page\n"); n != 1 {
							t.Errorf("%d pages, want 1", n)
						}
						stream := pageStreams(out)
						fills := len(fillOps.FindAllString(stream, -1))
						hasColour := strings.Contains(stream, fillOp(mapColourRGB[MapCoral]))
						hasHeavy := strings.Contains(stream, heavy)
						if colourIn {
							if fills != 0 {
								t.Errorf("Colour-in: %d fills, want none", fills)
							}
							if hasHeavy != visited {
								t.Errorf("Colour-in: heavy outline %v, want %v", hasHeavy, visited)
							}
						} else {
							if fills == 0 {
								t.Error("filled: no fills")
							}
							if hasColour != visited {
								t.Errorf("filled: visited colour set %v, want %v", hasColour, visited)
							}
						}
						if !strings.Contains(stream, " re W n") || !strings.Contains(stream, "\nQ") {
							t.Error("the map is not clipped")
						}
						if !strings.Contains(stream, " Tj") {
							t.Error("no text drawn")
						}
					})
				}
			}
		}
	}
}

// The map prints a title: the shared one, else "Places I've been" in the
// UI language.
func TestBuildMapTitle(t *testing.T) {
	opts := `{"region":"europe","countries":["Spain"]}`
	none := pageStreams(renderPage(t, MapKind, PageSettings{}, opts))
	explicit := pageStreams(renderPage(t, MapKind, PageSettings{Title: "Places I've been"}, opts))
	other := pageStreams(renderPage(t, MapKind, PageSettings{Title: "Travels"}, opts))
	if none != explicit {
		t.Error("the default title is not \"Places I've been\"")
	}
	if none == other {
		t.Error("a set title did not replace the default")
	}
	es := pageStreams(renderPage(t, MapKind, PageSettings{Labels: spanishMapLabels}, opts))
	esExplicit := pageStreams(renderPage(t, MapKind, PageSettings{Labels: spanishMapLabels, Title: "Lugares que he visitado"}, opts))
	if es != esExplicit || es == none {
		t.Error("the Spanish default title is not \"Lugares que he visitado\"")
	}
}

// Drawing restores the line width and colours it changes.
func TestDrawMapRestoresState(t *testing.T) {
	for _, colourIn := range []bool{false, true} {
		pdf := newPDF(A4)
		pdf.AddPage()
		pdf.SetLineWidth(0.3)
		pdf.SetFillColor(1, 2, 3)
		pdf.SetDrawColor(4, 5, 6)
		o := decodeMapDoc(t, `{"countries":["Malta","France"]}`)
		drawMapShapes(pdf, o.sheet(A4, PageLabels{}.resolved(), fixedMeasure), colourIn)
		if w := pdf.GetLineWidth(); math.Abs(w-0.3) > 1e-9 {
			t.Errorf("colourIn=%v: line width %v", colourIn, w)
		}
		if r, g, b := pdf.GetFillColor(); r != 1 || g != 2 || b != 3 {
			t.Errorf("colourIn=%v: fill colour %d %d %d", colourIn, r, g, b)
		}
		if r, g, b := pdf.GetDrawColor(); r != 4 || g != 5 || b != 6 {
			t.Errorf("colourIn=%v: draw colour %d %d %d", colourIn, r, g, b)
		}
		if err := pdf.Error(); err != nil {
			t.Fatal(err)
		}
	}
}
