package sheet

import (
	"fmt"
	"math"
	"testing"
)

// nearTol reports whether a and b differ by less than tol.
func nearTol(a, b, tol float64) bool { return math.Abs(a-b) < tol }

// Reference values from the Natural Earth polynomial (Šavrič, Patterson
// and Jenny 2011, as in d3-geo-projection's naturalEarth1), computed
// independently.
func TestNaturalEarthProjection(t *testing.T) {
	tests := []struct {
		name string
		p    lonLat
		x, y float64
	}{
		{"origin", lonLat{0, 0}, 0, 0},
		{"antimeridian on the equator", lonLat{180, 0}, 0.8707 * math.Pi, 0},
		{"north pole", lonLat{0, 90}, 0, 1.4223905067305962},
		{"90° E, 45° N", lonLat{90, 45}, 1.2319937779929164, 0.7930513944125287},
		{"120° W, 30° S", lonLat{-120, -30}, -1.7456493288097872, -0.5291484743893573},
	}
	for _, tt := range tests {
		x, y := naturalEarth(tt.p)
		if !nearTol(x, tt.x, 1e-9) || !nearTol(y, tt.y, 1e-9) {
			t.Errorf("%s: (%v, %v), want (%v, %v)", tt.name, x, y, tt.x, tt.y)
		}
	}
	// Symmetric about both axes; y does not depend on the longitude.
	for _, p := range []lonLat{{30, 20}, {150, 70}, {-75, 10}, {179, -58}} {
		x, y := naturalEarth(p)
		mx, my := naturalEarth(lonLat{-p.Lon, p.Lat})
		fx, fy := naturalEarth(lonLat{p.Lon, -p.Lat})
		_, y0 := naturalEarth(lonLat{0, p.Lat})
		if !nearTol(mx, -x, 1e-12) || !nearTol(my, y, 1e-12) || !nearTol(fx, x, 1e-12) || !nearTol(fy, -y, 1e-12) || !nearTol(y0, y, 1e-12) {
			t.Errorf("%+v: not symmetric", p)
		}
	}
}

// Lambert azimuthal equal-area on the unit sphere: the centre maps to the
// origin and a point c degrees away lies 2·sin(c/2) from it, in its
// direction.
func TestLambertAzimuthalProjection(t *testing.T) {
	tests := []struct {
		name   string
		centre lonLat
		p      lonLat
		x, y   float64
	}{
		{"centre", lonLat{15, 52}, lonLat{15, 52}, 0, 0},
		{"90° east on the equator", lonLat{0, 0}, lonLat{90, 0}, math.Sqrt2, 0},
		{"north pole from the equator", lonLat{0, 0}, lonLat{0, 90}, 0, math.Sqrt2},
		{"60° west", lonLat{0, 0}, lonLat{-60, 0}, -1, 0},
		{"wraps past 180°", lonLat{160, -20}, lonLat{-170, -20}, 0, 0}, // only the sign of x is checked
	}
	for _, tt := range tests {
		x, y := laea(tt.centre, tt.p)
		if tt.name == "wraps past 180°" {
			if x <= 0 {
				t.Errorf("%s: x = %v, want east of the centre", tt.name, x)
			}
			continue
		}
		if !nearTol(x, tt.x, 1e-9) || !nearTol(y, tt.y, 1e-9) {
			t.Errorf("%s: (%v, %v), want (%v, %v)", tt.name, x, y, tt.x, tt.y)
		}
	}
	for _, tt := range []struct{ c, p lonLat }{
		{lonLat{15, 52}, lonLat{40, 60}},
		{lonLat{-100, 45}, lonLat{-155, 20}},
		{lonLat{160, -20}, lonLat{-175, -40}},
	} {
		x, y := laea(tt.c, tt.p)
		want := 2 * math.Sin(angularDistance(tt.c, tt.p)*math.Pi/360)
		if !nearTol(math.Hypot(x, y), want, 1e-9) {
			t.Errorf("%+v from %+v: %v from the centre, want %v", tt.p, tt.c, math.Hypot(x, y), want)
		}
	}
}

func TestAngularDistance(t *testing.T) {
	for _, tt := range []struct {
		a, b lonLat
		want float64
	}{
		{lonLat{0, 0}, lonLat{0, 0}, 0},
		{lonLat{0, 0}, lonLat{90, 0}, 90},
		{lonLat{0, 0}, lonLat{0, 90}, 90},
		{lonLat{0, 0}, lonLat{180, 0}, 180},
		{lonLat{170, 10}, lonLat{-170, 10}, 19.6931}, // across 180°
		{lonLat{15, 52}, lonLat{-165, -52}, 180},
	} {
		if got := angularDistance(tt.a, tt.b); !nearTol(got, tt.want, 1e-3) {
			t.Errorf("%+v to %+v: %v°, want %v°", tt.a, tt.b, got, tt.want)
		}
	}
}

