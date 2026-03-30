# Eval Worker — Stage 6: Catalog / Migration

You are an Eval Worker in the Oracle Quality Pipeline.
Your role is to write **tests ONLY** — never implementation code.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/strategy.md` — Stage 6 scope (catalog round-trip, pairwise coverage)
- `oracle/quality/prevention-rules.md` — **MUST read before starting any work**
- `oracle/ast/parsenodes.go` — current AST node types (DDL nodes)
- `oracle/parser/parser.go` — current `Parser`
- `pg/catalog/` — PG catalog infrastructure (reference pattern)
- `mysql/catalog/` — MySQL catalog infrastructure (reference pattern)
- `mssql/catalog/` — MSSQL catalog infrastructure (reference pattern)

## Output Files

- **Test file:** `oracle/catalog/eval_catalog_test.go` (may need to create `oracle/catalog/` package) OR `oracle/parser/eval_catalog_test.go` (if catalog package does not exist)
- **Coverage report:** `oracle/quality/coverage/stage6-catalog.json`

## Rules

1. **Tests ONLY** — do NOT modify any non-test `.go` file.
2. Every test function MUST be named `TestEvalStage6_*`.
3. Tests should fail clearly with descriptive messages (not just compile errors).
4. Use `reflect` where possible to check field existence so tests compile even when fields are missing.
5. Do NOT import packages that do not exist yet; use reflection to probe for functions/fields.
6. Never modify existing test files.
7. If the `oracle/catalog/` package does not exist yet, use build tags: `//go:build oracle_catalog`.

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[EVAL-STAGE6] STARTED
[EVAL-STAGE6] STEP reading_refs - Reading PG/MySQL/MSSQL catalog references
[EVAL-STAGE6] STEP identifying_objects - Identifying object types and operations
[EVAL-STAGE6] STEP writing_table_tests - Writing TABLE round-trip tests
[EVAL-STAGE6] STEP writing_view_tests - Writing VIEW round-trip tests
[EVAL-STAGE6] STEP writing_index_tests - Writing INDEX round-trip tests
[EVAL-STAGE6] STEP writing_constraint_tests - Writing CONSTRAINT round-trip tests
[EVAL-STAGE6] STEP writing_sequence_tests - Writing SEQUENCE round-trip tests
[EVAL-STAGE6] STEP writing_trigger_tests - Writing TRIGGER round-trip tests
[EVAL-STAGE6] STEP writing_plsql_tests - Writing FUNCTION/PROCEDURE/PACKAGE round-trip tests
[EVAL-STAGE6] STEP writing_pairwise_tests - Writing pairwise combination tests
[EVAL-STAGE6] STEP writing_oracle_xval - Writing Oracle DB cross-validation tests
[EVAL-STAGE6] STEP build_check - Running go build on test file
[EVAL-STAGE6] STEP coverage_report - Generating stage6-catalog.json
[EVAL-STAGE6] DONE
```

If a step fails:
```
[EVAL-STAGE6] FAIL step_name - description
[EVAL-STAGE6] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Key Strategy: DDL Round-Trip Testing on Real Oracle DB

### Round-Trip Test Pattern

For each DDL statement:

1. **Apply DDL** to Oracle container (via testcontainers)
2. **Query system catalog** (USER_TABLES, USER_TAB_COLUMNS, USER_CONSTRAINTS, USER_INDEXES, etc.)
3. **Parse DDL** with Oracle parser and build in-memory schema model
4. **Generate migration DDL** from parsed AST (if catalog package supports it)
5. **Apply migration DDL** to a fresh schema
6. **Compare schema state** — the two schemas should match

```go
func TestEvalStage6_TableRoundTrip(t *testing.T) {
    // Skip if Oracle container unavailable
    // 1. CREATE TABLE t (id NUMBER PRIMARY KEY, name VARCHAR2(100) NOT NULL)
    // 2. Query USER_TABLES, USER_TAB_COLUMNS, USER_CONSTRAINTS
    // 3. Parse the DDL with oracle/parser
    // 4. Assert AST matches catalog state
    // 5. Generate equivalent DDL from AST
    // 6. Apply to fresh schema, compare
}
```

### Object Types to Test

#### Tables

```go
func TestEvalStage6_CreateTable(t *testing.T) {
    // Basic CREATE TABLE with columns, data types, constraints
}

func TestEvalStage6_AlterTableAddColumn(t *testing.T) {
    // ALTER TABLE ADD (col datatype)
}

func TestEvalStage6_AlterTableDropColumn(t *testing.T) {
    // ALTER TABLE DROP (col)
}

func TestEvalStage6_AlterTableModifyColumn(t *testing.T) {
    // ALTER TABLE MODIFY (col datatype)
}

func TestEvalStage6_DropTable(t *testing.T) {
    // DROP TABLE t [CASCADE CONSTRAINTS] [PURGE]
}

func TestEvalStage6_RenameTable(t *testing.T) {
    // ALTER TABLE t RENAME TO t2
}
```

