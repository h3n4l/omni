#!/usr/bin/env python3
"""Synthesis integration test."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def load_json(path: Path) -> dict:
    with path.open() as fh:
        return json.load(fh)


def main() -> None:
    result = subprocess.run(
        [sys.executable, "scripts/pg-semantic/run_stage.py", "--family", "create_table", "--stage", "synthesize"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(result.stderr or result.stdout)

    family = load_json(ROOT / "pg" / "semantic" / "families" / "create_table.json")
    index = load_json(ROOT / "pg" / "semantic" / "index.json")
    if family.get("status") not in {"in_progress", "done"}:
        raise SystemExit("family synthesis must compute summary status")
    if not any(entry["id"] == "create_table" for entry in index["families"]):
        raise SystemExit("index must contain create_table after synthesis")
    print("PASS: synthesis validated")


if __name__ == "__main__":
    main()
