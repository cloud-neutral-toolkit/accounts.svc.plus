#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: $0 <repo-path> [gitleaks-config-path]" >&2
  exit 1
fi

repo_path=$1
config_path=${2:-}

if [[ ! -d "$repo_path/.git" ]]; then
  echo "Error: not a git repository: $repo_path" >&2
  exit 1
fi

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "Error: gitleaks is not installed or not in PATH." >&2
  exit 1
fi

config_args=()
if [[ -n "$config_path" ]]; then
  config_args=(--config "$config_path")
elif [[ -f "$repo_path/config/gitleaks.toml" ]]; then
  config_args=(--config "$repo_path/config/gitleaks.toml")
fi

(
  cd "$repo_path"
  gitleaks detect -v "${config_args[@]}"
)
