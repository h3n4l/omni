#!/usr/bin/env python3
"""Initialize per-family semantic artifacts from the seed universe."""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SEED_PATH = ROOT / "pg" / "semantic" / "families.seed.json"
STATE_PATH = ROOT / "pg" / "semantic" / "STATE.json"
FAMILIES_DIR = ROOT / "pg" / "semantic" / "families"
INDEX_PATH = ROOT / "pg" / "semantic" / "index.json"
STAGES = [
    "discover",
    "map",
    "trace_writes",
    "trace_reads",
    "map_tests",
    "plan_translation",
    "synthesize",
]


def load_json(path: Path) -> dict:
    with path.open() as fh:
        return json.load(fh)


def write_json(path: Path, data: dict) -> None:
    with path.open("w") as fh:
        json.dump(data, fh, indent=2)
        fh.write("\n")


def main() -> None:
    seed = load_json(SEED_PATH)
    state = load_json(STATE_PATH)
    state_map = {family["id"]: family for family in state["families"]}
    FAMILIES_DIR.mkdir(parents=True, exist_ok=True)

    index = {
        "version": 1,
        "source_root": seed["source_root"],
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "families": [],
    }

    for family in seed["families"]:
        state_entry = state_map[family["id"]]
        artifact = {
            "family": {
                "id": family["id"],
                "slug": family["slug"],
                "category": family["category"],
            },
            "scope": {
                "statements": family["statements"],
                "entry_ast_nodes": family["entry_ast_nodes"],
                "dispatch_roots": family["dispatch_roots"],
            },
            "dispatcher": [],
            "handlers": [],
            "writes": [],
            "reads": [],
            "regress_files": [],
            "scenarios": [],
            "translation_targets": [],
            "evidence": [],
            "stage_status": {stage: state_entry["stages"].get(stage, "pending") for stage in STAGES},
        }
        artifact_path = FAMILIES_DIR / f"{family['id']}.json"
        write_json(artifact_path, artifact)

        index["families"].append(
            {
                "id": family["id"],
                "slug": family["slug"],
                "category": family["category"],
                "path": str(artifact_path.relative_to(ROOT)),
                "status": state_entry["status"],
                "scenario_count": state_entry["scenario_count"],
            }
        )

    write_json(INDEX_PATH, index)
    print(f"PASS: initialized {len(index['families'])} family artifacts")


if __name__ == "__main__":
    main()
