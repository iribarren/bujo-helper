package i18n

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Lang
	}{
		{"es", ES},
		{"ES", ES},
		{"es-ES", ES},
		{"es_ES.UTF-8", ES},
		{"es-419", ES},
		{" es ", ES},
		{"en", EN},
		{"en-US", EN},
		{"", EN},
		{"fr", EN},
		{"esperanto", EN},
		{"e", EN},
	}
	for _, tt := range tests {
		if got := Parse(tt.in); got != tt.want {
			t.Errorf("Parse(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTFallsBackToEnglishThenToTheKey(t *testing.T) {
	saved := catalogs
	t.Cleanup(func() { catalogs = saved })
	catalogs = map[Lang]map[string]string{
		EN: {"both": "Both", "enOnly": "English only", "fmt": "%d items in %s"},
		ES: {"both": "Ambos", "fmt": "%d elementos en %s"},
	}
	tests := []struct {
		lang Lang
		key  string
		args []any
		want string
	}{
		{ES, "both", nil, "Ambos"},
		{EN, "both", nil, "Both"},
		{ES, "enOnly", nil, "English only"},
		{ES, "missing.key", nil, "missing.key"},
		{Lang("fr"), "both", nil, "Both"},
		{ES, "fmt", []any{3, "Covers"}, "3 elementos en Covers"},
		{EN, "fmt", []any{3, "Covers"}, "3 items in Covers"},
	}
	for _, tt := range tests {
		if got := T(tt.lang, tt.key, tt.args...); got != tt.want {
			t.Errorf("T(%q, %q, %v) = %q, want %q", tt.lang, tt.key, tt.args, got, tt.want)
		}
	}
}

func TestTDoesNotFormatWithoutArgs(t *testing.T) {
	// A message with a literal percent sign is returned as is.
	saved := catalogs
	t.Cleanup(func() { catalogs = saved })
	catalogs = map[Lang]map[string]string{EN: {"pct": "100% done"}}
	if got := T(EN, "pct"); got != "100% done" {
		t.Errorf("T = %q, want it unformatted", got)
	}
}

// verbRE matches a fmt verb with its flags, width and precision, but not an
// escaped percent sign.
var verbRE = regexp.MustCompile(`%[-+# 0]*[0-9]*(\.[0-9]+)?[a-zA-Z%]`)

func verbs(s string) []string {
	var out []string
	for _, v := range verbRE.FindAllString(s, -1) {
		if v != "%%" {
			out = append(out, v)
		}
	}
	return out
}

func TestCatalogsMatchTheEnglishKeysAndVerbs(t *testing.T) {
	en := catalogs[EN]
	if len(en) == 0 {
		t.Fatal("the English catalog is empty")
	}
	for _, lang := range []Lang{ES} {
		cat := catalogs[lang]
		for key, enMsg := range en {
			msg, ok := cat[key]
			if !ok {
				t.Errorf("%s: missing key %q", lang, key)
				continue
			}
			if msg == "" {
				t.Errorf("%s: empty message for %q", lang, key)
			}
			if ev, mv := verbs(enMsg), verbs(msg); !slices.Equal(ev, mv) {
				t.Errorf("%s %q: verbs %v, want %v as in English", lang, key, mv, ev)
			}
		}
		for key := range cat {
			if _, ok := en[key]; !ok {
				t.Errorf("%s: key %q is not in the English catalog", lang, key)
			}
		}
	}
	for lang, cat := range catalogs {
		for key, msg := range cat {
			for _, v := range verbs(msg) {
				if v[len(v)-1] == 'w' {
					t.Errorf("%s %q uses %s; wrap errors at the call site instead", lang, key, v)
				}
			}
		}
	}
}

// The month and weekday lists have one non-blank entry per month and day
// in every language.
func TestCalendarListsAreComplete(t *testing.T) {
	for lang := range catalogs {
		for key, n := range map[string]int{"pdf.months": 12, "pdf.weekdays": 7} {
			items := strings.Split(T(lang, key), ",")
			if len(items) != n {
				t.Errorf("%s %q: %d entries, want %d", lang, key, len(items), n)
			}
			for i, item := range items {
				if strings.TrimSpace(item) == "" {
					t.Errorf("%s %q: entry %d is blank", lang, key, i)
				}
			}
		}
	}
}

// The habit tracker title marks where the month name goes with "{month}"
// in every language.
func TestHabitsTitleHasTheMonth(t *testing.T) {
	for lang := range catalogs {
		if msg := T(lang, "pdf.habitsTitle"); strings.Count(msg, "{month}") != 1 {
			t.Errorf("%s %q: want one {month}", lang, msg)
		}
	}
}
