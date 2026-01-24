#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ -z "${SUPERADMIN_USERNAME}" ] || [ -z "${SUPERADMIN_PASSWORD}" ]; then
  echo "❌ 请指定用户名与密码"
  exit 1
fi

if psql "${DB_URL}" -Atc "SELECT 1 FROM users WHERE username='${SUPERADMIN_USERNAME}' OR email='${SUPERADMIN_EMAIL}' LIMIT 1" | grep -qx '1'; then
  echo "⚠️ 超级管理员已存在，跳过创建"
  exit 0
fi

go run ./cmd/createadmin/main.go \
  --driver postgres \
  --dsn "${DB_URL}" \
  --username "${SUPERADMIN_USERNAME}" \
  --password "${SUPERADMIN_PASSWORD}" \
  --email "${SUPERADMIN_EMAIL}"
