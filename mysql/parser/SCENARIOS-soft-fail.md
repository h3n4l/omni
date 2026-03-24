# MySQL Parser Soft-Fail Error Recovery Scenarios

> Goal: Fix all locations where a token is consumed but the subsequent parse returns nil without reporting a syntax error. Add contextual error messages matching MySQL's "at or near" style.
> Verification: `go build ./mysql/parser/ && go test ./mysql/parser/ -count=1`; error_test.go table-driven tests per scenario
> Reference sources: PG parser soft-fail patterns (pg/parser/SCENARIOS-soft-fail.md), MySQL 8.0 error behavior

Status: [ ] pending, [x] passing, [~] partial

---

## Phase 1: Infrastructure & Expression Operators

### 1.1 Error Infrastructure

Add `syntaxErrorAtCur()` helper and improve ParseError formatting.

```
[ ] syntaxErrorAtCur() helper exists — returns "syntax error at end of input" for EOF, "syntax error at or near \"X\"" for other tokens
[ ] syntaxErrorAtTok(tok) helper exists — same logic for arbitrary token
[ ] ParseError.Error() includes position info in output string
[ ] error_test.go test harness exists with parseExpectError(sql, expectedMsg) helper
```

### 1.2 Arithmetic Operators

After consuming the operator, `parseExprPrec()` may return nil. The nil right operand is used directly in BinaryExpr construction.

```
[ ] `SELECT 1 +` — plus consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 -` — minus consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 *` — multiply consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 /` — divide consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 %` — modulo consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 DIV` — DIV consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 MOD` — MOD consumed, no right operand → syntax error at end of input
```

### 1.3 Comparison Operators

