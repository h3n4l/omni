#!/usr/bin/env python3
"""Pipeline metrics storage in SQLite.

CLI usage:
    python3 metrics.py init <db>
    python3 metrics.py phase <db> <engine> <new_phase>
    python3 metrics.py diff-batches <db> <engine> <progress_file>

Library usage:
    from metrics import MetricsDB
    db = MetricsDB("metrics.db")
    db.record_invocation(engine="mysql", phase="build", result_event={...})
"""

import json
import sqlite3
import sys
from datetime import datetime


class MetricsDB:
    def __init__(self, db_path):
        self.db_path = db_path
        self.conn = sqlite3.connect(db_path)
        self._init_tables()

    def _init_tables(self):
        self.conn.executescript("""
            CREATE TABLE IF NOT EXISTS invocations (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                engine TEXT NOT NULL,
                phase TEXT NOT NULL,
                timestamp TEXT NOT NULL,
                duration_ms INTEGER,
                duration_api_ms INTEGER,
                cost_usd REAL,
                num_turns INTEGER,
                input_tokens INTEGER,
                output_tokens INTEGER,
                cache_read_tokens INTEGER,
                cache_creation_tokens INTEGER,
                model_usage TEXT,
                log_file TEXT,
                session_id TEXT,
                is_error INTEGER DEFAULT 0,
                error_text TEXT
            );

            CREATE TABLE IF NOT EXISTS batch_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                engine TEXT NOT NULL,
                batch_id INTEGER,
                batch_name TEXT,
                old_status TEXT,
                new_status TEXT,
                timestamp TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS phase_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                engine TEXT NOT NULL,
                old_phase TEXT,
                new_phase TEXT,
                timestamp TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS semantic_stage_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                family TEXT NOT NULL,
                stage TEXT NOT NULL,
                status TEXT NOT NULL,
                scenario_count INTEGER DEFAULT 0,
                timestamp TEXT NOT NULL
            );
        """)
        self.conn.commit()

    def record_invocation(self, engine, phase, result_event, log_file=None):
        usage = result_event.get("usage", {})
        model_usage = result_event.get("modelUsage", {})
        is_error = result_event.get("is_error", False)

        self.conn.execute("""
            INSERT INTO invocations
            (engine, phase, timestamp, duration_ms, duration_api_ms, cost_usd,
             num_turns, input_tokens, output_tokens, cache_read_tokens,
             cache_creation_tokens, model_usage, log_file, session_id,
             is_error, error_text)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            engine, phase,
            datetime.now().isoformat(),
            result_event.get("duration_ms"),
            result_event.get("duration_api_ms"),
            result_event.get("total_cost_usd"),
            result_event.get("num_turns"),
            usage.get("input_tokens"),
            usage.get("output_tokens"),
            usage.get("cache_read_input_tokens"),
            usage.get("cache_creation_input_tokens"),
            json.dumps(model_usage) if model_usage else None,
            log_file,
            result_event.get("session_id"),
            1 if is_error else 0,
            "; ".join(result_event.get("errors", [])) if is_error else None,
        ))
        self.conn.commit()

    def record_phase(self, engine, new_phase):
        row = self.conn.execute(
            "SELECT new_phase FROM phase_events WHERE engine=? ORDER BY id DESC LIMIT 1",
            (engine,),
        ).fetchone()
        old_phase = row[0] if row else "idle"

        if old_phase == new_phase:
            return

        self.conn.execute(
            "INSERT INTO phase_events (engine, old_phase, new_phase, timestamp) VALUES (?, ?, ?, ?)",
            (engine, old_phase, new_phase, datetime.now().isoformat()),
        )
        self.conn.commit()

    def diff_batches(self, engine, progress_file):
        """Compare current PROGRESS.json with last known state and record changes."""
        with open(progress_file) as f:
            data = json.load(f)

        # Build last-known map: batch_id -> most recent status
        rows = self.conn.execute(
            "SELECT batch_id, new_status FROM batch_events WHERE engine=? ORDER BY id",
            (engine,),
        ).fetchall()
        last_known = {}
        for batch_id, status in rows:
            last_known[batch_id] = status

        now = datetime.now().isoformat()
        changes = []
        for b in data.get("batches", []):
            bid = b["id"]
            new_status = b["status"]
            old_status = last_known.get(bid)

            if old_status != new_status:
                changes.append((engine, bid, b.get("name", ""), old_status or "", new_status, now))

        if changes:
            self.conn.executemany(
                "INSERT INTO batch_events (engine, batch_id, batch_name, old_status, new_status, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
                changes,
            )
            self.conn.commit()

        return len(changes)

    def close(self):
        self.conn.close()

    def record_semantic_stage(self, family, stage, status, scenario_count=0):
        self.conn.execute(
            "INSERT INTO semantic_stage_events (family, stage, status, scenario_count, timestamp) VALUES (?, ?, ?, ?, ?)",
            (family, stage, status, scenario_count, datetime.now().isoformat()),
        )
        self.conn.commit()


def main():
    if len(sys.argv) < 2:
        print("Usage: metrics.py <command> [args]")
        print("Commands: init, phase, diff-batches, semantic-stage")
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "init":
        db_path = sys.argv[2] if len(sys.argv) > 2 else "metrics.db"
        db = MetricsDB(db_path)
        db.close()
        print(f"Initialized {db_path}")

    elif cmd == "phase":
        if len(sys.argv) < 5:
            print("Usage: metrics.py phase <db> <engine> <new_phase>")
            sys.exit(1)
        db_path, engine, new_phase = sys.argv[2], sys.argv[3], sys.argv[4]
        db = MetricsDB(db_path)
        db.record_phase(engine, new_phase)
        db.close()

    elif cmd == "diff-batches":
        if len(sys.argv) < 5:
            print("Usage: metrics.py diff-batches <db> <engine> <progress_file>")
            sys.exit(1)
        db_path, engine, progress_file = sys.argv[2], sys.argv[3], sys.argv[4]
        db = MetricsDB(db_path)
        n = db.diff_batches(engine, progress_file)
        db.close()
        if n > 0:
            print(f"  Recorded {n} batch status changes")

    elif cmd == "semantic-stage":
        if len(sys.argv) < 6:
            print("Usage: metrics.py semantic-stage <db> <family> <stage> <status> [scenario_count]")
            sys.exit(1)
        db_path, family, stage, status = sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5]
        scenario_count = int(sys.argv[6]) if len(sys.argv) > 6 else 0
        db = MetricsDB(db_path)
        db.record_semantic_stage(family, stage, status, scenario_count)
        db.close()

    else:
        print(f"Unknown command: {cmd}")
        sys.exit(1)


if __name__ == "__main__":
    main()
