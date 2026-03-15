# BNF-First Parser Review Skill - T-SQL (MSSQL)

You are reviewing a recursive descent T-SQL (SQL Server) parser against BNF specifications.

**Working directory:** `/Users/rebeliceyang/Github/omni`
**Parser source:** `mssql/parser/`
**BNF files:** `mssql/parser/bnf/`
**BNF catalog:** `mssql/parser/MSSQL_BNF_CATALOG.json`
**Reference:** Microsoft T-SQL documentation (https://learn.microsoft.com/en-us/sql/t-sql/)
**AST definitions:** `mssql/ast/`
**Tests:** `mssql/parser/compare_test.go`

## Your Task

1. Read `mssql/parser/PROGRESS_SUMMARY.json` (lightweight version -- done batches are compressed, only pending/failed/in_progress have full detail)
2. Pick the next batch to work on:
   - If any batch has `"status": "in_progress"`, **resume that batch** (it was interrupted mid-work -- read the existing code in its target file and continue from where it left off)
   - Otherwise, find the first batch with `"status": "pending"` whose dependencies (by id) are all `"done"`
   - If any batch has `"status": "failed"`, **retry it** (reset to `"in_progress"` and try again)
3. Execute the BNF-first review for that batch following the steps below
4. Update `mssql/parser/PROGRESS.json` (the full file, NOT the summary):
   - Set `"in_progress"` before starting work
   - Set `"done"` only after `go build` and `go test` pass
   - Set `"failed"` with `"error"` if you cannot make tests pass

If all batches are `"done"`, output `ALL_BATCHES_COMPLETE` and stop.

## Progress Logging (MANDATORY)

You MUST print progress markers to stdout at each step:

```
[BATCH N] STARTED - batch_name
[BATCH N] STEP reading_bnf - Reading BNF files for this batch
[BATCH N] STEP comparing - Comparing parser code against BNF
[BATCH N] STEP fixing - Fixing gaps found in review
[BATCH N] STEP writing_tests - Adding test cases for gaps
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

**Do NOT skip these markers.**

## BNF-First Review Workflow

### Step 1: Read BNF Files (MANDATORY FIRST STEP)

**This is the most critical step. The BNF is the source of truth.**

For every statement in the batch:

1. **Read the BNF file**: `mssql/parser/bnf/{statement-name}-transact-sql.bnf`
2. **Read the BNF catalog**: Check `mssql/parser/MSSQL_BNF_CATALOG.json` for statement metadata and status
3. These BNF files are the **authoritative ground truth** — do NOT use WebFetch or other sources
4. **Extract the COMPLETE syntax** -- every branch, every option, every sub-clause
5. **Do NOT abbreviate** -- never write `...` or truncate the BNF

### Step 2: Compare Parser Against BNF

For each BNF rule, systematically check:

1. **Does the parser handle every branch?** Walk the BNF top-to-bottom and verify each alternative is implemented
2. **Are optional clauses handled?** Every `[ ... ]` in the BNF must have a conditional check
3. **Are repeating elements handled?** Every `{ ... } [ ,...n ]` must loop
4. **Is keyword matching correct?** Compare token constants against BNF keywords
5. **Are AST nodes complete?** Every semantic element in the BNF should be captured in the AST

Record all gaps found.

### Step 3: Fix Gaps

For each gap found:

1. **Add missing branches** to the parser
2. **Add missing AST fields** to `mssql/ast/parsenodes.go`
3. **Add serialization** to `mssql/ast/outfuncs.go` for any new node types
4. **Ensure BNF comment** on every parse function is complete and matches the official BNF exactly

### Step 4: Write Tests for Gaps

Add test cases to `compare_test.go` covering:
- Every newly added branch
- Edge cases for optional clauses
- Combinations of options

### Step 5: Build and Test

```bash
# Must compile
cd /Users/rebeliceyang/Github/omni && go build ./mssql/...

# Run batch-specific tests
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./mssql/parser/ -run "TestParse{BatchName}"

# Run full test suite (no regressions)
cd /Users/rebeliceyang/Github/omni && go test ./mssql/...
```

### Step 6: Update Progress

Edit `PROGRESS.json`:
- **Before starting work**: set `"status"` to `"in_progress"`
- **After all tests pass**: set `"status"` to `"done"`
- **If tests fail and you cannot fix**: set `"status"` to `"failed"` and add `"error": "description"`
- **NEVER mark `"done"` if `go build ./mssql/...` or `go test ./mssql/...` fails**

### Step 7: Git Commit

```bash
../../scripts/git-commit.sh "mssql/parser: review batch N - name" mssql/
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `mssql/parser/parser.go` | Parser struct, Parse() entry point, helpers |
| `mssql/parser/lexer.go` | Lexer, Token, keywords, token constants |
| `mssql/ast/parsenodes.go` | AST node struct definitions |
| `mssql/ast/node.go` | Node interface, List, String, Integer, etc. |
| `mssql/ast/outfuncs.go` | NodeToString for AST comparison |
| `mssql/parser/compare_test.go` | Test infrastructure |
| `mssql/parser/PROGRESS.json` | Full implementation progress tracking |
| `mssql/parser/PROGRESS_SUMMARY.json` | Compact progress for batch picker |
| `mssql/parser/bnf/` | BNF grammar files (282 files) |
| `mssql/parser/MSSQL_BNF_CATALOG.json` | BNF catalog (288 statements) |

## T-SQL Specific Notes

### Lexer Notes
- **Bracketed identifiers**: `[column name]` uses square brackets for quoting
- **Variables**: `@local_var`, `@@global_var`
- **N-strings**: `N'unicode string'` for nvarchar literals
- **Comments**: `--` line and `/* */` block (nested), no `#` comments
- **Not-equal**: Both `!=` and `<>` are valid
- **Special operators**: `!<` (not less than), `!>` (not greater than)
- **String concat**: `+` operator (same as addition)
- **Static methods**: `type::Method()` syntax

### Expression Precedence (highest to lowest)
1. `()` grouping, function calls
2. `::` scope resolution
3. `~` bitwise NOT, unary `+`, unary `-`
4. `*`, `/`, `%`
5. `+`, `-` (binary), `&` bitwise AND
6. `^` bitwise XOR
7. `|` bitwise OR
8. `=`, `>`, `<`, `>=`, `<=`, `<>`, `!=`, `!<`, `!>`
9. `NOT`
10. `AND`
11. `OR`
12. `BETWEEN`, `IN`, `LIKE`, `IS [NOT] NULL`, `EXISTS`, `ANY`, `ALL`, `SOME`

### T-SQL Specific Constructs
- **GO**: Batch separator (not a SQL statement -- handled specially)
- **TOP**: Uses parentheses: `TOP (10)` (parentheses optional for literals)
- **CROSS APPLY / OUTER APPLY**: T-SQL specific join types
- **OUTPUT clause**: Available on INSERT, UPDATE, DELETE, MERGE
- **MERGE**: Full MERGE with WHEN MATCHED/NOT MATCHED/NOT MATCHED BY SOURCE
- **TRY...CATCH**: Error handling blocks
- **IIF**: Inline IF function
- **CONVERT**: Type conversion with optional style parameter
- **CHOOSE**: Returns item at specified index
- **STRING_AGG**: String aggregation with WITHIN GROUP

## Batch Summary Table

| Batch | Name | Scope | Status |
|-------|------|-------|--------|
| 0-153 | *(implementation batches)* | Full T-SQL parser implementation | done |
| 154 | bnf_review_infrastructure | Lexer, parser helpers, expressions, types, names | pending |
| 155 | bnf_review_select | SELECT (CTE, TOP, JOIN, APPLY, FOR XML/JSON, OPTION) | pending |
| 156 | bnf_review_dml | INSERT, UPDATE, DELETE, MERGE, COPY INTO | pending |
| 157 | bnf_review_create_table | CREATE TABLE (temporal, graph, ledger, memory-optimized) | pending |
| 158 | bnf_review_alter_table | ALTER TABLE (all action types) | pending |
| 159 | bnf_review_index | All index types + fulltext | pending |
| 160 | bnf_review_view_trigger | VIEW + TRIGGER (DML, DDL, LOGON) | pending |
| 161 | bnf_review_routines | PROCEDURE, FUNCTION, EXECUTE | pending |
| 162 | bnf_review_database | CREATE/ALTER/DROP DATABASE, filegroups | pending |
| 163 | bnf_review_security | Users, logins, roles, GRANT/DENY/REVOKE | pending |
| 164 | bnf_review_crypto | Keys, certificates, credentials | pending |
| 165 | bnf_review_audit_event | Audit + event session | pending |
| 166 | bnf_review_variables_cursors_control_flow | DECLARE, SET, cursors, IF/WHILE/TRY | pending |
| 167 | bnf_review_transaction | Transaction statements + GO | pending |
| 168 | bnf_review_backup_restore | BACKUP/RESTORE | pending |
| 169 | bnf_review_ha_server | Availability groups, endpoints, resource governor | pending |
| 170 | bnf_review_service_broker | All service broker statements | pending |
| 171 | bnf_review_clr_external | Assembly, CLR, external objects | pending |
| 172 | bnf_review_schema_objects | Type, sequence, synonym, statistics, schema, partition | pending |
| 173 | bnf_review_utility_admin_drop | DBCC, admin, DROP, utility, rowset functions | pending |

## Parse Function Requirements

**Every parse function MUST have the COMPLETE BNF in its comment. This is a hard requirement.**

```go
// parseAlterTableStmt parses an ALTER TABLE statement.
//
// BNF: mssql/parser/bnf/alter-table-transact-sql.bnf
//
//  ALTER TABLE [ database_name . [ schema_name ] . | schema_name . ] table_name
//  {
//      ALTER COLUMN column_name
//      {
//          [ new_data_type [ ( precision [ , scale ] ) ] ]
//          [ COLLATE collation_name ]
//          [ NULL | NOT NULL ]
//          | { ADD | DROP } { ROWGUIDCOL | PERSISTED | NOT FOR REPLICATION | SPARSE | HIDDEN }
//          | { ADD | DROP } MASKED [ WITH ( FUNCTION = 'mask_function' ) ]
//      }
//      | [ WITH { CHECK | NOCHECK } ] ADD
//          { column_definition | computed_column_definition | table_constraint } [ ,...n ]
//      | DROP { [ CONSTRAINT ] [ IF EXISTS ] constraint_name [ ,...n ]
//             | COLUMN [ IF EXISTS ] column_name [ ,...n ] }
//      | [ WITH { CHECK | NOCHECK } ] { CHECK | NOCHECK } CONSTRAINT
//          { ALL | constraint_name [ ,...n ] }
//      | { ENABLE | DISABLE } TRIGGER { ALL | trigger_name [ ,...n ] }
//      | SWITCH [ PARTITION source_partition_number_expression ]
//          TO target_table [ PARTITION target_partition_number_expression ]
//      | SET ( FILESTREAM_ON = { partition_scheme_name | filegroup | "default" | "NULL" } )
//      | REBUILD [ [ PARTITION = ALL ] [ WITH ( rebuild_index_option [ ,...n ] ) ]
//               | [ PARTITION = partition_number [ WITH ( single_partition_rebuild_index_option ) ] ] ]
//  }
func (p *Parser) parseAlterTableStmt() *nodes.AlterTableStmt {
```

**The comment BNF must match the `.bnf` file exactly. No abbreviation, no `...` for omitted branches. Every branch in the BNF must have a corresponding code path.**

**Return type conventions:**
- Expression parsers return `ExprNode` (not `Node`)
- Table reference parsers return `TableExpr`
- Statement parsers return `StmtNode`

**Rules for parse function implementation:**

1. **Use the existing AST types** from `mssql/ast/` -- do NOT create new node types without updating both `parsenodes.go` and `outfuncs.go`
2. **Record positions** on EVERY AST node that has a `Loc` field: `Loc: nodes.Loc{Start: p.pos()}` at start, `node.Loc.End = p.pos()` at end
3. **Token constants** are in `mssql/parser/lexer.go`
4. **Keywords are case-insensitive** -- the lexer handles this
5. **Error recovery**: Skip to next semicolon for statement-level errors, record errors but continue parsing
