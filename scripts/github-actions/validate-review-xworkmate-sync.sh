#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-https://accounts.svc.plus}"
REVIEW_ACCOUNT_EMAIL="${REVIEW_ACCOUNT_EMAIL:-review@svc.plus}"
REVIEW_ACCOUNT_PASSWORD="${REVIEW_ACCOUNT_PASSWORD:-Review123!}"

login_json="$(
  curl \
    --silent \
    --show-error \
    --fail \
    --location \
    --max-time 20 \
    --header 'Content-Type: application/json' \
    --header 'Accept: application/json' \
    --data "{\"identifier\":\"${REVIEW_ACCOUNT_EMAIL}\",\"password\":\"${REVIEW_ACCOUNT_PASSWORD}\"}" \
    "${BASE_URL}/api/auth/login"
)"

session_token="$(
  LOGIN_JSON="${login_json}" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["LOGIN_JSON"])
token = (payload.get("token") or payload.get("access_token") or "").strip()
if not token:
    raise SystemExit("review account login did not return a session token")
print(token)
PY
)"

sync_json="$(
  curl \
    --silent \
    --show-error \
    --fail \
    --location \
    --max-time 20 \
    --header 'Accept: application/json' \
    --header "Authorization: Bearer ${session_token}" \
    "${BASE_URL}/api/auth/xworkmate/profile/sync"
)"

SYNC_JSON="${sync_json}" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["SYNC_JSON"])
bridge_server_url = (payload.get("BRIDGE_SERVER_URL") or "").strip()
bridge_auth_token = (payload.get("BRIDGE_AUTH_TOKEN") or "").strip()

if not bridge_server_url:
    raise SystemExit("review xworkmate sync did not return BRIDGE_SERVER_URL")

if not bridge_auth_token:
    raise SystemExit("review xworkmate sync did not return BRIDGE_AUTH_TOKEN")
PY
