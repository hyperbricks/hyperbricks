#!/usr/bin/env python3
"""Build a reproducible HyperBricks skill ZIP from reviewed, committed sources."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import subprocess
import sys
import tempfile
from urllib.parse import unquote, urlsplit
from zipfile import ZIP_DEFLATED, ZipFile, ZipInfo


SKILL_PATH = PurePosixPath('.agents/skills/hyperbricks')
VERSION_PATH = 'assets/version.md'
INDEX = 'references/DOCUMENTATION_INDEX.md'
MANIFEST = 'references/documentation-manifest.json'
REPOSITORY_URL = 'https://github.com/hyperbricks/hyperbricks'
# Match the native `hyperbricks skills install` archive limits.
MAX_ARCHIVE_BYTES = 32 * 1024 * 1024
MAX_EXPANDED_BYTES = 64 * 1024 * 1024
MAX_FILE_BYTES = 8 * 1024 * 1024
MAX_ENTRIES = 2000
SEMVER = re.compile(r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)'
                    r'(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?'
                    r'(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?')


class PackageError(ValueError):
    """The release sources are incomplete, stale, or unsafe to distribute."""


def git(repository: Path, *arguments: str) -> bytes:
    result = subprocess.run(('git', '-C', str(repository), *arguments),
                            capture_output=True, check=False)
    if result.returncode:
        raise PackageError(result.stderr.decode('utf-8', errors='replace').strip())
    return result.stdout


def archive_filename(version: str) -> str:
    match = SEMVER.fullmatch(version)
    if not match or any(part.isdigit() and len(part) > 1 and part.startswith('0')
                        for part in (match.group(4) or '').split('.')):
        raise PackageError(f'Invalid HyperBricks runtime version: {version!r}')
    return f'hyperbricks-skill-{version}.zip'


def safe_path(name: str) -> PurePosixPath:
    path = PurePosixPath(name)
    if (not name or path.is_absolute() or path.as_posix() != name
            or any(part.startswith('.') or part == '__MACOSX' or part.endswith((' ', '.'))
                   or re.fullmatch(r'(?i)(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\..*)?', part)
                   or ':' in part or '\\' in part or any(ord(char) < 32 for char in part)
                   for part in path.parts)):
        raise PackageError(f'Unsafe or metadata archive path: {name!r}')
    return path


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def source_hash(data: bytes) -> str:
    # The documentation generator reads text with Python universal newlines.
    return sha256(data.decode('utf-8').replace('\r\n', '\n').replace('\r', '\n').encode('utf-8'))


def committed_files(repository: Path, revision: str, directory: str) -> dict[str, bytes]:
    files = {}
    for entry in git(repository, 'ls-tree', '-r', '-z', revision, '--', directory).split(b'\0'):
        if not entry:
            continue
        metadata, name = entry.split(b'\t', 1)
        mode, kind, object_id = metadata.split()
        if kind != b'blob' or not stat.S_ISREG(int(mode, 8)):
            raise PackageError(f'Non-regular committed source: {name.decode("utf-8")}')
        files[name.decode('utf-8')] = git(repository, 'cat-file', 'blob', object_id.decode('ascii'))
    return files


def local_links(name: str, content: str, files: dict[str, bytes]) -> set[str]:
    links = set()
    for target in re.findall(r'\]\(([^\s)]+)(?:\s+"[^"\n]*")?\)', content):
        url = urlsplit(target.strip('<>'))
        if url.scheme or url.netloc or not url.path:
            continue
        relative = safe_path(unquote(url.path))
        resolved = (PurePosixPath(name).parent / relative).as_posix()
        if resolved not in files:
            raise PackageError(f'{name} links to a missing bundled file: {target}')
        links.add(resolved)
    return links


def validate_snapshot(files: dict[str, bytes], docs: dict[str, bytes], version: str) -> None:
    if len(files) > MAX_ENTRIES or sum(map(len, files.values())) > MAX_EXPANDED_BYTES:
        raise PackageError('Skill exceeds the native installer entry or expanded-size limit.')
    names = set()
    for name, data in files.items():
        safe_path(name)
        if len(data) > MAX_FILE_BYTES:
            raise PackageError(f'Skill file exceeds the native installer size limit: {name}')
        if name.casefold() in names:
            raise PackageError(f'Archive paths collide on a case-insensitive filesystem: {name}')
        names.add(name.casefold())
    for required in ('SKILL.md', INDEX, MANIFEST):
        if required not in files:
            raise PackageError(f'Skill is missing {required}')
    try:
        manifest = json.loads(files[MANIFEST])
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise PackageError(f'Invalid documentation manifest: {error}') from error
    if (not isinstance(manifest, dict) or manifest.get('schema_version') != 2
            or manifest.get('source_repository') != REPOSITORY_URL
            or manifest.get('hyperbricks_version') != version
            or not isinstance(manifest.get('documents'), list) or not manifest['documents']):
        raise PackageError('Documentation manifest schema, repository, or runtime version is invalid.')
    expected_docs = {name for name in docs if PurePosixPath(name).suffix.lower() in {'.md', '.markdown'}
                     and not name.startswith('docs/compilations/')}
    documents = set()
    source_entries = []
    for document in manifest['documents']:
        if not isinstance(document, dict):
            raise PackageError('Invalid manifest document.')
        source = document.get('source_path', '')
        bundled = document.get('bundled_path', '')
        if (not isinstance(source, str) or not isinstance(bundled, str)
                or safe_path(source).parts[0] != 'docs' or bundled != source
                or source in documents or source not in expected_docs):
            raise PackageError(f'Invalid or duplicate manifest document: {source!r}')
        data = files.get(f'references/{bundled}')
        if data is None or sha256(data) != document.get('bundled_sha256'):
            raise PackageError(f'Missing or modified bundled documentation: {bundled}')
        actual_source_hash = source_hash(docs[source])
        if actual_source_hash != document.get('source_sha256'):
            raise PackageError(f'Stale documentation snapshot: {source}')
        documents.add(source)
        source_entries.append((source, actual_source_hash))
    bundled_docs = {name.removeprefix('references/') for name in files if name.startswith('references/docs/')}
    if documents != expected_docs or documents != bundled_docs:
        raise PackageError('Documentation manifest must include exactly every canonical document and bundled document.')
    digest = sha256(json.dumps([version, source_entries], ensure_ascii=False,
                              separators=(',', ':')).encode('utf-8'))
    if digest != manifest.get('source_digest'):
        raise PackageError('Documentation manifest source digest is stale.')
    skill = files['SKILL.md'].decode('utf-8')
    frontmatter = re.match(r'\A---\r?\n(.*?)\r?\n---\r?\n(.+)', skill, re.DOTALL)
    if (not frontmatter or not re.search(r'^name: hyperbricks\s*$', frontmatter.group(1), re.MULTILINE)
            or not re.search(r'^description: \S.+$', frontmatter.group(1), re.MULTILINE)
            or not frontmatter.group(2).strip()):
        raise PackageError('SKILL.md must contain HyperBricks skill frontmatter and instructions.')
    if not local_links('SKILL.md', skill, files).intersection({INDEX} | {f'references/{name}' for name in documents}):
        raise PackageError('SKILL.md must link to its bundled documentation.')
    index = files[INDEX].decode('utf-8')
    if not re.search(r'^- \*\*HyperBricks version:\*\* ' + re.escape(version) + r'\s*$', index, re.MULTILINE):
        raise PackageError('Documentation index version does not match the manifest.')
    if not {f'references/{name}' for name in documents}.issubset(local_links(INDEX, index, files)):
        raise PackageError('Documentation index does not link to every bundled document.')


def release_files(repository: Path) -> tuple[dict[str, bytes], str, str]:
    for relative in ('.agents', '.agents/skills', str(SKILL_PATH)):
        if (repository / relative).is_symlink():
            raise PackageError(f'Symlinked canonical skill directory: {relative}')
    status = git(repository, 'status', '--porcelain', '--untracked-files=all', '--',
                 str(SKILL_PATH), VERSION_PATH, 'docs', ':(exclude)docs/compilations').strip()
    if status:
        raise PackageError('The skill, runtime version, or documentation has uncommitted changes. '
                           'Synchronize and commit reviewed sources before packaging:\n'
                           + status.decode('utf-8', errors='replace'))
    revision = git(repository, 'rev-parse', 'HEAD').decode().strip()
    version = git(repository, 'show', f'{revision}:{VERSION_PATH}').decode('utf-8').strip()
    archive_filename(version)
    skill = committed_files(repository, revision, str(SKILL_PATH))
    prefix = str(SKILL_PATH) + '/'
    files = {name.removeprefix(prefix): data for name, data in skill.items()}
    if not files:
        raise PackageError(f'No committed skill found under {SKILL_PATH}')
    # Include no ignored metadata, install receipts, symlinks, or other local
    # additions. A distribution must match the complete canonical source tree.
    actual = set()
    for directory, directories, filenames in os.walk(repository / SKILL_PATH, followlinks=False):
        for name in directories + filenames:
            path = Path(directory) / name
            if path.is_symlink() or not (path.is_dir() or path.is_file()):
                raise PackageError(f'Skill contains a symlink or special file: {path}')
            if path.is_file():
                relative = path.relative_to(repository / SKILL_PATH).as_posix()
                safe_path(relative)
                actual.add(relative)
                if files.get(relative) != path.read_bytes():
                    raise PackageError(f'Skill file is uncommitted or changed during packaging: {relative}')
    if actual != set(files):
        raise PackageError('Canonical skill files differ from the committed tree.')
    docs = committed_files(repository, revision, 'docs')
    validate_snapshot(files, docs, version)
    # Hash consistency alone cannot prove the bundled prose came from docs/.
    # Reuse its owning generator to check the complete committed snapshot.
    generator = repository / 'scripts/compilation-generation/build_skill_documentation.py'
    check = subprocess.run((sys.executable, '-B', str(generator), '--check', '--ref', revision),
                           cwd=repository, capture_output=True, check=False)
    if check.returncode:
        raise PackageError('Generated skill documentation does not match canonical docs/:\n'
                           + check.stderr.decode('utf-8', errors='replace').strip())
    return files, version, revision


def build_archive(repository: Path, output: Path | None = None) -> tuple[Path, str, str]:
    files, version, revision = release_files(repository)
    output = output or repository / 'bin' / archive_filename(version)
    if not output.is_absolute():
        output = repository / output
    safe_path(output.name)
    checksum = output.with_name(output.name + '.sha256')
    source = (repository / SKILL_PATH).resolve()
    if output.is_symlink() or checksum.is_symlink():
        raise PackageError('Archive and checksum outputs must not be symlinks.')
    if output.resolve() == source or source in output.resolve().parents:
        raise PackageError('Archive output must not be inside the source skill directory.')
    if any(path.exists() and not path.is_file() for path in (output, checksum)):
        raise PackageError('Archive and checksum outputs must be regular files.')
    output.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix='.hyperbricks-skill-', suffix='.zip', dir=output.parent)
    os.close(descriptor)
    temporary = Path(name)
    temporary_checksum = temporary.with_suffix('.sha256')
    previous_archive = temporary.with_suffix('.previous')
    preserve_backup = False
    try:
        with ZipFile(temporary, 'w', compression=ZIP_DEFLATED, compresslevel=9) as archive:
            for path, data in sorted(files.items()):
                entry = ZipInfo(f'hyperbricks/{path}', date_time=(1980, 1, 1, 0, 0, 0))
                entry.create_system = 3
                entry.external_attr = (stat.S_IFREG | 0o644) << 16
                entry.compress_type = ZIP_DEFLATED
                archive.writestr(entry, data, compresslevel=9)
        if temporary.stat().st_size > MAX_ARCHIVE_BYTES:
            raise PackageError('Skill ZIP exceeds the native installer archive-size limit.')
        with ZipFile(temporary) as archive:
            if archive.testzip() is not None:
                raise PackageError('Generated skill archive failed its integrity check.')
        temporary_checksum.write_text(f'{sha256(temporary.read_bytes())}  {output.name}\n', encoding='utf-8')
        temporary_checksum.chmod(0o644)
        temporary.chmod(0o644)
        if output.exists():
            shutil.copy2(output, previous_archive)
        os.replace(temporary, output)
        try:
            os.replace(temporary_checksum, checksum)
        except OSError:
            # Keep an existing published ZIP/checksum pair together on failure.
            if previous_archive.exists():
                try:
                    os.replace(previous_archive, output)
                except OSError as error:
                    preserve_backup = True
                    raise PackageError('Checksum publication and archive recovery failed. '
                                       f'The previous archive is preserved at {previous_archive}') from error
            else:
                output.unlink()
            raise
    finally:
        temporary.unlink(missing_ok=True)
        temporary_checksum.unlink(missing_ok=True)
        if not preserve_backup:
            previous_archive.unlink(missing_ok=True)
    return output, version, revision


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, help='Destination ZIP and adjacent .sha256 (default: bin/hyperbricks-skill-<version>.zip).')
    args = parser.parse_args()
    try:
        repository = Path(git(Path.cwd(), 'rev-parse', '--show-toplevel').decode().strip())
        output, version, revision = build_archive(repository, args.output)
    except (OSError, UnicodeError, PackageError) as error:
        print(f'error: {error}', file=sys.stderr)
        return 1
    print(f'HyperBricks skill ({version}): {output}\nSHA-256: {output}.sha256\nSource commit: {revision}')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
