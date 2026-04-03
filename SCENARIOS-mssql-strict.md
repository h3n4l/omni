# SCENARIOS: MSSQL L2 Strict — Option Validation

## Goal

Eliminate all "omni too permissive" oracle mismatches where omni accepts invalid option names that SQL Server 2022 rejects. Drive TestKeywordOracleOptionPositions to 0 mismatches.

## Reference

- Oracle test: `mssql/parser/oracle_test.go` → `TestKeywordOracleOptionPositions` (80 cases, 33 mismatches)
- Oracle: SQL Server 2022 via testcontainers (`SET PARSEONLY ON`)
- SqlScriptDOM OptionsHelper classes: `../SqlScriptDOM/SqlScriptDom/Parser/TSql/*.cs`
- Build check: `go build ./mssql/...`
- Full regression: `go test ./mssql/... -count=1 -short`

## Current state

33 mismatches across 17 categories. 3 categories already correct (DBCC, table hints, triggers). All mismatches are "SQL Server REJECTS, omni ACCEPTS" — omni is too permissive in accepting any keyword as an option name.

---

## Phase 1: Infrastructure (sequential)

### Section 1.1: Option validation framework

- [ ] `optionSet` type defined for declaring valid option names per position
- [ ] helper function `isValidOption(opts optionSet) bool` checks current token against valid set
- [ ] helper function `expectOption(opts optionSet) (string, error)` consumes and validates
- [ ] optionSet supports both keyword tokens and identifier strings (for unregistered option names)
- [ ] enforcement test: `TestOptionValidation` verifies valid options accepted, invalid rejected
- [ ] framework does not break any existing parser tests

Verification: `go build ./mssql/...` + `go test ./mssql/... -count=1 -short`

---

## Phase 2: SET and database options (independent files)

### Section 2.1: SET predicate options (declare_set.go)

- [ ] `SET ANSI_NULLS ON` accepted (valid)
- [ ] `SET QUOTED_IDENTIFIER OFF` accepted (valid)
- [ ] `SET ARITHABORT ON` accepted (valid)
- [ ] `SET SELECT ON` rejected (invalid — SELECT is not a SET option)
- [ ] `SET FROM OFF` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM PredicateSetOptionsHelper (25 options)

Verification: oracle test `set_predicate/*` → 0 mismatches

### Section 2.2: ALTER DATABASE SET options (alter_objects.go)

- [ ] `ALTER DATABASE db SET RECOVERY SIMPLE` accepted (valid)
- [ ] `ALTER DATABASE db SET READ_ONLY` accepted (valid)
- [ ] `ALTER DATABASE db SET ANSI_NULLS ON` accepted (valid)
- [ ] `ALTER DATABASE db SET SELECT ON` rejected (invalid)
- [ ] `ALTER DATABASE db SET FROM OFF` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM DatabaseOptionKindHelper

Verification: oracle test `db_set/*` → 0 mismatches

---

## Phase 3: Index and DDL options (independent files)

### Section 3.1: Index WITH options (create_index.go, alter_objects.go)

- [ ] `WITH (FILLFACTOR = 80)` accepted (valid)
- [ ] `WITH (PAD_INDEX = ON)` accepted (valid — PARSEONLY may reject due to missing table, but parse should succeed)
- [ ] `WITH (SELECT = 1)` rejected (invalid)
- [ ] `WITH (FROM = 1)` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM IndexOptionHelper + IndexStateOption

Verification: oracle test `index/*` → 0 mismatches

### Section 3.2: CREATE TABLE WITH options (create_table.go)

- [ ] `WITH (MEMORY_OPTIMIZED = ON)` accepted (valid — PARSEONLY may reject due to engine, but parse should succeed)
- [ ] `WITH (SELECT = ON)` rejected (invalid)
- [ ] `WITH (FROM = ON)` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM TableOptionHelper

Verification: oracle test `create_table/*` → 0 mismatches

### Section 3.3: CREATE/ALTER PROC WITH options (create_proc.go)

- [ ] `WITH RECOMPILE` accepted (valid)
- [ ] `WITH ENCRYPTION` accepted (valid)
- [ ] `WITH SELECT` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM ProcedureOptionHelper

Verification: oracle test `proc/*` → 0 mismatches

### Section 3.4: CREATE VIEW WITH options (create_view.go)

- [ ] `WITH SCHEMABINDING` accepted (valid)
- [ ] `WITH ENCRYPTION` accepted (valid)
- [ ] `WITH SELECT` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM ViewOptionHelper

Verification: oracle test `view/*` → 0 mismatches

---

## Phase 4: Query options (select.go)

### Section 4.1: Query hints — OPTION clause (select.go)

