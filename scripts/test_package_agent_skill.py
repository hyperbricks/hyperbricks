"""Release packaging checks against disposable, committed documentation snapshots."""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from zipfile import ZipFile


SCRIPT = Path(__file__).with_name('package_agent_skill.py')
SPEC = importlib.util.spec_from_file_location('agent_packager', SCRIPT)
packager = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(packager)


class PackageAgentSkillTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.source = self.root / packager.SKILL_PATH
        self.output = self.root / 'bin/hyperbricks-skill-v1.3.0-beta.zip'
        self.environment = dict(os.environ, GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull)
        self.git('init', '-q')
        self.git('config', 'user.name', 'Packaging test')
        self.git('config', 'user.email', 'test@example.invalid')
        self.write('.gitignore', '/bin/\n.DS_Store\n')
        self.write('assets/version.md', 'v1.3.0-beta\n')
        for name in ('QUICKSTART.md', 'YAML_USAGE.md'):
            self.write(f'docs/{name}', f'# {name}\nSource-owned documentation.\n')
            self.skill(f'references/docs/{name}', f'# {name}\nBundled documentation.\n')
        self.skill('SKILL.md', '---\nname: hyperbricks\ndescription: Build HyperBricks projects.\n---\n'
                   '# HyperBricks\nRead [Quickstart](references/docs/QUICKSTART.md).\n')
        self.skill('references/DOCUMENTATION_INDEX.md', '# HyperBricks Documentation Snapshot\n'
                   '- **HyperBricks version:** v1.3.0-beta\n'
                   '- [Quickstart](docs/QUICKSTART.md)\n- [YAML](docs/YAML_USAGE.md)\n'
                   '- [Manifest](documentation-manifest.json)\n')
        self.skill('agents/openai.yaml', 'interface:\n  display_name: HyperBricks\n')
        self.skill('assets/icon.svg', '<svg xmlns="http://www.w3.org/2000/svg"/>\n')
        self.refresh_manifest()
        generator_directory = self.root / 'scripts/compilation-generation'
        generator_directory.mkdir(parents=True)
        for name in ('build_skill_documentation.py', 'build_markdown_compilations.py'):
            shutil.copy2(SCRIPT.parent / 'compilation-generation' / name, generator_directory / name)
        generator = generator_directory / 'build_skill_documentation.py'
        result = subprocess.run([sys.executable, '-B', str(generator), '--ref', 'WORKTREE'],
                                cwd=self.root, capture_output=True, text=True, env=self.environment)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.manifest = json.loads((self.source / packager.MANIFEST).read_text())
        self.commit()

    def git(self, *arguments):
        result = subprocess.run(['git', '-C', str(self.root), *arguments], env=self.environment,
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def write(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def skill(self, name, content):
        return self.write(str(packager.SKILL_PATH / name), content)

    def refresh_manifest(self):
        documents = []
        for path in sorted((self.root / 'docs').rglob('*.md')):
            name = path.relative_to(self.root).as_posix()
            documents.append({'source_path': name, 'bundled_path': name,
                              'source_sha256': packager.source_hash(path.read_bytes()),
                              'bundled_sha256': packager.sha256((self.source / 'references' / name).read_bytes())})
        entries = [(document['source_path'], document['source_sha256']) for document in documents]
        self.manifest = {'schema_version': 2, 'hyperbricks_version': 'v1.3.0-beta',
                         'source_repository': packager.REPOSITORY_URL,
                         'source_digest': packager.sha256(json.dumps(['v1.3.0-beta', entries],
                                            ensure_ascii=False, separators=(',', ':')).encode()),
                         'documents': documents}
        self.write_manifest()

    def write_manifest(self):
        self.skill(packager.MANIFEST, json.dumps(self.manifest))

    def commit(self):
        self.git('add', '--all')
        self.git('-c', 'commit.gpgsign=false', 'commit', '-qm', 'Fixture snapshot')

    def package(self):
        return packager.build_archive(self.root, self.output)

    def assert_rejected_preserves_output(self, expression):
        self.output.parent.mkdir(exist_ok=True)
        self.output.write_bytes(b'previous distribution')
        with self.assertRaisesRegex(packager.PackageError, expression):
            self.package()
        self.assertEqual(self.output.read_bytes(), b'previous distribution')
        self.assertEqual(list(self.output.parent.iterdir()), [self.output])

    def test_complete_archive_matches_exact_committed_skill_and_is_deterministic(self):
        output, version, revision = self.package()
        self.assertEqual((output, version, revision), (self.output, 'v1.3.0-beta', self.git('rev-parse', 'HEAD')))
        expected = {p.relative_to(self.source).as_posix(): p.read_bytes()
                    for p in self.source.rglob('*') if p.is_file()}
        with ZipFile(output) as archive:
            self.assertEqual(set(archive.namelist()), {f'hyperbricks/{name}' for name in expected})
            self.assertIsNone(archive.testzip())
            for name, data in expected.items():
                self.assertEqual(archive.read(f'hyperbricks/{name}'), data)
            for entry in archive.infolist():
                self.assertEqual(entry.date_time, (1980, 1, 1, 0, 0, 0))
                self.assertEqual(entry.external_attr >> 16, stat.S_IFREG | 0o644)
        first = output.read_bytes()
        checksum = output.with_name(output.name + '.sha256')
        self.assertEqual(checksum.read_text(), f'{packager.sha256(first)}  {output.name}\n')
        for path in self.source.rglob('*'):
            os.utime(path, (1800000000, 1800000000))
        self.package()
        self.assertEqual(output.read_bytes(), first)
        self.assertEqual(checksum.read_text(), f'{packager.sha256(first)}  {output.name}\n')

    def test_unrelated_work_does_not_prevent_packaging(self):
        self.write('app.go', 'package app\n')
        self.package()

    def test_dirty_version_skill_and_docs_are_rejected(self):
        for name in ('assets/version.md', str(packager.SKILL_PATH / 'SKILL.md'), 'docs/QUICKSTART.md'):
            with self.subTest(name=name):
                path = self.root / name
                original = path.read_text()
                path.write_text(original + 'Changed\n')
                self.assert_rejected_preserves_output('uncommitted changes')
                path.write_text(original)

    def test_untracked_relevant_document_and_skill_file_are_rejected(self):
        for name in ('docs/NEW.md', str(packager.SKILL_PATH / 'references/NEW.md')):
            path = self.write(name, 'Uncommitted\n')
            self.assert_rejected_preserves_output('uncommitted changes')
            path.unlink()

    def test_committed_source_change_requires_snapshot_regeneration(self):
        self.write('docs/QUICKSTART.md', 'Updated canonical source\n')
        self.commit()
        self.assert_rejected_preserves_output('Stale documentation snapshot')

    def test_committed_new_or_deleted_document_requires_snapshot_regeneration(self):
        self.write('docs/NEW.md', 'New canonical source\n')
        self.commit()
        self.assert_rejected_preserves_output('every canonical document')
        (self.root / 'docs/QUICKSTART.md').unlink()
        self.commit()
        self.assert_rejected_preserves_output('Invalid or duplicate')

    def test_missing_or_modified_bundled_doc_is_rejected(self):
        self.skill('references/docs/QUICKSTART.md', 'Tampered bundle\n')
        self.commit()
        self.assert_rejected_preserves_output('modified bundled documentation')
        (self.source / 'references/docs/QUICKSTART.md').unlink()
        self.commit()
        self.assert_rejected_preserves_output('Missing or modified')

    def test_bundle_changes_with_updated_hashes_still_require_canonical_generation(self):
        self.skill('references/docs/QUICKSTART.md', 'Hand-edited generated documentation\n')
        for document in self.manifest['documents']:
            document['bundled_sha256'] = packager.sha256(
                (self.source / 'references' / document['bundled_path']).read_bytes())
        self.write_manifest()
        self.commit()
        self.assert_rejected_preserves_output('does not match canonical docs')

    def test_manifest_version_schema_repository_and_digest_are_validated(self):
        original = dict(self.manifest)
        for field, value in (('hyperbricks_version', 'v1.3.1'), ('schema_version', 3),
                             ('source_repository', 'https://example.invalid'), ('source_digest', '0' * 64)):
            self.manifest = dict(original, **{field: value})
            self.write_manifest()
            self.commit()
            self.assert_rejected_preserves_output('manifest')

    def test_duplicate_or_traversal_document_is_rejected(self):
        original = self.manifest['documents'][:]
        self.manifest['documents'] += self.manifest['documents'][:1]
        self.write_manifest()
        self.commit()
        self.assert_rejected_preserves_output('duplicate')
        self.manifest['documents'] = original
        self.manifest['documents'][0]['source_path'] = '../private.md'
        self.write_manifest()
        self.commit()
        self.assert_rejected_preserves_output('Unsafe')

    def test_version_filename_preserves_full_runtime_version(self):
        self.assertEqual(packager.archive_filename('v1.3.0-beta.2+build.3'),
                         'hyperbricks-skill-v1.3.0-beta.2+build.3.zip')
        for value in ('../../other', '1.3.0', 'v1.3.0\n', 'v1.3.0-01', 'v1.03.0'):
            with self.subTest(value=value), self.assertRaises(packager.PackageError):
                packager.archive_filename(value)

    def test_index_and_skill_links_must_remain_self_contained(self):
        self.skill('SKILL.md', '---\nname: hyperbricks\ndescription: Build modules.\n---\n'
                   '[Quickstart](references/docs/MISSING.md)\n')
        self.commit()
        self.assert_rejected_preserves_output('missing bundled file')

    def test_index_requires_matching_version_and_every_document(self):
        path = self.source / packager.INDEX
        original = path.read_text()
        for content, message in ((original.replace('v1.3.0-beta', 'v1.3.1'), 'index version'),
                                 ('\n'.join(line for line in original.splitlines()
                                            if 'docs/YAML_USAGE.md' not in line) + '\n', 'every bundled document')):
            path.write_text(content)
            self.commit()
            self.assert_rejected_preserves_output(message)

    def test_unsafe_case_colliding_and_ignored_metadata_paths_are_rejected(self):
        path = self.skill('.DS_Store', 'metadata')
        self.assert_rejected_preserves_output('metadata')
        path.unlink()
        path = self.skill('assets/CON.svg', 'reserved')
        self.commit()
        self.assert_rejected_preserves_output('Unsafe')
        path.unlink()
        self.commit()
        files, version, revision = packager.release_files(self.root)
        # Case-insensitive hosts cannot create both names on disk, but Git and
        # ZIP can contain both. Validate the committed-file representation.
        files['assets/Icon.svg'] = b'collision'
        docs = packager.committed_files(self.root, revision, 'docs')
        with self.assertRaisesRegex(packager.PackageError, 'collide'):
            packager.validate_snapshot(files, docs, version)

    def test_symlinked_source_or_committed_member_is_rejected(self):
        path = self.source / 'assets/external.svg'
        path.symlink_to(self.root / 'docs/QUICKSTART.md')
        self.commit()
        self.assert_rejected_preserves_output('Non-regular')
        path.unlink()
        self.commit()
        moved = self.root / 'moved-skill'
        self.source.rename(moved)
        self.source.symlink_to(moved, target_is_directory=True)
        self.assert_rejected_preserves_output('Symlinked canonical')

    def test_destination_symlink_and_source_directory_are_preserved(self):
        self.output.parent.mkdir()
        other = self.root / 'other.zip'
        other.write_bytes(b'original')
        self.output.symlink_to(other)
        with self.assertRaisesRegex(packager.PackageError, 'symlink'):
            self.package()
        self.assertEqual(other.read_bytes(), b'original')
        with self.assertRaisesRegex(packager.PackageError, 'inside the source'):
            packager.build_archive(self.root, self.source / 'archive.zip')

    def test_checksum_symlink_or_directory_cannot_replace_another_path(self):
        self.output.parent.mkdir()
        other = self.root / 'other.txt'
        other.write_bytes(b'original')
        checksum = self.output.with_name(self.output.name + '.sha256')
        checksum.symlink_to(other)
        with self.assertRaisesRegex(packager.PackageError, 'symlink'):
            self.package()
        self.assertEqual(other.read_bytes(), b'original')
        self.assertFalse(self.output.exists())
        checksum.unlink()
        checksum.mkdir()
        with self.assertRaisesRegex(packager.PackageError, 'regular files'):
            self.package()

    def test_checksum_publication_failure_restores_previous_archive_pair(self):
        self.package()
        previous = self.output.read_bytes()
        checksum = self.output.with_name(self.output.name + '.sha256')
        previous_checksum = checksum.read_bytes()
        self.skill('SKILL.md', (self.source / 'SKILL.md').read_text() + '\nNew instruction.\n')
        self.commit()
        replace = os.replace

        def fail_checksum(source, destination):
            if destination == checksum:
                raise OSError('Checksum replacement failed')
            replace(source, destination)

        with patch.object(packager.os, 'replace', side_effect=fail_checksum):
            with self.assertRaisesRegex(OSError, 'Checksum replacement failed'):
                self.package()
        self.assertEqual(self.output.read_bytes(), previous)
        self.assertEqual(checksum.read_bytes(), previous_checksum)
        self.assertEqual(set(self.output.parent.iterdir()), {self.output, checksum})

    def test_failed_atomic_replace_preserves_output_and_cleans_temporary(self):
        self.output.parent.mkdir()
        self.output.write_bytes(b'previous archive')
        with patch.object(packager.os, 'replace', side_effect=OSError('Replacement failed')):
            with self.assertRaisesRegex(OSError, 'Replacement failed'):
                self.package()
        self.assertEqual(self.output.read_bytes(), b'previous archive')
        self.assertEqual(list(self.output.parent.iterdir()), [self.output])

    def test_failed_checksum_and_rollback_preserves_recovery_backup(self):
        self.package()
        previous = self.output.read_bytes()
        checksum = self.output.with_name(self.output.name + '.sha256')
        previous_checksum = checksum.read_bytes()
        self.skill('SKILL.md', (self.source / 'SKILL.md').read_text() + '\nNew instruction.\n')
        self.commit()
        replace = os.replace

        def fail_checksum_and_recovery(source, destination):
            if destination == checksum or source.suffix == '.previous':
                raise OSError('Cannot replace destination')
            replace(source, destination)

        with patch.object(packager.os, 'replace', side_effect=fail_checksum_and_recovery):
            with self.assertRaisesRegex(packager.PackageError, 'previous archive is preserved at') as raised:
                self.package()
        backups = list(self.output.parent.glob('*.previous'))
        self.assertEqual(len(backups), 1)
        self.assertEqual(backups[0].read_bytes(), previous)
        self.assertIn(str(backups[0]), str(raised.exception))
        self.assertEqual(checksum.read_bytes(), previous_checksum)
        self.assertEqual(set(self.output.parent.iterdir()), {self.output, checksum, backups[0]})

    def test_installer_limits_fail_before_replacing_a_distribution(self):
        for setting in ('MAX_ENTRIES', 'MAX_EXPANDED_BYTES', 'MAX_FILE_BYTES', 'MAX_ARCHIVE_BYTES'):
            with self.subTest(setting=setting), patch.object(packager, setting, 1):
                self.assert_rejected_preserves_output('native installer')

    def test_cli_default_output_uses_release_version(self):
        result = subprocess.run([sys.executable, str(SCRIPT)], cwd=self.root,
                                capture_output=True, text=True, env=self.environment)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(self.output.is_file())
        self.assertIn('v1.3.0-beta', result.stdout)


if __name__ == '__main__':
    unittest.main()
