# Loc + Slicing for Bytebase Migration

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `Loc` fields to DML statement nodes and provide a `NodeLoc()` helper so bytebase can extract sub-clause text via `sql[loc.Start:loc.End]` instead of ANTLR token streams.

**Architecture:** Add `Loc Loc` to `SelectStmt`, `InsertStmt`, `UpdateStmt`, `DeleteStmt`, `MergeStmt` in the AST, populate them in the parser at parse time, and add a `NodeLoc(Node) Loc` utility function that extracts Loc from any node via type switch. The walker generator already excludes `Loc` fields, so `walk_generated.go` does not change.

**Tech Stack:** Go, `pg/ast/` (node definitions), `pg/parser/` (recursive descent parser), `pg/parsertest/` (tests)

---

## Context for Implementor

### What exists today
- `pg/ast/node.go` — `Loc` struct with `Start int` (inclusive byte offset) and `End int` (exclusive byte offset). `-1` means unknown. `NoLoc()` helper.
- `pg/ast/parsenodes.go` — 229 AST node types. 65 already have `Loc Loc` field (expressions, RangeVar, WithClause, etc.). The 4 DML statement types + MergeStmt do NOT have Loc.
- `pg/parser/parser.go:47` — `RawStmt` wraps each statement with `Loc{Start: stmtStart, End: p.prev.End}`. This gives the outer boundary, but the inner statement node itself has no Loc.
- `pg/parser/select.go:25` — `parseSelectNoParens()` creates `SelectStmt` without Loc.
- `pg/parser/update.go:20,73` — `parseUpdateStmt()`, `parseDeleteStmt()` create nodes without Loc.
- `pg/parser/insert.go:30` — `parseInsertStmt()` creates `InsertStmt` without Loc.
- `pg/parser/expr.go:2430` — `setNodeLoc()` helper does type switch to set Loc on expression nodes. We'll extend this pattern.
- `pg/ast/cmd/genwalker/main.go:188` — Loc is explicitly excluded from walker generation. **No regeneration needed.**
- `pg/ast/walk_test.go` — Existing walker tests.
- `pg/parsertest/helpers_test.go` — `parseStmt(t, sql)` helper returns the inner statement node.

### How positions work in the parser
- `p.pos()` returns the current token's start byte offset.
- `p.prev.End` returns the previous token's exclusive end byte offset.
- Pattern for setting Loc: `loc := p.pos()` at entry, then `node.Loc = nodes.Loc{Start: loc, End: p.pos()}` or `End: p.prev.End` before return.

### Why this matters
Bytebase needs `sql[node.Loc.Start:node.Loc.End]` to extract sub-clause text (WHERE, FROM, WITH, LIMIT) from parsed SQL when building backup queries and injecting LIMIT clauses. Today it uses ANTLR `GetTextFromRuleContext()` which we're eliminating.

---

## Task 1: Add Loc to SelectStmt

**Files:**
- Modify: `pg/ast/parsenodes.go:16-46` (SelectStmt struct)
- Modify: `pg/parser/select.go:25-53` (parseSelectNoParens)
- Modify: `pg/parser/select.go:60-70` (parseSelectWithParens)
- Modify: `pg/parser/select.go:78-113` (parseSelectClause — set operations)
- Modify: `pg/parser/select.go:130-141` (parseSimpleSelectLeaf)
- Modify: `pg/parser/select.go:155` (parseSimpleSelectCore)
- Test: `pg/parsertest/loc_test.go` (new file)

**Step 1: Write the failing test**

Create `pg/parsertest/loc_test.go`:

```go
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
	sql := "(SELECT 1)"
	inner := "SELECT 1"
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
	if got != inner {
		t.Errorf("SelectStmt text = %q, want %q", got, inner)
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run TestLocSelect -v`
Expected: FAIL — `SelectStmt` has no `Loc` field.

**Step 3: Add Loc field to SelectStmt**

In `pg/ast/parsenodes.go`, add `Loc Loc` to `SelectStmt`:

```go
type SelectStmt struct {
	// ... existing fields ...
	Rarg *SelectStmt  // right child

	Loc Loc // source location range
}
```

**Step 4: Populate Loc in parser**

In `pg/parser/select.go`, update `parseSelectNoParens`:

