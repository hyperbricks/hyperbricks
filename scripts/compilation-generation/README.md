# Compilation generation

These scripts collect separately maintained HyperBricks documents into two
snapshot compilations. The generated files preserve their source paths and are
not intended to read as one continuous manual.

After changing canonical documentation or the skill, run one command from the
repository root:

```sh
./scripts/sync_docs.sh
```

It regenerates the README and component reference, every compilation format,
both skill-reference trees, and the packaged Codex plugin mirror from the
current working tree. It runs the focused checks and never commits, tags, or
pushes. Repeating it without source changes leaves tracked output bytes
unchanged.

To build compilations alone, run:

```sh
./scripts/compilation-generation/build_compilations.sh
```

The script creates `.venv-compilations/`, installs the Python packages pinned in
`compilation-requirements.txt` and Mermaid CLI, then builds the documentation
and skills compilations as Markdown, PDF, and EPUB in `docs/compilations/`. The
environment is ignored by Git; it is not part of the repository or a release
commit.
When Markdown is generated (`all` or `--format markdown`), the documentation
compilation and a versioned, one-file-per-source documentation snapshot are also
synchronized to the HyperBricks skill and packaged Codex plugin references. The
skills compilation remains a publication artifact in `docs/compilations/`.
PDF-only and EPUB-only builds leave the skill references unchanged.

## Text and cover copy

Edit `compilation-texts.json` to change the titles, introductions, topic labels,
topics, and PDF cover text. It contains separate `documentation` and `skills`
sections. The top-level `title`, `subtitle`, `description`, `topics_label`, and
`topics` values are shared by the generated formats; the nested `cover` values
only control the PDF cover.

The default cover copy uses only `{version}`, `{subtitle}`, and
`{document_count}` to keep the release output stable. The template engine also
supports `{snapshot_date}` and `{short_commit}` only when building custom
compilations with an explicit committed `--ref`; the working-tree build rejects
them so it cannot silently introduce clock or commit-ID churn.
Unknown placeholders, missing fields, and invalid JSON stop the build.

You need Python 3 with `venv` and `pip`, Node.js with `npm`, and network access
for the first build. PDF generation also needs fonts. The defaults use Arial
and Menlo on macOS; on another system, specify an Arial-compatible font
directory and a code-font file:

```sh
./scripts/compilation-generation/build_compilations.sh \
  --font-dir /path/to/font-directory \
  --code-font /path/to/monospace.ttf
```

For Markdown only, run the standalone builder without installing PDF or
Mermaid dependencies:

```sh
./scripts/compilation-generation/build_markdown_compilations.py
```

Build only the separate documentation snapshot used by the skill:

```sh
./scripts/compilation-generation/build_skill_documentation.py
```

The snapshot writes `DOCUMENTATION_INDEX.md`, `documentation-manifest.json`, and
generated documents under `SKILLS/hyperbricks/references/docs/`. The documents
retain the source prose from `/docs`; links within `/docs` remain local and links
to other repository paths use the release version from `assets/version.md`.
Each document has a stable `Generated from docs/...` notice without a commit ID.
The index links to the manifest for provenance instead of embedding a snapshot
date. The manifest records a digest of the canonical document inventory and
content, the runtime version, and source/bundled hashes for every document.
It does not depend on a commit ID that may become unreachable after an amend.

Pass `--check` to verify an existing snapshot against the current working tree
without writing it. The one-command workflow runs both mirror checks;
manually:

```sh
.venv-compilations/bin/python scripts/compilation-generation/build_skill_documentation.py --check
.venv-compilations/bin/python scripts/compilation-generation/build_skill_documentation.py --check --output-dir codex-plugin/hyperbricks/skills/hyperbricks/references
bash scripts/check_codex_plugin_sync.sh
```

With an explicit `--ref`, the check compares against that committed revision.
Changed source text, version, document inventory, generated links, or bundled
content fail. The tree-sync script checks
that skill and plugin copies match each other; it does not replace the content
checks against canonical documentation.

For safety, a custom non-empty output directory must already contain a valid
snapshot manifest. Use an empty directory for its first generation; the
repository root, canonical `docs/` tree, and symlinked output paths are never
accepted as output.

The snapshot records content rather than a particular commit. Canonical docs,
generated references, and the packaged mirror can be verified together before
any commit and committed as one coherent change.

The EPUB uses the PDF's Arial/sans-serif typography, navy/teal heading palette,
code panels, and rendered Mermaid diagrams, with relative sizes and wrapping
for e-readers. It includes chapter navigation, internal links, and a dark theme.
Reader font/theme overrides may change its appearance. No system fonts are
embedded or required for EPUB generation.

Build only EPUBs (also accepts `--ref` and `--output-dir`):

```sh
./scripts/compilation-generation/build_compilations.sh --format epub
```

To rebuild EPUBs from existing generated Markdown, without rebuilding PDF:

```sh
.venv-compilations/bin/python scripts/compilation-generation/build_epub_compilations.py
```

The documents included in a compilation are read from the current working tree
by default, including uncommitted changes. Pass `--ref <revision>` to select a
committed snapshot instead. The generator code, visual assets,
and `compilation-texts.json` are read from the current checkout, so text changes
can be previewed before committing them. Run the PDF and EPUB regression tests
after the environment has been created:

```sh
.venv-compilations/bin/python -m unittest discover \
  -s scripts/compilation-generation -p 'test_build*.py'
```
