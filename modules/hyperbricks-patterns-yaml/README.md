# hyperbricks-patterns

This module is a pattern library for HyperBricks.

Its job is to give agents and developers small, working examples of common HyperBricks composition patterns so they can copy an existing shape instead of inventing a new one.

## Browser Runtime

The shared browser entry imports the repository's pinned `htmx.org` 4.0.0 package. Request elements declare their own targets and `hx-swap="innerHTML"` so updates preserve the surrounding panel. These templates require no implicit attribute inheritance or compatibility extension.

The browser helpers use HTMX 4's colon-separated events. Request diagnostics read `event.detail.ctx.request.action`; section scrolling reads the settled swap task's target. Listeners are attached to `document`, including history updates, so they also work when Back or Forward restores page content.

After rebuilding browser assets, check `/status-demo` navigation and diagnostics, `/menu-demo` and `/docs` content/sidebar updates, section-rail anchors, and the login and write forms. The menu and docs templates use `hx-select-oob` to update their sibling sidebar after the content swap. API refresh probes listen for the configured `HX-Trigger` events. GET buttons that restart the workflow do not need the enclosing form's values.

The module follows HTMX 4's response handling: error responses can render in the declared feedback target, while `HX-Redirect` handles navigation when configured. See the official [HTMX 4 upgrade guide](https://raw.githubusercontent.com/bigskysoftware/htmx/v4.0.0/dist/skills/htmx-upgrade-from-htmx2.md) for the changed browser contract. The Unpoly demo keeps its own browser runtime.

## What This Module Contains

- `hyperbricks/` Pattern demo configs and routes.
- `templates/` Demo templates used by the pattern pages and fragments.
- `plugins/` Small demo plugins that show recommended plugin contracts.
- `docs/` Short pattern explanations and the docs index.
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

## Build The Plugins

Run these commands from the project root, which contains `modules/` and `bin/`:

```sh
export HYPERBRICKS_LOCAL_PATH="$PWD"
```

Use a `hyperbricks` executable built from that same checkout. The export makes plugin builds use your local HyperBricks source instead of a published release. It applies to both global and module plugins in the current terminal session.

### Global Plugins

The package enables Markdown, Tailwind CSS, and the legacy Esbuild plugin. Their source directories belong under the project's `plugins/` directory. `HYPERBRICKS_LOCAL_PATH` does not change where plugin sources are found.

Rebuild global plugins whose source is already present:

```sh
hyperbricks plugin build markdown@2.0.0
hyperbricks plugin build tailwindcss@2.0.0
hyperbricks plugin build esbuild@2.0.0
```

If a source directory is missing, run the matching `install` command instead. This downloads the source and builds it using the same local checkout export:

```sh
# Run only for plugins whose source is missing from plugins/.
hyperbricks plugin install markdown@2.0.0
hyperbricks plugin install tailwindcss@2.0.0
hyperbricks plugin install esbuild@2.0.0
```

Use `build` for subsequent rebuilds, including after editing plugin source.

### Module Plugins

The demo plugin sources are included under this module's `plugins/` directory. Build them from the project root with `--module`:

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

After rebuilding the HyperBricks executable, rebuild these plugins against the same checkout and restart the server. Reloading a page does not replace a native plugin that is already loaded.

Open `/guarded-demo/login` and use `demo` / `open-sesame` to check the login flow.

## Start Here

Read these files first:

- `docs/README.md` Short pattern list.
- `docs/patterns-index.md` Learning order, live demo routes, and plain-language notes.

## Pattern Map

- `docs/template-config-plugin.md` Use when a plugin should compute values and hand rendering to a template.
- `docs/htmx-canonical-fragment-demo.md` Use when full page routes and fragment routes must stay separate.
- `docs/menu-htmx-demo.md` Use when a real page menu should be progressively enhanced with HTMX.
- `docs/config-driven-section-rail.md` Use when section navigation should be described in config and rendered by a shared shell.
- `docs/api-fragment-write-demo.md` Use when a POST-style action should call a backend and render feedback.
- `docs/single-plugin-many-actions.md` Use when several explicit routes belong to one workflow plugin.
- `docs/plugin-vs-api-route-split.md` Use when choosing between a thin backend-forwarding route and a richer plugin-owned flow.
- `docs/guarded-page-demo.md` Use when public login, protected pages, and forbidden redirects must work together.
- `docs/unpoly-fragment-demo.md` Use when a browser client should replace a fragment through ordinary HTML routes and explicit HTTP response configuration.

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
- a short explainer in `docs/`
- an entry in `docs/README.md`
- an entry in `docs/patterns-index.md`

Keep the pattern small. It should teach one decision clearly.

## Project patterns guides

- [Project patterns, version 2](docs/PROJECT_PATTERNS_V2.md): find examples by topic.
- [Original project patterns](docs/PROJECT_PATTERNS.md): retained for comparison.
