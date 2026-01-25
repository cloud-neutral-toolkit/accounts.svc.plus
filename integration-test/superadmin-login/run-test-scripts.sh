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

ensure_superadmin_password() {
  if [ -n "${SUPERADMIN_PASSWORD:-}" ] && [ "${SUPERADMIN_PASSWORD}" != "ChangeMe" ]; then
    return 0
  fi

  if command -v openssl >/dev/null; then
    SUPERADMIN_PASSWORD="$(openssl rand -base64 12 | tr -d '\n' | tr '/+' 'Aa' | cut -c1-10)"
  else
    SUPERADMIN_PASSWORD="$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 10)"
  fi

  if [ -f .env ]; then
    if grep -q '^SUPERADMIN_PASSWORD=' .env; then
      tmpfile="$(mktemp)"
      sed "s/^SUPERADMIN_PASSWORD=.*/SUPERADMIN_PASSWORD=${SUPERADMIN_PASSWORD}/" .env > "$tmpfile"
      mv "$tmpfile" .env
    else
      printf "\nSUPERADMIN_PASSWORD=%s\n" "$SUPERADMIN_PASSWORD" >> .env
    fi
  else
    printf "SUPERADMIN_PASSWORD=%s\n" "$SUPERADMIN_PASSWORD" > .env
  fi
  echo "✅ 已写入 .env 的 SUPERADMIN_PASSWORD"
}

if ! command -v make >/dev/null; then
  echo "❌ 未找到 make，请先安装或确保在 PATH 中" >&2
  exit 1
fi

echo ">>> init-db"
make init-db

echo ">>> create-db-user"
make create-db-user

echo ">>> create-super-admin"
ensure_superadmin_password
export SUPERADMIN_PASSWORD
create_output="$(make create-super-admin 2>&1 | tee /dev/stderr)"

superadmin_password=""
if echo "$create_output" | grep -q "已生成随机初始密码"; then
  superadmin_password="$(echo "$create_output" | sed -n 's/.*已生成随机初始密码: \([A-Za-z0-9]\+\).*/\1/p' | tail -n1)"
fi
if [ -z "$superadmin_password" ]; then
  superadmin_password="$(echo "$create_output" | sed -n 's/.*密码：\s*//p' | tail -n1)"
fi

cat <<EOM

✅ 集成测试步骤完成（DB 初始化 + 用户创建 + 超级管理员创建）

接下来手动登录验证：
- 网址：https://console.svc.plus/login
- 邮箱：admin@svc.plus
- 密码：${superadmin_password:-请查看上方 create-super-admin 输出}

提示：如果页面无法访问，请先确保 account 服务已启动并对外可用。
EOM
