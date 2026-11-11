// Package match ranks search hits by how closely they match the requested item.
package match

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"cover-downloader/internal/media"
)

// Normalize folds text for comparison: accents are stripped, letters are
// lowercased, and every run of non-alphanumeric characters becomes a single
// space.
func Normalize(s string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range norm.NFKD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// Combining mark left over from decomposition: drop it.
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pendingSpace = false
			b.WriteRune(unicode.ToLower(r))
		default:
			pendingSpace = true
		}
	}
	return b.String()
}

// Rank returns a copy of items stable-sorted so the closest title matches
// come first. When item.Creator is set, hits whose creator contains the
// query's surname (its last word) win ties.
func Rank[T any](items []T, item media.Item, title func(T) string, creator func(T) string) []T {
	query := Normalize(item.Title)
	surname := lastWord(Normalize(item.Creator))

	type scored struct {
		v       T
		score   int
		surname bool
	}
	ranked := make([]scored, len(items))
	for i, v := range items {
		ranked[i] = scored{
			v:       v,
			score:   titleScore(query, title(v)),
			surname: surname != "" && strings.Contains(Normalize(creator(v)), surname),
		}
	}
	slices.SortStableFunc(ranked, func(a, b scored) int {
		if a.score != b.score {
			return a.score - b.score
		}
		switch {
		case a.surname && !b.surname:
			return -1
		case b.surname && !a.surname:
			return 1
		}
		return 0
	})

	out := make([]T, len(ranked))
	for i, s := range ranked {
		out[i] = s.v
	}
	return out
}

// titleScore mirrors the legacy rank_by_title: lower is better.
func titleScore(query, title string) int {
	full := Normalize(title)
	main := Normalize(mainTitle(title))
	switch {
	case main == query || full == query:
		return 0
	case strings.HasPrefix(full, query) || (main != "" && strings.HasPrefix(query, main)):
		return 1
	case strings.Contains(full, query):
		return 2
	default:
		return 3
	}
}

// mainTitle returns the part of a title before the first ':' or '('.
func mainTitle(title string) string {
	if i := strings.IndexAny(title, ":("); i >= 0 {
		return title[:i]
	}
	return title
}

func lastWord(s string) string {
	if i := strings.LastIndexByte(s, ' '); i >= 0 {
		return s[i+1:]
	}
	return s
}
