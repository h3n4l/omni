# PostgreSQL Semantic Pipeline Rebuild Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild the PostgreSQL semantic pipeline around Codex so it extracts, traces, and organizes statement-family semantic knowledge from `../postgres`, limited to top-level statement families that interact with system catalogs.

**Architecture:** Replace the current parser-batch-inspired `pg/catalog` pipeline with a family/scenario pipeline. A seeded family universe defines scope, Codex stage drivers process one family at a time, each stage writes structured JSON artifacts, and a synthesizer produces a per-family semantic inventory plus a global index. The source of truth is PostgreSQL C code and `src/test/regress`, not Omni's current `pg/catalog` implementation.

**Tech Stack:** Bash, Python 3, JSON, YAML or JSON seed files, Codex CLI orchestration, PostgreSQL source tree in `../postgres`

### Task 1: Define the new semantic scope model

**Files:**
- Create: `pg/semantic/README.md`
- Create: `pg/semantic/families.seed.json`
- Create: `pg/semantic/schema/family.schema.json`
- Create: `pg/semantic/schema/index.schema.json`
- Modify: `pg/catalog/PROGRESS.json`

**Step 1: Write the failing validation test for the seed universe**

Create a small validator test script that loads the future seed file and asserts:
- every family has a stable `id`
- every family has a `slug`
- every family has a `category`
- every family category is one of `ddl`, `dml`, `query`, `set`, `object_control`
- no banned low-level entries like `names`, `types`, `expressions`, `functions`

Expected family examples:
- `create_table`
- `alter_table`
- `drop`
- `select`
- `insert`
- `update`
- `delete`
- `merge`
- `set`
- `show`
- `create_index`
- `create_view`
- `create_schema`
- `create_function`
- `create_type`
- `create_extension`
- `create_sequence`
- `grant_revoke`
- `comment`
- `trigger`
- `policy`

**Step 2: Run the validator to verify it fails**

Run: `python3 scripts/pg-semantic/validate_seed.py`

Expected: FAIL because the file and validator do not exist yet.

**Step 3: Write the minimal scope model**

Create `pg/semantic/families.seed.json` with one object per family:

```json
{
  "version": 1,
  "source_root": "../postgres",
  "families": [
    {
      "id": "create_table",
      "slug": "create-table",
      "category": "ddl",
      "statements": ["CREATE TABLE", "CREATE TABLE AS", "SELECT INTO"],
      "entry_ast_nodes": ["CreateStmt", "CreateTableAsStmt"],
      "dispatch_roots": ["ProcessUtility", "transformOptionalSelectInto"],
      "status": "active"
    }
  ]
}
```

Create schemas for family files and the global index.

**Step 4: Run the validator to verify it passes**

Run: `python3 scripts/pg-semantic/validate_seed.py`

Expected: PASS with a summary of family count and category count.

**Step 5: Commit**

```bash
git add pg/semantic/README.md pg/semantic/families.seed.json pg/semantic/schema/family.schema.json pg/semantic/schema/index.schema.json pg/catalog/PROGRESS.json scripts/pg-semantic/validate_seed.py
git commit -m "plan: define pg semantic family scope model"
```

### Task 2: Replace the old progress model with family/scenario state

**Files:**
- Create: `pg/semantic/STATE.json`
- Create: `scripts/pg-semantic/next_family.py`
- Create: `scripts/pg-semantic/update_state.py`
- Modify: `scripts/pg-catalog-driver.sh`

**Step 1: Write the failing state transition test**

Add a test script that:
- creates a temporary `STATE.json`
- marks a family `discover=done`
- asserts `map` is now actionable
- asserts `trace_writes` and `trace_reads` stay blocked until `map=done`
- asserts `synthesize` remains blocked until all prior stages are done

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_state_machine.py`

Expected: FAIL because the state helpers do not exist yet.

**Step 3: Write the state model**

Create `pg/semantic/STATE.json` with this shape:

```json
{
  "version": 1,
  "families": [
    {
      "id": "create_table",
      "status": "pending",
      "stages": {
        "discover": "pending",
        "map": "pending",
        "trace_writes": "pending",
        "trace_reads": "pending",
        "map_tests": "pending",
        "plan_translation": "pending",
        "synthesize": "pending"
      },
      "scenario_count": 0,
      "updated_at": ""
    }
  ]
}
```

Implement `next_family.py` and `update_state.py` so stages can run independently and through a top-level driver.

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_state_machine.py`

