#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ -f "${PGLOGICAL_INIT_FILE}" ]; then
  echo ">>> 初始化 pglogical schema (REPLICATION_MODE=pglogical)"
  if PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -U "${DB_ADMIN_USER}" -d "${DB_NAME}" \
    -Atc "SELECT rolsuper FROM pg_roles WHERE rolname = current_user" 2>/dev/null | grep -qx 't'; then
    PGPASSWORD="${DB_ADMIN_PASS}" psql -h "${DB_HOST}" -U "${DB_ADMIN_USER}" -d "${DB_NAME}" \
      -v ON_ERROR_STOP=1 -f "${PGLOGICAL_INIT_FILE}"
  elif psql "${DB_URL}" -Atc "SELECT rolsuper FROM pg_roles WHERE rolname = current_user" | grep -qx 't'; then
    psql "${DB_URL}" -v ON_ERROR_STOP=1 -f "${PGLOGICAL_INIT_FILE}"
  else
    echo "⚠️ 当前用户非超级用户，跳过 pglogical 初始化"
  fi
fi

if [ -f "${PGLOGICAL_PATCH_FILE}" ]; then
  echo ">>> 应用 pglogical 默认值补丁"
  psql "${DB_URL}" -v ON_ERROR_STOP=1 -f "${PGLOGICAL_PATCH_FILE}"
fi
