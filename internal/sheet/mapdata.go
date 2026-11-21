package sheet

import (
	_ "embed"
	"math"
	"sync"

	"bujo-helper/internal/sheet/mapdata"
)

// mapDataFile is the country map of the Visited map page kind: Natural Earth
// 1:50m admin-0 countries (public domain), simplified and quantized by
// mapgen. Antarctica is left out.
//
//go:generate go run mapgen/main.go -out mapdata/countries.bin
//go:embed mapdata/countries.bin
var mapDataFile []byte

// lonLat is a geographic point in degrees.
type lonLat struct {
	Lon, Lat float64
}

// mapCountry is one country of the embedded map.
type mapCountry struct {
	ISO2, ISO3 string   // ISO 3166-1 codes; empty when Natural Earth has none
	Names      []string // every Natural Earth name (EN and ES), deduplicated; Names[0] is NE NAME
	NameEN     string   // English display name
	NameES     string   // Spanish display name
	Continent  string   // Natural Earth CONTINENT ("Europe", "North America", "Seven seas (open ocean)"...)
	// Rings holds the outer rings and holes of all the country's polygons,
	// each closed (last point = first). Draw them as one path with the
	// even-odd fill rule so holes (Lesotho in South Africa) stay empty.
	// Natural Earth splits polygons at ±180°, so no ring crosses it.
	Rings    [][]lonLat
	Min, Max lonLat // bounding box of all rings
}

var mapData struct {
	once      sync.Once
	countries []*mapCountry
	byISO3    map[string]*mapCountry
	err       error
}

// mapCountries returns the decoded embedded countries, sorted by English
// name. The data is decoded once; callers must not modify it.
func mapCountries() ([]*mapCountry, error) {
	mapData.once.Do(func() {
		raw, err := mapdata.Decode(mapDataFile)
		if err != nil {
			mapData.err = err
			return
		}
		mapData.byISO3 = map[string]*mapCountry{}
		for _, r := range raw {
			c := &mapCountry{
				ISO2: r.ISO2, ISO3: r.ISO3, Names: r.Names,
				NameEN: r.NameEN, NameES: r.NameES, Continent: r.Continent,
				Rings: make([][]lonLat, len(r.Rings)),
				Min:   lonLat{Lon: math.Inf(1), Lat: math.Inf(1)},
				Max:   lonLat{Lon: math.Inf(-1), Lat: math.Inf(-1)},
			}
			for i, ring := range r.Rings {
				c.Rings[i] = make([]lonLat, len(ring))
				for j, p := range ring {
					c.Rings[i][j] = lonLat(p)
					c.Min.Lon, c.Max.Lon = math.Min(c.Min.Lon, p.Lon), math.Max(c.Max.Lon, p.Lon)
					c.Min.Lat, c.Max.Lat = math.Min(c.Min.Lat, p.Lat), math.Max(c.Max.Lat, p.Lat)
				}
			}
			mapData.countries = append(mapData.countries, c)
			if c.ISO3 != "" {
				mapData.byISO3[c.ISO3] = c
			}
		}
	})
	return mapData.countries, mapData.err
}

// mapCountryByISO3 returns the country with an ISO 3166-1 alpha-3 code, or
// nil.
func mapCountryByISO3(code string) *mapCountry {
	if _, err := mapCountries(); err != nil {
		return nil
	}
	return mapData.byISO3[code]
}
