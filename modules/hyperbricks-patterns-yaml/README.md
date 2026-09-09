# hyperbricks-patterns-yaml

This module is a pattern library for HyperBricks.

Its job is to give agents and developers small, working examples of common HyperBricks composition patterns so they can copy an existing shape instead of inventing a new one.

## Browser Runtime

The shared browser entry imports the repository's pinned `htmx.org` 4.0.0 package. Request elements declare their own targets and `hx-swap="innerHTML"` so updates preserve the surrounding panel. These templates require no implicit attribute inheritance or compatibility extension.

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

Run these commands from the project root, which contains `modules/` and `bin/`:

```sh
go build -o bin/hyperbricks-patterns ./cmd/hyperbricks
export HYPERBRICKS_LOCAL_PATH="$PWD"
```

The commands below use `./bin/hyperbricks-patterns`, built from this checkout. Native plugins and the runtime must share the same build inputs; matching the displayed version number alone is not sufficient. The export makes plugin builds use your local HyperBricks source instead of a published release. It applies to both global and module plugins in the current terminal session.

### Option 1: Build Global Plugins From Source

The package enables Markdown, Tailwind CSS, and the legacy Esbuild plugin. Their source directories belong under the project's `plugins/` directory. `HYPERBRICKS_LOCAL_PATH` does not change where plugin sources are found.

Rebuild global plugins whose source is already present:

```sh
./bin/hyperbricks-patterns plugin build markdown@2.0.0
./bin/hyperbricks-patterns plugin build tailwindcss@2.0.0
./bin/hyperbricks-patterns plugin build esbuild@2.0.0
```

If a source directory is missing, run the matching `install` command instead. This downloads the source and builds it using the same local checkout export:

```sh
# Run only for plugins whose source is missing from plugins/.
./bin/hyperbricks-patterns plugin install markdown@2.0.0
./bin/hyperbricks-patterns plugin install tailwindcss@2.0.0
./bin/hyperbricks-patterns plugin install esbuild@2.0.0
```

Use `build` for subsequent rebuilds, including after editing plugin source.

### Option 2: Install Published Global Plugins

`plugin install` downloads source from the plugin registry's repository and compiles it locally; it does not download a prebuilt native binary. It requires Git, Go, and network access. Use the same runtime binary and `HYPERBRICKS_LOCAL_PATH` export shown above.

For this module's configured versions, use the pinned `install` commands in Option 1. To select the highest published version instead, omit `@<version>`:

```sh
./bin/hyperbricks-patterns plugin list
./bin/hyperbricks-patterns plugin install markdown
./bin/hyperbricks-patterns plugin install tailwindcss
./bin/hyperbricks-patterns plugin install esbuild
```

Omitting the version selects the highest semantic version in the registry, not necessarily the latest compatible version. Check the compatibility information before adopting it. The CLI does not treat `@latest` as an alias. Use `install` for this operation; `plugin update` is currently a placeholder.

If an installed version differs from this module's pinned versions, update both the `hyperbricks.plugins.enabled` entries in `package.hyperbricks.yaml` and the corresponding `plugin:` references in the module YAML. Use the exact **Config name** printed by the installer. Installation does not update those references automatically. The shared JavaScript build uses native `esbuild`; the legacy Esbuild plugin is still enabled in this module's package.

Installing again copies the published source into `plugins/<name>/<version>/`. Use `build` when you want to preserve and compile local source edits.

### Build The Included Module Plugins


The four demo plugins are module-local sources, not global registry installs. Their sources are included under this module's `plugins/` directory. Both global-plugin options above still require this step. Build them from the project root with `--module`:

```sh
./bin/hyperbricks-patterns plugin build template-config-demo@2.0.0 --module hyperbricks-patterns-yaml
./bin/hyperbricks-patterns plugin build guarded-demo-auth@1.0.0 --module hyperbricks-patterns-yaml
./bin/hyperbricks-patterns plugin build workflow-actions-demo@1.0.0 --module hyperbricks-patterns-yaml
./bin/hyperbricks-patterns plugin build route-split-demo@1.0.0 --module hyperbricks-patterns-yaml
```

Both global and module builds write their compiled plugins to `bin/plugins/`. Module plugin names include `__hyperbricks-patterns-yaml`; the matching names are already enabled in `package.hyperbricks.yaml`. Check that each build reports `Build successful`.

### Start Or Restart

Stop any running instance, then start the module again to load the rebuilt plugins:

```sh
./bin/hyperbricks-patterns start -m hyperbricks-patterns-yaml --port 8080
```

After rebuilding the HyperBricks executable, rebuild these plugins against the same checkout and restart the server. Reloading a page does not replace a native plugin that is already loaded.

The API write and route-split examples call mock endpoints on this same server. Their default base URL is `http://127.0.0.1:8080`. When using a different port, set the matching address before starting:

```sh
PATTERNS_API_BASE_URL=http://127.0.0.1:8129 ./bin/hyperbricks-patterns start -m hyperbricks-patterns-yaml --port 8129
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
- [Recommended project patterns](../../docs/PROJECT_PATTERNS.md): follow the broader Project Desk learning path, including setup, composition, server logic, and packaging.
