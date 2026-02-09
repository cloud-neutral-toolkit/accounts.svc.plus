#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
  echo "✅ 已读取 .env"
fi

if ! command -v curl >/dev/null; then
  echo "❌ 未找到 curl，请先安装或确保在 PATH 中" >&2
  exit 1
fi

API_BASE_URL="${API_BASE_URL:-${BASE_URL:-https://accounts.svc.plus}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-admin@svc.plus}"
LOGIN_PASSWORD="${SUPERADMIN_PASSWORD:-}"

if [ -z "$LOGIN_PASSWORD" ]; then
  echo "❌ 缺少 SUPERADMIN_PASSWORD（可写入 .env）" >&2
  exit 1
fi

login_payload=$(cat <<JSON
{"email":"${LOGIN_EMAIL}","identifier":"${LOGIN_EMAIL}","password":"${LOGIN_PASSWORD}"}
JSON
)

login_response=$(curl -sS -w "\n%{http_code}" -X POST "${API_BASE_URL}/api/auth/login" \
  -H "Content-Type: application/json" \
  -d "$login_payload")

login_body=$(printf "%s" "$login_response" | sed '$d')
login_status=$(printf "%s" "$login_response" | tail -n1)

if [ "$login_status" != "200" ]; then
  echo "❌ 登录失败: HTTP ${login_status}" >&2
  echo "$login_body" >&2
  exit 1
fi

if command -v python3 >/dev/null; then
  token=$(printf "%s" "$login_body" | python3 - <<'PY'
import json, sys
try:
  payload = json.load(sys.stdin)
  print(payload.get('token',''))
except Exception:
  print('')
PY
)
else
  echo "❌ 未找到 python3，无法解析登录 token" >&2
  exit 1
fi

if [ -z "$token" ]; then
  echo "❌ 登录响应中未包含 token" >&2
  echo "$login_body" >&2
  exit 1
fi

session_response=$(curl -sS -w "\n%{http_code}" -X GET "${API_BASE_URL}/api/auth/session" \
  -H "Authorization: Bearer ${token}")

session_body=$(printf "%s" "$session_response" | sed '$d')
session_status=$(printf "%s" "$session_response" | tail -n1)

if [ "$session_status" != "200" ]; then
  echo "❌ session 校验失败: HTTP ${session_status}" >&2
  echo "$session_body" >&2
  exit 1
fi

echo "✅ API 登录测试通过"
