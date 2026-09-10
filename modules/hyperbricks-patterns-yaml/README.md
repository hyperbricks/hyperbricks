# hyperbricks-patterns-yaml

This module is a pattern library for HyperBricks.

Its job is to give agents and developers small, working examples of common HyperBricks composition patterns so they can copy an existing shape instead of inventing a new one.

## Browser Runtime

The shared browser entry imports the repository's pinned `htmx.org` 4.0.0 package. Request elements declare their own targets and `hx-swap="innerHTML"` so updates preserve the surrounding panel. These templates require no implicit attribute inheritance or compatibility extension.

Install the repository's pinned browser dependencies once before starting this module from a clean checkout:

```sh
npm ci
```

HyperBricks then rebuilds the ignored browser bundle from `resources/js/` when the module starts.

The browser helpers use HTMX 4's colon-separated events. Request diagnostics read `event.detail.ctx.request.action`; section scrolling reads the settled swap task's target. Listeners are attached to `document`, including history updates, so they also work when Back or Forward restores page content.

After rebuilding browser assets, check `/status-demo` navigation and diagnostics, `/menu-demo` and `/docs` content/sidebar updates, section-rail anchors, and the login and write forms. The menu and docs templates use `hx-select-oob` to update their sibling sidebar after the content swap. The docs links also replace the article header. API refresh probes listen for the configured `HX-Trigger` events. GET buttons that restart the workflow do not need the enclosing form's values.

The module follows HTMX 4's response handling: error responses can render in the declared feedback target, while `HX-Redirect` handles navigation when configured. See the official [HTMX 4 upgrade guide](https://raw.githubusercontent.com/bigskysoftware/htmx/v4.0.0/dist/skills/htmx-upgrade-from-htmx2.md) for the changed browser contract. The Unpoly demo keeps its own browser runtime.

## What This Module Contains

- `hyperbricks/` Pattern demo configs and routes.
- `templates/` Demo templates used by the pattern pages and fragments.
- `plugins/` Small demo plugins that show recommended plugin contracts.
- `docs/pages/` Markdown articles rendered by the demo at `/docs`.
- `docs/SOURCE_GUIDE.md` Developer-facing map of examples and source files.
- `resources/` Shared CSS and JS used by the demo module.

## How Agents Should Use This

Before proposing a new HyperBricks shape, check whether this module already has a working pattern for it.

Use this module when you need a concrete example for:

- canonical page routes plus HTMX fragment routes
- a full page and fragment updated with Unpoly
- menu navigation enhanced with HTMX
- config-driven section rails
- guarded pages and login/forbidden flows
- `API_FRAGMENT_RENDER` write actions
- one plugin handling multiple route actions
- deciding between `<PLUGIN>` and `API_FRAGMENT_RENDER`
- plugin-to-template handoff with `<TEMPLATE>`

The rule is simple:

- prefer copying an existing pattern from this module
- only invent a new pattern when none of these fits
- if you add a new pattern, add both a working demo and a short doc

## Install Or Build The Plugins

