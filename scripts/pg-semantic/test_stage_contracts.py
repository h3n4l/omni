#!/usr/bin/env python3
"""Verify that all semantic pipeline stage contracts exist and are well formed."""

from __future__ import annotations

from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SKILL_PATH = ROOT / "pg" / "semantic" / "SKILL.md"
STAGE_DIR = ROOT / "pg" / "semantic" / "stages"
STAGES = [
    "discover",
    "map",
    "trace_writes",
    "trace_reads",
    "map_tests",
    "plan_translation",
    "synthesize",
]


def assert_contains(text: str, fragment: str, path: Path) -> None:
    if fragment not in text:
        raise AssertionError(f"{path} is missing required fragment: {fragment}")


def main() -> None:
    skill_text = SKILL_PATH.read_text()
    assert_contains(skill_text, "../postgres", SKILL_PATH)
    assert_contains(skill_text, "pg/semantic/STATE.json", SKILL_PATH)

    for stage in STAGES:
        path = STAGE_DIR / f"{stage}.md"
        text = path.read_text()
        assert_contains(text, "../postgres", path)
        assert_contains(text, "Output contract:", path)
        assert_contains(text, "Emit only JSON", path)
        assert_contains(text, "pg/semantic/families/<family>.json", path)
        print(f"PASS: {stage}")


if __name__ == "__main__":
    main()
