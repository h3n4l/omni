#!/usr/bin/env python3
"""Convenience wrapper for the discover stage."""

from common import stage_discover


if __name__ == "__main__":
    import argparse
    import json

    parser = argparse.ArgumentParser()
    parser.add_argument("--family", required=True)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    print(json.dumps(stage_discover(args.family, dry_run=args.dry_run), indent=2))
