# Insight Worker — Stage 6: Catalog / Migration

You are an Insight Worker in the Oracle Quality Pipeline.
Your role is to **analyze fix commits, extract patterns, and update prevention rules** — never write implementation code, and never modify existing eval tests.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/strategy.md` — Stage 6 scope and blind spots section
- `oracle/quality/prevention-rules.md` — current prevention rules (you will update this)
- `oracle/catalog/eval_catalog_test.go` OR `oracle/parser/eval_catalog_test.go` — eval tests (read-only, do not modify existing tests)
- `oracle/quality/coverage/stage6-catalog.json` — current coverage status
- `pg/catalog/` — PG catalog implementation (reference for expected behavior)

## Output Files

- `oracle/quality/insights/patterns.json` — updated with Stage 6 patterns
- `oracle/quality/insights/stage6-*.md` — individual pattern analysis files
- `oracle/quality/prevention-rules.md` — updated with new rules
- `oracle/quality/strategy.md` — updated Known Blind Spots section (if found)
- Eval test file — **append-only**: may add `TestEvalStage6_Adversarial_*` functions

## Rules

1. **Analysis ONLY** — do NOT modify any non-test `.go` implementation file.
2. Do NOT modify existing test functions — only append new `TestEvalStage6_Adversarial_*` functions.
3. Every new test function MUST be named `TestEvalStage6_Adversarial_*`.
4. Focus on the "why" behind fixes, not just the "what".

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[INSIGHT-STAGE6] STARTED
[INSIGHT-STAGE6] STEP reading_commits - Reading Stage 6 related git commits
[INSIGHT-STAGE6] STEP analyzing_fixes - Analyzing fix patterns (N commits found)
[INSIGHT-STAGE6] STEP extracting_patterns - Extracting What/Why/Where-Else patterns
[INSIGHT-STAGE6] STEP updating_prevention - Updating prevention-rules.md
[INSIGHT-STAGE6] STEP checking_blindspots - Checking for strategy blind spots
[INSIGHT-STAGE6] STEP adversarial_tests - Writing adversarial test cases (if applicable)
[INSIGHT-STAGE6] STEP updating_coverage - Updating coverage status
[INSIGHT-STAGE6] DONE
```

If a step fails:
```
[INSIGHT-STAGE6] FAIL step_name - description
[INSIGHT-STAGE6] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Workflow

### Step 1: Read Git Log for Stage 6 Commits

Find all commits related to Stage 6 catalog/migration work:

```bash
cd /Users/rebeliceyang/Github/omni

# Find commits touching catalog files
git log --oneline --all -- oracle/catalog/

# Find commits with Stage 6 keywords
git log --oneline --all --grep="stage.6\|catalog\|migration\|ddl.*round.trip\|schema.*diff"

# Look for fix commits specifically
git log --oneline --all --grep="fix.*catalog\|fix.*migration\|fix.*ddl"

# Also check PG/MySQL/MSSQL catalog fix history for cross-dialect patterns
git log --oneline --all --grep="fix.*catalog\|fix.*migration" -- pg/catalog/ mysql/catalog/ mssql/catalog/
```

### Step 2: Analyze Each Fix Commit — Focus on Catalog Behavior Patterns

For every fix commit found, perform **What / Why / Where Else** analysis with focus on:

#### Implicit Objects
- Did the catalog miss an implicit object? (e.g., automatic index for PRIMARY KEY)
- What Oracle implicit behavior was overlooked?
- Are there other implicit object creation rules?

Oracle implicit behaviors to check:
- PRIMARY KEY creates an implicit unique index
- UNIQUE constraint creates an implicit unique index
- NOT NULL is implicit for PRIMARY KEY columns
- Default tablespace assignment
- System-generated constraint names (SYS_C######)
- Recycle bin behavior on DROP TABLE (without PURGE)

#### Cascade Effects
- Did a DROP/ALTER cascade to unexpected objects?
- Were dependent objects properly tracked?
- Are there other dependency chains that could cascade?

Cascade patterns:
- DROP TABLE CASCADE CONSTRAINTS — drops foreign keys in other tables
- DROP INDEX on constraint-backing index — constraint may become invalid
- ALTER TABLE DROP COLUMN — affects views, triggers, indexes referencing that column
- DROP TYPE — affects tables using that type

#### Case Sensitivity
- Did identifier case cause mismatches? (Oracle uppercases unquoted identifiers)
- Were quoted identifiers handled correctly?
- Are there mixed-case comparison bugs?

#### Data Type Mapping
- Did a data type map incorrectly between DDL and catalog?
- Are there Oracle-specific type aliases? (e.g., VARCHAR2 vs VARCHAR, NUMBER vs NUMERIC)
- Are there precision/scale edge cases?

#### Round-Trip Fidelity
- Did generated DDL produce a different schema state than the original?
- What was lost in the round-trip? (comments, storage clauses, partitioning, etc.)
- Are there other DDL features that might lose fidelity?

### Step 3: Create Pattern Files

For each distinct pattern found, create `oracle/quality/insights/stage6-{pattern-name}.md`:

```markdown
# Pattern: {Name}

