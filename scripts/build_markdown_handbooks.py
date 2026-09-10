#!/usr/bin/env python3
"""Build the consolidated HyperBricks documentation and skills handbooks.

The source is read from a committed Git snapshot rather than the working tree.
This keeps the generated content and its reported revision in agreement.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from datetime import date
from pathlib import Path, PurePosixPath
import os
import posixpath
import re
import subprocess
import sys
import tempfile
from typing import Iterable


REPOSITORY_URL = "https://github.com/hyperbricks/hyperbricks"
MONTH_NAMES = (
    "January",
    "February",
    "March",
    "April",
    "May",
    "June",
    "July",
    "August",
    "September",
    "October",
    "November",
    "December",
)

DOCUMENTATION_ORDER = (
    "INTRODUCTION",
    "QUICKSTART",
    "PROJECT_PATTERNS",
    "HYPERBRICKS_CLI",
    "YAML_USAGE",
    "ROUTING",
    "HTMX_FRAGMENTS_AND_CANONICAL_URLS",
    "HTTP_RESPONSES",
    "ROUTE_GUARD",
    "API_RENDER",
    "GOJA_RENDER",
    "IMAGES",
    "ESBUILD",
    "PLUGINS",
    "LIVE_MODE_HTTP",
    "RUNTIME_GATEWAY",
    "DEPLOY",
    "DOCKER",
    "REFERENCE",
)

SKILLS_ORDER = (
    "SKILLS/hyperbricks/SKILL.md",
    "SKILLS/hyperbricks/references/project-lifecycle.md",
    "SKILLS/hyperbricks/references/authoring.md",
    "SKILLS/hyperbricks/references/integrations.md",
    "SKILLS/hyperbricks/references/plugins.md",
)

LINK_RE = re.compile(
    r"(?P<image>!)?\[(?P<label>[^\]]*)\]"
    r"\((?P<destination><[^>]+>|[^\s)]+)"
    r"(?P<title>\s+(?:\"[^\"]*\"|'[^']*'|\([^)]*\)))?\)"
)
HEADING_RE = re.compile(r"^(?P<indent>\s*)(?P<marks>#{1,6})\s+(?P<title>.+?)\s*$")
FENCE_RE = re.compile(r"^\s*(?P<fence>`{3,}|~{3,})")


@dataclass(frozen=True)
class SourceDocument:
    path: str
    title: str
    text: str
    anchor: str


@dataclass(frozen=True)
class Handbook:
    filename: str
    title: str
    subtitle: str
    description: str
    topics: str
    sources: tuple[SourceDocument, ...]


class BuildError(RuntimeError):
    """Raised when the requested source snapshot cannot be assembled."""


def run_git(repository: Path, *arguments: str) -> str:
    result = subprocess.run(
        ("git", *arguments),
        cwd=repository,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip()
        raise BuildError(f"git {' '.join(arguments)} failed: {detail}")
    return result.stdout


def repository_root() -> Path:
    script_root = Path(__file__).resolve().parent.parent
    root = run_git(script_root, "rev-parse", "--show-toplevel").strip()
    return Path(root)


def resolve_commit(repository: Path, ref: str) -> str:
    return run_git(repository, "rev-parse", "--verify", f"{ref}^{{commit}}").strip()


def committed_paths(repository: Path, commit: str, directory: str) -> tuple[str, ...]:
    listing = run_git(
        repository,
        "ls-tree",
        "-r",
        "--name-only",
        commit,
        "--",
        directory,
    )
    return tuple(line for line in listing.splitlines() if line)


def committed_text(repository: Path, commit: str, path: str) -> str:
    return run_git(repository, "show", f"{commit}:{path}")


def documentation_paths(repository: Path, commit: str) -> tuple[str, ...]:
    available = {
        path
        for path in committed_paths(repository, commit, "docs")
        if PurePosixPath(path).suffix.lower() in {".md", ".markdown"}
    }
    preferred = tuple(
        path
        for name in DOCUMENTATION_ORDER
        if (path := f"docs/{name}.md") in available
    )
    preferred_set = set(preferred)
    return preferred + tuple(sorted(available - preferred_set))


def heading_title(text: str, fallback: str) -> str:
    fence_character = ""
    fence_length = 0
    for line in text.splitlines():
        fence = FENCE_RE.match(line)
        if fence:
            marker = fence.group("fence")
            if not fence_character:
                fence_character = marker[0]
                fence_length = len(marker)
            elif marker[0] == fence_character and len(marker) >= fence_length:
                fence_character = ""
                fence_length = 0
            continue
        if fence_character:
            continue
        heading = HEADING_RE.match(line)
        if heading and len(heading.group("marks")) == 1:
            return heading.group("title").strip()
    return fallback


def slug(value: str) -> str:
    value = re.sub(r"`([^`]*)`", r"\1", value)
    value = re.sub(r"<([A-Z][A-Z0-9_]*)>", r"\1", value)
    value = re.sub(r"<[^>]+>", "", value)
    value = value.strip().lower()
    value = re.sub(r"[^\w\- ]", "", value, flags=re.UNICODE)
    value = re.sub(r"[\s\-]+", "-", value)
    return value.strip("-") or "section"


def chapter_anchor(section_label: str, path: str) -> str:
    stem = PurePosixPath(path).with_suffix("").as_posix().replace("/", "-")
    return f"hb-{slug(section_label)}-{slug(stem)}"


def collect_sources(
    repository: Path,
    commit: str,
    paths: Iterable[str],
    section_label: str,
) -> tuple[SourceDocument, ...]:
    sources = []
    for path in paths:
        text = committed_text(repository, commit, path)
        fallback = (
            "HyperBricks skill"
            if path == "SKILLS/hyperbricks/SKILL.md"
            else PurePosixPath(path).stem.replace("_", " ").replace("-", " ").title()
        )
        sources.append(
            SourceDocument(
                path=path,
                title=heading_title(text, fallback),
                text=text,
                anchor=chapter_anchor(section_label, path),
            )
        )
    return tuple(sources)


def normalize_repository_path(source_path: str, target: str) -> str:
    joined = posixpath.join(posixpath.dirname(source_path), target)
    return posixpath.normpath(joined).lstrip("/")


def split_destination(destination: str) -> tuple[str, str]:
    if "#" not in destination:
        return destination, ""
    path, fragment = destination.split("#", 1)
    return path, fragment


def rewrite_link(
    match: re.Match[str],
    source: SourceDocument,
    included: dict[str, SourceDocument],
    commit: str,
) -> str:
    image = match.group("image") or ""
    label = match.group("label")
    raw_destination = match.group("destination")
    title = match.group("title") or ""
    angle_wrapped = raw_destination.startswith("<") and raw_destination.endswith(">")
    destination = raw_destination[1:-1] if angle_wrapped else raw_destination

    if re.match(r"^[a-z][a-z0-9+.-]*:", destination, flags=re.IGNORECASE):
        return match.group(0)
    if destination.startswith("/"):
        return match.group(0)

    target_path, fragment = split_destination(destination)
    resolved_path = source.path if not target_path else normalize_repository_path(source.path, target_path)

    if resolved_path in included:
        target = included[resolved_path]
        rewritten = f"#{target.anchor}"
        if fragment:
            rewritten += f"--{slug(fragment)}"
    elif not target_path and fragment:
        rewritten = f"#{source.anchor}--{slug(fragment)}"
    else:
        base = (
            "https://raw.githubusercontent.com/hyperbricks/hyperbricks"
            if image
            else f"{REPOSITORY_URL}/blob"
        )
        rewritten = f"{base}/{commit}/{resolved_path}"
        if fragment:
            rewritten += f"#{fragment}"

    if angle_wrapped:
        rewritten = f"<{rewritten}>"
    return f"{image}[{label}]({rewritten}{title})"


def rewrite_links(
    line: str,
    source: SourceDocument,
    included: dict[str, SourceDocument],
    commit: str,
) -> str:
    return LINK_RE.sub(
        lambda match: rewrite_link(match, source, included, commit),
        line,
    )


def strip_front_matter(lines: list[str]) -> tuple[list[str], list[str]]:
    if not lines or lines[0].strip() != "---":
        return [], lines
    for index, line in enumerate(lines[1:], start=1):
        if line.strip() == "---":
            return lines[1:index], lines[index + 1 :]
    return [], lines


def transform_source(
    source: SourceDocument,
    included: dict[str, SourceDocument],
    commit: str,
    expose_front_matter: bool,
) -> str:
    original_lines = source.text.splitlines()
    front_matter, lines = (
        strip_front_matter(original_lines)
        if expose_front_matter
        else ([], original_lines)
    )
    output: list[str] = []

    if expose_front_matter and front_matter:
        output.extend(("### Skill metadata", "", "```yaml", *front_matter, "```", ""))

    fence_character = ""
    fence_length = 0
    skipped_document_title = False
    heading_counts: dict[str, int] = {}

    for line in lines:
        fence = FENCE_RE.match(line)
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

        heading = HEADING_RE.match(line)
        if heading:
            level = len(heading.group("marks"))
            title = heading.group("title").strip()
            if level == 1 and title == source.title and not skipped_document_title:
                skipped_document_title = True
                continue

            base_anchor = f"{source.anchor}--{slug(title)}"
            heading_counts[base_anchor] = heading_counts.get(base_anchor, 0) + 1
            count = heading_counts[base_anchor]
            anchor = base_anchor if count == 1 else f"{base_anchor}-{count}"
            output.append(f'<a id="{anchor}"></a>')
            output.append("")
            output.append(
                f"{'#' * min(6, level + 1)} "
                f"{rewrite_links(title, source, included, commit)}"
            )
            continue

        output.append(rewrite_links(line, source, included, commit))

    return "\n".join(output).strip()


def render_handbook(handbook: Handbook, commit: str, commit_date: date) -> str:
    included = {source.path: source for source in handbook.sources}
    short_commit = commit[:7]
    source_url = f"{REPOSITORY_URL}/tree/{commit}"
    lines = [
        "<!-- Generated by scripts/build_markdown_handbooks.py. Do not edit directly. -->",
        "",
        f"# {handbook.title}",
        "",
        f"**{handbook.subtitle}**",
        "",
        handbook.description,
        "",
        handbook.topics,
        "",
        f"- **Included documents:** {len(handbook.sources)}",
        f"- **Source snapshot:** [Git commit `{short_commit}`]({source_url})",
        (
            f"- **Snapshot date:** {commit_date.day} "
            f"{MONTH_NAMES[commit_date.month - 1]} {commit_date.year}"
        ),
        "",
        "## Contents",
        "",
    ]

    for index, source in enumerate(handbook.sources, start=1):
        lines.append(f"{index}. [{source.title}](#{source.anchor}) — `{source.path}`")

    for index, source in enumerate(handbook.sources, start=1):
        source_url = f"{REPOSITORY_URL}/blob/{commit}/{source.path}"
        lines.extend(
            (
                "",
                "---",
                "",
                f'<a id="{source.anchor}"></a>',
                "",
                f"## {index:02d}. {source.title}",
                "",
                f"_Source: [`{source.path}`]({source_url})_",
                "",
                transform_source(
                    source,
                    included,
                    commit,
                    expose_front_matter=source.path.endswith("/SKILL.md"),
                ),
            )
        )

    return "\n".join(lines).rstrip() + "\n"


def build_handbooks(repository: Path, commit: str) -> tuple[Handbook, Handbook]:
    docs = collect_sources(
        repository,
        commit,
        documentation_paths(repository, commit),
        "documentation",
    )
    if not docs:
        raise BuildError("no Markdown documentation files found under docs")

    snapshot_paths = set(committed_paths(repository, commit, "SKILLS/hyperbricks"))
    missing_skills = [path for path in SKILLS_ORDER if path not in snapshot_paths]
    if missing_skills:
        raise BuildError(
            "required skills source files are missing from the snapshot: "
            + ", ".join(missing_skills)
        )
    skills = collect_sources(repository, commit, SKILLS_ORDER, "skills")

    return (
        Handbook(
            filename="HyperBricks-Documentation.md",
            title="HyperBricks Documentation",
            subtitle="Developer handbook",
            description="A complete collection of the guides and references in the docs directory.",
            topics="Declarative applications. Component runtime. Hypermedia.",
            sources=docs,
        ),
        Handbook(
            filename="HyperBricks-Skills.md",
            title="HyperBricks Skills Handbook",
            subtitle="Practical workflows and references",
            description=(
                "The HyperBricks skill and its supporting guides, collected into one "
                "practical handbook."
            ),
            topics="Project lifecycle. Authoring. Integrations. Plugins.",
            sources=skills,
        ),
    )


def write_atomic(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{path.name}.",
        dir=path.parent,
        text=True,
    )
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(content)
        os.chmod(temporary_name, 0o644)
        os.replace(temporary_name, path)
    except BaseException:
        try:
            os.unlink(temporary_name)
        except FileNotFoundError:
            pass
        raise


def display_path(path: Path, repository: Path) -> Path:
    try:
        return path.relative_to(repository)
    except ValueError:
        return path


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Build consolidated Markdown handbooks from a committed HyperBricks "
            "repository snapshot."
        )
    )
    parser.add_argument(
        "--ref",
        default="HEAD",
        help="Git commit, tag, or branch to build (default: HEAD).",
    )
    parser.add_argument(
        "--output-dir",
        default="output/markdown",
        help="Output directory, relative to the repository root unless absolute.",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Verify that the existing outputs match the selected ref without writing them.",
    )
    return parser.parse_args()


def main() -> int:
    arguments = parse_arguments()
    try:
        repository = repository_root()
        commit = resolve_commit(repository, arguments.ref)
        commit_date_text = run_git(
            repository,
            "show",
            "-s",
            "--format=%cs",
            commit,
        ).strip()
        commit_date = date.fromisoformat(commit_date_text)
        output_directory = Path(arguments.output_dir)
        if not output_directory.is_absolute():
            output_directory = repository / output_directory

        handbooks = build_handbooks(repository, commit)
        stale: list[Path] = []
        for handbook in handbooks:
            output = output_directory / handbook.filename
            content = render_handbook(handbook, commit, commit_date)
            if arguments.check:
                if not output.is_file() or output.read_text(encoding="utf-8") != content:
                    stale.append(output)
                continue
            write_atomic(output, content)
            print(f"Wrote {display_path(output, repository)} ({len(handbook.sources)} sources)")

        if stale:
            for path in stale:
                print(f"Out of date: {display_path(path, repository)}", file=sys.stderr)
            return 1
        if arguments.check:
            print(f"Markdown handbooks match {commit[:7]}")
        return 0
    except (BuildError, OSError, UnicodeError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
