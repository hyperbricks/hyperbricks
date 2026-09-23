"""Compilation structure, PDF layout, and content regression checks."""

from datetime import date
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch

from pypdf import PdfReader

import build_compilations
from build_markdown_compilations import (
    BuildError,
    Compilation,
    CoverTexts,
    DOCUMENTATION_SECTIONS,
    GENERATED_SKILL_COMPILATIONS,
    GENERATED_SKILL_DOCUMENTATION,
    GENERATED_SKILL_DOCUMENTATION_PREFIX,
    SKILLS_ORDER,
    SourceDocument,
    build_compilations as assemble_compilations,
    documentation_paths,
    format_text,
    load_compilation_texts,
    render_compilation,
    repository_link_ref,
    skill_source_paths,
    source_heading_entries,
    snapshot_version,
    transform_source,
)


class CompilationTests(unittest.TestCase):
    def test_version_is_read_from_selected_snapshot(self):
        with patch("build_markdown_compilations.run_git", return_value="v1.2.4-beta\n") as git:
            self.assertEqual(snapshot_version(Path("."), "old-commit"), "v1.2.4-beta")
            git.assert_called_once_with(Path("."), "show", "old-commit:assets/version.md")

    def test_invalid_snapshot_version_fails(self):
        for value in ("\n", "v1.2.4-beta\nv1.2.5-beta"):
            with patch("build_markdown_compilations.run_git", return_value=value):
                with self.assertRaises(BuildError):
                    snapshot_version(Path("."), "old-commit")

    def test_compilation_includes_version(self):
        compilation = Compilation("test.md", "Test Compilation v1.2.5-beta", "Source documents", "Description", "Topics", (), "v1.2.5-beta")
        markdown = render_compilation(compilation, "a" * 40, date(2026, 9, 22))
        self.assertIn("# Test Compilation v1.2.5-beta", markdown)
        self.assertIn("- **HyperBricks version:** v1.2.5-beta", markdown)

    def test_generated_copy_comes_from_text_file(self):
        def paths(_repository, _commit, directory):
            if directory == "docs":
                return ("docs/INTRODUCTION.md",)
            return tuple(SKILLS_ORDER)

        with patch("build_markdown_compilations.snapshot_version", return_value="v1.2.5-beta"), \
             patch("build_markdown_compilations.committed_paths", side_effect=paths), \
             patch("build_markdown_compilations.committed_text", return_value="# Source\n"):
            documentation, skills = assemble_compilations(Path("."), "a" * 40)
        texts = load_compilation_texts()
        values = {
            "version": "v1.2.5-beta",
            "document_count": 1,
            "short_commit": "aaaaaaa",
        }
        self.assertEqual(
            documentation.title,
            format_text(texts["documentation"]["title"], values, "documentation.title"),
        )
        values["document_count"] = len(SKILLS_ORDER)
        self.assertEqual(
            skills.title,
            format_text(texts["skills"]["title"], values, "skills.title"),
        )
        self.assertEqual(documentation.kind, "documentation")
        self.assertEqual(skills.kind, "skills")
        self.assertEqual(skills.repository_ref, "v1.2.5-beta")
        self.assertEqual(documentation.cover.heading, texts["documentation"]["cover"]["heading"])
        self.assertEqual(skills.cover.detail, texts["skills"]["cover"]["detail"])
        self.assertNotIn("Handbook", skills.title)

    def test_repository_links_use_release_version(self):
        self.assertEqual(repository_link_ref("v1.2.5-beta"), "v1.2.5-beta")
        with self.assertRaisesRegex(BuildError, "GitHub-safe release tag"):
            repository_link_ref("release/v1.2.5-beta")

    def test_text_file_rejects_unknown_placeholders(self):
        source = Path(__file__).with_name("compilation-texts.json").read_text(encoding="utf-8")
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "texts.json"
            path.write_text(source.replace("{version}", "{unknown}", 1), encoding="utf-8")
            with self.assertRaisesRegex(BuildError, "unsupported placeholder"):
                load_compilation_texts(path)

    def test_text_file_rejects_duplicate_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "texts.json"
            path.write_text('{"documentation": {}, "documentation": {}}', encoding="utf-8")
            with self.assertRaisesRegex(BuildError, "duplicate key"):
                load_compilation_texts(path)

    def test_documentation_sections_match_readme_navigation(self):
        self.assertEqual(DOCUMENTATION_SECTIONS, (
            ("Start", (("Introduction", "docs/INTRODUCTION.md"), ("Quickstart", "docs/QUICKSTART.md"), ("How-to guides", "docs/HOWTOS.md"), ("Troubleshooting", "docs/TROUBLESHOOTING.md"))),
            ("Application model", (("Routing", "docs/ROUTING.md"), ("Component reference", "docs/REFERENCE.md"), ("Markdown", "docs/MARKDOWN.md"), ("Spaces CMS", "docs/SPACES.md"), ("Authoring", "docs/AUTHOR.md"))),
            ("Logic and assets", (("API Render", "docs/API_RENDER.md"), ("Server Scripts", "docs/GOJA_RENDER.md"), ("Plugins", "docs/PLUGINS.md"), ("JavaScript and CSS", "docs/ESBUILD.md"))),
            ("Delivery", (("Deploy Guide", "docs/DEPLOY.md"), ("Docker Deploy", "docs/DOCKER.md"), ("Migration Guide", "docs/MIGRATION.md"))),
        ))

    def test_grouped_contents_are_unnumbered_and_keep_source_titles(self):
        sources = (
            SourceDocument("docs/INTRODUCTION.md", "Introduction", "# Introduction\n", "intro", "Start", "Introduction"),
            SourceDocument("docs/GOJA_RENDER.md", "Goja Render", "# Goja Render\n", "goja", "Logic and assets", "Server Scripts"),
            SourceDocument("docs/OTHER.md", "Other Reference", "# Other Reference\n", "other", "Additional documents"),
        )
        compilation = Compilation("test.md", "HyperBricks Documentation Compilation v1", "Source documents", "Separate documents.", "Topics", sources, "v1")
        markdown = render_compilation(compilation, "a" * 40, date(2026, 9, 22))
        self.assertIn("### Start\n\n- [Introduction](#intro)", markdown)
        self.assertIn("### Logic and assets\n\n- [Server Scripts](#goja)", markdown)
        self.assertIn("### Additional documents\n\n- [Other Reference](#other)", markdown)
        self.assertNotRegex(markdown, r"(?m)^\d+\. \[")
        self.assertIn("## Goja Render", markdown)
        self.assertNotIn("## 02. Goja Render", markdown)

    def test_skills_contents_use_skill_sections(self):
        source = SourceDocument(
            "SKILLS/hyperbricks/SKILL.md",
            "HyperBricks",
            "# HyperBricks\n\n## Working principles\n\nText.\n\n"
            "### Detail\n\nMore.\n\n## Execute the task\n",
            "hyperbricks",
        )
        compilation = Compilation(
            "test.md",
            "HyperBricks Skills Compilation",
            "Skill sources",
            "Description",
            "Topics",
            (source,),
            kind="skills",
        )
        markdown = render_compilation(compilation, "a" * 40, date(2026, 9, 22))
        self.assertIn("- [Working principles](#hyperbricks--working-principles)", markdown)
        self.assertIn("- [Execute the task](#hyperbricks--execute-the-task)", markdown)
        self.assertNotIn("- [HyperBricks](#hyperbricks)", markdown)
        self.assertEqual(
            source_heading_entries(source, 2),
            (
                ("Working principles", "hyperbricks--working-principles"),
                ("Execute the task", "hyperbricks--execute-the-task"),
            ),
        )

    def test_skills_pdf_contents_use_sections_without_instructions(self):
        source = SourceDocument(
            "SKILLS/hyperbricks/SKILL.md",
            "HyperBricks",
            "# HyperBricks\n\n## Working principles\n\nText.\n\n## Execute the task\n\nMore.\n",
            "hyperbricks",
        )
        compilation = Compilation(
            "test.md",
            "HyperBricks Skills Compilation",
            "Skill sources",
            "Description",
            "Topics",
            (source,),
            kind="skills",
        )
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_compilations.render_pdf(
                compilation,
                "a" * 40,
                date(2026, 9, 22),
                destination,
                Path("/System/Library/Fonts/Supplemental"),
                Path("/System/Library/Fonts/Menlo.ttc"),
            )
            reader = PdfReader(destination)
            contents = reader.pages[1].extract_text()
            self.assertIn("Working principles", contents)
            self.assertIn("Execute the task", contents)
            self.assertNotIn("Select a title", contents)
            self.assertNotIn("Each entry is a separate source document", contents)

    def test_mermaid_print_colors_preserve_graph_and_stroke_width(self):
        source = "flowchart TB\n A --> B\n classDef node fill:#ffffff00,stroke:#ffffff,color:#ffffff,stroke-width:2.5px;\n class A,B node;"
        result = build_compilations.print_mermaid_source(source)
        self.assertIn("A --> B", result)
        self.assertIn("class A,B node;", result)
        self.assertIn("stroke-width:2.5px", result)
        self.assertIn("fill:#f0f5f8,stroke:#087f8c,color:#123044", result)
        self.assertNotIn("#ffffff", result)

    def test_missing_mermaid_renderer_fails_explicitly(self):
        with self.assertRaisesRegex(BuildError, "Mermaid CLI is missing"):
            build_compilations.render_mermaid("flowchart TB\n A --> B", "/missing/mmdc")

    def test_mermaid_embeds_image_instead_of_source(self):
        from io import BytesIO
        from PIL import Image

        data = BytesIO()
        Image.new("RGB", (300, 200), "white").save(data, format="PNG")
        source = SourceDocument("docs/test.md", "Diagram", "# Diagram\n\n```mermaid\nflowchart TB\n A --> B\n```", "diagram")
        compilation = Compilation("test.md", "HyperBricks Documentation Compilation v1", "Source documents", "Test document", "Diagrams.", (source,))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            with patch("build_compilations.render_mermaid", return_value=data.getvalue()) as renderer:
                build_compilations.render_pdf(compilation, "a" * 40, date(2026, 9, 16), destination,
                                          Path("/System/Library/Fonts/Supplemental"), Path("/System/Library/Fonts/Menlo.ttc"), "/test/mmdc")
                renderer.assert_called_once()
            reader = PdfReader(destination)
            self.assertNotIn("flowchart TB", "\n".join(page.extract_text() for page in reader.pages))
            self.assertTrue(any(page.images for page in reader.pages))

    def test_pdf_uses_compilation_title_and_grouped_outline(self):
        sources = (
            SourceDocument("docs/INTRODUCTION.md", "Introduction", "# Introduction\n\nStart here.\n", "intro", "Start", "Introduction"),
            SourceDocument("docs/DEPLOY.md", "Deploy", "# Deploy\n\nShip it.\n", "deploy", "Delivery", "Deploy Guide"),
        )
        compilation = Compilation(
            "test.md",
            "HyperBricks Documentation Compilation v1.2.5-beta",
            "Source documents",
            "Separate documents.",
            "Topics",
            sources,
            "v1.2.5-beta",
            CoverTexts(
                label="CUSTOM DOCUMENTATION LABEL",
                brand="Custom HyperBricks",
                heading="Custom compilation",
                detail="Release {version}",
                summary="{document_count} documents • {snapshot_date}",
                source="Commit {short_commit}",
            ),
        )
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_compilations.render_pdf(
                compilation,
                "a" * 40,
                date(2026, 9, 16),
                destination,
                Path("/System/Library/Fonts/Supplemental"),
                Path("/System/Library/Fonts/Menlo.ttc"),
            )
            reader = PdfReader(destination)
            self.assertEqual(reader.metadata.title, compilation.title)
            cover = reader.pages[0].extract_text()
            self.assertIn("CUSTOM DOCUMENTATION LABEL", cover)
            self.assertIn("Custom compilation", cover)
            self.assertIn("Release v1.2.5-beta", cover)
            self.assertIn("2 documents", cover)
            self.assertIn("Commit aaaaaaa", cover)
            self.assertNotIn("handbook", cover.lower())
            self.assertEqual(reader.outline[0].title, "Start")
            self.assertEqual(reader.outline[1][0].title, "Introduction")
            self.assertEqual(reader.outline[2].title, "Delivery")
            self.assertEqual(reader.outline[3][0].title, "Deploy Guide")

    def test_generated_compilations_are_not_documentation_sources(self):
        paths = ("docs/INTRODUCTION.md", "docs/MARKDOWN.md",
                 "docs/compilations/HyperBricks-Documentation.md",
                 "docs/compilations/HyperBricks-Skills.md")
        with patch("build_markdown_compilations.committed_paths", return_value=paths):
            self.assertEqual(documentation_paths(Path("."), "a" * 40),
                             ("docs/INTRODUCTION.md", "docs/MARKDOWN.md"))

    def test_documentation_paths_put_unknown_documents_last(self):
        paths = (
            "docs/Z_EXTRA.md",
            "docs/ESBUILD.md",
            "docs/INTRODUCTION.md",
            "docs/A_EXTRA.md",
            "docs/compilations/HyperBricks-Documentation.md",
        )
        with patch("build_markdown_compilations.committed_paths", return_value=paths):
            self.assertEqual(
                documentation_paths(Path("."), "a" * 40),
                ("docs/INTRODUCTION.md", "docs/ESBUILD.md", "docs/A_EXTRA.md", "docs/Z_EXTRA.md"),
            )

    def test_generated_compilations_are_not_skills_sources(self):
        extra = "SKILLS/hyperbricks/references/extra.md"
        generated_document = f"{GENERATED_SKILL_DOCUMENTATION_PREFIX}INTRODUCTION.md"
        paths = (
            set(SKILLS_ORDER)
            | GENERATED_SKILL_COMPILATIONS
            | GENERATED_SKILL_DOCUMENTATION
            | {generated_document, extra}
        )
        selected = skill_source_paths(paths)
        self.assertIn(extra, selected)
        self.assertTrue(GENERATED_SKILL_COMPILATIONS.isdisjoint(selected))
        self.assertTrue(GENERATED_SKILL_DOCUMENTATION.isdisjoint(selected))
        self.assertNotIn(generated_document, selected)

    def test_explicit_anchors_resolve_with_accents_and_across_chapters(self):
        anchor = "run-the-night-owl-cafe-example"
        text = (f'# Caf\u00e9 guide\n\n[Jump](#{anchor})\n\n'
                f'<a id="{anchor}"></a>\n\n## Run the Night Owl Caf\u00e9 example\n\nCAFE-TARGET\n\n'
                f'```html\n<a id="{anchor}"></a>\n```\n')
        other_text = (f'# Another guide\n\n[Own section](#{anchor}) and '
                      f'[Caf\u00e9 section](cafe.md#{anchor}).\n\n'
                      f'<a id="{anchor}"></a>\n\n## Another example\n\nOTHER-TARGET\n')
        source = SourceDocument("docs/cafe.md", "Caf\u00e9 guide", text, "cafe-guide")
        other = SourceDocument("docs/other.md", "Another guide", other_text, "other-guide")
        compilation = Compilation("test.md", "HyperBricks Documentation Compilation v1", "Source documents",
                                  "Test document", "Explicit anchors.", (source, other))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_compilations.render_pdf(compilation, "a" * 40, date(2026, 9, 16), destination,
                                      Path("/System/Library/Fonts/Supplemental"), Path("/System/Library/Fonts/Menlo.ttc"))
            reader = PdfReader(destination)
            target_page = next(page for page in reader.pages if "CAFE-TARGET" in page.extract_text())
            other_page = next(page for page in reader.pages if "OTHER-TARGET" in page.extract_text())
            destinations = [annotation.get_object()["/Dest"][0]
                            for annotation in other_page["/Annots"]]
            self.assertIn(target_page.indirect_reference, destinations)
            self.assertIn(other_page.indirect_reference, destinations)
        transformed = transform_source(source, {source.path: source, other.path: other}, "a" * 40, False)
        self.assertIn(f'<a id="cafe-guide--{anchor}"></a>', transformed)
        self.assertIn(f'```html\n<a id="{anchor}"></a>\n```', transformed)

    def test_missing_fonts_fail_without_replacing_output(self):
        compilation = Compilation("test.md", "HyperBricks Documentation Compilation v1", "Source documents", "Test document", "Test.", ())
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            destination.write_bytes(b"existing output")
            with self.assertRaisesRegex(BuildError, "Required font missing"):
                build_compilations.render_pdf(compilation, "a" * 40, date(2026, 9, 16), destination,
                                          Path(directory) / "missing", Path(directory) / "missing-code.ttf")
            self.assertEqual(destination.read_bytes(), b"existing output")

    def test_code_highlighting_preserves_text_and_unknown_languages(self):
        code = ('# Highlighting\n\n```yaml\n# YAML comment\nenabled: true\n'
                'count: 42\nmessage: "Hello"\n```\n\n'
                '```go\nfunc main() { fmt.Println("Hello") }\n```\n\n'
                '```html\n<p class="intro">Hello</p>\n```\n\n'
                '```javascript\nconst count = 42;\n```\n\n'
                '```bash\necho "Hello"\n```\n\n'
                '```unknown-language\nopaque: <visitor>\n```\n')
        source = SourceDocument("docs/test.md", "Highlighting", code, "highlighting")
        compilation = Compilation("test.md", "HyperBricks Documentation Compilation v1", "Source documents",
                                  "Test document", "Syntax highlighting.", (source,))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_compilations.render_pdf(compilation, "a" * 40, date(2026, 9, 16), destination,
                                      Path("/System/Library/Fonts/Supplemental"), Path("/System/Library/Fonts/Menlo.ttc"))
            reader = PdfReader(destination)
            content = "\n".join(page.extract_text() for page in reader.pages)
            for line in ('enabled: true', 'count: 42', 'message: "Hello"',
                         'func main() { fmt.Println("Hello") }', '<p class="intro">Hello</p>',
                         'const count = 42;', 'echo "Hello"', 'opaque: <visitor>'):
                self.assertIn(line, content)
            token_colors = {}
            for page in reader.pages:
                color = None
                for arguments, operator in page.get_contents().operations:
                    if operator == b"rg":
                        color = tuple(arguments)
                    elif operator == b"Tj" and str(arguments[0]) in {'enabled', 'true', '42', '# YAML comment'}:
                        token_colors[str(arguments[0])] = color
            self.assertEqual(len(set(token_colors.values())), 4)
            self.assertNotIn("Continues on next page", content)
            self.assertNotIn("Continued from previous page", content)

    def test_long_code_tables_links_and_outline(self):
        text = "# Test chapter\n\n## Target\n\n[Jump](#target)\n\n"
        text += "```yaml\n" + "\n".join(f"field_{i}: " + "x" * 150 for i in range(90)) + "\n```\n\n"
        text += "| Field | Description |\n| --- | --- |\n"
        text += "\n".join(f"| key_{i} | Description {i} |" for i in range(90))
        text += "\n\nEND-OF-CHAPTER\n"
        source = SourceDocument("docs/test.md", "Test chapter", text, "test-chapter")
        compilation = Compilation("test.md", "HyperBricks Documentation Compilation v1", "Source documents", "Test document", "Pagination and links.", (source,))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_compilations.render_pdf(compilation, "a" * 40, date(2026, 9, 16), destination,
                                      Path("/System/Library/Fonts/Supplemental"), Path("/System/Library/Fonts/Menlo.ttc"))
            reader = PdfReader(destination)
            content = "\n".join(page.extract_text() for page in reader.pages)
            self.assertIn("field_89", content)
            self.assertIn("Description 89", content)
            self.assertIn("END-OF-CHAPTER", content)
            self.assertEqual(set(re.findall(r"field_(\d+):", content)), {str(i) for i in range(90)})
            self.assertIn("Continues on next page", content)
            self.assertEqual(content.count("Continues on next page"),
                             content.count("Continued from previous page"))
            for index, page in enumerate(reader.pages):
                if "Continues on next page" in page.extract_text():
                    self.assertIn("Continued from previous page", reader.pages[index + 1].extract_text())
            self.assertEqual(len(reader.outline), 1)
            self.assertGreater(len(reader.pages), 5)
            self.assertTrue(any(page.get("/Annots") for page in reader.pages))


if __name__ == "__main__":
    unittest.main()
