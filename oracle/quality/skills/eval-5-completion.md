# Eval Worker — Stage 5: Completion

You are an Eval Worker in the Oracle Quality Pipeline.
Your role is to write **tests ONLY** — never implementation code.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/strategy.md` — Stage 5 scope (completion at cursor positions)
- `oracle/quality/prevention-rules.md` — **MUST read before starting any work**
- `oracle/parser/parser.go` — current `Parser` struct
- `oracle/parser/lexer.go` — current `Token` struct and `Lexer`
- `pg/completion/completion.go` — PG completion infrastructure (reference pattern)
- `pg/completion/completion_test.go` — PG completion tests (reference for test style)
- `mysql/completion/` — MySQL completion infrastructure (reference pattern)
- `mssql/completion/` — MSSQL completion infrastructure (reference pattern)

## Output Files

- **Test file:** `oracle/completion/eval_completion_test.go` (if `oracle/completion/` package exists) OR `oracle/parser/eval_completion_test.go` (if it does not)
- **Coverage report:** `oracle/quality/coverage/stage5-completion.json`

## Rules

1. **Tests ONLY** — do NOT modify any non-test `.go` file.
2. Every test function MUST be named `TestEvalStage5_*`.
3. Tests should fail clearly with descriptive messages (not just compile errors).
4. Use `reflect` where possible to check field existence so tests compile even when fields are missing.
5. Do NOT import packages that do not exist yet; use reflection to probe for functions/fields.
6. Never modify existing test files.
7. If the `oracle/completion/` package does not exist yet, write tests that will compile once it is created. Use build tags if necessary: `//go:build oracle_completion`.

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[EVAL-STAGE5] STARTED
[EVAL-STAGE5] STEP reading_refs - Reading PG/MySQL/MSSQL completion references
[EVAL-STAGE5] STEP identifying_positions - Identifying key cursor positions per statement type
[EVAL-STAGE5] STEP writing_select_tests - Writing SELECT completion tests
[EVAL-STAGE5] STEP writing_dml_tests - Writing INSERT/UPDATE/DELETE completion tests
[EVAL-STAGE5] STEP writing_ddl_tests - Writing DDL completion tests
[EVAL-STAGE5] STEP writing_plsql_tests - Writing PL/SQL completion tests
[EVAL-STAGE5] STEP build_check - Running go build on test file
[EVAL-STAGE5] STEP coverage_report - Generating stage5-completion.json
[EVAL-STAGE5] DONE
```

If a step fails:
```
[EVAL-STAGE5] FAIL step_name - description
[EVAL-STAGE5] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Key Strategy: Cursor Position Completion Testing

For each statement type, identify key cursor positions and assert that completion candidates include/exclude specific keywords.

### Cursor Position Convention

Use `|` to mark the cursor position in SQL strings. The test infrastructure should strip the `|` and use its byte offset as the cursor position.

```go
// Example: "SELECT | FROM t" means cursor is after "SELECT " (position 7)
func cursorPos(sql string) (string, int) {
    idx := strings.Index(sql, "|")
    if idx < 0 {
        return sql, len(sql)
    }
    return sql[:idx] + sql[idx+1:], idx
}
```

### Statement Types and Key Positions

#### SELECT Statement

```go
func TestEvalStage5_SelectKeyword(t *testing.T) {
    // Cursor at start: "|" → candidates should include SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, ...
}

func TestEvalStage5_SelectColumns(t *testing.T) {
    // "SELECT |" → candidates should include *, column names, functions, DISTINCT, ALL
}

func TestEvalStage5_SelectFrom(t *testing.T) {
    // "SELECT col |" → candidates should include FROM, ,
}

func TestEvalStage5_SelectTable(t *testing.T) {
    // "SELECT col FROM |" → candidates should include table names, schema names, (subquery
}

func TestEvalStage5_SelectWhere(t *testing.T) {
    // "SELECT col FROM t |" → candidates should include WHERE, ORDER BY, GROUP BY, HAVING, JOIN, UNION, ;
}

func TestEvalStage5_SelectWhereExpr(t *testing.T) {
    // "SELECT col FROM t WHERE |" → candidates should include column names, functions, NOT, EXISTS, (
}

func TestEvalStage5_SelectOrderBy(t *testing.T) {
    // "SELECT col FROM t ORDER BY |" → candidates should include column names, expressions
}

func TestEvalStage5_SelectJoin(t *testing.T) {
    // "SELECT col FROM t |" → candidates should include INNER JOIN, LEFT JOIN, RIGHT JOIN, CROSS JOIN, etc.
}
```

#### INSERT Statement

```go
func TestEvalStage5_InsertInto(t *testing.T) {
    // "INSERT |" → candidates should include INTO
}

func TestEvalStage5_InsertTable(t *testing.T) {
    // "INSERT INTO |" → candidates should include table names
}

func TestEvalStage5_InsertValues(t *testing.T) {
    // "INSERT INTO t (a, b) |" → candidates should include VALUES, SELECT
}
```

#### UPDATE Statement

```go
func TestEvalStage5_UpdateTable(t *testing.T) {
    // "UPDATE |" → candidates should include table names
}

func TestEvalStage5_UpdateSet(t *testing.T) {
    // "UPDATE t |" → candidates should include SET
}

func TestEvalStage5_UpdateSetCol(t *testing.T) {
    // "UPDATE t SET |" → candidates should include column names
}
```

