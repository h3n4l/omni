package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bytebase/omni/oracle/ast"
)

// ---------------------------------------------------------------------------
// Stage 1 — Adversarial tests
//
// These tests target edge cases discovered during insight analysis.
// They may fail — that is expected. The driver will dispatch the impl
// worker to fix failures.
// ---------------------------------------------------------------------------

// TestEvalStage1_Adversarial_NoLoc_ZeroValue verifies that NoLoc() is
// distinguishable from the zero-value Loc{0,0}.
func TestEvalStage1_Adversarial_NoLoc_ZeroValue(t *testing.T) {
	noLoc := ast.NoLoc()
	zeroLoc := ast.Loc{}

	if noLoc == zeroLoc {
		t.Fatal("NoLoc() == Loc{} — sentinel value must be distinguishable from zero-value position")
	}
	if noLoc.Start != -1 || noLoc.End != -1 {
		t.Fatalf("NoLoc() = Loc{%d,%d}, want Loc{-1,-1}", noLoc.Start, noLoc.End)
	}
	if zeroLoc.Start != 0 || zeroLoc.End != 0 {
		t.Fatalf("Loc{} = Loc{%d,%d}, want Loc{0,0}", zeroLoc.Start, zeroLoc.End)
	}
}

// TestEvalStage1_Adversarial_TokenEnd_EOF verifies Token.End for EOF token.
// EOF is a zero-length token; End should equal Loc (both at end of input).
func TestEvalStage1_Adversarial_TokenEnd_EOF(t *testing.T) {
	lex := NewLexer("")
	tok := lex.NextToken()
	if tok.Type != tokEOF {
		t.Fatalf("expected EOF token for empty input, got type %d", tok.Type)
	}
	if tok.End != tok.Loc {
		t.Fatalf("EOF token: End (%d) != Loc (%d) — EOF should be zero-length", tok.End, tok.Loc)
	}
}

// TestEvalStage1_Adversarial_TokenEnd_EmptyString verifies that an empty
// string literal '' has End > Loc (the two quote characters occupy bytes).
func TestEvalStage1_Adversarial_TokenEnd_EmptyString(t *testing.T) {
	lex := NewLexer("''")
	tok := lex.NextToken()
	if tok.End <= tok.Loc {
		t.Fatalf("empty string token '': End (%d) should be > Loc (%d)", tok.End, tok.Loc)
	}
	if tok.End != 2 {
		t.Fatalf("empty string token '': End = %d, want 2 (two quote chars)", tok.End)
	}
}

// TestEvalStage1_Adversarial_TokenEnd_UnicodeToken verifies that Token.End
// tracks byte offsets (not character offsets) for multi-byte Unicode input.
func TestEvalStage1_Adversarial_TokenEnd_UnicodeToken(t *testing.T) {
	// U+00E9 (e-acute) is 2 bytes in UTF-8.
	// The quoted identifier "cafe\u0301" is "caf" + e-acute = 4 chars, but 5 bytes.
	// With double quotes: total 7 bytes.
	input := `"caf` + "\u00e9" + `"`
	lex := NewLexer(input)
	tok := lex.NextToken()
	expectedEnd := len(input) // byte length
	if tok.End != expectedEnd {
		t.Fatalf("Unicode identifier token: End = %d, want %d (byte offset, not char offset)",
			tok.End, expectedEnd)
	}
}

// TestEvalStage1_Adversarial_TokenEnd_AfterComment verifies Token.End
// when a comment precedes a token. The comment should be skipped and the
// returned token's Loc/End should cover only the actual token.
func TestEvalStage1_Adversarial_TokenEnd_AfterComment(t *testing.T) {
	input := "/* comment */ SELECT"
	lex := NewLexer(input)
	tok := lex.NextToken()
	// The token should be SELECT, starting after the comment + space.
	if tok.Str != "SELECT" && !strings.EqualFold(tok.Str, "SELECT") {
		t.Fatalf("expected SELECT token after comment, got %q (type %d)", tok.Str, tok.Type)
	}
	selectStart := strings.Index(input, "SELECT")
	if tok.Loc != selectStart {
		t.Fatalf("SELECT after comment: Loc = %d, want %d", tok.Loc, selectStart)
	}
	if tok.End != selectStart+6 {
		t.Fatalf("SELECT after comment: End = %d, want %d", tok.End, selectStart+6)
	}
}

// TestEvalStage1_Adversarial_ParseError_DefaultFormat verifies that
// ParseError with empty Severity and Code still produces PG-compatible format.
func TestEvalStage1_Adversarial_ParseError_DefaultFormat(t *testing.T) {
	pe := &ParseError{
		Message:  "unexpected token",
		Position: 42,
	}
	got := pe.Error()
	want := "ERROR: unexpected token (SQLSTATE 42601)"
	if got != want {
		t.Fatalf("ParseError with empty Severity/Code:\n  got:  %q\n  want: %q", got, want)
	}
}

