package sheet

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The registry lists each page kind once, with a label, a file tag and
// defaults that decode back to themselves.
func TestPageKindsRegistry(t *testing.T) {
	if len(PageKinds) == 0 {
		t.Fatal("no page kinds")
	}
	seen := map[string]bool{}
	for _, k := range PageKinds {
		if k.ID == "" || k.Label == "" || k.FileTag == "" {
			t.Errorf("kind %+v: want an ID, a label and a file tag", k)
		}
		if seen[k.ID] {
			t.Errorf("kind %q listed twice", k.ID)
		}
		seen[k.ID] = true
		got, err := PageKindByID(k.ID)
		if err != nil || got.ID != k.ID {
			t.Errorf("PageKindByID(%q) = %+v, %v", k.ID, got, err)
		}
		raw, err := json.Marshal(k.Defaults)
		if err != nil {
			t.Fatalf("%s: marshal defaults: %v", k.ID, err)
		}
		if _, err := k.decode(raw); err != nil {
			t.Errorf("%s: defaults %s do not decode: %v", k.ID, raw, err)
		}
	}
	if !seen[DotGridKind] {
		t.Errorf("no %q kind", DotGridKind)
	}
}

func TestPageKindByIDRejectsUnknownKinds(t *testing.T) {
	for _, id := range []string{"", "starmap", "DotGrid"} {
		if _, err := PageKindByID(id); err == nil || !strings.Contains(err.Error(), `"`+id+`"`) {
			t.Errorf("PageKindByID(%q): error %v, want one naming the kind", id, err)
		}
	}
}

// Options are decoded from JSON over the kind's defaults; unknown fields,
// wrong types, trailing data and values out of range are rejected.
func TestDecodeDotGridOptions(t *testing.T) {
	kind, err := PageKindByID(DotGridKind)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		raw     string
		want    DotGridOptions
		wantErr string // part of the error; "" for none
	}{
		{"no options take the defaults", "", DotGridOptions{Spacing: 5}, ""},
		{"null takes the defaults", "null", DotGridOptions{Spacing: 5}, ""},
		{"empty object takes the defaults", "{}", DotGridOptions{Spacing: 5}, ""},
		{"4 mm", `{"spacing":4}`, DotGridOptions{Spacing: 4}, ""},
		{"6 mm", `{"spacing": 6}`, DotGridOptions{Spacing: 6}, ""},
		{"spacing too small", `{"spacing":3}`, DotGridOptions{}, "3"},
		{"spacing too large", `{"spacing":7}`, DotGridOptions{}, "7"},
		{"zero spacing", `{"spacing":0}`, DotGridOptions{}, "0"},
		{"fractional spacing", `{"spacing":4.5}`, DotGridOptions{}, "spacing"},
		{"spacing as a string", `{"spacing":"5"}`, DotGridOptions{}, "spacing"},
		{"unknown field", `{"spacing":5,"colour":"red"}`, DotGridOptions{}, `"colour"`},
		{"not an object", `[5]`, DotGridOptions{}, "options"},
		{"trailing data", `{"spacing":5} {}`, DotGridOptions{}, "options"},
		{"malformed", `{"spacing":`, DotGridOptions{}, "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := kind.decode(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := doc.(DotGridOptions); !ok || got != tt.want {
				t.Errorf("decoded %#v, want %#v", doc, tt.want)
			}
		})
	}
}

func TestPageCount(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		page    PageSettings
		options string
		want    int
		wantErr string
	}{
		{"default dot grid", DotGridKind, PageSettings{}, "", 1, ""},
		{"dot grid with a title on B5", DotGridKind, PageSettings{Paper: "b5", Title: "Notes"}, `{"spacing":4}`, 1, ""},
		{"unknown kind", "starmap", PageSettings{}, "", 0, `"starmap"`},
		{"unknown paper", DotGridKind, PageSettings{Paper: "a3"}, "", 0, `"a3"`},
		{"unknown cut", DotGridKind, PageSettings{Cut: "zigzag"}, "", 0, `"zigzag"`},
		{"bad options", DotGridKind, PageSettings{}, `{"spacing":9}`, 0, "9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PageCount(tt.kind, tt.page, json.RawMessage(tt.options))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("PageCount = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
}

// A bad kind, setting or option writes nothing.
func TestBuildPageWritesNothingOnError(t *testing.T) {
	for _, tt := range []struct {
		kind    string
		page    PageSettings
		options string
	}{
		{"starmap", PageSettings{}, ""},
		{DotGridKind, PageSettings{Paper: "a3"}, ""},
		{DotGridKind, PageSettings{}, `{"spacing":5,"x":1}`},
	} {
		var buf bytes.Buffer
		if err := BuildPage(&buf, tt.kind, tt.page, json.RawMessage(tt.options)); err == nil {
			t.Errorf("BuildPage(%q, %+v, %s): want an error", tt.kind, tt.page, tt.options)
		}
		if buf.Len() != 0 {
			t.Errorf("BuildPage(%q, %+v, %s) wrote %d bytes", tt.kind, tt.page, tt.options, buf.Len())
		}
	}
}
