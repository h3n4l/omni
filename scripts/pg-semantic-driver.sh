#!/bin/bash
# Codex-based PostgreSQL semantic pipeline driver.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
METRICS_DB="$ROOT_DIR/metrics.db"
STAGE=""
FAMILY=""
MAX_FAMILIES=1000
RUN_ALL=false
DRY_RUN=false
EXECUTOR="local"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --stage) STAGE="$2"; shift 2 ;;
        --family) FAMILY="$2"; shift 2 ;;
        --max-families) MAX_FAMILIES="$2"; shift 2 ;;
        --all) RUN_ALL=true; shift ;;
        --dry-run) DRY_RUN=true; shift ;;
        --executor) EXECUTOR="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ "$RUN_ALL" = false && -z "$STAGE" ]]; then
    echo "Usage: $0 [--family NAME] --stage STAGE [--max-families N] [--dry-run] | --all"
    exit 1
fi

run_stage() {
    local family="$1"
    local stage="$2"
    local dry_args=()
    if [[ "$DRY_RUN" = true ]]; then
        dry_args+=(--dry-run)
    fi
    echo "[semantic] family=$family stage=$stage executor=$EXECUTOR dry_run=$DRY_RUN"
    if [[ ${#dry_args[@]} -gt 0 ]]; then
        python3 "$ROOT_DIR/scripts/pg-semantic/run_stage.py" --family "$family" --stage "$stage" --executor "$EXECUTOR" "${dry_args[@]}"
    else
        python3 "$ROOT_DIR/scripts/pg-semantic/run_stage.py" --family "$family" --stage "$stage" --executor "$EXECUTOR"
    fi
    if [[ "$DRY_RUN" = false ]]; then
        python3 "$ROOT_DIR/scripts/metrics.py" init "$METRICS_DB" >/dev/null
        python3 "$ROOT_DIR/scripts/metrics.py" semantic-stage "$METRICS_DB" "$family" "$stage" "done" >/dev/null
    fi
}

if [[ -n "$FAMILY" && -n "$STAGE" ]]; then
    run_stage "$FAMILY" "$STAGE"
    exit 0
fi

if [[ -z "$FAMILY" && -n "$STAGE" ]]; then
    COUNT=0
    while [[ $COUNT -lt $MAX_FAMILIES ]]; do
        NEXT="$(python3 "$ROOT_DIR/scripts/pg-semantic/next_family.py" --stage "$STAGE" 2>/dev/null || true)"
        if [[ -z "$NEXT" ]]; then
            break
        fi
        run_stage "$NEXT" "$STAGE"
        COUNT=$((COUNT + 1))
        if [[ "$DRY_RUN" = true ]]; then
            break
        fi
    done
    exit 0
fi

if [[ "$RUN_ALL" = true ]]; then
    STAGES=(discover map trace_writes trace_reads map_tests plan_translation synthesize)
    for stage_name in "${STAGES[@]}"; do
        if [[ -n "$FAMILY" ]]; then
            run_stage "$FAMILY" "$stage_name"
        else
            COUNT=0
            while [[ $COUNT -lt $MAX_FAMILIES ]]; do
                NEXT="$(python3 "$ROOT_DIR/scripts/pg-semantic/next_family.py" --stage "$stage_name" 2>/dev/null || true)"
                if [[ -z "$NEXT" ]]; then
                    break
                fi
                run_stage "$NEXT" "$stage_name"
                COUNT=$((COUNT + 1))
                if [[ "$DRY_RUN" = true ]]; then
                    break
                fi
            done
        fi
    done
    exit 0
fi

echo "Invalid argument combination"
exit 1
