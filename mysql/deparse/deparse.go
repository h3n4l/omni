// Package deparse converts MySQL AST nodes back to SQL text,
// matching MySQL 8.0's SHOW CREATE VIEW formatting.
package deparse

import (
	"fmt"

	ast "github.com/bytebase/omni/mysql/ast"
)

// Deparse converts an expression AST node to its SQL text representation,
// matching MySQL 8.0's canonical formatting (as seen in SHOW CREATE VIEW).
func Deparse(node ast.ExprNode) string {
	if node == nil {
		return ""
	}
	return deparseExpr(node)
}

func deparseExpr(node ast.ExprNode) string {
	switch n := node.(type) {
	case *ast.IntLit:
		return fmt.Sprintf("%d", n.Value)
	case *ast.FloatLit:
		return n.Value
	case *ast.NullLit:
		return "NULL"
	case *ast.UnaryExpr:
		return deparseUnaryExpr(n)
	case *ast.ParenExpr:
		return deparseExpr(n.Expr)
	default:
		return fmt.Sprintf("/* unsupported: %T */", node)
	}
}

func deparseUnaryExpr(n *ast.UnaryExpr) string {
	operand := deparseExpr(n.Operand)
	switch n.Op {
	case ast.UnaryMinus:
		return "-" + operand
	case ast.UnaryPlus:
		// MySQL drops unary plus entirely
		return operand
	default:
		return operand
	}
}

