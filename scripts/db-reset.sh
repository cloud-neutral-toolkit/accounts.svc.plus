#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

echo "⚠️ 即将重置整个 PostgreSQL 数据库集群 ..."
read -r -p "确定要重置数据库集群? 这将删除所有数据! [y/N] " confirm
if [ "${confirm}" = "y" ] || [ "${confirm}" = "Y" ]; then
  echo ">>> 停止 PostgreSQL 服务 ..."
  sudo systemctl stop postgresql
  echo ">>> 删除数据库集群 16 main ..."
  sudo pg_dropcluster --stop 16 main
  echo ">>> 清理数据目录 ..."
  sudo rm -rf /var/lib/postgresql/16/main
  echo ">>> 清理配置目录 ..."
  sudo rm -rf /etc/postgresql/16/main
  echo ">>> 创建新的数据库集群 ..."
  sudo pg_createcluster 16 main --start
  echo "✓ PostgreSQL 集群重置完成"
else
  echo "取消重置"
fi
