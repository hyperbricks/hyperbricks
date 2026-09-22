#!/usr/bin/env python3
"""Build Markdown, styled PDF, and reflowable EPUB handbooks from one snapshot."""

from __future__ import annotations

import argparse
from datetime import date
from html import escape
from io import BytesIO
from pathlib import Path
import os
import re
import sys
import subprocess
import tempfile

import build_markdown_handbooks as assembly


def arguments():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ref", default="HEAD", help="Committed Git revision (default: HEAD)")
    parser.add_argument("--format", choices=("all", "markdown", "pdf", "epub"), default="all")
    parser.add_argument("--output-dir", default="docs/handbooks", help="Destination for generated books (default: docs/handbooks)")
    parser.add_argument("--font-dir", default="/System/Library/Fonts/Supplemental")
    parser.add_argument("--code-font", default="/System/Library/Fonts/Menlo.ttc")
    parser.add_argument("--mermaid-cli", default=os.environ.get("HB_MERMAID_CLI", str(Path(__file__).resolve().parents[2] / ".venv-handbooks/mermaid/node_modules/.bin/mmdc")))
    return parser.parse_args()


def print_mermaid_source(source):
    # Mermaid emits class/style colors as inline !important declarations, which
    # cannot be overridden by print CSS. Adapt only those presentation statements.
    palette = {"fill": "#f0f5f8", "stroke": "#087f8c", "color": "#123044"}
    lines = []
    for line in source.splitlines():
        if re.match(r"^\s*(classDef|style|linkStyle)\s", line):
            line = re.sub(r"\b(fill|stroke|color)\s*:\s*[^,;]+",
                          lambda match: f"{match[1]}:{palette[match[1]]}", line)
        lines.append(line)
    return "\n".join(lines)


def render_mermaid(source, executable):
    if not executable or not Path(executable).is_file():
        raise assembly.BuildError("Mermaid CLI is missing; run bash scripts/handbook-generation/build_handbooks.sh or supply --mermaid-cli")
    assets = Path(__file__).resolve().parent
    with tempfile.TemporaryDirectory(prefix="hb-mermaid-") as directory:
        input_path = Path(directory) / "diagram.mmd"
        output_path = Path(directory) / "diagram.png"
        input_path.write_text(print_mermaid_source(source), encoding="utf-8")
        try:
            result = subprocess.run(
                [str(executable), "-i", str(input_path), "-o", str(output_path),
                 "-b", "white", "-w", "1600", "-s", "2",
                 "-C", str(assets / "handbook-mermaid.css"),
                 "-c", str(assets / "handbook-mermaid.json")],
                capture_output=True, text=True, timeout=120, check=False,
            )
        except subprocess.TimeoutExpired as error:
            raise assembly.BuildError("Mermaid rendering exceeded 120 seconds") from error
        if result.returncode or not output_path.is_file():
            raise assembly.BuildError(f"Mermaid rendering failed: {(result.stderr or result.stdout)[-2500:]}")
        return output_path.read_bytes()


