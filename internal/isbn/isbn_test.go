package isbn

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"978-84-663-5377-9", "9788466353779", true},
		{"84 663 5377 1", "8466353771", true},
		{"950644174x", "950644174X", true},
		{"9781101658055", "9781101658055", true},
		{"9788466353778", "", false}, // bad checksum
		{"8466353772", "", false},    // bad checksum
		{"B00B7NPRY8", "", false},    // Kindle ASIN
		{"12345", "", false},
		{"", "", false},
		{"97884663537790", "", false},
	}
	for _, tt := range tests {
		got, ok := Normalize(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("Normalize(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestTo13(t *testing.T) {
	tests := map[string]string{
		"8466353771":        "9788466353779",
		"0441013597":        "9780441013593",
		"950644174X":        "9789506441746",
		"978-84-663-5377-9": "9788466353779",
		"8466353772":        "",
		"B00B7NPRY8":        "",
	}
	for in, want := range tests {
		if got := To13(in); got != want {
			t.Errorf("To13(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTo10(t *testing.T) {
	tests := map[string]string{
		"9788466353779":     "8466353771",
		"978-84-663-5377-9": "8466353771",
		"9789506441746":     "950644174X",
		"8466353771":        "8466353771",
		"9798000000000":     "", // 979 has no ISBN-10 form (and this checksum is wrong)
		"9791032305690":     "", // valid 979 ISBN-13, still no ISBN-10
		"9788466353778":     "",
	}
	for in, want := range tests {
		if got := To10(in); got != want {
			t.Errorf("To10(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFind(t *testing.T) {
	tests := map[string]string{
		"https://is1-ssl.mzstatic.com/image/thumb/Publication126/v4/aa/bb/9788466342667.jpg/100x100bb.jpg":                                           "9788466342667",
		"https://is1-ssl.mzstatic.com/image/thumb/Publication122/v4/85/70/2f/85702f34-2982-e2ee-d883-b5ffc3ae0897/9781101658055.d.jpg/100x100bb.jpg": "9781101658055",
		"https://example.com/12345/9788466342668.jpg":             "", // bad checksum
		"https://example.com/19788466342667.jpg":                  "", // embedded in a longer number
		"https://example.com/a/9788466342667/b/9781101658055.jpg": "9788466342667",
		"no isbn here": "",
	}
	for in, want := range tests {
		if got := Find(in); got != want {
			t.Errorf("Find(%q) = %q, want %q", in, got, want)
		}
	}
}
