package deparse

import (
	"testing"

	ast "github.com/bytebase/omni/mysql/ast"
	"github.com/bytebase/omni/mysql/parser"
)

// parseExpr parses a SQL expression by wrapping it in SELECT and extracting
// the first target list expression from the AST.
func parseExpr(t *testing.T, expr string) ast.ExprNode {
	t.Helper()
	sql := "SELECT " + expr
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", sql, err)
	}
	if list.Len() == 0 {
		t.Fatalf("no statements parsed from %q", sql)
	}
	sel, ok := list.Items[0].(*ast.SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", list.Items[0])
	}
	if len(sel.TargetList) == 0 {
		t.Fatalf("no target list in SELECT from %q", sql)
	}
	target := sel.TargetList[0]
	// TargetList entries may be ResTarget wrapping the actual expression.
	if rt, ok := target.(*ast.ResTarget); ok {
		return rt.Val
	}
	return target
}

func TestDeparse_Section_1_1_IntFloatNull(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		// Integer literals
		{"integer_1", "1", "1"},
		{"negative_integer", "-5", "-5"},
		{"large_integer", "9999999999", "9999999999"},
		{"zero", "0", "0"},

		// Float literals
		{"float_1_5", "1.5", "1.5"},
		{"float_with_exponent", "1.5e10", "1.5e10"},
		{"float_zero_point_five", "0.5", "0.5"},

		// NULL literal
		{"null", "NULL", "NULL"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := parseExpr(t, tc.input)
			got := Deparse(node)
			if got != tc.expected {
				t.Errorf("Deparse(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestDeparse_NilNode(t *testing.T) {
	got := Deparse(nil)
	if got != "" {
		t.Errorf("Deparse(nil) = %q, want empty string", got)
	}
}
