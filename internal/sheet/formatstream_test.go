package sheet

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

// ptPerMM converts millimetres to the points of a PDF content stream.
const ptPerMM = 72 / 25.4

// shownText matches a line of text written with an embedded font: its fill
// grey, its position and its string, as fpdf writes them.
var shownText = regexp.MustCompile(`q ([0-9.]+) g BT ([0-9.-]+) ([0-9.-]+) Td \(((?:[^\\)]|\\.)*)\) Tj ET Q`)

// streamText is a line of text drawn in a content stream.
type streamText struct {
	Grey string // fill grey, as fpdf writes it
	X, Y string // position, in points from the bottom left, as fpdf writes it
	Text string
}

// shownTexts returns the lines of text drawn in stream, in order. An
// embedded font's string holds UTF-16BE code units, escaped as fpdf does,
// and its ToUnicode map is the identity.
func shownTexts(stream string) []streamText {
	var out []streamText
	for _, m := range shownText.FindAllStringSubmatch(stream, -1) {
		var raw []byte
		for i := 0; i < len(m[4]); i++ {
			c := m[4][i]
			if c == '\\' && i+1 < len(m[4]) {
				i++
				c = m[4][i]
				if c == 'r' {
					c = '\r'
				}
			}
			raw = append(raw, c)
		}
		units := make([]uint16, len(raw)/2)
		for i := range units {
			units[i] = uint16(raw[2*i])<<8 | uint16(raw[2*i+1])
		}
		out = append(out, streamText{Grey: m[1], X: m[2], Y: m[3], Text: string(utf16.Decode(units))})
	}
	return out
}

// greyOp is how fpdf sets grey level v (0–255) for filling text.
func greyOp(v int) string { return fmt.Sprintf("%.3f", float64(v)/255) }

// A spine is filled with the cover's dominant colour inside its cut
// border, and its title and creator are written top to bottom (turned
// −90° about the spine's top-right corner) in the text colour that
// contrasts more with it.
func TestBuildSpinesDrawColourTextColourAndRotation(t *testing.T) {
	dir := t.TempDir()
	// PNG keeps the solid colours exact; neither is a grey, which fpdf
	// would write with g instead of rg.
	colours := []color.RGBA{{200, 40, 40, 255}, {250, 230, 120, 255}}
	paths := []string{
		writeImage(t, dir, "Dune - Frank Herbert.png", 66, 100, colours[0]),
		writeImage(t, dir, "Kindred - Octavia Butler.png", 66, 100, colours[1]),
	}
	got := pageStreams(render(t, paths, Options{Format: "spines"}))
	g, err := Options{Format: "spines"}.Grid()
	if err != nil {
		t.Fatal(err)
	}
	pages, err := g.Cells(len(paths))
	if err != nil {
		t.Fatal(err)
	}
	const k = ptPerMM
	at := 0
	for i, spine := range pages[0] {
		bg := colours[i]
		fg := textColorFor(bg)
		fill := fmt.Sprintf("%.3f %.3f %.3f rg\n", float64(bg.R)/255, float64(bg.G)/255, float64(bg.B)/255)
		rect := fmt.Sprintf("%.2f %.2f %.2f %.2f re B\n", spine.X*k, (g.Page.Height-spine.Y)*k, spine.W*k, -spine.H*k)
		// A −90° turn about (x, y), the top-right corner in points.
		x, y := (spine.X+spine.W)*k, (g.Page.Height-spine.Y)*k
		turn := fmt.Sprintf("q\n0.00000 -1.00000 1.00000 0.00000 %.5f %.5f cm\n", x-y, y+x)
		f := strings.Index(got[at:], fill)
		r := strings.Index(got[at:], rect)
		q := strings.Index(got[at:], turn)
		if f < 0 || r < f || q < r {
			t.Fatalf("spine %d: want fill %q, then rect %q, then turn %q after offset %d in\n%s", i, fill, rect, turn, at, got)
		}
		block := got[at+q+len(turn):]
		end := strings.Index(block, "\nQ\n")
		if end < 0 {
			t.Fatalf("spine %d: the turn is never ended in\n%s", i, block)
		}
		block = block[:end]
		texts := shownTexts(block)
		title, creator := ParseName(paths[i])
		if len(texts) != 2 || texts[0].Text != title || texts[1].Text != creator {
			t.Errorf("spine %d: turned texts %+v, want %q then %q", i, texts, title, creator)
		}
		for _, tx := range texts {
			if want := greyOp(int(fg.R)); tx.Grey != want {
				t.Errorf("spine %d: %q in grey %s, want %s (text colour %v on %v)", i, tx.Text, tx.Grey, want, fg, bg)
			}
		}
		at += q + len(turn) + end
	}
}

