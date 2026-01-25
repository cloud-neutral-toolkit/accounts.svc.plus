#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

pkill -f "${APP_NAME}" || echo "⚠️ 未找到运行进程"