Expected: PASS with stage gating assertions.

**Step 5: Commit**

```bash
git add pg/semantic/STATE.json scripts/pg-semantic/next_family.py scripts/pg-semantic/update_state.py scripts/pg-semantic/test_state_machine.py scripts/pg-catalog-driver.sh
git commit -m "plan: add family scenario state machine for pg semantic pipeline"
```

### Task 3: Create the family artifact layout

**Files:**
- Create: `pg/semantic/families/.gitkeep`
- Create: `pg/semantic/families/create_table.json`
- Create: `pg/semantic/index.json`
- Create: `scripts/pg-semantic/init_family_artifact.py`
- Create: `scripts/pg-semantic/validate_family_artifact.py`

**Step 1: Write the failing artifact validation test**

Add a validator test that expects each family artifact to contain:
- `family`
- `scope`
- `dispatcher`
- `handlers`
- `writes`
- `reads`
- `regress_files`
- `scenarios`
- `translation_targets`
- `evidence`
- `stage_status`

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_family_artifact.py`

Expected: FAIL because the artifact generator is missing.

**Step 3: Write the artifact initializer**

Create one JSON file per family under `pg/semantic/families/`. The file should start with placeholders and explicit stage ownership:

```json
{
  "family": {
    "id": "create_table",
    "slug": "create-table",
    "category": "ddl"
  },
  "scope": {
    "statements": [],
    "entry_ast_nodes": [],
    "dispatch_roots": []
  },
  "dispatcher": [],
  "handlers": [],
  "writes": [],
  "reads": [],
  "regress_files": [],
  "scenarios": [],
  "translation_targets": [],
  "evidence": [],
  "stage_status": {}
}
```

Generate `pg/semantic/index.json` from all family files.

**Step 4: Run the validator to verify it passes**

Run: `python3 scripts/pg-semantic/validate_family_artifact.py`

Expected: PASS with one line per valid family artifact.

**Step 5: Commit**

```bash
git add pg/semantic/families pg/semantic/index.json scripts/pg-semantic/init_family_artifact.py scripts/pg-semantic/validate_family_artifact.py scripts/pg-semantic/test_family_artifact.py
git commit -m "plan: create pg semantic family artifact layout"
```

### Task 4: Implement the new Codex stage contract

**Files:**
- Create: `pg/semantic/SKILL.md`
- Create: `pg/semantic/stages/discover.md`
- Create: `pg/semantic/stages/map.md`
- Create: `pg/semantic/stages/trace_writes.md`
- Create: `pg/semantic/stages/trace_reads.md`
- Create: `pg/semantic/stages/map_tests.md`
- Create: `pg/semantic/stages/plan_translation.md`
- Create: `pg/semantic/stages/synthesize.md`

**Step 1: Write the failing stage contract test**

Add a test that verifies each stage prompt file exists and contains:
- expected input artifact paths
- expected output artifact paths
- explicit reference to `../postgres`
- an instruction to emit only structured JSON or deterministic markers

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_stage_contracts.py`

Expected: FAIL because the stage prompts are not there.

**Step 3: Write the stage contracts**

Define each stage precisely:
- `discover`: refine scenarios inside the seeded family, never add new families
- `map`: top-level statement to dispatcher and primary handler chain
- `trace_writes`: handler to system catalog mutations
- `trace_reads`: analyzer and resolver to required catalog lookups
- `map_tests`: scenario to `src/test/regress/sql/*.sql`
- `plan_translation`: PG C functions to future translation targets
- `synthesize`: merge stage outputs into the family JSON and refresh the global index