// Continent views skip rings far from their centre (whose LAEA image is
// a huge distorted shape); the world view keeps everything.
func TestMapViewSkipsFarRings(t *testing.T) {
	near := []lonLat{{10, 50}, {12, 50}, {12, 52}, {10, 50}}
	antipode := []lonLat{{-166, -52}, {-164, -52}, {-164, -50}, {-166, -52}}
	half := []lonLat{{10, 50}, {140, 0}, {10, 40}, {10, 50}} // one point 110° away
	eu, world := newMapView(MapEurope), newMapView(MapWorld)
	if !eu.keepRing(near) || eu.keepRing(antipode) || eu.keepRing(half) {
		t.Error("Europe: want only the near ring kept")
	}
	if !world.keepRing(near) || !world.keepRing(antipode) || !world.keepRing(half) {
		t.Error("world: want every ring kept")
	}
}

// Every region has a view with a positive area; the world view spans the
// Natural Earth x range and stops well short of the south pole.
func TestMapViews(t *testing.T) {
	for _, r := range MapRegions {
		v := newMapView(r)
		if !(v.MaxX > v.MinX && v.MaxY > v.MinY) {
			t.Errorf("%s: empty view %+v", r, v)
		}
		aspect := (v.MaxY - v.MinY) / (v.MaxX - v.MinX)
		if aspect < 0.3 || aspect > 1.6 {
			t.Errorf("%s: view height/width %.2f", r, aspect)
		}
	}
	w := newMapView(MapWorld)
	if !nearTol(w.MaxX, 0.8707*math.Pi, 1e-9) || !nearTol(w.MinX, -0.8707*math.Pi, 1e-9) {
		t.Errorf("world x range %v..%v", w.MinX, w.MaxX)
	}
	_, south := naturalEarth(lonLat{0, -60})
	_, north := naturalEarth(lonLat{0, 84})
	if w.MinY < south || !nearTol(w.MaxY, north, 1e-9) {
		t.Errorf("world y range %v..%v", w.MinY, w.MaxY)
	}
	if len(w.outline()) < 100 || newMapView(MapEurope).outline() != nil {
		t.Error("want an outline for the world view only")
	}
}

// largestRing is the ring of c with the largest lon/lat bounding box: its
// main body (France without French Guiana).
func largestRing(c *mapCountry) []lonLat {
	var best []lonLat
	bestArea := -1.0
	for _, r := range c.Rings {
		minP, maxP := r[0], r[0]
		for _, p := range r {
			minP.Lon, minP.Lat = math.Min(minP.Lon, p.Lon), math.Min(minP.Lat, p.Lat)
			maxP.Lon, maxP.Lat = math.Max(maxP.Lon, p.Lon), math.Max(maxP.Lat, p.Lat)
		}
		if a := (maxP.Lon - minP.Lon) * (maxP.Lat - minP.Lat); a > bestArea {
			best, bestArea = r, a
		}
	}
	return best
}

// The key countries of each region have their main body inside its view:
// every point of the largest ring projects inside the view rectangle.
func TestMapRegionsShowTheirCountries(t *testing.T) {
	keys := map[string][]string{
		MapWorld:        {"France", "Brazil", "Australia", "Japan", "Canada", "South Africa", "New Zealand", "Iceland"},
		MapEurope:       {"France", "Poland", "Spain", "Iceland", "Portugal", "Greece", "Finland", "Ukraine"},
		MapAfrica:       {"Nigeria", "South Africa", "Egypt", "Madagascar", "Morocco", "Somalia"},
		MapAsia:         {"India", "Japan", "Indonesia", "Turkey", "Mongolia", "Philippines"},
		MapNorthAmerica: {"USA", "Mexico", "Canada", "Cuba", "Panama"},
		MapSouthAmerica: {"Brazil", "Argentina", "Chile", "Colombia", "Venezuela"},
		MapOceania:      {"Australia", "New Zealand", "Fiji", "Papua New Guinea", "Samoa"},
	}
	for _, region := range MapRegions {
		v := newMapView(region)
		cs, err := matchCountries(keys[region])
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cs {
			t.Run(fmt.Sprintf("%s/%s", region, c.NameEN), func(t *testing.T) {
				ring := largestRing(c)
				if !v.keepRing(ring) {
					t.Fatal("main body skipped")
				}
				for _, p := range ring {
					x, y := v.project(p)
					if x < v.MinX || x > v.MaxX || y < v.MinY || y > v.MaxY {
						t.Fatalf("%+v projects to (%.3f, %.3f), outside %+v", p, x, y, v)
					}
				}
			})
		}
	}
}
