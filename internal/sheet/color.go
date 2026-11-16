package sheet

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"
)

// colorSample is the longest side, in pixels, an image is downscaled to
// before its dominant colour is counted: enough to tell its main areas
// apart, few enough pixels to count quickly.
const colorSample = 64

// colorBits is how many high bits of each channel name a colour bucket:
// 4 bits make 16 levels a channel, 4096 buckets.
const colorBits = 4

// Black and white, the text colours of a spine.
var (
	colorBlack = color.RGBA{0, 0, 0, 255}
	colorWhite = color.RGBA{255, 255, 255, 255}
)

// dominantColor returns the main colour of img: the image is downscaled
// to at most colorSample pixels a side, its pixels are sorted into buckets
// of similar colour (the top colorBits bits of each channel), and the result
// is the mean colour of the fullest bucket (the first one, on a tie).
// Transparency is flattened onto white, as the cover would print; an empty
// image is white.
func dominantColor(img image.Image) color.RGBA {
	sb := img.Bounds()
	if sb.Empty() {
		return colorWhite
	}
	w, h := sb.Dx(), sb.Dy()
	if long := max(w, h); long > colorSample {
		w = max(1, int(math.Round(float64(w)*colorSample/float64(long))))
		h = max(1, int(math.Round(float64(h)*colorSample/float64(long))))
	}
	small := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(small, small.Bounds(), image.White, image.Point{}, draw.Src)
	if w == sb.Dx() && h == sb.Dy() {
		draw.Draw(small, small.Bounds(), img, sb.Min, draw.Over)
	} else {
		draw.BiLinear.Scale(small, small.Bounds(), img, sb, draw.Over, nil)
	}

	const shift = 8 - colorBits
	var count [1 << (3 * colorBits)]int
	var sum [1 << (3 * colorBits)][3]int
	for i := 0; i < len(small.Pix); i += 4 {
		r, g, b := small.Pix[i], small.Pix[i+1], small.Pix[i+2]
		k := int(r>>shift)<<(2*colorBits) | int(g>>shift)<<colorBits | int(b>>shift)
		count[k]++
		sum[k][0] += int(r)
		sum[k][1] += int(g)
		sum[k][2] += int(b)
	}
	best := 0
	for k := range count {
		if count[k] > count[best] {
			best = k
		}
	}
	n := float64(count[best])
	mean := func(s int) uint8 { return uint8(math.Round(float64(s) / n)) }
	return color.RGBA{mean(sum[best][0]), mean(sum[best][1]), mean(sum[best][2]), 255}
}

// textColorFor returns the text colour that reads best on bg: black or
// white, whichever has the higher WCAG 2 contrast ratio with it (black on
// a tie). Backgrounds lighter than a relative luminance of about 0.179
// (#767676 among greys) get black text.
func textColorFor(bg color.RGBA) color.RGBA {
	if contrastRatio(colorWhite, bg) > contrastRatio(colorBlack, bg) {
		return colorWhite
	}
	return colorBlack
}

// contrastRatio is the WCAG 2 contrast ratio of two colours, from 1 (the
// same luminance) to 21 (black and white).
func contrastRatio(a, b color.RGBA) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// relativeLuminance is the WCAG 2 relative luminance of c, from 0 (black)
// to 1 (white), of its linearised sRGB channels.
func relativeLuminance(c color.RGBA) float64 {
	linear := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}
