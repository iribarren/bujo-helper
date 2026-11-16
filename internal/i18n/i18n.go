// Package i18n translates the few user-facing strings the Go side shows:
// native dialogs, top-level errors, the overwrite prompt and PDF labels.
// The frontend translates the rest of the UI itself.
package i18n

import (
	"fmt"
	"strings"
)

// Lang is a UI language.
type Lang string

// The supported languages. EN is the default and the fallback.
const (
	EN Lang = "en"
	ES Lang = "es"
)

// Parse returns the language of a language tag or locale, such as "es",
// "es-ES" or "es_ES.UTF-8", ignoring case. Anything unsupported, including
// "", gives EN.
func Parse(s string) Lang {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-_.@"); i >= 0 {
		s = s[:i]
	}
	if Lang(s) == ES {
		return ES
	}
	return EN
}

// T returns the message for key in lang, formatted with args by
// fmt.Sprintf when there are any. A key missing in lang falls back to
// English, and a key missing there too gives the key itself.
func T(lang Lang, key string, args ...any) string {
	msg, ok := catalogs[lang][key]
	if !ok {
		if msg, ok = catalogs[EN][key]; !ok {
			msg = key
		}
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// All returns the message for key in every language, English first. It
// lets callers recognize a translated label, such as a dialog button.
func All(key string) []string {
	out := []string{T(EN, key)}
	for _, lang := range []Lang{ES} {
		out = append(out, T(lang, key))
	}
	return out
}
