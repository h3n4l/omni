# Recursive Descent Parser Implementation Skill (Oracle PL/SQL)

You are implementing a recursive descent Oracle PL/SQL parser.

**Working directory:** `/Users/rebeliceyang/Github/omni`
**Parser source:** `oracle/parser/`
**AST definitions:** `oracle/ast/`
**Tests:** `oracle/parser/compare_test.go`

## Reference Documentation

- **Oracle Database 23c SQL Language Reference**: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/
- **Oracle Database 23c PL/SQL Language Reference**: https://docs.oracle.com/en/database/oracle/oracle-database/23/lnpls/
- Also reference the ANTLR2-based open-source Oracle grammar for understanding edge cases

## Your Task

1. Read `oracle/parser/PROGRESS_SUMMARY.json` (lightweight version — done batches are compressed, only pending/failed/in_progress have full detail)
2. Pick the next batch to work on:
   - If any batch has `"status": "in_progress"`, **resume that batch** (it was interrupted mid-work — read the existing code in its target file and continue from where it left off)
   - Otherwise, find the first batch with `"status": "pending"` whose dependencies (by id) are all `"done"`
   - If any batch has `"status": "failed"`, **retry it** (reset to `"in_progress"` and try again)
3. Implement that batch following the steps below
4. Update `oracle/parser/PROGRESS.json` (the full file, NOT the summary):
   - Set `"in_progress"` before starting work
   - Set `"done"` only after `go build` and `go test` pass
   - Set `"failed"` with `"error"` if you cannot make tests pass

If all batches are `"done"`, output `ALL_BATCHES_COMPLETE` and stop.

## Progress Logging (MANDATORY)

You MUST print progress markers to stdout at each step. This is how the pipeline operator monitors your work. Use this exact format:

```
[BATCH N] STARTED - batch_name
[BATCH N] STEP reading_refs - Reading BNF and AST definitions
[BATCH N] STEP writing_tests - Writing test cases
[BATCH N] STEP writing_code - Implementing parse functions
[BATCH N] STEP build - Running go build
[BATCH N] STEP test - Running go test (X passed, Y failed)
[BATCH N] STEP commit - Committing changes
[BATCH N] DONE
```

If a step fails, print:
```
[BATCH N] FAIL test - description of failure
[BATCH N] RETRY - what you're fixing
```

**Do NOT skip these markers.** They appear in the build log and are essential for debugging pipeline issues.

## Implementation Steps for Each Batch

### Step 1: Read Official Documentation (MANDATORY)

**This is the most critical step. Do NOT skip it. Do NOT write BNF from memory.**

Documentation has been **pre-fetched** to local files. For every grammar rule in the batch:

1. **First check local docs**: Read from `oracle/parser/docs/{STATEMENT-NAME}.txt` (e.g., `CREATE-TABLE.txt`, `ALTER-TABLESPACE.txt`)
   - Use the Read tool to read these files — they contain the full page text with BNF
2. **Only if the local file is missing**, use WebFetch as fallback:
   - SQL Reference: `https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/{command}.html`
   - PL/SQL Reference: `https://docs.oracle.com/en/database/oracle/oracle-database/23/lnpls/{topic}.html`
3. **Extract the COMPLETE BNF/syntax diagram** — every branch, every option, every sub-clause
4. **Do NOT abbreviate** — never write `...` or truncate the BNF
5. **For sub-clauses that have their own doc page**, check `oracle/parser/docs/` first, then WebFetch if missing

### Step 2: Read AST and Existing Code

- Read the AST node definitions from `oracle/ast/parsenodes.go` (and `node.go`)
- Read the existing parser code in `oracle/parser/` to understand available helpers
- If the existing AST types don't cover all BNF branches, add new node types / fields to `parsenodes.go`

### Step 3: Write Tests FIRST (Test-Driven Development)

**TEST-DRIVEN**: Write tests FIRST, then implement.

**Every branch of the BNF must have at least one test case.** For example, if ALTER TABLE has ADD, MODIFY, DROP COLUMN, DROP CONSTRAINT, RENAME, ENABLE CONSTRAINT, DISABLE CONSTRAINT, MOVE, SPLIT PARTITION, EXCHANGE PARTITION — then you need at least 10 test cases, one per branch.

Add test cases to `compare_test.go` using SQL strings relevant to the batch's grammar rules.
Add a new `TestParse{BatchName}` function.

### Step 4: Write Parse Functions

Create or update the target file (e.g., `oracle/parser/select.go`).

**Every parse function MUST have the COMPLETE BNF in its comment. This is a hard requirement.**