// TestEvalStage1_Adversarial_ParseError_CustomSeverity verifies non-default
// severity and code values in ParseError.Error() output.
func TestEvalStage1_Adversarial_ParseError_CustomSeverity(t *testing.T) {
	pe := &ParseError{
		Severity: "WARNING",
		Code:     "01000",
		Message:  "deprecated syntax",
		Position: 0,
	}
	got := pe.Error()
	want := "WARNING: deprecated syntax (SQLSTATE 01000)"
	if got != want {
		t.Fatalf("ParseError with custom severity/code:\n  got:  %q\n  want: %q", got, want)
	}
}

// TestEvalStage1_Adversarial_ParserSource_SetByParse verifies that the
// Parse() function actually sets the source field on the parser.
func TestEvalStage1_Adversarial_ParserSource_SetByParse(t *testing.T) {
	sql := "SELECT 1 FROM dual"
	result, err := Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", sql, err)
	}
	if result.Len() == 0 {
		t.Fatalf("Parse(%q) returned 0 statements", sql)
	}
	// We cannot directly access p.source from outside the package since it's
	// unexported, but we verify indirectly: RawStmt.Loc should reference
	// valid byte offsets within the source string.
	raw, ok := result.Items[0].(*ast.RawStmt)
	if !ok {
		t.Fatal("first item is not *RawStmt")
	}
	if raw.Loc.Start < 0 || raw.Loc.End > len(sql) {
		t.Fatalf("RawStmt.Loc = {%d,%d} out of bounds for source len %d",
			raw.Loc.Start, raw.Loc.End, len(sql))
	}
	substr := sql[raw.Loc.Start:raw.Loc.End]
	if !strings.Contains(strings.ToUpper(substr), "SELECT") {
		t.Fatalf("sql[%d:%d] = %q, expected it to contain SELECT",
			raw.Loc.Start, raw.Loc.End, substr)
	}
}

// TestEvalStage1_Adversarial_RawStmtLoc_MultiStatement verifies RawStmt.Loc
// boundaries for multiple statements separated by semicolons.
func TestEvalStage1_Adversarial_RawStmtLoc_MultiStatement(t *testing.T) {
	sql := "SELECT 1; SELECT 2; SELECT 3"
	result, err := Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", sql, err)
	}
	if result.Len() != 3 {
		t.Fatalf("Parse(%q) returned %d statements, want 3", sql, result.Len())
	}

	prevEnd := -1
	for i, item := range result.Items {
		raw, ok := item.(*ast.RawStmt)
		if !ok {
			t.Fatalf("item %d is not *RawStmt", i)
		}
		if raw.Loc.Start < 0 {
			t.Fatalf("stmt %d: Loc.Start = %d, want >= 0", i, raw.Loc.Start)
		}
		if raw.Loc.End <= raw.Loc.Start {
			t.Fatalf("stmt %d: Loc.End (%d) <= Loc.Start (%d)", i, raw.Loc.End, raw.Loc.Start)
		}
		if raw.Loc.Start <= prevEnd {
			t.Fatalf("stmt %d: Loc.Start (%d) <= previous Loc.End (%d) — statements overlap",
				i, raw.Loc.Start, prevEnd)
		}
		prevEnd = raw.Loc.End
	}
}

// TestEvalStage1_Adversarial_NodeLoc_NoLocNode verifies that NodeLoc returns
// NoLoc() for a node type without a Loc field (ast.String, ast.Integer, etc).
func TestEvalStage1_Adversarial_NodeLoc_NoLocNode(t *testing.T) {
	if evalNodeLoc == nil {
		t.Skip("ast.NodeLoc not registered")
	}

	// ast.String has no Loc field
	strNode := &ast.String{Str: "test"}
	loc := evalNodeLoc(strNode)
	if loc.Start != -1 || loc.End != -1 {
		t.Fatalf("NodeLoc(String{}) = Loc{%d,%d}, want Loc{-1,-1}", loc.Start, loc.End)
	}

	// ast.Integer has no Loc field
	intNode := &ast.Integer{Ival: 42}
	loc2 := evalNodeLoc(intNode)
	if loc2.Start != -1 || loc2.End != -1 {
		t.Fatalf("NodeLoc(Integer{}) = Loc{%d,%d}, want Loc{-1,-1}", loc2.Start, loc2.End)
	}
}

// TestEvalStage1_Adversarial_ListSpan_SingleElement verifies ListSpan with
// a single-element list returns that element's Loc.
func TestEvalStage1_Adversarial_ListSpan_SingleElement(t *testing.T) {
	if evalListSpan == nil {
		t.Skip("ast.ListSpan not registered")
	}

	item := &ast.RawStmt{Loc: ast.Loc{Start: 5, End: 15}}
	list := &ast.List{Items: []ast.Node{item}}
	span := evalListSpan(list)
	if span.Start != 5 || span.End != 15 {
		t.Fatalf("ListSpan([{5,15}]) = Loc{%d,%d}, want Loc{5,15}", span.Start, span.End)
	}
}