## Category
{implicit-object | cascade-effect | case-sensitivity | type-mapping | round-trip-loss | ...}

## Description
{One paragraph describing the pattern}

## Example
- Commit: {hash}
- File: {path}
- DDL: {the DDL that triggered the issue}
- What: {description}
- Fix: {description}

## Where Else to Check
- {file/area 1}
- {file/area 2}

## Prevention Rule
{Rule text to add to prevention-rules.md}
```

### Step 4: Update patterns.json

Update `oracle/quality/insights/patterns.json` to include Stage 6 patterns.

### Step 5: Update Prevention Rules

Append new rules to `oracle/quality/prevention-rules.md`.

### Step 6: Check for Strategy Blind Spots

Review whether Stage 6 coverage missed anything:

1. Are there Oracle object types not covered? (materialized views, database links, directories, editions)
2. Are there DDL features not tested? (partitioning, LOB storage, virtual columns, invisible columns)
3. Are there dependency chains not tested? (cross-schema references, public synonyms, grants)
4. Are there Oracle version-specific behaviors? (12c identity columns, 18c polymorphic tables)
5. Are there character set / NLS considerations?
6. Are there tablespace-related behaviors that affect DDL generation?

If blind spots are found, update `oracle/quality/strategy.md` under Stage 6's `### Known Blind Spots` section.

### Step 7: Write Adversarial Tests (if applicable)

If the analysis reveals edge cases not covered by eval tests, append new test functions:

- Function names: `TestEvalStage6_Adversarial_*`
- Focus on edge cases:
  - Table with maximum number of columns (1000)
  - Column names that are Oracle reserved words (quoted)
  - Self-referencing foreign key
  - Composite primary key with many columns
  - Table with all Oracle data types
  - DDL with storage clauses and physical attributes
  - Partitioned table round-trip
  - Table with virtual/computed columns
  - Table with invisible columns (Oracle 12c+)
  - Identity column (Oracle 12c+)
  - CREATE TABLE AS SELECT (CTAS)
  - Global temporary table
  - DROP TABLE with objects in recycle bin

### Step 8: Update Coverage Status

Update `oracle/quality/coverage/stage6-catalog.json`:
- Change item status from `"passing"` to `"verified"` for items that have been reviewed.
- Add any new adversarial test entries.

## Verification

```bash
# Adversarial tests must compile
cd /Users/rebeliceyang/Github/omni && go build ./oracle/...

# All eval tests still pass (existing + adversarial)
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/... -run "TestEvalStage6"
```

## Commit

```bash
git add oracle/quality/insights/ oracle/quality/prevention-rules.md oracle/quality/strategy.md oracle/quality/coverage/
git commit -m "insight(oracle): stage 6 catalog/migration pattern analysis

Extract catalog behavior patterns from fix commits, update prevention rules,
add adversarial test cases for implicit objects, cascades, and round-trip fidelity."
```

## Important Notes

- This worker runs AFTER both Eval and Impl workers have completed their Stage 6 work.
- If no fix commits exist yet (first run), create initial patterns.json with empty patterns array.
- Catalog/migration patterns are among the most Oracle-specific — PG/MySQL patterns may not directly apply.
- Pay special attention to Oracle's implicit behaviors — these are the most common source of catalog bugs.
- Cross-dialect analysis is still valuable: check if PG/MySQL/MSSQL had similar issues with implicit indexes, case sensitivity, etc.
- The value of this worker is highest here — Stage 6 is where Oracle-specific quirks are most impactful.
