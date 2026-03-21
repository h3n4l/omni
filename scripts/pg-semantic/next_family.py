#!/usr/bin/env python3
"""Pick the next actionable family for a given semantic pipeline stage."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from update_state import DEFAULT_STATE_PATH, ORDER, stage_actionable


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--state", type=Path, default=DEFAULT_STATE_PATH)
    parser.add_argument("--stage", required=True, choices=ORDER)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    with args.state.open() as fh:
        data = json.load(fh)

    for family in data["families"]:
        if family["stages"][args.stage] != "pending":
            continue
        if not stage_actionable(family, args.stage):
            continue
        print(family["id"])
        return

    raise SystemExit(1)


if __name__ == "__main__":
    main()
