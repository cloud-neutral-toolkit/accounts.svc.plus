#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

psql "${DB_URL}" -v ON_ERROR_STOP=1 -v db_user="${DB_USER}" -f sql/reset_public_schema.sql
