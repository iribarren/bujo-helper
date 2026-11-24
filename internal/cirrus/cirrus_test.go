package cirrus

import "testing"

func TestTerms(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain title", "Heat", "Heat"},
		{"leading minus is not a negation", "-Saw", "Saw"},
		{"leading bang is not a negation", "!Saw", "Saw"},
		{"quotes are not a phrase", `"Heat"`, "Heat"},
		{"keyword prefix", "intitle:Up", "intitle Up"},
		{"colon after a word", "Alien: Romulus", "Alien Romulus"},
		{"apostrophe inside a word is kept", "Who's Afraid of Virginia Woolf?", "Who's Afraid of Virginia Woolf"},
		{"typographic apostrophe inside a word is kept", "Ocean’s Eleven", "Ocean’s Eleven"},
		{"ampersand", "Fast & Furious", "Fast Furious"},
		{"boolean words are lowercased", "NOT Another Teen Movie", "not Another Teen Movie"},
		{"AND and OR are lowercased", "Pride AND Prejudice OR Zombies", "Pride and Prejudice or Zombies"},
		{"lowercase and mixed-case words are kept", "Not Now and Then", "Not Now and Then"},
		{"symbol operators", "Tom && Jerry || Spike", "Tom Jerry Spike"},
		{"accents", "Léon", "Léon"},
		{"combining accent", "Léon", "Léon"},
		{"wildcards and fuzzy", "Al*en? Dune~2", "Al en Dune 2"},
		{"backslash", `AC\DC`, "AC DC"},
		{"regex slashes", "insource:/fo+/", "insource fo"},
		{"hyphen inside a word", "Spider-Man", "Spider Man"},
		{"full stop inside a word is kept", "Mr. Robot 2.0", "Mr Robot 2.0"},
		{"leading and trailing apostrophes", "'Salem's Lot'", "Salem's Lot"},
		{"parentheses and brackets", "Up (2009) [film]", "Up 2009 film"},
		{"spaces collapse", "  The \t Matrix  ", "The Matrix"},
		{"syntax only", `"-*?~:\" && ||`, ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Terms(tt.in); got != tt.want {
				t.Errorf("Terms(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
