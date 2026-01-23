#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Stunnel Configuration
# -----------------------------------------------------------------------------

USE_STUNNEL=0

if [ -n "${DB_TLS_HOST:-}" ] && [ -n "${DB_TLS_PORT:-}" ]; then
  USE_STUNNEL=1
  echo "Configuring Stunnel..."

  STUNNEL_CONF="/etc/stunnel/stunnel.conf"
  mkdir -p /etc/stunnel

  # Write certificate files from env vars if present
  if [ -n "${DB_CA:-}" ]; then
    echo "${DB_CA}" > /etc/stunnel/ca.pem
  fi
  if [ -n "${DB_CERT:-}" ]; then
    echo "${DB_CERT}" > /etc/stunnel/cert.pem
  fi
  if [ -n "${DB_KEY:-}" ]; then
    echo "${DB_KEY}" > /etc/stunnel/key.pem
  fi

  # Generate stunnel.conf
  cat > "${STUNNEL_CONF}" <<EOF
foreground = no
pid = /var/run/stunnel/stunnel.pid
socket = l:TCP_NODELAY=1
socket = r:TCP_NODELAY=1
debug = 5
output = /var/log/stunnel.log

[postgres]
client = yes
accept = 127.0.0.1:5432
connect = ${DB_TLS_HOST}:${DB_TLS_PORT}
verify = 2
EOF

  if [ -f "/etc/stunnel/ca.pem" ]; then
    echo "CAfile = /etc/stunnel/ca.pem" >> "${STUNNEL_CONF}"
  elif [ -f "/etc/ssl/certs/ca-certificates.crt" ]; then
    echo "CAfile = /etc/ssl/certs/ca-certificates.crt" >> "${STUNNEL_CONF}"
  fi
  if [ -f "/etc/stunnel/cert.pem" ] && [ -f "/etc/stunnel/key.pem" ]; then
    echo "cert = /etc/stunnel/cert.pem" >> "${STUNNEL_CONF}"
    echo "key = /etc/stunnel/key.pem" >> "${STUNNEL_CONF}"
  fi

  echo "Starting Stunnel..."
  stunnel "${STUNNEL_CONF}"
  
  # Wait for stunnel to be up (simple check)
  STUNNEL_UP=0
  for i in {1..30}; do
    if nc -z 127.0.0.1 5432; then
      echo "Stunnel is up!"
      STUNNEL_UP=1
      break
    fi
    echo "Waiting for Stunnel... ($i/30)"
    sleep 1
  done

  if [ "${STUNNEL_UP}" -eq 0 ]; then
    echo "Error: Stunnel failed to start or is not reachable on port 5432."
    echo "Stunnel logs:"
    if [ -f "/var/log/stunnel.log" ]; then
      cat /var/log/stunnel.log
    else
      echo "No stunnel log found."
    fi
    exit 1
  fi
fi

# -----------------------------------------------------------------------------
# App Configuration
# -----------------------------------------------------------------------------

CONFIG_FILE="${CONFIG_PATH:-/etc/xcontrol/account.yaml}"

# Copy default config if missing
if [ ! -f "${CONFIG_FILE}" ]; then
  if [ -f "/app/config/account.yaml" ]; then
     mkdir -p "$(dirname "${CONFIG_FILE}")"
     cp "/app/config/account.yaml" "${CONFIG_FILE}"
  elif [ -f "/usr/local/bin/account.yaml" ]; then
      mkdir -p "$(dirname "${CONFIG_FILE}")"
      cp "/usr/local/bin/account.yaml" "${CONFIG_FILE}"
  else 
      echo "Warning: No configuration file found to copy from."
  fi
fi

# Modify server port if Cloud Run provides PORT
if [ -n "${PORT:-}" ]; then
  tmp_cfg=$(mktemp)
  awk -v port="$PORT" '
    /^server:/ {print; in_server=1; addr_written=0; next}
    in_server && /^  addr:/ {print "  addr: \"0.0.0.0:" port "\""; addr_written=1; next}
    in_server && /^ [^ ]/ {in_server=0}
    {print}
    END {
      if (port != "" && in_server == 0 && addr_written == 0) {
        print "server:";
        print "  addr: \"0.0.0.0:" port "\"";
      }
    }
  ' "${CONFIG_FILE}" > "${tmp_cfg}"
  mv "${tmp_cfg}" "${CONFIG_FILE}"
fi

