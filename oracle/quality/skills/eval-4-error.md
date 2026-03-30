# Eval Worker — Stage 4: Error Quality

You are an Eval Worker in the Oracle Quality Pipeline.
Your role is to write **tests ONLY** — never implementation code.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/strategy.md` — Stage 4 scope (mutation-based error testing)
- `oracle/quality/prevention-rules.md` — **MUST read before starting any work**
- `oracle/parser/parser.go` — current `ParseError` struct and `Parser` struct
- `oracle/parser/lexer.go` — current `Token` struct and `Lexer`
- `oracle/ast/parsenodes.go` — current AST node types
- `oracle/parser/eval_foundation_test.go` — Stage 1 eval tests (reference for patterns)

## Output Files

- **Test file:** `oracle/parser/eval_error_test.go`
- **Coverage report:** `oracle/quality/coverage/stage4-error.json`

## Rules

1. **Tests ONLY** — do NOT modify any non-test `.go` file.
2. Every test function MUST be named `TestEvalStage4_*`.
3. Tests should fail clearly with descriptive messages (not just compile errors).
4. Use `reflect` where possible to check field existence so tests compile even when fields are missing.
5. Do NOT import packages that do not exist yet; use reflection to probe for functions/fields.
6. Never modify existing test files (`compare_test.go`, `eval_foundation_test.go`, etc.).

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[EVAL-STAGE4] STARTED
[EVAL-STAGE4] STEP reading_refs - Reading reference files and Stage 3 valid SQL
[EVAL-STAGE4] STEP writing_truncation - Writing truncation mutation tests
[EVAL-STAGE4] STEP writing_deletion - Writing deletion mutation tests
[EVAL-STAGE4] STEP writing_replacement - Writing replacement mutation tests
[EVAL-STAGE4] STEP writing_duplication - Writing duplication mutation tests
[EVAL-STAGE4] STEP writing_oracle_xval - Writing Oracle DB cross-validation tests
[EVAL-STAGE4] STEP build_check - Running go build on test file
[EVAL-STAGE4] STEP coverage_report - Generating stage4-error.json
[EVAL-STAGE4] DONE
```

If a step fails:
```
[EVAL-STAGE4] FAIL step_name - description
[EVAL-STAGE4] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Key Strategy: Mutation-Based Error Testing

Take valid SQL statements from Stage 3 tests and apply 4 mutation types to systematically produce invalid SQL. The error space is infinite, so mutations provide structured coverage.

### Source SQL

Gather valid SQL from:
- Stage 3 eval tests (`oracle/parser/eval_correctness_test.go` or equivalent)
- BNF production rule examples in `oracle/parser/bnf/`
- If Stage 3 tests do not exist yet, use a representative set of valid Oracle SQL:
  - `SELECT col FROM t WHERE x = 1`
  - `INSERT INTO t (a, b) VALUES (1, 2)`
  - `UPDATE t SET a = 1 WHERE b = 2`
  - `DELETE FROM t WHERE a = 1`
  - `CREATE TABLE t (id NUMBER PRIMARY KEY, name VARCHAR2(100))`
  - `ALTER TABLE t ADD (col NUMBER)`
  - `DROP TABLE t`
  - `CREATE INDEX idx ON t (col)`
  - `CREATE OR REPLACE VIEW v AS SELECT * FROM t`
  - `GRANT SELECT ON t TO user1`

### Mutation Type 1: Truncation

Cut SQL at each token boundary, producing progressively shorter statements.

```go
func TestEvalStage4_Truncation(t *testing.T) {
    // For each valid SQL:
    //   1. Lex into tokens
    //   2. For each token position i (from 1 to len-1):
    //      - Truncate SQL at token[i].Loc (start byte offset)
    //      - Parse truncated SQL
    //      - Assert: no panic
    //      - Assert: returns an error (or is valid prefix — some truncations are valid SQL)
    //      - If error: assert error.Position >= 0
}
```

### Mutation Type 2: Deletion

Remove one required constituent (token) at a time.

```go
func TestEvalStage4_Deletion(t *testing.T) {
    // For each valid SQL:
    //   1. Lex into tokens
    //   2. For each token position i:
    //      - Remove token[i] from the SQL string
    //      - Parse mutated SQL
    //      - Assert: no panic
    //      - If original token was a required keyword: assert returns error
    //      - If error: assert error.Position >= 0
}
```

### Mutation Type 3: Replacement

Swap a keyword with an invalid token.

```go
func TestEvalStage4_Replacement(t *testing.T) {
    // For each valid SQL:
    //   1. Lex into tokens
    //   2. For each keyword token:
    //      - Replace with "XYZZY" (nonsense identifier)
    //      - Parse mutated SQL
    //      - Assert: no panic
    //      - Assert: returns an error
    //      - Assert: error.Position >= 0
}
```

### Mutation Type 4: Duplication

Repeat a clause (duplicate a token or token group).

```go
func TestEvalStage4_Duplication(t *testing.T) {
    // For each valid SQL:
    //   1. Lex into tokens
    //   2. For each token position i:
    //      - Insert a duplicate of token[i] after itself
    //      - Parse mutated SQL
    //      - Assert: no panic
    //      - If duplicated token creates invalid SQL: assert returns error
    //      - If error: assert error.Position >= 0
}
```

### Oracle DB Cross-Validation

```go
func TestEvalStage4_OracleDBRejection(t *testing.T) {
    // Requires Oracle testcontainer (skip if not available)
    // For each mutated SQL that Oracle DB rejects:
    //   - Assert: parser also rejects it (returns error)
    //   - If parser accepts but Oracle rejects: report as false-positive
}
```

## Invariants (Must Hold for ALL Mutations)

For every mutated SQL string `s`:

1. **No Panic:** `func() { defer recover(); parser.Parse(s) }()` must not panic. Use `t.Run` with deferred recovery.
2. **Error Returned:** If the SQL is invalid, `Parse(s)` must return a non-nil error.
3. **Position Valid:** If an error is returned, `error.Position >= 0` (error points to a real location).

## Test Structure

```go
package parser

