package mapdata

import (
	"bytes"
	"compress/gzip"
	"io"
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

// raw builds an uncompressed payload for the decoder tests.
type raw struct{ bytes.Buffer }

func (b *raw) uv(v uint64) *raw { putUvarint(&b.Buffer, v); return b }
func (b *raw) v(v int64) *raw   { putVarint(&b.Buffer, v); return b }
func (b *raw) s(s string) *raw  { putString(&b.Buffer, s); return b }
func (b *raw) add(p ...byte) *raw {
	b.Write(p)
	return b
}

// head starts a payload: magic, scale 1000 and n countries.
func head(n uint64) *raw {
	b := &raw{}
	b.add(magic...)
	return b.uv(1000).uv(n)
}

// attrs writes a country's five attributes and its name count.
func (b *raw) attrs(names uint64) *raw {
	return b.s("XX").s("XXX").s("Europe").s("X").s("X").uv(names)
}

// triangle writes one ring of three points.
func (b *raw) triangle() *raw {
	return b.uv(3).v(0).v(0).v(1000).v(0).v(0).v(1000)
}

// overflow is a uvarint/varint longer than 64 bits.
var overflow = bytes.Repeat([]byte{0xff}, 11)

func gz(t *testing.T, p []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	if _, err := zw.Write(p); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func gunzip(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	p, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Every malformed input is an error, never a panic or a partial result.
func TestDecodeRejectsBadInput(t *testing.T) {
	good, err := Encode(fixture(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	payload := gunzip(t, good)
	badCRC := append([]byte(nil), good...)
	badCRC[len(badCRC)-8] ^= 0xff // gzip trailer: CRC-32, then size
	valid := head(1).attrs(0).uv(1).triangle().Bytes()

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"not gzip", []byte("hello world")},
		{"truncated gzip", good[:len(good)/2]},
		{"bad gzip checksum", badCRC},
		{"truncated payload", gz(t, payload[:len(payload)/2])},
		{"payload cut in a string", gz(t, head(1).uv(5).add('X', 'X').Bytes())},
		{"bad magic", gz(t, append([]byte("BJMAP\x02"), payload[len(magic):]...))},
		{"short magic", gz(t, []byte("BJM"))},
		{"zero scale", gz(t, (&raw{}).add(magic...).uv(0).uv(0).Bytes())},
		{"scale over limit", gz(t, (&raw{}).add(magic...).uv(1_000_001).uv(0).Bytes())},
		{"corrupt scale varint", gz(t, (&raw{}).add(magic...).add(overflow...).Bytes())},
		{"too many countries", gz(t, head(maxCountries+1).Bytes())},
		{"string over limit", gz(t, head(1).uv(maxString+1).Bytes())},
		{"too many names", gz(t, head(1).attrs(maxNames+1).Bytes())},
		{"too many rings", gz(t, head(1).attrs(0).uv(maxRings+1).Bytes())},
		{"too many points", gz(t, head(1).attrs(0).uv(1).uv(maxPoints+1).Bytes())},
		{"ring of two points", gz(t, head(1).attrs(0).uv(1).uv(2).v(0).v(0).v(1).v(1).Bytes())},
		{"corrupt point varint", gz(t, head(1).attrs(0).uv(1).uv(3).add(overflow...).Bytes())},
		{"missing country", gz(t, head(2).attrs(0).uv(1).triangle().Bytes())},
		{"trailing payload bytes", gz(t, append(append([]byte(nil), valid...), 0))},
		{"second gzip member", append(gz(t, valid), gz(t, []byte{0})...)},
		{"garbage after gzip", append(gz(t, valid), "junk"...)},
	}
	if _, err := Decode(gz(t, valid)); err != nil {
		t.Fatalf("the valid base payload failed: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Decode panicked: %v", r)
				}
			}()
			got, err := Decode(tt.data)
			if err == nil {
				t.Error("Decode accepted bad input")
			}
			if got != nil {
				t.Errorf("Decode returned %d countries with its error", len(got))
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
