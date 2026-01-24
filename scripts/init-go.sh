#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

if [ ! -f go.mod ]; then
  echo ">>> go.mod not found, initializing module"
  go mod init account
fi

go mod tidy

echo ">>> 检查 Go 环境"
if ! command -v go >/dev/null; then
  echo "未安装 Go，自动安装中..."
  if [ "${OS:-}" = "Darwin" ]; then
    brew install go@1.24
    brew link --overwrite --force go@1.24
  else
    sudo apt-get update
    sudo apt-get install -y golang
  fi
fi

echo ">>> 配置 Go Proxy"
if curl -fsSL --max-time 5 https://goproxy.cn >/dev/null; then
  go env -w GOPROXY=https://goproxy.cn,direct
else
  go env -w GOPROXY=https://proxy.golang.org,direct
fi

go mod tidy
