package parser

import (
	"testing"

	"github.com/bytebase/omni/mysql/ast"
)

// ParseAndCheck parses sql and verifies basic properties:
// - Parse succeeds for valid SQL
// - All Loc.Start fields on AST nodes are non-negative
func ParseAndCheck(t *testing.T, sql string) *ast.List {
	t.Helper()
	result, err := Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", sql, err)
	}
	if result == nil {
		t.Fatalf("Parse(%q) returned nil", sql)
	}
	// Walk AST and check all Loc.Start fields >= 0
	for i, item := range result.Items {
		checkLocations(t, sql, i, item)
	}
	return result
}

// ParseExpectError verifies that parsing the given SQL produces an error.
func ParseExpectError(t *testing.T, sql string) {
	t.Helper()
	_, err := Parse(sql)
	if err == nil {
		t.Fatalf("Parse(%q) expected error, got nil", sql)
	}
}

// ParseAndCompare parses sql and compares the AST string representation
// against the expected string.
func ParseAndCompare(t *testing.T, sql string, expected string) {
	t.Helper()
	result := ParseAndCheck(t, sql)
	if result.Len() == 0 {
		t.Fatalf("Parse(%q) returned empty list", sql)
	}
	got := ast.NodeToString(result.Items[0])
	if got != expected {
		t.Errorf("Parse(%q):\n  got:  %s\n  want: %s", sql, got, expected)
	}
}

// checkLocations recursively checks that Loc.Start fields are non-negative.
func checkLocations(t *testing.T, sql string, idx int, node ast.Node) {
	t.Helper()
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.SelectStmt:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] SelectStmt.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.InsertStmt:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] InsertStmt.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.UpdateStmt:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] UpdateStmt.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.DeleteStmt:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] DeleteStmt.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.CreateTableStmt:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] CreateTableStmt.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.IntLit:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] IntLit.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.StringLit:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] StringLit.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.BinaryExpr:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] BinaryExpr.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
		checkLocations(t, sql, idx, n.Left)
		checkLocations(t, sql, idx, n.Right)
	case *ast.UnaryExpr:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] UnaryExpr.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
		checkLocations(t, sql, idx, n.Operand)
	case *ast.ColumnRef:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] ColumnRef.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.FuncCallExpr:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] FuncCallExpr.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.CaseExpr:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] CaseExpr.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
	case *ast.RawStmt:
		if n.Loc.Start < 0 {
			t.Errorf("Parse(%q) stmt[%d] RawStmt.Loc.Start = %d, want >= 0", sql, idx, n.Loc.Start)
		}
		checkLocations(t, sql, idx, n.Stmt)
	}
}

// TestLexerBasic tests the lexer with basic tokens.
func TestLexerBasic(t *testing.T) {
	tests := []struct {
		input string
		types []int
	}{
		{"SELECT 1", []int{kwSELECT, tokICONST}},
		{"SELECT 'hello'", []int{kwSELECT, tokSCONST}},
		{"SELECT `col`", []int{kwSELECT, tokIDENT}},
		{"SELECT @var", []int{kwSELECT, tokIDENT}},
		{"SELECT @@global.var", []int{kwSELECT, tokIDENT}},
		{"1 + 2", []int{tokICONST, '+', tokICONST}},
		{"1 <= 2", []int{tokICONST, tokLessEq, tokICONST}},
		{"1 >= 2", []int{tokICONST, tokGreaterEq, tokICONST}},
		{"1 <> 2", []int{tokICONST, tokNotEq, tokICONST}},
		{"1 != 2", []int{tokICONST, tokNotEq, tokICONST}},
		{"1 <=> 2", []int{tokICONST, tokNullSafeEq, tokICONST}},
		{"1 << 2", []int{tokICONST, tokShiftLeft, tokICONST}},
		{"1 >> 2", []int{tokICONST, tokShiftRight, tokICONST}},
		{"a := 1", []int{tokIDENT, tokAssign, tokICONST}},
		{"0xFF", []int{tokXCONST}},
		{"0b101", []int{tokBCONST}},
		{"X'FF'", []int{tokXCONST}},
		{"b'101'", []int{tokBCONST}},
		{"3.14", []int{tokFCONST}},
		{".5", []int{tokFCONST}},
		{"1e10", []int{tokFCONST}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lex := NewLexer(tt.input)
			for i, want := range tt.types {
				tok := lex.NextToken()
				if tok.Type != want {
					t.Errorf("token[%d] type = %d, want %d (input=%q)", i, tok.Type, want, tt.input)
				}
			}
			// Should be at EOF
			tok := lex.NextToken()
			if tok.Type != tokEOF {
				t.Errorf("expected EOF, got type=%d (input=%q)", tok.Type, tt.input)
			}
		})
	}
}

