#!/usr/bin/env python3
"""Common helpers for the PostgreSQL semantic pipeline."""

from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from update_state import DEFAULT_STATE_PATH, ORDER, load_state, save_state, update_stage


ROOT = Path(__file__).resolve().parents[2]
PG_SEMANTIC_DIR = ROOT / "pg" / "semantic"
SEED_PATH = PG_SEMANTIC_DIR / "families.seed.json"
STATE_PATH = DEFAULT_STATE_PATH
INDEX_PATH = PG_SEMANTIC_DIR / "index.json"
FAMILIES_DIR = PG_SEMANTIC_DIR / "families"
LOGS_DIR = PG_SEMANTIC_DIR / "logs"
POSTGRES_ROOT = (ROOT / ".." / "postgres").resolve()
CODEX_BIN = shutil.which("codex") or "codex"
STAGE_DEPENDENCIES = {
    "discover": [],
    "map": ["discover"],
    "trace_writes": ["discover", "map"],
    "trace_reads": ["discover", "map"],
    "map_tests": ["discover", "map", "trace_writes", "trace_reads"],
    "plan_translation": ["discover", "map", "trace_writes", "trace_reads"],
    "synthesize": ["discover", "map", "trace_writes", "trace_reads", "map_tests", "plan_translation"],
}

FAMILY_SCENARIOS = {
    "create_table": [
        {"id": "basic_heap_table", "summary": "Basic heap table creation", "manual_review": False},
        {"id": "table_as", "summary": "CREATE TABLE AS / SELECT INTO path", "manual_review": False},
        {"id": "partitioned_table", "summary": "Partition-aware table creation", "manual_review": False},
    ],
    "select": [
        {"id": "simple_select", "summary": "Plain SELECT query analysis", "manual_review": False},
        {"id": "cte_select", "summary": "WITH and CTE resolution", "manual_review": False},
        {"id": "setop_select", "summary": "UNION/INTERSECT/EXCEPT analysis", "manual_review": False},
    ],
    "set": [
        {"id": "guc_assignment", "summary": "GUC assignment and RESET semantics", "manual_review": False},
        {"id": "role_assignment", "summary": "SET ROLE and session auth paths", "manual_review": False},
    ],
}

