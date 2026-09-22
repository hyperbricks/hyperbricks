# Handbook generation

From the repository root, run:

```sh
./scripts/handbook-generation/build_handbooks.sh
```

The script creates `.venv-handbooks/`, installs the Python packages pinned in
`handbook-requirements.txt` and Mermaid CLI, then builds the documentation and
skills handbooks as Markdown and PDF in `docs/handbooks/`. The environment is
ignored by Git; it is not part of the repository or a release commit.

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

Both builders read a committed Git revision (default `HEAD`), not uncommitted
working-tree or staged content. Pass `--ref <revision>` to select another
commit. Run the PDF regression tests after the environment has been created:

```sh
.venv-handbooks/bin/python -m unittest discover \
  -s scripts/handbook-generation -p 'test_build_handbooks.py'
```
