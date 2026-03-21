---
name: pg-read-trace-skill
description: Traces the PostgreSQL Analyzer (analyze.go/analyze.c) to identify which system catalogs (e.g., pg_operator, pg_type, pg_proc) are queried to resolve a specific SQL statement or semantic element. Use this to identify dependencies.
---
> Deprecated: use `pg/semantic/SKILL.md` and the staged pipeline under `pg/semantic/` instead.

# PostgreSQL Read Trace Skill

You are an expert on the PostgreSQL Analyzer and Semantic Engine. Your task is to identify the **read dependencies** (lookups) of a PostgreSQL SQL statement or semantic element on the system catalogs.

This is Step 3 of building the PostgreSQL Semantic Fidelity Pipeline.

## Your Task

1. Receive a SQL statement or semantic element (e.g., `OpExpr`, `SelectStmt`).
2. Trace the analyzer's execution path in `pg/catalog/analyze.go` and/or `src/backend/parser/analyze.c`.
3. Identify all points where system catalogs are queried.
4. Output the results in structured JSON.

## Target Functions (PG Internal)

Look for calls to these core catalog lookup functions:
- `SearchSysCache`, `SearchSysCacheCopy`, `SearchSysCacheList`
- `heap_open` (on catalog relations)
- `GetRelation`, `TypeByOID`, `LookupOperatorExact` (in Omni's `catalog.go`)

## Search Strategy

- Trace the `transform*` functions in `analyze.c` (e.g., `transformExpr`, `transformSelectStmt`).
- Follow sub-calls into functions that resolve names or types (e.g., `OpernameGetOprid`).
- Identify the specific OIDs of the system tables being read.

## Expected Output Format

```json
{
  "element": "OpExpr",
  "reads": [
    { "catalog": "pg_operator", "lookup_key": "opname, left, right", "purpose": "resolve operator" },
    { "catalog": "pg_type", "lookup_key": "oid", "purpose": "check operand compatibility" },
    { "catalog": "pg_cast", "lookup_key": "source, target", "purpose": "resolve implicit casting" }
  ]
}
```

## Guidelines

- Focus on the *mandatory* reads required for semantic resolution.
- Distinguish between literal lookups (by name) and semantic lookups (by OID/signature).
- Ensure consistency between the C logic and the Go implementation in `analyze.go`.
