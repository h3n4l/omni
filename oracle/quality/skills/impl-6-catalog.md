# Impl Worker — Stage 6: Catalog / Migration

You are an Impl Worker in the Oracle Quality Pipeline.
Your role is to write **implementation code ONLY** — never modify `*_eval_*_test.go` files.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/prevention-rules.md` — **MUST read before starting any work**
- `oracle/quality/strategy.md` — Stage 6 scope
- `oracle/catalog/eval_catalog_test.go` OR `oracle/parser/eval_catalog_test.go` — eval tests you must make pass
- `oracle/ast/parsenodes.go` — current AST node types (DDL nodes)
- `oracle/parser/parser.go` — current `Parser`
- `pg/catalog/` — PG catalog implementation (reference)
- `mysql/catalog/` — MySQL catalog implementation (reference)
- `mssql/catalog/` — MSSQL catalog implementation (reference)

## Goal

Make **all** eval tests pass:

```bash
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/... -run "TestEvalStage6"
```

While keeping **all existing tests** passing (Stages 1-5 and existing tests):

```bash
cd /Users/rebeliceyang/Github/omni && go test -count=1 ./oracle/...
```

## Rules

1. **Implementation ONLY** — do NOT modify any `*_eval_*_test.go` file.
2. Do NOT break existing tests.
3. Read `oracle/quality/prevention-rules.md` before starting.
4. Match PG/MySQL/MSSQL catalog infrastructure patterns as closely as possible.
5. Keep changes minimal and focused — do not refactor unrelated code.

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[IMPL-STAGE6] STARTED
[IMPL-STAGE6] STEP reading_eval - Reading eval test expectations
[IMPL-STAGE6] STEP reading_prevention - Reading prevention rules
[IMPL-STAGE6] STEP reading_pg_ref - Reading PG/MySQL/MSSQL catalog references
[IMPL-STAGE6] STEP create_package - Creating oracle/catalog/ package
[IMPL-STAGE6] STEP impl_schema_model - Implementing in-memory schema model
[IMPL-STAGE6] STEP impl_catalog_reader - Implementing Oracle catalog reader
[IMPL-STAGE6] STEP impl_ddl_builder - Implementing DDL builder from schema model
[IMPL-STAGE6] STEP impl_migration - Implementing schema diff and migration generation
[IMPL-STAGE6] STEP impl_round_trip - Implementing round-trip verification
[IMPL-STAGE6] STEP build - Running go build
[IMPL-STAGE6] STEP test_eval - Running Stage 6 eval tests
[IMPL-STAGE6] STEP test_existing - Running all existing tests (Stages 1-5)
[IMPL-STAGE6] STEP commit - Committing changes
[IMPL-STAGE6] DONE
```

If a step fails:
```
[IMPL-STAGE6] FAIL step_name - description
[IMPL-STAGE6] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Implementation Plan

### 1. Create `oracle/catalog/` Package

This package likely needs to be created from scratch. Follow the PG/MySQL/MSSQL pattern:

```
oracle/catalog/
  catalog.go          — main types and Schema model
  reader.go           — read schema from Oracle system catalog views
  builder.go          — generate DDL from Schema model
  diff.go             — compare two Schema models, produce migration DDL
```

### 2. Schema Model — `oracle/catalog/catalog.go`

```go
package catalog

// Schema represents an Oracle database schema.
type Schema struct {
    Name        string
    Tables      []*Table
    Views       []*View
    Indexes     []*Index
    Sequences   []*Sequence
    Triggers    []*Trigger
    Functions   []*Function
    Procedures  []*Procedure
    Packages    []*Package
    Types       []*Type
    Synonyms    []*Synonym
}

type Table struct {
    Name        string
    Columns     []*Column
    Constraints []*Constraint
    Comment     string
}

type Column struct {
    Name         string
    DataType     string
    Length       int
    Precision    int
    Scale        int
    Nullable     bool
    Default      string
    Comment      string
}

type Constraint struct {
    Name           string
    Type           string // "P" (primary), "R" (foreign), "U" (unique), "C" (check)
    Columns        []string
    RefTable       string
    RefColumns     []string
    DeleteRule     string
    SearchCondition string
}

