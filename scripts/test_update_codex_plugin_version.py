from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("update_codex_plugin_version.py")
SPEC = importlib.util.spec_from_file_location("update_codex_plugin_version", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class RefreshVersionTests(unittest.TestCase):
    def test_advances_legacy_codex_build_version(self):
        self.assertEqual(
            MODULE.refreshed_version("0.1.1+codex.20260923164609"),
            "0.1.2",
        )

    def test_advances_plain_patch_version(self):
        self.assertEqual(MODULE.refreshed_version("0.1.2"), "0.1.3")

    def test_removes_prerelease_and_build_metadata(self):
        self.assertEqual(
            MODULE.refreshed_version("1.2.0-beta.2+local.7"),
            "1.2.1",
        )

    def test_rejects_invalid_version(self):
        with self.assertRaises(MODULE.VersionError):
            MODULE.refreshed_version("release-1")

    def test_rejects_noncanonical_numeric_version(self):
        with self.assertRaises(MODULE.VersionError):
            MODULE.refreshed_version("0.01.1")

    def test_updates_only_version_and_is_deterministic(self):
        with tempfile.TemporaryDirectory() as directory:
            manifest = Path(directory) / "plugin.json"
            manifest.write_text(
                json.dumps(
                    {
                        "name": "hyperbricks",
                        "version": "0.1.1+codex.20260923164609",
                        "description": "Test plugin",
                    },
                    indent=2,
                )
                + "\n",
                encoding="utf-8",
            )

            previous, updated = MODULE.update_manifest(manifest)

            self.assertEqual(previous, "0.1.1+codex.20260923164609")
            self.assertEqual(updated, "0.1.2")
            self.assertEqual(
                json.loads(manifest.read_text(encoding="utf-8")),
                {
                    "name": "hyperbricks",
                    "version": "0.1.2",
                    "description": "Test plugin",
                },
            )

    def test_detects_changed_file_content(self):
        with tempfile.TemporaryDirectory() as directory:
            before = Path(directory) / "before"
            after = Path(directory) / "after"
            before.mkdir()
            after.mkdir()
            (before / "SKILL.md").write_text("old\n", encoding="utf-8")
            (after / "SKILL.md").write_text("new\n", encoding="utf-8")

            self.assertTrue(MODULE.skill_changed(before, after))

    def test_identical_skill_trees_do_not_report_a_change(self):
        with tempfile.TemporaryDirectory() as directory:
            before = Path(directory) / "before"
            after = Path(directory) / "after"
            (before / "references").mkdir(parents=True)
            (after / "references").mkdir(parents=True)
            (before / "references" / "index.md").write_text(
                "same\n", encoding="utf-8"
            )
            (after / "references" / "index.md").write_text(
                "same\n", encoding="utf-8"
            )

            self.assertFalse(MODULE.skill_changed(before, after))


if __name__ == "__main__":
    unittest.main()
