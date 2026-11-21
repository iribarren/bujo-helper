package sheet

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"testing"
)

// pageFormsTS is the Make pages form source, which mirrors MapRegions and
// MapColours (with the swatch colours) by hand.
const pageFormsTS = "../../frontend/src/pageForms.ts"

// tsTuple matches one entry of a TypeScript array of string tuples, such
// as ['teal', 'pages.colourTeal', '#2a9d8f'], capturing its first and
// third strings.
var tsTuple = regexp.MustCompile(`\[\s*'([^']*)'\s*,\s*'[^']*'\s*(?:,\s*'([^']*)'\s*)?\]`)

// tsArray returns the tuples of the array declared as `const name`.
func tsArray(t *testing.T, src, name string) [][]string {
	t.Helper()
	decl := regexp.MustCompile(`(?s)const ` + name + `\b[^=]*=\s*\[(.*?)\n\];`).FindStringSubmatch(src)
	if decl == nil {
		t.Fatalf("%s: no `const %s ... = [ ... ];`", pageFormsTS, name)
	}
	var out [][]string
	for _, m := range tsTuple.FindAllStringSubmatch(decl[1], -1) {
		out = append(out, m[1:])
	}
	return out
}

// The form offers exactly Go's regions and colours, in Go's order, and its
// swatches are the colours Go fills visited countries with.
func TestMapFormMatchesGo(t *testing.T) {
	// The frontend ships with the Go sources: a missing file fails, so a
	// move or rename can't silently turn this check off.
	raw, err := os.ReadFile(pageFormsTS)
	if err != nil {
		t.Fatalf("read the form source (moved? update pageFormsTS): %v", err)
	}
	src := string(raw)

	var regions []string
	for _, tuple := range tsArray(t, src, "MAP_REGIONS") {
		regions = append(regions, tuple[0])
	}
	if !reflect.DeepEqual(regions, MapRegions) {
		t.Errorf("MAP_REGIONS %q, want sheet.MapRegions %q", regions, MapRegions)
	}

	var colours, wantColours []string
	for _, tuple := range tsArray(t, src, "MAP_COLOURS") {
		colours = append(colours, tuple[0]+" "+tuple[1])
	}
	for _, c := range MapColours {
		rgb := mapColourRGB[c]
		wantColours = append(wantColours, fmt.Sprintf("%s #%02x%02x%02x", c, rgb.R, rgb.G, rgb.B))
	}
	if !reflect.DeepEqual(colours, wantColours) {
		t.Errorf("MAP_COLOURS %q, want sheet.MapColours with their fills %q", colours, wantColours)
	}
}
