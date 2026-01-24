#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

args=(export --dsn "${DB_URL}" --output "${ACCOUNT_EXPORT_FILE}")
if [ -n "${ACCOUNT_EMAIL_KEYWORD-}" ]; then
  args+=(--email "${ACCOUNT_EMAIL_KEYWORD}")
fi

go run ./cmd/migratectl/main.go "${args[@]}"
