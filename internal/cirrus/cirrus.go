// Package cirrus prepares user text for MediaWiki CirrusSearch queries.
package cirrus

import (
	"strings"
	"unicode"
)

// Terms turns user text (a title, a name) into plain words for a
// CirrusSearch query (the srsearch of Wikidata, Wikimedia Commons and
// Wikiquote), so the text is searched for literally. Its search syntax
// would change the query: phrases ("..."), negation (-word, !word), wildcards (* ?), fuzzy
// search (~), keywords (intitle:...), regexes (/.../), escapes (\) and the
// operators AND, OR, NOT, && and ||. See
// https://www.mediawiki.org/wiki/Help:CirrusSearch.
//
// Every rune other than a letter, mark or number becomes a space, except
// an apostrophe or a full stop between two letters or numbers (Who's,
// 2.0); the words AND, OR and NOT are lowercased. The search analyser
// splits words at punctuation anyway, so this keeps the recall of the
// unquoted text, which a phrase would lower. The result is empty when the
// text has no words.
func Terms(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		switch {
		case isWordRune(r):
			b.WriteRune(r)
		case (r == '\'' || r == '’' || r == '.') && i > 0 && i+1 < len(rs) && isWordRune(rs[i-1]) && isWordRune(rs[i+1]):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	words := strings.Fields(b.String())
	for i, w := range words {
		switch w {
		case "AND", "OR", "NOT":
			words[i] = strings.ToLower(w)
		}
	}
	return strings.Join(words, " ")
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r)
}
