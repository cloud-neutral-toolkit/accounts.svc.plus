#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ "${REPLICATION_MODE}" = "pglogical" ]; then
  echo ">>> 重新初始化 pglogical schema"
  bash scripts/init-db-pglogical.sh
else
  echo ">>> 当前 REPLICATION_MODE=${REPLICATION_MODE}，无需 pglogical 处理"
fi
