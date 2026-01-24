#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ -z "${SUPERADMIN_USERNAME}" ] || [ -z "${SUPERADMIN_PASSWORD}" ]; then
  echo "❌ 请指定用户名与密码"
  exit 1
fi

go run ./cmd/createadmin/main.go \
  --driver postgres \
  --dsn "${DB_URL}" \
  --username "${SUPERADMIN_USERNAME}" \
  --password "${SUPERADMIN_PASSWORD}" \
  --email "${SUPERADMIN_EMAIL}"
