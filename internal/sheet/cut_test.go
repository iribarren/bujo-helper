package sheet

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestCutModeByID(t *testing.T) {
	if DefaultCut != "border" || CutModes[0].ID != DefaultCut {
		t.Errorf("DefaultCut = %q, first cut mode %q; want border first", DefaultCut, CutModes[0].ID)
	}
	tests := []struct {
		id, want, label string
	}{
		{"", "border", "Border"},
		{"border", "border", "Border"},
		{"bleed", "bleed", "Bleed + crop marks"},
		{"none", "none", "None"},
	}
	for _, tt := range tests {
		c, err := CutModeByID(tt.id)
		if err != nil || c.ID != tt.want || c.Label != tt.label {
			t.Errorf("CutModeByID(%q) = %+v, %v; want %s (%q)", tt.id, c, err, tt.want, tt.label)
		}
	}
	if len(CutModes) != len(tests)-1 {
		t.Errorf("%d cut modes, want %d", len(CutModes), len(tests)-1)
	}
	if _, err := CutModeByID("kiss"); err == nil || !strings.Contains(err.Error(), "kiss") {
		t.Errorf(`CutModeByID("kiss"): error %v, want one naming the cut mode`, err)
	}
}

func TestCutModesHaveFileSafeTags(t *testing.T) {
	const safe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	for _, c := range CutModes {
		if c.FileTag == "" || strings.Trim(c.FileTag, safe) != "" {
			t.Errorf("cut mode %q: file tag %q, want letters and digits only", c.ID, c.FileTag)
		}
	}
}

// The cut mode never changes the grid, so page counts stay the same.
func TestCutModeKeepsTheGrid(t *testing.T) {
	for _, f := range Formats {
		for _, p := range Papers {
			for _, s := range Sizes {
				base := Options{Format: f.ID, Paper: p.ID, Size: s.ID}
				want, err := base.Grid()
				if err != nil {
					t.Fatal(err)
				}
				for _, c := range CutModes {
					opts := base
					opts.Cut = c.ID
					got, err := opts.Grid()
					if err != nil || got != want {
						t.Errorf("%s %s %s, cut %s: grid %+v, %v; want %+v", f.ID, p.ID, s.ID, c.ID, got, err, want)
					}
				}
			}
		}
	}
}

func TestOptionsRejectUnknownCutMode(t *testing.T) {
	for _, f := range Formats {
		if _, err := (Options{Format: f.ID, Cut: "kiss"}).Grid(); err == nil || !strings.Contains(err.Error(), "kiss") {
			t.Errorf(`%s: Grid() with cut "kiss": error %v, want one naming the cut mode`, f.ID, err)
		}
	}
}

func TestBleedBox(t *testing.T) {
	got := bleedBox(Box{X: 10, Y: 20, W: 30, H: 40})
	want := Box{X: 10 - Bleed, Y: 20 - Bleed, W: 30 + 2*Bleed, H: 40 + 2*Bleed}
	if got != want {
		t.Errorf("bleedBox = %+v, want %+v", got, want)
	}
}

// interior reports whether point (x, y) lies strictly inside b.
func interior(b Box, x, y float64) bool {
	return x > b.X+eps && x < b.X+b.W-eps && y > b.Y+eps && y < b.Y+b.H-eps
}

// within reports whether point (x, y) lies in b or on its edge.
func within(b Box, x, y float64) bool {
	return x >= b.X-eps && x <= b.X+b.W+eps && y >= b.Y-eps && y <= b.Y+b.H+eps
}

// grow returns b grown by d on every side.
func grow(b Box, d float64) Box {
	return Box{X: b.X - d, Y: b.Y - d, W: b.W + 2*d, H: b.H + 2*d}
}

// crosses reports whether segment l enters the interior of b. Crop marks
// are axis-aligned, so sampling along the segment is exact enough.
func crosses(l Line, b Box) bool {
	const steps = 64
	for i := 0; i <= steps; i++ {
		f := float64(i) / steps
		if interior(b, l.X1+(l.X2-l.X1)*f, l.Y1+(l.Y2-l.Y1)*f) {
			return true
		}
	}
	return false
}

