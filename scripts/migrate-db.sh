#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 执行数据库迁移"
go run ./cmd/migratectl/main.go migrate --dsn "${DB_URL}" --dir sql/migrations
