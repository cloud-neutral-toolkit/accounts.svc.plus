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

# Modify Config if needed
if [ -n "${PORT:-}" ]; then
  # Use sed for simpler replacement if format allows, but awk is robust
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

# If DB_HOST is set (either by user or Stunnel logic), reconstruct DSN
if [ -n "${DB_HOST:-}" ]; then
    DB_USER="${DB_USER:-$DEFAULT_DB_USER}"
    DB_PASS="${DB_PASSWORD:-$DEFAULT_DB_PASS}"
    DB_PORT="${DB_PORT:-$DEFAULT_DB_PORT}"
    DB_NAME="${DB_NAME:-$DEFAULT_DB_NAME}"
    DB_SSLMODE="${DB_SSLMODE:-$DEFAULT_SSLMODE}"

    # Construct DSN
    # Format: postgres://user:password@host:port/dbname?sslmode=disable
    DSN="postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"

    echo "Updating database configuration (DSN)..."
    # Escape pipe characters in DSN to avoid sed issues
    ESCAPED_DSN=$(printf '%s\n' "$DSN" | sed 's/|/\\|/g')
    sed -i "s|dsn: \".*\"|dsn: \"${ESCAPED_DSN}\"|g" "${CONFIG_FILE}"

elif [ -n "${DB_PASSWORD:-}" ]; then
    # Legacy fallback: User only provided password, assuming default host/user
    echo "Updating database password..."
    ESCAPED_PASS=$(printf '%s\n' "$DB_PASSWORD" | sed 's/|/\\|/g')
    sed -i "s|:password@|:${ESCAPED_PASS}@|g" "${CONFIG_FILE}"
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
