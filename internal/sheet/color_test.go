package sheet

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"
)

// solidImage returns a w x h image filled with c.
func solidImage(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// colorDistance is the largest difference between the channels of a and b.
func colorDistance(a, b color.RGBA) int {
	d := func(x, y uint8) int { return int(math.Abs(float64(x) - float64(y))) }
	return max(d(a.R, b.R), d(a.G, b.G), d(a.B, b.B))
}

func TestDominantColorOfSolidImages(t *testing.T) {
	tests := []struct {
		name string
		c    color.RGBA
	}{
		{"red", color.RGBA{220, 20, 30, 255}},
		{"navy", color.RGBA{10, 25, 90, 255}},
		{"white", color.RGBA{255, 255, 255, 255}},
		{"black", color.RGBA{0, 0, 0, 255}},
		{"pastel", color.RGBA{250, 210, 220, 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Large enough to be downscaled, and odd-sized.
			got := dominantColor(solidImage(301, 457, tt.c))
			if d := colorDistance(got, tt.c); d > 2 || got.A != 255 {
				t.Errorf("dominantColor = %v, want %v (off by %d)", got, tt.c, d)
			}
		})
	}
}

func TestDominantColorPicksTheMajority(t *testing.T) {
	majority := color.RGBA{30, 120, 60, 255}
	minority := color.RGBA{240, 200, 30, 255}
	// The top 70 % of the image is green, the bottom 30 % yellow.
	img := solidImage(200, 300, majority)
	for y := 210; y < 300; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, minority)
		}
	}
	if got := dominantColor(img); colorDistance(got, majority) > 2 {
		t.Errorf("dominantColor = %v, want the majority %v", got, majority)
	}
}

// The buckets are fixed, not clustered: a hue whose shades straddle a
// bucket edge is split, and a smaller solid area can beat it. Red levels
// 127 and 128 fall into buckets 7 and 8.
func TestDominantColorSplitsAHueAcrossABucketEdge(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for y := range 10 {
		for x := range 10 {
			c := color.NRGBA{0, 0, 200, 255} // 40 %
			switch {
			case y < 3:
				c = color.NRGBA{127, 0, 0, 255} // 30 %
			case y < 6:
				c = color.NRGBA{128, 0, 0, 255} // 30 %
			}
			img.SetNRGBA(x, y, c)
		}
	}
	if got, want := dominantColor(img), (color.RGBA{0, 0, 200, 255}); got != want {
		t.Errorf("dominantColor = %v, want the 40 %% blue %v over the split 60 %% red", got, want)
	}
}

func TestDominantColorOfANoisyImage(t *testing.T) {
	hue := color.RGBA{180, 60, 40, 255}
	rng := rand.New(rand.NewSource(1))
	jitter := func(v uint8) uint8 {
		return uint8(max(0, min(255, int(v)+rng.Intn(41)-20)))
	}
	img := image.NewNRGBA(image.Rect(0, 0, 240, 360))
	for y := 0; y < 360; y++ {
		for x := 0; x < 240; x++ {
			img.Set(x, y, color.RGBA{jitter(hue.R), jitter(hue.G), jitter(hue.B), 255})
		}
	}
	if got := dominantColor(img); colorDistance(got, hue) > 16 {
		t.Errorf("dominantColor = %v, want near %v", got, hue)
	}
}

// Transparency is flattened onto white, as on a printed cover.
func TestDominantColorTreatsTransparencyAsWhite(t *testing.T) {
	if got := dominantColor(solidImage(50, 80, color.NRGBA{0, 0, 255, 0})); colorDistance(got, colorWhite) > 2 {
		t.Errorf("fully transparent: dominantColor = %v, want white", got)
	}
	// Half-transparent blue over white is a light blue.
	want := color.RGBA{127, 127, 255, 255}
	if got := dominantColor(solidImage(50, 80, color.NRGBA{0, 0, 255, 128})); colorDistance(got, want) > 3 {
		t.Errorf("half transparent: dominantColor = %v, want near %v", got, want)
	}
}

func TestDominantColorOfTinyImages(t *testing.T) {
	c := color.RGBA{12, 200, 99, 255}
	if got := dominantColor(solidImage(1, 1, c)); colorDistance(got, c) > 0 {
		t.Errorf("1 x 1: dominantColor = %v, want %v", got, c)
	}
	if got := dominantColor(image.NewNRGBA(image.Rectangle{})); got != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("empty image: dominantColor = %v, want white", got)
	}
}

func TestTextColorFor(t *testing.T) {
	tests := []struct {
		name string
		bg   color.RGBA
		want color.RGBA
	}{
		{"white", colorWhite, colorBlack},
		{"black", colorBlack, colorWhite},
		{"yellow", color.RGBA{255, 255, 0, 255}, colorBlack},
		{"navy", color.RGBA{0, 0, 128, 255}, colorWhite},
		// Pure red has a luminance of 0.2126: 5.25:1 against black,
		// 4.00:1 against white.
		{"red", color.RGBA{255, 0, 0, 255}, colorBlack},
		// Mid grey (#808080, luminance 0.216) is just on the light side
		// of the boundary (luminance 0.179, about #767676).
		{"mid grey", color.RGBA{128, 128, 128, 255}, colorBlack},
		{"grey 118", color.RGBA{118, 118, 118, 255}, colorBlack},
		{"grey 117", color.RGBA{117, 117, 117, 255}, colorWhite},
		{"pastel", color.RGBA{250, 210, 220, 255}, colorBlack},
		{"dark green", color.RGBA{0, 80, 30, 255}, colorWhite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := textColorFor(tt.bg); got != tt.want {
				t.Errorf("textColorFor(%v) = %v, want %v", tt.bg, got, tt.want)
			}
		})
	}
}

func TestTextColorForPicksTheHigherContrast(t *testing.T) {
	for v := 0; v <= 255; v++ {
		bg := color.RGBA{uint8(v), uint8(v), uint8(v), 255}
		got := textColorFor(bg)
		other := colorWhite
		if got == colorWhite {
			other = colorBlack
		} else if got != colorBlack {
			t.Fatalf("textColorFor(%v) = %v, want black or white", bg, got)
		}
		if cg, co := contrastRatio(got, bg), contrastRatio(other, bg); cg < co {
			t.Errorf("grey %d: %v has contrast %.3f, %v has %.3f", v, got, cg, other, co)
		}
	}
}

func TestContrastRatio(t *testing.T) {
	if r := contrastRatio(colorBlack, colorWhite); math.Abs(r-21) > 1e-9 {
		t.Errorf("black on white: %v, want 21", r)
	}
	if r := contrastRatio(colorWhite, colorBlack); math.Abs(r-21) > 1e-9 {
		t.Errorf("white on black: %v, want 21", r)
	}
	if r := contrastRatio(navyish, navyish); math.Abs(r-1) > 1e-9 {
		t.Errorf("same colour: %v, want 1", r)
	}
}

var navyish = color.RGBA{20, 30, 100, 255}
