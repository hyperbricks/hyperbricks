---
name: hyperbricks
description: Develop, change, review, troubleshoot, and package HyperBricks projects using documentation matched to the selected runtime. Use for HyperBricks applications, modules, YAML components, CLI workflows, and core tooling; do not use for unrelated web projects.
metadata:
  short-description: Develop and maintain HyperBricks applications
---

# HyperBricks

Use HyperBricks to develop and maintain full-stack web applications, including
their component configuration, server behavior, assets, and delivery.

Work from the existing project. Use documentation that matches the runtime in
use for product behavior, configuration, commands, and supported fields. Do not
infer those contracts from another framework or HyperBricks version.

## Working principles

- Work from the user's requested scope and the existing project.
- Separate project-specific conventions from HyperBricks product contracts.
- Read only the documentation required for the current task.
- Prefer project evidence over assumptions about names, paths, routes, ownership,
  or structure.
- Use only documented fields and commands.
- Do not silently combine documentation from different HyperBricks versions.
- Verify only the layers affected by the requested work.
- State when exact-version documentation or runtime verification is unavailable.

## Resolve the project, runtime, and documentation

Before relying on product documentation:

1. Read the project's own instructions, README files, configuration, and relevant
   source files.
2. Identify the module, executable, or source checkout actually used by the
   project. Do not assume the first `hyperbricks` executable on `PATH` is the one
   the project uses.
3. Determine the runtime version with the selected executable when possible.
4. If the project uses a HyperBricks source checkout, identify its Git revision.
   A development build can have a version label that does not fully identify its
   source state.
5. Read [the bundled documentation manifest](references/documentation-manifest.json)
   before relying on the bundled snapshot.

Select product documentation in this order:

1. The documentation in the HyperBricks checkout used to build or run the
   project.
2. The generated files under `references/docs/` when their manifest matches the
   selected runtime or source revision.
3. Documentation from the public release tag matching the selected runtime.

If none matches exactly, use the bundled generated files only as a disclosed
fallback.

Use project documentation for local choices and conventions. Use matched
HyperBricks documentation, the selected CLI's help, and matching source code for
product contracts.

If documentation and the executable disagree, check their provenance before
continuing. Prefer evidence closest to the executable in use and report the
mismatch. Never fill a documentation gap by inventing a field, flag, default, or
compatibility claim.

## Route the task

The links below open the bundled generated snapshot. When a matching checkout or
release source was selected, read the same repository paths there instead.

Read the smallest sufficient set. Documents in the same row are alternatives or
supplements; do not load all of them automatically.

