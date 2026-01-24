#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

REGION_DB_URL="${DB_URL}" \
NODE_NAME="node_global" \
NODE_DSN="host=global-homepage.svc.plus port=5432 dbname=account user=pglogical password=xxxx" \
SUBSCRIPTION_NAME="sub_from_cn" \
PROVIDER_DSN="host=cn-homepage.svc.plus port=5432 dbname=account user=pglogical password=xxxx" \
  bash scripts/init-pglogical-region.sh
