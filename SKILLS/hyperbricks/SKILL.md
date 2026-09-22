---
name: hyperbricks
description: Set up, scaffold, extend, troubleshoot, and package HyperBricks projects using the project-aware author command, native YAML components, templates, and page/fragment patterns. Use for source-owned Spaces CMS editing and native Markdown content workflows too.
metadata:
  short-description: Build and manage HyperBricks projects
---

## HyperBricks

HyperBricks is a native full-stack build system with an integrated rendering engine for hypermedia web applications. Configure and connect components in `*.hyperbricks.yaml` files. The engine renders them on the server as HTML pages and fragments using your templates.

- **Backend logic:** Call APIs, run trusted JavaScript with `goja_render`, or use native Go plugins.
- **Frontend assets:** Bundle JavaScript, TypeScript, and CSS with the built-in `esbuild` component.
- **Rendering:** Choose cached or dynamic output per page or fragment, or export static HTML and assets.
- **Deployment:** Use the CLI to create and run modules, build archives, and manage deployments.
- **Spaces:** Create pages from shared sources and declare which fields users can edit in the development browser editor.

Templates use Go `html/template` and Sprig functions. See [JavaScript and CSS](../../docs/ESBUILD.md) for asset bundling with the embedded [esbuild library](https://esbuild.github.io/).

YAML parsing, runtime configuration, and component execution are separate layers. The parser converts YAML into ordered configuration maps. The runtime reads those maps and calls registered components such as `hypermedia`, `fragment`, and `template`. When fixing behavior, change the layer that owns it.

## Source Of Truth

Use documentation and examples that match the HyperBricks runtime. When a binary is available, run `hyperbricks version` before selecting documentation.

Resolve sources in this order:

1. Follow the application's own instructions and module README for project-specific choices.
2. When a matching HyperBricks checkout is available, read repository files from that checkout. During core development, use its current Git revision.
3. Otherwise, read the public [HyperBricks repository](https://github.com/hyperbricks/hyperbricks) at the release tag matching the installed CLI. Check the reported version and its `v`-prefixed form when resolving the tag; do not substitute a newer release. Fetch a file with `https://github.com/hyperbricks/hyperbricks/blob/<tag-or-revision>/<path>` and browse a module directory with `https://github.com/hyperbricks/hyperbricks/tree/<tag-or-revision>/<path>`.
4. Use `main` for a development build or when the user explicitly requests current unreleased documentation. If no matching public tag or checkout exists, use the references bundled with this skill and the installed CLI's `--help`, and disclose that exact-version manuals were unavailable.

Paths beginning with `docs/`, `modules/`, `pkg/`, or `cmd/` below are relative to the HyperBricks repository root. They are not paths inside the installed skill:

- [Introduction](../../docs/INTRODUCTION.md) and [Quickstart](../../docs/QUICKSTART.md): introduction and first module.
- [CLI reference](../../docs/HYPERBRICKS_CLI.md): commands, flags, module selection, and render diagnostics.
- [Authoring](../../docs/AUTHOR.md): project context, schema-driven specs, preview/apply, and source-owned Space creation.
- [YAML usage](../../docs/YAML_USAGE.md): component syntax, imports, inheritance, resolvers, and templates.
- [Component reference](../../docs/REFERENCE.md): component fields and supported values.
- [HyperBricks type examples](../../docs/HYPERBRICKS_TYPE_EXAMPLES.md): commented YAML examples for all native types, source variants, prerequisites, and per-type field reference links.
- [How-to guides](../../docs/HOWTOS.md): practical introductions and examples for module setup, templates, routes, assets, plugins, and API components.
- [Pattern source guide](../../modules/hyperbricks-patterns-yaml/docs/SOURCE_GUIDE.md): standalone pattern demos and their source files.
- [Localized Spaces pattern](../../modules/hyperbricks-patterns-yaml/docs/pages/localized-spaces.md): runnable English/German Spaces pattern with two page sources and four routes.
- [Routing](../../docs/ROUTING.md): URL matching and route configuration.
- [Esbuild](../../docs/ESBUILD.md): browser asset bundling.
- [Spaces](../../docs/SPACES.md): source-owned frontend editing, CRUD, assets, metadata, and development configuration.
- [Markdown](../../docs/MARKDOWN.md): the native component, file constraints, and editable fields.
- [Goja render](../../docs/GOJA_RENDER.md), [API render](../../docs/API_RENDER.md), and [Plugins](../../docs/PLUGINS.md): server-side logic.
- [Route guards](../../docs/ROUTE_GUARD.md): authentication and authorization checks on routes.
- [Deployment](../../docs/DEPLOY.md): archives, upload, activation, and runtime deployment.
- [Docker deployment](../../docs/DOCKER.md): Docker deploy host, configuration, persistence, and plugin builds.


The bundled `references/` files work without a checkout. Use a matching checkout or public repository revision for full manuals, runnable modules, generated references, and component source.

Repository links resolve relative to this checkout. If the skill is installed separately, find those files in the matching checkout or public revision using the rules above.

## Project Structure

Run CLI commands from the project root, normally the directory containing `modules/`. A standard module has these files and directories:

```text
modules/demo/
  package.hyperbricks.yaml   Module settings and application configuration.
  hyperbricks/              YAML component definitions.
  templates/                Go HTML templates.
  resources/                Source CSS, JavaScript, scripts, and data files.
  static/                   Files served at /static/...
  rendered/                 Generated static HTML and assets.
bin/plugins/                Compiled plugins shared by the project.
```

`-m demo` selects `modules/demo`. Selecting a module does not change the CLI's working directory. The package configuration can override directory locations.

HyperBricks loads top-level `*.hyperbricks.yaml` files from the configured `hyperbricks/` directory. Files in subdirectories require explicit `imports`.

## CLI Commands

Create and run a module:

```sh
hyperbricks version
hyperbricks init -m demo
hyperbricks start -m demo --port 8080
```

`init` creates the module configuration and starter files under `modules/demo/`. `start` serves that module at `http://localhost:8080/`. For an existing module, run `start` with its name. Stop the server with Ctrl+C.

Direct `start` also accepts a module directory:

```sh
hyperbricks start -m ./modules/demo
hyperbricks start -m demo --config profiles/development.hyperbricks.yaml
```

`--config` is relative to the selected module and must remain inside it. Other commands retain their documented module-name options; inspect `hyperbricks <command> --help` before using flags from another version.

Use the project's pinned or built binary when it needs unreleased components. A development binary can retain an older version label; compare its source revision as well when checking compatibility.

For configuration or rendering problems, inspect the server log and its JSON diagnostic URL first; see [Troubleshooting and verification](#troubleshooting-and-verification).

See [Project lifecycle](references/project-lifecycle.md) for installation, starters, module selection, and additional CLI commands.

Use the command that matches the requested level of authoring:

- `hyperbricks scaffold` opens the interactive Bubble Tea wizard for a person at
  a terminal. It selects a module, category, starter, top-level YAML file, root
  name, and route/title where applicable, then reviews the library-generated YAML
  and staged assets before writing.
- `hyperbricks scaffold --non-interactive` runs that same starter workflow with
  named flags and is the path for agents and scripts. Use it when a built-in
  starter matches the requested root.
  Preview with `--dry-run --json`, then apply the same named options without
  `--dry-run`. It requires `--module`, `--type`, and `--file`; accepts `--source
  inline|file` for `template` and `markdown`; accepts `--name`, `--route`, and
  `--title`; and supports `--dry-run` and `--json`. It uses the same core YAML
  template library and planning path as the wizard. Omitted names receive unique
  starter-based defaults. Do not make a human construct JSON for this workflow.
- `hyperbricks space` is the human-facing creator for an inheriting Space from an
  existing Hypermedia source. Its wizard is interactive when no creation options
  are supplied; `--source`, `--name`, `--title`, `--route`, `--dry-run`, `--json`,
  or `--non-interactive` select its flag-driven path. Discover valid sources with
  `hyperbricks space --list --json` before supplying `--source`.
- `hyperbricks author` is the project-aware authoring API.
  Use it when the change must modify or compose existing roots, or account for
  ownership, inheritance, imports, nested files, children, editing contracts,
  custom properties, batches, or other source-aware integration. Do not choose
  `author` merely because the caller is an agent.

For `scaffold` and `space`, preview and inspect configuration/source changes; do not
promise runtime rendering or start a server/static export unless runtime work is
requested. Preview does not execute components or prove rendered appearance or HTMX
behavior. When runtime work is requested, use the initialized module and check the
affected routes, assets, and diagnostics.

Read [Scaffolding, Authoring, and Spaces](references/scaffolding.md) for the command
decision guide and [CLI reference](../../docs/HYPERBRICKS_CLI.md) for complete flags.

When using `author` and the owner is known, start with `context --target <shell> --json`. Otherwise, use `context --list --json` to find root names, types, routes, and source files. Select an actual target; do not guess the starter's shell name.

Use the focused context's example, bindings, import scope, and navigation convention. Read manual sections, type examples, or the schema only when a question remains. Prepare one batch with the context revision, preview it, and apply the same spec. Pass specs through stdin or temporary files; do not store them in the module root.

Apply reviewed specs with `--summary --json` to avoid printing the preview YAML again. Then run `author inspect --target <page> --json` with the selected module to check ownership, bindings, and editable fields. Repeat `--target` for several owners changed by the same operation.

Inspect an unchanged owner only when the operation changed its editing rules, template bindings, route ownership, or import scope. A new page's reference to a shared shell, home page, menu, or sibling does not require extra inspections. Check diagnostics. Request full context again only when you need source text or broader discovery.

For complete component examples, open the relevant section of [HyperBricks type examples](../../docs/HYPERBRICKS_TYPE_EXAMPLES.md). Use its commented YAML and source variants. Follow the component's field reference for further requirements and include that link in the answer.

Include required bindings and assets, or state their prerequisites. The examples show selected fields. When scaffolding, adapt them if focused context has no suitable pattern or the component is unfamiliar. Preserve existing project owners and conventions.

Use `add-root` for pages, fragments, reusable bricks, and Space sources;
`add-child` for existing source-owned targets; and `create-space` to instantiate
a discovered Hypermedia source. For a localized site, initialize the module,
add route-less page sources, then create each language/page Space separately.
See the [Localized Spaces pattern](../../modules/hyperbricks-patterns-yaml/docs/pages/localized-spaces.md)
for the resulting source and route layout.

Check `hyperbricks author --help` in the selected binary. If a matching
development checkout has the command but its built binary does not, use
`go run ./cmd/hyperbricks author` from that checkout. For scaffolding-only
requests, verify the preview and applied source changes without starting a server
or exporting static output unless requested.

## Package Configuration

`package.hyperbricks.yaml` uses ordinary YAML mappings. Runtime settings belong under `hyperbricks`. For example, these settings enable development watching and set the server port:

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    reload: true
  server:
    port: 8080
```

Merge those settings into the generated package file; retain its directories and other application settings. Restart the server after changing the package configuration.

## Components And Routes

Named top-level definitions in a `*.hyperbricks.yaml` file create components; `imports` and `vars` are file-level settings. Each component's entries form an ordered YAML sequence. The `type` field selects the component; other fields configure it or contain child components.

Create `modules/demo/hyperbricks/projects.hyperbricks.yaml`:

```yaml
projects_page:
  - type: hypermedia
  - route: projects
  - title: Projects
  - content:
      - type: html
      - value: '<h1>Projects</h1>'
```

This serves `/projects`. `projects_page` is the name used to reference the component in YAML; `route: projects` sets its URL. `type: hypermedia` produces the full HTML document, including the title. Its `content` child supplies the `<h1>` inside that document.

Three component types can define routes:

| Type | Response |
| --- | --- |
| `hypermedia` | A complete HTML document. |
| `fragment` | Rendered child content without the document wrapper. |
| `api_fragment_render` | Calls the configured API endpoint and renders its response through a template. |

`route: index` serves `/`. A `template`, `html`, `tree`, `markdown`, or `api_render` component renders within a page or fragment; it does not create a URL by itself.

Use lowercase `type` names in YAML. HyperBricks generates runtime `@type` and `@order` metadata; do not write those fields in source files.

## Fields And Children

In the page above, `title` and `route` are fields. `content` contains another component definition, with its own `type` and `value`. A child component can declare its type or use `inherit` to reuse another definition.

Use `tree` when several children should render in a specified order:

```yaml
introduction:
  - type: tree
  - heading:
      - type: html
      - value: '<h1>Welcome</h1>'
  - description:
      - type: text
      - value: Start with a project.
```

Here `heading` renders before `description`. By contrast, fields such as `values`, `headers`, and `data` contain ordinary data mappings. Their keys do not specify the order in which child components render.

## Imports And Inheritance

Use file-level `imports` to load definitions from another YAML file:

```yaml
imports:
  - partials/cards.hyperbricks.yaml
```

The path is relative to the importing file. For example, in `hyperbricks/app.hyperbricks.yaml`, this loads `hyperbricks/partials/cards.hyperbricks.yaml`.

Use `inherit` to copy a named component and override selected fields:

```yaml
base_card:
  - type: template
  - inline: '<article><h2>{{.title}}</h2><p>{{.body}}</p></article>'
  - values:
      title: Base title
      body: Shared description

featured_card:
  - inherit: base_card
  - values:
      title: Featured
```

`featured_card` uses the same template and body text as `base_card`, with the title `Featured`. Add it below a page or another rendering component to display it; the reusable definition does not create a route.

When overriding a component stored inside a `values` mapping, repeat that component's `inherit` reference. The nested value must still contain a complete component definition. See the layout example in [Authoring](references/authoring.md#move-reusable-pieces-without-losing-them).

## Vars And Resolvers

Resolvers supply values from configuration, variables, the environment, or files. The resolver name determines what the result contains:

| YAML value | Result |
| --- | --- |
| `{var: site.title}` | The `title` value from file-level `vars.site`. |
| `{config: myconf.site.name}` | The value from the module's package configuration. |
| `{env: {name: SITE_NAME, default: Demo}}` | The environment value, or `Demo` when unset. |
| `{path: {base: resources, path: js/main.js}}` | A filesystem path to the JavaScript file. |
| `{file: {base: resources, path: copy/intro.txt}}` | The text read from the file. |
| `template: {file: cards/project.html}` | Loads a template from the configured templates directory. |

`base: module` starts at the selected module; `base: root` starts at the CLI working directory. A disk path is not a browser URL. Files in the configured static directory are served under `/static/`, regardless of that directory's name on disk.

Flow mappings such as `{base: resources, path: js/main.js}` and their indented block form are equivalent. Use `|` for multiline template or script content. See [Resolvers and quoting](references/authoring.md#resolvers-and-quoting) for string declarations and additional examples.

## Templates And Request Input

Templates use Go `html/template` with Sprig functions. Configured `values` are available by name; allowed query parameters are available under `.Params`:

```yaml
search:
  - type: template
  - querykeys: [q]
  - inline: |
      <h2>{{.heading}}</h2>
      <p>Search: {{.Params.q}}</p>
  - values:
      heading: Search projects
```

Place this component inside a page or fragment. For a request with `?q=atlas`, the template displays `Search: atlas`. `querykeys: [q]` exposes only `q`; `querykeys: []` exposes none. Validate input before using it in calculations or backend operations. Repeated query keys produce a list rather than a string.

Set `nocache: true` on the containing page or fragment when its output depends on request input. This controls HyperBricks' internal response cache; inspect HTTP headers separately when configuring a browser or proxy cache.

Ordinary template values are HTML-escaped. Use `safe` only for HTML already trusted by the project. Sprig expressions such as `{{.tags | sortAlpha | join ", "}}` sort and format a list. Use keyed mappings for lists of records under `values`: the current template preprocessing can filter direct lists of maps. The navigation example in [Authoring](references/authoring.md#configuration-and-navigation-data) shows the supported structure.

## Pages And Fragments

Define reusable content once, then render it in both a page and a fragment:

```yaml
help_content:
  - type: template
  - inline: '<section><h1>{{.heading}}</h1></section>'
  - values:
      heading: Project help

help_page:
  - type: hypermedia
  - route: help
  - title: Project help
  - body:
      - type: template
      - inline: '<main id="content">{{.content}}</main>'
      - values:
          content:
            - inherit: help_content

help_fragment:
  - type: fragment
  - route: fragments/help
  - content:
      - inherit: help_content
```

`/help` returns a complete page containing `<main id="content">`. The URL `/fragments/help` returns only the section produced by `help_content`.

In a shared page layout containing that same `#content` element, use:

```html
<a href="/help" hx-get="/fragments/help" hx-target="#content"
   hx-swap="innerHTML" hx-push-url="/help">Help</a>
```

With HTMX loaded, the link replaces the contents of `#content` and changes the browser URL to `/help`. Without JavaScript, `href` opens the complete page. Load HTMX once in the shared layout. Also include UTF-8 charset and viewport metadata in its head; the [page recipe](references/authoring.md#one-view-a-full-page-and-a-fragment) shows that configuration.

For menu generation, page and fragment navigation, and configuration-driven sections, use [Pattern source guide](../../modules/hyperbricks-patterns-yaml/docs/SOURCE_GUIDE.md). Use [How-to guides](../../docs/HOWTOS.md) for short explanations and examples of common application tasks. Resolve both repository-relative paths through the Source Of Truth rules above.

## Spaces And Markdown

For requests to make a page editable, add a CMS, manage Space instances, or upload
and render Markdown, read [Spaces and Markdown](references/spaces-markdown.md).
Use the built-in features when the selected runtime supports them; no plugin
installation or compilation is required for this workflow. The published
v1.2.4-beta binary does not include these features. Verify the actual
runtime version and source revision before using them.

A **Space source** is a named root or orphan that resolves to `hypermedia` through loaded imports. A **Space** inherits that source in YAML and supplies its own route, title, editable content, and head metadata.

The source defines the editing rules. The frontend edits instances and their declared assets. Spaces does not add a component type, database, or separate application.

For a multi-page language switch, use one source per page shape and one Space per
language and page. Keep the template and `editable` declarations on the source;
override localized navigation values, `htmltag`, title, and content on each
instance. The [Localized Spaces pattern](../../modules/hyperbricks-patterns-yaml/docs/pages/localized-spaces.md)
shows the full import and route layout.

- On `template`, `editable.<key>` authorizes editing `values.<key>`.
- On `markdown`, `editable.file` or `editable.content` authorizes that direct field.
- Markdown renders exactly one declared `content` string or resources-relative
  `file`. It never selects a file from query parameters. Pages/fragments own routes.
- Configure the CMS under `hyperbricks.development.frontend_editing`. It is
  development-only, localhost-only by default, and read-only until writes are
  explicitly enabled. Native Markdown rendering also works in live/static output.
- Source YAML, resource files, and loaded imports remain the source of truth.
  Saving does not publish or refresh the browser; the existing watcher/cache
  configuration owns runtime refresh. Trash comments a managed import, not the file.

The reference includes a complete source/instance recipe, asset policies, metadata,
verification steps, and links to the maintained manuals. Use it when adapting an
existing module as well as when starting a new one.

## Native esbuild

Use `esbuild` to bundle browser JavaScript, TypeScript, or ordinary CSS. Source files belong in `resources/`; output must stay in the configured static directory.

```yaml
browser_script:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/main.js}
  - outfile:
      path: {base: static, path: js/app.js}
  - cache: true
  - fingerprint: true
  - enclose: '<script src="|" defer></script>'
```

Create `resources/js/main.js` and add `browser_script` to the page's `head` using `inherit`. It builds the source and renders a script tag with the fingerprinted public URL. Edit the source file, not the generated bundle. No esbuild plugin installation is required.

For CSS configuration, head attachment, dependencies, and development watching, see [Native asset bundling](references/authoring.md#native-asset-bundling).

## Server Logic And Route Guards

Choose a component for the operation you need:

| Requirement | Component |
| --- | --- |
| Format text or lists while rendering HTML | `template` with Sprig. |
| Calculate a result from configured values and query input | `goja_render`. Its synchronous `main(input)` returns data for the template. |
| Read API data as part of rendering a page | `api_render`, nested inside that page. |
| Provide a URL that calls an API and returns rendered feedback | `api_fragment_render`, with `route`, `endpoint`, and `method`. |
| Use Go libraries, storage, or custom request handling | A Go plugin. |

`goja_render` is beta and intended for trusted project scripts. It exposes no Node.js, filesystem, or network APIs. Each execution receives a fresh JavaScript runtime; its returned object is available in the template as `.Data`.

API templates receive the parsed API response in `.Data` and the upstream status
in `.Status`. Neither API component caches upstream responses: every component
execution makes a fresh API request. `api_render` is nested and has no `route`
or `nocache` field. Its parent `hypermedia` or `fragment` owns the rendered-output
cache policy; a parent cache hit skips the nested API call. Put `nocache: true`
on that route owner when every route request must fetch current API data.
`api_fragment_render` is itself a route owner and always bypasses the
rendered-output cache, so every invocation calls its upstream.

The current `api_fragment_render` response to the browser can be HTTP 200 even
when the API returned an error. Use the API result to decide whether a submitted
form succeeded and whether another panel should refresh.

Add a `guard` to each page, fragment, or API action that requires access checks. A rejected request stops before its child components render. Requiring a token to be present does not validate it: configure `guard.authorize.endpoint` for the service that checks access. The API or storage operation must also enforce permissions when called directly.

See [Server logic and integrations](references/integrations.md) for calculation, API, form, and guard examples. See [Plugins](references/plugins.md) for manifests, exact plugin names, and building against the runtime used by the project.

For native plugin builds, leave `HYPERBRICKS_LOCAL_PATH` unset with an installed published release. Set it only for development against local HyperBricks source, including a CLI installed from that checkout with `go install ./cmd/hyperbricks`. See [plugin build modes](references/plugins.md#module-local-source-versus-local-runtime).

## Static Output And Runtime Archives

Export static HTML and assets:

```sh
hyperbricks static -m demo
hyperbricks static -m demo --serve
```

`hyperbricks static` renders a new snapshot. With `--serve`, HyperBricks serves that snapshot after rendering completes. If a rendered target contains `api_render`, the API is called during rendering. To serve existing output without rebuilding it, use a standalone static file server; see [Static export boundaries](references/project-lifecycle.md#static-export-boundaries) for route discovery and runtime limitations.

For a runtime deployment, build and run an archive:

```sh
hyperbricks build --hra -m demo
hyperbricks start --deploy -m demo --port 8081
```

The archive is written under `deploy/demo/`. `start --deploy` runs that packaged module. `build --zip` provides the alternative runtime archive format.

For runtime archives, stage only the intended source files; the archive builder does not apply Git ignore rules. See [Delivery formats](references/project-lifecycle.md#choose-the-delivery-format).

## Troubleshooting And Verification

Use the server log and JSON render diagnostics endpoint to investigate broken output or check error reporting. A missing value or incomplete page does not show whether HyperBricks recorded diagnostics.

In development or debug mode, look for `Render failed`, `Render warning`, or `Render notice` in the log. A `diagnostics_url` field can contain a path such as:

```text
/__hyperbricks/render-diagnostics?request_id=hb-12
```

Open or fetch the logged path on the running server with the original request ID. The JSON identifies the request and route. Each `errors` entry can include `file`, `path` (component path), `key`, `type`, and `err` (message). When the development dashboard is enabled, use its **Errors** section or `/__hyperbricks/errors` to read these diagnostics in the developer interface.

Use those details to find the source, fix it, and request the route again. HTTP 200 can still include component errors. Check `X-Hyperbricks-Render-Error-Count` and use `X-Hyperbricks-Request-ID` to find the matching diagnostic record.

Without a request ID, `/__hyperbricks/render-diagnostics` returns up to ten current records containing diagnostics. The store retains the latest outcome for each request context, including healthy outcomes, up to 200 contexts. A successful retry clears that context's earlier error. Repeated requests replace earlier request IDs; eviction, configuration reload, and restart can also expire links.

Open configuration-load diagnostic links after startup. Use `/__hyperbricks/render-diagnostics?view=current` for all retained diagnostics and checked/unchecked route information. An empty error list does not prove every route or input was tested. The endpoint is disabled in live mode. Static exports omit the link because their temporary server stops.

If setup prevents startup, read the terminal error; the endpoint is not available yet. Check the runtime version and mode before concluding that error reporting is missing.

| Problem | Check |
| --- | --- |
| `/projects` returns 404 | The selected module, `route: projects`, startup errors, and imports for nested YAML files. |
| A template value is empty | Its configured `values`, selected `querykeys`, and whether the template needs `.name`, `.Params.name`, or `.Data.name`. |
| CSS or JavaScript is missing | The source entry, build errors, configured static directory, and generated asset URL. |
| An edit is not visible | Loaded source file, watch directories, route cache, and whether the browser is viewing static output. |
| Spaces is missing or read-only | Actual runtime/revision, development mode, nested `frontend_editing` configuration, host policy, and explicit `spaces.write`. |
| A field is absent from Spaces | The inherited source's `editable`, component owner, field identity, and active import graph; see [Spaces and Markdown](references/spaces-markdown.md). |
| Markdown shows a filename or fails to render | `file` versus `content`, configured resources root, extension/size, and source-aware render diagnostics. |
| A plugin cannot load | Enabled plugin name, compiled filename, plugins directory, and runtime/toolchain compatibility. |

After changing a route, request its URL and check the response and server diagnostics. For a fragment, confirm that the response contains no extra page wrapper. After changing navigation, test direct access, HTMX updates, reload, Back, and Forward. For forms, check a valid submission and a rejected one.

After packaging, inspect the archive and run the packaged module locally. Additional diagnostics and commands are in [Project lifecycle](references/project-lifecycle.md#develop-and-diagnose).
