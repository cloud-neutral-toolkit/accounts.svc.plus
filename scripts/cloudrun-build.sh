#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ -z "${GCP_PROJECT}" ]; then
  echo "⚠️ GCP_PROJECT 不能为空，跳过 Cloud Run 构建"
  exit 0
fi

gcloud builds submit --tag "${CLOUD_RUN_IMAGE}" .