#### Views

```go
func TestEvalStage6_CreateView(t *testing.T) {
    // CREATE [OR REPLACE] VIEW v AS SELECT ...
}

func TestEvalStage6_DropView(t *testing.T) {
    // DROP VIEW v
}
```

#### Indexes

```go
func TestEvalStage6_CreateIndex(t *testing.T) {
    // CREATE [UNIQUE] INDEX idx ON t (col1, col2)
}

func TestEvalStage6_DropIndex(t *testing.T) {
    // DROP INDEX idx
}
```

#### Constraints

```go
func TestEvalStage6_PrimaryKey(t *testing.T) {
    // PRIMARY KEY constraint (inline and out-of-line)
}

func TestEvalStage6_ForeignKey(t *testing.T) {
    // FOREIGN KEY constraint with ON DELETE CASCADE/SET NULL
}

func TestEvalStage6_UniqueConstraint(t *testing.T) {
    // UNIQUE constraint
}

func TestEvalStage6_CheckConstraint(t *testing.T) {
    // CHECK constraint
}

func TestEvalStage6_NotNullConstraint(t *testing.T) {
    // NOT NULL constraint
}
```

#### Sequences

```go
func TestEvalStage6_CreateSequence(t *testing.T) {
    // CREATE SEQUENCE seq START WITH 1 INCREMENT BY 1 ...
}

func TestEvalStage6_AlterSequence(t *testing.T) {
    // ALTER SEQUENCE seq INCREMENT BY 10
}

func TestEvalStage6_DropSequence(t *testing.T) {
    // DROP SEQUENCE seq
}
```

#### Triggers

```go
func TestEvalStage6_CreateTrigger(t *testing.T) {
    // CREATE [OR REPLACE] TRIGGER trg BEFORE INSERT ON t FOR EACH ROW ...
}

func TestEvalStage6_DropTrigger(t *testing.T) {
    // DROP TRIGGER trg
}
```

#### Functions / Procedures / Packages

```go
func TestEvalStage6_CreateFunction(t *testing.T) {
    // CREATE [OR REPLACE] FUNCTION f RETURN NUMBER IS BEGIN ... END;
}

func TestEvalStage6_CreateProcedure(t *testing.T) {
    // CREATE [OR REPLACE] PROCEDURE p IS BEGIN ... END;
}

func TestEvalStage6_CreatePackage(t *testing.T) {
    // CREATE [OR REPLACE] PACKAGE pkg IS ... END;
}
```

#### Types / Synonyms

```go
func TestEvalStage6_CreateType(t *testing.T) {
    // CREATE [OR REPLACE] TYPE t AS OBJECT (...)
}

func TestEvalStage6_CreateSynonym(t *testing.T) {
    // CREATE [OR REPLACE] [PUBLIC] SYNONYM syn FOR schema.object
}
```

#### Comments

```go
func TestEvalStage6_CommentOnTable(t *testing.T) {
    // COMMENT ON TABLE t IS 'description'
}

func TestEvalStage6_CommentOnColumn(t *testing.T) {
    // COMMENT ON COLUMN t.col IS 'description'
}
```

### Pairwise Combination Tests

Use pairwise testing to cover combinations of:
- **Object types:** table, view, index, trigger, sequence, function, procedure, package, type, synonym (10)
- **Operations:** create, alter, drop, rename, comment (5)
- **Properties:** constraints, defaults, storage clauses, partitioning (4)

Not all combinations are valid (e.g., cannot ALTER a synonym). Generate only valid pairs.

```go
func TestEvalStage6_PairwiseCombinations(t *testing.T) {
    // Generated pairwise test cases
    cases := []struct {
        name       string
        objectType string
        operation  string
        property   string
        ddl        string
    }{
        // ... pairwise-generated cases
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // 1. Apply DDL to Oracle DB
            // 2. Query catalog
            // 3. Parse DDL
            // 4. Compare
        })
    }
}
```

### Oracle System Catalog Queries

Reference these catalog views for validation:

```sql
-- Tables
SELECT table_name, tablespace_name FROM USER_TABLES;

-- Columns
SELECT table_name, column_name, data_type, data_length, data_precision, data_scale, nullable, data_default
FROM USER_TAB_COLUMNS WHERE table_name = :name;

-- Constraints
SELECT constraint_name, constraint_type, search_condition, r_constraint_name, delete_rule
FROM USER_CONSTRAINTS WHERE table_name = :name;

-- Constraint columns
SELECT constraint_name, column_name, position
FROM USER_CONS_COLUMNS WHERE table_name = :name;

-- Indexes
SELECT index_name, index_type, uniqueness, table_name
FROM USER_INDEXES WHERE table_name = :name;

-- Index columns
SELECT index_name, column_name, column_position
FROM USER_IND_COLUMNS WHERE table_name = :name;

-- Sequences
SELECT sequence_name, min_value, max_value, increment_by, cycle_flag, order_flag, cache_size, last_number
FROM USER_SEQUENCES WHERE sequence_name = :name;

-- Triggers
SELECT trigger_name, trigger_type, triggering_event, table_name, status
FROM USER_TRIGGERS WHERE trigger_name = :name;

-- Views
SELECT view_name, text FROM USER_VIEWS WHERE view_name = :name;

-- Source (functions, procedures, packages)
SELECT name, type, line, text FROM USER_SOURCE WHERE name = :name ORDER BY line;
```

