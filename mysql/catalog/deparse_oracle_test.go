package catalog

import (
	"fmt"
	"strings"
	"testing"

	nodes "github.com/bytebase/omni/mysql/ast"
	"github.com/bytebase/omni/mysql/deparse"
	"github.com/bytebase/omni/mysql/parser"
)

// extractExprFromView extracts the expression portion from SHOW CREATE VIEW output.
// MySQL 8.0 SHOW CREATE VIEW returns:
//
//	CREATE ALGORITHM=UNDEFINED DEFINER=`root`@`%` SQL SECURITY DEFINER VIEW `test`.`v` AS select <expr> AS `<alias>` from `test`.`t`
//
// We extract <expr> — the part between "AS select " and the first " AS `".
func extractExprFromView(showCreate string) string {
	idx := strings.Index(showCreate, " AS select ")
	if idx < 0 {
		return showCreate
	}
	selectPart := showCreate[idx+len(" AS select "):]

	// Find " AS `" which marks the column alias
	aliasIdx := strings.Index(selectPart, " AS `")
	if aliasIdx < 0 {
		return selectPart
	}
	return selectPart[:aliasIdx]
}

// deparseExprForOracle parses a SQL expression and deparses it via our deparser.
func deparseExprForOracle(t *testing.T, expr string) string {
	t.Helper()
	sql := "SELECT " + expr + " FROM t"
	stmts, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", sql, err)
	}
	if stmts.Len() == 0 {
		t.Fatalf("no statements parsed from %q", sql)
	}
	sel, ok := stmts.Items[0].(*nodes.SelectStmt)
	if !ok {
		t.Fatalf("expected SelectStmt, got %T", stmts.Items[0])
	}
	if len(sel.TargetList) == 0 {
		t.Fatalf("no target list in SELECT from %q", sql)
	}
	target := sel.TargetList[0]
	if rt, ok := target.(*nodes.ResTarget); ok {
		return deparse.Deparse(rt.Val)
	}
	return deparse.Deparse(target)
}

// TestDeparse_Section_3_2_Oracle verifies TRIM special forms against MySQL 8.0.
func TestDeparse_Section_3_2_Oracle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping oracle test in short mode")
	}
	oracle, cleanup := startOracle(t)
	defer cleanup()

	// Create base table
	oracle.execSQLDirect("CREATE TABLE IF NOT EXISTS t (a VARCHAR(50), b VARCHAR(50))")

	cases := []struct {
		name  string
		input string // expression to test
	}{
		{"trim_simple", "TRIM(a)"},
		{"trim_leading", "TRIM(LEADING 'x' FROM a)"},
		{"trim_trailing", "TRIM(TRAILING 'x' FROM a)"},
		{"trim_both", "TRIM(BOTH 'x' FROM a)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viewName := "v_" + tc.name
			selectSQL := fmt.Sprintf("SELECT %s FROM t", tc.input)

			// Create view on MySQL 8.0
			oracle.execSQLDirect("DROP VIEW IF EXISTS " + viewName)
			createSQL := fmt.Sprintf("CREATE VIEW %s AS %s", viewName, selectSQL)
			if err := oracle.execSQLDirect(createSQL); err != nil {
				t.Fatalf("CREATE VIEW failed: %v", err)
			}

			mysqlOutput, err := oracle.showCreateView(viewName)
			if err != nil {
				t.Fatalf("SHOW CREATE VIEW failed: %v", err)
			}

			// Extract just the expression from MySQL's output
			mysqlExpr := extractExprFromView(mysqlOutput)

			// Our deparser output
			omniExpr := deparseExprForOracle(t, tc.input)

			t.Logf("MySQL:  %s", mysqlExpr)
			t.Logf("Omni:   %s", omniExpr)

			if mysqlExpr != omniExpr {
				t.Errorf("mismatch:\n--- mysql ---\n%s\n--- omni ---\n%s", mysqlExpr, omniExpr)
			}
		})
	}
}
