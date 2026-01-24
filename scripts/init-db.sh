#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 初始化数据库 schema"
if ! command -v psql >/dev/null; then
  echo "❌ 未检测到 psql，请安装 PostgreSQL 客户端"
  exit 1
fi

bash scripts/init-db-core.sh
bash scripts/init-db-replication.sh