def render_pdf(handbook, commit, snapshot_date, destination, font_dir, code_font, mermaid_cli=None):
    from markdown_it import MarkdownIt
    from pypdf import PdfReader
    from pygments import lex
    from pygments.lexers import get_lexer_by_name, TextLexer
    from pygments.token import Token
    from pygments.util import ClassNotFound
    from reportlab.lib import colors
    from reportlab.lib.enums import TA_LEFT
    from reportlab.lib.pagesizes import A4
    from reportlab.lib.styles import ParagraphStyle
    from reportlab.lib.utils import ImageReader
    from reportlab.pdfbase import pdfmetrics
    from reportlab.pdfbase.ttfonts import TTFont
    from reportlab.platypus import (
        BaseDocTemplate, Flowable, Frame, PageBreak, PageTemplate,
        Image, Paragraph, Spacer, Table, TableStyle,
    )
    from reportlab.platypus.tableofcontents import TableOfContents

    fonts = {
        "HB": font_dir / "Arial.ttf",
        "HB-Bold": font_dir / "Arial Bold.ttf",
        "HB-Italic": font_dir / "Arial Italic.ttf",
        "HB-BoldItalic": font_dir / "Arial Bold Italic.ttf",
        "HB-Code": code_font,
    }
    for name, path in fonts.items():
        if not path.is_file():
            raise assembly.BuildError(f"Required font missing: {path}; set --font-dir/--code-font")
        pdfmetrics.registerFont(TTFont(name, str(path)))
    pdfmetrics.registerFontFamily("HB", normal="HB", bold="HB-Bold", italic="HB-Italic", boldItalic="HB-BoldItalic")
    ink = colors.HexColor("#123044")
    text = colors.HexColor("#243642")
    teal = colors.HexColor("#087f8c")
    border = colors.HexColor("#cedae1")
    width, height = A4
    content_width = width - 112
    styles = {
        "body": ParagraphStyle("body", fontName="HB", fontSize=9.5, leading=14, textColor=text, spaceAfter=7, splitLongWords=True),
        "small": ParagraphStyle("small", fontName="HB", fontSize=8, leading=11, textColor=text, spaceAfter=7),
        "chapter": ParagraphStyle("chapter", fontName="HB-Bold", fontSize=25, leading=30, textColor=ink, spaceAfter=14, keepWithNext=True),
        "h2": ParagraphStyle("h2", fontName="HB-Bold", fontSize=15, leading=19, textColor=ink, spaceBefore=10, spaceAfter=7, keepWithNext=True),
        "h3": ParagraphStyle("h3", fontName="HB-Bold", fontSize=11.5, leading=15, textColor=teal, spaceBefore=8, spaceAfter=6, keepWithNext=True),
        "cell": ParagraphStyle("cell", fontName="HB", fontSize=8, leading=11, textColor=text, splitLongWords=True),
    }
    kind = "Skills" if "Skills" in handbook.title else "Documentation"
    running = "SKILLS HANDBOOK" if kind == "Skills" else "DEVELOPMENT HANDBOOK"
    code_palette = (
        (Token.Comment, colors.HexColor("#576579")),
        (Token.Keyword, colors.HexColor("#6639ba")),
        (Token.Name.Tag, colors.HexColor("#0550ae")),
        (Token.Name.Attribute, colors.HexColor("#0550ae")),
        (Token.Name.Builtin, colors.HexColor("#6639ba")),
        (Token.Literal.String, colors.HexColor("#116329")),
        (Token.Literal.Number, colors.HexColor("#953800")),
        (Token.Operator, colors.HexColor("#6639ba")),
    )

    class Code(Flowable):
        def __init__(self, raw, language="", lines=None, continued=False, continues=False):
            super().__init__()
            self.raw, self.language, self.lines = raw, language, lines
            self.continued, self.continues = continued, continues
            self.spaceBefore, self.spaceAfter = 5, 10

        def colored_lines(self):
            options = {"stripnl": False, "ensurenl": False}
            try:
                lexer = get_lexer_by_name(self.language.lower(), **options)
            except ClassNotFound:
                lexer = TextLexer(**options)
            source = self.raw.expandtabs(4)
            lines = [[]]
            for token, value in lex(source, lexer):
                color = next((color for category, color in code_palette if token in category), text)
                for index, piece in enumerate(value.split("\n")):
                    if index:
                        lines.append([])
                    if piece:
                        lines[-1].append((piece, color))
            if source.endswith("\n"):
                lines.pop()
            return lines or [[]]

        def wrap(self, available_width, available_height):
            self.width = available_width
            if self.lines is None:
                self.lines = []
                line_width = available_width - 28
                for logical_line in self.colored_lines():
                    line, remaining = [], line_width
                    for piece, color in logical_line:
                        while piece:
                            end = len(piece)
                            if pdfmetrics.stringWidth(piece, "HB-Code", 8) > remaining:
                                lower, upper = 0, len(piece)
                                while lower < upper:
                                    middle = (lower + upper + 1) // 2
                                    if pdfmetrics.stringWidth(piece[:middle], "HB-Code", 8) <= remaining:
                                        lower = middle
                                    else:
                                        upper = middle - 1
                                end = lower
                            if end:
                                part = piece[:end]
                                line.append((part, color))
                                remaining -= pdfmetrics.stringWidth(part, "HB-Code", 8)
                                piece = piece[end:]
                            if piece:
                                if not line:
                                    raise assembly.BuildError("PDF code frame is too narrow for a character")
                                self.lines.append(line)
                                line, remaining = [], line_width
                    self.lines.append(line)
            self.height = 36 + max(1, len(self.lines)) * 12 + (18 if self.continues else 0)
            return self.width, self.height

        def split(self, available_width, available_height):
            self.wrap(available_width, available_height)
            # Reserve footer space before splitting so the notice cannot cover code.
            count = int((available_height - 36 - 18) // 12)
            if count < 2 or count >= len(self.lines):
                return []
            return [Code("", self.language, self.lines[:count], self.continued, True),
                    Code("", self.language, self.lines[count:], True, self.continues)]

        def draw(self):
            c = self.canv
            c.setStrokeColor(border)
            c.setFillColor(colors.HexColor("#f0f5f8"))
            c.roundRect(0, 0, self.width, self.height, 6, fill=1, stroke=1)
            c.setFillColor(colors.HexColor("#e4edf2"))
            c.rect(1, self.height - 27, self.width - 2, 20, fill=1, stroke=0)
            c.setFillColor(teal)
            c.roundRect(12, self.height - 19, 3, 9, 1, fill=1, stroke=0)
            c.setFillColor(colors.HexColor("#426176"))
            c.setFont("HB-Bold", 7.4)
            c.drawString(22, self.height - 18, self.language.upper() or "CODE")
            if self.continued:
                c.setFont("HB", 7.2)
                c.drawRightString(self.width - 14, self.height - 18, "Continued from previous page")
            for index, line in enumerate(self.lines):
                code = c.beginText(14, self.height - 40 - index * 12)
                code.setFont("HB-Code", 8)
                for piece, color in line:
                    code.setFillColor(color)
                    code.textOut(piece)
                c.drawText(code)
            if self.continues:
                c.setStrokeColor(border)
                c.setLineWidth(.5)
                c.line(14, 20, self.width - 14, 20)
                c.setFillColor(teal)
                c.setFont("HB-Italic", 7.2)
                c.drawRightString(self.width - 14, 8, "Continues on next page")

    class Book(BaseDocTemplate):
        def afterFlowable(self, flowable):
            if hasattr(flowable, "chapter_anchor"):
                self.canv.bookmarkPage(flowable.chapter_anchor)
                self.canv.addOutlineEntry(flowable.getPlainText(), flowable.chapter_anchor, level=0)
                self.notify("TOCEntry", (0, flowable.getPlainText(), self.page, flowable.chapter_anchor))

    def decorate(c, doc):
        if doc.page == 1:
            c.saveState()
            c.setFillColor(teal)
            c.setFont("HB-Bold", 9)
            c.drawString(56, height - 165, f"HYPERBRICKS / {kind.upper()}")
            c.setFillColor(ink)
            c.setFont("HB-Bold", 42)
            c.drawString(56, height - 220, "HyperBricks")
            c.drawString(56, height - 268, "Skills handbook" if kind == "Skills" else "Documentation")
            c.setFont("HB-Bold", 21)
            c.drawString(56, height - 338, handbook.subtitle)
            description = Paragraph(escape(handbook.description), styles["body"])
            _, desc_height = description.wrap(content_width, 100)
            description.drawOn(c, 56, height - 377 - desc_height)
            c.setFont("HB", 9.5)
            c.setFillColor(text)
            c.drawString(56, height - 407 - desc_height, handbook.topics)
            c.setFillColor(teal)
            c.setFont("HB-Bold", 9)
            stamp = f"{snapshot_date.day} {assembly.MONTH_NAMES[snapshot_date.month - 1]} {snapshot_date.year}"
            c.drawString(56, height - 518, f"{len(handbook.sources)} documents • {stamp}")
            c.setFillColor(text)
            c.setFont("HB", 8)
            c.drawString(56, height - 540, f"Source snapshot: Git commit {commit[:7]}; all chapters use this committed revision.")
            c.restoreState()
            return
        c.saveState()
        c.setFillColor(teal)
        c.rect(50, height - 35, 26, 3, fill=1, stroke=0)
        c.setFont("HB", 7.2)
        c.drawRightString(width - 56, height - 34, running)
        c.setStrokeColor(border)
        c.line(50, 42, width - 50, 42)
        c.setFillColor(text)
        c.setFont("HB", 7.5)
        c.drawString(50, 29, f"HyperBricks / {kind}")
        c.drawRightString(width - 50, 29, str(doc.page))
        c.restoreState()

    def inline(tokens):
        parts = []
        for token in tokens or []:
            if token.type == "text":
                parts.append(escape(token.content))
            elif token.type == "code_inline":
                parts.append(f'<font name="HB-Code" size="8">{escape(token.content)}</font>')
            elif token.type in {"strong_open", "strong_close", "em_open", "em_close"}:
                tag = "b" if token.type.startswith("strong") else "i"
                parts.append(f"<{tag}>" if token.nesting == 1 else f"</{tag}>")
            elif token.type == "link_open":
                parts.append(f'<a href="{escape(token.attrGet("href"), quote=True)}" color="#087f8c">')
            elif token.type == "link_close":
                parts.append("</a>")
            elif token.type in {"softbreak", "hardbreak"}:
                parts.append(" " if token.type == "softbreak" else "<br/>")
            elif token.type == "image":
                raise assembly.BuildError("Embedded Markdown images require an explicit PDF asset renderer")
            elif token.type == "html_inline":
                parts.append(escape(token.content))
            else:
                raise assembly.BuildError(f"Unsupported inline Markdown token: {token.type}")
        return "".join(parts)

    parser = MarkdownIt("commonmark", {"html": True}).enable("table")
    diagrams = {}

    def blocks(markdown):
        tokens = parser.parse(markdown)
        result, anchors, lists = [], [], []
        index = 0
        while index < len(tokens):
            token = tokens[index]
            if token.type == "html_block":
                found = re.findall(r'<a id="([^"]+)"></a>', token.content)
                if found:
                    anchors.extend(found)
                elif not token.content.lstrip().startswith("<!--"):
                    result.append(Code(token.content, "HTML"))
            elif token.type == "heading_open":
                level = int(token.tag[1:])
                markup = "".join(f'<a name="{escape(a, quote=True)}"/>' for a in anchors)
                anchors.clear()
                result.append(Paragraph(markup + inline(tokens[index + 1].children), styles["h2" if level <= 3 else "h3"]))
                index += 2
            elif token.type == "paragraph_open":
                content = tokens[index + 1].content
                found = re.findall(r'<a id="([^"]+)"></a>', content)
                if found and not re.sub(r'<a id="[^"]+"></a>', "", content).strip():
                    anchors.extend(found)
                    index += 3
                    continue
                prefix = ""
                if lists:
                    prefix = "• " if lists[-1] is None else f"{lists[-1]}. "
                    if lists[-1] is not None:
                        lists[-1] += 1
                result.append(Paragraph(prefix + inline(tokens[index + 1].children), styles["body"]))
                index += 2
            elif token.type in {"fence", "code_block"}:
                language = token.info.split()[0] if token.info else ""
                if language.lower() == "mermaid":
                    if token.content not in diagrams:
                        diagrams[token.content] = render_mermaid(token.content, mermaid_cli)
                    data = diagrams[token.content]
                    image_width, image_height = ImageReader(BytesIO(data)).getSize()
                    scale = min(content_width / image_width, (height - 130) / image_height)
                    diagram = Image(BytesIO(data), width=image_width * scale, height=image_height * scale)
                    diagram.hAlign = "CENTER"
                    result.extend([Spacer(1, 8), diagram, Spacer(1, 10)])
                else:
                    result.append(Code(token.content, language))
            elif token.type in {"bullet_list_open", "ordered_list_open"}:
                lists.append(None if token.type == "bullet_list_open" else int(token.attrGet("start") or 1))
            elif token.type in {"bullet_list_close", "ordered_list_close"}:
                lists.pop()
            elif token.type == "table_open":
                rows, row = [], []
                index += 1
                while tokens[index].type != "table_close":
                    entry = tokens[index]
                    if entry.type == "tr_open":
                        row = []
                    elif entry.type == "inline":
                        row.append(Paragraph(inline(entry.children), styles["cell"]))
                    elif entry.type == "tr_close":
                        rows.append(row)
                    index += 1
                table = Table(rows, colWidths=[content_width / len(rows[0])] * len(rows[0]), repeatRows=1, splitInRow=1, hAlign="LEFT")
                table.setStyle(TableStyle([
                    ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#e4edf2")),
                    ("VALIGN", (0, 0), (-1, -1), "TOP"),
                    ("GRID", (0, 0), (-1, -1), .4, border),
                    ("LEFTPADDING", (0, 0), (-1, -1), 7),
                    ("RIGHTPADDING", (0, 0), (-1, -1), 7),
                    ("TOPPADDING", (0, 0), (-1, -1), 6),
                    ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
                ]))
                result.extend([table, Spacer(1, 10)])
            index += 1
        return result

    included = {source.path: source for source in handbook.sources}
    story = [Spacer(1, 1), PageBreak(), Paragraph("Contents", styles["chapter"]),
             Paragraph("Guides are arranged in a practical reading order. Select a title to jump to its chapter.", styles["body"])]
    toc = TableOfContents()
    toc.levelStyles = [ParagraphStyle("toc", parent=styles["body"], spaceBefore=5, spaceAfter=5, alignment=TA_LEFT)]
    story.append(toc)
    for number, source in enumerate(handbook.sources, 1):
        story.extend([PageBreak(), Paragraph(f"{number:02d} / {escape(source.path)}", ParagraphStyle("label", parent=styles["small"], fontName="HB-Bold", textColor=teal))])
        chapter = Paragraph(f'<a name="{source.anchor}"/>{escape(source.title)}', styles["chapter"])
        chapter.chapter_anchor = source.anchor
        story.append(chapter)
        transformed = assembly.transform_source(source, included, commit, expose_front_matter=source.path.endswith("/SKILL.md"))
        story.extend(blocks(transformed))
    destination.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(suffix=".pdf", dir=destination.parent)
    os.close(descriptor)
    try:
        book = Book(temporary, pagesize=A4, title=handbook.title, author="HyperBricks", subject=handbook.subtitle)
        frame = Frame(56, 54, content_width, height - 110, leftPadding=0, rightPadding=0, topPadding=0, bottomPadding=0)
        book.addPageTemplates(PageTemplate(id="handbook", frames=frame, onPage=decorate))
        book.multiBuild(story)
        reader = PdfReader(temporary)
        if len(reader.pages) < 3 or not reader.outline:
            raise assembly.BuildError("PDF validation failed: missing pages or chapter outline")
        os.chmod(temporary, destination.stat().st_mode & 0o777 if destination.exists() else 0o644)
        os.replace(temporary, destination)
        print(f"Wrote {destination} ({len(reader.pages)} pages)")
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def main():
    args = arguments()
    try:
        repository = assembly.repository_root()
        commit = assembly.resolve_commit(repository, args.ref)
        snapshot_date = date.fromisoformat(assembly.run_git(repository, "show", "-s", "--format=%cs", commit).strip())
        output = Path(args.output_dir)
        if not output.is_absolute():
            output = repository / output
        for handbook in assembly.build_handbooks(repository, commit):
            if args.format in {"all", "markdown"}:
                path = output / handbook.filename
                assembly.write_atomic(path, assembly.render_handbook(handbook, commit, snapshot_date))
                print(f"Wrote {path} ({len(handbook.sources)} sources)")
            if args.format in {"all", "pdf"}:
                render_pdf(handbook, commit, snapshot_date, output / Path(handbook.filename).with_suffix(".pdf"), Path(args.font_dir), Path(args.code_font), args.mermaid_cli)
            if args.format in {"all", "epub"}:
                from build_epub_handbooks import epub_for
                path = output / Path(handbook.filename).with_suffix(".epub")
                epub_for(Path(handbook.filename), path, args.mermaid_cli,
                         markdown=assembly.render_handbook(handbook, commit, snapshot_date))
                print(f"Wrote {path}")
        print(f"Source snapshot: {commit}")
        return 0
    except (assembly.BuildError, OSError, ValueError, ImportError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
