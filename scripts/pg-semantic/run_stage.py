#!/usr/bin/env python3
"""Run one stage for one semantic family."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from common import LOGS_DIR, ROOT, run_stage


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--family", required=True)
    parser.add_argument("--stage", required=True)
    parser.add_argument("--executor", choices=["local", "codex"], default="local")
    parser.add_argument("--dry-run", action="store_true")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    payload = run_stage(args.family, args.stage, dry_run=args.dry_run, executor=args.executor)
    LOGS_DIR.mkdir(parents=True, exist_ok=True)
    log_path = LOGS_DIR / f"{args.family}_{args.stage}.json"
    if not args.dry_run:
        with log_path.open("w") as fh:
            json.dump(payload, fh, indent=2)
            fh.write("\n")
    print(json.dumps(payload, indent=2))


if __name__ == "__main__":
    main()
