package mapdata

import (
	"bytes"
	"math"
	"reflect"
	"strings"
	"testing"
)

// fixture is a small hand-made dataset: one country with an outer ring and a
// hole, one with two islands and empty optional fields.
func fixture() []Country {
	return []Country{
		{
			ISO2: "FR", ISO3: "FRA", Continent: "Europe",
			NameEN: "France", NameES: "Francia",
			Names: []string{"France", "French Republic", "Francia"},
			Rings: [][]LonLat{
				{{-4.5, 48.4}, {2.5, 51.1}, {8.2, 49}, {7.6, 43.8}, {-1.8, 43.4}, {-4.5, 48.4}},
				{{1, 46}, {2, 47}, {3, 46}, {1, 46}},
			},
		},
		{
			NameEN: "Somaliland", NameES: "Somalilandia", Continent: "Africa",
			Names: []string{"Somaliland"},
			Rings: [][]LonLat{
				{{-179.99, -16.5}, {-179.5, -16.1}, {-179.8, -16.9}},
				{{179.99, -16.5}, {179.5, -16.1}, {179.8, -16.9}, {179.99, -16.5}},
			},
		},
	}
}

func TestRoundTrip(t *testing.T) {
	in := fixture()
	data, err := Encode(in, 1000)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("got %d countries, want %d", len(out), len(in))
	}
	for i := range in {
		a, b := in[i], out[i]
		if a.ISO2 != b.ISO2 || a.ISO3 != b.ISO3 || a.Continent != b.Continent ||
			a.NameEN != b.NameEN || a.NameES != b.NameES || !reflect.DeepEqual(a.Names, b.Names) {
			t.Errorf("country %d attributes: got %+v, want %+v", i, b, a)
		}
		if len(b.Rings) != len(a.Rings) {
			t.Fatalf("country %d: got %d rings, want %d", i, len(b.Rings), len(a.Rings))
		}
		for r, ring := range b.Rings {
			want := closed(a.Rings[r])
			if len(ring) != len(want) {
				t.Fatalf("country %d ring %d: got %d points, want %d (closed)", i, r, len(ring), len(want))
			}
			for p := range ring {
				if math.Abs(ring[p].Lon-want[p].Lon) > 1e-9 || math.Abs(ring[p].Lat-want[p].Lat) > 1e-9 {
					t.Errorf("country %d ring %d point %d: got %v, want %v", i, r, p, ring[p], want[p])
				}
			}
		}
	}
}

func TestEncodeDeterministic(t *testing.T) {
	a, err := Encode(fixture(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encode(fixture(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("same input encoded to different bytes")
	}
}

func TestEncodeQuantizes(t *testing.T) {
	in := []Country{{NameEN: "X", Names: []string{"X"}, Rings: [][]LonLat{
		{{0.123456, 0.987654}, {1.0004, 0}, {0, 1.0006}},
	}}}
	data, err := Encode(in, 100)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []LonLat{{0.12, 0.99}, {1, 0}, {0, 1}, {0.12, 0.99}}
	got := out[0].Rings[0]
	for i := range want {
		if math.Abs(got[i].Lon-want[i].Lon) > 1e-9 || math.Abs(got[i].Lat-want[i].Lat) > 1e-9 {
			t.Errorf("point %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestEncodeErrors(t *testing.T) {
	tests := []struct {
		name  string
		in    []Country
		scale int
		want  string
	}{
		{"degenerate ring", []Country{{NameEN: "X", Rings: [][]LonLat{{{0, 0}, {1, 1}, {0, 0}}}}}, 100, "fewer than 3"},
		{"zero scale", fixture(), 0, "scale"},
		{"out of range", []Country{{NameEN: "X", Rings: [][]LonLat{{{0, 0}, {181, 1}, {0, 1}}}}}, 100, "out of range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encode(tt.in, tt.scale)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got error %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestDecodeRejectsBadInput(t *testing.T) {
	good, err := Encode(fixture(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"not gzip", []byte("hello world")},
		{"truncated", good[:len(good)/2]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(tt.data); err == nil {
				t.Error("Decode accepted bad input")
			}
		})
	}
}

func closed(ring []LonLat) []LonLat {
	if ring[0] == ring[len(ring)-1] {
		return ring
	}
	return append(append([]LonLat(nil), ring...), ring[0])
}
