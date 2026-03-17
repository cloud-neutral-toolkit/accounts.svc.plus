#!/usr/bin/env python3
"""Package a skill folder into a distributable .skill archive."""

from __future__ import annotations

import sys
import zipfile
from pathlib import Path

from validate_skill import validate_skill


def should_include(file_path: Path) -> bool:
    if "__pycache__" in file_path.parts:
        return False
    if file_path.suffix == ".pyc":
        return False
    return True


def package_skill(skill_path: str | Path, output_dir: str | Path | None = None) -> Path:
    skill_dir = Path(skill_path).resolve()
    if not skill_dir.exists():
        raise FileNotFoundError(f"Skill folder not found: {skill_dir}")
    if not skill_dir.is_dir():
        raise NotADirectoryError(f"Path is not a directory: {skill_dir}")

    valid, message = validate_skill(skill_dir)
    if not valid:
        raise ValueError(message)

    destination = Path(output_dir).resolve() if output_dir else Path.cwd()
    destination.mkdir(parents=True, exist_ok=True)

    output_path = destination / f"{skill_dir.name}.skill"
    with zipfile.ZipFile(output_path, "w", zipfile.ZIP_DEFLATED) as archive:
        for file_path in skill_dir.rglob("*"):
            if file_path.is_file() and should_include(file_path):
                archive.write(file_path, file_path.relative_to(skill_dir.parent))

    return output_path


def main() -> int:
    if len(sys.argv) < 2 or len(sys.argv) > 3:
        print("Usage: package_skill.py <path/to/skill-folder> [output-directory]")
        return 1

    try:
        output_path = package_skill(sys.argv[1], sys.argv[2] if len(sys.argv) == 3 else None)
    except Exception as exc:  # pragma: no cover - command-line wrapper
        print(f"Error: {exc}")
        return 1

    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