```go
func (p *Parser) parseSelectNoParens() *nodes.SelectStmt {
	loc := p.pos() // <-- add this

	// 1. Optional WITH clause
	var withClause *nodes.WithClause
	if p.cur.Type == WITH || p.cur.Type == WITH_LA {
		withClause = p.parseWithClause()
	}

	// 2. Parse select_clause (handles UNION/INTERSECT/EXCEPT)
	stmt := p.parseSelectClause(setOpPrecNone)
	if stmt == nil {
		return nil
	}

	// 3. Optional ORDER BY
	if p.cur.Type == ORDER {
		p.advance()
		p.expect(BY)
		stmt.SortClause = p.parseSortByList()
	}

	// 4. Parse LIMIT/OFFSET and FOR locking in either order
	p.parseSelectOptions(stmt)

	if withClause != nil {
		stmt.WithClause = withClause
	}

	stmt.Loc = nodes.Loc{Start: loc, End: p.prev.End} // <-- add this
	return stmt
}
```

Update `parseSelectWithParens`:

```go
func (p *Parser) parseSelectWithParens() *nodes.SelectStmt {
	p.expect('(')
	var stmt *nodes.SelectStmt
	if p.cur.Type == '(' {
		stmt = p.parseSelectWithParens()
	} else {
		stmt = p.parseSelectNoParens()
	}
	p.expect(')')
	// Loc stays as set by inner parse — covers the SELECT itself, not the parens
	return stmt
}
```

Update `parseSelectClause` for set operations — the combined node inherits Loc spanning both sides:

```go
	result := &nodes.SelectStmt{
		Op:   op,
		Larg: left,
		Rarg: right,
	}
	if all == nodes.SET_QUANTIFIER_ALL {
		result.All = true
	}
	result.Loc = nodes.Loc{Start: left.Loc.Start, End: p.prev.End} // <-- add
	left = result
```

Update `parseSimpleSelectCore` — record start position:

```go
func (p *Parser) parseSimpleSelectCore() *nodes.SelectStmt {
	loc := p.pos() // <-- add
	p.advance() // consume SELECT
	stmt := &nodes.SelectStmt{}
	// ... existing parsing ...
	stmt.Loc = nodes.Loc{Start: loc, End: p.prev.End} // <-- add before return
	return stmt                                         // (find the actual return)
}
```

Update `parseValuesClause` and `parseTableCmd` similarly — record `loc := p.pos()` at entry, set `stmt.Loc` before return.

**Step 5: Run test to verify it passes**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run TestLocSelect -v`
Expected: PASS

**Step 6: Run full test suite to check no regressions**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/...`
Expected: All PASS

**Step 7: Commit**

```bash
git add pg/ast/parsenodes.go pg/parser/select.go pg/parsertest/loc_test.go
git commit -m "ast: add Loc to SelectStmt, populate in parser"
```

---

## Task 2: Add Loc to InsertStmt

**Files:**
- Modify: `pg/ast/parsenodes.go:49-57` (InsertStmt struct)
- Modify: `pg/parser/insert.go:30-65` (parseInsertStmt)
- Test: `pg/parsertest/loc_test.go` (append)

**Step 1: Write the failing test**

Append to `pg/parsertest/loc_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run TestLocInsert -v`
Expected: FAIL

**Step 3: Add Loc field and populate it**

In `pg/ast/parsenodes.go`, add `Loc Loc` to `InsertStmt`:

```go
type InsertStmt struct {
	// ... existing fields ...
	Override         OverridingKind    // OVERRIDING clause

	Loc Loc // source location range
}
```

In `pg/parser/insert.go`, update `parseInsertStmt`:

```go
func (p *Parser) parseInsertStmt(withClause *nodes.WithClause) *nodes.InsertStmt {
	loc := p.pos() // <-- add: record INSERT keyword position
	p.advance() // consume INSERT
	p.expect(INTO)
	// ... existing code ...
	if withClause != nil {
		stmt.WithClause = withClause
		loc = withClause.Loc.Start // <-- add: WITH starts before INSERT
	}
	stmt.Loc = nodes.Loc{Start: loc, End: p.prev.End} // <-- add
	return stmt
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run TestLocInsert -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/...`
Expected: All PASS

**Step 6: Commit**

