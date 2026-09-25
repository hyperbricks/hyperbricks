"""Regression checks for the documentation snapshot bundled with the skill."""

import json
from pathlib import Path, PurePosixPath
import tempfile
import unittest
from unittest.mock import patch

from build_markdown_compilations import BuildError, SourceDocument
from build_skill_documentation import (
    DOCUMENTS_DIRECTORY,
    INDEX_FILENAME,
    MANIFEST_FILENAME,
    build_skill_documentation,
    bundled_document_path,
    ensure_safe_output_directory,
    existing_generated_paths,
    read_managed_manifest,
    remove_stale_outputs,
    render_document,
)


class SkillDocumentationTests(unittest.TestCase):
    def test_bundled_paths_preserve_the_docs_tree(self):
        self.assertEqual(
            bundled_document_path("docs/guides/EXAMPLE.md"),
            PurePosixPath("docs/guides/EXAMPLE.md"),
        )
        with self.assertRaisesRegex(BuildError, "outside docs"):
            bundled_document_path("SKILLS/hyperbricks/SKILL.md")

    def test_document_links_stay_local_and_repository_links_use_release_version(self):
        commit = "a" * 40
        repository_ref = "v1.2.5-beta"
        source = SourceDocument(
            "docs/ONE.md",
            "One",
            (
                "# One\n\n"
                "[Two](TWO.md#details)\n\n"
                "[Module](../modules/demo/README.md)\n\n"
                "![Logo](../assets/logo.svg)\n\n"
                "```markdown\n[Do not rewrite](../modules/demo/README.md)\n```\n"
            ),
            "one",
        )
        output_paths = {
            source.path: PurePosixPath("docs/ONE.md"),
            "docs/TWO.md": PurePosixPath("docs/TWO.md"),
        }
        files = {
            "docs/ONE.md",
            "docs/TWO.md",
            "modules/demo/README.md",
            "assets/logo.svg",
        }
        directories = {"docs", "modules", "modules/demo", "assets"}

        rendered = render_document(
            source,
            output_paths,
            commit,
            files,
            directories,
            repository_ref,
        )

        self.assertIn("[Two](TWO.md#details)", rendered)
        self.assertTrue(
            rendered.startswith(
                "<!-- Generated from docs/ONE.md. Do not edit directly. -->\n"
            )
        )
        self.assertNotIn(commit, rendered)
        self.assertIn(
            f"[Module](https://github.com/hyperbricks/hyperbricks/blob/{repository_ref}/modules/demo/README.md)",
            rendered,
        )
        self.assertIn(
            f"![Logo](https://raw.githubusercontent.com/hyperbricks/hyperbricks/{repository_ref}/assets/logo.svg)",
            rendered,
        )
        self.assertIn(
            "```markdown\n[Do not rewrite](../modules/demo/README.md)\n```",
            rendered,
        )

    def test_missing_repository_link_target_fails_the_build(self):
        source = SourceDocument(
            "docs/ONE.md",
            "One",
            "# One\n\n[Missing](../missing.md)\n",
            "one",
        )
        with self.assertRaisesRegex(BuildError, "link target is missing"):
            render_document(
                source,
                {source.path: PurePosixPath("docs/ONE.md")},
                "b" * 40,
                {source.path},
                {"docs"},
            )

    def test_snapshot_contains_index_manifest_and_separate_documents(self):
        source = SourceDocument(
            "docs/INTRODUCTION.md",
            "Introduction",
            "# Introduction\n\nHello.\n",
            "intro",
            "Start",
            "Introduction",
        )
        with patch(
            "build_skill_documentation.assembly.snapshot_version",
            return_value="v1.2.5-beta",
        ), patch(
            "build_skill_documentation.assembly.documentation_paths",
            return_value=(source.path,),
        ), patch(
            "build_skill_documentation.assembly.collect_sources",
            return_value=(source,),
        ), patch(
            "build_skill_documentation.snapshot_repository_paths",
            return_value=({source.path}, {"docs"}),
        ):
            outputs = build_skill_documentation(
                Path("."),
                "c" * 40,
            )
        self.assertIn(
            "**Source version:** [`v1.2.5-beta`](https://github.com/hyperbricks/hyperbricks/tree/v1.2.5-beta)",
            outputs[PurePosixPath(INDEX_FILENAME)],
        )

        self.assertEqual(
            set(outputs),
            {
                PurePosixPath(INDEX_FILENAME),
                PurePosixPath(MANIFEST_FILENAME),
                PurePosixPath("docs/INTRODUCTION.md"),
            },
        )
        self.assertIn("[Introduction](docs/INTRODUCTION.md)", outputs[PurePosixPath(INDEX_FILENAME)])
        self.assertIn(
            "**Snapshot provenance:** [Manifest](documentation-manifest.json)",
            outputs[PurePosixPath(INDEX_FILENAME)],
        )
        self.assertNotIn("Snapshot date", outputs[PurePosixPath(INDEX_FILENAME)])
        manifest = json.loads(outputs[PurePosixPath(MANIFEST_FILENAME)])
        self.assertEqual(manifest["schema_version"], 2)
        self.assertEqual(manifest["hyperbricks_version"], "v1.2.5-beta")
        self.assertRegex(manifest["source_digest"], r"^[0-9a-f]{64}$")
        self.assertNotIn("source_commit", manifest)
        self.assertEqual(
            manifest["documents"][0]["source_path"],
            "docs/INTRODUCTION.md",
        )

    def test_stale_generated_documents_are_detected_and_removed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            docs = root / DOCUMENTS_DIRECTORY
            docs.mkdir()
            (docs / "CURRENT.md").write_text("current", encoding="utf-8")
            (docs / "STALE.md").write_text("stale", encoding="utf-8")
            (root / INDEX_FILENAME).write_text("index", encoding="utf-8")

            self.assertEqual(
                existing_generated_paths(root),
                {
                    PurePosixPath(INDEX_FILENAME),
                    PurePosixPath("docs/CURRENT.md"),
                    PurePosixPath("docs/STALE.md"),
                },
            )
            expected = {
                PurePosixPath(INDEX_FILENAME),
                PurePosixPath("docs/CURRENT.md"),
            }
            remove_stale_outputs(root, expected)
            self.assertTrue((docs / "CURRENT.md").is_file())
            self.assertFalse((docs / "STALE.md").exists())

    def test_output_directory_rejects_repository_and_unmanaged_content(self):
        with tempfile.TemporaryDirectory() as directory:
            repository = Path(directory) / "repository"
            repository.mkdir()
            (repository / "docs").mkdir()

            with self.assertRaisesRegex(BuildError, "repository root"):
                ensure_safe_output_directory(repository, repository)
            with self.assertRaisesRegex(BuildError, "canonical docs"):
                ensure_safe_output_directory(repository, repository / "docs")

            unmanaged = Path(directory) / "unmanaged"
            unmanaged.mkdir()
            (unmanaged / "keep.txt").write_text("keep", encoding="utf-8")
            with self.assertRaisesRegex(BuildError, "non-empty directory"):
                ensure_safe_output_directory(repository, unmanaged)

    def test_output_directory_rejects_symlinked_generated_tree(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repository = root / "repository"
            repository.mkdir()
            (repository / "docs").mkdir()
            output = root / "snapshot"
            output.mkdir()
            outside = root / "outside"
            outside.mkdir()
            (output / DOCUMENTS_DIRECTORY).symlink_to(
                outside,
                target_is_directory=True,
            )

            with self.assertRaisesRegex(BuildError, "symlinks"):
                ensure_safe_output_directory(repository, output)

    def test_managed_output_accepts_legacy_manifest_for_regeneration(self):
        with tempfile.TemporaryDirectory() as directory:
            repository = Path(directory) / "repository"
            repository.mkdir()
            (repository / "docs").mkdir()
            output = Path(directory) / "snapshot"
            output.mkdir()
            commit = "d" * 40
            (output / MANIFEST_FILENAME).write_text(
                json.dumps(
                    {
                        "schema_version": 1,
                        "source_repository": "https://github.com/hyperbricks/hyperbricks",
                        "source_commit": commit,
                        "documents": [],
                    }
                ),
                encoding="utf-8",
            )

            ensure_safe_output_directory(repository, output)
            self.assertEqual(read_managed_manifest(output)["source_commit"], commit)


if __name__ == "__main__":
    unittest.main()
