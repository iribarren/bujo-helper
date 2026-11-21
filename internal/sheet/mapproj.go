package sheet

import "math"

// Map projections of the Visited map page kind. Projected coordinates are
// on the unit sphere, x east and y north; the page layout scales and flips
// them.

// mapMaxDistance is how far, in degrees of great circle, a ring of a
// continent view may reach from the view's centre: past it, the Lambert
// azimuthal image of a ring is a large distorted shape around the edge of
// the projected disc (antipode artefacts), so the ring is skipped.
const mapMaxDistance = 100.0

// mapBoundaryStep is the step, in degrees, at which the edges of a view's
// lon/lat box are projected to find its bounds and the world's outline.
const mapBoundaryStep = 0.5

const degree = math.Pi / 180

// Visited map regions (see mapRegionViews).
const (
	MapWorld        = "world"
	MapAfrica       = "africa"
	MapAsia         = "asia"
	MapEurope       = "europe"
	MapNorthAmerica = "north-america"
	MapSouthAmerica = "south-america"
	MapOceania      = "oceania"
)

// MapRegions are the regions to choose from.
var MapRegions = []string{MapWorld, MapAfrica, MapAsia, MapEurope, MapNorthAmerica, MapSouthAmerica, MapOceania}

// naturalEarth projects p with the Natural Earth projection (Šavrič,
// Patterson and Jenny 2011), the polynomial form also used by
// d3-geo-projection's naturalEarth1.
func naturalEarth(p lonLat) (x, y float64) {
	l, f := p.Lon*degree, p.Lat*degree
	f2 := f * f
	f4 := f2 * f2
	x = l * (0.8707 - 0.131979*f2 + f4*(-0.013791+f4*(0.003971*f2-0.001529*f4)))
	y = f * (1.007226 + f2*(0.015085+f4*(-0.044475+0.028874*f2-0.005916*f4)))
	return x, y
}

// wrapLon is lon in degrees normalized to [-180, 180).
func wrapLon(lon float64) float64 {
	lon = math.Mod(lon+180, 360)
	if lon < 0 {
		lon += 360
	}
	return lon - 180
}

// laea projects p with the Lambert azimuthal equal-area projection centred
// at c. Longitudes are taken relative to the centre's, so a view may cross
// the antimeridian. The antipode of c, which has no image, maps to the
// origin; views never reach it (see mapMaxDistance).
func laea(c, p lonLat) (x, y float64) {
	l := wrapLon(p.Lon-c.Lon) * degree
	f, f0 := p.Lat*degree, c.Lat*degree
	cosC := math.Sin(f0)*math.Sin(f) + math.Cos(f0)*math.Cos(f)*math.Cos(l)
	if cosC <= -1 {
		return 0, 0
	}
	k := math.Sqrt(2 / (1 + cosC))
	x = k * math.Cos(f) * math.Sin(l)
	y = k * (math.Cos(f0)*math.Sin(f) - math.Sin(f0)*math.Cos(f)*math.Cos(l))
	return x, y
}

// angularDistance is the great-circle distance between a and b in degrees.
func angularDistance(a, b lonLat) float64 {
	fa, fb := a.Lat*degree, b.Lat*degree
	dl := (b.Lon - a.Lon) * degree
	// Haversine, accurate for small distances too.
	h := math.Pow(math.Sin((fb-fa)/2), 2) + math.Cos(fa)*math.Cos(fb)*math.Pow(math.Sin(dl/2), 2)
	return 2 * math.Asin(math.Min(1, math.Sqrt(h))) / degree
}

// mapRegionView is a region of the map: the projection's centre (unused by
// the world view) and the lon/lat box the view shows. MaxLon may pass 180
// for a view across the antimeridian.
type mapRegionView struct {
	Centre                         lonLat
	MinLon, MaxLon, MinLat, MaxLat float64
}