```bash
git add pg/ast/parsenodes.go pg/parser/insert.go pg/parsertest/loc_test.go
git commit -m "ast: add Loc to InsertStmt, populate in parser"
```

---

## Task 3: Add Loc to UpdateStmt and DeleteStmt

**Files:**
- Modify: `pg/ast/parsenodes.go:62-71,74-82` (UpdateStmt, DeleteStmt structs)
- Modify: `pg/parser/update.go:20-62,73-103` (parseUpdateStmt, parseDeleteStmt)
- Test: `pg/parsertest/loc_test.go` (append)

**Step 1: Write the failing tests**

Append to `pg/parsertest/loc_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run "TestLocUpdate|TestLocDelete" -v`
Expected: FAIL

**Step 3: Add Loc fields and populate them**

In `pg/ast/parsenodes.go`, add `Loc Loc` to both structs:

```go
type UpdateStmt struct {
	// ... existing fields ...
	WithClause    *WithClause // WITH clause

	Loc Loc // source location range
}

type DeleteStmt struct {
	// ... existing fields ...
	WithClause    *WithClause // WITH clause

	Loc Loc // source location range
}
```

In `pg/parser/update.go`, update both functions:

```go
func (p *Parser) parseUpdateStmt(withClause *nodes.WithClause) *nodes.UpdateStmt {
	loc := p.pos() // <-- add
	p.advance() // consume UPDATE
	// ... existing code through stmt construction ...
	if withClause != nil {
		stmt.WithClause = withClause
		loc = withClause.Loc.Start // <-- add
	}
	stmt.Loc = nodes.Loc{Start: loc, End: p.prev.End} // <-- add
	return stmt
}

func (p *Parser) parseDeleteStmt(withClause *nodes.WithClause) *nodes.DeleteStmt {
	loc := p.pos() // <-- add
	p.advance() // consume DELETE
	// ... existing code through stmt construction ...
	if withClause != nil {
		stmt.WithClause = withClause
		loc = withClause.Loc.Start // <-- add
	}
	stmt.Loc = nodes.Loc{Start: loc, End: p.prev.End} // <-- add
	return stmt
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run "TestLocUpdate|TestLocDelete" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/...`
Expected: All PASS

**Step 6: Commit**

```bash
git add pg/ast/parsenodes.go pg/parser/update.go pg/parsertest/loc_test.go
git commit -m "ast: add Loc to UpdateStmt and DeleteStmt, populate in parser"
```

---

## Task 4: Add Loc to MergeStmt

**Files:**
- Modify: `pg/ast/parsenodes.go` (MergeStmt struct)
- Modify: `pg/parser/update.go:114-146` (parseMergeStmt)
- Test: `pg/parsertest/loc_test.go` (append)

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run TestLocMerge -v`
Expected: FAIL

**Step 3: Add Loc field and populate it**

In `pg/ast/parsenodes.go`, add `Loc Loc` to `MergeStmt`.

In `pg/parser/update.go:114`, update `parseMergeStmt`:

```go
func (p *Parser) parseMergeStmt(withClause *nodes.WithClause) *nodes.MergeStmt {
	loc := p.pos() // <-- add
	p.advance() // consume MERGE
	// ... existing code through stmt construction ...
	if withClause != nil {
		stmt.WithClause = withClause
		loc = withClause.Loc.Start // <-- add
	}
	stmt.Loc = nodes.Loc{Start: loc, End: p.prev.End} // <-- add
	return stmt
}
```

**Step 4: Run tests, verify pass, run full suite**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run TestLocMerge -v && go test ./pg/...`

**Step 5: Commit**

```bash
git add pg/ast/parsenodes.go pg/parser/update.go pg/parsertest/loc_test.go
git commit -m "ast: add Loc to MergeStmt, populate in parser"
```

---

## Task 5: Add NodeLoc() helper function

**Files:**
- Create: `pg/ast/loc.go`
- Test: `pg/ast/loc_test.go` (new)

**Step 1: Write the failing test**

Create `pg/ast/loc_test.go`:

