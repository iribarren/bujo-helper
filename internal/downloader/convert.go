package downloader

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/webp"
)

// webpJPEGQuality is the quality of a WebP image saved as JPEG: high, as
// the image is printed and was already compressed once.
const webpJPEGQuality = 92

// isWebP reports whether data starts like a WebP file ("RIFF", a size,
// "WEBP").
func isWebP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

// convertWebP decodes a WebP image and encodes it as JPEG, or as PNG when it
// has transparent pixels, so every saved image opens in common viewers and
// print tools. It returns the encoded bytes and their file extension.
func convertWebP(data []byte) ([]byte, string, error) {
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode WebP: %w", err)
	}
	var buf bytes.Buffer
	if hasAlpha(img) {
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", fmt.Errorf("encode WebP as PNG: %w", err)
		}
		return buf.Bytes(), ".png", nil
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: webpJPEGQuality}); err != nil {
		return nil, "", fmt.Errorf("encode WebP as JPEG: %w", err)
	}
	return buf.Bytes(), ".jpg", nil
}

// hasAlpha reports whether img has a pixel that is not fully opaque.
func hasAlpha(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return !o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return true
			}
		}
	}
	return false
}