`HYPERBRICKS_LOCAL_PATH` is a development-only override for a local HyperBricks checkout. Normal plugin builds for an installed published release leave it unset. See [plugin build modes](../../docs/PLUGINS.md#local-runtime-development).

Repository maintainers can use the centralized [plugin build and smoke scripts](../../scripts/plugins/README.md). This focused command rebuilds the shared and module-local plugins against the current checkout:

```sh
scripts/plugins/build_hyperbricks_plugins.sh --module hyperbricks-patterns-yaml
```

Run the commands below from the project root with a compatible installed published `hyperbricks` release. A release installation does **not** require `HYPERBRICKS_LOCAL_PATH`; leave it unset. If you exported it in an earlier session, run `unset HYPERBRICKS_LOCAL_PATH` first.

For local-source development, install the CLI from this checkout and set the override so plugins use the same source:

```sh
go install ./cmd/hyperbricks
export HYPERBRICKS_LOCAL_PATH="$PWD"
```

Ensure Go's installation directory is on `PATH`. Installing with `go install ./cmd/hyperbricks` still produces a local-source build. Native plugins and their host must use matching source, toolchains, and shared dependencies.

Alternatively, run the local CLI directly:

```sh
HYPERBRICKS_LOCAL_PATH="$PWD" go run ./cmd/hyperbricks plugin build template-config-demo@2.0.0 --module hyperbricks-patterns-yaml
go run ./cmd/hyperbricks start -m hyperbricks-patterns-yaml --port 8080
```

The override selects the **HyperBricks source checkout**, not the plugin source directory. Building a plugin from its source files for an installed published release does not itself require this override.

### Option 1: Build Global Plugins From Source

The package enables the Markdown and Tailwind CSS plugins. Their source directories belong under the project's `plugins/` directory. `HYPERBRICKS_LOCAL_PATH` does not change where plugin sources are found. The shared JavaScript bundle uses HyperBricks' native `esbuild` component and needs no Esbuild plugin.

Rebuild global plugins whose source is already present:

```sh
hyperbricks plugin build markdown@2.0.0
hyperbricks plugin build tailwindcss@2.0.0
```

If a source directory is missing, run the matching `install` command instead. This downloads and builds the source for the selected runtime:

```sh
# Run only for plugins whose source is missing from plugins/.
hyperbricks plugin install markdown@2.0.0
hyperbricks plugin install tailwindcss@2.0.0
```

Use `build` for subsequent rebuilds, including after editing plugin source.

### Option 2: Install Published Global Plugins

`plugin install` downloads source from the plugin registry's repository and compiles it locally; it does not download a prebuilt native binary. It requires Git, Go, and network access. Use the same runtime as the server; set `HYPERBRICKS_LOCAL_PATH` only when targeting a local HyperBricks checkout.

For this module's configured versions, use the pinned `install` commands in Option 1. To select the highest published version instead, omit `@<version>`:

```sh
hyperbricks plugin list
hyperbricks plugin install markdown
hyperbricks plugin install tailwindcss
```

Omitting the version selects the highest semantic version in the registry, not necessarily the latest compatible version. Check the compatibility information before adopting it. The CLI does not treat `@latest` as an alias. `plugin update` is reserved but not implemented; it exits nonzero and makes no changes. Install the required version explicitly with `plugin install <name>@<version>`.

If an installed version differs from this module's pinned versions, update both the `hyperbricks.plugins.enabled` entries in `package.hyperbricks.yaml` and the corresponding `plugin:` references in the module YAML. Use the exact **Config name** printed by the installer. Installation does not update those references automatically. Native `esbuild` is configured as an ordinary component in `hyperbricks/partials/esbuild.hyperbricks.yaml`, outside the plugin list.

Installing again copies the published source into `plugins/<name>/<version>/`. Use `build` when you want to preserve and compile local source edits.

### Build The Included Module Plugins


The four demo plugins are module-local sources, not global registry installs. Their sources are included under this module's `plugins/` directory. Both global-plugin options above still require this step. Build them from the project root with `--module`:

```sh
hyperbricks plugin build template-config-demo@2.0.0 --module hyperbricks-patterns-yaml
hyperbricks plugin build guarded-demo-auth@1.0.0 --module hyperbricks-patterns-yaml
hyperbricks plugin build workflow-actions-demo@1.0.0 --module hyperbricks-patterns-yaml
hyperbricks plugin build route-split-demo@1.0.0 --module hyperbricks-patterns-yaml
```

Both global and module builds write their compiled plugins to `bin/plugins/`. Module plugin names include `__hyperbricks-patterns-yaml`; the matching names are already enabled in `package.hyperbricks.yaml`. Check that each build reports `Build successful`.

### Start Or Restart

Stop any running instance, then start the module again to load the rebuilt plugins:

```sh
hyperbricks start -m hyperbricks-patterns-yaml --port 8080
```

After changing the HyperBricks checkout, rerun `go install ./cmd/hyperbricks` if using the installed command, rebuild these plugins, and restart the server. Reloading a page does not replace a native plugin that is already loaded.

The API write and route-split examples call mock endpoints on this same server. Their default base URL is `http://127.0.0.1:8080`. When using a different port, set the matching address before starting:

```sh
PATTERNS_API_BASE_URL=http://127.0.0.1:8129 hyperbricks start -m hyperbricks-patterns-yaml --port 8129
```

Open `/guarded-demo/login` and use `demo` / `open-sesame` to check the login flow.

## Start Here

Read these files first:

- `docs/pages/module-overview.md` Short pattern list.
- `docs/pages/patterns-index.md` Learning order, live demo routes, and plain-language notes.

## Pattern Map

- `docs/pages/template-config-plugin.md` Use when a plugin should compute values and hand rendering to a template.
- `docs/pages/htmx-canonical-fragment-demo.md` Use when full page routes and fragment routes must stay separate.
- `docs/pages/menu-htmx-demo.md` Use when a real page menu should be progressively enhanced with HTMX.
- `docs/pages/config-driven-section-rail.md` Use when section navigation should be described in config and rendered by a shared shell.
- `docs/pages/api-fragment-write-demo.md` Use when a POST-style action should call a backend and render feedback.
- `docs/pages/single-plugin-many-actions.md` Use when several explicit routes belong to one workflow plugin.
- `docs/pages/plugin-vs-api-route-split.md` Use when choosing between a thin backend-forwarding route and a richer plugin-owned flow.
- `docs/pages/guarded-page-demo.md` Use when public login, protected pages, and forbidden redirects must work together.
- `docs/pages/unpoly-fragment-demo.md` Use when a browser client should replace a fragment through ordinary HTML routes and explicit HTTP response configuration.

## Live Demo Routes

These are the current demo entry points:

- `/index`
- `/docs`
- `/status-demo`
- `/menu-demo`
- `/section-rail-demo`
- `/api-fragment-write-demo`
- `/single-plugin-actions-demo`
- `/plugin-vs-api-route-split`
- `/guarded-demo`
- `/unpoly-demo`

## When Adding A New Pattern

Add all of these:

- one self-contained `.hyperbricks.yaml` demo in `hyperbricks/`
- templates in `templates/` if the pattern needs them
- a plugin in `plugins/` if the pattern is plugin-based
- a short explainer in `docs/pages/`
- an entry in `docs/pages/module-overview.md`
- an entry in `docs/pages/patterns-index.md`
- a docs route in `hyperbricks/90-docs.hyperbricks.yaml` and a matching link in `templates/patterns/docs-shell.html`

Keep the pattern small. It should teach one decision clearly.

## Project patterns guides

- [Module source guide](docs/SOURCE_GUIDE.md): find this module’s examples and source files by topic.
- [Recommended project patterns](../../docs/PROJECT_PATTERNS.md): choose and combine project structure, composition, server logic, and delivery patterns.
