#!/usr/bin/env python3
"""Trace read/write integration test."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def load_family(name: str) -> dict:
    with (ROOT / "pg" / "semantic" / "families" / f"{name}.json").open() as fh:
        return json.load(fh)


def run_stage(family: str, stage: str) -> None:
    result = subprocess.run(
        [sys.executable, "scripts/pg-semantic/run_stage.py", "--family", family, "--stage", stage, "--executor", "local"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit(result.stderr or result.stdout)


def main() -> None:
    for stage in ("discover", "map", "trace_writes", "trace_reads"):
        run_stage("create_table", stage)
    run_stage("select", "discover")
    run_stage("select", "map")
    run_stage("select", "trace_reads")

    create_table = load_family("create_table")
    catalogs = {item["catalog"] for item in create_table["writes"]}
    required = {"pg_class", "pg_attribute", "pg_type", "pg_depend"}
    if not required.issubset(catalogs):
        raise SystemExit(f"create_table writes missing catalogs: {sorted(required - catalogs)}")
    create_table_write_paths = [item.get("path", "") for item in create_table.get("evidence", []) if item.get("stage") == "trace_writes"]
    if not any("catalog/heap.c" in path for path in create_table_write_paths):
        raise SystemExit("create_table write evidence should include catalog/heap.c")

    select = load_family("select")
    read_catalogs = {item["catalog"] for item in select["reads"]}
    if not {"pg_operator", "pg_type", "pg_class", "pg_namespace"}.issubset(read_catalogs):
        raise SystemExit("select reads missing expected semantic catalogs")
    select_read_paths = [item.get("path", "") for item in select.get("evidence", []) if item.get("stage") == "trace_reads"]
    if not any("parser/analyze.c" in path or "parser/parse_type.c" in path or "catalog/namespace.c" in path for path in select_read_paths):
        raise SystemExit("select read evidence should include analyze/type/namespace paths")

    set_family = load_family("set")
    for stage in ("discover", "map", "trace_reads"):
        run_stage("set", stage)
    set_evidence = [item for item in set_family.get("evidence", []) if item.get("stage") == "trace_reads"]
    if not set_evidence:
        set_family = load_family("set")
        set_evidence = [item for item in set_family.get("evidence", []) if item.get("stage") == "trace_reads"]
    if not any(":" in item.get("path", "") for item in set_evidence):
        raise SystemExit("set trace_reads should include source line references")
    if not any("guc_funcs.c" in item.get("path", "") or "pg_db_role_setting.c" in item.get("path", "") for item in set_evidence):
        raise SystemExit("set trace_reads should include guc or db_role_setting evidence")
    print("PASS: trace io validated")


if __name__ == "__main__":
    main()