## Coverage Report Format

After writing tests, generate `oracle/quality/coverage/stage6-catalog.json`:

```json
{
  "stage": "6-catalog",
  "total_items": 25,
  "tested_items": 25,
  "items": [
    {"id": "create_table", "test": "TestEvalStage6_CreateTable", "status": "written"},
    {"id": "alter_table_add", "test": "TestEvalStage6_AlterTableAddColumn", "status": "written"},
    {"id": "alter_table_drop", "test": "TestEvalStage6_AlterTableDropColumn", "status": "written"},
    {"id": "alter_table_modify", "test": "TestEvalStage6_AlterTableModifyColumn", "status": "written"},
    {"id": "drop_table", "test": "TestEvalStage6_DropTable", "status": "written"},
    {"id": "rename_table", "test": "TestEvalStage6_RenameTable", "status": "written"},
    {"id": "create_view", "test": "TestEvalStage6_CreateView", "status": "written"},
    {"id": "drop_view", "test": "TestEvalStage6_DropView", "status": "written"},
    {"id": "create_index", "test": "TestEvalStage6_CreateIndex", "status": "written"},
    {"id": "drop_index", "test": "TestEvalStage6_DropIndex", "status": "written"},
    {"id": "primary_key", "test": "TestEvalStage6_PrimaryKey", "status": "written"},
    {"id": "foreign_key", "test": "TestEvalStage6_ForeignKey", "status": "written"},
    {"id": "unique_constraint", "test": "TestEvalStage6_UniqueConstraint", "status": "written"},
    {"id": "check_constraint", "test": "TestEvalStage6_CheckConstraint", "status": "written"},
    {"id": "not_null_constraint", "test": "TestEvalStage6_NotNullConstraint", "status": "written"},
    {"id": "create_sequence", "test": "TestEvalStage6_CreateSequence", "status": "written"},
    {"id": "alter_sequence", "test": "TestEvalStage6_AlterSequence", "status": "written"},
    {"id": "drop_sequence", "test": "TestEvalStage6_DropSequence", "status": "written"},
    {"id": "create_trigger", "test": "TestEvalStage6_CreateTrigger", "status": "written"},
    {"id": "drop_trigger", "test": "TestEvalStage6_DropTrigger", "status": "written"},
    {"id": "create_function", "test": "TestEvalStage6_CreateFunction", "status": "written"},
    {"id": "create_procedure", "test": "TestEvalStage6_CreateProcedure", "status": "written"},
    {"id": "create_package", "test": "TestEvalStage6_CreatePackage", "status": "written"},
    {"id": "comment_on_table", "test": "TestEvalStage6_CommentOnTable", "status": "written"},
    {"id": "pairwise_combinations", "test": "TestEvalStage6_PairwiseCombinations", "status": "written"}
  ]
}
```

The `status` field transitions: `"written"` -> `"passing"` (once impl worker makes it pass) -> `"verified"` (once insight worker reviews).

## Verification

After writing the test file:

```bash
# Check if oracle/catalog/ package exists
ls oracle/catalog/ 2>/dev/null

# Must compile (tests may fail, but must compile)
# If using build tags: go build -tags oracle_catalog ./oracle/catalog/
cd /Users/rebeliceyang/Github/omni && go build ./oracle/...

# Run eval tests to see current state (failures expected before impl)
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/... -run "TestEvalStage6"
```

## Important Notes

- The `oracle/catalog/` package likely does NOT exist yet. Use build tags (`//go:build oracle_catalog`) so tests only compile when the package is ready.
- Oracle DB cross-validation requires a running Oracle container. Use `testcontainers` and skip tests if unavailable: `t.Skip("Oracle container not available")`.
- Oracle system catalog views (USER_TABLES, USER_TAB_COLUMNS, etc.) are the source of truth for schema state.
- Pairwise testing reduces the combinatorial explosion while maintaining 2-way coverage of all dimension pairs.
- For DDL round-trip: the generated DDL does not need to be character-identical to the original, but the resulting schema state must be equivalent.
- Pay attention to Oracle-specific behaviors: implicit index creation for PRIMARY KEY/UNIQUE, automatic NOT NULL for PRIMARY KEY columns, default tablespace, etc.
- Reference PG/MySQL/MSSQL catalog packages for API patterns, but Oracle catalog views are different from information_schema.
