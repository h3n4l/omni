# SCENARIOS: MSSQL Keyword System Alignment with SqlScriptDOM

## Goal

Align omni's MSSQL parser keyword system with SqlScriptDOM TSql170. Drive all 6 enforcement tests to PASS:
- `TestKeywordCompleteness`: 0 unregistered keywords
- `TestNoStringKeywordMatch`: 0 string-based keyword matches (eqFold + matchesKeywordCI)
- `TestKeywordClassification`: Core/Context classification matches SqlScriptDOM golden list
- `TestCoreKeywordNotIdentifier`: Core keywords rejected as unquoted identifiers
- `TestContextKeywordAsIdentifier`: Context keywords accepted as identifiers + bare aliases
- `TestKeywordCasePreservation`: Keyword tokens preserve original case

## Reference

- Enforcement tests: `mssql/parser/keyword_classification_test.go`
- Build check: `go build ./mssql/...`
- Full regression: `go test ./mssql/... -count=1`
- SqlScriptDOM reference: `../SqlScriptDOM/SqlScriptDom/Parser/TSql/`

---

## Phase 1: Infrastructure (sequential — shared files)

### Section 1.1: Lexer case preservation (1 site)

- [ ] keyword tokens preserve original case in Str field (currently lowercased)
- [ ] `SELECT MyPartition FROM t` → AST stores "MyPartition" not "mypartition"
- [ ] `CREATE TABLE MyWindow (a INT)` → table name preserves case
- [ ] non-keyword identifiers still preserve case (no regression)
- [ ] reverseKeywordMap still works for completion (maps token type → uppercase string)

Verification: `go test ./mssql/parser/ -run TestKeywordCasePreservation -count=1`

### Section 1.2: Classification infrastructure

- [ ] `KeywordCategory` type defined (Core, Context)
- [ ] `Keyword` struct with Name, Token, Category fields
- [ ] classification table mapping each keyword to its category
- [ ] 180 Core keywords match SqlScriptDOM golden list exactly
- [ ] 279 Context keywords added to keywordMap and classified
- [ ] all existing keywordMap entries classified (no unclassified)
- [ ] `lookupKeywordCategory(token int) KeywordCategory` helper available
- [ ] `isContextKeyword(token int) bool` helper available
- [ ] completion: reverseKeywordMap includes all new context keywords
- [ ] completion: `Collect()` on empty input returns context keywords as candidates

Verification: `go test ./mssql/parser/ -run TestKeywordClassification -count=1` + `go test ./mssql/parser/ -run TestKeywordCompleteness -count=1`

### Section 1.3: parseIdentifier classification-aware

- [ ] `isIdentLike()` rejects Core keywords, accepts Context keywords and tokIDENT
- [ ] `parseIdentifier()` rejects Core keywords, accepts Context keywords and tokIDENT
- [ ] bracket-quoted Core keywords accepted: `[select]` parses as identifier
- [ ] `isColLabel()` or equivalent accepts ALL keywords (for alias positions)
- [ ] Core keyword as table name fails: `CREATE TABLE select (a INT)` → error
- [ ] Context keyword as table name works: `CREATE TABLE window (a INT)` → success

Verification: `go test ./mssql/parser/ -run TestCoreKeywordNotIdentifier -count=1`

### Section 1.4: Fix tokIDENT-gated paths (85 sites across 18 files)

These are `.Type == tokIDENT` checks that must be widened to also accept Context keyword tokens, otherwise context keywords stop working as identifiers after registration.

- [ ] parser.go (29): statement dispatch lookahead — next.Type == tokIDENT checks for context-sensitive statement recognition
- [ ] create_table.go (21): PERSISTED, SPARSE, HIDDEN, MASKED, ENCRYPTED, GENERATED, ALWAYS, NODE, EDGE, PERIOD, SYSTEM_TIME, FILESTREAM_ON, TEXTIMAGE_ON, column options
- [ ] select.go (9): bare alias detection, FETCH NEXT/FIRST/ONLY/ROW/TIES, OVER clause
- [ ] merge.go (5): USING, MATCHED, TARGET, SOURCE
- [ ] execute.go (3): OUT parameter, exec string parsing
- [ ] alter_table.go (2): PERIOD, SYSTEM_TIME
- [ ] backup_restore.go (2): option parsing
- [ ] create_proc.go (2): OUT parameter
- [ ] name.go (2): base identifier checks
- [ ] update_delete.go (2): output clause context
- [ ] control_flow.go (1): TIME in WAITFOR
- [ ] create_database.go (1): database options
- [ ] create_sequence.go (1): sequence options
- [ ] create_trigger.go (1): trigger timing
- [ ] drop.go (1): CASCADE
- [ ] expr.go (1): expression parsing
- [ ] fulltext.go (1): fulltext options
- [ ] type.go (1): type name parsing

