#!/bin/bash
# Quality pipeline driver for Oracle parser: eval → impl → insight flow.
#
# Usage: ./quality-driver.sh <stage> [options]
#
# Stages: 1-foundation, 2-loc, 3-ast, 4-error, 5-completion, 6-catalog
#
# The pipeline runs three phases per stage:
#   Eval phase:    run eval skill to establish/check test baselines
#   Impl phase:    run impl skill in a loop until eval tests pass
#   Insight phase: run insight skill to discover new test cases
#   If insight adds failing tests, re-enter impl phase
#
# Options:
#   --eval-only              Run only the eval phase
#   --impl-only              Run only the impl phase
#   --insight-only           Run only the insight phase
#   --max-impl-iterations N  Max impl iterations per round (default 50)
#   --max-insight-rounds N   Max insight rounds (default 3)
#
# Examples:
#   ./quality-driver.sh 1-foundation
#   ./quality-driver.sh 2-loc --max-impl-iterations 30
#   ./quality-driver.sh 3-ast --eval-only

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OMNI_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse arguments
STAGE=""
EVAL_ONLY=false
IMPL_ONLY=false
INSIGHT_ONLY=false
MAX_IMPL_ITERATIONS=50
MAX_INSIGHT_ROUNDS=3

while [[ $# -gt 0 ]]; do
    case "$1" in
        --eval-only) EVAL_ONLY=true; shift ;;
        --impl-only) IMPL_ONLY=true; shift ;;
        --insight-only) INSIGHT_ONLY=true; shift ;;
        --max-impl-iterations) MAX_IMPL_ITERATIONS="$2"; shift 2 ;;
        --max-insight-rounds) MAX_INSIGHT_ROUNDS="$2"; shift 2 ;;
        *) STAGE="$1"; shift ;;
    esac
done

if [ -z "$STAGE" ]; then
    echo "Usage: $0 <stage> [--eval-only] [--impl-only] [--insight-only] [--max-impl-iterations N] [--max-insight-rounds N]"
    echo "Stages: 1-foundation, 2-loc, 3-ast, 4-error, 5-completion, 6-catalog"
    exit 1
fi

# Extract stage number (e.g., "1-foundation" → "1")
STAGE_NUM="${STAGE%%-*}"

if ! [[ "$STAGE_NUM" =~ ^[1-6]$ ]]; then
    echo "Error: invalid stage '$STAGE'. Must start with 1-6."
    exit 1
fi

# Paths
SKILLS_DIR="$OMNI_DIR/oracle/quality/skills"
LOG_DIR="$OMNI_DIR/oracle/quality/logs"
EVAL_SKILL="$SKILLS_DIR/eval-${STAGE}.md"
IMPL_SKILL="$SKILLS_DIR/impl-${STAGE}.md"
INSIGHT_SKILL="$SKILLS_DIR/insight-${STAGE}.md"

mkdir -p "$LOG_DIR"

echo "=== Oracle Quality Pipeline: Stage $STAGE ==="
echo "Working directory: $OMNI_DIR"
echo "Max impl iterations: $MAX_IMPL_ITERATIONS"
echo "Max insight rounds: $MAX_INSIGHT_ROUNDS"
echo ""

# ──────────────────────────────────────────────
# Regression guard: run eval tests for all prior stages
# ──────────────────────────────────────────────
run_regression_guard() {
    if [ "$STAGE_NUM" -le 1 ]; then
        echo "  No prior stages to check."
        return 0
    fi

    local prior_max=$((STAGE_NUM - 1))
    local pattern="TestEvalStage[1-${prior_max}]"
    echo "  Running regression guard: go test ./oracle/... -run '$pattern' -count=1"

    local log_file="$LOG_DIR/regression_${STAGE}_$(date '+%Y%m%d_%H%M%S').log"
    if cd "$OMNI_DIR" && go test ./oracle/... -run "$pattern" -count=1 2>&1 | tee "$log_file"; then
        echo "  Regression guard passed."
        return 0
    else
        echo "  ERROR: Regression guard failed! Prior stage tests are broken."
        echo "  Fix regressions before proceeding with stage $STAGE."
        return 1
    fi
}

# ──────────────────────────────────────────────
# Run eval tests and return exit code
# ──────────────────────────────────────────────
run_eval_tests() {
    local pattern="TestEvalStage${STAGE_NUM}"
    echo "  Running eval tests: go test ./oracle/... -run '$pattern' -count=1"
    cd "$OMNI_DIR" && go test ./oracle/... -run "$pattern" -count=1 2>&1
}

# ──────────────────────────────────────────────
# Eval phase: invoke eval skill
# ──────────────────────────────────────────────
run_eval() {
    echo ""
    echo "========================================="
    echo "  Phase: EVAL (stage $STAGE)"
    echo "========================================="

    if [ ! -f "$EVAL_SKILL" ]; then
        echo "  Error: eval skill not found: $EVAL_SKILL"
        return 1
    fi

    local timestamp
    timestamp=$(date '+%Y%m%d_%H%M%S')
    local log_file="$LOG_DIR/eval_${STAGE}_${timestamp}.log"

    echo "  Invoking eval skill..."
    cd "$OMNI_DIR" && claude -p "$(cat "$EVAL_SKILL")" \
        --dangerously-skip-permissions \
        2>&1 | tee "$log_file"

    echo "  Eval phase complete. Log: $log_file"
}

