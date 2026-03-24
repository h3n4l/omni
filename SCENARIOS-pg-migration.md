# PG Catalog Migration Generator Scenarios

> Goal: Implement `func GenerateMigration(from, to *Catalog, diff *SchemaDiff) *MigrationPlan` that produces ordered, structured DDL operations from a SchemaDiff.
> Verification: LoadSQL two schemas, Diff, GenerateMigration, assert ops contain expected DDL. Roundtrip: apply generated SQL to `from` catalog, verify result matches `to`.
> Reference sources: omni pg/catalog diff types, pgschema internal/diff patterns, PostgreSQL DDL semantics

Status: [ ] pending, [x] passing, [~] partial (needs upstream change)

---

## Phase 1: Infrastructure + Core DDL Generation

### 1.1 MigrationPlan Types and Entry Point

- [ ] MigrationPlan, MigrationOp, MigrationOpType types compile
- [ ] GenerateMigration on empty SchemaDiff returns empty plan
- [ ] GenerateMigration on identical catalogs returns empty plan
- [ ] MigrationPlan.SQL() joins all statements with semicolons and newlines
- [ ] MigrationPlan.Summary() groups ops by type and counts add/drop/modify
- [ ] MigrationPlan.Filter() returns subset matching predicate
- [ ] MigrationPlan.HasWarnings() detects ops with non-empty Warning
- [ ] MigrationPlan.Warnings() returns only warning ops

### 1.2 Schema DDL

- [ ] CREATE SCHEMA for added schema
- [ ] DROP SCHEMA CASCADE for dropped schema
- [ ] ALTER SCHEMA OWNER TO for modified schema owner
- [ ] Schema operations ordered before table operations

### 1.3 Table CREATE and DROP

- [ ] CREATE TABLE with columns and inline PK/UNIQUE/CHECK
- [ ] CREATE TABLE with column defaults
- [ ] CREATE TABLE with NOT NULL columns
- [ ] CREATE TABLE with column identity (GENERATED ALWAYS AS IDENTITY)
- [ ] CREATE TABLE with generated column (GENERATED ALWAYS AS ... STORED)
- [ ] CREATE UNLOGGED TABLE for unlogged tables
- [ ] DROP TABLE CASCADE for dropped tables
- [ ] All identifiers double-quoted in generated DDL
- [ ] Schema-qualified table names in DDL

### 1.4 Column ALTER

- [ ] ALTER TABLE ADD COLUMN for new column
- [ ] ALTER TABLE ADD COLUMN with NOT NULL and DEFAULT
- [ ] ALTER TABLE DROP COLUMN for removed column
- [ ] ALTER TABLE ALTER COLUMN TYPE for type change (implicit cast)
- [ ] ALTER TABLE ALTER COLUMN TYPE ... USING for type change (no implicit cast)
- [ ] USING clause decision uses FindCoercionPathway, not heuristic
- [ ] Type change with existing default: DROP DEFAULT → ALTER TYPE → SET DEFAULT
- [ ] ALTER TABLE ALTER COLUMN SET NOT NULL
- [ ] ALTER TABLE ALTER COLUMN DROP NOT NULL
- [ ] ALTER TABLE ALTER COLUMN SET DEFAULT
- [ ] ALTER TABLE ALTER COLUMN DROP DEFAULT
- [ ] ALTER TABLE ALTER COLUMN ADD GENERATED ... AS IDENTITY
- [ ] ALTER TABLE ALTER COLUMN DROP IDENTITY
- [ ] Multiple column changes on same table batched into fewer statements

### 1.5 Constraint DDL

