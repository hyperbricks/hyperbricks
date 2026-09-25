"""Content-stability checks against disposable Git snapshots and worktrees."""

from contextlib import redirect_stderr, redirect_stdout
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import build_skill_documentation as skill_docs


class SkillDocumentationStabilityTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.repository = self.root / "repository"
        self.repository.mkdir()
        self.output = self.root / "snapshot"
        self.git_environment = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith("GIT_")
        }
        self.git_environment.update(
            {
                "GIT_CONFIG_NOSYSTEM": "1",
                "GIT_CONFIG_GLOBAL": os.devnull,
                "GIT_AUTHOR_NAME": "Documentation Test",
                "GIT_AUTHOR_EMAIL": "documentation-test@example.invalid",
                "GIT_COMMITTER_NAME": "Documentation Test",
                "GIT_COMMITTER_EMAIL": "documentation-test@example.invalid",
            }
        )
        self.git("init", "--quiet")
        self.write_source("assets/version.md", "v1.2.5-beta\n")
        self.write_source("assets/logo.svg", "<svg/>\n")
        self.write_source("modules/demo/README.md", "# Demo module\n")
        self.write_source(
            "docs/INTRODUCTION.md",
            "# Introduction\n\n"
            "The canonical introduction.\n\n"
            "[How-to](HOWTOS.md#usage)\n\n"
            "[Module](../modules/demo/README.md)\n\n"
            "![Logo](../assets/logo.svg)\n",
        )
        self.write_source("docs/HOWTOS.md", "# How-to guides\n\n## Usage\n")
        self.write_source("docs/EXTRA.md", "# Extra documentation\n")
        self.original_commit = self.commit("Initial documents", "2026-09-24")

    def git(self, *arguments, environment=None):
        result = subprocess.run(
            ["git", *arguments],
            cwd=self.repository,
            env=environment or self.git_environment,
            text=True,
            capture_output=True,
            check=True,
        )
        return result.stdout.strip()

    def write_source(self, relative, content):
        destination = self.repository / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(content, encoding="utf-8")

    def commit(self, message, day="2026-09-25"):
        self.git("add", "--all")
        environment = dict(self.git_environment)
        environment.update(
            {
                "GIT_AUTHOR_DATE": f"{day}T10:00:00+00:00",
                "GIT_COMMITTER_DATE": f"{day}T10:00:00+00:00",
            }
        )
        self.git(
            "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", message,
            environment=environment,
        )
        return self.git("rev-parse", "HEAD")

    def invoke(self, *arguments):
        stdout = io.StringIO()
        stderr = io.StringIO()
        with patch(
            "build_skill_documentation.assembly.repository_root",
            return_value=self.repository,
        ), patch(
            "sys.argv",
            ["build_skill_documentation.py", "--output-dir", str(self.output), *arguments],
        ), patch.dict(os.environ, self.git_environment, clear=True), \
                redirect_stdout(stdout), redirect_stderr(stderr):
            status = skill_docs.main()
        return status, stdout.getvalue(), stderr.getvalue()

    def generate(self):
        status, stdout, stderr = self.invoke()
        self.assertEqual(status, 0, stdout + stderr)

    def assert_check_passes(self, *arguments):
        before = self.snapshot_bytes()
        status, stdout, stderr = self.invoke("--check", *arguments)
        self.assertEqual(status, 0, stdout + stderr)
        self.assertEqual(self.snapshot_bytes(), before, "--check must not write files")

    def assert_check_fails(self, *arguments):
        before = self.snapshot_bytes()
        status, stdout, stderr = self.invoke("--check", *arguments)
        self.assertNotEqual(status, 0, stdout + stderr)
        self.assertEqual(self.snapshot_bytes(), before, "--check must not write files")

    def snapshot_bytes(self):
        return {
            path.relative_to(self.output).as_posix(): path.read_bytes()
            for path in self.output.rglob("*")
            if path.is_file()
        }

    def manifest(self):
        return json.loads(
            (self.output / skill_docs.MANIFEST_FILENAME).read_text(encoding="utf-8")
        )

    def write_manifest(self, manifest):
        (self.output / skill_docs.MANIFEST_FILENAME).write_text(
            json.dumps(manifest, indent=2) + "\n", encoding="utf-8"
        )

    def unrelated_commit(self):
        self.write_source("unrelated.txt", "An unrelated implementation change.\n")
        return self.commit("Unrelated change on a later day")

    def test_regeneration_is_identical_after_an_unrelated_commit(self):
        self.generate()
        before = self.snapshot_bytes()
        later_commit = self.unrelated_commit()
        self.generate()
        after = self.snapshot_bytes()
        self.assertEqual(before, after)
        self.assertEqual(self.manifest()["schema_version"], 2)
        self.assertNotIn("source_commit", self.manifest())

        document = after["docs/INTRODUCTION.md"].decode("utf-8")
        self.assertTrue(document.startswith(
            "<!-- Generated from docs/INTRODUCTION.md. Do not edit directly. -->\n"
        ))
        self.assertIn("[How-to](HOWTOS.md#usage)", document)
        self.assertIn(
            "https://github.com/hyperbricks/hyperbricks/blob/"
            "v1.2.5-beta/modules/demo/README.md",
            document,
        )
        self.assertIn(
            "https://raw.githubusercontent.com/hyperbricks/hyperbricks/"
            "v1.2.5-beta/assets/logo.svg",
            document,
        )
        self.assertNotIn(self.original_commit, document)
        self.assertNotIn(later_commit, document)
        index = after[skill_docs.INDEX_FILENAME].decode("utf-8")
        self.assertNotIn("Snapshot date", index)
        self.assertIn(skill_docs.MANIFEST_FILENAME, index)

    def test_explicit_ref_accepts_unchanged_content(self):
        self.generate()
        self.unrelated_commit()
        self.assert_check_passes("--ref", "HEAD")
        self.assert_check_passes()

    def test_default_check_detects_uncommitted_canonical_changes(self):
        self.generate()
        self.write_source("docs/EXTRA.md", "# Extra documentation\n\nNew prose.\n")
        self.assert_check_fails()
        self.assert_check_passes("--ref", "HEAD")
        self.generate()
        self.assert_check_passes()
        self.assert_check_fails("--ref", "HEAD")

    def test_explicit_ref_detects_added_source_document(self):
        self.generate()
        self.write_source("docs/NEW.md", "# New documentation\n")
        self.commit("Add canonical document")
        self.assert_check_fails("--ref", "HEAD")

    def test_explicit_ref_detects_removed_source_document(self):
        self.generate()
        (self.repository / "docs/EXTRA.md").unlink()
        self.commit("Remove canonical document")
        self.assert_check_fails("--ref", "HEAD")

    def test_explicit_ref_detects_version_change_and_rewrites_repository_links(self):
        self.generate()
        self.write_source("assets/version.md", "v1.2.6-beta\n")
        self.commit("Bump runtime version")
        self.assert_check_fails("--ref", "HEAD")
        self.generate()
        document = (self.output / "docs/INTRODUCTION.md").read_text(encoding="utf-8")
        self.assertIn("/blob/v1.2.6-beta/modules/demo/README.md", document)
        self.assertNotIn("/blob/v1.2.5-beta/", document)
        self.assert_check_passes("--ref", "HEAD")

    def test_explicit_ref_rejects_missing_repository_link_target(self):
        self.generate()
        (self.repository / "modules/demo/README.md").unlink()
        self.commit("Remove linked repository file")
        self.assert_check_fails("--ref", "HEAD")
        status, stdout, stderr = self.invoke()
        self.assertNotEqual(status, 0, stdout + stderr)
        self.assertIn("link target is missing", stderr)

    def test_checks_reject_tampered_bundled_document(self):
        self.generate()
        self.unrelated_commit()
        (self.output / "docs/EXTRA.md").write_text("Tampered content.\n", encoding="utf-8")
        self.assert_check_fails()
        self.assert_check_fails("--ref", "HEAD")

    def test_checks_reject_tampered_manifest_document_hashes(self):
        self.generate()
        self.unrelated_commit()
        original_manifest = self.manifest()
        for field in ("source_sha256", "bundled_sha256"):
            with self.subTest(field=field):
                manifest = json.loads(json.dumps(original_manifest))
                manifest["documents"][0][field] = "0" * 64
                self.write_manifest(manifest)
                self.assert_check_fails()
                self.assert_check_fails("--ref", "HEAD")

    def test_checks_reject_tampered_recorded_provenance(self):
        self.generate()
        self.unrelated_commit()
        original_manifest = self.manifest()
        for field, value in (
            ("source_digest", "0" * 64),
            ("source_digest", "not-a-digest"),
            ("schema_version", 1),
        ):
            with self.subTest(field=field, value=value):
                manifest = dict(original_manifest)
                manifest[field] = value
                self.write_manifest(manifest)
                self.assert_check_fails()
                self.assert_check_fails("--ref", "HEAD")

    def test_check_survives_rewritten_commit_history(self):
        self.generate()
        self.write_source("unrelated.txt", "Changed outside the documentation.\n")
        self.commit("Unrelated change")
        self.git("-c", "commit.gpgsign=false", "commit", "--amend", "--no-edit", "--quiet")
        self.assert_check_passes()

    def test_checks_reject_unexpected_generated_document(self):
        self.generate()
        self.unrelated_commit()
        (self.output / "docs/STALE.md").write_text("# Stale document\n", encoding="utf-8")
        self.assert_check_fails()
        self.assert_check_fails("--ref", "HEAD")


if __name__ == "__main__":
    unittest.main()
