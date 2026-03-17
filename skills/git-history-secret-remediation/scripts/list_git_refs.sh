#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <repo-path>" >&2
  exit 1
fi

repo_path=$1

if [[ ! -d "$repo_path/.git" ]]; then
  echo "Error: not a git repository: $repo_path" >&2
  exit 1
fi

git -C "$repo_path" for-each-ref --format='%(refname)' refs/heads refs/tags refs/remotes/origin