# Database Configuration
DEFAULT_DB_USER="shenlan"
DEFAULT_DB_PASS="password"
DEFAULT_DB_HOST="127.0.0.1"
DEFAULT_DB_PORT="5432"
DEFAULT_DB_NAME="account"
DEFAULT_SSLMODE="disable"

# If Stunnel is active, force connection to localhost:5432
if [ "$USE_STUNNEL" -eq 1 ]; then
    DB_HOST="127.0.0.1"
    DB_PORT="5432"
fi

# Override store driver/DSN from environment and fallback to memory when absent
store_driver="${STORE_DRIVER:-}"
store_dsn="${DB_DSN:-${DATABASE_URL:-}}"

# If DB_HOST is set (either by user or Stunnel logic), construct DSN
if [ -n "${DB_HOST:-}" ]; then
    DB_USER="${DB_USER:-$DEFAULT_DB_USER}"
    DB_PASS="${DB_PASSWORD:-$DEFAULT_DB_PASS}"
    DB_PORT="${DB_PORT:-$DEFAULT_DB_PORT}"
    DB_NAME="${DB_NAME:-$DEFAULT_DB_NAME}"
    DB_SSLMODE="${DB_SSLMODE:-$DEFAULT_SSLMODE}"

    # Construct DSN
    # Format: postgres://user:password@host:port/dbname?sslmode=disable
    DSN="postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"
    store_dsn="$DSN"
    if [ -z "${store_driver}" ]; then
        store_driver="postgres"
    fi
elif [ -n "${store_dsn}" ]; then
    if [ -z "${store_driver}" ]; then
        store_driver="postgres"
    fi
elif [ -z "${store_driver}" ] && [ -z "${store_dsn}" ]; then
    store_driver="memory"
fi

# Apply store configuration to config file
if [ -n "${store_driver}" ] || [ -n "${store_dsn}" ]; then
  tmp_cfg=$(mktemp)
  awk -v driver="$store_driver" -v dsn="$store_dsn" '
    BEGIN {in_store=0; driver_written=0; dsn_written=0}
    /^store:/ {print; in_store=1; next}
    in_store && /^  driver:/ {
      if (driver != "") {print "  driver: \"" driver "\""; driver_written=1; next}
    }
    in_store && /^  dsn:/ {
      if (dsn != "") {print "  dsn: \"" dsn "\""; dsn_written=1; next}
    }
    in_store && /^[^ ]/ {
      if (driver != "" && driver_written == 0) {print "  driver: \"" driver "\""; driver_written=1}
      if (dsn != "" && dsn_written == 0) {print "  dsn: \"" dsn "\""; dsn_written=1}
      in_store=0
    }
    {print}
    END {
      if (in_store) {
        if (driver != "" && driver_written == 0) print "  driver: \"" driver "\"";
        if (dsn != "" && dsn_written == 0) print "  dsn: \"" dsn "\"";
      }
    }
  ' "${CONFIG_FILE}" > "${tmp_cfg}"
  mv "${tmp_cfg}" "${CONFIG_FILE}"
fi

# Inject DB Password into DSN if DB_PASSWORD is set
if [ -n "${DB_PASSWORD:-}" ]; then
    sed -i "s|:password@|:${DB_PASSWORD}@|g" "${CONFIG_FILE}"
fi

# Inject Auth Secrets
if [ -n "${AUTH_PUBLIC_TOKEN:-}" ]; then
  ESCAPED_TOKEN=$(printf '%s\n' "$AUTH_PUBLIC_TOKEN" | sed 's/|/\\|/g')
  sed -i "s|publicToken: \".*\"|publicToken: \"${ESCAPED_TOKEN}\"|g" "${CONFIG_FILE}"
fi
if [ -n "${AUTH_REFRESH_SECRET:-}" ]; then
  ESCAPED_SECRET=$(printf '%s\n' "$AUTH_REFRESH_SECRET" | sed 's/|/\\|/g')
  sed -i "s|refreshSecret: \".*\"|refreshSecret: \"${ESCAPED_SECRET}\"|g" "${CONFIG_FILE}"
fi
if [ -n "${AUTH_ACCESS_SECRET:-}" ]; then
  ESCAPED_SECRET=$(printf '%s\n' "$AUTH_ACCESS_SECRET" | sed 's/|/\\|/g')
  sed -i "s|accessSecret: \".*\"|accessSecret: \"${ESCAPED_SECRET}\"|g" "${CONFIG_FILE}"
fi

exec /usr/local/bin/account --config "${CONFIG_FILE}" "$@"
