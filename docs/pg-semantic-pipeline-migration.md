# PG Semantic Pipeline Migration

The PostgreSQL semantic pipeline now lives under `pg/semantic/`.

## What Changed

- Old location: `pg/catalog/PROGRESS.json`
- New location: `pg/semantic/STATE.json`

- Old artifact: root `PG_SEMANTIC_INVENTORY.json`
- New artifacts:
  - `pg/semantic/families/*.json`
  - `pg/semantic/index.json`

- Old execution entrypoint: `scripts/pg-catalog-driver.sh`
- New execution entrypoint: `scripts/pg-semantic-driver.sh`

## Model Migration

- Old parser-like batch -> new semantic `family`
- Old batch-internal coverage -> new `scenario`
- Old `mapper` -> new `map`
- Old `write_trace` -> new `trace_writes`
- Old `read_trace` -> new `trace_reads`
- Old `synthesize` -> new `synthesize`

## Source of Truth

The new pipeline uses `../postgres` as the semantic source of truth.
Omni `pg/catalog/*.go` is no longer authoritative semantic evidence.

## Scope Changes

The old model mixed in non-top-level parser groups such as:
- `names`
- `types`
- `expressions`
- `functions`

The new model only tracks top-level statement families with stable system catalog semantics:
- DDL
- DML
- Query
- SET/SHOW
- Object control

## Compatibility

`scripts/pg-catalog-driver.sh` remains as a deprecated wrapper that forwards to `scripts/pg-semantic-driver.sh`.
