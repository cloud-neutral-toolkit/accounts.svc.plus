#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 重置业务 schema (sql/schema.sql)"
bash scripts/reset-public-schema.sh
bash scripts/init-db-core.sh
