---
name: hyperbricks
description: Set up, develop, troubleshoot, and package HyperBricks projects using the CLI, YAML components, templates, and recommended page and fragment patterns.
metadata:
  short-description: Build and manage HyperBricks projects
---

## HyperBricks

**HyperBricks** is a fullstack **web application build system and component runtime** for [hypermedia](https://hypermedia.systems/book/contents/) applications. It enables you to build dynamic, modular web applications by describing your app’s state, structure, and behavior in declarative configuration files — called *hyperbricks*.

HyperBricks is designed to provide full control over both the front-end and back-end of an application — without the complexity of traditional fullstack frameworks or CMSs.

With HyperBricks, you can:

* **Design** your application’s structure and interactive behavior using readable, reusable configs
* **Dynamically update** parts of your site without a full page reload (thanks to HTMX)
* **Maintain** full control over templates, routing, and rendering — with no boilerplate or JavaScript lock-in
* **Manage** state and logic for your app in a modular, versionable, and scalable way

HyperBricks uses `*.hyperbricks.yaml` configuration files to define pages and HTML
fragments, supply template data, and connect server-side logic. Templates control
the HTML markup using Go’s `html/template` and Sprig functions.

**YAML is the source format; the runtime contract is component-based.** YAML
source becomes ordered runtime configuration maps, which the runtime uses to
configure and dispatch registered components. When implementing or diagnosing
behavior, distinguish source parsing, runtime configuration, and component
execution, and make changes in the layer that owns the behavior.

HyperBricks’ native `esbuild` component bundles JavaScript, TypeScript, and CSS.
See the [esbuild component documentation](docs/ESBUILD.md) for usage. It uses
[esbuild](https://esbuild.github.io/), a third-party Go library for fast web asset bundling.

For server-side logic, projects can call APIs, run trusted JavaScript with `goja_render`, or use Go
plugins. The CLI creates and runs modules, exports static pages, and packages modules for deployment.

## Source Of Truth

Use the documentation for the HyperBricks version running the project. These
paths are relative to the HyperBricks repository:

- `docs/INTRODUCTION.md` and `docs/QUICKSTART.md`: introduction and first module.
- `docs/HYPERBRICKS_CLI.md`: commands, flags, module selection, and render diagnostics.
- `docs/YAML_USAGE.md`: component syntax, imports, inheritance, resolvers, and templates.
- `docs/REFERENCE.md`: component fields and supported values.
- `docs/ROUTING.md`: URL matching and route configuration.
- `docs/ESBUILD.md`: browser asset bundling.
- `docs/GOJA_RENDER.md`, `docs/API_RENDER.md`, and `docs/PLUGINS.md`: server-side logic.
- `docs/ROUTE_GUARD.md`: authentication and authorization checks on routes.
- `docs/DEPLOY.md`: archives, upload, activation, and runtime deployment.
- `docs/DOCKER.md`: Docker deploy host, configuration, persistence, and plugin builds.


In an application outside that repository, use the references included with
this skill and the installed CLI's `--help`. For complete manuals or component
source, use a [HyperBricks checkout](https://github.com/hyperbricks/hyperbricks)
at the matching release or revision. The application's own instructions and
module README describe its project-specific configuration.

## Project Structure

Run CLI commands from the project root, normally the directory containing
`modules/`. A standard module has these files and directories:

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

`-m demo` selects `modules/demo`. Selecting a module does not change the CLI's
working directory. The package configuration can override directory locations.

HyperBricks loads top-level `*.hyperbricks.yaml` files from the configured
`hyperbricks/` directory. Files in subdirectories require explicit `imports`.

## CLI Commands

Create and run a module:

```sh
hyperbricks version
hyperbricks init -m demo
hyperbricks start -m demo --port 8080
```

`init` creates the module configuration and starter files under `modules/demo/`.
`start` serves that module at `http://localhost:8080/`. For an existing module,
run `start` with its name. Stop the server with Ctrl+C.

Direct `start` also accepts a module directory:

```sh
hyperbricks start -m ./modules/demo
hyperbricks start -m demo --config profiles/development.hyperbricks.yaml
```

`--config` is relative to the selected module and must remain inside it. Other
commands retain their documented module-name options; inspect
`hyperbricks <command> --help` before using flags from another version.

Use the project's pinned or built binary when it needs unreleased components.
A development binary can retain an older version label; compare its source
revision as well when checking compatibility.

For configuration or rendering problems, inspect the server log and its JSON
diagnostic URL first; see [Troubleshooting and verification](#troubleshooting-and-verification).

See [Project lifecycle](references/project-lifecycle.md) for installation,
starters, module selection, and additional CLI commands.

## Package Configuration

`package.hyperbricks.yaml` uses ordinary YAML mappings. Runtime settings belong
under `hyperbricks`. For example, these settings enable development watching
and set the server port:

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    reload: true
  server:
    port: 8080
```

Merge those settings into the generated package file; retain its directories
and other application settings. Restart the server after changing the package
configuration.

## Components And Routes

Named top-level definitions in a `*.hyperbricks.yaml` file create components;
`imports` and `vars` are file-level settings. Each component's entries form an
ordered YAML sequence. The `type` field selects the component; other fields
configure it or contain child components.

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

This serves `/projects`. `projects_page` is the name used to reference the
component in YAML; `route: projects` sets its URL. `type: hypermedia` produces
the full HTML document, including the title. Its `content` child supplies the
`<h1>` inside that document.

Three component types can define routes:

| Type | Response |
| --- | --- |
| `hypermedia` | A complete HTML document. |
| `fragment` | Rendered child content without the document wrapper. |
| `api_fragment_render` | Calls the configured API endpoint and renders its response through a template. |

`route: index` serves `/`. A `template`, `html`, `tree`, or `api_render`
component renders within a page or fragment; it does not create a URL by itself.

Use lowercase `type` names in YAML. HyperBricks generates runtime `@type` and
`@order` metadata; do not write those fields in source files.

## Fields And Children

In the page above, `title` and `route` are fields. `content` contains another
component definition, with its own `type` and `value`. A child component can
declare its type or use `inherit` to reuse another definition.

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

Here `heading` renders before `description`. By contrast, fields such as
`values`, `headers`, and `data` contain ordinary data mappings. Their keys do
not specify the order in which child components render.

## Imports And Inheritance

Use file-level `imports` to load definitions from another YAML file:

```yaml
imports:
  - partials/cards.hyperbricks.yaml
```

The path is relative to the importing file. For example, in
`hyperbricks/app.hyperbricks.yaml`, this loads
`hyperbricks/partials/cards.hyperbricks.yaml`.

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

`featured_card` uses the same template and body text as `base_card`, with the
title `Featured`. Add it below a page or another rendering component to display
it; the reusable definition does not create a route.

When overriding a component stored inside a `values` mapping, repeat that
component's `inherit` reference. The nested value must still contain a complete
component definition. See the layout example in
[Authoring](references/authoring.md#move-reusable-pieces-without-losing-them).

## Vars And Resolvers

Resolvers supply values from configuration, variables, the environment, or
files. The resolver name determines what the result contains:

| YAML value | Result |
| --- | --- |
| `{var: site.title}` | The `title` value from file-level `vars.site`. |
| `{config: myconf.site.name}` | The value from the module's package configuration. |
| `{env: {name: SITE_NAME, default: Demo}}` | The environment value, or `Demo` when unset. |
| `{path: {base: resources, path: js/main.js}}` | A filesystem path to the JavaScript file. |
| `{file: {base: resources, path: copy/intro.txt}}` | The text read from the file. |
| `template: {file: cards/project.html}` | Loads a template from the configured templates directory. |

`base: module` starts at the selected module; `base: root` starts at the CLI
working directory. A disk path is not a browser URL. Files in the configured
static directory are served under `/static/`, regardless of that directory's
name on disk.

Flow mappings such as `{base: resources, path: js/main.js}` and their indented
block form are equivalent. Use `|` for multiline template or script content.
See [Resolvers and quoting](references/authoring.md#resolvers-and-quoting) for
string declarations and additional examples.

## Templates And Request Input

Templates use Go `html/template` with Sprig functions. Configured `values`
are available by name; allowed query parameters are available under `.Params`:

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

Place this component inside a page or fragment. For a request with `?q=atlas`,
the template displays `Search: atlas`. `querykeys: [q]` exposes only `q`;
`querykeys: []` exposes none. Validate input before using it in calculations
or backend operations. Repeated query keys produce a list rather than a string.

Set `nocache: true` on the containing page or fragment when its output depends
on request input. This controls HyperBricks' internal response cache; inspect
HTTP headers separately when configuring a browser or proxy cache.

Ordinary template values are HTML-escaped. Use `safe` only for HTML already
trusted by the project. Sprig expressions such as
`{{.tags | sortAlpha | join ", "}}` sort and format a list. Use keyed mappings
for lists of records under `values`: the current template preprocessing can
filter direct lists of maps. The navigation example in
[Authoring](references/authoring.md#configuration-and-navigation-data) shows the
supported structure.

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

`/help` returns a complete page containing `<main id="content">`. The URL
`/fragments/help` returns only the section produced by `help_content`.

In a shared page layout containing that same `#content` element, use:

```html
<a href="/help" hx-get="/fragments/help" hx-target="#content"
   hx-swap="innerHTML" hx-push-url="/help">Help</a>
```

With HTMX loaded, the link replaces the contents of `#content` and changes the
browser URL to `/help`. Without JavaScript, `href` opens the complete page.
Load HTMX once in the shared layout. Also include UTF-8 charset and viewport
metadata in its head; the [page recipe](references/authoring.md#one-view-a-full-page-and-a-fragment)
shows that configuration.

For menu generation, page and fragment navigation, and configuration-driven
sections, use the [patterns example module README](/path/to/hyperbricks/modules/hyperbricks-patterns-yaml/README.md).
Its version 2 topic guide points to the example source files. This module lives
in the separate testing project, outside the HyperBricks repository.

## Native esbuild

Use `esbuild` to bundle browser JavaScript, TypeScript, or ordinary CSS. Source
files belong in `resources/`; output must stay in the configured static directory.

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

Create `resources/js/main.js` and add `browser_script` to the page's `head`
using `inherit`. It builds the source and renders a script tag with the
fingerprinted public URL. Edit the source file, not the generated bundle.
No esbuild plugin installation is required.

For CSS configuration, head attachment, dependencies, and development watching,
see [Native asset bundling](references/authoring.md#native-asset-bundling).

## Server Logic And Route Guards

Choose a component for the operation you need:

| Requirement | Component |
| --- | --- |
| Format text or lists while rendering HTML | `template` with Sprig. |
| Calculate a result from configured values and query input | `goja_render`. Its synchronous `main(input)` returns data for the template. |
| Read API data as part of rendering a page | `api_render`, nested inside that page. |
| Provide a URL that calls an API and returns rendered feedback | `api_fragment_render`, with `route`, `endpoint`, and `method`. |
| Use Go libraries, storage, or custom request handling | A Go plugin. |

`goja_render` is beta and intended for trusted project scripts. It exposes no
Node.js, filesystem, or network APIs. Each execution receives a fresh JavaScript
runtime; its returned object is available in the template as `.Data`.

API templates receive the parsed API response in `.Data` and the upstream
status in `.Status`. The current `api_fragment_render` response to the browser
can be HTTP 200 even when the API returned an error. Use the API result to
decide whether a submitted form succeeded and whether another panel should refresh.

Add a `guard` to each page, fragment, or API action that requires access checks.
A rejected request stops before its child components render. Requiring a token
to be present does not validate it: configure `guard.authorize.endpoint` for
the service that checks access. The API or storage operation must also enforce
permissions when called directly.

See [Server logic and integrations](references/integrations.md) for calculation,
API, form, and guard examples. See [Plugins](references/plugins.md) for manifests,
exact plugin names, and building against the runtime used by the project.

## Static Output And Runtime Archives

Export static HTML and assets:

```sh
hyperbricks static -m demo
hyperbricks static -m demo --serve
```

The first command requests the module's routes through a local runtime and
writes the results to its configured render directory. The second serves
those generated files. Request-time calculations and server form actions
require a running HyperBricks server after deployment.

For a runtime deployment, build and run an archive:

```sh
hyperbricks build --hra -m demo
hyperbricks start --deploy -m demo --port 8081
```

The archive is written under `deploy/demo/`. `start --deploy` runs that packaged
module. `build --zip` provides the alternative runtime archive format.

Static export discovers routes as well as configured targets: `static.routes`
is not an allowlist. To export selected pages, use a separate package
configuration whose `hyperbricks.directories.hyperbricks` points to a directory
that loads only those page definitions. The Project Desk handbook example
includes such a configuration. For runtime archives, stage only the intended
source files; the archive builder does not apply Git ignore rules.
See [Delivery formats](references/project-lifecycle.md#choose-the-delivery-format).

## Troubleshooting And Verification

HyperBricks has built-in error reporting in the server log and a JSON render
diagnostics endpoint. Use these when diagnosing broken output or assessing
the runtime's error feedback; a missing value or incomplete page alone does
not establish that diagnostics are absent.

In development or debug mode, look for `Render diagnostics recorded` in the
log. An error-level entry includes a URL such as:

```text
http://localhost:8080/__hyperbricks/render-diagnostics?request_id=hb-12
```

Open or fetch the actual logged URL, preserving its host, port, and request ID.
The JSON identifies the request and route; its `errors` entries include `file`,
`path` (component path), `key`, `type`, and `err` (message), where available.
Use those details to locate the owning source, fix it, and request the affected
route again to verify the result. An HTTP 200 response can still have component
errors; check `X-Hyperbricks-Render-Error-Count` and use
`X-Hyperbricks-Request-ID` to correlate a rendered response with its record.

Without a request ID, `/__hyperbricks/render-diagnostics` returns the ten most
recent records. Warning-only records may exist without the error-level log
entry. The runtime retains the latest 200 records in memory; older links expire
and restarting clears them.

Configuration-load diagnostic links use `localhost` and the configured server
port; open them after startup completes. The endpoint is disabled in live mode,
and static exports omit the link because their temporary server stops. If a
setup failure prevents the server from starting, use the terminal error; the
endpoint is not available yet. Check the selected runtime version and mode
before interpreting an unavailable endpoint as missing error reporting.

| Problem | Check |
| --- | --- |
| `/projects` returns 404 | The selected module, `route: projects`, startup errors, and imports for nested YAML files. |
| A template value is empty | Its configured `values`, selected `querykeys`, and whether the template needs `.name`, `.Params.name`, or `.Data.name`. |
| CSS or JavaScript is missing | The source entry, build errors, configured static directory, and generated asset URL. |
| An edit is not visible | Loaded source file, watch directories, route cache, and whether the browser is viewing static output. |
| A plugin cannot load | Enabled plugin name, compiled filename, plugins directory, and runtime/toolchain compatibility. |

After changing a route, request its URL and check the response and server
diagnostics. For a fragment, confirm that the response contains no extra page
wrapper. After changing navigation, test direct access, HTMX updates, reload,
Back, and Forward. For forms, check a valid submission and a rejected one.

After packaging, inspect the archive and run the packaged module locally.
Additional diagnostics and commands are in
[Project lifecycle](references/project-lifecycle.md#develop-and-diagnose).
