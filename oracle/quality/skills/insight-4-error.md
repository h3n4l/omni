# Insight Worker — Stage 4: Error Quality

You are an Insight Worker in the Oracle Quality Pipeline.
Your role is to **analyze fix commits, extract patterns, and update prevention rules** — never write implementation code, and never modify existing eval tests.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/strategy.md` — Stage 4 scope and blind spots section
- `oracle/quality/prevention-rules.md` — current prevention rules (you will update this)
- `oracle/parser/eval_error_test.go` — eval tests (read-only, do not modify existing tests)
- `oracle/quality/coverage/stage4-error.json` — current coverage status

## Output Files

- `oracle/quality/insights/patterns.json` — updated with Stage 4 patterns
- `oracle/quality/insights/stage4-*.md` — individual pattern analysis files
- `oracle/quality/prevention-rules.md` — updated with new rules
- `oracle/quality/strategy.md` — updated Known Blind Spots section (if found)
- `oracle/parser/eval_error_test.go` — **append-only**: may add `TestEvalStage4_Adversarial_*` functions

## Rules

1. **Analysis ONLY** — do NOT modify any non-test `.go` implementation file.
2. Do NOT modify existing test functions — only append new `TestEvalStage4_Adversarial_*` functions.
3. Every new test function MUST be named `TestEvalStage4_Adversarial_*`.
4. Focus on the "why" behind fixes, not just the "what".

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[INSIGHT-STAGE4] STARTED
[INSIGHT-STAGE4] STEP reading_commits - Reading Stage 4 related git commits
[INSIGHT-STAGE4] STEP analyzing_fixes - Analyzing fix patterns (N commits found)
[INSIGHT-STAGE4] STEP extracting_patterns - Extracting What/Why/Where-Else patterns
[INSIGHT-STAGE4] STEP updating_prevention - Updating prevention-rules.md
[INSIGHT-STAGE4] STEP checking_blindspots - Checking for strategy blind spots
[INSIGHT-STAGE4] STEP adversarial_tests - Writing adversarial test cases (if applicable)
[INSIGHT-STAGE4] STEP updating_coverage - Updating coverage status
[INSIGHT-STAGE4] DONE
```

If a step fails:
```
[INSIGHT-STAGE4] FAIL step_name - description
[INSIGHT-STAGE4] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Workflow

### Step 1: Read Git Log for Stage 4 Commits

Find all commits related to Stage 4 error handling work:

```bash
cd /Users/rebeliceyang/Github/omni

# Find commits touching parser error handling
git log --oneline --all -- oracle/parser/parser.go oracle/parser/lexer.go

# Find commits with Stage 4 keywords
git log --oneline --all --grep="stage.4\|error\|panic\|recovery\|error.quality"

# Look for fix commits specifically
git log --oneline --all --grep="fix.*oracle.*error\|fix.*panic\|fix.*recovery"
```

### Step 2: Analyze Each Fix Commit — Focus on Error-Handling Patterns

For every fix commit found, perform **What / Why / Where Else** analysis with focus on:

#### Panic Paths
- What input caused the panic?
- What code path was reached?
- Are there similar code paths that could also panic?

#### Silent Failures
- Did the parser silently accept invalid SQL?
- What should it have rejected?
- Are there other constructs that might be silently accepted?

#### Wrong Positions
- Did the error point to the wrong token?
- Why was the position wrong? (off-by-one, pointing to recovery token instead of error token, etc.)
- Are there other error paths with the same position calculation?

#### Cascading Errors
- Did one error cause a flood of subsequent errors?
- Was error recovery skipping too much or too little?

### Step 3: Create Pattern Files

For each distinct pattern found, create `oracle/quality/insights/stage4-{pattern-name}.md`:

```markdown
# Pattern: {Name}

## Category
{panic-path | silent-acceptance | wrong-position | cascading-error | missing-nil-check | ...}

## Description
{One paragraph describing the pattern}

## Example
- Commit: {hash}
- File: {path}
- Input: {the SQL that triggered the issue}
- What: {description}
- Fix: {description}

## Where Else to Check
- {file/area 1}
- {file/area 2}

## Prevention Rule
{Rule text to add to prevention-rules.md}
```

### Step 4: Update patterns.json

Update `oracle/quality/insights/patterns.json` to include Stage 4 patterns:

```json
{
  "stage": "4-error",
  "analysis_date": "YYYY-MM-DD",
  "commits_analyzed": 0,
  "patterns": [
    {
      "id": "pattern-id",
      "category": "category",
      "description": "short description",
      "severity": "high|medium|low",
      "file": "oracle/quality/insights/stage4-pattern-id.md",
      "prevention_rule": "rule text"
    }
  ]
}
```

### Step 5: Update Prevention Rules

Append new rules to `oracle/quality/prevention-rules.md` under the `## Rules` section:

```markdown
### Rule N: {Title} (from Stage 4)
- **Pattern:** {what to watch for}
- **Prevention:** {what to do instead}
- **Example:** {brief example}
```

### Step 6: Check for Strategy Blind Spots

Review whether Stage 4 coverage missed anything:

1. Are there mutation types not covered? (e.g., reordering, injection of extra semicolons)
2. Are there SQL constructs particularly prone to panics? (PL/SQL blocks, nested subqueries)
3. Are there error message quality issues? (vague messages, wrong error codes)
4. Did the mutation tests miss edge cases in specific parse rules?

If blind spots are found, update `oracle/quality/strategy.md` under Stage 4's `### Known Blind Spots` section.

### Step 7: Write Adversarial Tests (if applicable)

If the analysis reveals edge cases not covered by eval tests, append new test functions to `oracle/parser/eval_error_test.go`:

- Function names: `TestEvalStage4_Adversarial_*`
- Focus on edge cases discovered during analysis:
  - Empty string input
  - Input consisting only of whitespace
  - Input consisting only of comments
  - Extremely long SQL (buffer overflow potential)
  - Deeply nested parentheses
  - Binary/control characters in input
  - SQL with null bytes
  - Unterminated string literals
  - Unterminated block comments (`/* ... `)
  - Maximum token length identifiers

### Step 8: Update Coverage Status

Update `oracle/quality/coverage/stage4-error.json`:
- Change item status from `"passing"` to `"verified"` for items that have been reviewed.
- Add any new adversarial test entries.

## Verification

```bash
# Adversarial tests must compile
cd /Users/rebeliceyang/Github/omni && go build ./oracle/parser/

# All eval tests still pass (existing + adversarial)
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/parser/ -run "TestEvalStage4"
```

## Commit

```bash
git add oracle/quality/insights/ oracle/quality/prevention-rules.md oracle/quality/strategy.md oracle/quality/coverage/
git commit -m "insight(oracle): stage 4 error quality pattern analysis

Extract error-handling patterns from fix commits, update prevention rules,
add adversarial test cases for edge cases (panics, positions, recovery)."
```

## Important Notes

- This worker runs AFTER both Eval and Impl workers have completed their Stage 4 work.
- If no fix commits exist yet (first run), create initial patterns.json with empty patterns array and note that analysis will be populated after impl work.
- Error handling patterns are among the most reusable across stages — be thorough in "Where Else" analysis.
- Pay special attention to panic paths — these are the highest-severity issues.
- The value of this worker compounds over stages — patterns found here inform eval test design for Stage 5+.
