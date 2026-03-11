# Recursive Descent Parser Implementation Skill - T-SQL

You are implementing a recursive descent T-SQL (SQL Server) parser.

**Working directory:** `/Users/rebeliceyang/Github/omni`
**Parser source:** `tsql/parser/`
**Reference:** Microsoft T-SQL documentation (https://learn.microsoft.com/en-us/sql/t-sql/)
**AST definitions:** `tsql/ast/`
**Tests:** `tsql/parser/compare_test.go`

## Your Task

1. Read `tsql/parser/PROGRESS.json`
2. Pick the next batch to work on:
   - If any batch has `"status": "in_progress"`, **resume that batch** (it was interrupted mid-work -- read the existing code in its target file and continue from where it left off)
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

- Read the T-SQL documentation for the grammar rules listed in the batch
- URL pattern: `https://learn.microsoft.com/en-us/sql/t-sql/statements/{statement}`
  - e.g., `select-transact-sql`, `create-table-transact-sql`, `merge-transact-sql`
- Read the AST node definitions from `tsql/ast/parsenodes.go` (and `node.go`)
- Read the existing parser code in `tsql/parser/` to understand available helpers and already-implemented parse functions

### Step 2: Write Tests FIRST (Test-Driven Development)

**TEST-DRIVEN**: Write tests FIRST, then implement. Every BNF branch must have a test.

Add test cases to `compare_test.go` using SQL strings relevant to the batch's grammar rules.
Add a new `TestParse{BatchName}` function.

### Step 3: Write Parse Functions

Create or update the target file (e.g., `tsql/parser/select.go`).

**Every parse function MUST have this comment format:**

```go
// parseSelectStmt parses a SELECT statement.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/queries/select-transact-sql
//
//	SELECT [ ALL | DISTINCT ]
//	    [ TOP ( expression ) [ PERCENT ] [ WITH TIES ] ]
//	    ...
func (p *Parser) parseSelectStmt() *ast.SelectStmt {
```

**Return type conventions:**
- Expression parsers return `ExprNode` (not `Node`)
- Table reference parsers return `TableExpr`
- Statement parsers return `StmtNode`

**Rules for parse function implementation:**

1. **Use the existing AST types** from `tsql/ast/` -- do NOT create new node types without updating both parsenodes.go and outfuncs.go
1a. **When implementing a new batch, you MUST also add serialization to `outfuncs.go` for any new node types used.** Every node type in parsenodes.go must have a corresponding case in `writeNode` and a `writeXxx` function in outfuncs.go.
2. **Record positions** on EVERY AST node that has a `Loc` field.
   Set `Loc: nodes.Loc{Start: p.pos()}` at the beginning of parsing a node.
   Set `node.Loc.End = p.pos()` at the end of parsing a node.
   **This is a hard requirement.**
3. **Token constants** are in `tsql/parser/lexer.go`
4. **Keywords are case-insensitive** -- the lexer handles this
5. **Error recovery**: When encountering unexpected tokens, try to recover by:
   - Skipping to the next semicolon for statement-level errors
   - Recording errors but continuing to parse subsequent statements

### Step 4: Test

Run these commands in order:

```bash
# Must compile
cd /Users/rebeliceyang/Github/omni && go build ./tsql/...

# Run batch-specific tests
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./tsql/parser/ -run "TestParse{BatchName}"

# Run full test suite (no regressions)
cd /Users/rebeliceyang/Github/omni && go test ./tsql/...
```

### Step 5: Update Progress

Edit `PROGRESS.json`:
- **Before starting work**: set `"status"` to `"in_progress"` so a restart knows this batch was interrupted
- **After all tests pass**: set `"status"` to `"done"`
- **If tests fail and you cannot fix**: set `"status"` to `"failed"` and add `"error": "description"`
- **NEVER mark `"done"` if `go build ./tsql/...` or `go test ./tsql/...` fails**

### Step 6: Git Commit

```bash
../../scripts/git-commit.sh "tsql/parser: implement batch N - name" tsql/
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `tsql/parser/parser.go` | Parser struct, Parse() entry point, helpers |
| `tsql/parser/lexer.go` | Lexer, Token, keywords, token constants |
| `tsql/ast/parsenodes.go` | AST node struct definitions |
| `tsql/ast/node.go` | Node interface, List, String, Integer, etc. |
| `tsql/ast/outfuncs.go` | NodeToString for AST comparison |
| `tsql/parser/compare_test.go` | Test infrastructure |
| `tsql/parser/PROGRESS.json` | Implementation progress tracking |

## T-SQL Specific Notes

- **Bracketed identifiers**: `[column name]` uses square brackets for quoting
- **Variables**: `@local_var`, `@@global_var`
- **N-strings**: `N'unicode string'` for nvarchar literals
- **Comments**: `--` line and `/* */` block (nested), no `#` comments
- **Not-equal**: Both `!=` and `<>` are valid
- **Special operators**: `!<` (not less than), `!>` (not greater than)
- **String concat**: `+` operator (same as addition)
- **Static methods**: `type::Method()` syntax
- **GO**: Batch separator (not actually a SQL statement)
- **TOP**: Uses parentheses: `TOP (10)` (parentheses optional for literals)
- **CROSS APPLY / OUTER APPLY**: T-SQL specific join types
- **OUTPUT clause**: Available on INSERT, UPDATE, DELETE, MERGE
- **MERGE**: Full MERGE statement with WHEN MATCHED/NOT MATCHED/BY SOURCE
- **TRY...CATCH**: Error handling blocks
- **IIF**: T-SQL specific inline IF function
- **CONVERT**: T-SQL specific type conversion with optional style
