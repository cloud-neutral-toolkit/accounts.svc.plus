#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo ">>> 导出 schema 到 ${SCHEMA_FILE}"
PG_DUMP_BIN="${PG_DUMP_BIN:-pg_dump}"
SERVER_VERSION="$(psql "${DB_URL}" -Atc "SHOW server_version" 2>/dev/null || true)"
SERVER_MAJOR="${SERVER_VERSION%%.*}"
LOCAL_VERSION="$(${PG_DUMP_BIN} --version 2>/dev/null || true)"
LOCAL_MAJOR="$(echo "${LOCAL_VERSION}" | awk '{print $3}' | cut -d. -f1)"

if [ -n "${SERVER_MAJOR}" ] && [ -n "${LOCAL_MAJOR}" ] && [ "${SERVER_MAJOR}" != "${LOCAL_MAJOR}" ]; then
  echo "⚠️ pg_dump 版本不匹配（server=${SERVER_MAJOR}, local=${LOCAL_MAJOR}），跳过导出"
  echo "   可设置 PG_DUMP_BIN 指向匹配版本的 pg_dump"
  exit 0
fi

TMP_SCHEMA="/tmp/schema.$$.sql"
${PG_DUMP_BIN} -s -O -x "${DB_URL}" > "${TMP_SCHEMA}"
if [ ! -w "${SCHEMA_FILE}" ]; then
  echo "⚠️ ${SCHEMA_FILE} 不可写，已将导出结果保留在 ${TMP_SCHEMA}"
  exit 0
fi

mv "${TMP_SCHEMA}" "${SCHEMA_FILE}"
