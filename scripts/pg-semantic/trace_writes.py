#!/usr/bin/env python3
"""Convenience wrapper for the trace_writes stage."""

from common import stage_trace_writes


if __name__ == "__main__":
    import argparse
    import json

    parser = argparse.ArgumentParser()
    parser.add_argument("--family", required=True)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    print(json.dumps(stage_trace_writes(args.family, dry_run=args.dry_run), indent=2))