// mapRegionViews are the views of the MapRegions. The world stops at 58° S
// (no Antarctica in the data) and 84° N; the continents' boxes are tuned so
// each reads well on a portrait page.
var mapRegionViews = map[string]mapRegionView{
	MapWorld:        {MinLon: -180, MaxLon: 180, MinLat: -58, MaxLat: 84},
	MapEurope:       {Centre: lonLat{15, 52}, MinLon: -25, MaxLon: 45, MinLat: 34, MaxLat: 71},
	MapAfrica:       {Centre: lonLat{17, 2}, MinLon: -20, MaxLon: 55, MinLat: -36, MaxLat: 38},
	MapAsia:         {Centre: lonLat{90, 40}, MinLon: 25, MaxLon: 150, MinLat: -11, MaxLat: 78},
	MapNorthAmerica: {Centre: lonLat{-100, 45}, MinLon: -170, MaxLon: -50, MinLat: 7, MaxLat: 84},
	MapSouthAmerica: {Centre: lonLat{-60, -20}, MinLon: -92, MaxLon: -32, MinLat: -56, MaxLat: 13},
	MapOceania:      {Centre: lonLat{160, -20}, MinLon: 110, MaxLon: 200, MinLat: -48, MaxLat: 5},
}

// mapView is a region's projection and the rectangle it shows, in
// projected units: the bounds of its lon/lat box's edges.
type mapView struct {
	world                  bool
	region                 mapRegionView
	MinX, MinY, MaxX, MaxY float64
}

// newMapView returns the view of a region (one of MapRegions).
func newMapView(region string) mapView {
	v := mapView{world: region == MapWorld, region: mapRegionViews[region]}
	v.MinX, v.MinY = math.Inf(1), math.Inf(1)
	v.MaxX, v.MaxY = math.Inf(-1), math.Inf(-1)
	for _, p := range v.boundary() {
		x, y := v.project(p)
		v.MinX, v.MaxX = math.Min(v.MinX, x), math.Max(v.MaxX, x)
		v.MinY, v.MaxY = math.Min(v.MinY, y), math.Max(v.MaxY, y)
	}
	return v
}

// project projects p in the view's projection.
func (v mapView) project(p lonLat) (x, y float64) {
	if v.world {
		return naturalEarth(p)
	}
	return laea(v.region.Centre, p)
}

// keepRing reports whether a ring is drawn: always in the world view; in a
// continent view, only when every point is within mapMaxDistance of the
// centre.
func (v mapView) keepRing(ring []lonLat) bool {
	if v.world {
		return true
	}
	for _, p := range ring {
		if angularDistance(v.region.Centre, p) > mapMaxDistance {
			return false
		}
	}
	return true
}

// boundary is the edge of the view's lon/lat box, closed, every
// mapBoundaryStep degrees: the east meridian north, the north parallel
// west, the west meridian south and the south parallel east.
func (v mapView) boundary() []lonLat {
	r := v.region
	var out []lonLat
	steps := func(from, to float64) int { return int(math.Ceil(math.Abs(to-from) / mapBoundaryStep)) }
	edge := func(a, b lonLat) {
		n := steps(a.Lon, b.Lon) + steps(a.Lat, b.Lat)
		for i := 0; i < n; i++ {
			t := float64(i) / float64(n)
			out = append(out, lonLat{a.Lon + t*(b.Lon-a.Lon), a.Lat + t*(b.Lat-a.Lat)})
		}
	}
	se, ne := lonLat{r.MaxLon, r.MinLat}, lonLat{r.MaxLon, r.MaxLat}
	nw, sw := lonLat{r.MinLon, r.MaxLat}, lonLat{r.MinLon, r.MinLat}
	edge(se, ne)
	edge(ne, nw)
	edge(nw, sw)
	edge(sw, se)
	return append(out, se)
}

// outline is the world view's frame, its boundary in lon/lat (closed); nil
// for a continent view, which is framed by the map rectangle only.
func (v mapView) outline() []lonLat {
	if !v.world {
		return nil
	}
	return v.boundary()
}
