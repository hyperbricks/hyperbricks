# Handbook generation

From the repository root, run:

```sh
./scripts/handbook-generation/build_handbooks.sh
```

The script creates `.venv-handbooks/`, installs the Python packages pinned in
`handbook-requirements.txt` and Mermaid CLI, then builds the documentation and
skills handbooks as Markdown, PDF, and EPUB in `docs/handbooks/`. The environment is
ignored by Git; it is not part of the repository or a release commit.
When Markdown is generated (`all` or `--format markdown`), the two Markdown
handbooks are also synchronized to the HyperBricks skill and packaged Codex
plugin references. PDF-only and EPUB-only builds leave those copies unchanged.

You need Python 3 with `venv` and `pip`, Node.js with `npm`, and network access
for the first build. PDF generation also needs fonts. The defaults use Arial
and Menlo on macOS; on another system, specify an Arial-compatible font
directory and a code-font file:

```sh
./scripts/handbook-generation/build_handbooks.sh \
  --font-dir /path/to/font-directory \
  --code-font /path/to/monospace.ttf
```

For Markdown only, run the standalone builder without installing PDF or
Mermaid dependencies:

```sh
./scripts/handbook-generation/build_markdown_handbooks.py
```

The EPUB uses the PDF's Arial/sans-serif typography, navy/teal heading palette,
code panels, and rendered Mermaid diagrams, with relative sizes and wrapping
for e-readers. It includes chapter navigation, internal links, and a dark theme.
Reader font/theme overrides may change its appearance. No system fonts are
embedded or required for EPUB generation.

Build only EPUBs from a committed snapshot (also accepts `--output-dir`):

```sh
./scripts/handbook-generation/build_handbooks.sh --format epub
```

To rebuild EPUBs from existing generated Markdown, without rebuilding PDF:

```sh
.venv-handbooks/bin/python scripts/handbook-generation/build_epub_handbooks.py
```

The snapshot builders read a committed Git revision (default `HEAD`), not uncommitted
working-tree or staged content. Pass `--ref <revision>` to select another
commit. Run the PDF and EPUB regression tests after the environment has been created:

```sh
.venv-handbooks/bin/python -m unittest discover \
  -s scripts/handbook-generation -p 'test_build*handbooks.py'
```