- [ ] ALTER TABLE ADD CONSTRAINT ... PRIMARY KEY
- [ ] ALTER TABLE ADD CONSTRAINT ... UNIQUE
- [ ] ALTER TABLE ADD CONSTRAINT ... CHECK (expr)
- [ ] ALTER TABLE ADD CONSTRAINT ... FOREIGN KEY REFERENCES
- [ ] FK with ON DELETE/UPDATE actions in generated DDL
- [ ] FK with DEFERRABLE INITIALLY DEFERRED in generated DDL
- [ ] Constraint with NOT VALID generates ADD CONSTRAINT ... NOT VALID
- [ ] ALTER TABLE DROP CONSTRAINT for removed constraints
- [ ] Modified constraint generated as DROP + ADD (no ALTER CONSTRAINT for structure)
- [ ] EXCLUDE constraint ADD/DROP generated correctly

### 1.6 Index DDL

- [ ] CREATE INDEX for added standalone index
- [ ] CREATE UNIQUE INDEX for unique indexes
- [ ] CREATE INDEX ... USING hash/gist/gin for non-btree
- [ ] CREATE INDEX with WHERE clause (partial index)
- [ ] CREATE INDEX with INCLUDE columns
- [ ] CREATE INDEX with expression columns
- [ ] CREATE INDEX with DESC/NULLS FIRST options
- [ ] DROP INDEX for removed index
- [ ] Modified index generated as DROP + CREATE (indexes can't be ALTERed)
- [ ] PK/UNIQUE backing indexes NOT generated (managed by constraint DDL)

### 1.7 Partition and Inheritance DDL

- [ ] CREATE TABLE ... PARTITION BY RANGE/LIST/HASH for partitioned table
- [ ] CREATE TABLE ... PARTITION OF ... FOR VALUES FROM ... TO for range partition
- [ ] CREATE TABLE ... PARTITION OF ... FOR VALUES IN (...) for list partition
- [ ] CREATE TABLE ... PARTITION OF ... DEFAULT for default partition
- [ ] ALTER TABLE REPLICA IDENTITY (DEFAULT/FULL/NOTHING) for replica identity change
- [ ] CREATE VIEW ... WITH CHECK OPTION for view check option
- [ ] Table inheritance: INHERITS clause in CREATE TABLE

---

## Phase 2: Functions, Sequences, Types, Views, Triggers

### 2.1 Sequence DDL

- [ ] CREATE SEQUENCE with all options (INCREMENT, MINVALUE, MAXVALUE, START, CACHE, CYCLE)
- [ ] DROP SEQUENCE for removed sequence
- [ ] ALTER SEQUENCE for modified properties (increment, min, max, cache, cycle)
- [ ] SERIAL-owned sequences not generated (managed by column)

### 2.2 Function and Procedure DDL

- [ ] CREATE FUNCTION with full signature (args, return type, language, body)
- [ ] CREATE FUNCTION with volatility, strictness, security, parallel, leakproof attributes
- [ ] Dollar-quoting for function body ($$...$$)
- [ ] DROP FUNCTION with argument types in signature
- [ ] CREATE OR REPLACE FUNCTION for modified function (body/attribute changes)
- [ ] CREATE PROCEDURE for added procedure
- [ ] DROP PROCEDURE with argument types
- [ ] Modified procedure generated as DROP + CREATE (no CREATE OR REPLACE PROCEDURE in older PG)
- [ ] Overloaded functions produce distinct DDL (different arg signatures)

### 2.3 Enum Type DDL

- [ ] CREATE TYPE ... AS ENUM ('val1', 'val2') for added enum
- [ ] DROP TYPE for removed enum
- [ ] ALTER TYPE ADD VALUE 'new_val' for appended value
- [ ] ALTER TYPE ADD VALUE 'new_val' AFTER 'existing' for positioned value
- [ ] ALTER TYPE ADD VALUE 'new_val' BEFORE 'existing' for positioned value
- [ ] Enum value removal generates Warning (PG limitation)
- [ ] ALTER TYPE ADD VALUE cannot run in transaction (Transactional = false)

### 2.4 Domain Type DDL

- [ ] CREATE DOMAIN for added domain (base type, default, NOT NULL, constraints)
- [ ] DROP DOMAIN for removed domain
- [ ] ALTER DOMAIN SET DEFAULT / DROP DEFAULT
- [ ] ALTER DOMAIN SET NOT NULL / DROP NOT NULL
- [ ] ALTER DOMAIN ADD CONSTRAINT / DROP CONSTRAINT

### 2.5 Trigger DDL

- [ ] CREATE TRIGGER with timing, events, level, function
- [ ] CREATE TRIGGER with WHEN clause
- [ ] CREATE TRIGGER with REFERENCING OLD/NEW TABLE AS
- [ ] DROP TRIGGER ON table for removed trigger
- [ ] Modified trigger generated as DROP + CREATE

### 2.6 View and Materialized View DDL

- [ ] CREATE VIEW AS ... for added view
- [ ] DROP VIEW for removed view
- [ ] CREATE OR REPLACE VIEW for modified view (definition change)
- [ ] CREATE MATERIALIZED VIEW AS ... for added matview
- [ ] DROP MATERIALIZED VIEW for removed matview
- [ ] Modified matview generated as DROP + CREATE (no CREATE OR REPLACE)
- [ ] View dependency chain: DROP dependents before target, recreate after
- [ ] Matview indexes generated after matview creation

### 2.7 Range Type DDL

- [ ] CREATE TYPE ... AS RANGE for added range type
- [ ] DROP TYPE for removed range type
- [ ] Range subtype change generates Warning (requires DROP + CREATE, may break dependents)

### 2.8 Extension DDL

- [ ] CREATE EXTENSION for added extension
- [ ] DROP EXTENSION for removed extension
- [ ] Extension schema change generates appropriate DDL
- [ ] Extension operations ordered before types and tables

---

## Phase 3: Metadata, Ordering, and Roundtrip

### 3.1 Policy and RLS DDL

- [ ] CREATE POLICY on table for added policy
- [ ] DROP POLICY on table for removed policy
- [ ] ALTER POLICY for simple changes (roles, USING, WITH CHECK)
- [ ] Complex policy change as DROP + CREATE
- [ ] ALTER TABLE ENABLE ROW LEVEL SECURITY
- [ ] ALTER TABLE DISABLE ROW LEVEL SECURITY
- [ ] ALTER TABLE FORCE ROW LEVEL SECURITY
- [ ] ALTER TABLE NO FORCE ROW LEVEL SECURITY

### 3.2 Comment DDL

- [ ] COMMENT ON TABLE for added/changed comment
- [ ] COMMENT ON TABLE IS NULL for removed comment
- [ ] COMMENT ON COLUMN for column comments
- [ ] COMMENT ON INDEX, FUNCTION, SCHEMA, TYPE, SEQUENCE, CONSTRAINT, TRIGGER
- [ ] Comments generated after object creation

### 3.3 Grant DDL

- [ ] GRANT privilege ON object TO role for added grant
- [ ] REVOKE privilege ON object FROM role for removed grant
- [ ] GRANT WITH GRANT OPTION
- [ ] Modified grant generated as REVOKE + GRANT
- [ ] Grants generated after object creation

### 3.4 Operation Ordering

- [ ] DROP phase before CREATE phase before ALTER phase
- [ ] Within DROP: triggers → views → functions → constraints → indexes → columns → tables → sequences → types → extensions → schemas
- [ ] Within CREATE: schemas → extensions → types → sequences → tables (no FK) → functions → views → triggers → deferred FKs → indexes → policies
- [ ] FK constraints deferred until all tables created
- [ ] FK cycle detected — all FKs deferred to ALTER phase
- [ ] Types created before tables that use them
- [ ] Functions created before views/triggers that reference them

### 3.5 Roundtrip Verification

- [ ] Simple roundtrip: apply migration SQL to `from` catalog → Diff with `to` is empty
- [ ] Roundtrip with table add + column modify + index drop
- [ ] Roundtrip with function + view + trigger changes
- [ ] Roundtrip with enum add value + domain alter
- [ ] Roundtrip with FK across schemas
- [ ] Roundtrip with all object types simultaneously