Verification: `go test ./mssql/parser/ -run TestContextKeywordAsIdentifier -count=1` + `go test ./mssql/... -count=1`

---

## Phase 2: String match migration (per-file, parallelizable after Phase 1)

Replace all `strings.EqualFold` and `matchesKeywordCI` calls with token type checks. Each section covers one or more parser files.

### Section 2.1a: parser.go — CREATE dispatch (~70 sites)

- [ ] replace matchesKeywordCI in CREATE sub-dispatch (CREATE CERTIFICATE, CREATE MASTER KEY, CREATE SYMMETRIC KEY, CREATE ASSEMBLY, CREATE SERVICE, etc.)
- [ ] replace matchesKeywordCI in CREATE lookahead (next token checks for statement type recognition)
- [ ] all CREATE statement recognition uses keyword tokens

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep parser.go` count decreasing

### Section 2.1b: parser.go — ALTER/DROP dispatch (~70 sites)

- [ ] replace matchesKeywordCI in ALTER sub-dispatch (ALTER DATABASE, ALTER ENDPOINT, ALTER AVAILABILITY GROUP, etc.)
- [ ] replace matchesKeywordCI in DROP sub-dispatch
- [ ] all ALTER/DROP statement recognition uses keyword tokens

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep parser.go` count decreasing

### Section 2.1c: parser.go — remaining dispatch + eqFold (~72 sites)

- [ ] replace 1 strings.EqualFold with token check
- [ ] replace remaining matchesKeywordCI in standalone statements (SEND, RECEIVE, GET CONVERSATION, etc.)
- [ ] replace matchesKeywordCI in expression/misc dispatch paths
- [ ] parser.go contributes 0 violations

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep parser.go` → 0

### Section 2.2: service_broker.go (60 sites)

- [ ] replace 60 matchesKeywordCI with token checks
- [ ] CREATE/ALTER message type, contract, queue, service, route, binding, priority, event notification

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep service_broker` → 0

### Section 2.3: fulltext.go (45 sites)

- [ ] replace 45 matchesKeywordCI with token checks
- [ ] CREATE/ALTER fulltext index, catalog, stoplist

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep fulltext` → 0

### Section 2.4: select.go (38 sites)

- [ ] replace 38 strings.EqualFold with token checks
- [ ] WINDOW clause, FETCH NEXT/FIRST, GROUPING SETS, ROLLUP, CUBE, XMLNAMESPACES
- [ ] OPTION hints: LOOP/HASH/MERGE JOIN, FAST, MAXDOP, MAXRECURSION, OPTIMIZE, etc.
- [ ] OVER clause: PARTITION, ORDER, ROWS, RANGE, GROUPS

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep 'select\.go'` → 0

### Section 2.5: security_principals.go (36 sites)

- [ ] replace 36 matchesKeywordCI with token checks
- [ ] CREATE/ALTER LOGIN, USER, ROLE, APPLICATION ROLE

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep security_principals` → 0

### Section 2.6: create_table.go (34 sites) + alter_table.go (18 sites)

- [ ] replace 34 strings.EqualFold in create_table.go with token checks
- [ ] replace 18 strings.EqualFold in alter_table.go with token checks
- [ ] PERSISTED, SPARSE, HIDDEN, MASKED, ENCRYPTED, GENERATED, ALWAYS
- [ ] PERIOD, SYSTEM_TIME, FILESTREAM_ON, TEXTIMAGE_ON
- [ ] NODE, EDGE, FILETABLE, CHANGE_TRACKING

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -E 'create_table|alter_table'` → 0

### Section 2.7: server.go (30 sites) + availability.go (23 sites)

