#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ ! -f "${ACCOUNT_SYNC_CONFIG}" ]; then
  echo "❌ 未找到配置文件 ${ACCOUNT_SYNC_CONFIG}"
  exit 1
fi

go run ./cmd/syncctl/main.go push --config "${ACCOUNT_SYNC_CONFIG}"