import (
    "testing"
    "strings"
)

// validSQL is the corpus of valid SQL from Stage 3 (or representative set).
var stage4ValidSQL = []string{
    "SELECT col FROM t WHERE x = 1",
    "INSERT INTO t (a, b) VALUES (1, 2)",
    // ... more
}

// mutate applies a mutation function and tests the invariants.
func stage4AssertNoMutationPanic(t *testing.T, sql string, mutationType string) {
    t.Helper()
    defer func() {
        if r := recover(); r != nil {
            t.Fatalf("[%s] parser panicked on input: %q\npanic: %v", mutationType, sql, r)
        }
    }()
    _, err := Parse(sql)
    if err != nil {
        // Verify error has valid position
        // Use type assertion or reflect to check Position field
    }
}
```

## Coverage Report Format

After writing tests, generate `oracle/quality/coverage/stage4-error.json` using the canonical schema:

```json
{
  "stage": 4,
  "surface": "error",
  "status": "eval_complete",
  "items": [
    {"id": "truncation", "description": "Truncation mutation tests", "tested": true},
    {"id": "deletion", "description": "Deletion mutation tests", "tested": true},
    {"id": "replacement", "description": "Replacement mutation tests", "tested": true},
    {"id": "duplication", "description": "Duplication mutation tests", "tested": true},
    {"id": "oracle_db_rejection", "description": "Oracle DB rejection cross-validation", "tested": true}
  ],
  "total": 5,
  "tested": 5,
  "gaps": []
}
```

Each item uses `"tested": true/false` (not a `"status"` string). The `"gaps"` array lists IDs of items where `"tested"` is false.

## Verification

After writing the test file:

```bash
# Must compile (tests may fail, but must compile)
cd /Users/rebeliceyang/Github/omni && go build ./oracle/parser/

# Run eval tests to see current state (failures expected before impl)
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/parser/ -run "TestEvalStage4" -timeout 300s
```

## Important Notes

- Tests MUST compile even if the features are not yet implemented. Use `reflect` to probe for fields/functions that may not exist.
- The mutation approach generates potentially thousands of test cases from a small corpus of valid SQL — this is intentional. Use `t.Run` subtests for each mutation.
- For truncation, some prefixes may be valid SQL (e.g., `SELECT col FROM t` is valid even without `WHERE`). The test should only assert no-panic for all truncations, and error-returned for clearly invalid ones.
- For Oracle DB cross-validation, use `testcontainers` with Oracle XE. Skip tests if container is unavailable using `t.Skip("Oracle container not available")`.
- Prefer clear failure messages: `t.Fatalf("parser panicked on truncated SQL %q (truncated at position %d)", sql, pos)`.
