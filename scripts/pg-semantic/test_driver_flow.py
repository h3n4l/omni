#!/usr/bin/env python3
"""Driver orchestration smoke test."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def main() -> None:
    driver = ROOT / "scripts" / "pg-semantic-driver.sh"
    for executor in ("local", "codex"):
        result = subprocess.run(
            [str(driver), "--family", "create_table", "--stage", "discover", "--executor", executor, "--dry-run"],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        if result.returncode != 0:
            raise SystemExit(result.stderr or result.stdout)
        if "create_table" not in result.stdout:
            raise SystemExit("driver output must mention create_table")
        if executor not in result.stdout:
            raise SystemExit(f"driver output must mention executor={executor}")
    print("PASS: driver dry run validated")


if __name__ == "__main__":
    main()
