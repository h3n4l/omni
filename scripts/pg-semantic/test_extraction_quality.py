#!/usr/bin/env python3
"""Quality checks for deeper extraction output on core families."""

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
        [sys.executable, "scripts/pg-semantic/run_stage.py", "--family", family, "--stage", stage, "--executor", "local"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(result.stderr or result.stdout)


def require_chain(items: list[dict], label: str) -> None:
    if not items:
        raise SystemExit(f"{label} is empty")
    if not any(item.get("chain") for item in items):
        raise SystemExit(f"{label} must include non-empty chain fields")


def require_source_path(items: list[dict], label: str) -> None:
    if not all("source_path" in item and ":" in item["source_path"] for item in items):
        raise SystemExit(f"{label} must include source_path with line numbers")


def main() -> None:
    for family, stages in {
        "create_table": ("discover", "map", "trace_writes", "trace_reads"),
        "select": ("discover", "map", "trace_reads"),
        "set": ("discover", "map", "trace_reads"),
    }.items():
        for stage in stages:
            run_stage(family, stage)

    create_table = load_family("create_table")
    require_chain(create_table["dispatcher"], "create_table dispatcher")
    require_chain(create_table["handlers"], "create_table handlers")
    require_chain(create_table["writes"], "create_table writes")
    require_source_path(create_table["writes"], "create_table writes")

    select = load_family("select")
    require_chain(select["dispatcher"], "select dispatcher")
    require_chain(select["reads"], "select reads")
    require_source_path(select["reads"], "select reads")

    set_family = load_family("set")
    require_chain(set_family["dispatcher"], "set dispatcher")
    require_chain(set_family["reads"], "set reads")
    require_source_path(set_family["reads"], "set reads")

    print("PASS: extraction quality validated")


if __name__ == "__main__":
    main()
