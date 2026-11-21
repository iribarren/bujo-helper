package sheet

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// countryAliases lists, per Natural Earth NAME, common names NE does not
// carry. Natural Earth has no admin-0 entry for England, Scotland, Wales or
// Northern Ireland, so they mean the United Kingdom. Aliases are matched
// normalized (case, accents and punctuation do not matter).
var countryAliases = map[string][]string{
	"United States of America": {"USA", "U.S.", "America", "EEUU", "EE. UU.", "EUA", "Estados Unidos de América"},
	"United Kingdom": {"UK", "U.K.", "Great Britain", "Britain", "England", "Scotland", "Wales",
		"Northern Ireland", "Gran Bretaña", "Inglaterra", "Escocia", "Gales", "Irlanda del Norte"},
	"Netherlands":          {"Holland", "Holanda"},
	"Czechia":              {"Chequia"},
	"Turkey":               {"Türkiye"},
	"Vatican":              {"Holy See", "Vaticano", "Santa Sede"},
	"Bosnia and Herz.":     {"Bosnia", "Bosnia Herzegovina"},
	"North Macedonia":      {"Macedonia"},
	"Kosovo":               {"XKX"},
	"Myanmar":              {"Burma"},
	"South Korea":          {"Korea", "Corea"},
	"North Korea":          {"DPRK"},
	"Vietnam":              {"Viet Nam"},
	"United Arab Emirates": {"UAE", "Emiratos Árabes"},
	"Saudi Arabia":         {"Arabia Saudí"},
	"eSwatini":             {"Swaziland", "Esuatini"},
	"Côte d'Ivoire":        {"Cote dIvoire"},
	"Dem. Rep. Congo":      {"DRC", "DR Congo", "RD Congo", "RDC", "Congo Kinshasa"},
	"Congo":                {"Congo Brazzaville", "Republic of Congo"},
	"South Africa":         {"Suráfrica"},
	"Timor-Leste":          {"Timor"},
	"Falkland Is.":         {"Malvinas"},
}

// countryFold spells out letters that Unicode decomposition does not split
// into a base letter and an accent.
var countryFold = map[rune]string{
	'ø': "o", 'æ': "ae", 'œ': "oe", 'ß': "ss", 'ł': "l", 'đ': "d", 'ð': "d", 'þ': "th", 'ı': "i",
}

// normalizeCountry folds a country name for matching: accents are removed,
// letters lowercased, "&" becomes "and", every other run of punctuation and
// spaces becomes one space, and a leading English "the" is dropped.
func normalizeCountry(s string) string {
	var b strings.Builder
	space := false
	word := func(w string) {
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteString(w)
	}
	for _, r := range norm.NFKD.String(s) {
		r = unicode.ToLower(r)
		switch {
		case unicode.Is(unicode.Mn, r):
			// Accent left over from decomposition.
		case r == '&':
			space = true
			word("and")
			space = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if f, ok := countryFold[r]; ok {
				word(f)
			} else {
				word(string(r))
			}
		default:
			space = true
		}
	}
	return strings.TrimPrefix(b.String(), "the ")
}

// countryIndex finds countries by normalized name, ISO code or alias.
type countryIndex struct {
	keys map[string][]*mapCountry
}

// newCountryIndex indexes every name and ISO code of the countries plus the
// aliases (NE NAME -> aliases; aliases of a missing country are ignored).
func newCountryIndex(countries []*mapCountry, aliases map[string][]string) *countryIndex {
	idx := &countryIndex{keys: map[string][]*mapCountry{}}
	byName := map[string]*mapCountry{}
	for _, c := range countries {
		for _, n := range c.Names {
			idx.add(normalizeCountry(n), c)
		}
		idx.add(strings.ToLower(c.ISO2), c)
		idx.add(strings.ToLower(c.ISO3), c)
		if len(c.Names) > 0 {
			byName[c.Names[0]] = c
		}
	}
	for name, list := range aliases {
		for _, alias := range list {
			if c := byName[name]; c != nil {
				idx.add(normalizeCountry(alias), c)
			}
		}
	}
	return idx
}

func (idx *countryIndex) add(key string, c *mapCountry) {
	if key == "" {
		return
	}
	for _, have := range idx.keys[key] {
		if have == c {
			return
		}
	}
	idx.keys[key] = append(idx.keys[key], c)
}

// lookup finds the country an entry names. When several countries share the
// key, the one whose NE NAME equals the entry wins; otherwise the entry is
// ambiguous.
func (idx *countryIndex) lookup(entry string) (*mapCountry, error) {
	entry = strings.TrimSpace(entry)
	key := normalizeCountry(entry)
	found := idx.keys[key]
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("unknown country %q", entry)
	case 1:
		return found[0], nil
	}
	var byName []*mapCountry
	for _, c := range found {
		if len(c.Names) > 0 && normalizeCountry(c.Names[0]) == key {
			byName = append(byName, c)
		}
	}
	if len(byName) == 1 {
		return byName[0], nil
	}
	names := make([]string, len(found))
	for i, c := range found {
		names[i] = c.NameEN
	}
	sort.Strings(names)
	list := strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	return nil, fmt.Errorf("country %q matches %s", entry, list)
}

var countryIdx struct {
	once sync.Once
	idx  *countryIndex
	err  error
}

// matchCountries resolves visited-country entries (English or Spanish names,
// ISO 3166-1 alpha-2/alpha-3 codes or aliases) to countries. Blank entries
// are skipped and a country named twice is returned once, in input order. It
// fails on the first unknown or ambiguous entry.
func matchCountries(entries []string) ([]*mapCountry, error) {
	countryIdx.once.Do(func() {
		countries, err := mapCountries()
		countryIdx.idx, countryIdx.err = newCountryIndex(countries, countryAliases), err
	})
	if countryIdx.err != nil {
		return nil, countryIdx.err
	}
	var out []*mapCountry
	seen := map[*mapCountry]bool{}
	for _, e := range entries {
		if normalizeCountry(e) == "" {
			continue
		}
		c, err := countryIdx.idx.lookup(e)
		if err != nil {
			return nil, err
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out, nil
}
