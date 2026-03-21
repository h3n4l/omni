#!/usr/bin/env python3
"""Update family stage state for the PostgreSQL semantic pipeline."""

from __future__ import annotations

import argparse
import json
from copy import deepcopy
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_STATE_PATH = ROOT / "pg" / "semantic" / "STATE.json"
ORDER = [
    "discover",
    "map",
    "trace_writes",
    "trace_reads",
    "map_tests",
    "plan_translation",
    "synthesize",
]
DEPENDENCIES = {
    "discover": [],
    "map": ["discover"],
    "trace_writes": ["map"],
    "trace_reads": ["map"],
    "map_tests": ["trace_writes", "trace_reads"],
    "plan_translation": ["trace_writes", "trace_reads"],
    "synthesize": ["discover", "map", "trace_writes", "trace_reads", "map_tests", "plan_translation"],
}
VALID_STAGE_STATUS = {"pending", "in_progress", "done", "failed"}
VALID_FAMILY_STATUS = {"pending", "in_progress", "done", "failed"}


def load_state(path: Path) -> dict[str, Any]:
    with path.open() as fh:
        return json.load(fh)


def save_state(path: Path, data: dict[str, Any]) -> None:
    tmp_path = path.with_suffix(path.suffix + ".tmp")
    with tmp_path.open("w") as fh:
        json.dump(data, fh, indent=2)
        fh.write("\n")
    tmp_path.replace(path)


def family_by_id(data: dict[str, Any], family_id: str) -> dict[str, Any]:
    for family in data["families"]:
        if family["id"] == family_id:
            return family
    raise KeyError(f"unknown family: {family_id}")


def recompute_family_status(family: dict[str, Any]) -> None:
    stage_values = list(family["stages"].values())
    if any(value == "failed" for value in stage_values):
        family["status"] = "failed"
    elif all(value == "done" for value in stage_values):
        family["status"] = "done"
    elif any(value == "in_progress" for value in stage_values) or any(value == "done" for value in stage_values):
        family["status"] = "in_progress"
    else:
        family["status"] = "pending"


def stage_actionable(family: dict[str, Any], stage: str) -> bool:
    if stage not in family["stages"]:
        raise KeyError(f"unknown stage: {stage}")

    required = DEPENDENCIES[stage]
    return all(family["stages"][item] == "done" for item in required)


def update_stage(
    data: dict[str, Any],
    family_id: str,
    stage: str,
    stage_status: str,
    scenario_count: int | None = None,
) -> dict[str, Any]:
    if stage_status not in VALID_STAGE_STATUS:
        raise ValueError(f"invalid stage status: {stage_status}")

    result = deepcopy(data)
    family = family_by_id(result, family_id)
    if stage not in family["stages"]:
        raise KeyError(f"unknown stage: {stage}")
    if stage_status == "in_progress" and not stage_actionable(family, stage):
        raise ValueError(f"stage {stage} is not actionable for family {family_id}")
    if stage_status == "done" and not stage_actionable(family, stage):
        raise ValueError(f"stage {stage} cannot be marked done before prerequisites")

    family["stages"][stage] = stage_status
    if scenario_count is not None:
        family["scenario_count"] = scenario_count
    family["updated_at"] = datetime.now(timezone.utc).isoformat()
    recompute_family_status(family)
    return result


def update_family_status(data: dict[str, Any], family_id: str, family_status: str) -> dict[str, Any]:
    if family_status not in VALID_FAMILY_STATUS:
        raise ValueError(f"invalid family status: {family_status}")
    result = deepcopy(data)
    family = family_by_id(result, family_id)
    family["status"] = family_status
    family["updated_at"] = datetime.now(timezone.utc).isoformat()
    return result


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--state", type=Path, default=DEFAULT_STATE_PATH)
    parser.add_argument("--family", required=True)
    parser.add_argument("--stage")
    parser.add_argument("--stage-status", choices=sorted(VALID_STAGE_STATUS))
    parser.add_argument("--family-status", choices=sorted(VALID_FAMILY_STATUS))
    parser.add_argument("--scenario-count", type=int)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    data = load_state(args.state)

    if args.stage and args.stage_status:
        data = update_stage(data, args.family, args.stage, args.stage_status, args.scenario_count)
    elif args.family_status:
        data = update_family_status(data, args.family, args.family_status)
    else:
        raise SystemExit("must provide either --stage/--stage-status or --family-status")

    save_state(args.state, data)


if __name__ == "__main__":
    main()