- [ ] replace 30 matchesKeywordCI in server.go with token checks
- [ ] replace 23 matchesKeywordCI in availability.go with token checks
- [ ] server configuration options, HADR cluster, availability group/replica/listener

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -E 'server\.|availability'` → 0

### Section 2.8: alter_objects.go (23 sites) + external.go (22 sites)

- [ ] replace 2 strings.EqualFold + 21 matchesKeywordCI in alter_objects.go
- [ ] replace 22 matchesKeywordCI in external.go
- [ ] ALTER DATABASE, ALTER ENDPOINT, etc.
- [ ] CREATE EXTERNAL data source, file format, table, etc.

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -E 'alter_objects|external'` → 0

### Section 2.9: utility.go (20 sites) + endpoint.go (17 sites)

- [ ] replace 9 strings.EqualFold + 11 matchesKeywordCI in utility.go
- [ ] replace 17 strings.EqualFold in endpoint.go
- [ ] SET options, DBCC, miscellaneous statements
- [ ] CREATE/ALTER ENDPOINT protocol and payload options

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -E 'utility|endpoint'` → 0

### Section 2.10: event.go (15 sites) + security_misc.go (14 sites) + security_audit.go (10 sites)

- [ ] replace 15 matchesKeywordCI in event.go
- [ ] replace 14 matchesKeywordCI in security_misc.go
- [ ] replace 10 matchesKeywordCI in security_audit.go
- [ ] CREATE EVENT SESSION, security policies, audit specifications

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -E 'event\.|security_misc|security_audit'` → 0

### Section 2.11: backup_restore.go (14 sites) + create_database.go (13 sites)

- [ ] replace 14 strings.EqualFold in backup_restore.go
- [ ] replace 13 matchesKeywordCI in create_database.go
- [ ] BACKUP/RESTORE options, WITH clauses
- [ ] CREATE/ALTER DATABASE options, filegroup, containment

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -E 'backup_restore|create_database'` → 0

### Section 2.12: declare_set.go (12 sites) + partition.go (10 sites) + security_keys.go (10 sites)

- [ ] replace 1 strings.EqualFold + 11 matchesKeywordCI in declare_set.go
- [ ] replace 10 matchesKeywordCI in partition.go
- [ ] replace 4 strings.EqualFold + 6 matchesKeywordCI in security_keys.go
- [ ] DECLARE/SET variable options, partition functions/schemes, key management

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -E 'declare_set|partition\.|security_keys'` → 0

### Section 2.13: remaining 18 files (70 sites total)

- [ ] assembly.go (7): replace matchesKeywordCI
- [ ] execute.go (7): replace strings.EqualFold
- [ ] drop.go (7): replace 5 eqFold + 2 matchesKeywordCI
- [ ] create_trigger.go (6): replace strings.EqualFold
- [ ] create_proc.go (6): replace strings.EqualFold
- [ ] control_flow.go (6): replace 2 eqFold + 4 matchesKeywordCI
- [ ] merge.go (5): replace strings.EqualFold
- [ ] create_index.go (5): replace matchesKeywordCI
- [ ] create_view.go (4): replace strings.EqualFold
- [ ] resource_governor.go (4): replace matchesKeywordCI
- [ ] create_statistics.go (3): replace strings.EqualFold
- [ ] create_sequence.go (2): replace strings.EqualFold
- [ ] create_schema.go (2): replace strings.EqualFold
- [ ] update_delete.go (2): replace strings.EqualFold
- [ ] insert.go (1): replace strings.EqualFold
- [ ] grant.go (1): replace matchesKeywordCI
- [ ] cursor.go (1): replace strings.EqualFold
- [ ] xml_schema.go (1): replace matchesKeywordCI

Verification: `go test ./mssql/parser/ -run TestNoStringKeywordMatch -count=1 2>&1 | grep -v _test` → 0 total

### Section 2.14: Delete matchesKeywordCI function

- [ ] remove `matchesKeywordCI` function from security_principals.go
- [ ] verify no remaining callers

Verification: `go build ./mssql/...` + `grep -r matchesKeywordCI mssql/parser/*.go` → 0

---

## Proof

### Section-local proof
Each section: verify its specific files contribute 0 violations to TestNoStringKeywordMatch.

### Global proof
After all sections: all 6 enforcement tests pass + `go test ./mssql/... -count=1` full regression green.
