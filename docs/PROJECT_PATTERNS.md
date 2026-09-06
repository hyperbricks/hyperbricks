# Recommended project patterns

HyperBricks describes how a request becomes HTML. YAML owns the route and
component structure, templates own the markup, and the CLI owns the module
lifecycle. Start with that division and add server or browser logic where the
application needs it.

The [Project Desk example](../modules/hyperbricks-basics/README.md) combines the
patterns below in one small application. Run it first, then follow its exercises
to add a page, reuse a panel, and change an estimate. The basic module uses local
sample data; the optional lessons introduce an API and a custom plugin.

These recommendations are for developers who know web development but are new
to HyperBricks. For the complete authoring contract, use
[YAML Usage](YAML_USAGE.md); for field names, use [Reference](REFERENCE.md).

## Choose a starting point

| Your task | Start with | Try it in Project Desk |
| --- | --- | --- |
| Create or run a project | A module and its package configuration | [README quick start](../modules/hyperbricks-basics/README.md#run-it) |
| Reuse a header, layout, or panel | Imports, inheritance, and template values | [Shared page shell](../modules/hyperbricks-basics/hyperbricks/partials/site.hyperbricks.yaml) |
| Format text or display a list | A template with YAML data and Sprig | [Project cards](../modules/hyperbricks-basics/templates/projects.html) |
| Make a page bookmarkable | A `hypermedia` route | [Page and fragment routes](../modules/hyperbricks-basics/hyperbricks/app.hyperbricks.yaml) |
| Update part of a page | A `fragment` route and HTMX | [Refreshable status panel](../modules/hyperbricks-basics/templates/status.html) |
| Keep menus consistent | Navigation described in data | [Shared navigation data](../modules/hyperbricks-basics/hyperbricks/partials/navigation.hyperbricks.yaml) |
| Bundle browser assets | Native `esbuild` | [CSS and JavaScript configuration](../modules/hyperbricks-basics/hyperbricks/partials/assets.hyperbricks.yaml) |
| Calculate a small result on the server | Trusted `goja_render` script | [Estimate script](../modules/hyperbricks-basics/resources/scripts/line-total.js) at `/estimate?quantity=3` |
| Render API data or submit a form | `api_render` or `api_fragment_render` | [Optional API routes](../modules/hyperbricks-basics/lessons/advanced/app.hyperbricks.yaml) |
| Run a custom server workflow | Explicit routes calling a plugin | [Optional plugin actions](../modules/hyperbricks-basics/lessons/plugin/hyperbricks/app.hyperbricks.yaml) |
| Protect a route | A route guard plus actual authorization | [Guarded settings lesson](../modules/hyperbricks-basics/docs/advanced.md#protect-the-settings-route) |
| Deliver the site | Public static snapshot or runtime archive | [Handbook and packaging exercise](../modules/hyperbricks-basics/README.md#choose-what-to-deliver) |

## 1. Start from a module

Run the CLI from the project root, normally the directory containing `modules/`:

```sh
hyperbricks version
hyperbricks init -m my-project
hyperbricks start -m my-project
```

`init` provides a working scaffold. Its `package.hyperbricks.yaml` is ordinary
YAML configuration; the files in `hyperbricks/` contain ordered component
definitions. Keep templates in `templates/`, source assets in `resources/`, and
public assets in `static/`. The `/static/` URL refers to the configured public
directory; a `path` resolver returns a filesystem path, not a public URL.

Use `hyperbricks <command> --help` for the installed flags. A development
checkout may contain features newer than its embedded version label; build and
verify the example against that checkout before assuming another installation
has the same components. See [CLI](HYPERBRICKS_CLI.md).

**Try it:** change the port, start the module, and open the new address. If a
file is missing, check the selected module and configured directory first.

## 2. Reuse structure with imports and inheritance

Keep a small entry file that explicitly imports shared definitions. Root YAML
files are discovered automatically; files in subdirectories need imports.
`inherit` copies a named definition and applies local overrides.

```yaml
imports:
  - partials/site.hyperbricks.yaml

projects_page:
  - inherit: basics_page
  - route: projects
  - title: Projects
```

This is the shape used by Project Desk's shared page shell. Keep names semantic
and references shallow enough to follow. Use ordinary maps for data and ordered
sequences for component objects. Order matters for rendered children.

**Try it:** add a second page using the shell, then change the shared footer.
Both pages should change. See [imports and inheritance](YAML_USAGE.md#imports).

## 3. Let templates own markup

A template receives simple data through `values`. It can also receive a
rendered component in a named slot, which lets the same layout display different
page content. Keep large HTML blocks in template files so their structure stays
easy to read.

```yaml
project_card:
  - type: template
  - inline: |
      <article>
        <h2>{{ .name | trim }}</h2>
        <p>{{ .summary | default "Details coming soon." }}</p>
      </article>
  - values:
      name: Atlas
      summary: A shared project handbook
```

Sprig helps with presentation, such as defaults, joining lists, and formatting
values. Take care with types: YAML source scalars are preserved as strings until
the consuming field converts them. Use explicit numeric conversion where needed;
the string `"false"` is not a Go boolean. Go templates escape ordinary values.
Use `safe` only for HTML that is already trusted.

**Try it:** change a card's data without editing its template. See
[Template Syntax](YAML_USAGE.md#template-syntax).

## 4. Give every page a real URL

Use `hypermedia` for a full page. A visitor should be able to open a project URL
directly, refresh it, or bookmark it and receive a complete page with initial
content. Use a `fragment` when a request only needs to replace a panel.

Project Desk's full pages and fragments reuse the same content definitions.
That keeps direct navigation and enhanced navigation in agreement. A fragment
returns the inside of the page's main content area, without another document
shell. See [Routing](ROUTING.md).

**Try it:** open `/projects` directly, then inspect its fragment response. The
page includes its layout; the fragment contains only the requested content.

## 5. Enhance ordinary links with HTMX

Keep `href` pointed at the page a visitor can open directly. The enhancement
can request a fragment and put the canonical page URL in browser history:

```html
<a href="/projects"
   hx-get="/fragments/projects"
   hx-target="#main-content"
   hx-swap="innerHTML"
   hx-push-url="/projects">Projects</a>
```

Here `#main-content` is the existing page container. This approach keeps a normal
link useful when browser JavaScript is disabled. Verify Back, Forward, reload,
active navigation, and page titles as part of the enhanced flow.

Another supported approach uses `menu` to request a canonical page and select
the needed part of its HTML. See the existing
[MENU example](../modules/hyperbricks-patterns-yaml/docs/menu-htmx-demo.md).
Choose the approach that fits the application; separate fragment routes are
useful, but not a universal requirement for every navigation link.

## 6. Describe navigation in configuration

Put navigation labels, canonical paths, and fragment paths in a small data map and
render that list through one shared template. A new section then needs one
navigation entry instead of duplicated HTML in every page.

Project Desk uses this pattern for its navigation. A larger application can use
route metadata with `menu` or a section rail when it needs more structure. The
[configuration-driven rail example](../modules/hyperbricks-patterns-yaml/docs/config-driven-section-rail.md)
shows the latter. Keep role-based visibility separate from actual authorization.

**Try it:** add a navigation label and its route, then check both direct and
HTMX navigation. The template should remain shared.

The example uses keyed maps for structured navigation and card collections.
The current template value preprocessing treats sequences differently and can
filter a direct list of maps. Use the verified map shape here, and keep numeric
prefixes on keys when a particular display order is needed. When overriding a
component inside a `values` map, repeat its `inherit` reference so the replaced
value remains a complete component.

## 7. Build assets through the native component

Keep source CSS and JavaScript under `resources/`. A native `esbuild` component
builds them into `static/`; its result supplies the public URL. Use that result
when fingerprints are enabled, since the generated filename can change.

Use ordinary CSS for the first project. Introduce tools such as Tailwind only
when the application needs them. Put browser behavior in small resource files
and use delegated events or appropriate HTMX lifecycle hooks when content is
replaced. See [native esbuild](ESBUILD.md).

**Try it:** edit a color or browser label, let development reload rebuild the
asset, and inspect the changed page. Refreshing generated output by hand should
not be part of the workflow.

## 8. Put logic in the component that fits

| Need | Good starting choice |
| --- | --- |
| Format text, iterate data, display a condition | Go template and Sprig |
| Small synchronous calculation from configured values and selected query input | `goja_render` |
| Render a read-only public API inside a page | `api_render` |
| Forward a form/API action and render its response | `api_fragment_render` |
| Access host services or implement a custom server workflow | A Go plugin |
| React to browser-only interaction | Browser JavaScript |

The estimate demonstrates `goja_render`: `quantity` comes from an explicit query
allowlist, the unit price comes from configuration, and the returned object is
rendered through `.Data`. Validate empty, repeated, and invalid input. Each
execution receives fresh JavaScript state; the component is not persistent
storage. It remains beta and is intended for trusted project scripts. See
[Goja Render](GOJA_RENDER.md).

**Try it:** request `?quantity=3`, then attempt to override the configured price
through the URL. Only the allowed quantity should affect the result.

## 9. Make API outcomes visible

The [optional API lesson](../modules/hyperbricks-basics/docs/advanced.md) adds a
local fixture service. It reads project data, accepts an edit, and returns real
success, validation, and conflict responses. State lives in that fixture's
memory and resets when it restarts; the lesson does not claim durable storage.

Use `api_render` for the read panel and `api_fragment_render` for the action.
The result template uses `.Data` and the upstream `.Status` to display feedback.
Refresh the affected panel after success so the screen reflects the server.

In the current implementation, an API fragment can return HTTP 200 to the
browser while `.Status` contains an upstream 422 or 409. A configured
`response.hx_trigger` is not conditional on upstream success. The lesson makes
feedback and refresh behavior depend on the actual upstream outcome. See
[API Render](API_RENDER.md).

**Try it:** submit a valid edit, an empty name, and an outdated revision. Check
both the feedback and whether the displayed project changed.

## 10. Keep custom workflow routes explicit

The [optional plugin lesson](../modules/hyperbricks-basics/docs/advanced.md)
shows explicit actions sharing one small plugin. The plugin computes values
and returns a template configuration; the normal renderer produces the HTML.
A synthetic `tree` is optional when several components need composing.

Use this when server-side behavior exceeds the simple template/API/script
contracts. Keep route selection visible in YAML and workflow decisions in the
plugin. The sample performs calculations and previews; it does not save data.
Build native plugins against the same HyperBricks checkout and compatible Go
toolchain as the host. See [Plugins](PLUGINS.md).

**Try it:** invoke both action routes and compare the template output. Both
should use the same plugin contract and report invalid input clearly.

## 11. Guard access before rendering

A guard belongs to a route owner: `hypermedia`, `fragment`, or
`api_fragment_render`. It can reject a request before child components or
upstream actions run. A token's presence alone does not establish permission;
the authorization service or plugin must validate access to the actual data.

The optional guarded settings lesson demonstrates missing, invalid, forbidden,
and allowed credentials using a loopback fixture. Those demonstration tokens
are for learning the flow. Use real authentication and authorization for a real
application. Hidden menu items do not protect a route. See
[Route Guard](ROUTE_GUARD.md).

**Try it:** request settings with each fixture credential and compare the
response with the corresponding direct API access.

## 12. Package the appropriate kind of output

Project Desk serves a public `/handbook` page and exports the same content as
`index.html` through a separate handbook configuration. That configuration loads
only the handbook route: `static.routes` adds export targets but does not limit
the runtime's discovered routes. The README's `stage_source.py --handbook`
command prepares this public configuration in a new project directory. The interactive
dashboard, request-specific estimate, API forms, and guarded lessons need a
running server. A snapshot records a result at build time; it does not turn a
server action into a browser implementation.

For a runtime archive, start with clean, declared source files. The example's
`SOURCE_FILES.txt` and `tools/stage_source.py` provide a reproducible source copy.
The archive builder does not generally follow `.gitignore`, so an ignored file
in a working module may still enter an archive. Inspect the archive contents.
The README walks through static export and building/starting an archive locally.
See [Deploy](DEPLOY.md).

**Try it:** serve the exported handbook without HyperBricks, then start the
runtime archive and use the estimate with two different quantities.

## Where these patterns come from

The existing [YAML patterns module](../modules/hyperbricks-patterns-yaml/README.md)
and the Composer module informed the example. Composer's shared page shell,
configuration-driven navigation, explicit page/fragment composition, and
template handoff show the same ownership rules at a larger scale.

The examples here use current native esbuild and simplified project data. They
do not require Composer's database, command bus, collaboration state, or runtime
gateway. Older examples remain useful references, but their deprecated asset
configuration, mock services, and demonstration authentication should not be
copied unchanged.

## Verification

Run [the module check](../scripts/test_hyperbricks_basics.py) from this checkout.
It exercises the documented module through real CLI and HTTP paths in a clean
temporary project. The module README records the checks, compatible checkout,
and browser exercises. A source example becomes a recommendation here after
its intended behavior has been verified in the integrated module; an inspected
older source file alone is not evidence that its entire application still runs.
