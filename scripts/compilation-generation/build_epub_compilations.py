#!/usr/bin/env python3
"""Build reflowable EPUB 3 compilations using the PDF compilation's visual language."""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
from html import escape
from pathlib import Path
import re
import os
import tempfile
import uuid
import xml.etree.ElementTree as ET
from zipfile import ZIP_DEFLATED, ZIP_STORED, ZipFile

from markdown_it import MarkdownIt

ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = ROOT / "docs/compilations"
DEFAULT_MERMAID = ROOT / ".venv-compilations/mermaid/node_modules/.bin/mmdc"
STYLES = """
:root { color-scheme: light dark; }
body { font-family: Arial, Helvetica, sans-serif; line-height: 1.5;
  max-width: 48em; margin: 2em auto; padding: 0 5%;
  color: #243642; background: #fff; text-align: left; }
h1,h2,h3,h4,h5,h6 { font-weight: bold; line-height: 1.2; color: #123044;
  hyphens: none; -webkit-hyphens: none; break-after: avoid; page-break-after: avoid; }
h1 { font-size: 2em; margin: 1em 0 .7em; }
h2 { font-size: 1.65em; margin: 1.4em 0 .7em; }
h2.chapter { break-before: page; page-break-before: always; border-top: .15em solid #087f8c; padding-top: .6em; }
h3 { font-size: 1.3em; margin-top: 1.3em; }
h4,h5,h6 { font-size: 1.1em; color: #087f8c; }
p { margin: .6em 0; orphans: 2; widows: 2; }
a { color: #087f8c; text-decoration: underline; }
li { margin: .25em 0; }
code,pre { font-family: Menlo, Consolas, monospace; hyphens: none; -webkit-hyphens: none; }
code { font-size: .85em; overflow-wrap: anywhere; word-wrap: break-word; }
pre { font-size: .85em; line-height: 1.45; white-space: pre-wrap;
  overflow-wrap: anywhere; word-wrap: break-word; padding: .75em;
  background: #f0f5f8; color: #243642; border: 1px solid #cedae1; border-radius: .25em; }
pre code { font-size: 1em; }
.syntax-comment { color: #576579; } .syntax-keyword { color: #6639ba; }
.syntax-name { color: #0550ae; } .syntax-string { color: #116329; }
.syntax-number { color: #953800; }
table { border-collapse: collapse; width: 100%; font-size: .85em; table-layout: fixed; }
th,td { border: 1px solid #cedae1; padding: .4em; vertical-align: top; overflow-wrap: anywhere; }
th { background: #f0f5f8; color: #123044; }
blockquote { border-left: .2em solid #087f8c; margin: 1em 0; padding-left: 1em; }
figure { margin: 1em 0; break-inside: avoid; page-break-inside: avoid; }
img { max-width: 100%; height: auto; }
figcaption { font-size: .85em; margin-top: .4em; }
hr { border: 0; border-top: 1px solid #cedae1; margin: 1.5em 0; }
@media (prefers-color-scheme: dark) {
  body { color: #e6edf3; background: #121a21; }
  h1,h2,h3 { color: #e6edf3; } h4,h5,h6,a { color: #70d1da; }
  pre,th { background: #1d2b36; color: #e6edf3; }
  .syntax-comment { color: #abb8c9; } .syntax-keyword { color: #d2a8ff; }
  .syntax-name { color: #79c0ff; } .syntax-string { color: #a5d6a7; }
  .syntax-number { color: #ffa657; }
}
"""


def document(title, body):
    return ('<?xml version="1.0" encoding="utf-8"?>\n<!DOCTYPE html>\n'
            '<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en">'
            f'<head><title>{escape(title)}</title><link rel="stylesheet" type="text/css" href="styles.css"/></head>'
            f'<body>{body}</body></html>')


