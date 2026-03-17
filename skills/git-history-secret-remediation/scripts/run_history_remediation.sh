#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <repo-path> <replace-text-file> [path-to-remove...]" >&2
  exit 1
fi

repo_path=$1
replace_text_file=$2
shift 2

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "[1/4] Inventory refs"
bash "$script_dir/list_git_refs.sh" "$repo_path"

echo "[2/4] Pre-scan"
if ! bash "$script_dir/run_gitleaks_history_scan.sh" "$repo_path"; then
  echo "Pre-scan found leaks. Continuing to remediation..." >&2
fi

echo "[3/4] Rewrite history"
bash "$script_dir/run_filter_repo_redaction.sh" "$repo_path" "$replace_text_file" "$@"

echo "[4/4] Post-scan"
bash "$script_dir/run_gitleaks_history_scan.sh" "$repo_path"
