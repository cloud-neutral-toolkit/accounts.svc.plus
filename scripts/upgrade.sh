#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/_common.sh"

systemctl stop xcontrol-account
cp xcontrol-account /usr/bin/xcontrol-account
systemctl start xcontrol-account
