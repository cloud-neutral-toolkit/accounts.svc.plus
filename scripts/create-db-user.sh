#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 创建数据库用户 ${DB_USER}"
if ! command -v psql >/dev/null; then
  echo "❌ 未检测到 psql，请安装 PostgreSQL 客户端"
  exit 1
fi

echo "正在以管理员身份创建用户..."
if PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_ADMIN_USER}" -d postgres \
  -Atc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -qx '1'; then
  echo "⚠️ 用户可能已存在"
else
  PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_ADMIN_USER}" -d postgres \
    -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASS}';"
fi
PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_ADMIN_USER}" -d postgres \
  -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"
echo "✓ 数据库用户创建完成"
