"""PDF pagination and content regression checks; run with the handbook environment."""

from datetime import date
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch

from pypdf import PdfReader

import build_handbooks
from build_markdown_handbooks import BuildError, Handbook, SourceDocument, documentation_paths, transform_source, snapshot_version, render_handbook


class HandbookTests(unittest.TestCase):
    def test_version_is_read_from_selected_snapshot(self):
        with patch("build_markdown_handbooks.run_git", return_value="v1.2.4-beta\n") as git:
            self.assertEqual(snapshot_version(Path("."), "old-commit"), "v1.2.4-beta")
            git.assert_called_once_with(Path("."), "show", "old-commit:assets/version.md")

    def test_invalid_snapshot_version_fails(self):
        for value in ("\n", "v1.2.4-beta\nv1.2.5-beta"):
            with patch("build_markdown_handbooks.run_git", return_value=value):
                with self.assertRaises(BuildError):
                    snapshot_version(Path("."), "old-commit")

    def test_handbook_includes_version(self):
        handbook = Handbook("test.md", "Test", "Handbook", "Description", "Topics", (), "v1.2.5-beta")
        markdown = render_handbook(handbook, "a" * 40, date(2026, 9, 22))
        self.assertIn("- **HyperBricks version:** v1.2.5-beta", markdown)

    def test_mermaid_print_colors_preserve_graph_and_stroke_width(self):
        source = "flowchart TB\n A --> B\n classDef node fill:#ffffff00,stroke:#ffffff,color:#ffffff,stroke-width:2.5px;\n class A,B node;"
        result = build_handbooks.print_mermaid_source(source)
        self.assertIn("A --> B", result)
        self.assertIn("class A,B node;", result)
        self.assertIn("stroke-width:2.5px", result)
        self.assertIn("fill:#f0f5f8,stroke:#087f8c,color:#123044", result)
        self.assertNotIn("#ffffff", result)

    def test_missing_mermaid_renderer_fails_explicitly(self):
        with self.assertRaisesRegex(BuildError, "Mermaid CLI is missing"):
            build_handbooks.render_mermaid("flowchart TB\n A --> B", "/missing/mmdc")

    def test_mermaid_embeds_image_instead_of_source(self):
        from io import BytesIO
        from PIL import Image

        data = BytesIO()
        Image.new("RGB", (300, 200), "white").save(data, format="PNG")
        source = SourceDocument("docs/test.md", "Diagram", "# Diagram\n\n```mermaid\nflowchart TB\n A --> B\n```", "diagram")
        handbook = Handbook("test.md", "HyperBricks Documentation", "Developer handbook", "Test document", "Diagrams.", (source,))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            with patch("build_handbooks.render_mermaid", return_value=data.getvalue()) as renderer:
                build_handbooks.render_pdf(handbook, "a" * 40, date(2026, 9, 16), destination,
                                          Path("/System/Library/Fonts/Supplemental"), Path("/System/Library/Fonts/Menlo.ttc"), "/test/mmdc")
                renderer.assert_called_once()
            reader = PdfReader(destination)
            self.assertNotIn("flowchart TB", "\n".join(page.extract_text() for page in reader.pages))
            self.assertTrue(any(page.images for page in reader.pages))

    def test_generated_handbooks_are_not_sources(self):
        paths = ("docs/INTRODUCTION.md", "docs/MARKDOWN.md",
                 "docs/handbooks/HyperBricks-Documentation.md",
                 "docs/handbooks/HyperBricks-Skills.md")
        with patch("build_markdown_handbooks.committed_paths", return_value=paths):
            self.assertEqual(documentation_paths(Path("."), "a" * 40),
                             ("docs/INTRODUCTION.md", "docs/MARKDOWN.md"))

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
        handbook = Handbook("test.md", "HyperBricks Documentation", "Developer handbook",
                            "Test document", "Explicit anchors.", (source, other))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_handbooks.render_pdf(handbook, "a" * 40, date(2026, 9, 16), destination,
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
        handbook = Handbook("test.md", "HyperBricks Documentation", "Developer handbook", "Test document", "Test.", ())
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            destination.write_bytes(b"existing output")
            with self.assertRaisesRegex(BuildError, "Required font missing"):
                build_handbooks.render_pdf(handbook, "a" * 40, date(2026, 9, 16), destination,
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
        handbook = Handbook("test.md", "HyperBricks Documentation", "Developer handbook",
                            "Test document", "Syntax highlighting.", (source,))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_handbooks.render_pdf(handbook, "a" * 40, date(2026, 9, 16), destination,
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
        handbook = Handbook("test.md", "HyperBricks Documentation", "Developer handbook", "Test document", "Pagination and links.", (source,))
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "test.pdf"
            build_handbooks.render_pdf(handbook, "a" * 40, date(2026, 9, 16), destination,
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
