#!/usr/bin/env python3
"""Back up git remote fetch/push URLs to JSON."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


def run(repo_path: str, *args: str) -> str:
    return subprocess.check_output(["git", "-C", repo_path, *args], text=True).strip()


def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: backup_git_remotes.py <repo-path> <output-json>", file=sys.stderr)
        return 1

    repo_path, output_json = sys.argv[1], sys.argv[2]
    remotes = run(repo_path, "remote").splitlines()
    payload: dict[str, dict[str, list[str]]] = {}

    for remote in remotes:
        remote = remote.strip()
        if not remote:
            continue
        fetch_urls = run(repo_path, "remote", "get-url", "--all", remote).splitlines()
        try:
            push_urls = run(repo_path, "remote", "get-url", "--push", "--all", remote).splitlines()
        except subprocess.CalledProcessError:
            push_urls = fetch_urls
        payload[remote] = {
            "fetch": [url for url in fetch_urls if url],
            "push": [url for url in push_urls if url],
        }

    output_path = Path(output_json)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
