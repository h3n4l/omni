#!/bin/bash
# Deprecated wrapper for the legacy PG catalog pipeline.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "Deprecated wrapper: pg/catalog pipeline has moved to pg/semantic."
if [[ $# -eq 0 ]]; then
    exec "$ROOT_DIR/scripts/pg-semantic-driver.sh" --all
fi
exec "$ROOT_DIR/scripts/pg-semantic-driver.sh" "$@"