// TestLexerComments tests that comments are properly skipped.
func TestLexerComments(t *testing.T) {
	tests := []struct {
		input string
		types []int
	}{
		{"SELECT -- comment\n1", []int{kwSELECT, tokICONST}},
		{"SELECT # comment\n1", []int{kwSELECT, tokICONST}},
		{"SELECT /* comment */ 1", []int{kwSELECT, tokICONST}},
		{"SELECT /* nested /* comment */ */ 1", []int{kwSELECT, tokICONST}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lex := NewLexer(tt.input)
			for i, want := range tt.types {
				tok := lex.NextToken()
				if tok.Type != want {
					t.Errorf("token[%d] type = %d, want %d (input=%q)", i, tok.Type, want, tt.input)
				}
			}
		})
	}
}

// TestLexerStrings tests string literal scanning.
func TestLexerStrings(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{`'hello'`, "hello"},
		{`'it''s'`, "it's"},
		{`"double"`, "double"},
		{`'back\\slash'`, "back\\slash"},
		{`'new\nline'`, "new\nline"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lex := NewLexer(tt.input)
			tok := lex.NextToken()
			if tok.Type != tokSCONST {
				t.Errorf("type = %d, want tokSCONST", tok.Type)
			}
			if tok.Str != tt.value {
				t.Errorf("value = %q, want %q", tok.Str, tt.value)
			}
		})
	}
}

// TestLexerBacktickIdent tests backtick-quoted identifier scanning.
func TestLexerBacktickIdent(t *testing.T) {
	tests := []struct {
		input string
		value string
	}{
		{"`column`", "column"},
		{"`col``name`", "col`name"},
		{"`table name`", "table name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lex := NewLexer(tt.input)
			tok := lex.NextToken()
			if tok.Type != tokIDENT {
				t.Errorf("type = %d, want tokIDENT", tok.Type)
			}
			if tok.Str != tt.value {
				t.Errorf("value = %q, want %q", tok.Str, tt.value)
			}
		})
	}
}

// TestLexerKeywords tests keyword recognition.
func TestLexerKeywords(t *testing.T) {
	tests := []struct {
		input string
		typ   int
	}{
		{"SELECT", kwSELECT},
		{"select", kwSELECT},
		{"SeLeCt", kwSELECT},
		{"INSERT", kwINSERT},
		{"UPDATE", kwUPDATE},
		{"DELETE", kwDELETE},
		{"CREATE", kwCREATE},
		{"ALTER", kwALTER},
		{"DROP", kwDROP},
		{"TABLE", kwTABLE},
		{"FROM", kwFROM},
		{"WHERE", kwWHERE},
		{"AND", kwAND},
		{"OR", kwOR},
		{"NOT", kwNOT},
		{"NULL", kwNULL},
		{"TRUE", kwTRUE},
		{"FALSE", kwFALSE},
		{"DIV", kwDIV},
		{"MOD", kwMOD},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lex := NewLexer(tt.input)
			tok := lex.NextToken()
			if tok.Type != tt.typ {
				t.Errorf("keyword %q: type = %d, want %d", tt.input, tok.Type, tt.typ)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Batch 1: names — identifiers, qualified names, variable refs
// ---------------------------------------------------------------------------

// TestParseIdentifiers tests simple and backtick-quoted identifier parsing.
func TestParseIdentifiers(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"col1", "col1"},
		{"`my col`", "my col"},
		{"`select`", "select"},
		{"_myvar", "_myvar"},
		{"$dollar", "$dollar"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := &Parser{lexer: NewLexer(tt.input)}
			p.advance()
			name, _, err := p.parseIdentifier()
			if err != nil {
				t.Fatalf("parseIdentifier(%q) error: %v", tt.input, err)
			}
			if name != tt.want {
				t.Errorf("parseIdentifier(%q) = %q, want %q", tt.input, name, tt.want)
			}
		})
	}
}