```go
// parseCreateTableStmt parses a CREATE TABLE statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-TABLE.html
//
//  CREATE [ GLOBAL TEMPORARY | PRIVATE TEMPORARY | SHARDED | DUPLICATED ]
//      TABLE [ schema. ] table_name
//      [ SHARING = { METADATA | DATA | EXTENDED DATA | NONE } ]
//      { relational_table | object_table | XMLType_table }
//      [ MEMOPTIMIZE FOR READ ] [ MEMOPTIMIZE FOR WRITE ]
//      [ PARENT [ schema. ] table ] ;
//
//  relational_table:
//      [ ( relational_properties ) ]
//      [ DEFAULT COLLATION collation_name ]
//      [ ON COMMIT { DROP | PRESERVE | DELETE } ROWS ]
//      [ physical_properties ]
//      [ table_properties ]
//
//  relational_properties:
//      { column_definition | virtual_column_definition
//        | period_definition
//        | { out_of_line_constraint | out_of_line_ref_constraint }
//        | supplemental_logging_props }
//      [, ...]
func (p *Parser) parseCreateTableStmt() *ast.CreateTableStmt {
```

**The comment BNF must match the official docs exactly. No abbreviation, no `...` for omitted branches. Every branch in the BNF must have a corresponding code path in the function.**

**Return type conventions:**
- Expression parsers return `ExprNode` (e.g., `parseExpr() ast.ExprNode`)
- Table reference parsers return `TableExpr` (e.g., `parseTableRef() ast.TableExpr`)
- Statement parsers return `StmtNode` (e.g., `parseSelectStmt() *ast.SelectStmt` which satisfies `StmtNode`)

**Rules for parse function implementation:**

1. **Use the existing AST types** from `oracle/ast/` — do NOT create new node types unless absolutely necessary
1a. **Every branch in the BNF comment MUST have a corresponding implementation.** If the BNF says `{ ADD | MODIFY | DROP | RENAME | ENABLE | DISABLE | MOVE | SPLIT | MERGE | EXCHANGE }`, you must handle ALL of them, not just a subset. If a branch requires a new AST node type or field, add it to `parsenodes.go` and `outfuncs.go`.
1b. **Sub-clauses must be recursively complete.** If a statement's BNF references `column_definition`, and `column_definition` itself has a full BNF (with DEFAULT, IDENTITY, ENCRYPT, inline_constraint, etc.), you must fetch that sub-clause's BNF and implement it completely too.
2. **Record positions** on EVERY AST node that has a `Loc` field.
   Set `Loc: nodes.Loc{Start: p.pos()}` at the beginning of parsing a node.
   Set `node.Loc.End = p.pos()` at the end of parsing a node.
   **This is a hard requirement.**
3. **Token constants** are keyword constants (kw*) and lexer tokens (tok*) in `oracle/parser/lexer.go`
4. **Match the Oracle grammar semantics exactly**
5. **Error recovery**: When encountering unexpected tokens, try to recover by:
   - Skipping to the next semicolon for statement-level errors
   - Returning a partial AST node with what was parsed so far
6. **Operator precedence** in expressions: use Pratt parsing (precedence climbing)
7. **Serialization**: When implementing a new batch, you MUST also add serialization to `oracle/ast/outfuncs.go` for any new node types used. Every node type that has a `nodeTag()` method must have a corresponding case in `writeNode` and a `writeXxx` function.
8. **Incremental dispatch**: The `parseStmt` dispatch in `parser.go` should be extended incrementally as each statement batch is implemented. Do not wait until batch 23 — wire in each statement parser as it is completed.

Run these commands in order:

```bash
# Must compile
cd /Users/rebeliceyang/Github/omni && go build ./oracle/...

# Run batch-specific tests
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/parser/ -run "TestXxx"

# Run full test suite (no regressions)
cd /Users/rebeliceyang/Github/omni && go test ./oracle/...
```

### Step 5: Update Progress

Edit `PROGRESS.json`:
- **Before starting work**: set `"status"` to `"in_progress"`
- **After all tests pass**: set `"status"` to `"done"`
- **If tests fail and you cannot fix**: set `"status"` to `"failed"` and add `"error": "description"`
- **NEVER mark `"done"` if `go build ./oracle/...` or `go test ./oracle/...` fails**

### Step 6: Git Commit

