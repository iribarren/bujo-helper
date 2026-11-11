package locale

import (
	"reflect"
	"testing"
)

func TestFor(t *testing.T) {
	tests := []struct {
		country string
		want    Locale
	}{
		{"es", Locale{Country: "es", Language: "spa", ISBNPrefixes: []string{"97884"}}},
		{" ES ", Locale{Country: "es", Language: "spa", ISBNPrefixes: []string{"97884"}}},
		{"mx", Locale{Country: "mx", Language: "spa", ISBNPrefixes: []string{"978607", "978968", "978970"}}},
		{"ar", Locale{Country: "ar", Language: "spa", ISBNPrefixes: []string{"978950", "978987"}}},
		{"fr", Locale{Country: "fr", Language: "fre", ISBNPrefixes: []string{"9782"}}},
		{"de", Locale{Country: "de", Language: "ger", ISBNPrefixes: []string{"9783"}}},
		{"it", Locale{Country: "it", Language: "ita", ISBNPrefixes: []string{"97888"}}},
		{"pt", Locale{Country: "pt", Language: "por", ISBNPrefixes: []string{"978972", "978989"}}},
		{"br", Locale{Country: "br", Language: "por", ISBNPrefixes: []string{"97885", "978655"}}},
		{"gb", Locale{Country: "gb", Language: "eng", ISBNPrefixes: []string{"9780", "9781"}}},
		{"US", Locale{Country: "us", Language: "eng", ISBNPrefixes: []string{"9780", "9781"}}},
		{"jp", Locale{Country: "jp"}},
		{"", Locale{}},
	}
	for _, tt := range tests {
		if got := For(tt.country); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("For(%q) = %+v, want %+v", tt.country, got, tt.want)
		}
	}
}

func TestForReturnsACopy(t *testing.T) {
	l := For("es")
	l.ISBNPrefixes[0] = "0"
	if got := For("es").ISBNPrefixes[0]; got != "97884" {
		t.Errorf("table was mutated through a returned locale: %q", got)
	}
}

func TestIsLocal(t *testing.T) {
	tests := []struct {
		country, isbn string
		want          bool
	}{
		{"es", "9788466353779", true},
		{"es", "9781101658055", false},
		{"es", "", false},
		{"ar", "9789506441746", true},
		{"es", "9789506441746", false},
		{"mx", "9786070000000", true},
		{"us", "9780441013593", true},
		{"jp", "9784000000000", false},
	}
	for _, tt := range tests {
		if got := For(tt.country).IsLocal(tt.isbn); got != tt.want {
			t.Errorf("For(%q).IsLocal(%q) = %v, want %v", tt.country, tt.isbn, got, tt.want)
		}
	}
}
