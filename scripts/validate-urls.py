#!/usr/bin/env python3
"""Validate all documentation URLs in BNF_CATALOG.json.

Checks each URL for HTTP 200 response. Updates status to "url_ok" or "url_404".

Usage: python3 scripts/validate-urls.py <engine>
"""

import json
import os
import sys
import urllib.request
import urllib.error
from concurrent.futures import ThreadPoolExecutor, as_completed

def check_url(base_url, slug, suffix=".html"):
    """Check if URL returns 200. Falls back to GET if HEAD returns 403."""
    url = f"{base_url}/{slug}{suffix}"
    for method in ("HEAD", "GET"):
        req = urllib.request.Request(url, method=method, headers={
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
        })
        try:
            with urllib.request.urlopen(req, timeout=15) as resp:
                return slug, resp.status, url
        except urllib.error.HTTPError as e:
            if e.code == 403 and method == "HEAD":
                continue  # retry with GET
            return slug, e.code, url
        except Exception:
            return slug, 0, url
    return slug, 0, url


def main():
    if len(sys.argv) < 2:
        print("Usage: validate-urls.py <engine>")
        sys.exit(1)

    engine = sys.argv[1]
    omni_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    catalog_file = os.path.join(omni_dir, engine, "parser", f"{engine.upper()}_BNF_CATALOG.json")

    with open(catalog_file) as f:
        catalog = json.load(f)

    base_url = catalog.get("base_url") or catalog.get("doc_base_url")
    statements = catalog["statements"]

    # MSSQL docs don't use .html suffix
    suffix = "" if engine == "mssql" else ".html"

    print(f"Engine: {engine}")
    print(f"Base URL: {base_url}")
    print(f"Total statements: {len(statements)}")
    print()

    ok_count = 0
    fail_count = 0

    # Check URLs in parallel
    with ThreadPoolExecutor(max_workers=10) as executor:
        futures = {}
        for stmt in statements:
            slug = stmt["slug"]
            future = executor.submit(check_url, base_url, slug, suffix)
            futures[future] = stmt

        for future in as_completed(futures):
            stmt = futures[future]
            slug, status, url = future.result()
            if status == 200:
                stmt["status"] = "url_ok"
                stmt["url"] = url
                ok_count += 1
            else:
                stmt["status"] = "url_404"
                stmt["url"] = url
                stmt["http_status"] = status
                fail_count += 1
                print(f"  FAIL [{status}]: {stmt['name']} -> {url}")

    # Save updated catalog
    with open(catalog_file, "w") as f:
        json.dump(catalog, f, indent=2)
        f.write("\n")

    print()
    print(f"Results: {ok_count} OK, {fail_count} FAILED")
    if fail_count > 0:
        print("Fix failed URLs in BNF_CATALOG.json before proceeding.")
        sys.exit(1)
    else:
        print("All URLs validated successfully!")


if __name__ == "__main__":
    main()