func TestCropMarksOfOneBox(t *testing.T) {
	b := Box{X: 10, Y: 20, W: 30, H: 40}
	marks := cropMarks(b)
	if len(marks) != 8 {
		t.Fatalf("%d crop marks, want 8", len(marks))
	}
	for _, m := range marks {
		horizontal, vertical := near(m.Y1, m.Y2), near(m.X1, m.X2)
		if horizontal == vertical {
			t.Errorf("mark %+v is not horizontal or vertical", m)
			continue
		}
		// Each mark continues a cut edge: it lies on the line of one.
		if horizontal && !near(m.Y1, b.Y) && !near(m.Y1, b.Y+b.H) {
			t.Errorf("horizontal mark %+v is not on a cut edge", m)
		}
		if vertical && !near(m.X1, b.X) && !near(m.X1, b.X+b.W) {
			t.Errorf("vertical mark %+v is not on a cut edge", m)
		}
		if l := math.Hypot(m.X2-m.X1, m.Y2-m.Y1); !near(l, Gap/2-Bleed) {
			t.Errorf("mark %+v is %v mm long, want %v", m, l, Gap/2-Bleed)
		}
	}
}

// For every paper and size, with typical, shrink-to-fit and mixed covers:
// bleed boxes never overlap, and every crop mark stays outside every bleed
// box, on the page, and within its own cover's half of the gap.
func TestBleedGeometry(t *testing.T) {
	shapes := map[string][]float64{
		"typical": {0.656},
		"square":  {1.0},
		"wide":    {1.5},
		"mixed":   {2.0 / 3, 1, 0.5, 1.5, 0.656, 2.5, 0.3},
	}
	for _, p := range Papers {
		for _, s := range Sizes {
			g, err := Options{Paper: p.ID, Size: s.ID}.Grid()
			if err != nil {
				t.Fatal(err)
			}
			for shape, rs := range shapes {
				name := fmt.Sprintf("%s %s %s", p.ID, s.ID, shape)
				t.Run(name, func(t *testing.T) {
					ratios := make([]float64, g.PerPage())
					for i := range ratios {
						ratios[i] = rs[i%len(rs)]
					}
					pages, err := g.Flow(ratios)
					if err != nil {
						t.Fatal(err)
					}
					checkBleedPage(t, Box{W: g.Page.Width, H: g.Page.Height}, pages[0])
				})
			}
		}
	}
}

func checkBleedPage(t *testing.T, page Box, boxes []Box) {
	t.Helper()
	bleeds := make([]Box, len(boxes))
	for i, b := range boxes {
		bleeds[i] = bleedBox(b)
		if !within(page, bleeds[i].X, bleeds[i].Y) || !within(page, bleeds[i].X+bleeds[i].W, bleeds[i].Y+bleeds[i].H) {
			t.Errorf("bleed box %d %+v is off the page", i, bleeds[i])
		}
	}
	for i := range bleeds {
		for j := i + 1; j < len(bleeds); j++ {
			a, b := bleeds[i], bleeds[j]
			if a.X < b.X+b.W-eps && b.X < a.X+a.W-eps && a.Y < b.Y+b.H-eps && b.Y < a.Y+a.H-eps {
				t.Errorf("bleed boxes %d %+v and %d %+v overlap", i, a, j, b)
			}
		}
	}
	for i, b := range boxes {
		half := grow(b, Gap/2)
		for _, m := range cropMarks(b) {
			for _, end := range [][2]float64{{m.X1, m.Y1}, {m.X2, m.Y2}} {
				if !within(page, end[0], end[1]) {
					t.Errorf("cover %d: mark %+v is off the page", i, m)
				}
				if !within(half, end[0], end[1]) {
					t.Errorf("cover %d: mark %+v leaves its half of the gap", i, m)
				}
			}
			for j, bl := range bleeds {
				if crosses(m, bl) {
					t.Errorf("cover %d: mark %+v enters bleed box %d %+v", i, m, j, bl)
				}
			}
			for j, other := range boxes {
				if j != i && crosses(m, grow(other, Gap/2)) {
					t.Errorf("cover %d: mark %+v enters cover %d's half of the gap", i, m, j)
				}
			}
		}
	}
}

