// Package mapdata is the compact binary format of the embedded country map
// data used by the Visited map page kind. The generator (../mapgen) encodes
// Natural Earth countries with Encode; the sheet package decodes the
// embedded file with Decode.
//
// Format (the whole stream is gzip-compressed):
//
//	magic "BJMAP" + version byte 1
//	uvarint scale            coordinates are integers in units of 1/scale degree
//	uvarint country count
//	per country:
//	  string ISO2, ISO3, Continent, NameEN, NameES   (uvarint length + UTF-8)
//	  uvarint name count, then each name as a string
//	  uvarint ring count
//	  per ring: uvarint point count, then per point the zigzag varint delta of
//	  lon and lat from the previous point (the first point of each ring is a
//	  delta from 0,0). Rings are stored open: the closing point is implied.
//
// Encoding is deterministic: the same countries and scale give the same bytes.
package mapdata

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

var magic = []byte("BJMAP\x01")

// Sanity limits for decoding; the real data is far below them.
const (
	maxCountries = 10000
	maxNames     = 64
	maxRings     = 100000
	maxPoints    = 1000000
	maxString    = 1024
)

// LonLat is a geographic point in degrees.
type LonLat struct {
	Lon, Lat float64
}

// Country is one admin-0 country.
type Country struct {
	ISO2, ISO3 string // ISO 3166-1 alpha-2 and alpha-3; empty when NE has none
	Continent  string // Natural Earth CONTINENT
	NameEN     string // English display name
	NameES     string // Spanish display name
	Names      []string
	// Rings holds the outer rings and holes of all polygons of the country.
	// Decode returns closed rings (the last point equals the first).
	Rings [][]LonLat
}

// Encode writes countries in the gzip-compressed binary format, quantizing
// coordinates to 1/scale degree. Rings may be given open or closed; each
// needs at least 3 distinct points after quantization.
func Encode(countries []Country, scale int) ([]byte, error) {
	if scale <= 0 || scale > 1_000_000 {
		return nil, fmt.Errorf("mapdata: invalid scale %d", scale)
	}
	var raw bytes.Buffer
	raw.Write(magic)
	putUvarint(&raw, uint64(scale))
	putUvarint(&raw, uint64(len(countries)))
	for _, c := range countries {
		if len(c.Names) > maxNames || len(c.Rings) > maxRings {
			return nil, fmt.Errorf("mapdata: %s: too many names or rings", c.NameEN)
		}
		for _, s := range append([]string{c.ISO2, c.ISO3, c.Continent, c.NameEN, c.NameES}, c.Names...) {
			if len(s) > maxString {
				return nil, fmt.Errorf("mapdata: %s: string over %d bytes", c.NameEN, maxString)
			}
		}
		for _, s := range []string{c.ISO2, c.ISO3, c.Continent, c.NameEN, c.NameES} {
			putString(&raw, s)
		}
		putUvarint(&raw, uint64(len(c.Names)))
		for _, n := range c.Names {
			putString(&raw, n)
		}
		putUvarint(&raw, uint64(len(c.Rings)))
		for r, ring := range c.Rings {
			q, err := quantize(ring, scale)
			if err != nil {
				return nil, fmt.Errorf("mapdata: %s ring %d: %w", c.NameEN, r, err)
			}
			putUvarint(&raw, uint64(len(q)))
			var px, py int64
			for _, p := range q {
				putVarint(&raw, p[0]-px)
				putVarint(&raw, p[1]-py)
				px, py = p[0], p[1]
			}
		}
	}

	var out bytes.Buffer
	zw, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw.Bytes()); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// quantize rounds a ring to integer units, drops consecutive duplicates and
