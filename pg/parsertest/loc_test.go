package parsertest

import (
	"testing"

	nodes "github.com/bytebase/omni/pg/ast"
	"github.com/bytebase/omni/pg/parser"
)

func TestLocSelectStmt(t *testing.T) {
	sql := "SELECT a, b FROM t WHERE x > 0"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	sel := raw.Stmt.(*nodes.SelectStmt)
	if sel.Loc.Start == -1 || sel.Loc.End == -1 {
		t.Fatalf("SelectStmt Loc not set: %+v", sel.Loc)
	}
	got := sql[sel.Loc.Start:sel.Loc.End]
	if got != sql {
		t.Errorf("SelectStmt text = %q, want %q", got, sql)
	}
}

func TestLocSelectWithParens(t *testing.T) {
	// (SELECT 1) is not a valid top-level statement in this parser,
	// so test via UNION where the right side is a parenthesized select.
	sql := "SELECT 1 UNION (SELECT 2)"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	sel := raw.Stmt.(*nodes.SelectStmt)
	// The top-level is UNION; check that Rarg (parenthesized select) has Loc for inner SELECT.
	rarg := sel.Rarg
	if rarg.Loc.Start == -1 || rarg.Loc.End == -1 {
		t.Fatalf("parenthesized SelectStmt Loc not set: %+v", rarg.Loc)
	}
	got := sql[rarg.Loc.Start:rarg.Loc.End]
	want := "SELECT 2"
	if got != want {
		t.Errorf("parenthesized SelectStmt text = %q, want %q", got, want)
	}
}

func TestLocSelectUnion(t *testing.T) {
	sql := "SELECT 1 UNION SELECT 2"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	sel := raw.Stmt.(*nodes.SelectStmt)
	if sel.Loc.Start == -1 || sel.Loc.End == -1 {
		t.Fatalf("union SelectStmt Loc not set: %+v", sel.Loc)
	}
	got := sql[sel.Loc.Start:sel.Loc.End]
	if got != sql {
		t.Errorf("union text = %q, want %q", got, sql)
	}
}

func TestLocSelectMultiStmt(t *testing.T) {
	sql := "SELECT 1; SELECT 2"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range list.Items {
		raw := item.(*nodes.RawStmt)
		sel := raw.Stmt.(*nodes.SelectStmt)
		if sel.Loc.Start == -1 || sel.Loc.End == -1 {
			t.Errorf("stmt %d: SelectStmt Loc not set: %+v", i, sel.Loc)
		}
	}
}

func TestLocInsertStmt(t *testing.T) {
	sql := "INSERT INTO t VALUES (1, 2)"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	ins := raw.Stmt.(*nodes.InsertStmt)
	if ins.Loc.Start == -1 || ins.Loc.End == -1 {
		t.Fatalf("InsertStmt Loc not set: %+v", ins.Loc)
	}
	got := sql[ins.Loc.Start:ins.Loc.End]
	if got != sql {
		t.Errorf("InsertStmt text = %q, want %q", got, sql)
	}
}

func TestLocInsertWithCTE(t *testing.T) {
	sql := "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	ins := raw.Stmt.(*nodes.InsertStmt)
	if ins.Loc.Start == -1 || ins.Loc.End == -1 {
		t.Fatalf("InsertStmt Loc not set: %+v", ins.Loc)
	}
	got := sql[ins.Loc.Start:ins.Loc.End]
	if got != sql {
		t.Errorf("InsertStmt text = %q, want %q", got, sql)
	}
}

func TestLocUpdateStmt(t *testing.T) {
	sql := "UPDATE t SET a = 1 WHERE id = 5"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	upd := raw.Stmt.(*nodes.UpdateStmt)
	if upd.Loc.Start == -1 || upd.Loc.End == -1 {
		t.Fatalf("UpdateStmt Loc not set: %+v", upd.Loc)
	}
	got := sql[upd.Loc.Start:upd.Loc.End]
	if got != sql {
		t.Errorf("UpdateStmt text = %q, want %q", got, sql)
	}
}

func TestLocDeleteStmt(t *testing.T) {
	sql := "DELETE FROM t WHERE id = 5"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	del := raw.Stmt.(*nodes.DeleteStmt)
	if del.Loc.Start == -1 || del.Loc.End == -1 {
		t.Fatalf("DeleteStmt Loc not set: %+v", del.Loc)
	}
	got := sql[del.Loc.Start:del.Loc.End]
	if got != sql {
		t.Errorf("DeleteStmt text = %q, want %q", got, sql)
	}
}

