#!/usr/bin/env bash
#
# ============================================
# 🧱 setup_postgres_local.sh
# 初始化本地 PostgreSQL 数据库 (开发环境)
# --------------------------------------------
# 支持 Ubuntu / macOS
# 创建用户 + 数据库 + 导入 schema
# ============================================
set -euo pipefail

# -----------------------------------------------------------------------------
# 配置参数（可通过环境变量覆盖）
# -----------------------------------------------------------------------------
DB_NAME="${DB_NAME:-account}"
DB_USER="${DB_USER:-shenlan}"
DB_PASS="${DB_PASS:-password}"
DB_PORT="${DB_PORT:-5432}"
DB_HOST="${DB_HOST:-127.0.0.1}"
SCHEMA_FILE="${SCHEMA_FILE:-./sql/schema.sql}"

PG_SERVICE_NAME="${PG_SERVICE_NAME:-postgresql}"
OS=$(uname -s)

# -----------------------------------------------------------------------------
# 通用函数
# -----------------------------------------------------------------------------
log() { echo -e "[$(date '+%H:%M:%S')] $*"; }

require_postgres() {
  if ! command -v psql >/dev/null; then
    log "⚠️ 未检测到 psql，开始安装 PostgreSQL..."
    if [[ "$OS" == "Darwin" ]]; then
      brew install postgresql@16 || brew install postgresql
    else
      sudo apt-get update -y
      sudo apt-get install -y postgresql postgresql-contrib
    fi
  else
    log "✅ psql 已存在"
  fi
}

start_postgres() {
  if [[ "$OS" == "Darwin" ]]; then
    brew services start postgresql@16 || brew services start postgresql || true
  else
    sudo systemctl enable --now "${PG_SERVICE_NAME}" || true
  fi
}

create_user_and_db() {
  log "🧩 检查并创建数据库用户 ${DB_USER}"
  if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1; then
    sudo -u postgres psql -c "CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASS}';"
  else
    log "✅ 用户 ${DB_USER} 已存在"
  fi

  log "🧩 检查并创建数据库 ${DB_NAME}"
  if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
    sudo -u postgres createdb -O "${DB_USER}" "${DB_NAME}"
  else
    log "✅ 数据库 ${DB_NAME} 已存在"
  fi

  log "🔑 授权 ${DB_USER} 对数据库 ${DB_NAME} 拥有全部权限"
  sudo -u postgres psql -d "${DB_NAME}" -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};" >/dev/null || true
}

import_schema() {
  if [[ -f "${SCHEMA_FILE}" ]]; then
    log "📦 导入 schema 文件: ${SCHEMA_FILE}"
    PGPASSWORD="${DB_PASS}" psql "postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable" \
      -v ON_ERROR_STOP=1 -f "${SCHEMA_FILE}"
    log "✅ schema 导入成功"
  else
    log "⚠️ 未找到 schema 文件: ${SCHEMA_FILE}，跳过导入"
  fi
}

# -----------------------------------------------------------------------------
# 主流程
# -----------------------------------------------------------------------------
log "🚀 开始初始化 PostgreSQL 本地数据库"
require_postgres
start_postgres
create_user_and_db
import_schema
log "🎉 PostgreSQL 本地数据库初始化完成"
