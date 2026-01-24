#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 导出 schema 到 ${SCHEMA_FILE}"
pg_dump -s -O -x "${DB_URL}" > "${SCHEMA_FILE}"
