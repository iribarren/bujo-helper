package sheet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// unsafeTextMethods are fpdf methods that draw or measure text but that
// pdfDoc does not make printable: called on the embedded *fpdf.Fpdf, they
// would bring back the panic on runes above U+FFFF. Write is listed by its
// fpdf arity (height, text), so io.Writer's one-argument Write is not
// flagged.
var unsafeTextMethods = map[string]int{
	"Cell":            -1,
	"Cellf":           -1,
	"CellFormat":      -1,
	"MultiCell":       -1,
	"Write":           2,
	"Writef":          -1,
	"WriteAligned":    -1,
	"WriteLinkString": -1,
	"WriteLinkID":     -1,
	"SplitText":       -1,
	"SplitLines":      -1,
}

// unsafeTextCalls lists the calls in f to fpdf text methods that bypass
// pdfDoc's printable Text and GetStringWidth, as "file:line Name".
func unsafeTextCalls(fset *token.FileSet, f *ast.File) []string {
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		arity, ok := unsafeTextMethods[sel.Sel.Name]
		if !ok || (arity >= 0 && len(call.Args) != arity) {
			return true
		}
		pos := fset.Position(call.Pos())
		found = append(found, filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line)+" "+sel.Sel.Name)
		return true
	})
	return found
}

// TestDrawCodeUsesOnlyPrintableTextMethods keeps every text drawn by the
// sheet package on pdfDoc.Text and pdfDoc.GetStringWidth, the only methods
// that make text printable (see emoji_test.go for the panic they prevent).
// To draw text another way, add a printable version of that method to
// pdfDoc first, then drop it from unsafeTextMethods.
func TestDrawCodeUsesOnlyPrintableTextMethods(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range unsafeTextCalls(fset, f) {
			t.Errorf("%s bypasses pdfDoc's printable text; draw with Text or add a printable %s to pdfDoc", call, strings.Fields(call)[1])
		}
	}
}

// TestUnsafeTextCallsFlagsBypasses proves the guard catches each kind of
// bypass and lets io.Writer's Write through.
func TestUnsafeTextCallsFlagsBypasses(t *testing.T) {
	const src = `package p
func f() {
	pdf.MultiCell(10, 5, s, "", "L", false)
	pdf.CellFormat(10, 5, s, "", 0, "L", false, 0, "")
	pdf.Fpdf.Cell(10, 5, s)
	pdf.Write(5, s)
	lines := pdf.SplitText(s, 10)
	w.Write(buf)
	pdf.Text(1, 2, s)
	pdf.GetStringWidth(s)
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := unsafeTextCalls(fset, f)
	want := []string{"x.go:3 MultiCell", "x.go:4 CellFormat", "x.go:5 Cell", "x.go:6 Write", "x.go:7 SplitText"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("unsafeTextCalls = %q, want %q", got, want)
	}
}
