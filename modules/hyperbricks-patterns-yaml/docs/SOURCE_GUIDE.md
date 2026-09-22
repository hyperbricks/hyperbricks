# Patterns module source guide

Find the configuration, templates, and browser code for each example below. See the [module README](../README.md) for setup and the [How-to guides](../../../docs/HOWTOS.md) for short examples.

## Pages and fragments

Render shared content in a complete HTML document and in separate fragments. Each view has a page URL for direct access and history restoration.

- Configuration: [page and fragment definitions](../hyperbricks/20-htmx-canonical-fragment-demo.hyperbricks.yaml).
- Templates: [shared layout](../templates/patterns/layout-shell.html) and [panel navigation](../templates/patterns/status-demo.html).

## Menus inside a page

Generate navigation from page sections, titles, and ordering metadata. HTMX selects a content panel from each full-page response and updates the active sidebar.

- Configuration: [menu and page definitions](../hyperbricks/50-menu-htmx-demo.hyperbricks.yaml).
- Template: [menu placement and content panel](../templates/patterns/menu-htmx-shell.html).

## Sections and subsections

Generate links from configured labels, fragment names, target panels, and subsection anchors. The browser helper scrolls after a fragment update.

- Configuration: [navigation items and fragment routes](../hyperbricks/60-config-driven-section-rail.hyperbricks.yaml).
- Template: [section and subsection navigation](../templates/patterns/section-rail-shell.html).
- Browser behavior: [subsection scrolling](../resources/js/patterns-ui.js).

## Localized pages with Spaces

Use two route-less page sources and one shared template for four English and German Spaces. Each Space owns its URL and translated values.

- Live pages: [English home](/localized-spaces) and [German home](/localized-spaces/de).
- Configuration: [sources and managed imports](../hyperbricks/75-localized-spaces.hyperbricks.yaml).
- Template: [shared page markup](../templates/patterns/localized-spaces-page.html).
- Guide: [source, instance, route, and verification steps](pages/localized-spaces.md).

## Protected pages

Keep login public and guard protected routes. One demo plugin handles login, logout, and authorization; protect every page and action that requires access checks.

- Configuration: [public and protected routes](../hyperbricks/30-guarded-page-demo.hyperbricks.yaml).
- Plugin: [authentication example](../plugins/guarded-demo-auth/1.0.0/guarded_demo_auth_plugin.go).

## Forms and API responses

Map form fields to an API request, render feedback, and trigger a related panel update. The local mocks return HTTP 200 with different JSON shapes; they do not save data.

- Configuration: [API actions and refresh fragment](../hyperbricks/40-api-fragment-write-demo.hyperbricks.yaml).
- Templates: [form](../templates/patterns/api-fragment-write-demo.html) and [result](../templates/patterns/api-file-write-result.html).

## Plugin output in templates

Let a plugin prepare values and return a template configuration. The status demo renders that result in both a page and a fragment.

- Configuration: [plugin panel and its routes](../hyperbricks/20-htmx-canonical-fragment-demo.hyperbricks.yaml).
- Contract and source locations: [template-config plugin guide](pages/template-config-plugin.md).

## Several actions in one workflow

Declare landing, lookup, signup, and completion routes that call one plugin. Configuration selects each action; form fields carry the demo state between requests.

- Configuration: [workflow routes](../hyperbricks/70-single-plugin-many-actions.hyperbricks.yaml).
- Template: [workflow stage](../templates/patterns/workflow-actions-stage.html).

## Choosing an API route or a plugin

Compare an API-backed rename request with a plugin that validates input and computes an import plan. The API uses mocks; the plugin does not import files.

- Configuration: [API and plugin routes](../hyperbricks/80-plugin-vs-api-route-split.hyperbricks.yaml).
- Template: [comparison page](../templates/patterns/route-split-demo.html).

## JavaScript, CSS, and shared page setup

Build browser assets from source and attach them to the shared page head. Edit resource files rather than generated assets.

- Shared page: [document structure and head](../hyperbricks/partials/site.hyperbricks.yaml).
- JavaScript: [esbuild configuration](../hyperbricks/partials/esbuild.hyperbricks.yaml).
- CSS: [Tailwind configuration](../hyperbricks/partials/tailwind.hyperbricks.yaml).
- Browser behavior: [navigation and fragment lifecycle handling](../resources/js/patterns-ui.js).

## Publishing Markdown pages

Render these articles with the Markdown plugin inside a shared documentation layout. Configuration selects the files and page routes.

- Configuration: [documentation pages](../hyperbricks/90-docs.hyperbricks.yaml).
- Template: [documentation layout](../templates/patterns/docs-shell.html).

## Unpoly fragment replacement

Reuse a panel in a full page and a fragment. Unpoly replaces the panel; the ordinary link opens the full page when JavaScript is unavailable.

- Configuration: [page and fragment routes](../hyperbricks/85-unpoly-fragment-demo.hyperbricks.yaml).
- Contract and verification: [Unpoly guide](pages/unpoly-fragment-demo.md).

## Related guidance

For module setup, templates, server calculations, and static export, use [How-to guides](../../../docs/HOWTOS.md). For complete component contracts, consult [CLI](../../../docs/HYPERBRICKS_CLI.md), [YAML](../../../docs/YAML_USAGE.md), [routing](../../../docs/ROUTING.md), [assets](../../../docs/ESBUILD.md), [API rendering](../../../docs/API_RENDER.md), [guards](../../../docs/ROUTE_GUARD.md), [plugins](../../../docs/PLUGINS.md), and [deployment](../../../docs/DEPLOY.md).
