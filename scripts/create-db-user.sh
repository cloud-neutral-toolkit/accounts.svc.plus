#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 创建数据库用户 ${DB_USER}"
if ! command -v psql >/dev/null; then
  echo "❌ 未检测到 psql，请安装 PostgreSQL 客户端"
  exit 1
fi

echo "正在以 postgres 超级用户身份创建用户..."
sudo -u postgres psql -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASS}';" || echo "⚠️ 用户可能已存在"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"
echo "✓ 数据库用户创建完成"
