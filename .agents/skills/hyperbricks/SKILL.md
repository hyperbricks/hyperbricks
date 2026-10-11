---
name: hyperbricks
description: Develop, review, troubleshoot, and package HyperBricks modules and core tooling using documentation matched to the runtime. Use for HyperBricks YAML, templates, components, and CLI workflows.
metadata:
  short-description: Develop and maintain HyperBricks applications
---

# HyperBricks

Use the current agent's file, shell, and browser tools. No provider-specific
plugin is required; see [Agent setup](references/docs/AGENT_SETUP.md) for discovery.

## Select the right evidence

Use the project's selected executable and documentation from its source checkout
or matching release. A development build's version label alone may not identify
its source state. Before using bundled `references/docs/`, check the
[manifest](references/documentation-manifest.json); disclose a version mismatch
instead of mixing contracts. When the project already supplies matched docs,
use those without also loading the bundled copies.

The links below identify product documents. Read only the relevant sections;
with matched project docs, open the corresponding `docs/<filename>` there.

| Need | Start with |
| --- | --- |
| First project or common task | [Quickstart](references/docs/QUICKSTART.md), [How-to guides](references/docs/HOWTOS.md) |
| YAML, imports, inheritance, or fields | [YAML Usage](references/docs/YAML_USAGE.md), [Component Reference](references/docs/REFERENCE.md) |
| Module settings or CLI | [Package Configuration](references/docs/PACKAGE_CONFIGURATION.md), [CLI Reference](references/docs/HYPERBRICKS_CLI.md) |
| API data or response behavior | [API Render](references/docs/API_RENDER.md), [HTTP Responses](references/docs/HTTP_RESPONSES.md) |
| Page/fragment navigation | [HTMX Fragments](references/docs/HTMX_FRAGMENTS_AND_CANONICAL_URLS.md) |
| Editable content | [Spaces](references/docs/SPACES.md) |
| Runtime failure | [Troubleshooting](references/docs/TROUBLESHOOTING.md), then the affected feature |

For assets, scripts, plugins, delivery, and other features, use the
[Documentation Index](references/DOCUMENTATION_INDEX.md).

## Resolve composition before changing it

Run source-module commands from the project root containing `modules/`, for
example `hyperbricks start -m <module>`. Selecting a module does not change the
working directory; bare relative package paths still resolve from that directory.

Only files directly inside the configured HyperBricks source directory load
automatically. Nested files need explicit imports, relative to the importing
file. Imported files load before that file; preserve declared import order.
Imports near the top are a convention, not a syntax requirement. Use actual
component names and effective inherited values to locate the owner of a change.
See [Imports](references/docs/YAML_USAGE.md#imports).

For existing modules, choose direct edits when the affected owners and values
are clear. When imports or inheritance leave scope uncertain, consider focused
read-only `author context` or `author inspect`. Decide separately whether
`author apply` supports the requested mutation: manual template edits can still
benefit from read-only discovery. `author` remains optional; choose operations
only when their documented scope fits the task.
Read [Author](references/docs/AUTHOR.md) for the chosen operation; use
[Author Reference](references/docs/AUTHOR_REFERENCE.md) for exact JSON contracts.

For new definitions, follow the existing module and
[File Organization](references/docs/YAML_USAGE.md#file-organization):

- Prefer a separate top-level source file per root Hypermedia page; import
  reusable definitions from folders such as `assets/` and `partials/`.
- Leave a blank line between root component definitions. Preserve the documented
  Spaces structure.
- Apply these conventions within the change's scope; they do not justify
  reorganizing an existing module.

## Verify the affected behavior

After changing YAML, run `hyperbricks lint <changed-files...> --json` when
supported. Follow its quote/type diagnostics; preserve quotes required by YAML.
For module or package changes, use `hyperbricks doctor -m <module> --json` as the
static preflight when supported, selecting the same `--config` when applicable. Read the
[CLI Reference](references/docs/HYPERBRICKS_CLI.md) for diagnostics and exit status.
Doctor warnings remain unresolved uncertainty; a healthy result proves its
static checks only.

Prefer existing focused tests. For shared composition or template changes,
check affected routes and a representative unchanged consumer. Include relevant
input state, initial-page and fragment behavior, and success/empty/error cases
where affected. Check that shared presentation and controls remain consistent.
Exercise browser interactions when material to the task; HTTP responses alone
do not prove them. Use disposable state for mutating checks and report material
layers that could not be verified. A source preview does not prove rendering;
packaging does not prove deployment.

Within the HyperBricks repository, product behavior belongs in canonical
`docs/` sources. This skill is maintained in `.agents/skills/hyperbricks/`;
its documentation references are generated outputs, not editing targets.
