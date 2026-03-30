# Impl Worker — Stage 5: Completion

You are an Impl Worker in the Oracle Quality Pipeline.
Your role is to write **implementation code ONLY** — never modify `*_eval_*_test.go` files.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/prevention-rules.md` — **MUST read before starting any work**
- `oracle/quality/strategy.md` — Stage 5 scope
- `oracle/completion/eval_completion_test.go` OR `oracle/parser/eval_completion_test.go` — eval tests you must make pass
- `oracle/parser/parser.go` — current `Parser` struct
- `oracle/parser/lexer.go` — current `Token` and `Lexer`
- `pg/completion/completion.go` — PG completion implementation (reference)
- `pg/completion/resolve.go` — PG completion resolution (reference)
- `pg/completion/refs.go` — PG completion references (reference)
- `mysql/completion/` — MySQL completion implementation (reference)
- `mssql/completion/` — MSSQL completion implementation (reference)

## Goal

Make **all** eval tests pass:

```bash
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/... -run "TestEvalStage5"
```

While keeping **all existing tests** passing (Stages 1-4 and existing tests):

```bash
cd /Users/rebeliceyang/Github/omni && go test -count=1 ./oracle/...
```

## Rules

1. **Implementation ONLY** — do NOT modify any `*_eval_*_test.go` file.
2. Do NOT break existing tests.
3. Read `oracle/quality/prevention-rules.md` before starting.
4. Match PG/MySQL/MSSQL completion infrastructure patterns as closely as possible.
5. Keep changes minimal and focused — do not refactor unrelated code.

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[IMPL-STAGE5] STARTED
[IMPL-STAGE5] STEP reading_eval - Reading eval test expectations
[IMPL-STAGE5] STEP reading_prevention - Reading prevention rules
[IMPL-STAGE5] STEP reading_pg_ref - Reading PG completion reference implementation
[IMPL-STAGE5] STEP create_package - Creating oracle/completion/ package
[IMPL-STAGE5] STEP impl_types - Implementing Candidate, CompletionResult types
[IMPL-STAGE5] STEP impl_complete_func - Implementing Complete() function
[IMPL-STAGE5] STEP impl_keyword_candidates - Implementing keyword candidate generation
[IMPL-STAGE5] STEP impl_context_detection - Implementing cursor context detection
[IMPL-STAGE5] STEP impl_statement_handlers - Implementing per-statement-type handlers
[IMPL-STAGE5] STEP build - Running go build
[IMPL-STAGE5] STEP test_eval - Running Stage 5 eval tests
[IMPL-STAGE5] STEP test_existing - Running all existing tests (Stages 1-4)
[IMPL-STAGE5] STEP commit - Committing changes
[IMPL-STAGE5] DONE
```

If a step fails:
```
[IMPL-STAGE5] FAIL step_name - description
[IMPL-STAGE5] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Implementation Plan

### 1. Create `oracle/completion/` Package

This package likely needs to be created from scratch. Follow the PG/MySQL/MSSQL pattern:

```
oracle/completion/
  completion.go       — main Complete() function, types
  resolve.go          — context resolution (what kind of position is the cursor at?)
  refs.go             — reference/candidate generation
```

### 2. Core Types — `oracle/completion/completion.go`

```go
package completion

// Candidate represents a single completion suggestion.
type Candidate struct {
    Label    string // displayed text (e.g., "SELECT", "table_name")
    Kind     string // "keyword", "table", "column", "function", "schema", etc.
    Detail   string // optional additional info
}

// CompletionResult holds the list of candidates.
type CompletionResult struct {
    Candidates []Candidate
}

// Complete returns completion candidates for the given SQL at the given cursor position.
func Complete(sql string, pos int) *CompletionResult {
    // 1. Lex the SQL up to cursor position
    // 2. Determine context (what kind of position is the cursor at?)
    // 3. Generate candidates based on context
    return &CompletionResult{}
}
```

### 3. Context Detection — `oracle/completion/resolve.go`

Determine what the cursor is positioned at:

```go
type CursorContext int

const (
    ContextStatementStart CursorContext = iota
    ContextSelectColumns
    ContextFromClause
    ContextWhereClause
    ContextJoinClause
    ContextOrderByClause
    ContextGroupByClause
    ContextInsertInto
    ContextInsertColumns
    ContextInsertValues
    ContextUpdateTable
    ContextUpdateSet
    ContextDeleteFrom
    ContextCreateType
    ContextAlterTable
    ContextHint
    ContextPlsqlBlock
    // ... more contexts
)

func resolveContext(tokens []Token, cursorPos int) CursorContext {
    // Walk tokens backwards from cursor to determine context
}
```

### 4. Keyword Candidates — `oracle/completion/refs.go`

Pre-defined keyword sets for each context:

```go
var statementKeywords = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP", "GRANT", "REVOKE", "BEGIN", "DECLARE"}
var selectColumnKeywords = []string{"*", "DISTINCT", "ALL", "CASE", "NULL"}
var afterFromKeywords = []string{"WHERE", "ORDER BY", "GROUP BY", "HAVING", "UNION", "INTERSECT", "MINUS", "CONNECT BY", "START WITH"}
var joinKeywords = []string{"INNER JOIN", "LEFT JOIN", "LEFT OUTER JOIN", "RIGHT JOIN", "RIGHT OUTER JOIN", "CROSS JOIN", "NATURAL JOIN", "FULL OUTER JOIN"}
var createObjectKeywords = []string{"TABLE", "VIEW", "INDEX", "SEQUENCE", "TRIGGER", "FUNCTION", "PROCEDURE", "PACKAGE", "TYPE", "SYNONYM", "OR"}
var alterTableKeywords = []string{"ADD", "DROP", "MODIFY", "RENAME"}
var hintKeywords = []string{"FULL", "INDEX", "PARALLEL", "FIRST_ROWS", "ALL_ROWS", "USE_NL", "USE_HASH", "USE_MERGE", "LEADING", "ORDERED"}
```

### 5. Statement-Specific Handlers

Implement handlers for each statement type that produce context-appropriate candidates. Follow the pattern established by PG/MySQL/MSSQL completion.

## Verification

After all implementation:

```bash
# Stage 5 eval tests must pass
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/... -run "TestEvalStage5"

# All existing tests must still pass (Stages 1-4)
cd /Users/rebeliceyang/Github/omni && go test -count=1 ./oracle/...

# Build must succeed
cd /Users/rebeliceyang/Github/omni && go build ./oracle/...
```

## Commit

After all tests pass:

```bash
git add oracle/completion/
git commit -m "feat(oracle): implement completion infrastructure

Add oracle/completion/ package with context-aware keyword completion.
Supports SELECT, INSERT, UPDATE, DELETE, DDL, hints, and PL/SQL blocks."
```

## Important Notes

- This is likely a **greenfield** implementation — the `oracle/completion/` package probably does not exist yet.
- Study the PG, MySQL, and MSSQL completion packages carefully before starting. Reuse patterns and conventions.
- Completion depends heavily on the parser's ability to handle incomplete/partial SQL (Stage 4 error recovery).
- Start with keyword-only completion. Schema-aware completion (actual table/column names) can be added later.
- The completion infrastructure should be extensible — new statement types and contexts will be added over time.
- If the eval tests use build tags, make sure your implementation file headers match.