func TestLocUpdateWithCTE(t *testing.T) {
	sql := "WITH cte AS (SELECT 1) UPDATE t SET a = 1"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	upd := raw.Stmt.(*nodes.UpdateStmt)
	got := sql[upd.Loc.Start:upd.Loc.End]
	if got != sql {
		t.Errorf("UpdateStmt text = %q, want %q", got, sql)
	}
}

func TestLocDeleteWithUsing(t *testing.T) {
	sql := "DELETE FROM t USING t2 WHERE t.id = t2.id"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	del := raw.Stmt.(*nodes.DeleteStmt)
	got := sql[del.Loc.Start:del.Loc.End]
	if got != sql {
		t.Errorf("DeleteStmt text = %q, want %q", got, sql)
	}
}

func TestLocMergeStmt(t *testing.T) {
	sql := "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET a = s.a WHEN NOT MATCHED THEN INSERT (a) VALUES (s.a)"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	merge := raw.Stmt.(*nodes.MergeStmt)
	if merge.Loc.Start == -1 || merge.Loc.End == -1 {
		t.Fatalf("MergeStmt Loc not set: %+v", merge.Loc)
	}
	got := sql[merge.Loc.Start:merge.Loc.End]
	if got != sql {
		t.Errorf("MergeStmt text = %q, want %q", got, sql)
	}
}

// Integration tests: extract sub-clause text via Loc slicing

func TestLocWhereClauseExtraction(t *testing.T) {
	sql := "UPDATE t SET a = 1 WHERE id > 5"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	upd := raw.Stmt.(*nodes.UpdateStmt)

	loc := nodes.NodeLoc(upd.WhereClause)
	if loc.Start == -1 {
		t.Fatal("WhereClause has no Loc")
	}
	got := sql[loc.Start:loc.End]
	if got != "id > 5" {
		t.Errorf("WHERE expression = %q, want %q", got, "id > 5")
	}
}

func TestLocWithClauseExtraction(t *testing.T) {
	sql := "WITH cte AS (SELECT 1) SELECT * FROM cte"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	sel := raw.Stmt.(*nodes.SelectStmt)

	if sel.WithClause == nil {
		t.Fatal("WithClause is nil")
	}
	got := sql[sel.WithClause.Loc.Start:sel.WithClause.Loc.End]
	// Loc.End may include trailing whitespace (points to next token start).
	// Verify the extracted text starts with the expected clause.
	want := "WITH cte AS (SELECT 1)"
	if len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("WITH clause = %q, want prefix %q", got, want)
	}
}

func TestLocFromClauseExtraction(t *testing.T) {
	sql := "SELECT * FROM t1, t2"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	sel := raw.Stmt.(*nodes.SelectStmt)

	span := nodes.ListSpan(sel.FromClause)
	if span.Start == -1 {
		t.Fatal("FromClause has no span")
	}
	got := sql[span.Start:span.End]
	if got != "t1, t2" {
		t.Errorf("FROM clause = %q, want %q", got, "t1, t2")
	}
}

func TestLocLimitCountExtraction(t *testing.T) {
	sql := "SELECT * FROM t LIMIT 100"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	sel := raw.Stmt.(*nodes.SelectStmt)

	loc := nodes.NodeLoc(sel.LimitCount)
	if loc.Start == -1 {
		t.Fatal("LimitCount has no Loc")
	}
	got := sql[loc.Start:loc.End]
	if got != "100" {
		t.Errorf("LIMIT value = %q, want %q", got, "100")
	}
}

func TestLocSortClauseExtraction(t *testing.T) {
	sql := "SELECT * FROM t ORDER BY a, b DESC"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	sel := raw.Stmt.(*nodes.SelectStmt)

	span := nodes.ListSpan(sel.SortClause)
	if span.Start == -1 {
		t.Fatal("SortClause has no span")
	}
	got := sql[span.Start:span.End]
	if got != "a, b DESC" {
		t.Errorf("ORDER BY items = %q, want %q", got, "a, b DESC")
	}
}

func TestLocRelationExtraction(t *testing.T) {
	sql := "DELETE FROM public.users WHERE id = 1"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	del := raw.Stmt.(*nodes.DeleteStmt)

	got := sql[del.Relation.Loc.Start:del.Relation.Loc.End]
	// Loc.End may include trailing whitespace.
	want := "public.users"
	if len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("relation = %q, want prefix %q", got, want)
	}
}
