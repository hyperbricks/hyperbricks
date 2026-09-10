# Patterns module source guide

This guide helps you find examples of common HyperBricks tasks in the [patterns module](../README.md). It describes what each example covers and where its configuration and templates live. The implementation stays in the module so examples can evolve without copying their code into this document.

Use this guide to locate a working example and its source files. For guidance on selecting and combining patterns for module setup, templates, server calculations, and packaging, see [Recommended project patterns](../../../docs/PROJECT_PATTERNS.md).

## Pages and fragments

Build a shared page layout, insert content through template values, and reuse that content for both a complete page and a partial update. Give each page a direct URL while using separate fragment routes for panel updates.

The **HTMX canonical landing**, **Summary canonical page**, and **Settings canonical page** examples cover the shared layout, initial panel selection, page and fragment URLs, and local overrides of inherited configuration.

- Configuration: [page and fragment definitions](../hyperbricks/20-htmx-canonical-fragment-demo.hyperbricks.yaml).
- Templates: [shared layout](../templates/patterns/layout-shell.html) and [panel navigation](../templates/patterns/status-demo.html).

## Menus inside a page

Group pages into a menu, control their order, and render the menu inside a shared layout. Define ordinary and active menu items separately.

The **MENU + HTMX demo** uses page sections, titles, routes, and ordering metadata to generate navigation. Links fetch a full page and select its content panel. The sidebar is updated alongside the content to reflect the active page, while the browser address follows the selected page.

- Configuration: [menu and page definitions](../hyperbricks/50-menu-htmx-demo.hyperbricks.yaml).
- Template: [menu placement and content panel](../templates/patterns/menu-htmx-shell.html).

## Sections and subsections

Describe navigation in a configuration object when each item needs its own label, fragment, target panel, and optional subsection links.

The **Config-driven section rail** demonstrates a simple entry and grouped entries with subsections. Its template derives navigation from the configuration; browser code scrolls to a subsection after its fragment has loaded. This is distinct from the page-derived MENU example above.

- Configuration: [navigation items and fragment routes](../hyperbricks/60-config-driven-section-rail.hyperbricks.yaml).
- Template: [section and subsection navigation](../templates/patterns/section-rail-shell.html).
- Browser behavior: [subsection scrolling](../resources/js/patterns-ui.js).

## Protected pages

Keep public login pages separate from guarded content. Configure authentication input, authorization, and the responses for missing authentication or denied access. Inherit the protected configuration where it is needed.

The **Guarded page demo** connects login, logout, and authorization routes to one authentication plugin. It demonstrates one protected page; extending this to a section requires protecting each relevant page and action route.

- Configuration: [public and protected routes](../hyperbricks/30-guarded-page-demo.hyperbricks.yaml).
- Plugin: [authentication example](../plugins/guarded-demo-auth/1.0.0/guarded_demo_auth_plugin.go).

## Forms and API responses

Map form input to an upstream request, render the returned data as feedback, and refresh another part of the page after an action.

The **API fragment write demo** connects a form, API fragment routes, result templates, and a refresh event. Its upstream responses are local mocks: the example illustrates request and response handling, not durable storage or a complete demonstration of upstream HTTP failures.

- Configuration: [API actions and refresh fragment](../hyperbricks/40-api-fragment-write-demo.hyperbricks.yaml).
- Templates: [form](../templates/patterns/api-fragment-write-demo.html) and [result](../templates/patterns/api-file-write-result.html).

## Plugin output in templates

Let a plugin prepare values and return a template configuration. Keep the markup in a template, and include the rendered result in a page or fragment.

The **Plugin panel integration** example uses this handoff within the same page and fragment structure as the other status panels.

- Configuration: [plugin panel and its routes](../hyperbricks/20-htmx-canonical-fragment-demo.hyperbricks.yaml).
- Contract and source locations: [template-config plugin guide](../docs/pages/template-config-plugin.md).

## Several actions in one workflow

Keep related workflow decisions in one plugin while declaring each action route explicitly. Pass the selected action through configuration and let the plugin prepare the result for a template.

The **Single plugin, many actions** example includes landing, lookup, signup, and completion routes.

- Configuration: [workflow routes](../hyperbricks/70-single-plugin-many-actions.hyperbricks.yaml).
- Template: [workflow stage](../templates/patterns/workflow-actions-stage.html).

## Choosing an API route or a plugin

Use an API fragment when the route primarily maps input to an upstream call and renders its answer. Consider a plugin when local validation, normalization, or several processing steps must happen before rendering.

The **Plugin vs API route split** example places these approaches side by side: an API-backed rename flow and a plugin-owned import flow. The API side uses mock responses.

- Configuration: [API and plugin routes](../hyperbricks/80-plugin-vs-api-route-split.hyperbricks.yaml).
- Template: [comparison page](../templates/patterns/route-split-demo.html).

## JavaScript, CSS, and shared page setup

Keep browser source files separate from generated assets and include the resulting scripts and styles through the shared page configuration. Browser behavior that depends on replaced content needs to account for fragment updates.

These patterns support the module as a whole rather than having separate demo cards on its front page.

- Shared page: [document structure and head](../hyperbricks/partials/site.hyperbricks.yaml).
- JavaScript: [esbuild configuration](../hyperbricks/partials/esbuild.hyperbricks.yaml).
- CSS: [Tailwind configuration](../hyperbricks/partials/tailwind.hyperbricks.yaml).
- Browser behavior: [navigation and fragment lifecycle handling](../resources/js/patterns-ui.js).

## Publishing Markdown pages

Render Markdown through a plugin and insert it into a shared documentation layout. Keep document selection and routes in configuration.

The **Documentation** example brings the module overview and individual pattern guides together in one browser interface.

- Configuration: [documentation pages](../hyperbricks/90-docs.hyperbricks.yaml).
- Template: [documentation layout](../templates/patterns/docs-shell.html).

## Unpoly fragment replacement

The Unpoly example reuses a panel in a full page and a fragment, with a normal link fallback. It loads its own pinned client dependency.

- Configuration: [page and fragment routes](../hyperbricks/85-unpoly-fragment-demo.hyperbricks.yaml).
- Contract and verification: [Unpoly guide](pages/unpoly-fragment-demo.md).

## Related guidance

For setup, template formatting, server calculations, and packaging, use [Recommended project patterns](../../../docs/PROJECT_PATTERNS.md). For complete component contracts, consult [CLI](../../../docs/HYPERBRICKS_CLI.md), [YAML](../../../docs/YAML_USAGE.md), [routing](../../../docs/ROUTING.md), [assets](../../../docs/ESBUILD.md), [API rendering](../../../docs/API_RENDER.md), [guards](../../../docs/ROUTE_GUARD.md), [plugins](../../../docs/PLUGINS.md), and [deployment](../../../docs/DEPLOY.md).