// the closing point, and checks it still has 3 points.
func quantize(ring []LonLat, scale int) ([][2]int64, error) {
	s := float64(scale)
	q := make([][2]int64, 0, len(ring))
	for _, p := range ring {
		if math.IsNaN(p.Lon) || math.IsNaN(p.Lat) || p.Lon < -180 || p.Lon > 180 || p.Lat < -90 || p.Lat > 90 {
			return nil, fmt.Errorf("point %v out of range", p)
		}
		v := [2]int64{int64(math.Round(p.Lon * s)), int64(math.Round(p.Lat * s))}
		if len(q) > 0 && q[len(q)-1] == v {
			continue
		}
		q = append(q, v)
	}
	if len(q) > 1 && q[0] == q[len(q)-1] {
		q = q[:len(q)-1]
	}
	if len(q) < 3 {
		return nil, errors.New("fewer than 3 distinct points")
	}
	if len(q) > maxPoints {
		return nil, fmt.Errorf("over %d points", maxPoints)
	}
	return q, nil
}

// Decode reads data written by Encode.
func Decode(data []byte) ([]Country, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("mapdata: %w", err)
	}
	d := &decoder{r: bufio.NewReader(zr)}
	countries := d.countries()
	if d.err == nil {
		// Reading to EOF makes gzip verify its checksum.
		if _, err := io.Copy(io.Discard, d.r); err != nil {
			d.err = err
		}
	}
	if d.err != nil {
		return nil, fmt.Errorf("mapdata: %w", d.err)
	}
	return countries, nil
}

type decoder struct {
	r   *bufio.Reader
	err error
}

func (d *decoder) countries() []Country {
	head := make([]byte, len(magic))
	if _, err := io.ReadFull(d.r, head); err != nil {
		d.err = err
		return nil
	}
	if !bytes.Equal(head, magic) {
		d.err = errors.New("bad magic")
		return nil
	}
	scale := float64(d.count(1_000_000))
	n := d.count(maxCountries)
	if d.err == nil && scale == 0 {
		d.err = errors.New("zero scale")
	}
	var out []Country
	for i := 0; i < n && d.err == nil; i++ {
		var c Country
		c.ISO2, c.ISO3, c.Continent, c.NameEN, c.NameES = d.str(), d.str(), d.str(), d.str(), d.str()
		names := d.count(maxNames)
		for j := 0; j < names && d.err == nil; j++ {
			c.Names = append(c.Names, d.str())
		}
		rings := d.count(maxRings)
		for j := 0; j < rings && d.err == nil; j++ {
			c.Rings = append(c.Rings, d.ring(scale))
		}
		out = append(out, c)
	}
	return out
}

func (d *decoder) ring(scale float64) []LonLat {
	n := d.count(maxPoints)
	if d.err == nil && n < 3 {
		d.err = fmt.Errorf("ring with %d points", n)
	}
	ring := make([]LonLat, 0, n+1)
	var x, y int64
	for i := 0; i < n && d.err == nil; i++ {
		x += d.varint()
		y += d.varint()
		ring = append(ring, LonLat{float64(x) / scale, float64(y) / scale})
	}
	if d.err != nil {
		return nil
	}
	return append(ring, ring[0])
}

func (d *decoder) count(limit int) int {
	if d.err != nil {
		return 0
	}
	v, err := binary.ReadUvarint(d.r)
	if err != nil {
		d.err = err
		return 0
	}
	if v > uint64(limit) {
		d.err = fmt.Errorf("count %d over limit %d", v, limit)
		return 0
	}
	return int(v)
}

func (d *decoder) varint() int64 {
	if d.err != nil {
		return 0
	}
	v, err := binary.ReadVarint(d.r)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *decoder) str() string {
	n := d.count(maxString)
	if d.err != nil {
		return ""
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(d.r, b); err != nil {
		d.err = err
		return ""
	}
	return string(b)
}

func putUvarint(w *bytes.Buffer, v uint64) {
	w.Write(binary.AppendUvarint(nil, v))
}

func putVarint(w *bytes.Buffer, v int64) {
	w.Write(binary.AppendVarint(nil, v))
}

func putString(w *bytes.Buffer, s string) {
	putUvarint(w, uint64(len(s)))
	w.WriteString(s)
}