// A review card writes its title, creator and date label at the left of
// its text column, in their greys.
func TestBuildCardsWriteTitleCreatorAndDateLabel(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeImage(t, dir, "Dune - Frank Herbert.png", 66, 100, color.RGBA{200, 40, 40, 255}),
		writeImage(t, dir, "Kindred - Octavia Butler.png", 100, 100, color.RGBA{40, 200, 40, 255}),
		writeImage(t, dir, "Beloved - Toni Morrison.png", 150, 100, color.RGBA{40, 40, 200, 255}),
	}
	ratios, err := AspectRatios(paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, opts := range []Options{{Format: "cards"}, {Format: "cards", DateLabel: "Watched"}} {
		got := shownTexts(pageStreams(render(t, paths, opts)))
		g, err := opts.Grid()
		if err != nil {
			t.Fatal(err)
		}
		pages, err := g.Cells(len(paths))
		if err != nil {
			t.Fatal(err)
		}
		var want []streamText
		for i, cell := range pages[0] {
			c := layoutCard(cell, ratios[i])
			title, creator := ParseName(paths[i])
			x := func(b Box) string { return fmt.Sprintf("%.2f", b.X*ptPerMM) }
			want = append(want,
				streamText{Grey: greyOp(titleGrey), X: x(c.Title), Text: title},
				streamText{Grey: greyOp(textGrey), X: x(c.Creator), Text: creator},
				streamText{Grey: greyOp(textGrey), X: x(c.Date), Text: opts.dateLabel()})
		}
		if len(got) != len(want) {
			t.Fatalf("date label %q: %d texts %+v, want %d", opts.DateLabel, len(got), got, len(want))
		}
		for i := range want {
			if got[i].Grey != want[i].Grey || got[i].X != want[i].X || got[i].Text != want[i].Text {
				t.Errorf("date label %q, text %d: %+v, want %q in grey %s at x %s", opts.DateLabel, i, got[i], want[i].Text, want[i].Grey, want[i].X)
			}
		}
	}
}

// pathPoint is a point of a content stream path, in points.
type pathPoint struct{ X, Y float64 }

// strokedOutlines returns the closed and stroked paths of stream as lists
// of segments: each segment is its start point followed by the points of
// its operator (one for a line, three for a Bézier curve).
func strokedOutlines(stream string) [][][]pathPoint {
	var outlines [][][]pathPoint
	var segs [][]pathPoint
	var cur pathPoint
	inPath := false
	lines := strings.Split(stream, "\n")
	for i, line := range lines {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		op := f[len(f)-1]
		nums := make([]float64, 0, len(f)-1)
		for _, s := range f[:len(f)-1] {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				nums = nil
				break
			}
			nums = append(nums, v)
		}
		switch {
		case op == "m" && len(nums) == 2 && len(f) == 3:
			inPath, segs, cur = true, nil, pathPoint{nums[0], nums[1]}
		case inPath && op == "l" && len(nums) == 2:
			next := pathPoint{nums[0], nums[1]}
			segs = append(segs, []pathPoint{cur, next})
			cur = next
		case inPath && op == "c" && len(nums) == 6:
			seg := []pathPoint{cur, {nums[0], nums[1]}, {nums[2], nums[3]}, {nums[4], nums[5]}}
			segs = append(segs, seg)
			cur = seg[3]
		case inPath && op == "h" && i+1 < len(lines) && lines[i+1] == "S":
			outlines = append(outlines, segs)
			inPath = false
		default:
			inPath = false
		}
	}
	return outlines
}