// TestParseQualifiedNames tests column refs (unqualified, table.col, schema.table.col, table.*, schema.table.*).
func TestParseQualifiedNames(t *testing.T) {
	tests := []struct {
		input string
		want  string // expected NodeToString
	}{
		{"col1", "{COLREF :loc 0 :col col1}"},
		{"`col`", "{COLREF :loc 0 :col col}"},
		{"t.col", "{COLREF :loc 0 :table t :col col}"},
		{"`db`.`tbl`.`col`", "{COLREF :loc 0 :schema db :table tbl :col col}"},
		{"t.*", "{COLREF :loc 0 :table t :star true}"},
		{"db.t.*", "{COLREF :loc 0 :schema db :table t :star true}"},
		// Keywords as identifiers in qualified names
		{"t.status", "{COLREF :loc 0 :table t :col status}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := &Parser{lexer: NewLexer(tt.input)}
			p.advance()
			ref, err := p.parseColumnRef()
			if err != nil {
				t.Fatalf("parseColumnRef(%q) error: %v", tt.input, err)
			}
			got := ast.NodeToString(ref)
			if got != tt.want {
				t.Errorf("parseColumnRef(%q):\n  got:  %s\n  want: %s", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseTableRef tests table reference parsing (simple and schema-qualified).
func TestParseTableRef(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users", "{TABLEREF :loc 0 :name users}"},
		{"`users`", "{TABLEREF :loc 0 :name users}"},
		{"mydb.users", "{TABLEREF :loc 0 :schema mydb :name users}"},
		{"`my db`.`my table`", "{TABLEREF :loc 0 :schema my db :name my table}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := &Parser{lexer: NewLexer(tt.input)}
			p.advance()
			ref, err := p.parseTableRef()
			if err != nil {
				t.Fatalf("parseTableRef(%q) error: %v", tt.input, err)
			}
			got := ast.NodeToString(ref)
			if got != tt.want {
				t.Errorf("parseTableRef(%q):\n  got:  %s\n  want: %s", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseTableRefWithAlias tests table reference with alias parsing.
func TestParseTableRefWithAlias(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users AS u", "{TABLEREF :loc 0 :name users :alias u}"},
		{"users u", "{TABLEREF :loc 0 :name users :alias u}"},
		{"mydb.users AS u", "{TABLEREF :loc 0 :schema mydb :name users :alias u}"},
		{"users", "{TABLEREF :loc 0 :name users}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := &Parser{lexer: NewLexer(tt.input)}
			p.advance()
			ref, err := p.parseTableRefWithAlias()
			if err != nil {
				t.Fatalf("parseTableRefWithAlias(%q) error: %v", tt.input, err)
			}
			got := ast.NodeToString(ref)
			if got != tt.want {
				t.Errorf("parseTableRefWithAlias(%q):\n  got:  %s\n  want: %s", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseVariableRefs tests user and system variable reference parsing.
func TestParseVariableRefs(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"@myvar", "{VAR :name myvar :loc 0}"},
		{"@@version", "{VAR :name version :loc 0 :system true}"},
		{"@@global.max_connections", "{VAR :name max_connections :loc 0 :system true :scope GLOBAL}"},
		{"@@session.sql_mode", "{VAR :name sql_mode :loc 0 :system true :scope SESSION}"},
		{"@@local.wait_timeout", "{VAR :name wait_timeout :loc 0 :system true :scope LOCAL}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := &Parser{lexer: NewLexer(tt.input)}
			p.advance()
			ref, err := p.parseVariableRef()
			if err != nil {
				t.Fatalf("parseVariableRef(%q) error: %v", tt.input, err)
			}
			got := ast.NodeToString(ref)
			if got != tt.want {
				t.Errorf("parseVariableRef(%q):\n  got:  %s\n  want: %s", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseIdentifierError tests that non-identifier tokens produce errors.
func TestParseIdentifierError(t *testing.T) {
	inputs := []string{
		"123",   // number
		"'str'", // string literal
		"(",     // punctuation
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			p := &Parser{lexer: NewLexer(input)}
			p.advance()
			_, _, err := p.parseIdentifier()
			if err == nil {
				t.Errorf("parseIdentifier(%q) expected error, got nil", input)
			}
		})
	}
}

// TestParseVariableRefError tests that non-variable tokens produce errors.
func TestParseVariableRefError(t *testing.T) {
	inputs := []string{
		"col",   // plain identifier
		"123",   // number
		"'str'", // string literal
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			p := &Parser{lexer: NewLexer(input)}
			p.advance()
			_, err := p.parseVariableRef()
			if err == nil {
				t.Errorf("parseVariableRef(%q) expected error, got nil", input)
			}
		})
	}
}

// TestParseEmpty tests parsing empty input.
func TestParseEmpty(t *testing.T) {
	result, err := Parse("")
	if err != nil {
		t.Fatalf("Parse(\"\") error: %v", err)
	}
	if result.Len() != 0 {
		t.Errorf("Parse(\"\") returned %d items, want 0", result.Len())
	}
}

// TestParseSemicolons tests parsing semicolons only.
func TestParseSemicolons(t *testing.T) {
	result, err := Parse(";;;")
	if err != nil {
		t.Fatalf("Parse(\";;;\") error: %v", err)
	}
	if result.Len() != 0 {
		t.Errorf("Parse(\";;;\") returned %d items, want 0", result.Len())
	}
}