- [ ] `OPTION (RECOMPILE)` accepted (valid)
- [ ] `OPTION (MAXDOP 1)` accepted (valid)
- [ ] `OPTION (SELECT)` rejected (invalid)
- [ ] `OPTION (FROM)` rejected (invalid)
- [ ] valid hint set matches SqlScriptDOM optimizer hint helpers

Verification: oracle test `query_hint/*` → 0 mismatches

### Section 4.2: FOR XML/JSON options (select.go)

- [ ] `FOR XML RAW, ELEMENTS` accepted (valid)
- [ ] `FOR XML RAW, ROOT('r')` accepted (valid)
- [ ] `FOR XML RAW, SELECT` rejected (invalid)
- [ ] `FOR JSON PATH, ROOT('r')` accepted (valid)
- [ ] `FOR JSON PATH, WITHOUT_ARRAY_WRAPPER` accepted (valid)
- [ ] `FOR JSON PATH, SELECT` rejected (invalid)
- [ ] valid option sets match SqlScriptDOM XmlForClauseOptionsHelper / JsonForClauseOptionsHelper

Verification: oracle test `for_xml/*` + `for_json/*` → 0 mismatches

### Section 4.3: Cursor options (cursor.go)

- [ ] `CURSOR FAST_FORWARD FOR` accepted (valid)
- [ ] `CURSOR SCROLL FOR` accepted (valid)
- [ ] `CURSOR STATIC FOR` accepted (valid)
- [ ] `CURSOR SELECT FOR` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM CursorOptionsHelper (13 options)

Verification: oracle test `cursor/*` → 0 mismatches

---

## Phase 5: Backup, restore, bulk operations (independent files)

### Section 5.1: Backup WITH options (backup_restore.go)

- [ ] `WITH COMPRESSION` accepted (valid)
- [ ] `WITH INIT` accepted (valid)
- [ ] `WITH SELECT` rejected (invalid)
- [ ] `WITH FROM` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM BackupOptionsNoValueHelper + BackupOptionsWithValueHelper

Verification: oracle test `backup/*` → 0 mismatches

### Section 5.2: Restore WITH options (backup_restore.go)

- [ ] `WITH NORECOVERY` accepted (valid)
- [ ] `WITH REPLACE` accepted (valid)
- [ ] `WITH SELECT` rejected (invalid)
- [ ] `WITH FROM` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM RestoreOptionNoValueHelper + RestoreOptionWithValueHelper

Verification: oracle test `restore/*` → 0 mismatches

### Section 5.3: Bulk insert WITH options (bulk_insert.go)

- [ ] `WITH (FIELDTERMINATOR = ',')` accepted (valid)
- [ ] `WITH (ROWTERMINATOR = '\n')` accepted (valid)
- [ ] `WITH (SELECT = 1)` rejected (invalid)
- [ ] `WITH (FROM = 1)` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM BulkInsertFlagOptionsHelper + IntOptionHelper + StringOptionHelper

Verification: oracle test `bulk_insert/*` → 0 mismatches

---

## Phase 6: Server and HA options (independent files)

### Section 6.1: Fulltext index options (fulltext.go)

- [ ] `WITH (CHANGE_TRACKING = AUTO)` accepted (valid)
- [ ] `WITH (SELECT = ON)` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM fulltext-related helpers

Verification: oracle test `fulltext/*` → 0 mismatches

### Section 6.2: Service broker options (service_broker.go)

- [ ] `CREATE MESSAGE TYPE msg VALIDATION = NONE` accepted (valid)
- [ ] `CREATE SERVICE svc ON QUEUE dbo.q (SELECT)` rejected (invalid — SELECT is not a contract name pattern)
- [ ] valid patterns match SqlScriptDOM service broker grammar

Verification: oracle test `broker/*` → 0 mismatches

### Section 6.3: Availability group options (availability.go)

- [ ] `WITH (AUTOMATED_BACKUP_PREFERENCE = SECONDARY)` accepted (valid)
- [ ] `WITH (SELECT = ON)` rejected (invalid)
- [ ] valid option set matches SqlScriptDOM AvailabilityReplicaOptionsHelper

Verification: oracle test `ag/*` → 0 mismatches

### Section 6.4: Endpoint options (endpoint.go)

- [ ] `STATE = STARTED AS TCP (LISTENER_PORT = 5022)` accepted (valid)
- [ ] `SELECT = STARTED AS TCP` rejected (invalid — SELECT is not an endpoint option)
- [ ] valid option set matches SqlScriptDOM EndpointProtocolOptionsHelper

Verification: oracle test `endpoint/*` → 0 mismatches

---

## Proof

### Section-local proof
Each section: its oracle test subcases show 0 mismatches.

### Global proof
After all sections: `TestKeywordOracleOptionPositions` → 0 mismatches total. `go test ./mssql/... -count=1 -short` all green.
