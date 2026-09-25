# Compilation generation

These scripts collect separately maintained HyperBricks documents into two
snapshot compilations. The generated files preserve their source paths and are
not intended to read as one continuous manual.

From the repository root, run:

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

Cover text may use `{version}`, `{subtitle}`, `{document_count}`,
`{snapshot_date}`, and `{short_commit}`. The shared top-level text may use
`{version}`, `{document_count}`, and `{short_commit}`. Unknown placeholders,
missing fields, and invalid JSON stop the build with an explicit error.

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
date. The source commit and date live centrally in the manifest alongside the
runtime version, complete document inventory, and source/bundled content hashes.
An unrelated source commit therefore does not change individual documents or
their bundled hashes. Publication compilations retain their snapshot dates.

Pass `--check` to verify an existing snapshot without writing it. With no
explicit `--ref`, the check uses the `source_commit` recorded in the snapshot
manifest, so committing the generated files does not make the check stale.
To check freshness against the current committed canonical documentation, run:

```sh
.venv-compilations/bin/python scripts/compilation-generation/build_skill_documentation.py --check --ref HEAD
.venv-compilations/bin/python scripts/compilation-generation/build_skill_documentation.py --check --ref HEAD --output-dir codex-plugin/hyperbricks/skills/hyperbricks/references
bash scripts/check_codex_plugin_sync.sh
```

With an explicit `--ref`, the check first verifies the files and manifest exactly
against their recorded source revision, then compares their content with the
requested revision. Different source commits or dates alone do not make the
snapshot stale. Changed source text, version, document inventory, generated
links, or bundled content still fail. This is read-only and uses committed
sources; uncommitted `docs/` edits are not included. The tree-sync script checks
that skill and plugin copies match each other; it does not replace the content
checks against canonical documentation.

For safety, a custom non-empty output directory must already contain a valid
snapshot manifest. Use an empty directory for its first generation; the
repository root, canonical `docs/` tree, and symlinked output paths are never
accepted as output.

The snapshot records a committed source revision. Commit canonical documentation
and tooling changes first, generate the snapshot from that commit, then commit
the generated skill and plugin files separately. This avoids claiming that
uncommitted documentation was part of the recorded source revision.

The EPUB uses the PDF's Arial/sans-serif typography, navy/teal heading palette,
code panels, and rendered Mermaid diagrams, with relative sizes and wrapping
for e-readers. It includes chapter navigation, internal links, and a dark theme.
Reader font/theme overrides may change its appearance. No system fonts are
embedded or required for EPUB generation.

Build only EPUBs from a committed snapshot (also accepts `--output-dir`):

```sh
./scripts/compilation-generation/build_compilations.sh --format epub
```

To rebuild EPUBs from existing generated Markdown, without rebuilding PDF:

```sh
.venv-compilations/bin/python scripts/compilation-generation/build_epub_compilations.py
```

The documents included in a compilation are read from a committed Git revision
(default `HEAD`), not from uncommitted working-tree or staged content. Pass
`--ref <revision>` to select another commit. The generator code, visual assets,
and `compilation-texts.json` are read from the current checkout, so text changes
can be previewed before committing them. Run the PDF and EPUB regression tests
after the environment has been created:

```sh
.venv-compilations/bin/python -m unittest discover \
  -s scripts/compilation-generation -p 'test_build*.py'
```
