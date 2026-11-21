//go:build ignore

// Command mapgen builds internal/sheet/mapdata/countries.bin, the embedded
// country map of the Visited map page kind, from the Natural Earth 1:50m
// admin-0 countries (public domain, https://www.naturalearthdata.com/).
//
// Run from internal/sheet with `go generate` (see mapdata.go) or:
//
//	go run mapgen/main.go -out mapdata/countries.bin
//
// It downloads the GeoJSON of a pinned natural-earth-vector release (or reads
// -in), drops Antarctica, simplifies every ring with Douglas–Peucker,
// quantizes it and writes the mapdata binary format. The download is parsed
// as JSON data only. Output is deterministic for a given input.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"time"

	"bujo-helper/internal/sheet/mapdata"
)

const (
	// neVersion is the pinned natural-earth-vector release.
	neVersion = "v5.1.2"
	neURL     = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/" + neVersion +
		"/geojson/ne_50m_admin_0_countries.geojson"
	maxDownload = 64 << 20

	// tolerance is the Douglas–Peucker tolerance in degrees.
	tolerance = 0.02
	// scale quantizes coordinates to 1/scale degree. 0.001° keeps the
	// smallest states (Vatican, Monaco) recognisable.
	scale = 1000
)

var (
	iso2RE = regexp.MustCompile(`^[A-Z]{2}$`)
	iso3RE = regexp.MustCompile(`^[A-Z]{3}$`)
)

type feature struct {
	Properties map[string]any `json:"properties"`
	Geometry   struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

func main() {
	in := flag.String("in", "", "read this GeoJSON file instead of downloading "+neURL)
	out := flag.String("out", "mapdata/countries.bin", "output file")
	flag.Parse()

	src, err := readSource(*in)
	if err != nil {
		log.Fatal(err)
	}
	var fc struct {
		Features []feature `json:"features"`
	}
	if err := json.Unmarshal(src, &fc); err != nil {
		log.Fatalf("parse GeoJSON: %v", err)
	}
	countries, err := build(fc.Features)
	if err != nil {
		log.Fatal(err)
	}
	data, err := mapdata.Encode(countries, scale)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatal(err)
	}
	rings, points := 0, 0
	for _, c := range countries {
		rings += len(c.Rings)
		for _, r := range c.Rings {
			points += len(r)
		}
	}
	fmt.Printf("Natural Earth %s: %d countries, %d rings, %d points, %d bytes -> %s\n",
		neVersion, len(countries), rings, points, len(data), *out)
}

func readSource(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(neURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", neURL, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownload))
}

// build converts the features to countries sorted by English name.
func build(features []feature) ([]mapdata.Country, error) {
	// Codes NE assigns natively (ISO_A2/ISO_A3); an _EH fallback code that
	// another feature owns natively is dropped (e.g. Indian Ocean Ter. -> AU).
	native := map[string]bool{}
	for _, f := range features {
		for _, k := range []string{"ISO_A2", "ISO_A3"} {
			if v := prop(f, k); iso2RE.MatchString(v) || iso3RE.MatchString(v) {
				native[v] = true
			}
		}
	}

	var out []mapdata.Country
	for _, f := range features {
		if prop(f, "ADM0_A3") == "ATA" || prop(f, "ISO_A3") == "ATA" {
			continue
		}
		c := mapdata.Country{
			ISO2:      code(f, "ISO_A2", "ISO_A2_EH", iso2RE, native),
			ISO3:      code(f, "ISO_A3", "ISO_A3_EH", iso3RE, native),
			Continent: prop(f, "CONTINENT"),
			NameEN:    first(prop(f, "NAME_EN"), prop(f, "NAME")),
		}
		c.NameES = first(prop(f, "NAME_ES"), c.NameEN)
		seen := map[string]bool{}
		for _, k := range []string{"NAME", "NAME_LONG", "ADMIN", "FORMAL_EN", "NAME_EN", "NAME_ES"} {
			if v := prop(f, k); v != "" && !seen[v] {
				seen[v] = true
				c.Names = append(c.Names, v)
			}
		}
		polys, err := polygons(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.NameEN, err)
		}
		c.Rings = simplifyCountry(polys)
		if len(c.Rings) == 0 {
			return nil, fmt.Errorf("%s: no ring survives simplification", c.NameEN)
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].NameEN < out[j].NameEN })
	return out, nil
}

// prop returns a string property, or "" when it is missing, not a string or
// NE's "-99" placeholder.
func prop(f feature, key string) string {
	s, _ := f.Properties[key].(string)
	if s == "-99" {
		return ""
	}
	return s
}

