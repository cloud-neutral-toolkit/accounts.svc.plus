#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ -z "${GCP_PROJECT}" ]; then
  echo "❌ GCP_PROJECT 不能为空"
  exit 1
fi
if [ ! -f "${CLOUD_RUN_STUNNEL_CONF}" ]; then
  echo "❌ 未找到 stunnel 配置: ${CLOUD_RUN_STUNNEL_CONF}"
  exit 1
fi

gcloud secrets versions add stunnel-config --data-file "${CLOUD_RUN_STUNNEL_CONF}" --project "${GCP_PROJECT}"