func TestBleedPad(t *testing.T) {
	tests := []struct {
		px     int
		mm     float64
		want   int
		reason string
	}{
		{591, 50, 12, "300 dpi: 11.82 px/mm"},
		{100, 50, 2, "2 px/mm, exact"},
		{59, 50, 2, "1.18 px/mm rounds up"},
		{50, 50, 1, "1 px/mm, exact"},
		{10, 50, 1, "under 1 px/mm still pads one pixel"},
		{295, 25, 12, "mini cover at 300 dpi"},
	}
	for _, tt := range tests {
		if got := bleedPad(tt.px, tt.mm); got != tt.want {
			t.Errorf("bleedPad(%d, %v) = %d, want %d (%s)", tt.px, tt.mm, got, tt.want, tt.reason)
		}
	}
}

func TestExtendEdges(t *testing.T) {
	const w, h, pad = 3, 2, 2
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			src.Set(x, y, color.RGBA{uint8(10 + 40*x), uint8(10 + 100*y), 7, 255})
		}
	}
	got := extendEdges(src, pad)
	if b := got.Bounds(); b != image.Rect(0, 0, w+2*pad, h+2*pad) {
		t.Fatalf("bounds %v, want %dx%d at 0,0", b, w+2*pad, h+2*pad)
	}
	clamp := func(v, hi int) int { return max(0, min(v, hi-1)) }
	for y := range h + 2*pad {
		for x := range w + 2*pad {
			want := src.RGBAAt(clamp(x-pad, w), clamp(y-pad, h))
			if c := got.RGBAAt(x, y); c != want {
				t.Errorf("pixel %d,%d = %v, want %v", x, y, c, want)
			}
		}
	}
	if same := extendEdges(src, 0); same.Bounds() != src.Bounds() || same.RGBAAt(2, 1) != src.RGBAAt(2, 1) {
		t.Error("extendEdges with no pad changed the image")
	}
}

// The padded image is drawn so its unpadded pixels land exactly on the cut
// box, and it covers the whole bleed box.
func TestPaddedBox(t *testing.T) {
	tests := []struct {
		box       Box
		w, h, pad int
	}{
		{Box{X: 20, Y: 30, W: 32.8, H: 50}, 388, 591, bleedPad(591, 50)},
		{Box{X: 8, Y: 15.5, W: 35.6, H: 23.73}, 420, 280, bleedPad(280, 23.73)},
		{Box{X: 10, Y: 10, W: 33, H: 50}, 66, 100, bleedPad(100, 50)},
		{Box{X: 10, Y: 10, W: 50, H: 50}, 10, 10, bleedPad(10, 50)},
	}
	for _, tt := range tests {
		got := paddedBox(tt.box, tt.w, tt.h, tt.pad)
		// Pixel pad .. pad+w maps onto the cut box.
		pxW, pxH := got.W/float64(tt.w+2*tt.pad), got.H/float64(tt.h+2*tt.pad)
		inner := Box{X: got.X + float64(tt.pad)*pxW, Y: got.Y + float64(tt.pad)*pxH, W: float64(tt.w) * pxW, H: float64(tt.h) * pxH}
		if !near(inner.X, tt.box.X) || !near(inner.Y, tt.box.Y) || !near(inner.W, tt.box.W) || !near(inner.H, tt.box.H) {
			t.Errorf("paddedBox(%+v, %d, %d, %d): inner pixels on %+v, want %+v", tt.box, tt.w, tt.h, tt.pad, inner, tt.box)
		}
		bl := bleedBox(tt.box)
		if got.X > bl.X+eps || got.Y > bl.Y+eps || got.X+got.W < bl.X+bl.W-eps || got.Y+got.H < bl.Y+bl.H-eps {
			t.Errorf("paddedBox(%+v, ...) = %+v does not cover the bleed box %+v", tt.box, got, bl)
		}
	}
}
