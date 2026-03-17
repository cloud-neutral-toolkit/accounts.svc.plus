#!/usr/bin/env python3
"""Restore git remote fetch/push URLs from JSON."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path


def git(repo_path: str, *args: str) -> None:
    subprocess.check_call(["git", "-C", repo_path, *args])


def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: restore_git_remotes.py <repo-path> <input-json>", file=sys.stderr)
        return 1

    repo_path, input_json = sys.argv[1], sys.argv[2]
    data = json.loads(Path(input_json).read_text(encoding="utf-8"))

    for remote, urls in data.items():
        fetch_urls = urls.get("fetch") or []
        push_urls = urls.get("push") or []
        if not fetch_urls:
            continue

        existing = subprocess.run(
            ["git", "-C", repo_path, "remote", "get-url", remote],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        if existing.returncode != 0:
            git(repo_path, "remote", "add", remote, fetch_urls[0])
        else:
            git(repo_path, "remote", "set-url", remote, fetch_urls[0])

        for url in fetch_urls[1:]:
            git(repo_path, "remote", "set-url", "--add", remote, url)

        if push_urls:
            git(repo_path, "remote", "set-url", "--push", remote, push_urls[0])
            for url in push_urls[1:]:
                git(repo_path, "remote", "set-url", "--push", "--add", remote, url)

    print(input_json)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
