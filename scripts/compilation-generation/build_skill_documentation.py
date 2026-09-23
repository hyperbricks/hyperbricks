#!/usr/bin/env python3
"""Build the versioned documentation snapshot bundled with the HyperBricks skill."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from datetime import date
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import posixpath
import re
import sys

import build_markdown_compilations as assembly


INDEX_FILENAME = "DOCUMENTATION_INDEX.md"
MANIFEST_FILENAME = "documentation-manifest.json"
DOCUMENTS_DIRECTORY = "docs"
GENERATOR_PATH = "scripts/compilation-generation/build_skill_documentation.py"
TRUSTED_OUTPUT_DIRECTORIES = (
    "SKILLS/hyperbricks/references",
    "codex-plugin/hyperbricks/skills/hyperbricks/references",
)


@dataclass(frozen=True)
class BundledDocument:
    source: assembly.SourceDocument
    output_path: PurePosixPath
    content: str


def snapshot_repository_paths(
    repository: Path,
    commit: str,
) -> tuple[set[str], set[str]]:
    listing = assembly.run_git(
        repository,
        "ls-tree",
        "-r",
        "--name-only",
        commit,
    )
    files = {line for line in listing.splitlines() if line}
    directories: set[str] = set()
    for filename in files:
        path = PurePosixPath(filename)
        for parent in path.parents:
            if parent == PurePosixPath("."):
                break
            directories.add(parent.as_posix())
    return files, directories


def bundled_document_path(source_path: str) -> PurePosixPath:
    path = PurePosixPath(source_path)
    try:
        relative = path.relative_to("docs")
    except ValueError as error:
        raise assembly.BuildError(
            f"skill documentation source is outside docs/: {source_path}"
        ) from error
    if relative == PurePosixPath(".") or ".." in relative.parts:
        raise assembly.BuildError(f"invalid skill documentation path: {source_path}")
    return PurePosixPath(DOCUMENTS_DIRECTORY) / relative


def repository_target_url(
    resolved_path: str,
    commit: str,
    repository_ref: str,
    image: bool,
    files: set[str],
    directories: set[str],
) -> str:
    if resolved_path not in files and resolved_path not in directories:
        raise assembly.BuildError(
            f"documentation link target is missing from snapshot {commit[:7]}: "
            f"{resolved_path}"
        )
    if image:
        if resolved_path not in files:
            raise assembly.BuildError(
                f"documentation image target is not a file: {resolved_path}"
            )
        base = "https://raw.githubusercontent.com/hyperbricks/hyperbricks"
    else:
        kind = "tree" if resolved_path in directories else "blob"
        base = f"{assembly.REPOSITORY_URL}/{kind}"
    return f"{base}/{repository_ref}/{resolved_path}"


def rewrite_document_link(
    match: re.Match[str],
    source: assembly.SourceDocument,
    output_paths: dict[str, PurePosixPath],
    commit: str,
    repository_ref: str,
    files: set[str],
    directories: set[str],
) -> str:
    image_marker = match.group("image") or ""
    label = match.group("label")
    raw_destination = match.group("destination")
    title = match.group("title") or ""
    angle_wrapped = raw_destination.startswith("<") and raw_destination.endswith(">")
    destination = raw_destination[1:-1] if angle_wrapped else raw_destination

    if re.match(r"^[a-z][a-z0-9+.-]*:", destination, flags=re.IGNORECASE):
        return match.group(0)
    if destination.startswith("/") or destination.startswith("#"):
        return match.group(0)

    target_path, fragment = assembly.split_destination(destination)
    if not target_path:
        return match.group(0)

    resolved_path = assembly.normalize_repository_path(source.path, target_path)
    if resolved_path in output_paths:
        current_output = output_paths[source.path]
        target_output = output_paths[resolved_path]
        rewritten = posixpath.relpath(
            target_output.as_posix(),
            current_output.parent.as_posix(),
        )
    else:
        rewritten = repository_target_url(
            resolved_path,
            commit,
            repository_ref,
            bool(image_marker),
            files,
            directories,
        )

    if fragment:
        rewritten += f"#{fragment}"
    if angle_wrapped:
        rewritten = f"<{rewritten}>"
    return f"{image_marker}[{label}]({rewritten}{title})"


def render_document(
    source: assembly.SourceDocument,
    output_paths: dict[str, PurePosixPath],
    commit: str,
    files: set[str],
    directories: set[str],
    repository_ref: str = "",
) -> str:
    repository_ref = repository_ref or commit
    notice = (
        f"<!-- Generated from {source.path} at Git commit {commit}. "
        f"Do not edit directly. -->"
    )
    output = [notice, ""]
    fence_character = ""
    fence_length = 0

    for line in source.text.splitlines():
        fence = assembly.FENCE_RE.match(line)
        if fence:
            marker = fence.group("fence")
            if not fence_character:
                fence_character = marker[0]
                fence_length = len(marker)
            elif marker[0] == fence_character and len(marker) >= fence_length:
                fence_character = ""
                fence_length = 0
            output.append(line)
            continue

        if fence_character:
            output.append(line)
            continue

        output.append(
            assembly.LINK_RE.sub(
                lambda match: rewrite_document_link(
                    match,
                    source,
                    output_paths,
                    commit,
                    repository_ref,
                    files,
                    directories,
                ),
                line,
            )
        )

    return "\n".join(output).rstrip() + "\n"


def render_index(
    documents: tuple[BundledDocument, ...],
    version: str,
    commit: str,
    commit_date: date,
) -> str:
    repository_ref = assembly.repository_link_ref(version)
    source_url = f"{assembly.REPOSITORY_URL}/tree/{repository_ref}"
    lines = [
        f"<!-- Generated by {GENERATOR_PATH}. Do not edit directly. -->",
        "",
        "# HyperBricks Documentation Snapshot",
        "",
        "This is a versioned, generated mirror of the canonical documents in "
        "`docs/`. The HyperBricks skill uses the separate files below for "
        "targeted reading. Product behavior remains owned by the source documents.",
        "",
        f"- **HyperBricks version:** {version}",
        f"- **Source version:** [`{repository_ref}`]({source_url})",
        f"- **Snapshot date:** {commit_date.isoformat()}",
        f"- **Included documents:** {len(documents)}",
        "",
        "## Contents",
        "",
    ]

    current_section = ""
    for document in documents:
        source = document.source
        if source.section and source.section != current_section:
            if current_section:
                lines.append("")
            lines.extend((f"### {source.section}", ""))
            current_section = source.section
        label = source.display_title or source.title
        lines.append(
            f"- [{label}]({document.output_path.as_posix()}) — `{source.path}`"
        )

    return "\n".join(lines).rstrip() + "\n"


def sha256_text(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def render_manifest(
    documents: tuple[BundledDocument, ...],
    version: str,
    commit: str,
    commit_date: date,
) -> str:
    manifest = {
        "schema_version": 1,
        "hyperbricks_version": version,
        "source_repository": assembly.REPOSITORY_URL,
        "source_commit": commit,
        "source_date": commit_date.isoformat(),
        "documents": [
            {
                "source_path": document.source.path,
                "bundled_path": document.output_path.as_posix(),
                "source_sha256": sha256_text(document.source.text),
                "bundled_sha256": sha256_text(document.content),
            }
            for document in documents
        ],
    }
    return json.dumps(manifest, indent=2, ensure_ascii=False) + "\n"


def read_managed_manifest(output_directory: Path) -> dict | None:
    path = output_directory / MANIFEST_FILENAME
    if not path.is_file():
        return None
    try:
        manifest = json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError, UnicodeError):
        return None
    if not isinstance(manifest, dict):
        return None
    if manifest.get("schema_version") != 1:
        return None
    if manifest.get("source_repository") != assembly.REPOSITORY_URL:
        return None
    if not isinstance(manifest.get("documents"), list):
        return None
    return manifest


def ensure_safe_output_directory(repository: Path, output_directory: Path) -> None:
    resolved_repository = repository.resolve()
    lexical_output = Path(os.path.abspath(output_directory))
    resolved_output = output_directory.resolve()
    canonical_docs = (resolved_repository / "docs").resolve()
    trusted = {
        resolved_repository / relative
        for relative in TRUSTED_OUTPUT_DIRECTORIES
    }

    if output_directory.is_symlink() or (
        lexical_output in trusted and lexical_output != resolved_output
    ):
        raise assembly.BuildError(
            "refusing to use a symlinked skill documentation output path"
        )
    if resolved_output == resolved_repository or canonical_docs in (
        resolved_output,
        *resolved_output.parents,
    ):
        raise assembly.BuildError(
            "refusing to use the repository root or canonical docs tree as the "
            "skill documentation output directory"
        )

    generated_paths = (
        output_directory / INDEX_FILENAME,
        output_directory / MANIFEST_FILENAME,
        output_directory / DOCUMENTS_DIRECTORY,
    )
    for path in generated_paths:
        if path.is_symlink():
            raise assembly.BuildError(
                "refusing to use symlinks in a skill documentation snapshot"
            )
        resolved_path = path.resolve()
        if resolved_output not in (resolved_path, *resolved_path.parents):
            raise assembly.BuildError(
                "refusing to write skill documentation outside its output directory"
            )
    documents_directory = output_directory / DOCUMENTS_DIRECTORY
    if documents_directory.is_dir():
        for path in documents_directory.rglob("*"):
            if path.is_symlink():
                raise assembly.BuildError(
                    "refusing to use symlinks in a skill documentation snapshot"
                )
            resolved_path = path.resolve()
            if resolved_output not in (resolved_path, *resolved_path.parents):
                raise assembly.BuildError(
                    "refusing to write skill documentation outside its output directory"
                )

    if lexical_output in trusted or not output_directory.exists():
        return
    if not any(output_directory.iterdir()):
        return
    if read_managed_manifest(output_directory) is None:
        raise assembly.BuildError(
            "refusing to write into a non-empty directory that is not a managed "
            "HyperBricks skill documentation snapshot"
        )


def manifest_source_commit(output_directory: Path) -> str:
    manifest = read_managed_manifest(output_directory)
    commit = manifest.get("source_commit") if manifest else None
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise assembly.BuildError(
            f"{MANIFEST_FILENAME} does not contain a valid source_commit; "
            "pass --ref explicitly"
        )
    return commit


def build_skill_documentation(
    repository: Path,
    commit: str,
    commit_date: date,
) -> dict[PurePosixPath, str]:
    version = assembly.snapshot_version(repository, commit)
    repository_ref = assembly.repository_link_ref(version)
    sources = assembly.collect_sources(
        repository,
        commit,
        assembly.documentation_paths(repository, commit),
        "documentation",
        navigation=assembly.DOCUMENTATION_NAVIGATION,
        default_section=assembly.ADDITIONAL_DOCUMENTS_SECTION,
    )
    if not sources:
        raise assembly.BuildError("no Markdown documentation files found under docs")

    files, directories = snapshot_repository_paths(repository, commit)
    output_paths = {
        source.path: bundled_document_path(source.path) for source in sources
    }
    documents = tuple(
        BundledDocument(
            source=source,
            output_path=output_paths[source.path],
            content=render_document(
                source,
                output_paths,
                commit,
                files,
                directories,
                repository_ref,
            ),
        )
        for source in sources
    )

    outputs = {
        document.output_path: document.content for document in documents
    }
    outputs[PurePosixPath(INDEX_FILENAME)] = render_index(
        documents,
        version,
        commit,
        commit_date,
    )
    outputs[PurePosixPath(MANIFEST_FILENAME)] = render_manifest(
        documents,
        version,
        commit,
        commit_date,
    )
    return outputs


def existing_generated_paths(output_directory: Path) -> set[PurePosixPath]:
    paths: set[PurePosixPath] = set()
    for filename in (INDEX_FILENAME, MANIFEST_FILENAME):
        if (output_directory / filename).is_file():
            paths.add(PurePosixPath(filename))
    documents_directory = output_directory / DOCUMENTS_DIRECTORY
    if documents_directory.is_dir():
        for path in documents_directory.rglob("*"):
            if path.is_file():
                paths.add(PurePosixPath(path.relative_to(output_directory).as_posix()))
    return paths


def remove_stale_outputs(
    output_directory: Path,
    expected: set[PurePosixPath],
) -> None:
    for relative in sorted(
        existing_generated_paths(output_directory) - expected,
        key=lambda path: (len(path.parts), path.as_posix()),
        reverse=True,
    ):
        (output_directory / Path(relative.as_posix())).unlink()

    documents_directory = output_directory / DOCUMENTS_DIRECTORY
    if documents_directory.is_dir():
        for directory in sorted(
            (path for path in documents_directory.rglob("*") if path.is_dir()),
            key=lambda path: len(path.parts),
            reverse=True,
        ):
            try:
                directory.rmdir()
            except OSError:
                pass


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--ref",
        help=(
            "Git commit, tag, or branch to build. Defaults to HEAD when writing "
            "and to the manifest's source_commit with --check."
        ),
    )
    parser.add_argument(
        "--output-dir",
        default="SKILLS/hyperbricks/references",
        help=(
            "Skill references directory, relative to the repository root unless "
            "absolute."
        ),
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Verify the generated snapshot without writing it.",
    )
    return parser.parse_args()


def main() -> int:
    arguments = parse_arguments()
    try:
        repository = assembly.repository_root()
        output_directory = Path(arguments.output_dir)
        if not output_directory.is_absolute():
            output_directory = repository / output_directory
        ensure_safe_output_directory(repository, output_directory)

        source_ref = arguments.ref
        if source_ref is None:
            source_ref = (
                manifest_source_commit(output_directory)
                if arguments.check
                else "HEAD"
            )
        commit = assembly.resolve_commit(repository, source_ref)
        commit_date = date.fromisoformat(
            assembly.run_git(
                repository,
                "show",
                "-s",
                "--format=%cs",
                commit,
            ).strip()
        )
        outputs = build_skill_documentation(repository, commit, commit_date)
        expected = set(outputs)
        stale: list[Path] = []
        for relative, content in outputs.items():
            output = output_directory / Path(relative.as_posix())
            if arguments.check:
                if not output.is_file() or output.read_text(encoding="utf-8") != content:
                    stale.append(output)
                continue
            assembly.write_atomic(output, content)

        unexpected = existing_generated_paths(output_directory) - expected
        if arguments.check:
            stale.extend(
                output_directory / Path(relative.as_posix())
                for relative in sorted(unexpected)
            )
        else:
            remove_stale_outputs(output_directory, expected)

        if stale:
            for path in stale:
                print(
                    f"Out of date: {assembly.display_path(path, repository)}",
                    file=sys.stderr,
                )
            return 1
        if arguments.check:
            print(f"Skill documentation snapshot matches {commit[:7]}")
        else:
            print(
                f"Wrote {len(outputs) - 2} skill documentation files and provenance "
                f"for {commit[:7]}"
            )
        return 0
    except (assembly.BuildError, OSError, UnicodeError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
