#!/usr/bin/env python3
"""Regress mapping and translation planning integration test."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def load_family(name: str) -> dict:
    with (ROOT / "pg" / "semantic" / "families" / f"{name}.json").open() as fh:
        return json.load(fh)


def run_stage(family: str, stage: str) -> None:
    result = subprocess.run(
        [sys.executable, "scripts/pg-semantic/run_stage.py", "--family", family, "--stage", stage],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(result.stderr or result.stdout)


def main() -> None:
    for stage in ("discover", "map", "trace_writes", "trace_reads", "map_tests", "plan_translation"):
        run_stage("create_table", stage)

    family = load_family("create_table")
    if not family["regress_files"]:
        raise SystemExit("map_tests must populate regress_files")
    if not family["translation_targets"]:
        raise SystemExit("plan_translation must populate translation_targets")
    if not all(target["path"].startswith("../postgres/") for target in family["translation_targets"]):
        raise SystemExit("translation targets must point to PG C files")
    print("PASS: regress mapping and translation planning validated")


if __name__ == "__main__":
    main()