The top-level `pg/semantic/SKILL.md` should instruct Codex how to choose the next family and next actionable stage.

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_stage_contracts.py`

Expected: PASS and list all seven stage contracts.

**Step 5: Commit**

```bash
git add pg/semantic/SKILL.md pg/semantic/stages scripts/pg-semantic/test_stage_contracts.py
git commit -m "plan: define codex stage contracts for pg semantic extraction"
```

### Task 5: Replace the old driver with a reusable Codex orchestrator

**Files:**
- Create: `scripts/pg-semantic-driver.sh`
- Create: `scripts/pg-semantic/run_stage.py`
- Modify: `scripts/pg-catalog-driver.sh`
- Modify: `scripts/stream-filter.py`
- Modify: `scripts/metrics.py`

**Step 1: Write the failing orchestrator test**

Add a test that simulates:
- selecting the next actionable family for a stage
- building the Codex prompt from the right stage contract
- writing a stage log to `pg/semantic/logs/`
- updating stage state on success

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_driver_flow.py`

Expected: FAIL because the driver and runner do not exist.

**Step 3: Write the new driver**

Create `scripts/pg-semantic-driver.sh` with flags:

```bash
./scripts/pg-semantic-driver.sh --stage map --max-families 5
./scripts/pg-semantic-driver.sh --family create_table --stage trace_writes
./scripts/pg-semantic-driver.sh --all
```

Requirements:
- use Codex, not Gemini
- read `pg/semantic/STATE.json`
- read and write family artifacts
- log every run under `pg/semantic/logs/`
- support full orchestration and single-stage reruns

Extend `stream-filter.py` and `metrics.py` to record `family`, `stage`, and `scenario_count`.

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_driver_flow.py`

Expected: PASS with one dry-run stage invocation.

**Step 5: Commit**

```bash
git add scripts/pg-semantic-driver.sh scripts/pg-semantic/run_stage.py scripts/pg-catalog-driver.sh scripts/stream-filter.py scripts/metrics.py scripts/pg-semantic/test_driver_flow.py
git commit -m "plan: add codex pg semantic pipeline orchestrator"
```

### Task 6: Implement family discovery and mapping against `../postgres`

**Files:**
- Create: `scripts/pg-semantic/discover_family.py`
- Create: `scripts/pg-semantic/map_family.py`
- Create: `scripts/pg-semantic/test_discover_map.py`
- Modify: `pg/semantic/stages/discover.md`
- Modify: `pg/semantic/stages/map.md`

**Step 1: Write the failing integration test**

Design a test around `create_table` and `select` that asserts:
- `discover` adds plausible scenarios but does not invent new families
- `map` records at least one dispatcher and one primary handler
- output references real `../postgres/src/backend/...` paths

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_discover_map.py`

Expected: FAIL because no stage implementation exists.

**Step 3: Write the stage helpers**

Implement wrappers that:
- prepare deterministic prompts for Codex
- pass family seed and current artifact state
- verify the returned JSON before writing it
- reject outputs that reference Omni code as source of truth

For `create_table`, expected mappings should resemble:
- dispatcher rooted in `ProcessUtility`
- primary handler chain including `DefineRelation`

For `select`, expected mappings should resemble:
- analyzer rooted in `transformSelectStmt`

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_discover_map.py`

Expected: PASS with validated dispatcher and handler fields.

**Step 5: Commit**

```bash
git add scripts/pg-semantic/discover_family.py scripts/pg-semantic/map_family.py scripts/pg-semantic/test_discover_map.py pg/semantic/stages/discover.md pg/semantic/stages/map.md
git commit -m "plan: implement discover and map stages for pg semantic families"
```

### Task 7: Implement catalog write and read tracing

**Files:**
- Create: `scripts/pg-semantic/trace_writes.py`
- Create: `scripts/pg-semantic/trace_reads.py`
- Create: `scripts/pg-semantic/test_trace_io.py`
- Modify: `pg/semantic/stages/trace_writes.md`
- Modify: `pg/semantic/stages/trace_reads.md`

**Step 1: Write the failing tracing test**

Build assertions for:
- `create_table` traces `pg_class`, `pg_attribute`, `pg_type`, `pg_depend`
- `select` traces catalog reads needed for type, operator, relation, and namespace resolution
- `set` records catalog reads only when semantically required

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_trace_io.py`

Expected: FAIL because the trace stages are missing.

**Step 3: Write the trace stages**

