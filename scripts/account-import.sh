#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ ! -f "${ACCOUNT_IMPORT_FILE}" ]; then
  echo "❌ 未找到文件 ${ACCOUNT_IMPORT_FILE}"
  exit 1
fi

args=(import --dsn "${DB_URL}" --file "${ACCOUNT_IMPORT_FILE}")
if [ -n "${ACCOUNT_IMPORT_MERGE-}" ]; then
  args+=(--merge)
fi
if [ -n "${ACCOUNT_IMPORT_MERGE_STRATEGY-}" ]; then
  args+=(--merge-strategy "${ACCOUNT_IMPORT_MERGE_STRATEGY}")
fi
if [ -n "${ACCOUNT_IMPORT_DRY_RUN-}" ]; then
  args+=(--dry-run)
fi
if [ -n "${ACCOUNT_IMPORT_MERGE_ALLOWLIST-}" ]; then
  for uuid in ${ACCOUNT_IMPORT_MERGE_ALLOWLIST}; do
    args+=(--merge-allowlist "${uuid}")
  done
fi
if [ -n "${ACCOUNT_IMPORT_EXTRA_FLAGS-}" ]; then
  # shellcheck disable=SC2206
  extra=( ${ACCOUNT_IMPORT_EXTRA_FLAGS} )
  args+=("${extra[@]}")
fi

go run ./cmd/migratectl/main.go "${args[@]}"
