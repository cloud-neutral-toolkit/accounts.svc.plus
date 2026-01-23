#!/usr/bin/env bash
set -euo pipefail

CONFIG_FILE="${CONFIG_PATH:-/etc/xcontrol/account.yaml}"
CONFIG_TEMPLATE="${CONFIG_TEMPLATE:-/app/config/account.yaml}"
mkdir -p "$(dirname "${CONFIG_FILE}")"

if [ ! -f "${CONFIG_FILE}" ]; then
  if [ -f "${CONFIG_TEMPLATE}" ]; then
    envsubst < "${CONFIG_TEMPLATE}" > "${CONFIG_FILE}"
  else
    echo "missing config template: ${CONFIG_TEMPLATE}" >&2
    exit 1
  fi
fi

if [ -n "${PORT:-}" ]; then
  tmp_cfg=$(mktemp)
  awk -v port="$PORT" '
    /^server:/ {print; in_server=1; addr_written=0; next}
    in_server && /^  addr:/ {print "  addr: \":" port "\""; addr_written=1; next}
    in_server && /^ [^ ]/ {in_server=0}
    {print}
    END {
      if (port != "" && in_server == 0 && addr_written == 0) {
        print "server:";
        print "  addr: \":" port "\"";
      }
    }
  ' "${CONFIG_FILE}" > "${tmp_cfg}"
  CONFIG_FILE="${tmp_cfg}"
fi

if [ -n "${DB_HOST:-}" ] && [ -n "${DB_PORT:-}" ]; then
  if [ "${DB_HOST}" = "127.0.0.1" ] || [ "${DB_HOST}" = "localhost" ]; then
    if command -v nc >/dev/null; then
      wait_seconds="${STUNNEL_WAIT_SECONDS:-30}"
      i=0
      while ! nc -z "${DB_HOST}" "${DB_PORT}" >/dev/null 2>&1; do
        i=$((i + 1))
        if [ "${i}" -ge "${wait_seconds}" ]; then
          echo "stunnel not ready after ${wait_seconds}s on ${DB_HOST}:${DB_PORT}" >&2
          break
        fi
        sleep 1
      done
    fi
  fi
fi

exec /usr/local/bin/account --config "${CONFIG_FILE}" "$@"
