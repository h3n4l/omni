#!/usr/bin/env python3
"""Migration layer test."""

from __future__ import annotations

from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def main() -> None:
    driver_text = (ROOT / "scripts" / "pg-catalog-driver.sh").read_text()
    if "pg/catalog/PROGRESS.json" in driver_text and "deprecated wrapper" not in driver_text:
        raise SystemExit("pg-catalog-driver.sh must stop treating pg/catalog/PROGRESS.json as authoritative")

    migration_doc = ROOT / "docs" / "pg-semantic-pipeline-migration.md"
    if not migration_doc.exists():
        raise SystemExit("migration document is missing")

    for rel in (
        "scripts/pg-bnf-source-mapper/SKILL.md",
        "scripts/pg-write-trace-skill/SKILL.md",
        "scripts/pg-read-trace-skill/SKILL.md",
        "scripts/pg-inventory-synthesizer/SKILL.md",
    ):
        text = (ROOT / rel).read_text()
        if "Deprecated" not in text:
            raise SystemExit(f"{rel} must declare deprecation")
    print("PASS: migration validated")


if __name__ == "__main__":
    main()