Requirements:
- output normalized catalog names
- record trigger functions for writes
- record lookup keys and purpose for reads
- attach evidence paths and function names from `../postgres`
- keep conditional branches explicit instead of flattening them away

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_trace_io.py`

Expected: PASS for representative family fixtures.

**Step 5: Commit**

```bash
git add scripts/pg-semantic/trace_writes.py scripts/pg-semantic/trace_reads.py scripts/pg-semantic/test_trace_io.py pg/semantic/stages/trace_writes.md pg/semantic/stages/trace_reads.md
git commit -m "plan: add pg semantic catalog read and write tracing"
```

### Task 8: Implement regress mapping and translation planning

**Files:**
- Create: `scripts/pg-semantic/map_regress.py`
- Create: `scripts/pg-semantic/plan_translation.py`
- Create: `scripts/pg-semantic/test_regress_translation.py`
- Modify: `pg/semantic/stages/map_tests.md`
- Modify: `pg/semantic/stages/plan_translation.md`

**Step 1: Write the failing coverage test**

Create tests that assert:
- each scenario can point at one or more `src/test/regress/sql/*.sql` files in `../postgres`
- translation targets are expressed as PG C function chains, not Omni files
- scenarios with no test are marked for manual review

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_regress_translation.py`

Expected: FAIL because the stage helpers do not exist.

**Step 3: Write the stage helpers**

The regress mapping stage should:
- search only under `../postgres/src/test/regress/sql/`
- map files to scenario coverage, not just family labels

The translation planning stage should:
- identify the future translation root
- list ordered PG C functions to port
- classify each target as `dispatcher`, `analyzer`, `resolver`, `catalog_writer`, or `validator`

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_regress_translation.py`

Expected: PASS with scenario coverage and ordered translation targets.

**Step 5: Commit**

```bash
git add scripts/pg-semantic/map_regress.py scripts/pg-semantic/plan_translation.py scripts/pg-semantic/test_regress_translation.py pg/semantic/stages/map_tests.md pg/semantic/stages/plan_translation.md
git commit -m "plan: map pg regress coverage and translation targets"
```

### Task 9: Implement synthesis and index generation

**Files:**
- Create: `scripts/pg-semantic/synthesize_family.py`
- Create: `scripts/pg-semantic/build_index.py`
- Create: `scripts/pg-semantic/test_synthesis.py`
- Modify: `pg/semantic/stages/synthesize.md`
- Modify: `pg/semantic/index.json`

**Step 1: Write the failing synthesis test**

Assert that synthesis:
- merges stage outputs into one family artifact
- refreshes the global index
- computes status summaries like `mapped`, `traced`, `covered`, `planned`
- preserves evidence paths for later review

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_synthesis.py`

Expected: FAIL because synthesis does not exist yet.

**Step 3: Write the synthesizer**

Requirements:
- never discard raw evidence produced by prior stages
- summarize scenario coverage counts
- mark family `status` conservatively
- rebuild `pg/semantic/index.json` from the family artifacts on every run

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_synthesis.py`

Expected: PASS with one synthesized family and one updated index entry.

**Step 5: Commit**

```bash
git add scripts/pg-semantic/synthesize_family.py scripts/pg-semantic/build_index.py scripts/pg-semantic/test_synthesis.py pg/semantic/stages/synthesize.md pg/semantic/index.json
git commit -m "plan: synthesize pg semantic family artifacts and index"
```

### Task 10: Migrate and deprecate the old pipeline

**Files:**
- Modify: `scripts/pg-catalog-driver.sh`
- Modify: `scripts/pg-bnf-source-mapper/SKILL.md`
- Modify: `scripts/pg-write-trace-skill/SKILL.md`
- Modify: `scripts/pg-read-trace-skill/SKILL.md`
- Modify: `scripts/pg-inventory-synthesizer/SKILL.md`
- Create: `docs/pg-semantic-pipeline-migration.md`

**Step 1: Write the failing migration check**

Add a check that fails if:
- old driver still points at `pg/catalog/PROGRESS.json`
- old skill docs still claim the parser-style batch model is authoritative
- no migration document exists

**Step 2: Run the check to verify it fails**

Run: `python3 scripts/pg-semantic/test_migration.py`

Expected: FAIL because migration is incomplete.

**Step 3: Write the migration layer**

Requirements:
- either convert `scripts/pg-catalog-driver.sh` into a thin compatibility wrapper or clearly deprecate it
- mark old skills as deprecated and point to `pg/semantic/`
- document old-to-new mapping:
  - parser batch -> family
  - mapper/write_trace/read_trace/synthesize -> new staged pipeline
  - `PG_SEMANTIC_INVENTORY.json` -> `pg/semantic/families/*.json` plus `pg/semantic/index.json`

**Step 4: Run the check to verify it passes**

Run: `python3 scripts/pg-semantic/test_migration.py`

Expected: PASS with migration notes present.

**Step 5: Commit**

```bash
git add scripts/pg-catalog-driver.sh scripts/pg-bnf-source-mapper/SKILL.md scripts/pg-write-trace-skill/SKILL.md scripts/pg-read-trace-skill/SKILL.md scripts/pg-inventory-synthesizer/SKILL.md docs/pg-semantic-pipeline-migration.md scripts/pg-semantic/test_migration.py
git commit -m "plan: deprecate old pg catalog pipeline in favor of codex semantic pipeline"
```

### Task 11: Verify the rebuilt pipeline end to end

**Files:**
- Modify: `scripts/pg-semantic-driver.sh`
- Modify: `scripts/metrics.py`
- Modify: `scripts/dashboard.go`
- Create: `scripts/pg-semantic/test_end_to_end.py`

**Step 1: Write the failing end-to-end test**

Create a dry-run integration test that executes:
- `discover`
- `map`
- `trace_writes`
- `trace_reads`
- `map_tests`
- `plan_translation`
- `synthesize`

for `create_table` and asserts the final family file and index are valid.

**Step 2: Run the test to verify it fails**

Run: `python3 scripts/pg-semantic/test_end_to_end.py`

Expected: FAIL because the complete pipeline is not wired yet.

**Step 3: Wire the verification path**

Requirements:
- expose family and stage visibility in metrics
- optionally extend the dashboard to show semantic pipeline progress separately from parser progress
- make the end-to-end dry run deterministic enough for CI

**Step 4: Run the test to verify it passes**

Run: `python3 scripts/pg-semantic/test_end_to_end.py`

Expected: PASS with one complete family run.

**Step 5: Commit**

```bash
git add scripts/pg-semantic-driver.sh scripts/metrics.py scripts/dashboard.go scripts/pg-semantic/test_end_to_end.py
git commit -m "plan: verify codex pg semantic pipeline end to end"
```

## Notes for the Implementer

- Do not reuse `pg/catalog/PROGRESS.json` as the primary semantic state store. Its parser-derived batch taxonomy is the wrong shape for this pipeline.
- Do not use Omni `pg/catalog/*.go` as the source of truth for semantic extraction. It can be used as a comparison target later, not as evidence.
- Every structured artifact should store evidence pointing into `../postgres`.
- Keep family scope controlled by `pg/semantic/families.seed.json`. Discovery may add scenarios, not new family IDs.
- Prefer JSON for machine-written artifacts. Add Markdown only for migration docs and operator-facing overview docs.
- Preserve rerun safety. A failed stage should be restartable without wiping prior completed stage outputs.

## Suggested initial family seed

- `create_table`
- `alter_table`
- `drop`
- `create_index`
- `create_view`
- `create_schema`
- `create_function`
- `create_type`
- `create_extension`
- `create_sequence`
- `comment`
- `grant_revoke`
- `trigger`
- `policy`
- `select`
- `insert`
- `update`
- `delete`
- `merge`
- `set`
- `show`

## Verification commands after implementation

```bash
python3 scripts/pg-semantic/validate_seed.py
python3 scripts/pg-semantic/validate_family_artifact.py
python3 scripts/pg-semantic/test_state_machine.py
python3 scripts/pg-semantic/test_stage_contracts.py
python3 scripts/pg-semantic/test_driver_flow.py
python3 scripts/pg-semantic/test_discover_map.py
python3 scripts/pg-semantic/test_trace_io.py
python3 scripts/pg-semantic/test_regress_translation.py
python3 scripts/pg-semantic/test_synthesis.py
python3 scripts/pg-semantic/test_migration.py
python3 scripts/pg-semantic/test_end_to_end.py
./scripts/pg-semantic-driver.sh --family create_table --all
```
