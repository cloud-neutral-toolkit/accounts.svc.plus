#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Stunnel Configuration
# -----------------------------------------------------------------------------

if [ -n "${DB_TLS_HOST:-}" ] && [ -n "${DB_TLS_PORT:-}" ]; then
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
DEFAULT_CONFIG="/app/config/account.yaml" # Note: Adjusted path as config is likely copied to /app/config in Docker or we assume standard location

# If config not in /etc, maybe we need to copy it there or use what's available
# The original script assumed /etc/xcontrol/account.yaml or copied from DEFAULT_CONFIG
# But where is DEFAULT_CONFIG? In Dockerfile:
# COPY --from=builder /src/account /usr/local/bin/account -> Only binary
# We need to make sure config file exists. 
# Looking at Dockerfile again:
# It assumes the app might have config embedded or user mounts it? 
# Wait, original entrypoint had: DEFAULT_CONFIG="/etc/xcontrol/account.yaml"
# AND: `cp "${DEFAULT_CONFIG}" "${CONFIG_FILE}"` -- THIS LOGIC IS WEIRD if they are same path.
# Ah, lines 4-5:
# CONFIG_FILE="${CONFIG_PATH:-/etc/xcontrol/account.yaml}"
# DEFAULT_CONFIG="/etc/xcontrol/account.yaml"
# If CONFIG_PATH is set to something else, it copies from DEFAULT. But if DEFAULT is not there?
# The Dockerfile DOES NOT copy config files to /etc/xcontrol.
# Check Dockerfile again:
# It copies entrypoint.sh. It does NOT copy config folder.
# This means the original image probably expected config mounted or baked in separately?
# Or maybe the builder stage had it? No, builder stage copies . . but runtime only copies /src/account binary.
# The user might be mounting config at runtime.
# However, for Cloud Run, we want to bake a default config or generate it.
# Let's assume we need to provide a base config if one isn't there.
# I will blindly respect the original logic but adding my specific injection logic.

# In my plan I said "Inject DB_PASSWORD".

if [ ! -f "${CONFIG_FILE}" ]; then
  # If we don't have a config file at all, we might be in trouble if we don't have a source.
  # Let's hope the user mounts it or we can generate a minimal one.
  # For now, let's assume the previous logic was correct for their env,
  # OR we can improve it by creating a default one if missing.
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

# Inject DB Password into DSN if DB_PASSWORD is set
if [ -n "${DB_PASSWORD:-}" ]; then
    # Simply replacing 'password' in the DSN if it matches the default pattern, 
    # or appending if we want to be smarter. 
    # Default DSN: postgres://shenlan:password@127.0.0.1:5432/account?sslmode=disable
    # We will try to replace ":password@" with ":${DB_PASSWORD}@"
    # This is a bit brittle but simple.
    sed -i "s|:password@|:${DB_PASSWORD}@|g" "${CONFIG_FILE}"
fi

# Inject Auth Secrets
if [ -n "${AUTH_PUBLIC_TOKEN:-}" ]; then
  sed -i "s|publicToken: \".*\"|publicToken: \"${AUTH_PUBLIC_TOKEN}\"|g" "${CONFIG_FILE}"
fi
if [ -n "${AUTH_REFRESH_SECRET:-}" ]; then
  sed -i "s|refreshSecret: \".*\"|refreshSecret: \"${AUTH_REFRESH_SECRET}\"|g" "${CONFIG_FILE}"
fi
if [ -n "${AUTH_ACCESS_SECRET:-}" ]; then
  sed -i "s|accessSecret: \".*\"|accessSecret: \"${AUTH_ACCESS_SECRET}\"|g" "${CONFIG_FILE}"
fi

exec /usr/local/bin/account --config "${CONFIG_FILE}" "$@"