#### DELETE Statement

```go
func TestEvalStage5_DeleteFrom(t *testing.T) {
    // "DELETE |" → candidates should include FROM
}
```

#### DDL Statements

```go
func TestEvalStage5_CreateType(t *testing.T) {
    // "CREATE |" → candidates should include TABLE, VIEW, INDEX, SEQUENCE, TRIGGER, FUNCTION, PROCEDURE, PACKAGE, TYPE, SYNONYM, OR
}

func TestEvalStage5_CreateTableCol(t *testing.T) {
    // "CREATE TABLE t (|" → candidates should include column names (for templates), data types are part of column definition
}

func TestEvalStage5_AlterTable(t *testing.T) {
    // "ALTER TABLE t |" → candidates should include ADD, DROP, MODIFY, RENAME
}
```

#### Oracle-Specific

```go
func TestEvalStage5_OracleHint(t *testing.T) {
    // "SELECT /*+ |" → candidates should include hint names: FULL, INDEX, PARALLEL, etc.
}

func TestEvalStage5_PlsqlBlock(t *testing.T) {
    // "BEGIN |" → candidates should include statement keywords, DECLARE, END, IF, FOR, WHILE, etc.
}
```

### Completion Result Structure

Tests should assert on the completion result structure (adapt based on what PG/MySQL/MSSQL use):

```go
type CompletionResult struct {
    Candidates []Candidate
}

type Candidate struct {
    Label    string // displayed text
    Kind     string // "keyword", "table", "column", "function", etc.
    Detail   string // optional detail
}
```

### Negative Assertions

Also test that irrelevant candidates are NOT included:

```go
func TestEvalStage5_SelectNoInsert(t *testing.T) {
    // "SELECT |" → candidates should NOT include INSERT, UPDATE, DELETE
    // (these are statement-level keywords, not valid after SELECT)
}
```

## Coverage Report Format

After writing tests, generate `oracle/quality/coverage/stage5-completion.json` using the canonical schema:

```json
{
  "stage": 5,
  "surface": "completion",
  "status": "eval_complete",
  "items": [
    {"id": "select_keyword", "description": "Completion at empty input", "tested": true},
    {"id": "select_columns", "description": "Completion after SELECT", "tested": true},
    {"id": "select_from", "description": "Completion after SELECT col", "tested": true},
    {"id": "select_table", "description": "Completion after FROM", "tested": true},
    {"id": "select_where", "description": "Completion after FROM table", "tested": true},
    {"id": "select_where_expr", "description": "Completion after WHERE", "tested": true},
    {"id": "select_order_by", "description": "Completion after ORDER BY", "tested": true},
    {"id": "select_join", "description": "Completion for JOIN types", "tested": true},
    {"id": "insert_into", "description": "Completion after INSERT", "tested": true},
    {"id": "insert_table", "description": "Completion after INSERT INTO", "tested": true},
    {"id": "insert_values", "description": "Completion after column list", "tested": true},
    {"id": "update_table", "description": "Completion after UPDATE", "tested": true},
    {"id": "update_set", "description": "Completion after UPDATE table", "tested": true},
    {"id": "update_set_col", "description": "Completion after SET", "tested": true},
    {"id": "delete_from", "description": "Completion after DELETE", "tested": true},
    {"id": "create_type", "description": "Completion after CREATE", "tested": true},
    {"id": "create_table_col", "description": "Completion inside CREATE TABLE", "tested": true},
    {"id": "alter_table", "description": "Completion after ALTER TABLE t", "tested": true},
    {"id": "oracle_hint", "description": "Completion inside Oracle hint", "tested": true},
    {"id": "plsql_block", "description": "Completion inside PL/SQL block", "tested": true}
  ],
  "total": 20,
  "tested": 20,
  "gaps": []
}
```

Each item uses `"tested": true/false` (not a `"status"` string). The `"gaps"` array lists IDs of items where `"tested"` is false.

## Verification

After writing the test file:

```bash
# Check if oracle/completion/ package exists
ls oracle/completion/ 2>/dev/null

# Must compile (tests may fail, but must compile)
# If using build tags: go build -tags oracle_completion ./oracle/completion/
cd /Users/rebeliceyang/Github/omni && go build ./oracle/...

# Run eval tests to see current state (failures expected before impl)
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/... -run "TestEvalStage5"
```

## Important Notes

- The `oracle/completion/` package likely does NOT exist yet. Use one of these strategies:
  1. **Build tag:** Add `//go:build oracle_completion` to the test file so it only compiles when the package is ready.
  2. **Parser package:** Write tests in `oracle/parser/` that test partial-parse behavior (completion often builds on partial parsing).
  3. **Interface-based:** Define the expected completion interface in the test file and assert against it.
- Reference PG's completion infrastructure (`pg/completion/`) for the expected API shape. Oracle completion should follow the same patterns.
- Completion quality depends heavily on the parser's ability to handle incomplete SQL — this builds on Stage 4 error recovery.
- For Oracle-specific features (hints, PL/SQL blocks, CONNECT BY), ensure cursor positions cover these constructs.
- Use `t.Skip("oracle/completion package not yet implemented")` for tests that cannot compile without the package.
