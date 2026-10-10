"""Exercise release orchestration in disposable repositories with expensive tools stubbed."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parent


class AgentPipelineTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        (self.bin / 'python3').symlink_to(sys.executable)
        self.environment = dict(os.environ, PATH=str(self.bin) + os.pathsep + os.environ['PATH'],
                                PIPELINE_EVENTS=str(self.root / 'events'))
        self.environment.pop('COMPILATION_VENV', None)
        self.canonical = self.root / '.agents/skills/hyperbricks'
        self.write('.agents/skills/hyperbricks/SKILL.md', 'Maintained skill instructions\n')
        self.write('.agents/skills/hyperbricks/references/doc.md', 'Old generated documentation\n')
        self.write('docs/QUICKSTART.md', 'Canonical documentation\n')
        self.write('assets/version.md', 'v1.3.0-beta\n')
        (self.root / 'scripts').mkdir()
        for name in ('sync_docs.sh', 'pre_release.sh', 'prepare_landing_downloads.sh'):
            shutil.copy2(SCRIPTS / name, self.root / 'scripts' / name)

    def write(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return path

    def executable(self, name, content):
        path = self.write(name, content)
        path.chmod(0o755)
        return path

    def run_script(self, name, *arguments, success=True):
        result = subprocess.run(['bash', str(self.root / 'scripts' / name), *map(str, arguments)],
                                cwd=self.root, env=self.environment, capture_output=True, text=True)
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def events(self):
        path = self.root / 'events'
        return path.read_text().splitlines() if path.exists() else []

    def stage_stub(self, name, stage, extra=''):
        return self.executable(name, '#!/bin/sh\n'
                               f'echo {stage} >> "$PIPELINE_EVENTS"\n'
                               f'[ "${{FAIL_STAGE:-}}" != "{stage}" ] || exit 17\n' + extra)

    def setup_sync(self):
        self.stage_stub('scripts/compilation-generation/build_compilations.sh', 'build',
                        'cp docs/QUICKSTART.md .agents/skills/hyperbricks/references/doc.md\n')
        self.executable('.venv-compilations/bin/python', '#!/bin/sh\n'
                        'case "$*" in\n'
                        '  *unittest*) stage=generator-tests;;\n'
                        '  *build_markdown*) stage=compilation-check;;\n'
                        '  *build_skill*) stage=snapshot-check;;\n'
                        '  *) exit 2;;\nesac\n'
                        'echo "$stage" >> "$PIPELINE_EVENTS"\n'
                        '[ "${FAIL_STAGE:-}" != "$stage" ] || exit 17\n')
        self.stage_stub('bin/go', 'docs-tests')
        self.stage_stub('bin/git', 'diff-check')
        (self.bin / 'python3').unlink()
        self.stage_stub('bin/python3', 'release-tests')

    def test_sync_builds_then_checks_and_never_changes_the_source_documents(self):
        self.setup_sync()
        self.run_script('sync_docs.sh')
        self.assertEqual(self.events(), ['build', 'generator-tests', 'release-tests', 'docs-tests',
                                        'compilation-check', 'snapshot-check', 'diff-check'])
        self.assertEqual((self.root / 'docs/QUICKSTART.md').read_text(), 'Canonical documentation\n')
        self.assertEqual((self.canonical / 'references/doc.md').read_text(), 'Canonical documentation\n')
        self.assertEqual((self.canonical / 'SKILL.md').read_text(), 'Maintained skill instructions\n')
        self.assertFalse((self.root / 'codex-plugin').exists())
        self.assertFalse((self.root / 'SKILLS').exists())

    def test_sync_propagates_each_build_and_check_failure(self):
        self.setup_sync()
        stages = ['build', 'generator-tests', 'release-tests', 'docs-tests',
                  'compilation-check', 'snapshot-check', 'diff-check']
        for index, stage in enumerate(stages):
            with self.subTest(stage=stage):
                (self.root / 'events').unlink(missing_ok=True)
                self.environment['FAIL_STAGE'] = stage
                self.run_script('sync_docs.sh', success=False)
                self.assertEqual(self.events(), stages[:index + 1])

    def test_sync_requires_canonical_source_before_generation(self):
        self.setup_sync()
        (self.canonical / 'SKILL.md').unlink()
        self.run_script('sync_docs.sh', success=False)
        self.assertEqual(self.events(), [])

    def test_removed_plugin_option_is_rejected_before_side_effects(self):
        for name in ('pre_release.sh', 'sync_docs.sh', 'prepare_landing_downloads.sh'):
            with self.subTest(name=name):
                result = self.run_script(name, '--with-codex-plugin', success=False)
                self.assertEqual(result.returncode, 2)
        self.assertEqual(self.events(), [])

    def setup_prerelease(self):
        self.stage_stub('tests.sh', 'core-tests', 'printf "%s\\n" "$@" > core-args.txt\n')
        self.stage_stub('scripts/sync_docs.sh', 'sync', 'printf "%s\\n" "$@" > sync-args.txt\n')

    def test_pre_release_preserves_all_four_core_scopes_then_synchronizes(self):
        self.setup_prerelease()
        self.run_script('pre_release.sh')
        self.assertEqual(self.events(), ['core-tests', 'sync'])
        self.assertEqual((self.root / 'core-args.txt').read_text().splitlines(),
                         ['--with-docs', '--with-plugins', '--with-modules', '--with-screenshots'])
        self.assertEqual((self.root / 'sync-args.txt').read_text().strip(), '')

    def test_failed_core_scope_prevents_documentation_sync(self):
        self.setup_prerelease()
        self.environment['FAIL_STAGE'] = 'core-tests'
        self.run_script('pre_release.sh', success=False)
        self.assertEqual(self.events(), ['core-tests'])

    def test_failed_sync_fails_pre_release(self):
        self.setup_prerelease()
        self.environment['FAIL_STAGE'] = 'sync'
        self.run_script('pre_release.sh', success=False)
        self.assertEqual(self.events(), ['core-tests', 'sync'])

    def test_historical_compilation_does_not_replace_current_skill_snapshot(self):
        directory = self.root / 'scripts/compilation-generation'
        directory.mkdir()
        shutil.copy2(SCRIPTS / 'compilation-generation/build_compilations.sh', directory)
        requirements = 'fake-dependency==1\n'
        self.write('scripts/compilation-generation/compilation-requirements.txt', requirements)
        self.write('.venv-compilations/compilation-requirements.txt', requirements)
        self.write('.venv-compilations/mermaid/version', '11.17.0\n')
        self.executable('.venv-compilations/mermaid/node_modules/.bin/mmdc', '#!/bin/sh\nexit 0\n')
        self.executable('.venv-compilations/bin/python', '#!/bin/sh\n'
                        'case "$1" in\n'
                        '  *build_compilations.py) mkdir -p docs/compilations; '
                        'echo compiled > docs/compilations/HyperBricks-Documentation.md;;\n'
                        '  *build_skill_documentation.py) echo regenerated > .agents/skills/hyperbricks/references/doc.md;;\n'
                        'esac\n')
        before = (self.canonical / 'references/doc.md').read_bytes()
        self.run_script('compilation-generation/build_compilations.sh', '--ref', 'v1.2.0', '--format', 'markdown')
        self.assertEqual((self.canonical / 'references/doc.md').read_bytes(), before)
        self.run_script('compilation-generation/build_compilations.sh', '--ref', 'WORKTREE', '--format', 'markdown')
        self.assertEqual((self.canonical / 'references/doc.md').read_text(), 'regenerated\n')

    def setup_downloads(self):
        self.write('editors/vscode/package.json', '{"version": "0.2.0"}\n')
        self.write('editors/vscode/.vscodeignore', 'node_modules\n')
        self.executable('bin/git', '#!/bin/sh\ncase "$*" in\n'
                        '  *status*) [ "${DIRTY_VSCODE:-}" != 1 ] || echo " M package.json";;\n'
                        '  *HEAD:editors/vscode*) echo recorded-commit;;\n'
                        '  *rev-parse*) echo "${VSCODE_HEAD:-recorded-commit}";;\n'
                        '  *show*) echo 1791648000;;\n'
                        'esac\nexit 0\n')
        self.executable('bin/node', '#!/bin/sh\necho 0.2.0\n')
        self.stage_stub('bin/npm', 'vscode-check')
        self.executable('editors/vscode/node_modules/.bin/vsce', '#!' + sys.executable + '\n'
                        'import os, pathlib, sys, zipfile\n'
                        'with open(os.environ["PIPELINE_EVENTS"], "a") as log: log.write("vscode-package\\n")\n'
                        'if os.environ.get("FAIL_STAGE") == "vscode-package": sys.exit(17)\n'
                        'ignore = pathlib.Path(sys.argv[sys.argv.index("--ignoreFile") + 1]).read_text()\n'
                        'assert ".dev/**" in ignore\n'
                        'with zipfile.ZipFile(sys.argv[sys.argv.index("--out") + 1], "w") as output:\n'
                        '    output.writestr("extension/package.json", "{}")\n'
                        '    if os.environ.get("VSIX_DEV"): output.writestr("extension/.dev/runtime", "private")\n')
        self.write('scripts/package_agent_skill.py',
                   'import hashlib, os, pathlib, sys, zipfile\n'
                   'with open(os.environ["PIPELINE_EVENTS"], "a") as log: log.write("skill-package\\n")\n'
                   'if os.environ.get("FAIL_STAGE") == "skill-package": sys.exit(17)\n'
                   'path = pathlib.Path(sys.argv[sys.argv.index("--output") + 1])\n'
                   'with zipfile.ZipFile(path, "w") as output:\n'
                   '    output.writestr("hyperbricks/SKILL.md", "Complete skill")\n'
                   'checksum = hashlib.sha256(path.read_bytes()).hexdigest()\n'
                   'if os.environ.get("SKILL_BAD_SHA"): checksum = "0" * 64\n'
                   'path.with_name(path.name + ".sha256").write_text(checksum + "  " + path.name + "\\n")\n')
        return self.root / 'landing/static/downloads'

    def check_downloads(self, destination):
        values = json.loads((destination / '../../resources/json/downloads.json').read_text())
        self.assertEqual(set(values), {'hyperbricks-skill', 'hyperbricks-skill-sha',
                                       'hyperbricks-vscode', 'hyperbricks-vscode-sha'})
        self.assertEqual(values['hyperbricks-skill'], 'hyperbricks-skill-v1.3.0-beta.zip')
        self.assertEqual(values['hyperbricks-vscode'], 'hyperbricks-vscode-0.2.0+vscode.20261010160000.vsix')
        self.assertEqual({p.name for p in destination.iterdir()}, set(values.values()))
        for key, filename in values.items():
            if key.endswith('-sha'):
                checksum, artifact = (destination / filename).read_text().split()
                self.assertEqual(checksum, hashlib.sha256((destination / artifact).read_bytes()).hexdigest())

    def test_downloads_use_one_shared_skill_and_preserve_vscode_checks(self):
        destination = self.setup_downloads()
        self.run_script('prepare_landing_downloads.sh', destination)
        self.check_downloads(destination)
        self.assertEqual(self.events(), ['skill-package', 'vscode-check', 'vscode-package'])
        self.assertFalse((self.root / 'codex-plugin').exists())

    def test_download_failures_do_not_publish_partial_results(self):
        destination = self.setup_downloads()
        for stage in ('skill-package', 'vscode-check', 'vscode-package'):
            with self.subTest(stage=stage):
                self.environment['FAIL_STAGE'] = stage
                self.run_script('prepare_landing_downloads.sh', destination, success=False)
                self.assertFalse(destination.exists())
                self.assertFalse((self.root / 'landing/resources/json/downloads.json').exists())

    def test_downloads_reject_dirty_or_wrong_vscode_revision_before_building(self):
        destination = self.setup_downloads()
        for variable, value in (('DIRTY_VSCODE', '1'), ('VSCODE_HEAD', 'different-commit')):
            with self.subTest(variable=variable):
                self.environment[variable] = value
                self.run_script('prepare_landing_downloads.sh', destination, success=False)
                self.environment.pop(variable)
                self.assertEqual(self.events(), [])
                self.assertFalse(destination.exists())

    def test_vsix_development_files_prevent_publication(self):
        destination = self.setup_downloads()
        self.environment['VSIX_DEV'] = '1'
        self.run_script('prepare_landing_downloads.sh', destination, success=False)
        self.assertFalse(destination.exists())

    def test_skill_packager_checksum_is_verified_before_copying(self):
        destination = self.setup_downloads()
        self.environment['SKILL_BAD_SHA'] = '1'
        self.run_script('prepare_landing_downloads.sh', destination, success=False)
        self.assertFalse(destination.exists())

    def test_copied_checksum_failure_keeps_previous_download_map(self):
        destination = self.setup_downloads()
        map_path = self.write('landing/resources/json/downloads.json', '{"previous": "release"}\n')
        real_cp = shutil.which('cp')
        self.executable('bin/cp', '#!/bin/sh\n'
                        f'"{real_cp}" "$@" || exit $?\n'
                        'case "$2" in *landing/static/downloads/*.zip) echo corrupt >> "$2";; esac\n')
        self.run_script('prepare_landing_downloads.sh', destination, success=False)
        self.assertEqual(map_path.read_text(), '{"previous": "release"}\n')


if __name__ == '__main__':
    unittest.main()
