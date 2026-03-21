#!/usr/bin/env python3
"""Validate family artifact initialization flow."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def run(cmd: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, cwd=ROOT, text=True, capture_output=True, check=False)


def main() -> None:
    init_result = run([sys.executable, "scripts/pg-semantic/init_family_artifact.py"])
    if init_result.returncode != 0:
        raise SystemExit(init_result.stderr or init_result.stdout)

    validate_result = run([sys.executable, "scripts/pg-semantic/validate_family_artifact.py"])
    if validate_result.returncode != 0:
        raise SystemExit(validate_result.stderr or validate_result.stdout)

    print("PASS: family artifact initialization validated")


if __name__ == "__main__":
    main()
