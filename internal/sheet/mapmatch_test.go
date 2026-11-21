package sheet

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestNormalizeCountry(t *testing.T) {
	tests := []struct{ in, want string }{
		{"  Côte d'Ivoire ", "cote d ivoire"},
		{"MÉXICO", "mexico"},
		{"Bosnia & Herzegovina", "bosnia and herzegovina"},
		{"U.S.A.", "u s a"},
		{"The Netherlands", "netherlands"},
		{"Guinea-Bissau", "guinea bissau"},
		{"Türkiye", "turkiye"},
		{"Færøerne", "faeroerne"},
		{"the", "the"},
	}
	for _, tt := range tests {
		if got := normalizeCountry(tt.in); got != tt.want {
			t.Errorf("normalizeCountry(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMatchCountries(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    []string // ISO3 codes, or NE NAME for countries without one
	}{
		{"english names", []string{"France", "Japan", "Germany"}, []string{"FRA", "JPN", "DEU"}},
		{"spanish names", []string{"Japón", "Alemania", "Estados Unidos", "España"}, []string{"JPN", "DEU", "USA", "ESP"}},
		{"long and formal names", []string{"Kingdom of Spain", "Russian Federation", "Lao PDR"}, []string{"ESP", "RUS", "LAO"}},
		{"iso codes", []string{"FR", "fra", "JP", "XK", "no"}, []string{"FRA", "FRA", "JPN", "Kosovo", "NOR"}},
		{"accents and case", []string{"cote d'ivoire", "MEXICO", "méxico", "  sao tome and principe "}, []string{"CIV", "MEX", "MEX", "STP"}},
		{"english aliases", []string{"USA", "UK", "England", "Scotland", "Holland", "Burma", "South Korea", "Korea", "North Korea", "Vatican", "Holy See", "Bosnia", "Macedonia", "Swaziland", "East Timor", "DRC", "DR Congo", "Ivory Coast", "Czech Republic", "Czechia", "Türkiye", "Turkey", "Russia", "America", "U.S."},
			[]string{"USA", "GBR", "GBR", "GBR", "NLD", "MMR", "KOR", "KOR", "PRK", "VAT", "VAT", "BIH", "MKD", "SWZ", "TLS", "COD", "COD", "CIV", "CZE", "CZE", "TUR", "TUR", "RUS", "USA", "USA"}},
		{"spanish aliases", []string{"Reino Unido", "Inglaterra", "EEUU", "EE. UU.", "Holanda", "Corea", "Vaticano", "Chequia", "Turquía", "Birmania", "Costa de Marfil"},
			[]string{"GBR", "GBR", "USA", "USA", "NLD", "KOR", "VAT", "CZE", "TUR", "MMR", "CIV"}},
		{"congo is the republic", []string{"Congo", "Dem. Rep. Congo"}, []string{"COG", "COD"}},
		{"similar names stay apart", []string{"Guinea", "Equatorial Guinea", "Niger", "Nigeria", "Dominica", "Dominican Republic", "Sudan", "South Sudan"},
			[]string{"GIN", "GNQ", "NER", "NGA", "DMA", "DOM", "SDN", "SSD"}},
		{"countries without iso code", []string{"Somaliland", "Kosovo", "N. Cyprus"}, []string{"Somaliland", "Kosovo", "N. Cyprus"}},
		{"duplicates collapse", []string{"Spain", "ES", "España", "spain"}, []string{"ESP"}},
		{"blank entries ignored", []string{"", "  ", "Peru", "\t"}, []string{"PER"}},
		{"order kept", []string{"Peru", "Chile", "Peru", "Argentina"}, []string{"PER", "CHL", "ARG"}},
		{"nothing", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchCountries(tt.entries)
			if err != nil {
				t.Fatalf("matchCountries(%q): %v", tt.entries, err)
			}
			var ids []string
			for _, c := range got {
				ids = append(ids, countryID(c))
			}
			if !reflect.DeepEqual(ids, dedupe(tt.want)) {
				t.Errorf("matchCountries(%q) = %v, want %v", tt.entries, ids, dedupe(tt.want))
			}
		})
	}
}

func TestMatchCountriesErrors(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    string
	}{
		{"unknown", []string{"France", "Narnia"}, `unknown country "Narnia"`},
		{"unknown code", []string{"ZZ"}, `unknown country "ZZ"`},
		{"partial name", []string{"United"}, `unknown country "United"`},
		{"ambiguous", []string{" San Martín "}, `country "San Martín" matches Saint Martin and Sint Maarten`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := matchCountries(tt.entries)
			if err == nil || err.Error() != tt.want {
				t.Errorf("got error %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMatchAmbiguous(t *testing.T) {
	a := &mapCountry{NameEN: "Alpha Land", Names: []string{"Alpha"}}
	b := &mapCountry{NameEN: "Beta Land", Names: []string{"Beta", "Shared"}}
	c := &mapCountry{NameEN: "Gamma Land", Names: []string{"Gamma", "Shared Isles"}, ISO2: "GA"}
	idx := newCountryIndex([]*mapCountry{a, b, c}, nil)
	// "Shared" is a name of b only; add a clash by hand.
	idx.add("shared", a)
	if _, err := idx.lookup("Shared"); err == nil || err.Error() != `country "Shared" matches Alpha Land and Beta Land` {
		t.Errorf("got %v, want ambiguity error", err)
	}
	// A clash where one country's NE NAME equals the entry resolves to it.
	idx.add("beta", c)
	if got, err := idx.lookup("beta"); err != nil || got != b {
		t.Errorf("lookup(beta) = %v, %v; want Beta Land", got, err)
	}
}

// TestCountryAliases checks every alias names an existing country and none
// shadows a name or code of a different country.
func TestCountryAliases(t *testing.T) {
	countries, err := mapCountries()
	if err != nil {
		t.Fatal(err)
	}
	plain := newCountryIndex(countries, nil)
	byName := map[string]*mapCountry{}
	for _, c := range countries {
		byName[c.Names[0]] = c
	}
	for target, aliases := range countryAliases {
		c := byName[target]
		if c == nil {
			t.Errorf("aliases %q: no country named %q", aliases, target)
			continue
		}
		for _, alias := range aliases {
			for _, other := range plain.keys[normalizeCountry(alias)] {
				if other != c {
					t.Errorf("alias %q -> %s shadows %s", alias, target, other.NameEN)
				}
			}
		}
	}
}

// TestCountryKeyCollisions lists the normalized names and codes shared by
// several countries and checks each resolves to one country by NE NAME, or
// is a known genuine ambiguity.
func TestCountryKeyCollisions(t *testing.T) {
	countries, err := mapCountries()
	if err != nil {
		t.Fatal(err)
	}
	// NE_ES names both Saint-Martin (FR) and Sint Maarten (NL) "San Martín".
	ambiguous := map[string]bool{"san martin": true}
	idx := newCountryIndex(countries, countryAliases)
	var keys []string
	for k, cs := range idx.keys {
		if len(cs) > 1 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		var names []string
		for _, c := range idx.keys[k] {
			names = append(names, c.NameEN)
		}
		got, err := idx.lookup(k)
		if ambiguous[k] {
			if err == nil {
				t.Errorf("collision %q (%s) resolved to %s, want ambiguous", k, strings.Join(names, ", "), got.NameEN)
			}
			t.Logf("collision %q (%s) is ambiguous", k, strings.Join(names, ", "))
			continue
		}
		if err != nil {
			t.Errorf("collision %q (%s) does not resolve: %v", k, strings.Join(names, ", "), err)
			continue
		}
		t.Logf("collision %q (%s) -> %s", k, strings.Join(names, ", "), got.NameEN)
	}
}

func countryID(c *mapCountry) string {
	if c.ISO3 != "" {
		return c.ISO3
	}
	return c.Names[0]
}

func dedupe(ids []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