func code(f feature, key, fallback string, re *regexp.Regexp, native map[string]bool) string {
	if v := prop(f, key); re.MatchString(v) {
		return v
	}
	if v := prop(f, fallback); re.MatchString(v) && !native[v] {
		return v
	}
	if v := prop(f, fallback); v != "" {
		log.Printf("%s: %s fallback %q dropped", prop(f, "NAME"), key, v)
	}
	return ""
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// polygons returns the rings of every polygon of the feature (outer ring and
// holes alike), each as GeoJSON closed rings.
func polygons(f feature) ([][]mapdata.LonLat, error) {
	var multi [][][][]float64
	switch f.Geometry.Type {
	case "Polygon":
		var poly [][][]float64
		if err := json.Unmarshal(f.Geometry.Coordinates, &poly); err != nil {
			return nil, err
		}
		multi = [][][][]float64{poly}
	case "MultiPolygon":
		if err := json.Unmarshal(f.Geometry.Coordinates, &multi); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported geometry %q", f.Geometry.Type)
	}
	var rings [][]mapdata.LonLat
	for _, poly := range multi {
		for _, ring := range poly {
			r := make([]mapdata.LonLat, 0, len(ring))
			for _, p := range ring {
				if len(p) < 2 {
					return nil, fmt.Errorf("point with %d coordinates", len(p))
				}
				r = append(r, mapdata.LonLat{Lon: clamp(p[0], -180, 180), Lat: clamp(p[1], -90, 90)})
			}
			rings = append(rings, r)
		}
	}
	return rings, nil
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// simplifyCountry simplifies every ring and drops the ones that collapse. A
// country whose rings all collapse (a micro-state) keeps its largest ring
// unsimplified.
func simplifyCountry(rings [][]mapdata.LonLat) [][]mapdata.LonLat {
	var out [][]mapdata.LonLat
	for _, r := range rings {
		if s := snap(simplifyRing(r)); s != nil {
			out = append(out, s)
		}
	}
	if len(out) > 0 {
		return out
	}
	largest, area := []mapdata.LonLat(nil), 0.0
	for _, r := range rings {
		if a := math.Abs(ringArea(r)); a > area {
			largest, area = r, a
		}
	}
	if s := snap(largest); s != nil {
		return [][]mapdata.LonLat{s}
	}
	return nil
}

// simplifyRing runs Douglas–Peucker on a closed ring, split at the point
// farthest from its start so both halves are proper polylines.
func simplifyRing(ring []mapdata.LonLat) []mapdata.LonLat {
	open := ring
	if len(open) > 1 && open[0] == open[len(open)-1] {
		open = open[:len(open)-1]
	}
	if len(open) < 3 {
		return nil
	}
	far, best := 0, -1.0
	for i, p := range open {
		if d := math.Hypot(p.Lon-open[0].Lon, p.Lat-open[0].Lat); d > best {
			far, best = i, d
		}
	}
	closedRing := append(append([]mapdata.LonLat(nil), open...), open[0])
	a := douglasPeucker(closedRing[:far+1])
	b := douglasPeucker(closedRing[far:])
	return append(a, b[1:]...)
}

func douglasPeucker(pts []mapdata.LonLat) []mapdata.LonLat {
	if len(pts) < 3 {
		return append([]mapdata.LonLat(nil), pts...)
	}
	idx, maxD := 0, 0.0
	for i := 1; i < len(pts)-1; i++ {
		if d := segDist(pts[i], pts[0], pts[len(pts)-1]); d > maxD {
			idx, maxD = i, d
		}
	}
	if maxD <= tolerance {
		return []mapdata.LonLat{pts[0], pts[len(pts)-1]}
	}
	left := douglasPeucker(pts[:idx+1])
	right := douglasPeucker(pts[idx:])
	return append(left[:len(left)-1], right...)
}

func segDist(p, a, b mapdata.LonLat) float64 {
	dx, dy := b.Lon-a.Lon, b.Lat-a.Lat
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(p.Lon-a.Lon, p.Lat-a.Lat)
	}
	t := clamp(((p.Lon-a.Lon)*dx+(p.Lat-a.Lat)*dy)/l2, 0, 1)
	return math.Hypot(p.Lon-(a.Lon+t*dx), p.Lat-(a.Lat+t*dy))
}

// snap quantizes a ring to the output grid and removes repeated points. It
// returns nil when fewer than 3 distinct points or no area remain.
func snap(ring []mapdata.LonLat) []mapdata.LonLat {
	var out []mapdata.LonLat
	for _, p := range ring {
		q := mapdata.LonLat{Lon: math.Round(p.Lon*scale) / scale, Lat: math.Round(p.Lat*scale) / scale}
		if len(out) > 0 && out[len(out)-1] == q {
			continue
		}
		out = append(out, q)
	}
	if len(out) > 1 && out[0] == out[len(out)-1] {
		out = out[:len(out)-1]
	}
	if len(out) < 3 || ringArea(out) == 0 {
		return nil
	}
	return append(out, out[0])
}

// ringArea is the signed shoelace area in square degrees.
func ringArea(ring []mapdata.LonLat) float64 {
	a := 0.0
	for i := range ring {
		p, q := ring[i], ring[(i+1)%len(ring)]
		a += p.Lon*q.Lat - q.Lon*p.Lat
	}
	return a / 2
}