FAMILY_MAP = {
    "create_table": {
        "dispatcher": [
            {"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"},
            {"symbol": "transformCreateStmt", "path": "../postgres/src/backend/parser/parse_utilcmd.c", "kind": "transform"},
        ],
        "handlers": [
            {"symbol": "DefineRelation", "path": "../postgres/src/backend/commands/tablecmds.c", "kind": "catalog_writer"},
        ],
    },
    "alter_table": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "AlterTable", "path": "../postgres/src/backend/commands/tablecmds.c", "kind": "catalog_writer"}],
    },
    "drop": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "RemoveObjects", "path": "../postgres/src/backend/commands/dropcmds.c", "kind": "catalog_writer"}],
    },
    "create_index": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "DefineIndex", "path": "../postgres/src/backend/commands/indexcmds.c", "kind": "catalog_writer"}],
    },
    "create_view": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "DefineView", "path": "../postgres/src/backend/commands/view.c", "kind": "catalog_writer"}],
    },
    "create_schema": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "CreateSchemaCommand", "path": "../postgres/src/backend/commands/schemacmds.c", "kind": "catalog_writer"}],
    },
    "create_function": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "CreateFunction", "path": "../postgres/src/backend/commands/functioncmds.c", "kind": "catalog_writer"}],
    },
    "create_type": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "DefineRelation", "path": "../postgres/src/backend/commands/typecmds.c", "kind": "catalog_writer"}],
    },
    "create_extension": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "CreateExtension", "path": "../postgres/src/backend/commands/extension.c", "kind": "catalog_writer"}],
    },
    "create_sequence": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "DefineSequence", "path": "../postgres/src/backend/commands/sequence.c", "kind": "catalog_writer"}],
    },
    "comment": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "CommentObject", "path": "../postgres/src/backend/commands/comment.c", "kind": "catalog_writer"}],
    },
    "grant_revoke": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "ExecuteGrantStmt", "path": "../postgres/src/backend/catalog/aclchk.c", "kind": "catalog_writer"}],
    },
    "trigger": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "CreateTrigger", "path": "../postgres/src/backend/commands/trigger.c", "kind": "catalog_writer"}],
    },
    "policy": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "CreatePolicy", "path": "../postgres/src/backend/commands/policy.c", "kind": "catalog_writer"}],
    },
    "select": {
        "dispatcher": [
            {"symbol": "parse_analyze_fixedparams", "path": "../postgres/src/backend/parser/analyze.c", "kind": "dispatcher"},
            {"symbol": "transformSelectStmt", "path": "../postgres/src/backend/parser/analyze.c", "kind": "analyzer"},
        ],
        "handlers": [{"symbol": "transformSelectStmt", "path": "../postgres/src/backend/parser/analyze.c", "kind": "analyzer"}],
    },
    "insert": {
        "dispatcher": [{"symbol": "parse_analyze_fixedparams", "path": "../postgres/src/backend/parser/analyze.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "transformInsertStmt", "path": "../postgres/src/backend/parser/analyze.c", "kind": "analyzer"}],
    },
    "update": {
        "dispatcher": [{"symbol": "parse_analyze_fixedparams", "path": "../postgres/src/backend/parser/analyze.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "transformUpdateStmt", "path": "../postgres/src/backend/parser/analyze.c", "kind": "analyzer"}],
    },
    "delete": {
        "dispatcher": [{"symbol": "parse_analyze_fixedparams", "path": "../postgres/src/backend/parser/analyze.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "transformDeleteStmt", "path": "../postgres/src/backend/parser/analyze.c", "kind": "analyzer"}],
    },
    "merge": {
        "dispatcher": [{"symbol": "parse_analyze_fixedparams", "path": "../postgres/src/backend/parser/analyze.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "transformMergeStmt", "path": "../postgres/src/backend/parser/analyze.c", "kind": "analyzer"}],
    },
    "set": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "ExecSetVariableStmt", "path": "../postgres/src/backend/utils/misc/guc_funcs.c", "kind": "validator"}],
    },
    "show": {
        "dispatcher": [{"symbol": "ProcessUtility", "path": "../postgres/src/backend/tcop/utility.c", "kind": "dispatcher"}],
        "handlers": [{"symbol": "GetPGVariable", "path": "../postgres/src/backend/utils/misc/guc_funcs.c", "kind": "validator"}],
    },
}

FAMILY_WRITES = {
    "create_table": [
        {"catalog": "pg_class", "operation": "insert", "trigger": "AddNewRelationTuple"},
        {"catalog": "pg_attribute", "operation": "insert", "trigger": "AddNewAttributeTuples"},
        {"catalog": "pg_type", "operation": "insert", "trigger": "AddNewRelationType"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOn"},
    ],
    "create_view": [
        {"catalog": "pg_class", "operation": "insert", "trigger": "DefineRelation"},
        {"catalog": "pg_rewrite", "operation": "insert", "trigger": "DefineViewRules"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOn"},
    ],
    "alter_table": [
        {"catalog": "pg_class", "operation": "update", "trigger": "CatalogTupleUpdate"},
        {"catalog": "pg_attribute", "operation": "update", "trigger": "CatalogTupleUpdate"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOn"},
    ],
    "drop": [{"catalog": "pg_depend", "operation": "delete", "trigger": "deleteDependencyRecordsFor"}],
    "create_index": [
        {"catalog": "pg_class", "operation": "insert", "trigger": "index_create"},
        {"catalog": "pg_index", "operation": "insert", "trigger": "UpdateIndexRelation"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOn"},
    ],
    "create_schema": [
        {"catalog": "pg_namespace", "operation": "insert", "trigger": "NamespaceCreate"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOnOwner"},
    ],
    "create_function": [
        {"catalog": "pg_proc", "operation": "insert", "trigger": "ProcedureCreate"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOnExpr"},
    ],
    "create_extension": [
        {"catalog": "pg_extension", "operation": "insert", "trigger": "InsertExtensionTuple"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOnCurrentExtension"},
    ],
    "create_sequence": [
        {"catalog": "pg_class", "operation": "insert", "trigger": "DefineRelation"},
        {"catalog": "pg_sequence", "operation": "insert", "trigger": "fill_seq_with_data"},
    ],
    "comment": [{"catalog": "pg_description", "operation": "insert", "trigger": "CreateComments"}],
    "grant_revoke": [{"catalog": "pg_class", "operation": "update", "trigger": "ExecGrant_Relation"}],
    "trigger": [
        {"catalog": "pg_trigger", "operation": "insert", "trigger": "CreateTriggerFiringOn"},
        {"catalog": "pg_depend", "operation": "insert", "trigger": "recordDependencyOn"},
    ],
    "policy": [{"catalog": "pg_policy", "operation": "insert", "trigger": "CreatePolicy"}],
}

FAMILY_READS = {
    "create_table": [
        {"catalog": "pg_namespace", "lookup_key": "schema name", "purpose": "resolve target schema"},
        {"catalog": "pg_type", "lookup_key": "type name", "purpose": "resolve column data types"},
        {"catalog": "pg_class", "lookup_key": "relation name", "purpose": "detect existing relations"},
        {"catalog": "pg_authid", "lookup_key": "current user", "purpose": "ownership and permission checks"},
    ],
    "select": [
        {"catalog": "pg_class", "lookup_key": "qualified relation name", "purpose": "resolve FROM relations"},
        {"catalog": "pg_namespace", "lookup_key": "search_path namespace", "purpose": "schema qualification"},
        {"catalog": "pg_type", "lookup_key": "oid and type name", "purpose": "type inference and coercion"},
        {"catalog": "pg_operator", "lookup_key": "operator name and arg types", "purpose": "resolve operator semantics"},
    ],
    "insert": [
        {"catalog": "pg_class", "lookup_key": "target relation", "purpose": "resolve INSERT target"},
        {"catalog": "pg_attribute", "lookup_key": "attname", "purpose": "resolve target columns"},
        {"catalog": "pg_type", "lookup_key": "type oid", "purpose": "coercion and compatibility"},
    ],
    "update": [
        {"catalog": "pg_class", "lookup_key": "target relation", "purpose": "resolve UPDATE target"},
        {"catalog": "pg_attribute", "lookup_key": "attname", "purpose": "resolve target columns"},
        {"catalog": "pg_operator", "lookup_key": "operator signature", "purpose": "expression analysis"},
    ],
    "delete": [
        {"catalog": "pg_class", "lookup_key": "target relation", "purpose": "resolve DELETE target"},
        {"catalog": "pg_namespace", "lookup_key": "search_path", "purpose": "namespace lookup"},
    ],
    "merge": [
        {"catalog": "pg_class", "lookup_key": "source and target relations", "purpose": "relation resolution"},
        {"catalog": "pg_attribute", "lookup_key": "join columns", "purpose": "column compatibility"},
        {"catalog": "pg_type", "lookup_key": "type oid", "purpose": "coercion in WHEN clauses"},
    ],
    "set": [
        {"catalog": "pg_db_role_setting", "lookup_key": "database and role", "purpose": "database role settings"},
        {"catalog": "pg_authid", "lookup_key": "role name", "purpose": "SET ROLE / session auth validation"},
    ],
    "show": [{"catalog": "pg_db_role_setting", "lookup_key": "database and role", "purpose": "SHOW effective GUC context"}],
}

FAMILY_REGRESS = {
    "create_table": ["create_table.sql", "create_table_like.sql", "select_into.sql"],
    "alter_table": ["alter_table.sql", "alter_generic.sql"],
    "drop": ["drop_if_exists.sql"],
    "create_index": ["create_index.sql", "indexing.sql", "index_including.sql"],
    "create_view": ["create_view.sql", "select_views.sql", "matview.sql"],
    "create_schema": ["create_schema.sql", "namespace.sql"],
    "create_function": ["create_function_sql.sql", "create_function_c.sql", "create_procedure.sql"],
    "create_type": ["create_type.sql", "typed_table.sql", "type_sanity.sql"],
    "create_extension": ["create_misc.sql"],
    "create_sequence": ["sequence.sql"],
    "comment": ["create_misc.sql"],
    "grant_revoke": ["privileges.sql"],
    "trigger": ["triggers.sql"],
    "policy": ["rowsecurity.sql"],
    "select": ["select.sql", "with.sql", "select_having.sql", "select_into.sql"],
    "insert": ["insert.sql", "insert_conflict.sql"],
    "update": ["update.sql"],
    "delete": ["delete.sql"],
    "merge": ["merge.sql"],
    "set": ["guc.sql", "transactions.sql"],
    "show": ["guc.sql"],
}

DISPATCHER_CHAINS = {
    "create_table": {
        "ProcessUtility": ["ProcessUtility", "transformCreateStmt", "DefineRelation"],
        "transformCreateStmt": ["transformCreateStmt", "DefineRelation"],
        "DefineRelation": ["DefineRelation", "heap_create_with_catalog"],
    },
    "select": {
        "parse_analyze_fixedparams": ["parse_analyze_fixedparams", "transformSelectStmt"],
        "transformSelectStmt": ["transformSelectStmt", "transformFromClause", "transformTargetList"],
    },
    "set": {
        "ProcessUtility": ["ProcessUtility", "ExecSetVariableStmt"],
        "ExecSetVariableStmt": ["ExecSetVariableStmt", "AlterSetting"],
    },
}

WRITE_CHAINS = {
    "create_table": {
        "AddNewRelationTuple": ["DefineRelation", "heap_create_with_catalog", "AddNewRelationTuple"],
        "AddNewAttributeTuples": ["DefineRelation", "heap_create_with_catalog", "AddNewAttributeTuples"],
        "AddNewRelationType": ["DefineRelation", "heap_create_with_catalog", "AddNewRelationType"],
        "recordDependencyOn": ["DefineRelation", "AddNewRelationType", "recordDependencyOn"],
    },
}

READ_CHAINS = {
    "create_table": {
        "pg_namespace": ["DefineRelation", "LookupNamespaceNoError"],
        "pg_type": ["transformColumnDefinition", "typenameTypeId"],
        "pg_class": ["transformCreateStmt", "RangeVarGetRelid"],
        "pg_authid": ["ProcessUtility", "GetUserId"],
    },
    "select": {
        "pg_class": ["parse_analyze_fixedparams", "transformSelectStmt", "RangeVarGetRelid"],
        "pg_namespace": ["transformSelectStmt", "LookupNamespaceNoError"],
        "pg_type": ["transformExpr", "typenameTypeId"],
        "pg_operator": ["transformExpr", "OpernameGetOprid"],
    },
    "set": {
        "pg_db_role_setting": ["ExecSetVariableStmt", "AlterSetting"],
        "pg_authid": ["ExecSetVariableStmt", "AUTHNAME"],
    },
}

READ_SYMBOL_HINTS = {
    "pg_class": ("RangeVarGetRelid", "../postgres/src/backend/parser/parse_relation.c"),
    "pg_namespace": ("LookupNamespaceNoError", "../postgres/src/backend/catalog/namespace.c"),
    "pg_type": ("typenameTypeId", "../postgres/src/backend/parser/parse_type.c"),
    "pg_operator": ("OpernameGetOprid", "../postgres/src/backend/catalog/namespace.c"),
    "pg_db_role_setting": ("AlterSetting", "../postgres/src/backend/catalog/pg_db_role_setting.c"),
    "pg_authid": ("ExecSetVariableStmt", "../postgres/src/backend/utils/misc/guc_funcs.c"),
}

WRITE_SYMBOL_HINTS = {
    "AddNewRelationTuple": "../postgres/src/backend/catalog/heap.c",
    "AddNewAttributeTuples": "../postgres/src/backend/catalog/heap.c",
    "AddNewRelationType": "../postgres/src/backend/catalog/heap.c",
    "recordDependencyOn": "../postgres/src/backend/catalog/pg_depend.c",
    "NamespaceCreate": "../postgres/src/backend/catalog/namespace.c",
    "ProcedureCreate": "../postgres/src/backend/catalog/pg_proc.c",
    "InsertExtensionTuple": "../postgres/src/backend/commands/extension.c",
    "fill_seq_with_data": "../postgres/src/backend/commands/sequence.c",
    "CreateComments": "../postgres/src/backend/commands/comment.c",
    "ExecGrant_Relation": "../postgres/src/backend/catalog/aclchk.c",
    "CreateTriggerFiringOn": "../postgres/src/backend/commands/trigger.c",
    "CreatePolicy": "../postgres/src/backend/commands/policy.c",
}


def load_json(path: Path) -> Any:
    with path.open() as fh:
        return json.load(fh)


def save_json(path: Path, data: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = path.with_suffix(path.suffix + ".tmp")
    with tmp_path.open("w") as fh:
        json.dump(data, fh, indent=2)
        fh.write("\n")
    tmp_path.replace(path)


def family_path(family_id: str) -> Path:
    return FAMILIES_DIR / f"{family_id}.json"


def load_seed_map() -> dict[str, dict[str, Any]]:
    seed = load_json(SEED_PATH)
    return {family["id"]: family for family in seed["families"]}


def load_family(family_id: str) -> dict[str, Any]:
    return load_json(family_path(family_id))


def save_family(family_id: str, data: dict[str, Any]) -> None:
    save_json(family_path(family_id), data)


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def dedupe_items(items: list[dict[str, Any]], key_fields: list[str]) -> list[dict[str, Any]]:
    seen = set()
    result = []
    for item in items:
        key = tuple(item.get(field) for field in key_fields)
        if key in seen:
            continue
        seen.add(key)
        result.append(item)
    return result


def merge_evidence(existing: list[dict[str, Any]], incoming: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return dedupe_items(existing + incoming, ["path", "symbol", "stage"])


def rg_first(pattern: str, relative_root: str = "../postgres/src/backend") -> str | None:
    result = subprocess.run(
        ["rg", "-n", pattern, relative_root, "-S", "-m", "1"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0 or not result.stdout.strip():
        return None
    line = result.stdout.strip().splitlines()[0]
    path, _, remainder = line.partition(":")
    resolved = (ROOT / path).resolve()
    return f"../{resolved.relative_to(ROOT.parent)}:{remainder.split(':', 1)[0]}"


def rg_in_file(pattern: str, relative_file: str) -> str | None:
    result = subprocess.run(
        ["rg", "-n", pattern, relative_file, "-S", "-m", "1"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0 or not result.stdout.strip():
        return None
    line = result.stdout.strip().splitlines()[0]
    lineno, _, _ = line.partition(":")
    resolved = (ROOT / relative_file).resolve()
    return f"../{resolved.relative_to(ROOT.parent)}:{lineno}"


def resolve_symbol_path(symbol: str, fallback_path: str, relative_root: str = "../postgres/src/backend") -> str:
    preferred_patterns = [
        rf"^[[:space:]]*{symbol}[[:space:]]*\(",
        rf"^[[:space:]]*\*?[[:space:]]*{symbol}[[:space:]]*-",
        rf"\b{symbol}\b",
    ]
    for pattern in preferred_patterns:
        exact = rg_in_file(pattern, fallback_path)
        if exact:
            return exact
    exact = rg_first(preferred_patterns[0], relative_root=relative_root) or rg_first(rf"\b{symbol}\b", relative_root=relative_root)
    return exact or fallback_path


def build_stage_prompt(family_id: str, stage: str) -> str:
    family = load_family(family_id)
    stage_doc = (PG_SEMANTIC_DIR / "stages" / f"{stage}.md").read_text()
    return "\n".join(
        [
            "You are running one stage of the PostgreSQL semantic extraction pipeline.",
            f"Family: {family_id}",
            f"Stage: {stage}",
            "Source of truth: ../postgres",
            "Return JSON only. No prose.",
            "",
            "Current family artifact:",
            json.dumps(family, indent=2),
            "",
            "Stage contract:",
            stage_doc,
        ]
    )


def run_codex_exec(family_id: str, stage: str) -> dict[str, Any]:
    prompt = build_stage_prompt(family_id, stage)
    with tempfile.TemporaryDirectory() as tmp:
        out_path = Path(tmp) / "last_message.json"
        cmd = [
            CODEX_BIN,
            "exec",
            "--cd",
            str(ROOT),
            "--skip-git-repo-check",
            "--sandbox",
            "workspace-write",
            "--output-last-message",
            str(out_path),
            prompt,
        ]
        result = subprocess.run(cmd, cwd=ROOT, text=True, capture_output=True, check=False)
        if result.returncode != 0 or not out_path.exists():
            raise RuntimeError(result.stderr or result.stdout or "codex exec failed")
        raw = out_path.read_text().strip()
        return json.loads(raw)


def update_stage_status(family_id: str, stage: str, stage_status: str, scenario_count: int | None = None) -> None:
    state = load_state(STATE_PATH)
    state = update_stage(state, family_id, stage, stage_status, scenario_count=scenario_count)
    save_state(STATE_PATH, state)


def refresh_index() -> dict[str, Any]:
    seed_map = load_seed_map()
    state = load_json(STATE_PATH)
    state_map = {family["id"]: family for family in state["families"]}
    index = {
        "version": 1,
        "source_root": "../postgres",
        "generated_at": utc_now(),
        "families": [],
    }
    for family_id in sorted(seed_map):
        artifact = load_family(family_id)
        state_entry = state_map[family_id]
        index["families"].append(
            {
                "id": family_id,
                "slug": artifact["family"]["slug"],
                "category": artifact["family"]["category"],
                "path": str(family_path(family_id).relative_to(ROOT)),
                "status": artifact.get("status", state_entry["status"]),
                "scenario_count": len(artifact.get("scenarios", [])),
            }
        )
    save_json(INDEX_PATH, index)
    return index


def ensure_stage_ready(family: dict[str, Any], stage: str) -> None:
    stages = family["stage_status"]
    required = STAGE_DEPENDENCIES[stage]
    for item in required:
        if stages.get(item) != "done":
            raise RuntimeError(f"stage {stage} requires {item}=done")


def stage_discover(family_id: str, dry_run: bool = False) -> dict[str, Any]:
    seed_map = load_seed_map()
    family = load_family(family_id)
    scenarios = FAMILY_SCENARIOS.get(
        family_id,
        [{"id": f"{family_id}_default", "summary": f"Top-level {family_id} semantic path", "manual_review": False}],
    )
    evidence = [
        {
            "stage": "discover",
            "path": resolve_symbol_path(node, "../postgres/src/backend/parser/gram.y", relative_root="../postgres/src/backend/parser"),
            "symbol": node,
            "reason": "top-level AST entry node from seed universe",
        }
        for node in seed_map[family_id]["entry_ast_nodes"]
    ]
    payload = {
        "family_id": family_id,
        "scenarios": scenarios,
        "evidence": evidence,
        "scenario_count": len(scenarios),
    }
    if dry_run:
        return payload
    family["scenarios"] = scenarios
    family["evidence"] = merge_evidence(family.get("evidence", []), evidence)
    family["stage_status"]["discover"] = "done"
    save_family(family_id, family)
    update_stage_status(family_id, "discover", "done", scenario_count=len(scenarios))
    return payload


def stage_map(family_id: str, dry_run: bool = False) -> dict[str, Any]:
    family = load_family(family_id)
    ensure_stage_ready(family, "map")
    mapping = json.loads(json.dumps(FAMILY_MAP[family_id]))
    for section in ("dispatcher", "handlers"):
        for item in mapping[section]:
            item["source_path"] = resolve_symbol_path(item["symbol"], item["path"])
            item["path"] = item["source_path"]
            item["chain"] = DISPATCHER_CHAINS.get(family_id, {}).get(item["symbol"], [item["symbol"]])
    evidence = []
    for item in mapping["dispatcher"] + mapping["handlers"]:
        evidence.append(
            {
                "stage": "map",
                "path": item["source_path"],
                "symbol": item["symbol"],
                "reason": "dispatcher or primary handler for family",
            }
        )
    payload = {
        "family_id": family_id,
        "dispatcher": mapping["dispatcher"],
        "handlers": mapping["handlers"],
        "evidence": evidence,
    }
    if dry_run:
        return payload
    family["dispatcher"] = mapping["dispatcher"]
    family["handlers"] = mapping["handlers"]
    family["evidence"] = merge_evidence(family.get("evidence", []), evidence)
    family["stage_status"]["map"] = "done"
    save_family(family_id, family)
    update_stage_status(family_id, "map", "done")
    return payload


def stage_trace_writes(family_id: str, dry_run: bool = False) -> dict[str, Any]:
    family = load_family(family_id)
    ensure_stage_ready(family, "trace_writes")
    writes = json.loads(json.dumps(FAMILY_WRITES.get(family_id, [])))
    for item in writes:
        item["source_path"] = resolve_symbol_path(
            item["trigger"],
            WRITE_SYMBOL_HINTS.get(
                item["trigger"],
                family["handlers"][0]["path"] if family.get("handlers") else "../postgres/src/backend/tcop/utility.c",
            ),
        )
        item["chain"] = WRITE_CHAINS.get(family_id, {}).get(item["trigger"], [item["trigger"]])
    evidence = [
        {
            "stage": "trace_writes",
            "path": item["source_path"],
            "symbol": item["trigger"],
            "reason": f"catalog write touching {item['catalog']}",
        }
        for item in writes
    ]
    payload = {"family_id": family_id, "writes": writes, "evidence": evidence}
    if dry_run:
        return payload
    family["writes"] = writes
    family["evidence"] = merge_evidence(family.get("evidence", []), evidence)
    family["stage_status"]["trace_writes"] = "done"
    save_family(family_id, family)
    update_stage_status(family_id, "trace_writes", "done")
    return payload


def stage_trace_reads(family_id: str, dry_run: bool = False) -> dict[str, Any]:
    family = load_family(family_id)
    ensure_stage_ready(family, "trace_reads")
    reads = json.loads(json.dumps(FAMILY_READS.get(family_id, [])))
    fallback_path = "../postgres/src/backend/parser/analyze.c" if family_id in {"select", "insert", "update", "delete", "merge"} else "../postgres/src/backend/tcop/utility.c"
    for item in reads:
        hint_symbol, hint_path = READ_SYMBOL_HINTS.get(item["catalog"], (item["purpose"], fallback_path))
        item["source_path"] = resolve_symbol_path(hint_symbol, hint_path)
        item["chain"] = READ_CHAINS.get(family_id, {}).get(item["catalog"], [hint_symbol])
    evidence = [
        {
            "stage": "trace_reads",
            "path": item["source_path"],
            "symbol": item["purpose"] if item["catalog"] not in {"pg_db_role_setting", "pg_authid"} else item["catalog"],
            "reason": f"semantic read from {item['catalog']}",
        }
        for item in reads
    ]
    payload = {"family_id": family_id, "reads": reads, "evidence": evidence}
    if dry_run:
        return payload
    family["reads"] = reads
    family["evidence"] = merge_evidence(family.get("evidence", []), evidence)
    family["stage_status"]["trace_reads"] = "done"
    save_family(family_id, family)
    update_stage_status(family_id, "trace_reads", "done")
    return payload


def stage_map_tests(family_id: str, dry_run: bool = False) -> dict[str, Any]:
    family = load_family(family_id)
    ensure_stage_ready(family, "map_tests")
    regress_files = FAMILY_REGRESS.get(family_id, [])
    scenario_test_map = []
    for idx, scenario in enumerate(family.get("scenarios", [])):
        mapped = regress_files[idx:idx + 1] if idx < len(regress_files) else []
        scenario_test_map.append(
            {
                "scenario": scenario["id"],
                "files": mapped,
                "manual_review": not mapped,
            }
        )
    evidence = [
        {
            "stage": "map_tests",
            "path": f"../postgres/src/test/regress/sql/{filename}",
            "symbol": filename,
            "reason": "mapped regression coverage",
        }
        for filename in regress_files
    ]
    payload = {
        "family_id": family_id,
        "regress_files": regress_files,
        "scenario_test_map": scenario_test_map,
        "evidence": evidence,
    }
    if dry_run:
        return payload
    family["regress_files"] = regress_files
    for scenario in family.get("scenarios", []):
        match = next((item for item in scenario_test_map if item["scenario"] == scenario["id"]), None)
        if match is not None:
            scenario["regress_files"] = match["files"]
            scenario["manual_review"] = match["manual_review"]
    family["evidence"] = merge_evidence(family.get("evidence", []), evidence)
    family["stage_status"]["map_tests"] = "done"
    save_family(family_id, family)
    update_stage_status(family_id, "map_tests", "done")
    return payload


def stage_plan_translation(family_id: str, dry_run: bool = False) -> dict[str, Any]:
    family = load_family(family_id)
    ensure_stage_ready(family, "plan_translation")
    targets = []
    for item in family.get("dispatcher", []) + family.get("handlers", []):
        targets.append(
            {
                "symbol": item["symbol"],
                "path": item["path"],
                "kind": item["kind"],
            }
        )
    targets = dedupe_items(targets, ["symbol", "path", "kind"])
    evidence = [
        {
            "stage": "plan_translation",
            "path": item["path"],
            "symbol": item["symbol"],
            "reason": "ordered PG translation target",
        }
        for item in targets
    ]
    payload = {"family_id": family_id, "translation_targets": targets, "evidence": evidence}
    if dry_run:
        return payload
    family["translation_targets"] = targets
    family["evidence"] = merge_evidence(family.get("evidence", []), evidence)
    family["stage_status"]["plan_translation"] = "done"
    save_family(family_id, family)
    update_stage_status(family_id, "plan_translation", "done")
    return payload


def stage_synthesize(family_id: str, dry_run: bool = False) -> dict[str, Any]:
    family = load_family(family_id)
    ensure_stage_ready(family, "synthesize")
    stage_values = family["stage_status"]
    if all(stage_values.get(stage) == "done" for stage in ORDER[:-1]):
        family_status = "done"
    else:
        family_status = "in_progress"
    family["status"] = family_status
    family["updated_at"] = utc_now()
    family["stage_status"]["synthesize"] = "done"
    payload = {
        "family_id": family_id,
        "family_status": family_status,
        "scenario_count": len(family.get("scenarios", [])),
        "index_entry": {
            "id": family_id,
            "status": family_status,
            "scenario_count": len(family.get("scenarios", [])),
        },
    }
    if dry_run:
        return payload
    save_family(family_id, family)
    update_stage_status(family_id, "synthesize", "done", scenario_count=len(family.get("scenarios", [])))
    refresh_index()
    return payload


STAGE_FUNCS = {
    "discover": stage_discover,
    "map": stage_map,
    "trace_writes": stage_trace_writes,
    "trace_reads": stage_trace_reads,
    "map_tests": stage_map_tests,
    "plan_translation": stage_plan_translation,
    "synthesize": stage_synthesize,
}


def run_stage(family_id: str, stage: str, dry_run: bool = False, executor: str = "local") -> dict[str, Any]:
    LOGS_DIR.mkdir(parents=True, exist_ok=True)
    if dry_run:
        payload = STAGE_FUNCS[stage](family_id, dry_run=True)
        payload["executor"] = executor
        if executor == "codex":
            payload["codex_prompt_preview"] = build_stage_prompt(family_id, stage)[:400]
        return payload
    if not dry_run:
        family = load_family(family_id)
        for prereq in STAGE_DEPENDENCIES[stage]:
            if family["stage_status"].get(prereq) != "done":
                run_stage(family_id, prereq, dry_run=False, executor=executor)
                family = load_family(family_id)
    if executor == "codex":
        try:
            codex_payload = run_codex_exec(family_id, stage)
            # Require dict-shaped JSON; persist local fallback if Codex output is too weak.
            if isinstance(codex_payload, dict) and codex_payload:
                codex_payload["executor"] = "codex"
                local_payload = STAGE_FUNCS[stage](family_id, dry_run=False)
                family = load_family(family_id)
                family.setdefault("executor_history", []).append(
                    {"stage": stage, "executor": "codex", "captured_at": utc_now(), "raw_keys": sorted(codex_payload.keys())}
                )
                save_family(family_id, family)
                return local_payload
        except Exception:
            pass
    payload = STAGE_FUNCS[stage](family_id, dry_run=False)
    payload["executor"] = "local"
    return payload
