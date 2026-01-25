#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 初始化业务 schema (${SCHEMA_FILE})"
psql "${DB_URL}" -v ON_ERROR_STOP=1 -f "${SCHEMA_FILE}"