// TestEvalStage1_Adversarial_ListSpan_NoLocItems verifies ListSpan with
// items that have NoLoc() returns NoLoc().
func TestEvalStage1_Adversarial_ListSpan_NoLocItems(t *testing.T) {
	if evalListSpan == nil {
		t.Skip("ast.ListSpan not registered")
	}

	// List of String nodes (no Loc field) — NodeLoc returns NoLoc()
	list := &ast.List{Items: []ast.Node{
		&ast.String{Str: "a"},
		&ast.String{Str: "b"},
	}}
	span := evalListSpan(list)
	if span.Start != -1 || span.End != -1 {
		t.Fatalf("ListSpan with NoLoc items = Loc{%d,%d}, want Loc{-1,-1}", span.Start, span.End)
	}
}

// TestEvalStage1_Adversarial_NodeLoc_NilInput verifies NodeLoc(nil) returns NoLoc.
func TestEvalStage1_Adversarial_NodeLoc_NilInput(t *testing.T) {
	if evalNodeLoc == nil {
		t.Skip("ast.NodeLoc not registered")
	}
	loc := evalNodeLoc(nil)
	if loc.Start != -1 || loc.End != -1 {
		t.Fatalf("NodeLoc(nil) = Loc{%d,%d}, want Loc{-1,-1}", loc.Start, loc.End)
	}
}

// TestEvalStage1_Adversarial_NodeLoc_AllParsenodesTypes verifies that NodeLoc
// handles every type in parsenodes.go by checking that reflect can find a Loc
// field and NodeLoc returns a non-NoLoc value when Loc is set.
func TestEvalStage1_Adversarial_NodeLoc_AllParsenodesTypes(t *testing.T) {
	if evalNodeLoc == nil {
		t.Skip("ast.NodeLoc not registered")
	}

	// Sample a few representative types to verify NodeLoc coverage.
	// A full exhaustive check would require enumerating all types.
	types := []struct {
		name string
		node ast.Node
	}{
		{"SelectStmt", &ast.SelectStmt{Loc: ast.Loc{Start: 0, End: 10}}},
		{"InsertStmt", &ast.InsertStmt{Loc: ast.Loc{Start: 0, End: 10}}},
		{"UpdateStmt", &ast.UpdateStmt{Loc: ast.Loc{Start: 0, End: 10}}},
		{"DeleteStmt", &ast.DeleteStmt{Loc: ast.Loc{Start: 0, End: 10}}},
		{"CreateTableStmt", &ast.CreateTableStmt{Loc: ast.Loc{Start: 0, End: 10}}},
		{"DropStmt", &ast.DropStmt{Loc: ast.Loc{Start: 0, End: 10}}},
		{"ColumnRef", &ast.ColumnRef{Loc: ast.Loc{Start: 3, End: 7}}},
		{"NumberLiteral", &ast.NumberLiteral{Loc: ast.Loc{Start: 1, End: 2}}},
	}

	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			loc := evalNodeLoc(tc.node)
			nodeLoc := reflect.ValueOf(tc.node).Elem().FieldByName("Loc")
			if !nodeLoc.IsValid() {
				t.Fatalf("%s does not have Loc field", tc.name)
			}
			wantStart := int(nodeLoc.FieldByName("Start").Int())
			wantEnd := int(nodeLoc.FieldByName("End").Int())
			if loc.Start != wantStart || loc.End != wantEnd {
				t.Fatalf("NodeLoc(%s) = Loc{%d,%d}, want Loc{%d,%d}",
					tc.name, loc.Start, loc.End, wantStart, wantEnd)
			}
		})
	}
}

// TestEvalStage1_Adversarial_Parse_EmptyInput verifies parsing empty string
// returns empty list without error.
func TestEvalStage1_Adversarial_Parse_EmptyInput(t *testing.T) {
	result, err := Parse("")
	if err != nil {
		t.Fatalf("Parse('') returned error: %v", err)
	}
	if result.Len() != 0 {
		t.Fatalf("Parse('') returned %d statements, want 0", result.Len())
	}
}

// TestEvalStage1_Adversarial_Parse_OnlySemicolons verifies parsing
// semicolons-only input returns empty list.
func TestEvalStage1_Adversarial_Parse_OnlySemicolons(t *testing.T) {
	result, err := Parse(";;;")
	if err != nil {
		t.Fatalf("Parse(';;;') returned error: %v", err)
	}
	if result.Len() != 0 {
		t.Fatalf("Parse(';;;') returned %d statements, want 0", result.Len())
	}
}
