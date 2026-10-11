# Set up a coding agent

HyperBricks provides one [Agent Skills](https://agentskills.io/specification)
skill for Codex, Claude Code, Gemini CLI, and other compatible agents. It contains
instructions and documentation matched to your HyperBricks version. Install it
in the project root containing `modules/`.

## Install with HyperBricks

From your application project root:

```bash
hyperbricks version
hyperbricks skills install
```

The command downloads the complete skill ZIP and its SHA-256 sidecar from
hyperbricks.eu for the exact installed runtime version, including its prerelease
suffix. It verifies the checksum and documentation manifest before installing.
No HyperBricks source checkout, Python, npm, GitHub CLI, or agent plugin is needed.
If that version's download has not been published, installation reports the
missing artifact; it does not silently install documentation for another version.

By default, installation supports all three clients:

```text
project/
├── modules/
├── .agents/skills/hyperbricks/
│   ├── SKILL.md
│   ├── agents/openai.yaml
│   ├── assets/
│   ├── references/
│   │   ├── DOCUMENTATION_INDEX.md
│   │   ├── documentation-manifest.json
│   │   └── docs/
│   └── .hyperbricks-install.json
└── .claude/skills/hyperbricks -> ../../.agents/skills/hyperbricks
```

Codex and Gemini read the shared directory. Claude reads the same contents
through a relative directory symlink. Optional `agents/openai.yaml` metadata
supplies Codex presentation; the skill itself uses the shared format.

Select one client or another project explicitly:

```bash
hyperbricks skills install --agent codex
hyperbricks skills install --agent gemini
hyperbricks skills install --agent claude --project /path/to/project
```

`--agent` accepts `all` (default), `codex`, `gemini`, or `claude`. Claude and
`all` need directory-symlink support; on Windows, use WSL or enable that support.
A symlink failure stops installation. `codex` and `gemini` need no Claude link.
Keep the installed skill and any Claude link in version control, including all
references. Git checkouts must preserve the Claude symlink as a symlink.

## Download the ZIP

From [hyperbricks.eu](https://hyperbricks.eu/), download **HyperBricks skill**
and its SHA-256 file for the version reported by `hyperbricks version`.
The files are named `hyperbricks-skill-<version>.zip` and
`hyperbricks-skill-<version>.zip.sha256`, with the leading `v` included in
`<version>`. Both are served under `/static/downloads/`.

For an offline installation, keep both files next to each other and run:

```bash
hyperbricks skills install --archive /path/to/hyperbricks-skill-<version>.zip
```

You can also install manually: verify the ZIP against its SHA-256 sidecar with
your system's checksum tool, then extract the complete `hyperbricks/` directory:

| Client | Put the extracted directory here, relative to your project root |
| --- | --- |
| Codex or Gemini CLI | `.agents/skills/hyperbricks/` |
| Claude Code | `.claude/skills/hyperbricks/` |

Preserve existing skills before replacing them. Copying only `SKILL.md` loses
the referenced documentation. A manual Claude-only installation may use a real
directory; for multiple clients use the CLI's shared directory and Claude link.
Manual copies have no CLI installation receipt and cannot be automatically
replaced using `--update`.

## Optional GitHub CLI installation

If you already use GitHub CLI with `gh skill install`, you can install the same
source directly from a published release tag:

```bash
gh skill install hyperbricks/hyperbricks .agents/skills/hyperbricks/SKILL.md \
  --allow-hidden-dirs --agent codex --scope project --pin <release-tag>
```

Choose `codex`, `claude-code`, or `gemini-cli` for the GitHub CLI's agent flag.
Use the exact runtime tag containing the shared skill source. This route is
optional; it is documented as public preview in GitHub CLI 2.90.0 and later.
Manage its updates through GitHub CLI. See the
[GitHub CLI manual](https://cli.github.com/manual/gh_skill_install).

## Check discovery

Start the agent in the project root and reload its skills or restart its session
after installation. Use the client's normal project trust and skill controls.

| Client | Discovery location | Check |
| --- | --- | --- |
| Codex | `.agents/skills/hyperbricks/` | Find `hyperbricks` in `/skills` or mention `$hyperbricks`. |
| Claude Code | `.claude/skills/hyperbricks/` symlink | Invoke `/hyperbricks`. |
| Gemini CLI | `.agents/skills/hyperbricks/` | Run `/skills reload`, then `/skills list`; check that `hyperbricks` is enabled. |

These locations follow the documented contracts for
[Codex](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills),
[Claude Code](https://code.claude.com/docs/en/skills#choose-where-skills-load), and
[Gemini CLI](https://geminicli.com/docs/cli/skills/#discovery-tiers).
Discovery support does not imply identical model behavior. If a client does not
list the skill, check its version, workspace trust, disabled skills, and the
actual directory it opened. Other agents can read `SKILL.md` and its references
explicitly; automatic discovery depends on their own supported locations.

Try a read-only first request:

> Use the HyperBricks skill to identify this project's runtime and module.
> Explain where its main page, template, and assets are defined. Do not change files.

Check that the response names existing files and the selected runtime. For a
later change, ask the agent to use `doctor` and `lint` where supported and verify
the affected behavior. `author` is useful when a change must respect existing
ownership, imports, inheritance, or editing contracts; it is not required merely
because an agent is doing the work. See [Changing Existing Projects](AUTHOR.md).

## Update and migrate

After upgrading HyperBricks, run:

```bash
hyperbricks skills install --update
```

Updates replace only an unchanged copy managed by this installer. Its receipt
records installed file hashes. Local edits, additions or removals stop the
update so you can preserve them. Keep application conventions in project
instructions. Repeating the same installation leaves skill content unchanged.

An existing independent `.claude/skills/hyperbricks` or
`.gemini/skills/hyperbricks` can conflict with the shared installation. Preserve
custom instructions and move the old copy out of discovery before retrying.
The installer does not overwrite project instructions or other skills.

The former Codex plugin is retired. Disable an installed HyperBricks plugin
in your client when switching to the project skill, and check user-level and
parent-project locations for older copies. The command manages this project's
skill only; it does not change personal plugins or global skills.

Inside the HyperBricks source repository, the shared directory and Claude link
are already present; no installation is needed. Developers regenerate its
references from their current canonical documentation.

## Project instructions and documentation provenance

The skill describes HyperBricks workflows. Project instructions describe your
application's conventions. Keep shared project instructions in `AGENTS.md`.
Where a client needs a context adapter, its root `CLAUDE.md` or `GEMINI.md` can
import that file using `@AGENTS.md`. Preserve existing client instructions when
adding an import; do not duplicate the full skill into context files. See the
[Claude context guide](https://code.claude.com/docs/en/memory#import-additional-files)
and [Gemini context guide](https://geminicli.com/docs/cli/gemini-md/).

The bundled manifest records the HyperBricks version and hashes of the source
and bundled documents. The skill first prefers documentation in the source
checkout actually used by the project, then a matching bundle or release tag.
A development version label alone cannot identify its exact source revision.
If documentation does not match the executable, establish the correct source
before relying on commands or fields.

Maintainers regenerate the shared snapshot from canonical `docs/` through
`scripts/sync_docs.sh` after README/reference are current, or through
`scripts/pre_release.sh` for the full preparation workflow. The release ZIP is
built from the committed skill tree with `scripts/package_agent_skill.py`.
There is no second skill source or plugin mirror.