// ... View, Index, Sequence, Trigger, Function, Procedure, Package, Type, Synonym
```

### 3. Catalog Reader — `oracle/catalog/reader.go`

Read schema state from Oracle system catalog views:

```go
// ReadSchema reads the schema from an Oracle database connection.
func ReadSchema(db *sql.DB, schemaName string) (*Schema, error) {
    schema := &Schema{Name: schemaName}
    // Query USER_TABLES, USER_TAB_COLUMNS, USER_CONSTRAINTS, etc.
    // Populate schema model
    return schema, nil
}
```

Query the following Oracle catalog views:
- `USER_TABLES` — table metadata
- `USER_TAB_COLUMNS` — column definitions
- `USER_CONSTRAINTS` — constraint definitions
- `USER_CONS_COLUMNS` — constraint column mappings
- `USER_INDEXES` — index metadata
- `USER_IND_COLUMNS` — index column mappings
- `USER_SEQUENCES` — sequence definitions
- `USER_TRIGGERS` — trigger metadata
- `USER_VIEWS` — view definitions
- `USER_SOURCE` — PL/SQL source (functions, procedures, packages)
- `USER_TAB_COMMENTS` — table comments
- `USER_COL_COMMENTS` — column comments
- `USER_SYNONYMS` — synonym definitions
- `USER_TYPES` — type definitions

### 4. DDL Builder — `oracle/catalog/builder.go`

Generate DDL statements from the schema model:

```go
// BuildDDL generates DDL statements to create all objects in the schema.
func BuildDDL(schema *Schema) []string {
    var ddl []string
    // Generate CREATE TABLE, CREATE VIEW, etc. in dependency order
    return ddl
}
```

### 5. Schema Diff — `oracle/catalog/diff.go`

Compare two schemas and produce migration DDL:

```go
// Diff compares two schemas and returns DDL to migrate from 'from' to 'to'.
func Diff(from, to *Schema) []string {
    var ddl []string
    // Compare tables, columns, constraints, etc.
    // Generate ALTER TABLE ADD/DROP/MODIFY, CREATE/DROP INDEX, etc.
    return ddl
}
```

### 6. AST-to-Schema Conversion

Build schema model from parsed DDL AST:

```go
// FromAST builds a Schema from a list of parsed DDL statements.
func FromAST(stmts []ast.Node) (*Schema, error) {
    schema := &Schema{}
    // Walk AST nodes, populate schema model
    return schema, nil
}
```

## Oracle-Specific Considerations

- **Implicit indexes:** Oracle creates implicit indexes for PRIMARY KEY and UNIQUE constraints. The catalog reader must handle these.
- **Implicit NOT NULL:** PRIMARY KEY columns are implicitly NOT NULL in Oracle.
- **Tablespace:** Oracle tables have a default tablespace. The DDL builder should include TABLESPACE when it differs from default.
- **Storage clauses:** STORAGE (INITIAL, NEXT, etc.) — include in DDL builder for accuracy.
- **Partitioning:** PARTITION BY RANGE/LIST/HASH — significant for catalog accuracy.
- **Object naming:** Oracle uppercases unquoted identifiers. Handle case sensitivity correctly.
- **CASCADE CONSTRAINTS:** DROP TABLE may need CASCADE CONSTRAINTS to drop dependent objects.
- **PURGE:** DROP TABLE ... PURGE to bypass recycle bin.

## Verification

After all implementation:

```bash
# Stage 6 eval tests must pass
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/... -run "TestEvalStage6"

# All existing tests must still pass (Stages 1-5)
cd /Users/rebeliceyang/Github/omni && go test -count=1 ./oracle/...

# Build must succeed
cd /Users/rebeliceyang/Github/omni && go build ./oracle/...
```

## Commit

After all tests pass:

```bash
git add oracle/catalog/
git commit -m "feat(oracle): implement catalog infrastructure

Add oracle/catalog/ package with schema model, catalog reader,
DDL builder, and schema diff for migration generation."
```

## Important Notes

- This is likely a **greenfield** implementation — the `oracle/catalog/` package probably does not exist yet.
- Study the PG, MySQL, and MSSQL catalog packages carefully before starting. Reuse patterns and conventions.
- Oracle's system catalog views are different from information_schema (used by PG/MySQL). Use USER_* views.
- The catalog reader requires a real Oracle database connection. Use testcontainers for integration tests.
- Schema comparison must handle Oracle's implicit behaviors (automatic indexes, uppercase naming, etc.).
- Dependency ordering is critical for DDL generation — tables before foreign keys, etc.
- If the eval tests use build tags, make sure your implementation file headers match.
