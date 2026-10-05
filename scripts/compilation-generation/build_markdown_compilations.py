#!/usr/bin/env python3
"""Build the HyperBricks documentation and skills compilations."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from datetime import date
import json
from pathlib import Path, PurePosixPath
import os
import posixpath
import re
from string import Formatter
import subprocess
import sys
import tempfile
from typing import Iterable, Optional


REPOSITORY_URL = "https://github.com/hyperbricks/hyperbricks"
WORKTREE = "WORKTREE"
TEXTS_PATH = Path(__file__).resolve().with_name("compilation-texts.json")
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

DOCUMENTATION_SECTIONS = (
    (
        "Start",
        (
            ("Introduction", "docs/INTRODUCTION.md"),
            ("Quickstart", "docs/QUICKSTART.md"),
            ("Package Configuration", "docs/PACKAGE_CONFIGURATION.md"),
            ("How-to guides", "docs/HOWTOS.md"),
            ("Troubleshooting", "docs/TROUBLESHOOTING.md"),
        ),
    ),
    (
        "Application model",
        (
            ("Routing", "docs/ROUTING.md"),
            ("Component reference", "docs/REFERENCE.md"),
            ("Markdown", "docs/MARKDOWN.md"),
            ("Spaces CMS", "docs/SPACES.md"),
            ("Authoring", "docs/AUTHOR.md"),
        ),
    ),
    (
        "Logic and assets",
        (
            ("API Render", "docs/API_RENDER.md"),
            ("Server Scripts", "docs/GOJA_RENDER.md"),
            ("Plugins", "docs/PLUGINS.md"),
            ("JavaScript and CSS", "docs/ESBUILD.md"),
        ),
    ),
    (
        "Development services",
        (
            ("Hooks and managed services", "docs/DEVELOPMENT_HOOKS.md"),
        ),
    ),
    (
        "Delivery",
        (
            ("Deploy Guide", "docs/DEPLOY.md"),
            ("Docker Deploy", "docs/DOCKER.md"),
            ("Migration Guide", "docs/MIGRATION.md"),
        ),
    ),
)

ADDITIONAL_DOCUMENTS_SECTION = "Additional documents"
DOCUMENTATION_NAVIGATION = {
    path: (section, label)
    for section, entries in DOCUMENTATION_SECTIONS
    for label, path in entries
}

SKILLS_ORDER = (
    "SKILLS/hyperbricks/SKILL.md",
)

GENERATED_SKILL_COMPILATIONS = {
    "SKILLS/hyperbricks/references/HyperBricks-Documentation.md",
    "SKILLS/hyperbricks/references/HyperBricks-Skills.md",
}
GENERATED_SKILL_DOCUMENTATION = {
    "SKILLS/hyperbricks/references/DOCUMENTATION_INDEX.md",
}
GENERATED_SKILL_DOCUMENTATION_PREFIX = "SKILLS/hyperbricks/references/docs/"

LINK_RE = re.compile(
    r"(?P<image>!)?\[(?P<label>[^\]]*)\]"
    r"\((?P<destination><[^>]+>|[^\s)]+)"
    r"(?P<title>\s+(?:\"[^\"]*\"|'[^']*'|\([^)]*\)))?\)"
)
HEADING_RE = re.compile(r"^(?P<indent>\s*)(?P<marks>#{1,6})\s+(?P<title>.+?)\s*$")
FENCE_RE = re.compile(r"^\s*(?P<fence>`{3,}|~{3,})")
ANCHOR_RE = re.compile(r'''<a\s+id\s*=\s*(["'])(?P<anchor>[^"']+)\1\s*>\s*</a>''')


@dataclass(frozen=True)
class SourceDocument:
    path: str
    title: str
    text: str
    anchor: str
    section: str = ""
    display_title: str = ""


@dataclass(frozen=True)
class CoverTexts:
    label: str
    brand: str
    heading: str
    detail: str
    summary: str
    source: str


@dataclass(frozen=True)
class Compilation:
    filename: str
    title: str
    subtitle: str
    description: str
    topics: str
    sources: tuple[SourceDocument, ...]
    version: str = ""
    cover: Optional[CoverTexts] = None
    kind: str = ""
    topics_label: str = "Topics"
    repository_ref: str = ""


class BuildError(RuntimeError):
    """Raised when the requested source snapshot cannot be assembled."""


TEXT_FIELDS = ("title", "subtitle", "description", "topics_label", "topics")
COVER_TEXT_FIELDS = ("label", "brand", "heading", "detail", "summary", "source")
TEXT_PLACEHOLDERS = {
    "version",
    "subtitle",
    "document_count",
    "snapshot_date",
    "short_commit",
}
CONTENT_PLACEHOLDERS = {"version", "document_count", "short_commit"}


def template_fields(
    template: str,
    location: str,
    allowed: set[str] = TEXT_PLACEHOLDERS,
) -> set[str]:
    try:
        parsed = tuple(Formatter().parse(template))
    except ValueError as error:
        raise BuildError(f"invalid text template in {location}: {error}") from error
    fields = {field for _, field, _, _ in parsed if field is not None}
    formatted = [field for _, field, spec, conversion in parsed if field is not None and (spec or conversion)]
    if formatted:
        raise BuildError(f"formatting is not supported in {location}")
    unsupported = fields - allowed
    if unsupported:
        raise BuildError(
            f"unsupported placeholder in {location}: " + ", ".join(sorted(unsupported))
        )
    return fields


def load_compilation_texts(path: Path = TEXTS_PATH) -> dict:
    def unique_object(pairs):
        value = {}
        for key, item in pairs:
            if key in value:
                raise BuildError(f"duplicate key in {path}: {key}")
            value[key] = item
        return value

    try:
        value = json.loads(
            path.read_text(encoding="utf-8"), object_pairs_hook=unique_object
        )
    except FileNotFoundError as error:
        raise BuildError(f"compilation text file is missing: {path}") from error
    except json.JSONDecodeError as error:
        raise BuildError(
            f"invalid JSON in {path} at line {error.lineno}, column {error.colno}: {error.msg}"
        ) from error

    if not isinstance(value, dict):
        raise BuildError(f"{path} must contain a JSON object")
    expected_sections = {"documentation", "skills"}
    if set(value) != expected_sections:
        raise BuildError(
            f"{path} must contain exactly these sections: "
            + ", ".join(sorted(expected_sections))
        )

    for section_name in ("documentation", "skills"):
        section = value[section_name]
        if not isinstance(section, dict):
            raise BuildError(f"{section_name} in {path} must be a JSON object")
        expected_fields = set(TEXT_FIELDS) | {"cover"}
        if set(section) != expected_fields:
            raise BuildError(
                f"{section_name} in {path} must contain exactly: "
                + ", ".join(sorted(expected_fields))
            )
        for field in TEXT_FIELDS:
            text = section[field]
            if not isinstance(text, str) or not text.strip():
                raise BuildError(f"{section_name}.{field} in {path} must be a non-empty string")
            template_fields(text, f"{section_name}.{field}", CONTENT_PLACEHOLDERS)
        cover = section["cover"]
        if not isinstance(cover, dict) or set(cover) != set(COVER_TEXT_FIELDS):
            raise BuildError(
                f"{section_name}.cover in {path} must contain exactly: "
                + ", ".join(sorted(COVER_TEXT_FIELDS))
            )
        for field in COVER_TEXT_FIELDS:
            text = cover[field]
            if not isinstance(text, str) or not text.strip():
                raise BuildError(
                    f"{section_name}.cover.{field} in {path} must be a non-empty string"
                )
            template_fields(text, f"{section_name}.cover.{field}")
    return value


def format_text(template: str, values: dict, location: str) -> str:
    fields = template_fields(template, location)
    missing = fields - set(values)
    if missing:
        raise BuildError(
            f"missing value for {location}: " + ", ".join(sorted(missing))
        )
    return template.format_map(values)


def cover_texts(section: dict) -> CoverTexts:
    cover = section["cover"]
    return CoverTexts(**{field: cover[field] for field in COVER_TEXT_FIELDS})


def ensure_worktree_texts_are_stable(texts: dict) -> None:
    for section_name, section in texts.items():
        for field in TEXT_FIELDS:
            if "short_commit" in template_fields(section[field], f"{section_name}.{field}"):
                raise BuildError(f"{section_name}.{field} cannot use short_commit with working-tree sources")
        for field in COVER_TEXT_FIELDS:
            unstable = template_fields(section["cover"][field], f"{section_name}.cover.{field}") & {
                "short_commit", "snapshot_date"
            }
            if unstable:
                raise BuildError(
                    f"{section_name}.cover.{field} cannot use {', '.join(sorted(unstable))} "
                    "with working-tree sources"
                )


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
    script_root = Path(__file__).resolve().parents[2]
    root = run_git(script_root, "rev-parse", "--show-toplevel").strip()
    return Path(root)


def resolve_commit(repository: Path, ref: str) -> str:
    return run_git(repository, "rev-parse", "--verify", f"{ref}^{{commit}}").strip()


def snapshot_version(repository: Path, commit: str) -> str:
    version = committed_text(repository, commit, "assets/version.md").strip()
    if not version or len(version.splitlines()) != 1:
        raise BuildError("assets/version.md must contain a single non-empty version line")
    return version


def repository_link_ref(version: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", version):
        raise BuildError(
            "assets/version.md must contain a GitHub-safe release tag name"
        )
    return version


def committed_paths(repository: Path, commit: str, directory: str) -> tuple[str, ...]:
    if commit == WORKTREE:
        listing = run_git(repository, "ls-files", "--cached", "--others", "--exclude-standard", "--", directory)
        return tuple(sorted({
            path for path in listing.splitlines()
            if (repository / path).is_file() and not (repository / path).is_symlink()
        }))
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
    if commit == WORKTREE:
        source = repository / path
        if source.is_symlink() or not source.is_file():
            raise BuildError(f"working-tree source is missing or symlinked: {path}")
        return source.read_text(encoding="utf-8")
    return run_git(repository, "show", f"{commit}:{path}")


def documentation_paths(repository: Path, commit: str) -> tuple[str, ...]:
    available = {
        path
        for path in committed_paths(repository, commit, "docs")
        if PurePosixPath(path).suffix.lower() in {".md", ".markdown"}
        and not path.startswith("docs/compilations/")
    }
    preferred = tuple(
        path
        for _, entries in DOCUMENTATION_SECTIONS
        for _, path in entries
        if path in available
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
    navigation=None,
    default_section: str = "",
) -> tuple[SourceDocument, ...]:
    sources = []
    for path in paths:
        text = committed_text(repository, commit, path)
        fallback = (
            "HyperBricks skill"
            if path == "SKILLS/hyperbricks/SKILL.md"
            else PurePosixPath(path).stem.replace("_", " ").replace("-", " ").title()
        )
        section, display_title = (navigation or {}).get(
            path,
            (default_section, ""),
        )
        sources.append(
            SourceDocument(
                path=path,
                title=heading_title(text, fallback),
                text=text,
                anchor=chapter_anchor(section_label, path),
                section=section,
                display_title=display_title,
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

        # Explicit source anchors must match the chapter-prefixed fragment links.
        line = ANCHOR_RE.sub(
            lambda match: f'<a id="{source.anchor}--{slug(match.group("anchor"))}"></a>',
            line,
        )

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


def render_compilation(compilation: Compilation, commit: str, commit_date: date) -> str:
    included = {source.path: source for source in compilation.sources}
    repository_ref = compilation.repository_ref or commit
    source_url = f"{REPOSITORY_URL}/tree/{repository_ref}"
    lines = [
        "<!-- Generated by scripts/compilation-generation/build_markdown_compilations.py. Do not edit directly. -->",
        "",
        f"# {compilation.title}",
        "",
        f"**{compilation.subtitle}**",
        "",
        compilation.description,
        "",
        f"**{compilation.topics_label}:** {compilation.topics}",
        "",
        *([f"- **HyperBricks version:** {compilation.version}"] if compilation.version else []),
        f"- **Included documents:** {len(compilation.sources)}",
        f"- **Source version:** [`{repository_ref}`]({source_url})",
        "",
        "## Contents",
        "",
    ]

    if compilation.kind == "skills":
        for source in compilation.sources:
            for title, anchor in source_heading_entries(source, 2):
                lines.append(f"- [{title}](#{anchor})")
    else:
        current_section = None
        for source in compilation.sources:
            if source.section and source.section != current_section:
                if current_section is not None:
                    lines.append("")
                lines.extend((f"### {source.section}", ""))
                current_section = source.section
            label = source.display_title or source.title
            lines.append(f"- [{label}](#{source.anchor}) — `{source.path}`")

    for source in compilation.sources:
        source_url = f"{REPOSITORY_URL}/blob/{repository_ref}/{source.path}"
        lines.extend(
            (
                "",
                "---",
                "",
                f'<a id="{source.anchor}"></a>',
                "",
                f"## {source.title}",
                "",
                f"_Source: [`{source.path}`]({source_url})_",
                "",
                transform_source(
                    source,
                    included,
                    repository_ref,
                    expose_front_matter=source.path.endswith("/SKILL.md"),
                ),
            )
        )

    return "\n".join(lines).rstrip() + "\n"


def source_heading_entries(
    source: SourceDocument,
    target_level: int,
) -> tuple[tuple[str, str], ...]:
    """Return visible headings and the anchors produced by transform_source."""
    entries = []
    heading_counts: dict[str, int] = {}
    fence_character = ""
    fence_length = 0
    for line in source.text.splitlines():
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
        if not heading:
            continue
        level = len(heading.group("marks"))
        title = heading.group("title").strip()
        base_anchor = f"{source.anchor}--{slug(title)}"
        heading_counts[base_anchor] = heading_counts.get(base_anchor, 0) + 1
        count = heading_counts[base_anchor]
        anchor = base_anchor if count == 1 else f"{base_anchor}-{count}"
        if level == target_level:
            entries.append((title, anchor))
    return tuple(entries)


def skill_source_paths(snapshot_paths: set[str]) -> tuple[str, ...]:
    missing_skills = [path for path in SKILLS_ORDER if path not in snapshot_paths]
    if missing_skills:
        raise BuildError(
            "required skills source files are missing from the snapshot: "
            + ", ".join(missing_skills)
        )
    extra_references = tuple(sorted(
        path for path in snapshot_paths
        if path.startswith("SKILLS/hyperbricks/references/")
        and PurePosixPath(path).suffix.lower() in {".md", ".markdown"}
        and path not in SKILLS_ORDER
        and path not in GENERATED_SKILL_COMPILATIONS
        and path not in GENERATED_SKILL_DOCUMENTATION
        and not path.startswith(GENERATED_SKILL_DOCUMENTATION_PREFIX)
    ))
    return SKILLS_ORDER + extra_references


def build_compilations(repository: Path, commit: str) -> tuple[Compilation, Compilation]:
    version = snapshot_version(repository, commit)
    link_ref = repository_link_ref(version)
    texts = load_compilation_texts()
    if commit == WORKTREE:
        ensure_worktree_texts_are_stable(texts)
    docs = collect_sources(
        repository,
        commit,
        documentation_paths(repository, commit),
        "documentation",
        navigation=DOCUMENTATION_NAVIGATION,
        default_section=ADDITIONAL_DOCUMENTS_SECTION,
    )
    if not docs:
        raise BuildError("no Markdown documentation files found under docs")

    snapshot_paths = set(committed_paths(repository, commit, "SKILLS/hyperbricks"))
    skills = collect_sources(
        repository,
        commit,
        skill_source_paths(snapshot_paths),
        "skills",
    )

    def compilation_for(
        section_name: str,
        filename: str,
        sources: tuple[SourceDocument, ...],
    ) -> Compilation:
        section = texts[section_name]
        values = {
            "version": version,
            "document_count": len(sources),
            "short_commit": commit[:7],
        }
        subtitle = format_text(
            section["subtitle"], values, f"{section_name}.subtitle"
        )
        return Compilation(
            filename=filename,
            title=format_text(section["title"], values, f"{section_name}.title"),
            subtitle=subtitle,
            description=format_text(
                section["description"], values, f"{section_name}.description"
            ),
            topics=format_text(section["topics"], values, f"{section_name}.topics"),
            sources=sources,
            version=version,
            cover=cover_texts(section),
            kind=section_name,
            topics_label=format_text(
                section["topics_label"], values, f"{section_name}.topics_label"
            ),
            repository_ref=link_ref,
        )

    return (
        compilation_for(
            "documentation", "HyperBricks-Documentation.md", docs
        ),
        compilation_for("skills", "HyperBricks-Skills.md", skills),
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
            "Build Markdown compilations from the working tree or a Git revision."
        )
    )
    parser.add_argument(
        "--ref",
        default=WORKTREE,
        help="Git commit, tag, or branch to build (default: current working tree).",
    )
    parser.add_argument(
        "--output-dir",
        default="docs/compilations",
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
        commit = WORKTREE if arguments.ref == WORKTREE else resolve_commit(repository, arguments.ref)
        commit_date = (
            date(1970, 1, 1) if commit == WORKTREE
            else date.fromisoformat(run_git(repository, "show", "-s", "--format=%cs", commit).strip())
        )
        output_directory = Path(arguments.output_dir)
        if not output_directory.is_absolute():
            output_directory = repository / output_directory

        compilations = build_compilations(repository, commit)
        stale: list[Path] = []
        for compilation in compilations:
            output = output_directory / compilation.filename
            content = render_compilation(compilation, commit, commit_date)
            if arguments.check:
                if not output.is_file() or output.read_text(encoding="utf-8") != content:
                    stale.append(output)
                continue
            write_atomic(output, content)
            print(f"Wrote {display_path(output, repository)} ({len(compilation.sources)} sources)")

        if stale:
            for path in stale:
                print(f"Out of date: {display_path(path, repository)}", file=sys.stderr)
            return 1
        if arguments.check:
            print(f"Markdown compilations match {commit[:7]}")
        return 0
    except (BuildError, OSError, UnicodeError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
