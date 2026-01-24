#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

CONF_FILE="${CLOUD_RUN_STUNNEL_CONF}"
if [ ! -f "${CONF_FILE}" ]; then
  echo "❌ 未找到 stunnel 配置: ${CONF_FILE}"
  exit 1
fi

if ! command -v stunnel >/dev/null; then
  echo "❌ 未检测到 stunnel，请先安装"
  exit 1
fi

if command -v ss >/dev/null; then
  if ss -ltn 2>/dev/null | grep -q ':15432'; then
    echo "✅ stunnel 已在 127.0.0.1:15432 监听"
    exit 0
  fi
elif command -v lsof >/dev/null; then
  if lsof -nP -iTCP:15432 -sTCP:LISTEN | grep -q LISTEN; then
    echo "✅ stunnel 已在 127.0.0.1:15432 监听"
    exit 0
  fi
fi

echo ">>> 启动 stunnel (client)"
# stunnel 需要写入 /var/run，优先使用 sudo 启动
if sudo -n true 2>/dev/null; then
  sudo stunnel "${CONF_FILE}" &
  echo "✅ stunnel 启动完成 (sudo)"
  exit 0
fi

echo "⚠️ sudo 不可用，使用用户态临时配置启动"
TMP_CONF="/tmp/stunnel-account-db-client.conf"
sed \
  -e 's#^pid = .*#pid = /tmp/stunnel-account-db-client.pid#' \
  -e 's#^output = .*#output = /tmp/stunnel-account-db-client.log#' \
  "${CONF_FILE}" > "${TMP_CONF}"

if [ ! -f /etc/ssl/certs/ca-certificates.crt ] && [ -f /etc/ssl/cert.pem ]; then
  sed -i '' 's#^CAfile = .*#CAfile = /etc/ssl/cert.pem#' "${TMP_CONF}"
fi

stunnel "${TMP_CONF}" &
echo "✅ stunnel 启动完成 (user mode)"