```bash
../../scripts/git-commit.sh "oracle/parser: implement batch N - name" oracle/
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `oracle/parser/parser.go` | Parser struct, Parse() entry point, helpers |
| `oracle/parser/lexer.go` | Lexer (Token, Lexer, NewLexer), keywords |
| `oracle/ast/node.go` | Node interface, List, String, Integer |
| `oracle/ast/parsenodes.go` | AST node struct definitions, enums |
| `oracle/ast/outfuncs.go` | AST serialization (NodeToString) |
| `oracle/parser/compare_test.go` | Test infrastructure |
| `oracle/parser/PROGRESS.json` | Batch progress tracking |

## Oracle-Specific Parsing Notes

### Lexer Specifics
- `"double-quoted identifiers"` (case-sensitive when quoted)
- Single-quoted strings with `''` escape (no backslash escape)
- `q'[delimiter]...[delimiter]'` alternative quoting mechanism
- `--` line comments, `/* */` block comments, `/*+ hints */`
- `||` string concatenation, `:=` assignment, `=>` named param, `..` range, `@` dblink, `**` exponent
- Bind variables: `:name`, `:1`
- National character literals: `N'text'`

### Oracle Dialect Differences from PostgreSQL
- MINUS instead of EXCEPT
- CONNECT BY / START WITH hierarchical queries
- PRIOR operator
- DECODE() function (like CASE but different syntax)
- (+) outer join syntax
- PIVOT / UNPIVOT
- INSERT ALL / INSERT FIRST (multi-table insert)
- MERGE statement with different syntax
- PL/SQL blocks (DECLARE/BEGIN/END/EXCEPTION)
- No dollar-quoted strings
- No backtick identifiers
- ROWID, ROWNUM, LEVEL pseudo-columns
- FLASHBACK queries (AS OF SCN/TIMESTAMP, VERSIONS BETWEEN)
- MODEL clause (spreadsheet-like calculations)
- SAMPLE clause (random row sampling)
- Analytic functions with OVER, KEEP (DENSE_RANK FIRST/LAST), WITHIN GROUP
- GROUPING SETS / CUBE / ROLLUP extensions to GROUP BY
- LATERAL inline views
- XMLTABLE / JSON_TABLE as table sources
- XML expressions (XMLELEMENT, XMLFOREST, XMLAGG)
- JSON expressions (JSON_OBJECT, JSON_ARRAY, JSON_VALUE, JSON_EXISTS, IS JSON)
- Cursor expressions: CURSOR(subquery)
- MULTISET operations (MULTISET UNION/INTERSECT/EXCEPT)
- TREAT expression for object type casting
- FORALL / BULK COLLECT / PIPE ROW in PL/SQL
- PRAGMA directives (AUTONOMOUS_TRANSACTION, EXCEPTION_INIT, etc.)
- PL/SQL CASE statement (distinct from CASE expression)
- PL/SQL EXIT [WHEN] / CONTINUE [WHEN]
- PL/SQL collection types in DECLARE (TABLE OF, VARRAY, RECORD, REF CURSOR)
- Named parameter notation (param => value)
- LOCK TABLE, CALL, RENAME statements
- SET ROLE, SET CONSTRAINT(S) session control
- AUDIT / NOAUDIT
- ASSOCIATE / DISASSOCIATE STATISTICS
- Compound triggers (multiple timing sections)
- DDL and database event triggers

### Known Gaps in Existing Batches (tracked as new batches)
- Batch 4 (select): PIVOT, UNPIVOT, MODEL, SAMPLE, FLASHBACK query clauses are listed in rules but not implemented
- Batch 3 (expressions): EXISTS skips subquery content instead of parsing it; parenthesized subquery detection missing
- Batch 8 (create_table): Partitioning, storage, LOB storage listed but implemented as stubs only
- Batch 14 (create_trigger): Compound triggers and DDL/database event triggers not implemented
- Batch 17 (alter_misc): ALTER INDEX/VIEW/SEQUENCE use skipToSemicolon stub returning placeholder node
- Batch 18 (grant): CREATE/ALTER/DROP USER/ROLE listed in rules but only GRANT/REVOKE are implemented
- Batch 21 (plsql_block): Procedure calls return nil; EXIT/CONTINUE missing

## Expression Parsing Strategy

Use **Pratt parsing / precedence climbing**:

Precedence levels (from low to high, matching Oracle):
1. OR
2. AND
3. NOT (prefix)
4. IS, IS NOT
5. comparison: =, <, >, <=, >=, !=, <>
6. LIKE, LIKEC, LIKE2, LIKE4, BETWEEN, IN
7. string concatenation: ||
8. addition: +, -
9. multiplication: *, /
10. unary: +, -, PRIOR, CONNECT_BY_ROOT
11. exponentiation: **
12. function call, column ref, bind variable