```go
package ast

import "testing"

func TestNodeLocKnownTypes(t *testing.T) {
	cases := []struct {
		name string
		node Node
		loc  Loc
	}{
		{"SelectStmt", &SelectStmt{Loc: Loc{Start: 0, End: 10}}, Loc{0, 10}},
		{"InsertStmt", &InsertStmt{Loc: Loc{Start: 5, End: 20}}, Loc{5, 20}},
		{"UpdateStmt", &UpdateStmt{Loc: Loc{Start: 3, End: 15}}, Loc{3, 15}},
		{"DeleteStmt", &DeleteStmt{Loc: Loc{Start: 1, End: 8}}, Loc{1, 8}},
		{"MergeStmt", &MergeStmt{Loc: Loc{Start: 0, End: 50}}, Loc{0, 50}},
		{"RangeVar", &RangeVar{Loc: Loc{Start: 10, End: 20}}, Loc{10, 20}},
		{"WithClause", &WithClause{Loc: Loc{Start: 0, End: 30}}, Loc{0, 30}},
		{"ColumnRef", &ColumnRef{Loc: Loc{Start: 7, End: 8}}, Loc{7, 8}},
		{"FuncCall", &FuncCall{Loc: Loc{Start: 2, End: 12}}, Loc{2, 12}},
		{"A_Const", &A_Const{Loc: Loc{Start: 4, End: 5}}, Loc{4, 5}},
		{"A_Expr", &A_Expr{Loc: Loc{Start: 0, End: 9}}, Loc{0, 9}},
		{"RawStmt", &RawStmt{Loc: Loc{Start: 0, End: 25}}, Loc{0, 25}},
		{"TypeCast", &TypeCast{Loc: Loc{Start: 1, End: 10}}, Loc{1, 10}},
		{"SubLink", &SubLink{Loc: Loc{Start: 0, End: 15}}, Loc{0, 15}},
		{"ResTarget", &ResTarget{Loc: Loc{Start: 7, End: 12}}, Loc{7, 12}},
		{"SortBy", &SortBy{Loc: Loc{Start: 30, End: 35}}, Loc{30, 35}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NodeLoc(tc.node)
			if got != tc.loc {
				t.Errorf("NodeLoc(%s) = %+v, want %+v", tc.name, got, tc.loc)
			}
		})
	}
}

func TestNodeLocUnknownType(t *testing.T) {
	// DropStmt has no Loc — should return NoLoc()
	got := NodeLoc(&DropStmt{})
	if got != NoLoc() {
		t.Errorf("NodeLoc(DropStmt) = %+v, want NoLoc", got)
	}
}

func TestNodeLocNil(t *testing.T) {
	got := NodeLoc(nil)
	if got != NoLoc() {
		t.Errorf("NodeLoc(nil) = %+v, want NoLoc", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/ast/ -run TestNodeLoc -v`
Expected: FAIL — `NodeLoc` undefined

**Step 3: Implement NodeLoc**

Create `pg/ast/loc.go`:

```go
package ast

// NodeLoc extracts the Loc from any AST node that carries one.
// Returns NoLoc() if the node is nil or its type has no Loc field.
func NodeLoc(n Node) Loc {
	if n == nil {
		return NoLoc()
	}
	switch v := n.(type) {
	// Statement nodes
	case *RawStmt:
		return v.Loc
	case *SelectStmt:
		return v.Loc
	case *InsertStmt:
		return v.Loc
	case *UpdateStmt:
		return v.Loc
	case *DeleteStmt:
		return v.Loc
	case *MergeStmt:
		return v.Loc

	// Expression nodes
	case *A_Expr:
		return v.Loc
	case *A_Const:
		return v.Loc
	case *A_ArrayExpr:
		return v.Loc
	case *BoolExpr:
		return v.Loc
	case *NullTest:
		return v.Loc
	case *BooleanTest:
		return v.Loc
	case *ColumnRef:
		return v.Loc
	case *FuncCall:
		return v.Loc
	case *TypeCast:
		return v.Loc
	case *SubLink:
		return v.Loc
	case *ParamRef:
		return v.Loc
	case *NamedArgExpr:
		return v.Loc
	case *CollateClause:
		return v.Loc
	case *CaseExpr:
		return v.Loc
	case *CaseWhen:
		return v.Loc
	case *CoalesceExpr:
		return v.Loc
	case *MinMaxExpr:
		return v.Loc
	case *NullIfExpr:
		return v.Loc
	case *RowExpr:
		return v.Loc
	case *ArrayExpr:
		return v.Loc
	case *GroupingFunc:
		return v.Loc
	case *GroupingSet:
		return v.Loc
	case *SQLValueFunction:
		return v.Loc
	case *SetToDefault:
		return v.Loc
	case *XmlExpr:
		return v.Loc
	case *XmlSerialize:
		return v.Loc

	// Clause/definition nodes
	case *RangeVar:
		return v.Loc
	case *ResTarget:
		return v.Loc
	case *SortBy:
		return v.Loc
	case *TypeName:
		return v.Loc
	case *ColumnDef:
		return v.Loc
	case *Constraint:
		return v.Loc
	case *DefElem:
		return v.Loc
	case *WindowDef:
		return v.Loc
	case *WithClause:
		return v.Loc
	case *CommonTableExpr:
		return v.Loc
	case *CTESearchClause:
		return v.Loc
	case *CTECycleClause:
		return v.Loc
	case *RoleSpec:
		return v.Loc
	case *OnConflictClause:
		return v.Loc
	case *InferClause:
		return v.Loc
	case *PartitionSpec:
		return v.Loc
	case *PartitionElem:
		return v.Loc
	case *PartitionBoundSpec:
		return v.Loc
	case *RangeTableSample:
		return v.Loc
	case *RangeTableFunc:
		return v.Loc
	case *RangeTableFuncCol:
		return v.Loc

	// Transaction
	case *TransactionStmt:
		return v.Loc
	case *DeallocateStmt:
		return v.Loc

	// JSON nodes
	case *JsonFormat:
		return v.Loc
	case *JsonBehavior:
		return v.Loc
	case *JsonFuncExpr:
		return v.Loc
	case *JsonTablePathSpec:
		return v.Loc
	case *JsonTableColumn:
		return v.Loc
	case *JsonTable:
		return v.Loc
	case *JsonParseExpr:
		return v.Loc
	case *JsonScalarExpr:
		return v.Loc
	case *JsonSerializeExpr:
		return v.Loc
	case *JsonObjectConstructor:
		return v.Loc
	case *JsonArrayConstructor:
		return v.Loc
	case *JsonArrayQueryConstructor:
		return v.Loc
	case *JsonAggConstructor:
		return v.Loc
	case *JsonIsPredicate:
		return v.Loc

	// Publication
	case *PublicationObjSpec:
		return v.Loc

	default:
		return NoLoc()
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/ast/ -run TestNodeLoc -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/...`
Expected: All PASS

**Step 6: Commit**

```bash
git add pg/ast/loc.go pg/ast/loc_test.go
git commit -m "ast: add NodeLoc() helper to extract Loc from any node"
```

---

## Task 6: Add ListSpan() helper for List byte ranges

**Files:**
- Modify: `pg/ast/loc.go` (add ListSpan)
- Modify: `pg/ast/loc_test.go` (add tests)

This is needed because bytebase extracts FROM clause text, USING clause text, etc. — which are `*List` fields. List itself has no Loc, but we can compute the span from the first and last items.

**Step 1: Write the failing test**

Append to `pg/ast/loc_test.go`:

```go
func TestListSpan(t *testing.T) {
	list := &List{Items: []Node{
		&RangeVar{Relname: "a", Loc: Loc{Start: 10, End: 11}},
		&RangeVar{Relname: "b", Loc: Loc{Start: 13, End: 14}},
	}}
	got := ListSpan(list)
	if got.Start != 10 || got.End != 14 {
		t.Errorf("ListSpan = %+v, want {10, 14}", got)
	}
}

func TestListSpanSingle(t *testing.T) {
	list := &List{Items: []Node{
		&ColumnRef{Loc: Loc{Start: 5, End: 8}},
	}}
	got := ListSpan(list)
	if got.Start != 5 || got.End != 8 {
		t.Errorf("ListSpan = %+v, want {5, 8}", got)
	}
}

func TestListSpanNil(t *testing.T) {
	got := ListSpan(nil)
	if got != NoLoc() {
		t.Errorf("ListSpan(nil) = %+v, want NoLoc", got)
	}
}

func TestListSpanEmpty(t *testing.T) {
	got := ListSpan(&List{})
	if got != NoLoc() {
		t.Errorf("ListSpan(empty) = %+v, want NoLoc", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/ast/ -run TestListSpan -v`