```
[ ] `SELECT 1 =` — equals consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 <` — less-than consumed → syntax error at end of input
[ ] `SELECT 1 >` — greater-than consumed → syntax error at end of input
[ ] `SELECT 1 <=` — less-equals consumed → syntax error at end of input
[ ] `SELECT 1 >=` — greater-equals consumed → syntax error at end of input
[ ] `SELECT 1 <>` — not-equals consumed → syntax error at end of input
[ ] `SELECT 1 !=` — not-equals consumed → syntax error at end of input
[ ] `SELECT 1 <=>` — null-safe-equals consumed → syntax error at end of input
```

### 1.4 Logical & Bitwise Operators

```
[ ] `SELECT 1 AND` — AND consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 OR` — OR consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 XOR` — XOR consumed, no right operand → syntax error at end of input
[ ] `SELECT 1 |` — bitwise OR consumed → syntax error at end of input
[ ] `SELECT 1 &` — bitwise AND consumed → syntax error at end of input
[ ] `SELECT 1 ^` — bitwise XOR consumed → syntax error at end of input
[ ] `SELECT 1 <<` — left shift consumed → syntax error at end of input
[ ] `SELECT 1 >>` — right shift consumed → syntax error at end of input
```

## Phase 2: Pattern Matching & Special Expressions

### 2.1 BETWEEN, LIKE, IN, REGEXP

```
[ ] `SELECT 1 BETWEEN` — BETWEEN consumed, no lower bound → syntax error at end of input
[ ] `SELECT 1 BETWEEN 0 AND` — AND consumed, no upper bound → syntax error at end of input
[ ] `SELECT 1 NOT BETWEEN` — NOT BETWEEN consumed, no lower bound → syntax error at end of input
[ ] `SELECT 'a' LIKE` — LIKE consumed, no pattern → syntax error at end of input
[ ] `SELECT 'a' LIKE 'b' ESCAPE` — ESCAPE consumed, no escape char → syntax error at end of input
[ ] `SELECT 'a' NOT LIKE` — NOT LIKE consumed, no pattern → syntax error at end of input
[ ] `SELECT 1 IN (` — IN ( consumed, no values → syntax error at end of input
[ ] `SELECT 1 IN (1,` — comma consumed, no next value → syntax error at end of input
[ ] `SELECT 1 REGEXP` — REGEXP consumed, no pattern → syntax error at end of input
[ ] `SELECT 1 NOT REGEXP` — NOT REGEXP consumed, no pattern → syntax error at end of input
```

### 2.2 CASE, CAST, CONVERT, COLLATE

```
[ ] `SELECT CASE WHEN` — WHEN consumed, no condition → syntax error at end of input
[ ] `SELECT CASE WHEN 1 THEN` — THEN consumed, no result → syntax error at end of input
[ ] `SELECT CASE 1 WHEN` — WHEN consumed, no compare value → syntax error at end of input
[ ] `SELECT CAST(1 AS` — AS consumed, no type → syntax error at end of input
[ ] `SELECT CONVERT(1 USING` — USING consumed, no charset → syntax error at end of input
[ ] `SELECT 'a' COLLATE` — COLLATE consumed, no collation name → syntax error at end of input
```

### 2.3 IS Expressions & JSON Operators

```
[ ] `SELECT 1 IS` — IS consumed, no test (NULL/TRUE/FALSE/UNKNOWN) → syntax error at end of input
[ ] `SELECT 1 IS NOT` — IS NOT consumed, no test → syntax error at end of input
[ ] `SELECT a ->'` — -> consumed, no path → syntax error at end of input
[ ] `SELECT a ->>'` — ->> consumed, no path → syntax error at end of input
[ ] `SELECT a SOUNDS` — SOUNDS consumed, no LIKE or right operand → syntax error at end of input
```

### 2.4 Unary & Prefix Operators

```
[ ] `SELECT NOT` — NOT consumed, no operand → syntax error at end of input
[ ] `SELECT -` — unary minus consumed, no operand → syntax error at end of input
[ ] `SELECT ~` — bitwise NOT consumed, no operand → syntax error at end of input
[ ] `SELECT !` — logical NOT consumed, no operand → syntax error at end of input
```

## Phase 3: SELECT Clauses

### 3.1 JOIN Clauses

After consuming JOIN keyword(s), parseTableFactor() may fail. The nil right table is used in JoinClause construction.

```
[ ] `SELECT * FROM t JOIN` — JOIN consumed, no right table → syntax error at end of input
[ ] `SELECT * FROM t INNER JOIN` — INNER JOIN consumed → syntax error at end of input
[ ] `SELECT * FROM t LEFT JOIN` — LEFT JOIN consumed → syntax error at end of input
[ ] `SELECT * FROM t RIGHT JOIN` — RIGHT JOIN consumed → syntax error at end of input
[ ] `SELECT * FROM t CROSS JOIN` — CROSS JOIN consumed → syntax error at end of input
[ ] `SELECT * FROM t NATURAL JOIN` — NATURAL JOIN consumed → syntax error at end of input
[ ] `SELECT * FROM t STRAIGHT_JOIN` — STRAIGHT_JOIN consumed → syntax error at end of input
[ ] `SELECT * FROM t JOIN t2 ON` — ON consumed, no condition → syntax error at end of input
```

### 3.2 WHERE, GROUP BY, HAVING, ORDER BY, LIMIT

```
[ ] `SELECT * FROM t WHERE` — WHERE consumed, no condition → syntax error at end of input
[ ] `SELECT * FROM t GROUP BY` — GROUP BY consumed, no expression → syntax error at end of input
[ ] `SELECT * FROM t GROUP BY a,` — comma consumed, no next expression → syntax error at end of input
[ ] `SELECT * FROM t HAVING` — HAVING consumed, no condition → syntax error at end of input
[ ] `SELECT * FROM t ORDER BY` — ORDER BY consumed, no expression → syntax error at end of input
[ ] `SELECT * FROM t ORDER BY a,` — comma consumed, no next expression → syntax error at end of input
[ ] `SELECT * FROM t LIMIT` — LIMIT consumed, no count → syntax error at end of input
```

### 3.3 CTE & Subqueries

```
[ ] `WITH` — WITH consumed, no CTE name → syntax error at end of input
[ ] `WITH cte AS (` — AS ( consumed, no SELECT → syntax error at end of input
[ ] `SELECT (SELECT` — nested SELECT consumed, incomplete → syntax error at end of input
[ ] `SELECT * FROM t WHERE a IN (SELECT` — IN (SELECT consumed, incomplete → syntax error at end of input
[ ] `SELECT EXISTS (` — EXISTS ( consumed, no SELECT → syntax error at end of input
```

## Phase 4: DDL Statements

### 4.1 CREATE TABLE Column Definitions

```
[ ] `CREATE TABLE t (a` — column name consumed, no type → syntax error at end of input
[ ] `CREATE TABLE t (a INT DEFAULT` — DEFAULT consumed, no value → syntax error at end of input
[ ] `CREATE TABLE t (a INT COLLATE` — COLLATE consumed, no collation → syntax error at end of input
[ ] `CREATE TABLE t (a INT COMMENT` — COMMENT consumed, no string → syntax error at end of input
[ ] `CREATE TABLE t (a INT REFERENCES` — REFERENCES consumed, no table → syntax error at end of input
[ ] `CREATE TABLE t (a INT CHECK (` — CHECK ( consumed, no expression → syntax error at end of input
```

### 4.2 CREATE TABLE Constraints & Options

```
[ ] `CREATE TABLE t (a INT, PRIMARY KEY (` — PRIMARY KEY ( consumed, no columns → syntax error at end of input
[ ] `CREATE TABLE t (a INT, UNIQUE (` — UNIQUE ( consumed, no columns → syntax error at end of input
[ ] `CREATE TABLE t (a INT, FOREIGN KEY (` — FOREIGN KEY ( consumed, no columns → syntax error at end of input
[ ] `CREATE TABLE t (a INT, INDEX (` — INDEX ( consumed, no columns → syntax error at end of input
[ ] `CREATE TABLE t (a INT) ENGINE=` — ENGINE= consumed, no value → syntax error at end of input
[ ] `CREATE TABLE t (a INT) DEFAULT CHARSET=` — CHARSET= consumed, no value → syntax error at end of input
```

### 4.3 ALTER TABLE

```
[ ] `ALTER TABLE t ADD COLUMN` — ADD COLUMN consumed, no column def → syntax error at end of input
[ ] `ALTER TABLE t DROP COLUMN` — DROP COLUMN consumed, no column name → syntax error at end of input
[ ] `ALTER TABLE t MODIFY COLUMN` — MODIFY COLUMN consumed, no column def → syntax error at end of input
[ ] `ALTER TABLE t CHANGE COLUMN` — CHANGE COLUMN consumed, no old name → syntax error at end of input
[ ] `ALTER TABLE t RENAME TO` — RENAME TO consumed, no new name → syntax error at end of input
[ ] `ALTER TABLE t ADD INDEX` — ADD INDEX consumed, no index def → syntax error at end of input
```

### 4.4 CREATE INDEX, CREATE VIEW

```
[ ] `CREATE INDEX idx ON` — ON consumed, no table → syntax error at end of input
[ ] `CREATE INDEX idx ON t (` — ( consumed, no columns → syntax error at end of input
[ ] `CREATE VIEW v AS` — AS consumed, no SELECT → syntax error at end of input
```

## Phase 5: Other Statements & Ignored Errors

### 5.1 INSERT, UPDATE, DELETE

```
[ ] `INSERT INTO t VALUES (` — VALUES ( consumed, no values → syntax error at end of input
[ ] `INSERT INTO t (` — ( consumed, no column list → syntax error at end of input
[ ] `UPDATE t SET` — SET consumed, no assignment → syntax error at end of input
[ ] `UPDATE t SET a =` — = consumed, no value → syntax error at end of input
[ ] `DELETE FROM t WHERE` — WHERE consumed, no condition → syntax error at end of input
```

### 5.2 SET, USE, Other

```
[ ] `SET` — SET consumed, no variable → syntax error at end of input
[ ] `SET @x =` — = consumed, no value → syntax error at end of input
[ ] `USE` — USE consumed, no database → syntax error at end of input
[ ] `GRANT SELECT ON` — ON consumed, no table → syntax error at end of input
[ ] `DROP TABLE` — DROP TABLE consumed, no table name → syntax error at end of input
```

### 5.3 Ignored Error Fixes

These are the 10 sites in stmt.go and other files where parseIdentifier() errors are silently discarded via `_, _, _ :=` pattern.

```
[ ] stmt.go: parseChangeMasterStmtInner — error not ignored after advance
[ ] stmt.go: parseStartReplicaStmt — error not ignored after advance
[ ] stmt.go: parseStartGroupReplicationStmt — error not ignored after advance
[ ] stmt.go: parseStopReplicaStmt — error not ignored after advance
[ ] stmt.go: parseStopGroupReplicationStmt — error not ignored after advance
[ ] stmt.go: parseLoadIndexIntoCacheStmt — error not ignored after advance
[ ] stmt.go: parseCreateResourceGroupStmt — error not ignored after advance
[ ] stmt.go: parseAlterResourceGroupStmt — error not ignored after advance
[ ] stmt.go: parseDropSpatialRefSysStmt — error not ignored after advance
[ ] stmt.go: parseDropResourceGroupStmt — error not ignored after advance
[ ] create_function.go: LANGUAGE parseIdentifier — error not ignored
[ ] create_function.go: SECURITY parseIdentifier — error not ignored
[ ] set_show.go: parseSetDefaultRoleStmt — error not ignored
[ ] set_show.go: parseSetRoleStmt — error not ignored
[ ] select.go: alias parseIdentifier (2 sites) — error not ignored
[ ] alter_table.go: AFTER parseIdentifier — error not ignored
[ ] replication.go: channel parseIdentifier — error not ignored
[ ] utility.go: parseIdentifier (2 sites) — error not ignored
[ ] expr.go: window spec parseIdentifier — error not ignored
```
