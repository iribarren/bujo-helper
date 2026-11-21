package sheet

import (
	"math"
	"regexp"
	"testing"
)

func TestMapDataIntegrity(t *testing.T) {
	countries, err := mapCountries()
	if err != nil {
		t.Fatalf("mapCountries: %v", err)
	}
	if n := len(countries); n < 235 || n > 245 {
		t.Errorf("got %d countries, want about 241", n)
	}
	iso2 := regexp.MustCompile(`^[A-Z]{2}$`)
	iso3 := regexp.MustCompile(`^[A-Z]{3}$`)
	// NE has no ISO code for these; Kosovo only has the user-assigned XK.
	noISO3 := map[string]bool{
		"Somaliland": true, "Turkish Republic of Northern Cyprus": true, "Siachen Glacier": true,
		"Kosovo": true, "Australian Indian Ocean Territories": true, "Ashmore and Cartier Islands": true,
	}
	seen2, seen3 := map[string]string{}, map[string]string{}
	for _, c := range countries {
		if c.ISO3 == "ATA" || c.Continent == "Antarctica" {
			t.Errorf("Antarctica present: %s", c.NameEN)
		}
		if c.NameEN == "" || c.NameES == "" || len(c.Names) == 0 || c.Continent == "" {
			t.Errorf("%q: missing names or continent: %+v", c.NameEN, c)
		}
		switch {
		case noISO3[c.NameEN]:
			if c.ISO3 != "" {
				t.Errorf("%s: unexpected ISO3 %q", c.NameEN, c.ISO3)
			}
		case !iso3.MatchString(c.ISO3):
			t.Errorf("%s: bad ISO3 %q", c.NameEN, c.ISO3)
		}
		if c.ISO2 != "" && !iso2.MatchString(c.ISO2) {
			t.Errorf("%s: bad ISO2 %q", c.NameEN, c.ISO2)
		}
		for code, seen := range map[string]map[string]string{c.ISO2: seen2, c.ISO3: seen3} {
			if code == "" {
				continue
			}
			if other, dup := seen[code]; dup {
				t.Errorf("code %s used by %s and %s", code, other, c.NameEN)
			}
			seen[code] = c.NameEN
		}
		if len(c.Rings) == 0 {
			t.Errorf("%s: no rings", c.NameEN)
		}
		for i, ring := range c.Rings {
			if len(ring) < 4 {
				t.Errorf("%s ring %d: %d points, want >= 4", c.NameEN, i, len(ring))
			}
			if ring[0] != ring[len(ring)-1] {
				t.Errorf("%s ring %d: not closed", c.NameEN, i)
			}
			for _, p := range ring {
				if p.Lon < -180 || p.Lon > 180 || p.Lat < -90 || p.Lat > 90 {
					t.Fatalf("%s ring %d: point %v out of range", c.NameEN, i, p)
				}
			}
		}
		if c.Min.Lon > c.Max.Lon || c.Min.Lat > c.Max.Lat {
			t.Errorf("%s: bad bounds %v %v", c.NameEN, c.Min, c.Max)
		}
	}
	if seen2["XK"] != "Kosovo" || seen2["FR"] != "France" || seen3["NOR"] != "Norway" || seen2["TW"] != "Taiwan" {
		t.Errorf("EH code fallbacks missing: XK=%q FR=%q NOR=%q TW=%q", seen2["XK"], seen2["FR"], seen3["NOR"], seen2["TW"])
	}
}

func TestMapDataKnownCountries(t *testing.T) {
	tests := []struct {
		iso3      string
		continent string
		at        lonLat // a point inside the country (capital)
	}{
		{"FRA", "Europe", lonLat{2.35, 48.86}},
		{"JPN", "Asia", lonLat{139.69, 35.69}},
		{"BRA", "South America", lonLat{-47.88, -15.79}},
		{"AUS", "Oceania", lonLat{149.13, -35.28}},
		{"LUX", "Europe", lonLat{6.13, 49.61}},
		{"SGP", "Asia", lonLat{103.82, 1.35}},
		{"MLT", "Europe", lonLat{14.45, 35.89}},
		{"VAT", "Europe", lonLat{12.434, 41.902}}, // NE 50m draws it slightly west
		{"USA", "North America", lonLat{-98, 39}},
		{"NOR", "Europe", lonLat{10.75, 59.91}},
	}
	for _, tt := range tests {
		t.Run(tt.iso3, func(t *testing.T) {
			c := mapCountryByISO3(tt.iso3)
			if c == nil {
				t.Fatalf("%s missing", tt.iso3)
			}
			if c.Continent != tt.continent {
				t.Errorf("continent %q, want %q", c.Continent, tt.continent)
			}
			if !countryContains(c, tt.at) {
				t.Errorf("%s does not contain %v (bounds %v..%v)", c.NameEN, tt.at, c.Min, c.Max)
			}
		})
	}
}

// TestMapDataFranceMainland checks a ring of France spans the hexagon around
// 2°E 46°N, so overseas parts did not swallow the mainland.
func TestMapDataFranceMainland(t *testing.T) {
	c := mapCountryByISO3("FRA")
	for _, ring := range c.Rings {
		minLon, minLat, maxLon, maxLat := ringBounds(ring)
		if minLon > -5 && minLon < -4 && maxLon > 7 && maxLon < 8.5 && minLat > 42 && minLat < 43 && maxLat > 50.5 && maxLat < 51.5 {
			return
		}
	}
	t.Error("no mainland France ring spanning about 4.8°W..8.2°E, 42.3°N..51.1°N")
}

func countryContains(c *mapCountry, p lonLat) bool {
	in := false
	for _, ring := range c.Rings {
		for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
			a, b := ring[i], ring[j]
			if (a.Lat > p.Lat) != (b.Lat > p.Lat) &&
				p.Lon < (b.Lon-a.Lon)*(p.Lat-a.Lat)/(b.Lat-a.Lat)+a.Lon {
				in = !in
			}
		}
	}
	return in
}

func ringBounds(ring []lonLat) (minLon, minLat, maxLon, maxLat float64) {
	minLon, minLat, maxLon, maxLat = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range ring {
		minLon, maxLon = math.Min(minLon, p.Lon), math.Max(maxLon, p.Lon)
		minLat, maxLat = math.Min(minLat, p.Lat), math.Max(maxLat, p.Lat)
	}
	return
}