// bezierAt returns the point at t of the cubic Bézier curve seg.
func bezierAt(seg []pathPoint, t float64) pathPoint {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return pathPoint{
		X: a*seg[0].X + b*seg[1].X + c*seg[2].X + d*seg[3].X,
		Y: a*seg[0].Y + b*seg[1].Y + c*seg[2].Y + d*seg[3].Y,
	}
}

// Each stub's outline cuts a semicircular notch into its top and bottom
// edges, centred on the perforation: every curve lies on the notch circle,
// inside the stub, and the notch runs from one end of its diameter on the
// edge to the other.
func TestBuildStubsNotchesBiteInwardAtThePerforation(t *testing.T) {
	paths := cutFixtures(t)
	got := pageStreams(render(t, paths, Options{Format: "stubs"}))
	ratios, err := AspectRatios(paths)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Options{Format: "stubs"}.Grid()
	if err != nil {
		t.Fatal(err)
	}
	pages, err := g.Cells(len(paths))
	if err != nil {
		t.Fatal(err)
	}
	outlines := strokedOutlines(got)
	if len(outlines) != len(paths) {
		t.Fatalf("%d closed outlines, want %d", len(outlines), len(paths))
	}
	// In points: m and l points are written to 0.01, and fpdf's Bézier arcs
	// run up to about 0.03 inside the true circle between their ends.
	const k, tol, arcTol = ptPerMM, 0.02, 0.05
	for i, cell := range pages[0] {
		s := layoutStub(cell, ratios[i])
		if s.Perforation.X != s.Notches[0].X || s.Perforation.X != s.Notches[1].X {
			t.Errorf("stub %d: notches at x %v and %v, perforation at %v", i, s.Notches[0].X, s.Notches[1].X, s.Perforation.X)
		}
		r := s.NotchRadius * k
		edges := []struct {
			name   string
			centre pathPoint
			inward float64 // the sign of y into the stub
		}{
			{"top", pathPoint{s.Perforation.X * k, (g.Page.Height - s.Stub.Y) * k}, -1},
			{"bottom", pathPoint{s.Perforation.X * k, (g.Page.Height - s.Stub.Y - s.Stub.H) * k}, 1},
		}
		for _, e := range edges {
			var curves [][]pathPoint
			for _, seg := range outlines[i] {
				if len(seg) == 4 && math.Abs(seg[0].Y-e.centre.Y) < r+tol {
					curves = append(curves, seg)
				}
			}
			if len(curves) < 2 {
				t.Errorf("stub %d, %s: %d curves, want a notch of at least 2", i, e.name, len(curves))
				continue
			}
			for _, seg := range curves {
				for _, tt := range []float64{0, 0.25, 0.5, 0.75, 1} {
					p := bezierAt(seg, tt)
					if d := math.Hypot(p.X-e.centre.X, p.Y-e.centre.Y); math.Abs(d-r) > arcTol {
						t.Errorf("stub %d, %s notch: %+v is %.3f pt from %+v, want the radius %.3f", i, e.name, p, d, e.centre, r)
					}
					if (p.Y-e.centre.Y)*e.inward < -tol {
						t.Errorf("stub %d, %s notch: %+v bulges out of the stub past %.2f", i, e.name, p, e.centre.Y)
					}
				}
			}
			ends := []pathPoint{curves[0][0], curves[len(curves)-1][3]}
			if lo, hi := math.Min(ends[0].X, ends[1].X), math.Max(ends[0].X, ends[1].X); math.Abs(lo-(e.centre.X-r)) > tol || math.Abs(hi-(e.centre.X+r)) > tol ||
				math.Abs(ends[0].Y-e.centre.Y) > tol || math.Abs(ends[1].Y-e.centre.Y) > tol {
				t.Errorf("stub %d, %s notch runs %+v to %+v, want the ends of its diameter on the edge about %+v", i, e.name, ends[0], ends[1], e.centre)
			}
		}
	}
}
