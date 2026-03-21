#!/usr/bin/env python3
"""Discover and map integration test."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def load_family(name: str) -> dict:
    with (ROOT / "pg" / "semantic" / "families" / f"{name}.json").open() as fh:
        return json.load(fh)


def main() -> None:
    for stage in ("discover", "map"):
        result = subprocess.run(
            [sys.executable, "scripts/pg-semantic/run_stage.py", "--family", "create_table", "--stage", stage, "--executor", "local"],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        if result.returncode != 0:
            raise SystemExit(result.stderr or result.stdout)

    family = load_family("create_table")
    if not family["scenarios"]:
        raise SystemExit("discover must add scenarios")
    if not family["dispatcher"] or not family["handlers"]:
        raise SystemExit("map must populate dispatcher and handlers")
    paths = [item.get("path", "") for item in family["dispatcher"] + family["handlers"]]
    if not any(path.startswith("../postgres/src/backend/") for path in paths):
        raise SystemExit("map output must reference ../postgres/src/backend paths")
    if not any("tcop/utility.c" in path for path in paths):
        raise SystemExit("create_table mapping must include tcop/utility.c")
    if not any("commands/tablecmds.c" in path for path in paths):
        raise SystemExit("create_table mapping must include commands/tablecmds.c")
    evidence_paths = [item.get("path", "") for item in family["evidence"] if item.get("stage") == "map"]
    if not any(":" in path for path in evidence_paths):
        raise SystemExit("map evidence should include source line references")
    print("PASS: discover and map validated")


if __name__ == "__main__":
    main()
