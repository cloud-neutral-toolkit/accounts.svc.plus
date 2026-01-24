#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

: "${REGION_DB_URL:?❌ 缺少 REGION_DB_URL}"
: "${NODE_NAME:?❌ 缺少 NODE_NAME}"
: "${NODE_DSN:?❌ 缺少 NODE_DSN}"
: "${SUBSCRIPTION_NAME:?❌ 缺少 SUBSCRIPTION_NAME}"
: "${PROVIDER_DSN:?❌ 缺少 PROVIDER_DSN}"

psql "${REGION_DB_URL}" -v ON_ERROR_STOP=1 \
  -v NODE_NAME="${NODE_NAME}" \
  -v NODE_DSN="${NODE_DSN}" \
  -v SUBSCRIPTION_NAME="${SUBSCRIPTION_NAME}" \
  -v PROVIDER_DSN="${PROVIDER_DSN}" \
  -f "${PGLOGICAL_REGION_FILE}"
