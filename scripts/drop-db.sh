#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo "⚠️  即将删除数据库 ${DB_NAME} ..."
read -r -p "确定要删除数据库 ${DB_NAME}? [y/N] " confirm
if [ "${confirm}" = "y" ] || [ "${confirm}" = "Y" ]; then
  echo ">>> 强制断开现有连接 ..."
  if ! PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -U "${DB_ADMIN_USER}" -d postgres \
    -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='${DB_NAME}' AND pid <> pg_backend_pid();"; then
    echo "⚠️ 无法断开所有连接（需要超级用户权限）"
  fi
  echo ">>> 清理 pglogical schema ..."
  PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -U "${DB_ADMIN_USER}" -d "${DB_NAME}" \
    -c "DROP SCHEMA IF EXISTS pglogical CASCADE;" >/dev/null 2>&1 || \
    echo "⚠️ 无法删除 pglogical schema（数据库可能不存在或缺少权限）"
  echo ">>> 删除数据库 ${DB_NAME} ..."
  if PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -U "${DB_ADMIN_USER}" -d postgres \
    -c "DROP DATABASE IF EXISTS ${DB_NAME};"; then
    echo ">>> 数据库已删除"
  else
    echo ">>> 删除失败"
  fi
else
  echo "取消删除"
fi
