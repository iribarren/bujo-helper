// Package locale maps a store country to the language and ISBN registration
// groups of its local book editions. The downloader uses it to prefer covers
// of local editions and to fall back silently to any edition.
package locale

import (
	"slices"
	"strings"
)

// Locale describes the local book editions of a country.
type Locale struct {
	// Country is the lowercase ISO country code.
	Country string
	// Language is the MARC language code used by Open Library (e.g. "spa").
	// Empty when the country is not in the table.
	Language string
	// ISBNPrefixes are the ISBN-13 prefixes (EAN prefix + registration
	// group) of the country's publishers. Empty means no preference.
	ISBNPrefixes []string
}

// table lists the known countries. To support another country, add a line
// with its Open Library language code and its ISBN-13 group prefixes.
var table = map[string]Locale{
	"es": {Language: "spa", ISBNPrefixes: []string{"97884"}},
	"mx": {Language: "spa", ISBNPrefixes: []string{"978607", "978968", "978970"}},
	"ar": {Language: "spa", ISBNPrefixes: []string{"978950", "978987"}},
	"fr": {Language: "fre", ISBNPrefixes: []string{"9782"}},
	"de": {Language: "ger", ISBNPrefixes: []string{"9783"}},
	"it": {Language: "ita", ISBNPrefixes: []string{"97888"}},
	"pt": {Language: "por", ISBNPrefixes: []string{"978972", "978989"}},
	"br": {Language: "por", ISBNPrefixes: []string{"97885", "978655"}},
	"gb": {Language: "eng", ISBNPrefixes: []string{"9780", "9781"}},
	"us": {Language: "eng", ISBNPrefixes: []string{"9780", "9781"}},
}

// For returns the locale of a country code, ignoring case and surrounding
// spaces. An unknown country yields a Locale with only Country set.
func For(country string) Locale {
	c := strings.ToLower(strings.TrimSpace(country))
	l, ok := table[c]
	if !ok {
		return Locale{Country: c}
	}
	l.Country = c
	l.ISBNPrefixes = slices.Clone(l.ISBNPrefixes)
	return l
}

// IsLocal reports whether isbn13 was registered by one of the locale's
// publishers. It is false for an empty ISBN or a locale without prefixes.
func (l Locale) IsLocal(isbn13 string) bool {
	if isbn13 == "" {
		return false
	}
	for _, p := range l.ISBNPrefixes {
		if strings.HasPrefix(isbn13, p) {
			return true
		}
	}
	return false
}
