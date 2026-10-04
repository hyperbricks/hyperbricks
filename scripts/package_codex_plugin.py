#!/usr/bin/env python3
"""Build and validate the upload archive for the HyperBricks Codex plugin."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys
import tempfile
from zipfile import BadZipFile, ZIP_DEFLATED, ZipFile, ZipInfo


PLUGIN_PATH = PurePosixPath("codex-plugin/hyperbricks")
ARCHIVE_PREFIX = PurePosixPath("hyperbricks")
DEFAULT_OUTPUT = Path("codex-plugin/hyperbricks.zip")
MANIFEST_PATH = ARCHIVE_PREFIX / ".codex-plugin/plugin.json"
MAX_ARCHIVE_BYTES = 100 * 1024 * 1024
MAX_UNCOMPRESSED_BYTES = 512 * 1024 * 1024
MAX_ENTRIES = 5_000
MAX_PATH_SEGMENTS = 20
MAX_PUBLIC_SHORT_DESCRIPTION_LENGTH = 30
CODEX_UPLOAD_VERSION = re.compile(
    r"^(0|[1-9]\d*)\."
    r"(0|[1-9]\d*)\."
    r"(0|[1-9]\d*)$"
)


class PackageError(RuntimeError):
    """Raised when the plugin cannot be packaged safely."""


def run_git(repository: Path, *arguments: str) -> str:
    result = subprocess.run(
        ("git", "-C", str(repository), *arguments),
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip()
        raise PackageError(detail or f"git {' '.join(arguments)} failed")
    return result.stdout


def run_git_bytes(repository: Path, *arguments: str) -> bytes:
    result = subprocess.run(
        ("git", "-C", str(repository), *arguments),
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if result.returncode:
        detail = result.stderr.decode("utf-8", errors="replace").strip()
        raise PackageError(detail or f"git {' '.join(arguments)} failed")
    return result.stdout


def repository_root() -> Path:
    return Path(run_git(Path.cwd(), "rev-parse", "--show-toplevel").strip())


def ensure_committed_plugin(repository: Path) -> None:
    status = run_git(
        repository,
        "status",
        "--porcelain",
        "--untracked-files=all",
        "--",
        str(PLUGIN_PATH),
    ).strip()
    if status:
        raise PackageError(
            "The plugin tree has uncommitted changes. Commit them before packaging:\n"
            f"{status}"
        )


def is_macos_metadata(path: PurePosixPath) -> bool:
    return any(
        part == "__MACOSX" or part == ".DS_Store" or part.startswith("._")
        for part in path.parts
    )


def validate_member(path_text: str) -> PurePosixPath:
    if not path_text or "\\" in path_text:
        raise PackageError(f"Invalid archive path: {path_text!r}")
    path = PurePosixPath(path_text)
    if path.is_absolute() or ".." in path.parts or "" in path.parts:
        raise PackageError(f"Unsafe archive path: {path_text!r}")
    if len(path.parts) > MAX_PATH_SEGMENTS:
        raise PackageError(f"Archive path is too deep: {path_text}")
    if not path.parts or path.parts[0] != ARCHIVE_PREFIX.name:
        raise PackageError(f"Archive member is outside {ARCHIVE_PREFIX}/: {path_text}")
    if is_macos_metadata(path):
        raise PackageError(f"macOS metadata must not be packaged: {path_text}")
    return path


def validate_manifest(data: bytes) -> dict[str, object]:
    try:
        manifest = json.loads(data.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise PackageError(f"Invalid {MANIFEST_PATH}: {error}") from error
    if not isinstance(manifest, dict):
        raise PackageError(f"{MANIFEST_PATH} must contain a JSON object")
    if manifest.get("name") != ARCHIVE_PREFIX.name:
        raise PackageError(
            f"{MANIFEST_PATH} name must be {ARCHIVE_PREFIX.name!r}"
        )
    version = manifest.get("version")
    if not isinstance(version, str) or not CODEX_UPLOAD_VERSION.fullmatch(version):
        raise PackageError(
            f"{MANIFEST_PATH} version must be plain MAJOR.MINOR.PATCH "
            "for Codex app upload compatibility"
        )
    description = manifest.get("description")
    if not isinstance(description, str) or not description.strip():
        raise PackageError(f"{MANIFEST_PATH} description must not be empty")
    interface = manifest.get("interface")
    if not isinstance(interface, dict):
        raise PackageError(f"{MANIFEST_PATH} interface must be an object")
    short_description = interface.get("shortDescription")
    if not isinstance(short_description, str) or not short_description.strip():
        raise PackageError(
            f"{MANIFEST_PATH} interface.shortDescription must not be empty"
        )
    if "\n" in short_description or "\r" in short_description:
        raise PackageError(
            f"{MANIFEST_PATH} interface.shortDescription must fit on one line"
        )
    if len(short_description) > MAX_PUBLIC_SHORT_DESCRIPTION_LENGTH:
        raise PackageError(
            f"{MANIFEST_PATH} interface.shortDescription must be at most "
            f"{MAX_PUBLIC_SHORT_DESCRIPTION_LENGTH} characters for public submission"
        )
    return manifest


def validate_archive(archive_path: Path) -> tuple[int, str]:
    if archive_path.stat().st_size > MAX_ARCHIVE_BYTES:
        raise PackageError("Plugin archive exceeds the 100 MB upload limit")
    try:
        with ZipFile(archive_path) as archive:
            entries = archive.infolist()
            if not entries:
                raise PackageError("Plugin archive is empty")
            if len(entries) > MAX_ENTRIES:
                raise PackageError("Plugin archive contains more than 5,000 entries")

            names: set[str] = set()
            uncompressed_bytes = 0
            skill_paths: list[PurePosixPath] = []
            for entry in entries:
                path = validate_member(entry.filename)
                if entry.filename in names:
                    raise PackageError(f"Duplicate archive path: {entry.filename}")
                names.add(entry.filename)
                if entry.flag_bits & 0x1:
                    raise PackageError(f"Encrypted archive member: {entry.filename}")
                uncompressed_bytes += entry.file_size
                if uncompressed_bytes > MAX_UNCOMPRESSED_BYTES:
                    raise PackageError("Expanded plugin archive exceeds 512 MB")

                mode = (entry.external_attr >> 16) & 0xFFFF
                if mode and not entry.is_dir() and not stat.S_ISREG(mode):
                    raise PackageError(
                        f"Unsupported non-regular archive member: {entry.filename}"
                    )
                if (
                    len(path.parts) == 4
                    and path.parts[1] == "skills"
                    and path.name == "SKILL.md"
                ):
                    skill_paths.append(path)

            manifest_name = str(MANIFEST_PATH)
            if manifest_name not in names:
                raise PackageError(f"Archive is missing {manifest_name}")
            if not skill_paths:
                raise PackageError("Archive must contain at least one skills/*/SKILL.md")
            manifest = validate_manifest(archive.read(manifest_name))
            return len(entries), str(manifest["version"])
    except BadZipFile as error:
        raise PackageError(f"Invalid ZIP archive: {error}") from error


def committed_plugin_files(
    repository: Path, revision: str
) -> list[tuple[PurePosixPath, int, bytes]]:
    listing = run_git_bytes(
        repository,
        "ls-tree",
        "-r",
        "-z",
        revision,
        "--",
        str(PLUGIN_PATH),
    )
    files: list[tuple[PurePosixPath, int, bytes]] = []
    for record in listing.split(b"\0"):
        if not record:
            continue
        metadata, path_bytes = record.split(b"\t", 1)
        mode_text, object_type, object_id = metadata.split()
        if object_type != b"blob":
            raise PackageError(
                f"Unsupported Git object in plugin: {object_type.decode()}"
            )
        repository_path = PurePosixPath(path_bytes.decode("utf-8"))
        try:
            relative_path = repository_path.relative_to(PLUGIN_PATH)
        except ValueError as error:
            raise PackageError(
                f"Committed path is outside {PLUGIN_PATH}: {repository_path}"
            ) from error
        mode = int(mode_text, 8)
        if not stat.S_ISREG(mode):
            raise PackageError(f"Plugin contains a non-regular file: {repository_path}")
        data = run_git_bytes(repository, "cat-file", "blob", object_id.decode())
        files.append((relative_path, mode, data))
    if not files:
        raise PackageError(f"No committed plugin files found under {PLUGIN_PATH}")
    return sorted(files, key=lambda item: str(item[0]))


def write_archive(repository: Path, revision: str, destination: Path) -> None:
    commit_timestamp = int(
        run_git(repository, "show", "-s", "--format=%ct", revision).strip()
    )
    committed_at = datetime.fromtimestamp(commit_timestamp, tz=timezone.utc)
    zip_timestamp = (
        max(committed_at.year, 1980),
        committed_at.month,
        committed_at.day,
        committed_at.hour,
        committed_at.minute,
        committed_at.second - committed_at.second % 2,
    )

    with ZipFile(destination, "w", compression=ZIP_DEFLATED, compresslevel=9) as archive:
        for relative_path, mode, data in committed_plugin_files(repository, revision):
            archive_path = ARCHIVE_PREFIX / relative_path
            info = ZipInfo(str(archive_path), date_time=zip_timestamp)
            info.create_system = 3
            info.external_attr = mode << 16
            info.compress_type = ZIP_DEFLATED
            archive.writestr(info, data, compress_type=ZIP_DEFLATED, compresslevel=9)


def build_archive(repository: Path, output: Path) -> tuple[int, str, str]:
    ensure_committed_plugin(repository)
    output.parent.mkdir(parents=True, exist_ok=True)
    revision = run_git(repository, "rev-parse", "HEAD").strip()

    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{output.name}.", suffix=".tmp", dir=output.parent
    )
    os.close(descriptor)
    temporary = Path(temporary_name)
    try:
        write_archive(repository, revision, temporary)
        entry_count, version = validate_archive(temporary)
        os.replace(temporary, output)
        output.chmod(0o644)
    finally:
        temporary.unlink(missing_ok=True)
    return entry_count, version, revision


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Package the committed HyperBricks Codex plugin as a validated ZIP."
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=DEFAULT_OUTPUT,
        help=f"archive destination relative to the repository root (default: {DEFAULT_OUTPUT})",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        repository = repository_root()
        output = args.output
        if not output.is_absolute():
            output = repository / output
        entry_count, version, revision = build_archive(repository, output)
    except (OSError, PackageError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1

    print(f"Wrote {output} ({entry_count} entries)")
    print(f"Plugin version: {version}")
    print(f"Source commit: {revision}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