def render_content(markdown, mermaid_cli):
    from build_compilations import render_mermaid
    from pygments import lex
    from pygments.lexers import get_lexer_by_name
    from pygments.token import Token
    from pygments.util import ClassNotFound
    md = MarkdownIt("commonmark", {"html": True, "xhtmlOut": True}).enable("table")
    assets, diagrams, headings = {}, {}, []
    default_fence = md.renderer.rules["fence"]

    def raw_html(tokens, index, options, env):
        raw = tokens[index].content
        # Generated anchors are markup, not examples. Other raw HTML is treated
        # as a source example; never execute scripts/styles from compilation prose.
        raw = re.sub(r"<!--.*?-->", "", raw, flags=re.S)
        if not raw.strip():
            return ""
        if re.fullmatch(r'\s*<a\s+id="[\w.-]+"\s*></a>\s*', raw):
            return raw
        if re.fullmatch(r'<a\s+id="[\w.-]+"\s*>|</a>', raw):
            return raw
        if re.fullmatch(r"\s*<br\s*/?>\s*", raw):
            return "<br/>"
        escaped = escape(raw)
        return f"<pre><code>{escaped}</code></pre>" if tokens[index].type == "html_block" else escaped

    def fence(tokens, index, options, env):
        token = tokens[index]
        if token.info.strip().lower() != "mermaid":
            language = token.info.split()[0] if token.info.strip() else "text"
            try:
                lexer = get_lexer_by_name(language, stripnl=False, ensurenl=False)
            except ClassNotFound:
                return default_fence(tokens, index, options, env)
            palette = ((Token.Comment, 'comment'), (Token.Keyword, 'keyword'),
                       (Token.Name.Tag, 'name'), (Token.Name.Attribute, 'name'),
                       (Token.Name.Builtin, 'keyword'), (Token.Literal.String, 'string'),
                       (Token.Literal.Number, 'number'), (Token.Operator, 'keyword'))
            pieces = []
            for kind, value in lex(token.content, lexer):
                category = next((name for match, name in palette if kind in match), None)
                text = escape(value)
                pieces.append(f'<span class="syntax-{category}">{text}</span>' if category else text)
            return '<pre><code>' + ''.join(pieces) + '</code></pre>\n'
        if token.content not in diagrams:
            name = f"images/diagram-{len(diagrams) + 1}.png"
            assets[name] = render_mermaid(token.content, mermaid_cli)
            diagrams[token.content] = name
        name = diagrams[token.content]
        return (f'<figure><img src="{name}" alt="HyperBricks diagram"/>'
                '<figcaption>HyperBricks architecture and flow diagram.</figcaption></figure>')

    md.renderer.rules.update(html_block=raw_html, html_inline=raw_html, fence=fence)
    tokens = md.parse(markdown)
    contents_labels = {
        anchor: re.sub(r"`([^`]*)`", r"\1", label)
        for label, anchor in re.findall(
            r"(?m)^- \[([^]]+)\]\(#([^)]+)\) — `[^`]+`$",
            markdown,
        )
    }
    inside_contents = False
    pending_anchor = ""
    for index, token in enumerate(tokens):
        if token.type in {"html_block", "inline"}:
            anchor_match = re.fullmatch(r'\s*<a\s+id="([\w.-]+)"\s*></a>\s*', token.content)
            if anchor_match:
                pending_anchor = anchor_match.group(1)
        if token.type == "heading_open":
            title = tokens[index + 1].content
            navigation_title = contents_labels.get(pending_anchor, title)
            pending_anchor = ""
            anchor = f"epub-heading-{len(headings) + 1}"
            token.attrSet("id", anchor)
            level = int(token.tag[1:])
            if level == 2:
                inside_contents = title == "Contents"
            navigation_level = 2 if inside_contents and level == 3 else level
            headings.append((navigation_level, navigation_title, anchor))
            if token.tag == "h2" and title != "Contents":
                token.attrSet("class", "chapter")
    body = md.renderer.render(tokens, md.options, {})
    # Keep the visible contents compact; source paths remain in chapter credits.
    body = re.sub(r'(<li><a href="#[^"]+">[^<]+</a>) — <code>[^<]+</code>', r'\1', body)
    return body, assets, headings


def epub_for(source, output, mermaid_cli=None, markdown=None, title=None):
    source, output = Path(source), Path(output)
    markdown_source = markdown if markdown is not None else source.read_text(encoding="utf-8")
    if title is None:
        heading = re.search(r"^#\s+(.+?)\s*$", markdown_source, flags=re.MULTILINE)
        title = heading.group(1) if heading else (
            "HyperBricks Skills Compilation"
            if "Skills" in source.name
            else "HyperBricks Documentation Compilation"
        )
    body, assets, headings = render_content(markdown_source,
                                           mermaid_cli or str(DEFAULT_MERMAID))
    content = document(title, body)
    nav_items = "".join(f'<li><a href="content.xhtml#{anchor}">{escape(label)}</a></li>'
                        for level, label, anchor in headings if level <= 2)
    nav = document(title, f'<nav epub:type="toc" id="toc"><h1>Contents</h1><ol>{nav_items}</ol></nav>')
    identifier = uuid.uuid5(uuid.NAMESPACE_URL, "https://github.com/hyperbricks/hyperbricks/" + source.name)
    modified = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    image_manifest = "".join(f'<item id="image-{i}" href="{name}" media-type="image/png"/>' for i, name in enumerate(assets))
    package = f'''<?xml version="1.0" encoding="utf-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book-id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book-id">urn:uuid:{identifier}</dc:identifier><dc:title>{escape(title)}</dc:title><dc:creator>HyperBricks</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">{modified}</meta></metadata><manifest><item id="content" href="content.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="css" href="styles.css" media-type="text/css"/>{image_manifest}</manifest><spine><itemref idref="content"/></spine></package>'''
    for xml in (content, nav, package):
        ET.fromstring(xml)
    output.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(suffix=".epub", dir=output.parent)
    os.close(fd)
    try:
        with ZipFile(temporary, "w", compression=ZIP_DEFLATED) as book:
            book.writestr("mimetype", "application/epub+zip", compress_type=ZIP_STORED)
            book.writestr("META-INF/container.xml", '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
            for name, data in {"content.xhtml": content, "nav.xhtml": nav, "styles.css": STYLES, "package.opf": package, **assets}.items():
                book.writestr("OEBPS/" + name, data)
        os.chmod(temporary, 0o644)
        os.replace(temporary, output)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output_dir", nargs="?", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--mermaid-cli", default=os.environ.get("HB_MERMAID_CLI", str(DEFAULT_MERMAID)))
    args = parser.parse_args()
    for name in ("HyperBricks-Skills.md", "HyperBricks-Documentation.md"):
        source = args.output_dir / name
        epub_for(source, source.with_suffix(".epub"), args.mermaid_cli)
        print(f"Built {source.with_suffix('.epub')}")


if __name__ == "__main__":
    main()
