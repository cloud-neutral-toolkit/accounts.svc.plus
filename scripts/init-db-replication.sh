#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ "${REPLICATION_MODE}" = "pglogical" ]; then
  bash scripts/init-db-pglogical.sh
else
  echo ">>> 跳过 pglogical 初始化 (REPLICATION_MODE=${REPLICATION_MODE})"
fi