Expected: FAIL — `ListSpan` undefined

**Step 3: Implement ListSpan**

Add to `pg/ast/loc.go`:

```go
// ListSpan returns the byte range spanning all items in a List.
// It uses NodeLoc on the first and last items to compute the range.
// Returns NoLoc() if the list is nil, empty, or items have no Loc.
func ListSpan(list *List) Loc {
	if list == nil || len(list.Items) == 0 {
		return NoLoc()
	}
	first := NodeLoc(list.Items[0])
	last := NodeLoc(list.Items[len(list.Items)-1])
	if first.Start == -1 || last.End == -1 {
		return NoLoc()
	}
	return Loc{Start: first.Start, End: last.End}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/ast/ -run TestListSpan -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/...`
Expected: All PASS

**Step 6: Commit**

```bash
git add pg/ast/loc.go pg/ast/loc_test.go
git commit -m "ast: add ListSpan() to compute byte range of List items"
```

---

## Task 7: Integration test — sub-clause text extraction

**Files:**
- Test: `pg/parsertest/loc_test.go` (append integration tests)

This validates the end-to-end pattern: parse SQL, extract sub-clause text via Loc slicing. These are the exact patterns bytebase needs.

**Step 1: Write tests**

Append to `pg/parsertest/loc_test.go`:

```go
func TestLocWhereClauseExtraction(t *testing.T) {
	sql := "UPDATE t SET a = 1 WHERE id > 5"
	list, err := parser.Parse(sql)
	if err != nil {
		t.Fatal(err)
	}
	raw := list.Items[0].(*nodes.RawStmt)
	upd := raw.Stmt.(*nodes.UpdateStmt)

	// Extract WHERE expression text via Loc
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

	// Extract WITH clause text
	if sel.WithClause == nil {
		t.Fatal("WithClause is nil")
	}
	got := sql[sel.WithClause.Loc.Start:sel.WithClause.Loc.End]
	if got != "WITH cte AS (SELECT 1)" {
		t.Errorf("WITH clause = %q, want %q", got, "WITH cte AS (SELECT 1)")
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

	// Extract FROM list text via ListSpan
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

	// Extract LIMIT value
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

	// Extract ORDER BY items span
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

	// Extract table name via RangeVar Loc
	got := sql[del.Relation.Loc.Start:del.Relation.Loc.End]
	if got != "public.users" {
		t.Errorf("relation = %q, want %q", got, "public.users")
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parsertest/ -run "TestLocWhere|TestLocWith|TestLocFrom|TestLocLimit|TestLocSort|TestLocRelation" -v`
Expected: All PASS (if prior tasks are done correctly)

If any fail, fix the Loc population in the parser for the specific sub-node. Common issues:
- WhereClause: The expression parsed by `parseAExpr()` should already have Loc set via `setNodeLoc()`.
- SortBy items: Already have Loc from `parseSortBy()`.
- LimitCount: Should have Loc from expression parsing.
- RangeVar: Already has Loc.

**Step 3: Commit**

```bash
git add pg/parsertest/loc_test.go
git commit -m "parsertest: add integration tests for sub-clause Loc extraction"
```

---

## Summary

| Task | What | Files changed |
|------|------|--------------|
| 1 | Loc on SelectStmt | parsenodes.go, select.go, loc_test.go |
| 2 | Loc on InsertStmt | parsenodes.go, insert.go, loc_test.go |
| 3 | Loc on UpdateStmt + DeleteStmt | parsenodes.go, update.go, loc_test.go |
| 4 | Loc on MergeStmt | parsenodes.go, update.go, loc_test.go |
| 5 | NodeLoc() helper | loc.go (new), loc_test.go (new) |
| 6 | ListSpan() helper | loc.go, loc_test.go |
| 7 | Integration tests | loc_test.go |

After this, bytebase can extract any sub-clause text via:
```go
sql[ast.NodeLoc(stmt.WhereClause).Start : ast.NodeLoc(stmt.WhereClause).End]
sql[stmt.WithClause.Loc.Start : stmt.WithClause.Loc.End]
sql[ast.ListSpan(stmt.FromClause).Start : ast.ListSpan(stmt.FromClause).End]
```