# ──────────────────────────────────────────────
# Impl phase: loop until eval tests pass
# ──────────────────────────────────────────────
run_impl() {
    echo ""
    echo "========================================="
    echo "  Phase: IMPL (stage $STAGE)"
    echo "========================================="

    if [ ! -f "$IMPL_SKILL" ]; then
        echo "  Error: impl skill not found: $IMPL_SKILL"
        return 1
    fi

    local iteration=0
    while [ $iteration -lt $MAX_IMPL_ITERATIONS ]; do
        iteration=$((iteration + 1))
        local timestamp
        timestamp=$(date '+%Y%m%d_%H%M%S')
        local log_file="$LOG_DIR/impl_${STAGE}_${iteration}_${timestamp}.log"

        # Check if eval tests already pass
        echo ""
        echo "  [Impl $iteration/$MAX_IMPL_ITERATIONS] Checking eval tests..."
        local test_exit=0
        run_eval_tests || test_exit=$?

        if [ "$test_exit" -eq 0 ]; then
            echo "  All eval tests pass. Impl phase complete."
            return 0
        fi

        echo "  Tests failing. Invoking impl skill..."
        cd "$OMNI_DIR" && claude -p "$(cat "$IMPL_SKILL")" \
            --dangerously-skip-permissions \
            2>&1 | tee "$log_file"

        sleep 2
    done

    echo "  Reached max impl iterations ($MAX_IMPL_ITERATIONS)."
    return 1
}

# ──────────────────────────────────────────────
# Insight phase: discover new tests, re-enter impl if needed
# ──────────────────────────────────────────────
run_insight() {
    echo ""
    echo "========================================="
    echo "  Phase: INSIGHT (stage $STAGE)"
    echo "========================================="

    if [ ! -f "$INSIGHT_SKILL" ]; then
        echo "  Error: insight skill not found: $INSIGHT_SKILL"
        return 1
    fi

    local round=0
    while [ $round -lt $MAX_INSIGHT_ROUNDS ]; do
        round=$((round + 1))
        local timestamp
        timestamp=$(date '+%Y%m%d_%H%M%S')
        local log_file="$LOG_DIR/insight_${STAGE}_${round}_${timestamp}.log"

        echo ""
        echo "  [Insight round $round/$MAX_INSIGHT_ROUNDS] Invoking insight skill..."
        cd "$OMNI_DIR" && claude -p "$(cat "$INSIGHT_SKILL")" \
            --dangerously-skip-permissions \
            2>&1 | tee "$log_file"

        echo "  Insight round $round complete. Checking if new failing tests were added..."

        # Check if eval tests still pass after insight added new tests
        local test_exit=0
        run_eval_tests || test_exit=$?

        if [ "$test_exit" -eq 0 ]; then
            echo "  All eval tests pass after insight. No impl needed."
            continue
        fi

        echo "  New failing tests detected. Re-entering impl phase..."
        run_impl || {
            echo "  Warning: impl did not fully resolve new tests from insight round $round."
        }
    done

    echo "  Insight phase complete ($round rounds)."
}

# ──────────────────────────────────────────────
# Final verification
# ──────────────────────────────────────────────
run_final_verification() {
    echo ""
    echo "========================================="
    echo "  Final Verification"
    echo "========================================="

    echo "  Running go build ./oracle/..."
    cd "$OMNI_DIR" && go build ./oracle/... 2>&1

    echo "  Running go test ./oracle/... -run TestEval -count=1"
    local log_file="$LOG_DIR/verify_${STAGE}_$(date '+%Y%m%d_%H%M%S').log"
    cd "$OMNI_DIR" && go test ./oracle/... -run TestEval -count=1 2>&1 | tee "$log_file"

    echo "  Final verification complete."
}

# ──────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────

# Determine which phases to run
RUN_EVAL=true
RUN_IMPL=true
RUN_INSIGHT=true

if [ "$EVAL_ONLY" = true ]; then
    RUN_EVAL=true; RUN_IMPL=false; RUN_INSIGHT=false
elif [ "$IMPL_ONLY" = true ]; then
    RUN_EVAL=false; RUN_IMPL=true; RUN_INSIGHT=false
elif [ "$INSIGHT_ONLY" = true ]; then
    RUN_EVAL=false; RUN_IMPL=false; RUN_INSIGHT=true
fi

# Regression guard (always run unless doing eval-only on stage 1)
echo ""
echo "========================================="
echo "  Regression Guard"
echo "========================================="
run_regression_guard

# Execute phases
if [ "$RUN_EVAL" = true ]; then
    run_eval
fi

if [ "$RUN_IMPL" = true ]; then
    run_impl || true
fi

if [ "$RUN_INSIGHT" = true ]; then
    run_insight
fi

# Final verification
run_final_verification

echo ""
echo "========================================="
echo "  PIPELINE COMPLETE: Oracle Stage $STAGE"
echo "========================================="
exit 0
