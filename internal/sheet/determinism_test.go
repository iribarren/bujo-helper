package sheet

import (
	"bytes"
	"image/color"
	"testing"
	"time"
)

// The same inputs give the same PDF bytes: building a checklist of
// repeated landscape images twice in one process, with fixed dates and
// nothing else set by the test, writes identical files.
func TestBuildIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	wide := writeImage(t, dir, "Wide - 2026.jpg", 120, 60, color.Gray{0x40})
	other := writeImage(t, dir, "Other - 2025.png", 90, 60, color.Gray{0xa0})
	paths := []string{wide, other, wide, wide, other, wide}
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, opts := range []Options{
		{Format: checkFormat, Header: "Watchlist"},
		{Format: fullFormat, Header: "Places"},
		{Format: cardsFormat},
	} {
		t.Run(opts.Format, func(t *testing.T) {
			var outs [2][]byte
			for i := range outs {
				pdf, err := build(paths, opts)
				if err != nil {
					t.Fatal(err)
				}
				pdf.SetCreationDate(when)
				pdf.SetModificationDate(when)
				var buf bytes.Buffer
				if err := pdf.Output(&buf); err != nil {
					t.Fatal(err)
				}
				outs[i] = buf.Bytes()
			}
			if !bytes.Equal(outs[0], outs[1]) {
				t.Errorf("two builds of the same %s differ (%d and %d bytes)", opts.Format, len(outs[0]), len(outs[1]))
			}
		})
	}
}
