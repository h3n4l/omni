#!/usr/bin/env python3
"""Pre-fetch official documentation for pending batches.

Reads PROGRESS.json, extracts doc URLs for pending/in_progress batches,
downloads HTML, converts to plain text, and saves to {engine}/parser/docs/.

Usage: python3 scripts/prefetch-docs.py <engine>
"""

import json
import os
import re
import sys
import urllib.request
import urllib.error
from html.parser import HTMLParser

# URL patterns per engine
URL_PATTERNS = {
    "oracle": [
        "https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/{slug}.html",
        "https://docs.oracle.com/en/database/oracle/oracle-database/23/lnpls/{slug}.html",
        "https://docs.oracle.com/en/database/oracle/oracle-database/26/sqlrf/{slug}.html",
        "https://docs.oracle.com/en/database/oracle/oracle-database/26/lnpls/{slug}.html",
    ],
    "pg": [
        "https://www.postgresql.org/docs/17/sql-{slug}.html",
    ],
    "mysql": [
        "https://dev.mysql.com/doc/refman/8.0/en/{slug}.html",
    ],
    "mssql": [
        "https://learn.microsoft.com/en-us/sql/t-sql/statements/{slug}",
    ],
}

# Manual slug overrides for Oracle docs where rule names don't match page URLs
ORACLE_SLUG_OVERRIDES = {
    # rule_name (without _stmt/_clause suffix) -> actual doc page slug(s)
    "create_audit_policy": ["CREATE-AUDIT-POLICY-Unified-Auditing"],
    "alter_audit_policy": ["ALTER-AUDIT-POLICY-Unified-Auditing"],
    "drop_audit_policy": ["DROP-AUDIT-POLICY-Unified-Auditing"],
    "create_json_duality_view": ["CREATE-JSON-RELATIONAL-DUALITY-VIEW"],
    "alter_json_duality_view": ["ALTER-JSON-RELATIONAL-DUALITY-VIEW"],
    "drop_json_duality_view": ["DROP-JSON-RELATIONAL-DUALITY-VIEW"],
    "create_mle_env": ["CREATE-MLE-ENV"],
    "create_mle_module": ["CREATE-MLE-MODULE"],
    "drop_mle_env": ["DROP-MLE-ENV"],
    "drop_mle_module": ["DROP-MLE-MODULE"],
    "create_property_graph": ["CREATE-PROPERTY-GRAPH"],
    "create_vector_index": ["CREATE-VECTOR-INDEX"],
    "create_logical_partition_tracking": ["CREATE-LOGICAL-PARTITION-TRACKING"],
    "create_pmem_filestore": ["CREATE-PMEM-FILESTORE"],
    "truncate_cluster": ["TRUNCATE-CLUSTER"],
    "drop_type_body": ["DROP-TYPE-BODY"],
    "create_controlfile": ["CREATE-CONTROLFILE"],
    "alter_database_dictionary": ["ALTER-DATABASE-DICTIONARY"],
    # Sub-clauses that are part of parent statement docs
    "tablespace_datafile": ["CREATE-TABLESPACE", "ALTER-TABLESPACE"],
    "tablespace_size": ["CREATE-TABLESPACE", "ALTER-TABLESPACE"],
    "cluster_hash": ["CREATE-CLUSTER"],
    "dimension_level": ["CREATE-DIMENSION"],
    "dimension_hierarchy": ["CREATE-DIMENSION"],
}


# Map batch rule names to doc page slugs
def rules_to_slugs(engine, batch):
    """Extract likely doc page slugs from batch name and rules."""
    slugs = set()
    name = batch.get("name", "")
    rules = batch.get("rules", [])
    desc = batch.get("description", "")

    for rule in rules:
        # Remove _stmt, _clause suffix
        r = re.sub(r"_(stmt|clause|full)$", "", rule)

        if engine == "oracle":
            # Check manual overrides first
            if r in ORACLE_SLUG_OVERRIDES:
                for s in ORACLE_SLUG_OVERRIDES[r]:
                    slugs.add(s)
                continue
            # Standard: create_table -> CREATE-TABLE
            slug = r.replace("_", "-").upper()
            slugs.add(slug)
            # Also try lowercase for PL/SQL docs
            slugs.add(r.replace("_", "-").lower())
        elif engine == "pg":
            slug = r.replace("_", "")
            slugs.add(slug)
        elif engine == "mysql":
            slug = r.replace("_", "-")
            slugs.add(slug)
        elif engine == "mssql":
            slug = r.replace("_", "-").lower()
            slugs.add(slug)

    # Extract from batch name for Oracle
    if engine == "oracle":
        parts = name.replace("_", " ").split()
        for i in range(len(parts)):
            for j in range(i + 1, min(i + 4, len(parts) + 1)):
                slug = "-".join(parts[i:j]).upper()
                if any(kw in slug.upper() for kw in ["CREATE", "ALTER", "DROP", "GRANT", "REVOKE"]):
                    slugs.add(slug)

    return slugs


