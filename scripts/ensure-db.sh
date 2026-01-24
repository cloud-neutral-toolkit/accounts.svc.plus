#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_ADMIN_USER}" -d postgres \
  -Atc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -qx '1'; then
  echo ">>> 数据库 ${DB_NAME} 已存在"
  exit 0
fi

echo ">>> 创建数据库 ${DB_NAME}"
PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_ADMIN_USER}" -d postgres \
  -c "CREATE DATABASE ${DB_NAME};"
