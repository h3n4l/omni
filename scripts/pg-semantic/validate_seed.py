#!/usr/bin/env python3
"""Validate the PostgreSQL semantic family seed universe."""

from __future__ import annotations

import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SEED_PATH = ROOT / "pg" / "semantic" / "families.seed.json"

ALLOWED_CATEGORIES = {"ddl", "dml", "query", "set", "object_control"}
BANNED_IDS = {"names", "types", "expressions", "functions", "xml", "json", "infrastructure"}


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    if not SEED_PATH.exists():
        fail(f"missing seed file: {SEED_PATH}")

    with SEED_PATH.open() as fh:
        data = json.load(fh)

    if data.get("source_root") != "../postgres":
        fail("source_root must be ../postgres")

    families = data.get("families")
    if not isinstance(families, list) or not families:
        fail("families must be a non-empty list")

    seen_ids = set()
    categories = {}

    for family in families:
        if not isinstance(family, dict):
            fail("every family entry must be an object")

        family_id = family.get("id")
        slug = family.get("slug")
        category = family.get("category")

        if not family_id or not isinstance(family_id, str):
            fail(f"invalid family id: {family!r}")
        if family_id in seen_ids:
            fail(f"duplicate family id: {family_id}")
        if family_id in BANNED_IDS:
            fail(f"out-of-scope low-level family present: {family_id}")
        if not slug or not isinstance(slug, str):
            fail(f"family {family_id} is missing slug")
        if category not in ALLOWED_CATEGORIES:
            fail(f"family {family_id} has invalid category: {category}")

        statements = family.get("statements")
        entry_ast_nodes = family.get("entry_ast_nodes")
        dispatch_roots = family.get("dispatch_roots")

        if not isinstance(statements, list) or not statements:
            fail(f"family {family_id} must define statements")
        if not isinstance(entry_ast_nodes, list) or not entry_ast_nodes:
            fail(f"family {family_id} must define entry_ast_nodes")
        if not isinstance(dispatch_roots, list) or not dispatch_roots:
            fail(f"family {family_id} must define dispatch_roots")

        seen_ids.add(family_id)
        categories[category] = categories.get(category, 0) + 1

    print(f"PASS: {len(families)} families validated from {SEED_PATH.relative_to(ROOT)}")
    for category in sorted(categories):
        print(f"  {category}: {categories[category]}")


if __name__ == "__main__":
    main()
