#!/usr/bin/env python3
"""End-to-end pipeline dry-run test."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def main() -> None:
    driver = ROOT / "scripts" / "pg-semantic-driver.sh"
    result = subprocess.run(
        [str(driver), "--family", "create_table", "--all"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(result.stderr or result.stdout)

    with (ROOT / "pg" / "semantic" / "families" / "create_table.json").open() as fh:
        family = json.load(fh)
    if not family["stage_status"].get("synthesize") == "done":
        raise SystemExit("full run must finish synthesize stage")
    print("PASS: end-to-end dry run validated")


if __name__ == "__main__":
    main()
