---
name: pg-inventory-synthesizer
description: Synthesizes the outputs of pg-bnf-source-mapper, pg-write-trace-skill, and pg-read-trace-skill into a master PG_SEMANTIC_INVENTORY.json. It maps statements to regression tests (pg_regress) to provide a complete coverage plan.
---
# PostgreSQL Inventory Synthesizer

You are the project manager and architect of the PostgreSQL Semantic Fidelity Pipeline. Your task is to **synthesize** all audit results into a master inventory.

This is the final Step (Step 4) of building the PostgreSQL Semantic Inventory.

## Your Task

1. Collect the JSON outputs from:
   - `pg-bnf-source-mapper` (Syntax to Source)
   - `pg-write-trace-skill` (Side-effects)
   - `pg-read-trace-skill` (Dependencies)
2. Map each statement to its relevant `pg_regress` test files (search for SQL statements in `src/test/regress/sql/`).
3. Compile a master `PG_SEMANTIC_INVENTORY.json` that represents the "Ground Truth" for full semantic coverage.
4. Update the master `PROGRESS.json` for the `pg/catalog` implementation.

## Strategy

- Group scenarios by logical statement type (e.g., `CREATE TABLE`, `ALTER TABLE`).
- For each statement, list all identified "Semantic Scenarios" (basic, inheritance, partitioned, etc.).
- Ensure every BNF statement from the parser is represented in this inventory.

## Expected Master JSON Structure

```json
{
  "statement": "CREATE TABLE",
  "bnf_slug": "create-table-stmt",
  "pg_source": "src/backend/commands/tablecmds.c:DefineRelation",
  "semantic_batches": [
    {
      "scenario": "basic_heap_table",
      "affects": ["pg_class", "pg_attribute", "pg_type", "pg_namespace"],
      "reads": ["pg_namespace", "pg_authid"],
      "regress_files": ["create_table.sql"]
    },
    {
      "scenario": "inherited_table",
      "affects": ["pg_class", "pg_attribute", "pg_inherits", "pg_depend"],
      "reads": ["pg_class", "pg_attribute"],
      "regress_files": ["inherit.sql"]
    }
  ]
}
```

## Guidelines

- This is the final blueprint for "Semantic Completeness".
- Ensure that the `regress_files` field contains enough tests to trigger all identified `affects` and `reads`.
- If a statement has zero regression tests (unlikely), mark it for "Manual Semantic Review".
