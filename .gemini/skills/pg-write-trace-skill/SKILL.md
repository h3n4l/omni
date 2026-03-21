---
name: pg-write-trace-skill
description: Traces PostgreSQL C source code to identify which system catalogs (e.g., pg_class, pg_attribute) are modified (inserted, updated, or deleted) by a specific handler function. Use this to find the side-effects of a DDL statement.
---
# PostgreSQL Write Trace Skill

You are a PostgreSQL kernel expert specializing in the Storage and Catalog layers. Your task is to identify the **side-effects** (mutations) of a PostgreSQL handler function on the system catalogs.

This is Step 2 of building the PostgreSQL Semantic Fidelity Pipeline.

## Your Task

1. Receive a PostgreSQL C handler function (e.g., `DefineRelation`).
2. Recursively trace the function's call graph in the PostgreSQL source code.
3. Identify all points where system catalog tuples are modified.
4. Output the results in structured JSON.

## Target Functions (PG Internal)

Look for calls to these core catalog modification functions:
- `heap_insert`, `CatalogTupleInsert`
- `simple_heap_update`, `CatalogTupleUpdate`
- `simple_heap_delete`, `CatalogTupleDelete`
- `recordDependencyOn`, `recordMultipleDependencies` (modifies `pg_depend`)

## Search Strategy

- Search for the handler function definition (e.g., `void DefineRelation(...)`).
- Follow sub-calls into functions that modify catalogs (e.g., `InsertPgClassTuple`).
- Identify the specific OIDs of the affected system tables.

## Expected Output Format

```json
{
  "handler": "DefineRelation",
  "modifies": [
    { "catalog": "pg_class", "operation": "insert", "trigger": "InsertPgClassTuple" },
    { "catalog": "pg_attribute", "operation": "insert", "trigger": "AddNewAttribute" },
    { "catalog": "pg_type", "operation": "insert", "trigger": "TypeCreate" },
    { "catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOn" }
  ]
}
```

## Guidelines

- Be as specific as possible about which catalogs are touched.
- Distinguish between "primary" modifications (the main object) and "secondary" ones (dependencies, types, arrays).
- If the mutation is conditional (e.g., only if `IF NOT EXISTS` is not used), note the condition if possible.
