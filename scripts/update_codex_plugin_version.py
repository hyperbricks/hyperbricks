#!/usr/bin/env python3
"""Advance the Codex plugin patch version after its packaged skill changes."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import sys


DEFAULT_MANIFEST = Path("codex-plugin/hyperbricks/.codex-plugin/plugin.json")
SEMVER = re.compile(
    r"^(?P<major>0|[1-9]\d*)\."
    r"(?P<minor>0|[1-9]\d*)\."
    r"(?P<patch>0|[1-9]\d*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)


class VersionError(RuntimeError):
    """Raised when the plugin version cannot be updated safely."""


def skill_tree(root: Path) -> dict[str, bytes]:
    if not root.is_dir():
        raise VersionError(f"skill tree is not a directory: {root}")
    files: dict[str, bytes] = {}
    try:
        for path in sorted(root.rglob("*")):
            if path.is_symlink():
                raise VersionError(f"skill tree contains a symlink: {path}")
            if path.is_file():
                files[path.relative_to(root).as_posix()] = path.read_bytes()
    except OSError as error:
        raise VersionError(f"cannot read skill tree {root}: {error}") from error
    return files


def skill_changed(before: Path, after: Path) -> bool:
    return skill_tree(before) != skill_tree(after)


def refreshed_version(current: str) -> str:
    match = SEMVER.fullmatch(current)
    if not match:
        raise VersionError(f"invalid semantic version: {current!r}")
    return (
        f"{match.group('major')}.{match.group('minor')}."
        f"{int(match.group('patch')) + 1}"
    )


def update_manifest(path: Path) -> tuple[str, str]:
    try:
        manifest = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise VersionError(f"cannot read {path}: {error}") from error
    if not isinstance(manifest, dict):
        raise VersionError(f"{path} must contain a JSON object")

    current = manifest.get("version")
    if not isinstance(current, str):
        raise VersionError(f"{path} must contain a string version")
    updated = refreshed_version(current)
    manifest["version"] = updated

    try:
        path.write_text(
            json.dumps(manifest, indent=2, ensure_ascii=False) + "\n",
            encoding="utf-8",
        )
    except OSError as error:
        raise VersionError(f"cannot write {path}: {error}") from error
    return current, updated


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Increment the Codex plugin patch version after a packaged skill "
            "change and emit a plain MAJOR.MINOR.PATCH version."
        )
    )
    parser.add_argument(
        "--manifest",
        type=Path,
        default=DEFAULT_MANIFEST,
        help=f"plugin manifest to update (default: {DEFAULT_MANIFEST})",
    )
    parser.add_argument(
        "--before-skill",
        type=Path,
        help="packaged skill snapshot from before synchronization",
    )
    parser.add_argument(
        "--after-skill",
        type=Path,
        help="packaged skill tree after synchronization",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if (args.before_skill is None) != (args.after_skill is None):
            raise VersionError(
                "--before-skill and --after-skill must be provided together"
            )
        if args.before_skill is not None and not skill_changed(
            args.before_skill, args.after_skill
        ):
            print("Codex plugin skill unchanged; version preserved.")
            return 0
        previous, updated = update_manifest(args.manifest)
    except VersionError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"Codex plugin version: {previous} -> {updated}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
