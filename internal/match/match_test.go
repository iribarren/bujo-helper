package match

import (
	"reflect"
	"testing"

	"cover-downloader/internal/media"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Cien años de soledad", "cien anos de soledad"},
		{"  The Name of the Wind!  ", "the name of the wind"},
		{"García Márquez", "garcia marquez"},
		{"Dune: Messiah (Book 2)", "dune messiah book 2"},
		{"ÉLAN—vital", "elan vital"},
		{"", ""},
		{"!!!", ""},
	}
	for _, tt := range tests {
		if got := Normalize(tt.in); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

type hit struct{ title, creator string }

func titles(hits []hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.title
	}
	return out
}

func TestRank(t *testing.T) {
	tests := []struct {
		name string
		item media.Item
		hits []hit
		want []string
	}{
		{
			name: "scores by title closeness, stable within a score",
			item: media.Item{Title: "Dune"},
			hits: []hit{
				{"The Road to Dune", ""},     // 2: contained
				{"Something Else", ""},       // 3
				{"Dune Messiah", ""},         // 1: starts with query
				{"Dune: Deluxe Edition", ""}, // 0: main title
				{"Children of Dune", ""},     // 2
				{"Dune", ""},                 // 0: exact
			},
			want: []string{"Dune: Deluxe Edition", "Dune", "Dune Messiah", "The Road to Dune", "Children of Dune", "Something Else"},
		},
		{
			name: "query starts with main title",
			item: media.Item{Title: "Dune Messiah"},
			hits: []hit{{"Unrelated", ""}, {"Dune (Collector's)", ""}},
			want: []string{"Dune (Collector's)", "Unrelated"},
		},
		{
			name: "accents are ignored",
			item: media.Item{Title: "Cien anos de soledad"},
			hits: []hit{{"Otra cosa", ""}, {"Cien años de soledad", ""}},
			want: []string{"Cien años de soledad", "Otra cosa"},
		},
		{
			name: "creator surname breaks ties",
			item: media.Item{Title: "Dune", Creator: "Frank Herbert"},
			hits: []hit{
				{"Dune", "Some Analyst"},
				{"Dune Messiah", "Frank Herbert"},
				{"Dune", "Frank Herbert; Brian Herbert"},
			},
			want: []string{"Dune", "Dune", "Dune Messiah"},
		},
		{
			name: "empty creator keeps order",
			item: media.Item{Title: "X"},
			hits: []hit{{"X", "b"}, {"X", "a"}},
			want: []string{"X", "X"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rank(tt.hits, tt.item,
				func(h hit) string { return h.title },
				func(h hit) string { return h.creator })
			if !reflect.DeepEqual(titles(got), tt.want) {
				t.Errorf("Rank titles = %q, want %q", titles(got), tt.want)
			}
		})
	}

	// The surname tie-break must put the Herbert "Dune" first.
	got := Rank([]hit{{"Dune", "Some Analyst"}, {"Dune", "Frank Herbert"}},
		media.Item{Title: "Dune", Creator: "Frank Herbert"},
		func(h hit) string { return h.title },
		func(h hit) string { return h.creator })
	if got[0].creator != "Frank Herbert" {
		t.Errorf("surname tie-break: first creator = %q", got[0].creator)
	}
}

func TestRankDoesNotMutateInput(t *testing.T) {
	in := []hit{{"B", ""}, {"A", ""}}
	Rank(in, media.Item{Title: "A"}, func(h hit) string { return h.title }, func(h hit) string { return h.creator })
	if in[0].title != "B" {
		t.Error("Rank mutated its input")
	}
}
