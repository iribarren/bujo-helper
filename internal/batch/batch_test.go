package batch

import (
	"reflect"
	"strings"
	"testing"

	"bujo-helper/internal/media"
)

func TestParseLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []media.Item
	}{
		{"empty", "", nil},
		{"title and author", "Dune, Frank Herbert", []media.Item{{Title: "Dune", Creator: "Frank Herbert"}}},
		{"no comma", "Dune", []media.Item{{Title: "Dune"}}},
		{"split on last comma", "Guns, Germs, and Steel, Jared Diamond", []media.Item{{Title: "Guns, Germs, and Steel", Creator: "Jared Diamond"}}},
		{"trailing comma", "Guns, Germs, and Steel,", []media.Item{{Title: "Guns, Germs, and Steel"}}},
		{
			"skips blanks and comments, handles CRLF",
			"# my list\r\n\r\nDune, Frank Herbert\r\n   \r\n  Emma ,  Jane Austen  \r\n#Ignored, x\n",
			[]media.Item{{Title: "Dune", Creator: "Frank Herbert"}, {Title: "Emma", Creator: "Jane Austen"}},
		},
		{"empty title skipped", ", Nobody", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLines(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLines(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatLinesRoundTrip(t *testing.T) {
	items := []media.Item{
		{Title: "Dune", Creator: "Frank Herbert"},
		{Title: "Emma"},
		{Title: "Guns, Germs, and Steel"},
		{Title: "Guns, Germs, and Steel", Creator: "Jared Diamond"},
	}
	text := FormatLines(items)
	want := "Dune, Frank Herbert\nEmma\nGuns, Germs, and Steel,\nGuns, Germs, and Steel, Jared Diamond\n"
	if text != want {
		t.Errorf("FormatLines = %q, want %q", text, want)
	}
	if got := ParseLines(text); !reflect.DeepEqual(got, items) {
		t.Errorf("round trip = %+v, want %+v", got, items)
	}
	if FormatLines(nil) != "" {
		t.Error("FormatLines(nil) should be empty")
	}
}

func TestReadCSV(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []media.Item
	}{
		{
			"comma with header",
			[]byte("Title,Author\nDune,Frank Herbert\n\"Guns, Germs, and Steel\",Jared Diamond\n"),
			[]media.Item{{Title: "Dune", Creator: "Frank Herbert"}, {Title: "Guns, Germs, and Steel", Creator: "Jared Diamond"}},
		},
		{
			"semicolon with BOM and Spanish header",
			[]byte("\xef\xbb\xbfTítulo;Autor\r\nCien años de soledad;Gabriel García Márquez\r\n"),
			[]media.Item{{Title: "Cien años de soledad", Creator: "Gabriel García Márquez"}},
		},
		{
			"windows-1252 bytes",
			[]byte("libro;autor\nCien a\xf1os de soledad;Gabriel Garc\xeda M\xe1rquez\n"),
			[]media.Item{{Title: "Cien años de soledad", Creator: "Gabriel García Márquez"}},
		},
		{
			"no header, single column, empty rows, extra columns",
			[]byte("\n\nDune\n,\nEmma,Jane Austen,1815,extra\n"),
			[]media.Item{{Title: "Dune"}, {Title: "Emma", Creator: "Jane Austen"}},
		},
		{
			"leading blank line before delimiter detection",
			[]byte("\n  \nName;Creator\nA, B;C\n"),
			[]media.Item{{Title: "A, B", Creator: "C"}},
		},
		{
			"lazy quotes and ragged rows",
			[]byte("He said \"hi\",Someone\nOnly title\n"),
			[]media.Item{{Title: "He said \"hi\"", Creator: "Someone"}, {Title: "Only title"}},
		},
		{"empty input", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadCSV(strings.NewReader(string(tt.in)))
			if err != nil {
				t.Fatalf("ReadCSV: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadCSV = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFormatLinesCreatorWithCommaSurvivesParse(t *testing.T) {
	cases := []struct {
		in   media.Item
		want media.Item
	}{
		// "Last, First" (common in catalog CSVs) becomes "First Last".
		{media.Item{Title: "Dune", Creator: "Herbert, Frank"}, media.Item{Title: "Dune", Creator: "Frank Herbert"}},
		{media.Item{Title: "Guns, Germs, and Steel", Creator: "Diamond, Jared"}, media.Item{Title: "Guns, Germs, and Steel", Creator: "Jared Diamond"}},
		// Several commas (co-authors): commas are dropped so the title stays intact.
		{media.Item{Title: "Good Omens", Creator: "Pratchett, Terry, Gaiman"}, media.Item{Title: "Good Omens", Creator: "Pratchett Terry Gaiman"}},
	}
	for _, c := range cases {
		got := ParseLines(FormatLines([]media.Item{c.in}))
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("round trip of %+v = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestWriteCSVRoundTrip(t *testing.T) {
	items := []media.Item{
		{Title: "Dune", Creator: "Frank Herbert"},
		{Title: "Guns, Germs, and Steel", Creator: "Jared Diamond"},
		{Title: `The "Quoted" One`},
	}
	var b strings.Builder
	if err := WriteCSV(&b, []string{"title", "author"}, items); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	if !strings.HasPrefix(b.String(), "title,author\n") {
		t.Errorf("WriteCSV output %q does not start with the header", b.String())
	}
	got, err := ReadCSV(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if !reflect.DeepEqual(got, items) {
		t.Errorf("round trip = %+v, want %+v", got, items)
	}
}
