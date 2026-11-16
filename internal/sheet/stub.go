package sheet

// Ticket stub size, in millimetres.
const (
	StubWidth  = 90.0
	StubHeight = 40.0
)

// Ticket stub shape, in millimetres. The stub shares the review card's
// padding, cover area, text sizes and stars.
const (
	stubEndWidth  = 20.0 // from the perforation to the right edge
	notchRadius   = 2.5  // of the semicircles cut into the top and bottom edges
	admitFontSize = 8.0  // points, of the "ADMIT ONE" on the stub end
)

// point is a position on a page, in mm.
type point struct {
	X, Y float64
}

// NewStubGrid derives the grid of ticket stubs on page: as many
// StubWidth × StubHeight stubs as fit within MinMargin margins and Gap
// between stubs, centred on the page both ways. It returns an error if not
// even one stub fits on the page.
func NewStubGrid(page Page) (Grid, error) {
	return newFixedGrid(page, StubWidth, StubHeight, "stub")
}

// stubParts are the boxes of the parts of one ticket stub on its page, in
// mm. The text boxes are the room each part may use.
type stubParts struct {
	Stub        Box      // the whole stub; its notched outline is the cut line
	Main        Box      // the part left of the perforation
	End         Box      // the stub end, right of the perforation
	PerfX       float64  // x of the perforation and of the notch centres
	Notches     [2]point // centres of the top and bottom notches
	NotchRadius float64
	Perforation Box   // the dashed line, zero wide, from notch to notch
	Cover       Box   // the poster image
	Text        Box   // the text column, right of the cover area
	Title       Box   // up to titleLines lines of title
	Creator     Box   // one line of creator, drawn right under the title
	Date        Box   // the "Date" label and the rule to write the date on
	Where       Box   // the "Where" label and its rule
	Stars       []Box // starCount squares, each holding one star outline
	// Admit is the room of the "ADMIT ONE" line on the stub end, upright:
	// one line of admitFontSize text, turned a quarter turn, centred.
	Admit Box
}

// layoutStub places the parts of a ticket stub in stub, for a poster with
// aspect ratio (width / height) ratio. The perforation runs stubEndWidth
// from the right edge, between two notches cut into the top and bottom
// edges. Left of it, inside cardPadding, the cover area is full height and
// 2:3 wide; the poster is shrunk to fit it, keeping its ratio, left-aligned
// and vertically centred. The text column fills the rest of the main part:
// the title and creator at the top, the stars on the bottom padding, and
// the date and where fields spread evenly between them. Right of the
// perforation, the "ADMIT ONE" line is centred in the stub end.
func layoutStub(stub Box, ratio float64) stubParts {
	perfX := stub.X + stub.W - stubEndWidth
	s := stubParts{
		Stub:        stub,
		Main:        Box{X: stub.X, Y: stub.Y, W: perfX - stub.X, H: stub.H},
		End:         Box{X: perfX, Y: stub.Y, W: stub.X + stub.W - perfX, H: stub.H},
		PerfX:       perfX,
		Notches:     [2]point{{perfX, stub.Y}, {perfX, stub.Y + stub.H}},
		NotchRadius: notchRadius,
		Perforation: Box{X: perfX, Y: stub.Y + notchRadius, H: stub.H - 2*notchRadius},
	}

	inner := padded(s.Main, cardPadding)
	areaW := inner.H * minCellRatio
	w, h := inner.H*ratio, inner.H
	if w > areaW {
		w, h = areaW, areaW/ratio
	}
	s.Cover = Box{X: inner.X, Y: inner.Y + (inner.H-h)/2, W: w, H: h}
	textX := inner.X + areaW + cardTextGap
	s.Text = Box{X: textX, Y: inner.Y, W: inner.X + inner.W - textX, H: inner.H}

	row := func(y, h float64) Box {
		return Box{X: s.Text.X, Y: y, W: s.Text.W, H: h}
	}
	s.Title = row(s.Text.Y, titleLines*lineHeight(titleFontSize))
	s.Creator = row(s.Title.Y+s.Title.H, lineHeight(creatorFontSize))
	starsY := s.Text.Y + s.Text.H - starSize
	s.Stars = make([]Box, starCount)
	for i := range s.Stars {
		s.Stars[i] = Box{X: s.Text.X + float64(i)*(starSize+starGap), Y: starsY, W: starSize, H: starSize}
	}
	// Three equal gaps around the two fields: below the creator, between
	// the fields and above the stars.
	field := lineHeight(labelFontSize)
	gap := (starsY - (s.Creator.Y + s.Creator.H) - 2*field) / 3
	s.Date = row(s.Creator.Y+s.Creator.H+gap, field)
	s.Where = row(s.Date.Y+field+gap, field)

	endInner := padded(s.End, cardPadding)
	across := lineHeight(admitFontSize)
	s.Admit = Box{X: endInner.X + (endInner.W-across)/2, Y: endInner.Y, W: across, H: endInner.H}
	return s
}

// padded is b shrunk by pad on every side.
func padded(b Box, pad float64) Box {
	return Box{X: b.X + pad, Y: b.Y + pad, W: b.W - 2*pad, H: b.H - 2*pad}
}
