# MySQL Recursive Descent Parser Implementation Skill

You are implementing a recursive descent MySQL 8.0 parser.

**Working directory:** `/Users/rebeliceyang/Github/omni`
**Parser source:** `mysql/parser/`
**AST definitions:** `mysql/ast/`
**Reference:** [MySQL 8.0 SQL Statements](https://dev.mysql.com/doc/refman/8.0/en/sql-statements.html)
**Tests:** `mysql/parser/compare_test.go`

## Your Task

1. Read `mysql/parser/PROGRESS.json`
2. Pick the next batch to work on:
   - If any batch has `"status": "in_progress"`, **resume that batch** (it was interrupted mid-work — read the existing code in its target file and continue from where it left off)
   - Otherwise, find the first batch with `"status": "pending"` whose dependencies (by id) are all `"done"`
   - If any batch has `"status": "failed"`, **retry it** (reset to `"in_progress"` and try again)
3. Implement that batch following the steps below
4. Update `PROGRESS.json`:
   - Set `"in_progress"` before starting work
   - Set `"done"` only after `go build` and `go test` pass
   - Set `"failed"` with `"error"` if you cannot make tests pass

If all batches are `"done"`, output `ALL_BATCHES_COMPLETE` and stop.

## Implementation Steps for Each Batch

### Step 1: Read References

- Read the AST node definitions from `mysql/ast/parsenodes.go` (and `node.go`)
- Read the existing parser code in `mysql/parser/` to understand available helpers and already-implemented parse functions
- Read the test cases listed in the batch's `"tests"` list in `mysql/parser/compare_test.go`

### Step 2: Fetch MySQL Documentation BNF

- For each major statement type, search the MySQL 8.0 documentation for its syntax
- URL pattern: `https://dev.mysql.com/doc/refman/8.0/en/sql-statements.html`
  - e.g., `select.html`, `insert.html`, `create-table.html`, `alter-table.html`
- Open-source MySQL grammars (e.g., from MySQL Workbench, TiDB parser, Vitess) can be referenced for understanding grammar structure but **MUST NOT be copied** due to licensing

### Step 3: Write Test Cases FIRST (Test-Driven Development)

**This is a hard requirement. For each batch:**

1. **FIRST** write test cases in `mysql/parser/compare_test.go` covering every BNF rule and branch listed in the batch
2. Every BNF alternative/branch must have at least one test case
3. Include both positive tests (valid SQL that should parse) and negative tests (invalid SQL that should fail)
4. Include edge cases: empty clauses, maximum nesting, unusual but valid syntax

### Step 4: Write Parse Functions

Create or update the target file (e.g., `mysql/parser/select.go`).

**Every parse function MUST have this comment format:**

```go
// parseSelectStmt parses a SELECT statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/select.html
//
//	SELECT
//	    [ALL | DISTINCT | DISTINCTROW]
//	    [HIGH_PRIORITY]
//	    [STRAIGHT_JOIN]
//	    [SQL_CALC_FOUND_ROWS]
//	    select_expr [, select_expr] ...
//	    [FROM table_references]
//	    [WHERE where_condition]
//	    [GROUP BY {col_name | expr | position} [ASC | DESC], ... [WITH ROLLUP]]
//	    [HAVING where_condition]
//	    [WINDOW window_name AS (window_spec) [, window_name AS (window_spec)] ...]
//	    [ORDER BY {col_name | expr | position} [ASC | DESC], ...]
//	    [LIMIT {[offset,] row_count | row_count OFFSET offset}]
//	    [FOR {UPDATE | SHARE} [OF tbl_name [, tbl_name] ...] [NOWAIT | SKIP LOCKED]]
//	    [INTO OUTFILE 'file_name' | INTO DUMPFILE 'file_name' | INTO var_name [, var_name]]
func (p *Parser) parseSelectStmt() *ast.SelectStmt {
```

**Rules for parse function implementation:**

1. **Use the existing AST types** from `mysql/ast/` — do NOT create new node types unless absolutely necessary
2. **Record positions** on EVERY AST node that has a `Location` field.
   Set `Loc: nodes.Loc{Start: p.pos()}` at the beginning of parsing that node, and
   set `node.Loc.End = p.pos()` at the end of parsing that node (after the last token is consumed).
   **This is a hard requirement.** Missing locations will cause features (SQL rewriting, error reporting) to break.
3. **Token constants** are in `mysql/parser/lexer.go` (keyword constants `kw*`, literal tokens `tok*`)
4. **Lexer types** (`Token`, `Lexer`, `NewLexer`) are in `mysql/parser/lexer.go`
5. **Error recovery**: When encountering unexpected tokens, try to recover by:
   - Skipping to the next semicolon for statement-level errors
   - Returning a partial AST node with what was parsed so far
6. **Operator precedence** in expressions: use Pratt parsing (precedence climbing)

### Step 5: Wire Up

- For batch 21 (stmt_dispatch), wire all statement parsers into the `parseStmt()` function
- Handle the `;`-separated list and wrap in `*ast.RawStmt` with position info
- Ensure `Parse()` in `parser.go` calls the top-level dispatch

### Step 6: Test

Run these commands in order:

```bash
# Must compile
cd /Users/rebeliceyang/Github/omni && go build ./mysql/...

# Run batch-specific tests
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./mysql/parser/ -run "TestXxx|TestYyy"

# Run full test suite (no regressions)
cd /Users/rebeliceyang/Github/omni && go test ./mysql/...
```

**Test strategy:**
- `mysql/parser/compare_test.go` contains `ParseAndCheck(t, sql)` which parses SQL and verifies:
  - Parse succeeds for valid SQL
  - AST structure matches expected output (via `ast.NodeToString()`)
  - Location fields are non-negative on all nodes
- For each batch, **add test cases to `compare_test.go`** using SQL strings relevant to the batch's grammar rules
- If a test fails, fix the parser implementation, never the test

### Step 7: Update Progress & Commit

Edit `PROGRESS.json`:
- **Before starting work**: set `"status"` to `"in_progress"`
- **After all tests pass**: set `"status"` to `"done"`
- **If tests fail and you cannot fix**: set `"status"` to `"failed"` and add `"error": "description"`
- **NEVER mark `"done"` if `go build ./mysql/...` or `go test ./mysql/...` fails**

After marking a batch as done, commit:
```bash
../../scripts/git-commit.sh "mysql/parser: implement batch N - name" mysql/
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `mysql/parser/parser.go` | Parser struct, Parse() entry point, helpers |
| `mysql/parser/lexer.go` | Lexer (Token, Lexer, NewLexer), keyword map, token constants |
| `mysql/ast/parsenodes.go` | AST node struct definitions |
| `mysql/ast/node.go` | Node interface, List, String, Integer |
| `mysql/ast/outfuncs.go` | AST serialization (NodeToString) |
| `mysql/parser/compare_test.go` | Test cases for parser validation |
| `mysql/parser/PROGRESS.json` | Batch tracking |

## MySQL-Specific Lexer Notes

The MySQL lexer handles these MySQL-specific constructs:
- **Backtick-quoted identifiers**: `` `table` ``, `` `column name` ``
- **Double-quoted strings**: When ANSI_QUOTES is off (default), `"hello"` is a string literal
- **`#` line comments**: In addition to `--` and `/* */`
- **`!=` operator**: Same as `<>`
- **No `::` typecast**: MySQL uses `CAST()` or `CONVERT()` instead
- **`@variable`**: User variables
- **`@@variable`**: System variables (@@global.var, @@session.var)
- **String charset introducer**: `_utf8 'hello'`, `_latin1 'hello'`
- **Hex literals**: `0xFF`, `X'FF'`
- **Bit literals**: `0b101`, `b'101'`
- **`<=>` operator**: NULL-safe equality
- **`DIV` operator**: Integer division
- **`:=` operator**: Assignment in SET and SELECT

## Expression Parsing Strategy

For expressions, use **Pratt parsing / precedence climbing**:

```go
func (p *Parser) parseExpr(minPrec int) nodes.ExprNode {
    left := p.parsePrimary() // atoms: literals, column refs, subqueries, func calls, etc.
    for {
        prec, ok := p.infixPrecedence(p.cur.Type)
        if !ok || prec < minPrec {
            break
        }
        op := p.advance()
        right := p.parseExpr(prec + 1) // +1 for left-assoc
        left = &nodes.BinaryExpr{...}
    }
    return left
}
```

**Return type rules for parse functions:**
1. Expression parse functions (e.g., `parseExpr`, `parsePrimary`) should return `nodes.ExprNode`
2. Table reference parse functions (e.g., `parseTableRef`, `parseJoinClause`) should return `nodes.TableExpr`
3. Statement parse functions (e.g., `parseSelectStmt`, `parseInsertStmt`) should return `nodes.StmtNode`
4. When implementing a new batch, you MUST also add serialization for any new node types to `outfuncs.go` (add a `writeNode` switch case and a corresponding `writeXxx` function for each new struct)

Precedence levels (low to high, matching MySQL):
1. `OR`, `||`
2. `XOR`
3. `AND`, `&&`
4. `NOT` (prefix)
5. `BETWEEN`, `CASE`, `WHEN`, `THEN`, `ELSE`
6. `=`, `<=>`, `>=`, `>`, `<=`, `<`, `<>`, `!=`, `IS`, `LIKE`, `REGEXP`, `IN`
7. `|`
8. `&`
9. `<<`, `>>`
10. `-`, `+`
11. `*`, `/`, `DIV`, `%`, `MOD`
12. `^`
13. Unary `-`, `~`
14. `!`
15. `BINARY`, `COLLATE`
16. `INTERVAL`

## Important Constraints

- Do NOT modify any files outside `mysql/`
- The `go.mod` at `/Users/rebeliceyang/Github/omni/go.mod` already exists
- Run `gofmt -w` on all created/modified files
- Use random sleep (1-3s) before git operations to avoid lock contention with other engine pipelines