| Intent | Start with |
| --- | --- |
| Understand HyperBricks or its application model | [Introduction](references/docs/INTRODUCTION.md) |
| Install HyperBricks or create a first module | [Quickstart](references/docs/QUICKSTART.md) |
| Follow a common task recipe | [How-to guides](references/docs/HOWTOS.md) |
| Check a source module before running or building it | [Doctor](references/docs/HYPERBRICKS_CLI.md#doctor) |
| Diagnose a runtime or rendered-output problem | [Troubleshooting](references/docs/TROUBLESHOOTING.md), then the affected feature |
| Change an existing project | [Changing Existing Projects](references/docs/AUTHOR.md) |
| Check the exact authoring contract | [Author Command Reference](references/docs/AUTHOR_REFERENCE.md) |
| Work with YAML, imports, inheritance, or resolvers | [YAML Usage](references/docs/YAML_USAGE.md) |
| Check component fields or supported values | [Component Reference](references/docs/REFERENCE.md) |
| Find a complete native type example | [Type Examples](references/docs/HYPERBRICKS_TYPE_EXAMPLES.md) |
| Work with routes | [Routing](references/docs/ROUTING.md) |
| Configure response status or headers | [HTTP Responses](references/docs/HTTP_RESPONSES.md) |
| Work with page and fragment navigation | [HTMX Fragments and Canonical URLs](references/docs/HTMX_FRAGMENTS_AND_CANONICAL_URLS.md) |
| Configure access checks | [Route Guard](references/docs/ROUTE_GUARD.md) |
| Investigate live-mode request behavior | [Live Mode HTTP](references/docs/LIVE_MODE_HTTP.md) |
| Work with Markdown | [Markdown](references/docs/MARKDOWN.md) |
| Work with editable Spaces | [Spaces](references/docs/SPACES.md) |
| Work with images | [Images](references/docs/IMAGES.md) |
| Build browser assets | [JavaScript and CSS](references/docs/ESBUILD.md) |
| Render data from an API | [API Render](references/docs/API_RENDER.md) |
| Run server-side scripts | [Server Scripts](references/docs/GOJA_RENDER.md) |
| Build or diagnose plugins | [Plugins](references/docs/PLUGINS.md) |
| Work with the runtime gateway | [Runtime Gateway](references/docs/RUNTIME_GATEWAY.md) |
| Check commands and flags | [CLI Reference](references/docs/HYPERBRICKS_CLI.md) |
| Build or deploy an application | [Deploy Guide](references/docs/DEPLOY.md) |
| Deploy with Docker | [Docker Deploy](references/docs/DOCKER.md) |
| Update an older project | [Migration Guide](references/docs/MIGRATION.md) |

Use the [Documentation Index](references/DOCUMENTATION_INDEX.md) when the task
does not fit the table or another document points to material not listed here.

[The documentation compilation](references/HyperBricks-Documentation.md) is a
publication fallback, not the normal skill input. Prefer the separate source
documents.

## Choose the task mode

### Change an existing project

Read the selected source's `docs/AUTHOR.md` first. Let that document determine
which supported workflow fits the change. Read `docs/AUTHOR_REFERENCE.md` only
when the exact machine-facing contract is needed. Do not duplicate its command
sequences, fields, or limits in `SKILL.md`.

After changing module source or package configuration, use the selected module
and package configuration with `hyperbricks doctor -m <module> --json` as the
default read-only static preflight when the selected CLI supports it. Add
`--config <file>` when the workflow uses an alternate package configuration.
Read [Doctor](references/docs/HYPERBRICKS_CLI.md#doctor) for its checks, output
contract, status meanings, and exit behavior. Add `--strict` only when the
acceptance policy requires warnings to fail. A `healthy` report confirms the
static checks Doctor performed. A `warning` report leaves the listed uncertainty
unresolved; `--strict` changes the acceptance policy and exit behavior only.
Neither status proves runtime, browser, integration, or delivery behavior.

### Diagnose

Observe the failure, logs, diagnostics, source, and selected runtime before
proposing a cause. If the selected CLI supports it and the problem may come from
module source or package configuration, run Doctor first with the selected
module and package configuration: `hyperbricks doctor -m <module> --json`. Add
`--config <file>` when the workflow uses an alternate package configuration,
and follow [Doctor](references/docs/HYPERBRICKS_CLI.md#doctor). Treat failures
and warnings as static evidence. A `warning` status remains unresolved even when
normal mode exits successfully. Then read `docs/TROUBLESHOOTING.md` and the
affected feature document for runtime, browser, external-service, plugin, and
packaging failures.

### Package or deploy

Read the delivery document for the requested target. Preserve the distinction
between source validation, generated output, packaged output, and a running
deployment. Do not expand packaging into deployment unless requested.

## Execute the task

Resolve the project, runtime, and documentation source; inspect the relevant
existing files and state; choose the smallest documented workflow that satisfies
the request; preserve unrelated files and conventions; and report the result,
documentation source, and checks performed.

Do not start a server, export, push, publish, deploy, or perform a mutating test
unless it belongs to the user's request. Use disposable or explicitly approved
targets for verification that writes application data.

## Verification boundaries

Choose checks proportional to the change:

- Source checks confirm that files and configuration have the intended form.
- Authoring checks confirm that a planned or applied source change satisfies the
  authoring contract.
- Runtime checks confirm that the selected application starts and serves the
  affected behavior.
- Browser checks confirm interactions and browser-visible assets.
- Integration checks confirm external services or plugins involved in the task.
- Delivery checks confirm generated or packaged output in its intended form.

A preview proves only the proposed source change. A successful source operation
does not prove that a route renders. A successful HTTP response does not prove
browser interaction. A built artifact does not prove that it runs.

Use the verification procedure from the relevant matched document. If a required
check cannot run, state what remains unverified and why. Never claim proof that
was not observed.

## Maintaining HyperBricks itself

Inside the HyperBricks repository, the current source documents under `docs/`
are authoritative for that revision.

The files under `references/docs/`, the documentation index, manifest, and
compilations are generated artifacts. Do not edit them directly. When product
knowledge is missing, add it to the appropriate source document first.

Use the repository's compilation tooling to regenerate the snapshots and run its
documentation, link, skill, and plugin synchronization checks after changing the
source documentation or `SKILL.md`.