class HTMLTextExtractor(HTMLParser):
    """Simple HTML to text converter that preserves structure."""

    def __init__(self):
        super().__init__()
        self.result = []
        self.skip = False
        self.in_pre = False

    def handle_starttag(self, tag, attrs):
        if tag in ("script", "style", "nav", "header", "footer"):
            self.skip = True
        elif tag == "pre":
            self.in_pre = True
        elif tag in ("p", "div", "li", "tr", "h1", "h2", "h3", "h4", "br"):
            self.result.append("\n")

    def handle_endtag(self, tag):
        if tag in ("script", "style", "nav", "header", "footer"):
            self.skip = False
        elif tag == "pre":
            self.in_pre = False
            self.result.append("\n")
        elif tag in ("p", "div", "h1", "h2", "h3", "h4"):
            self.result.append("\n")

    def handle_data(self, data):
        if not self.skip:
            self.result.append(data)

    def get_text(self):
        text = "".join(self.result)
        # Collapse excessive blank lines
        text = re.sub(r"\n{3,}", "\n\n", text)
        return text.strip()


def fetch_url(url):
    """Fetch URL and return text content."""
    req = urllib.request.Request(url, headers={
        "User-Agent": "Mozilla/5.0 (compatible; OmniParser/1.0)"
    })
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            html = resp.read().decode("utf-8", errors="replace")
            extractor = HTMLTextExtractor()
            extractor.feed(html)
            return extractor.get_text()
    except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError) as e:
        return None


def main():
    if len(sys.argv) < 2:
        print("Usage: prefetch-docs.py <engine>")
        sys.exit(1)

    engine = sys.argv[1]
    omni_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    progress_file = os.path.join(omni_dir, engine, "parser", "PROGRESS.json")
    docs_dir = os.path.join(omni_dir, engine, "parser", "docs")

    if not os.path.exists(progress_file):
        print(f"Error: {progress_file} not found")
        sys.exit(1)

    os.makedirs(docs_dir, exist_ok=True)

    with open(progress_file) as f:
        data = json.load(f)

    patterns = URL_PATTERNS.get(engine, [])
    if not patterns:
        print(f"No URL patterns for engine: {engine}")
        sys.exit(1)

    # Collect slugs from pending/in_progress batches
    all_slugs = set()
    for batch in data["batches"]:
        if batch["status"] in ("pending", "in_progress"):
            all_slugs.update(rules_to_slugs(engine, batch))

    print(f"Engine: {engine}")
    print(f"Slugs to fetch: {len(all_slugs)}")

    fetched = 0
    skipped = 0
    failed = 0

    for slug in sorted(all_slugs):
        # Check if already cached (including NOT_FOUND markers)
        cache_file = os.path.join(docs_dir, f"{slug}.txt")
        if os.path.exists(cache_file):
            skipped += 1
            continue

        # Try each URL pattern
        text = None
        for pattern in patterns:
            url = pattern.format(slug=slug)
            text = fetch_url(url)
            if text and len(text) > 200:
                break
            text = None

        if text:
            with open(cache_file, "w") as f:
                f.write(f"# {slug}\n")
                f.write(f"# Source: {url}\n\n")
                f.write(text)
            fetched += 1
            print(f"  OK: {slug} ({len(text)} chars)")
        else:
            # Write a NOT_FOUND marker so Claude skips WebFetch for this slug
            with open(cache_file, "w") as f:
                f.write(f"# {slug}\n")
                f.write("# NOT_FOUND: This document does not exist in official documentation.\n")
                f.write("# Do NOT attempt to WebFetch this URL — it will 404.\n")
                f.write("# Implement based on the batch description and existing code patterns.\n")
            failed += 1
            print(f"  MISS: {slug}")

    print(f"\nDone: {fetched} fetched, {skipped} cached, {failed} missed")


if __name__ == "__main__":
    main()
