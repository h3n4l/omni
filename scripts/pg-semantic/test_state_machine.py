#!/usr/bin/env python3
"""Lightweight state machine tests for the semantic pipeline."""

from __future__ import annotations

import json
import tempfile
from pathlib import Path

from update_state import load_state, save_state, stage_actionable, update_stage


ROOT = Path(__file__).resolve().parents[2]
STATE_PATH = ROOT / "pg" / "semantic" / "STATE.json"


def assert_true(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def main() -> None:
    with STATE_PATH.open() as fh:
        initial = json.load(fh)
    for family in initial["families"]:
        family["status"] = "pending"
        family["scenario_count"] = 0
        family["updated_at"] = ""
        for stage in family["stages"]:
            family["stages"][stage] = "pending"

    with tempfile.TemporaryDirectory() as tmp:
        tmp_state = Path(tmp) / "STATE.json"
        save_state(tmp_state, initial)

        state = load_state(tmp_state)
        family = next(item for item in state["families"] if item["id"] == "create_table")
        assert_true(stage_actionable(family, "discover"), "discover should be actionable first")
        assert_true(not stage_actionable(family, "map"), "map must be blocked before discover")
        assert_true(not stage_actionable(family, "trace_writes"), "trace_writes must be blocked before map")
        assert_true(not stage_actionable(family, "synthesize"), "synthesize must be blocked initially")

        state = update_stage(state, "create_table", "discover", "done", scenario_count=3)
        family = next(item for item in state["families"] if item["id"] == "create_table")
        assert_true(stage_actionable(family, "map"), "map should be actionable after discover")
        assert_true(not stage_actionable(family, "trace_reads"), "trace_reads must stay blocked before map")
        assert_true(family["scenario_count"] == 3, "scenario_count should be updated")

        state = update_stage(state, "create_table", "map", "done")
        family = next(item for item in state["families"] if item["id"] == "create_table")
        assert_true(stage_actionable(family, "trace_writes"), "trace_writes should unlock after map")
        assert_true(stage_actionable(family, "trace_reads"), "trace_reads should unlock after map")

        state = update_stage(state, "create_table", "trace_writes", "done")
        state = update_stage(state, "create_table", "trace_reads", "done")
        state = update_stage(state, "create_table", "map_tests", "done")
        state = update_stage(state, "create_table", "plan_translation", "done")
        family = next(item for item in state["families"] if item["id"] == "create_table")
        assert_true(stage_actionable(family, "synthesize"), "synthesize should unlock after all prior stages")

        print("PASS: state machine gating validated")


if __name__ == "__main__":
    main()
