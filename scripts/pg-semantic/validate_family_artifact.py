#!/usr/bin/env python3
"""Validate generated family artifacts and the global index."""

from __future__ import annotations

import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
FAMILIES_DIR = ROOT / "pg" / "semantic" / "families"
INDEX_PATH = ROOT / "pg" / "semantic" / "index.json"
REQUIRED_KEYS = {
    "family",
    "scope",
    "dispatcher",
    "handlers",
    "writes",
    "reads",
    "regress_files",
    "scenarios",
    "translation_targets",
    "evidence",
    "stage_status",
}


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def load_json(path: Path) -> dict:
    with path.open() as fh:
        return json.load(fh)


def main() -> None:
    if not INDEX_PATH.exists():
        fail(f"missing index: {INDEX_PATH}")

    index = load_json(INDEX_PATH)
    entries = index.get("families")
    if not isinstance(entries, list) or not entries:
        fail("index must contain at least one family")

    for entry in entries:
        path = ROOT / entry["path"]
        if not path.exists():
            fail(f"indexed artifact is missing: {entry['path']}")
        artifact = load_json(path)
        missing = sorted(REQUIRED_KEYS - set(artifact))
        if missing:
            fail(f"{entry['id']} missing keys: {', '.join(missing)}")
        for evidence in artifact.get("evidence", []):
            ev_path = evidence.get("path", "")
            if not ev_path.startswith("../postgres/"):
                fail(f"{entry['id']} evidence must stay rooted in ../postgres: {ev_path}")
            if ":" not in ev_path:
                fail(f"{entry['id']} evidence must include line number: {ev_path}")
        print(f"PASS: {entry['id']} -> {entry['path']}")


if __name__ == "__main__":
    main()
