---
name: hyperBricks-basic-cli-skills
description: Work on HyperBricks YAML routes, fragments, templates, plugins, CLI workflows, and deploys
metadata:
  short-description: Manage HyperBricks YAML projects with the CLI
---

# HyperBricks Skill For Codex

Use this skill when working on HyperBricks projects, `*.hyperbricks.yaml`
component source files, `package.hyperbricks.yaml`, CLI workflows, routes,
fragments, templates, plugins, static rendering, deploy archives, or plugin
development.

HyperBricks is a Go-based system for building and serving hypermedia web
applications from modular YAML source files. It supports full pages, HTMX
fragments, static rendering, dynamic rendering, Go templates, plugin-based
components, Tailwind CLI integration, and optional JS/TS bundling through
plugins.

## Source Of Truth

Before answering details, prefer the repo docs over memory:

- `docs/YAML_USAGE.md` for YAML syntax, ordering, imports, inheritance, value
  resolvers, and recovery behavior.
- `docs/REFERENCE.md` for generated component fields and fixture examples.
- `docs/HYPERBRICKS_CLI.md` for CLI commands and flags.
- `docs/API_RENDER.md` for `api_render` and `api_fragment_render`.
- `docs/ROUTE_GUARD.md` for route guard behavior.
- `docs/PLUGINS.md` for plugin naming, manifests, and YAML usage.
- `docs/DEPLOY.md`, `docs/DOCKER.md`, and `docs/RUNTIME_GATEWAY.md` for
  deploy-specific workflows.

## Core Mental Model

A HyperBricks module contains `*.hyperbricks.yaml` files in its `hyperbricks/`
directory. Each source file is a top-level YAML mapping. Most top-level keys
define named HyperBricks objects.

Each component object is an ordered sequence of single-key entries:

```yaml
page:
  - type: hypermedia
  - route: index
  - title: Home
  - main:
      - type: tree
      - hero:
          - type: html
          - value: |
              <h1>Hello</h1>
```

Source order matters. The YAML loader materializes component sequence order into
runtime `@order` metadata. Authors should not write `@type` or `@order` in YAML
source.

Use lowercase type names in YAML:

```yaml
- type: hypermedia
- type: fragment
- type: api_render
- type: api_fragment_render
- type: tree
- type: head
- type: template
- type: html
- type: text
- type: css
- type: javascript
- type: js
- type: image
- type: images
- type: json
- type: json_render
- type: menu
- type: plugin
- type: styles
```

The runtime normalizes known YAML type names to runtime tokens such as
`<HYPERMEDIA>` and `<TEMPLATE>`.

## Components And Routes

Root route owners initiate frontend output:

- `hypermedia` for full pages.
- `fragment` for HTMX partial responses.
- `api_fragment_render` for HTMX/API fragment routes backed by upstream calls.

Example page:

```yaml
page:
  - type: hypermedia
  - route: index
  - title: Welcome
  - content:
      - type: html
      - value: <p>Hello from HyperBricks.</p>
```

Example fragment:

```yaml
counter:
  - type: fragment
  - route: counter
  - body:
      - type: html
      - value: <span>1</span>
```

Use `tree` for nested structure, `template` for reusable Go template rendering,
`api_render` for remote API data rendered during page rendering, and
`api_fragment_render` for route-owned API fragments.

## Fields And Children

A scalar or data mapping entry is a field:

```yaml
hero:
  - type: html
  - value: <h1>Hello</h1>
  - enclose: <section>|</section>
```

An entry whose value is another ordered component sequence is a child:

```yaml
main:
  - type: tree
  - intro:
      - type: text
      - value: Welcome
```

Maps under fields such as `values`, `data`, `headers`, `queryparams`, and
package configuration are data maps. They are not render-order containers.

## Imports And Inheritance

Use file-level `imports` to load shared YAML source before the current file:

```yaml
imports:
  - partials/site.hyperbricks.yaml
  - partials/cards.hyperbricks.yaml

page:
  - type: hypermedia
  - route: index
  - hero:
      - inherit: shared_hero
```

Relative import paths are resolved from the importing file's directory.

Use `inherit` to deep-copy another named object and override selected fields:

```yaml
base_card:
  - type: template
  - template:
      file: cards/card.html
  - values:
      title: Base title
      body: Base body

featured_card:
  - inherit: base_card
  - values:
      title: Featured
```

Inheritance references use dotted paths such as `base_card`, `layout.header`, or
`page.main.hero`.

## Vars And Resolvers

Use file-level `vars` for reusable source values:

```yaml
vars:
  page:
    title: Home

page:
  - type: hypermedia
  - route: index
  - title:
      var: page.title
```

Use YAML resolver mappings instead of old marker strings:

```yaml
title:
  env:
    name: SITE_TITLE
    default: HyperBricks

asset:
  path:
    base: static
    path: css/app.css

content:
  text:
    file: docs/intro.md

card:
  - type: template
  - template:
      file: cards/card.html
  - values:
      title:
        format: "%s card"
        args:
          - var: page.title
```

Common runtime variables include `module_root`, `root`, `module`, `resources`,
`templates`, `static`, `hyperbricks`, and `render`.

Template files are loaded with `template.file`; text/resource files use the
current YAML resolver model. Do not use `{{TEMPLATE:...}}`, `{{TEXT:...}}`,
`{{FILE:...}}`, `{{VAR:...}}`, or `{{ENV:...}}`.

## Templates

HyperBricks templates use Go `html/template` with Sprig functions plus
HyperBricks helpers such as `safe`, `random`, and `valueOrEmpty`.

Go template syntax remains literal YAML content:

```yaml
card:
  - type: template
  - inline: |
      <article>
        <h2>{{ .title }}</h2>
        <p>{{ .body }}</p>
      </article>
  - values:
      title: YAML templates
      body: Rendered from YAML data
```

Use YAML block scalars for multiline HTML, CSS, JavaScript, JSON, text, or
inline template content.

## Route Guards

Route guards apply only to route-owning components: `hypermedia`, `fragment`,
and `api_fragment_render`.

Example:

```yaml
guarded_page:
  - type: hypermedia
  - route: private
  - guard:
      enabled: true
      auth:
        cookie: token
        header: Authorization
        scheme: Bearer
      require:
        authenticated: true
      on_unauthenticated:
        redirect: /login
        hx_redirect: /login
        status: 401
```

If a guard denies the request, children do not render, plugins do not execute,
templates do not render, and `api_fragment_render` does not call its upstream
endpoint.

## Project Structure

A standard module looks like:

```text
modules/<module>/
  hyperbricks/
  rendered/
  resources/
  static/
  templates/
  package.hyperbricks.yaml
```

Directory purposes:

```text
hyperbricks/                 YAML component source files.
rendered/                    Static output from `hyperbricks static`.
resources/                   Raw assets, JS sources, markdown, and data.
static/                      Public files served directly.
templates/                   Go HTML templates used by template providers.
package.hyperbricks.yaml     Module entrypoint and runtime config.
```

The runtime scans `*.hyperbricks.yaml` files in the configured `hyperbricks/`
directory. Subdirectories are not automatically loaded; use YAML `imports` when
shared files live below nested directories.

Run HyperBricks CLI commands from the repository or project root, normally the
directory that contains `modules/`.

## Package Configuration

`package.hyperbricks.yaml` is normal YAML configuration, not an ordered
component source file. Runtime configuration lives under `hyperbricks`.

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    reload: true
    frontend_errors: false
    dashboard: false
  server:
    port: 8080
    beautify: true
  directories:
    render:
      path:
        base: module
        path: rendered
    static:
      path:
        base: module
        path: static
    resources:
      path:
        base: module
        path: resources
    plugins: ./bin/plugins
    templates:
      path:
        base: module
        path: templates
    hyperbricks:
      path:
        base: module
        path: hyperbricks
```

Other top-level objects can be used for organization and config resolvers, but
only `hyperbricks` is runtime configuration.

## CLI Commands

Main CLI:

```bash
hyperbricks [command]
```

Important commands:

```text
build          Build runtime archive `.hra` or `.zip`.
init           Create package.hyperbricks.yaml and module directories.
init-starter   List or install official starters.
plugin         Manage plugins.
select         Select active module.
start          Start server.
static         Render static content.
version        Show version.
```

Always inspect command-specific flags with:

```bash
hyperbricks <command> --help
```

Common workflows:

```bash
hyperbricks init -m demo
hyperbricks start -m demo
hyperbricks static -m demo
hyperbricks build --hra -m demo
hyperbricks build --zip -m demo
```

Deploy runtime commands:

```bash
hyperbricks start --deploy -m demo
hyperbricks start --deploy-remote
hyperbricks start --deploy-local
hyperbricks start --deploy-init-config local
hyperbricks start --deploy-init-config remote
```

Starter commands:

```bash
hyperbricks init-starter list
hyperbricks init-starter get hello-world -m demo
hyperbricks init-starter get hello-world@1.0.0 -m demo
```

The target module directory must be missing or empty when installing a starter.

## Runtime Gateway

Runtime gateway is disabled by default. It is a host-based proxy hook for
isolated runtime views.

CLI example:

```bash
hyperbricks start -m my-module --port 8080 \
  --runtime-gateway \
  --runtime-domain runtime.local \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

Flat host suffix example:

```bash
hyperbricks start -m my-module --port 8080 \
  --runtime-gateway \
  --runtime-host-suffix -runtime.example.test,-live.example.test \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

Package config example:

```yaml
hyperbricks:
  server:
    runtime_gateway:
      enabled: true
      domain: runtime.local
      host_suffix: -runtime.example.test
      resolver: http://127.0.0.1:8080/resolve-runtime
```

Rules:

- The resolver must be an internal trusted endpoint.
- Startup fails if gateway is enabled without a domain or host suffix and a
  resolver.
- CLI flags override config.
- Runtime hosts must resolve to the HyperBricks server.

## Plugin Model

HyperBricks plugins are Go `.so` files. They are compiled into `./bin/plugins`
by default and enabled from `package.hyperbricks.yaml`.

Two plugin source types exist:

```text
Global plugins   Shared public plugins from ./plugins.
Custom plugins   Module-level plugins from modules/<module>/plugins.
```

Enable plugins with the compiled binary name without `.so`:

```yaml
hyperbricks:
  plugins:
    enabled:
      - ExamplePlugin@1.0.0
      - CustomWidget__demo@1.0.0
```

Use a plugin component with the same exact config name:

```yaml
page:
  - type: hypermedia
  - route: plugin-demo
  - main:
      - type: plugin
      - plugin: ExamplePlugin@1.0.0
      - data:
          title: Rendered by a plugin
```

Plugin commands:

```bash
hyperbricks plugin list
hyperbricks plugin install example@1.0.0
hyperbricks plugin build example@1.0.0
hyperbricks plugin build markdown-wasm@1.0.0
hyperbricks plugin build widget@1.0.0 --module demo
hyperbricks plugin update example
hyperbricks plugin remove example@1.0.0
```

Global plugin pattern:

```text
<Binary>@<version>
```

Custom module plugin pattern:

```text
<Binary>__<module>@<version>
```

Do not include `.so` or `.wasm` in `plugins.enabled` or plugin component
configuration.

## Plugin Manifest

Each plugin version has a `manifest.json`.

```json
{
  "plugin": "github.com/hyperbricks/plugins/example",
  "source": "example_plugin.go",
  "version": "1.0.0",
  "binary": "ExamplePlugin",
  "compatible_hyperbricks": [">=1.1.0-beta"],
  "description": "Example plugin"
}
```

Required fields:

```text
plugin
source
version
compatible_hyperbricks
description
```

Optional field:

```text
binary
```

If `binary` is omitted, HyperBricks derives the binary base name from the Go
source file:

```text
example_plugin.go -> ExamplePlugin
```

## Local HyperBricks Core Development

When answering a local plugin build question, decide whether "local" means only
a module-local plugin source or also a local HyperBricks core checkout.

Use a local checkout override instead of temporary Git tags.

Preferred local-core workflow:

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/GitHub/hyperbricks"
hyperbricks plugin build myplugin@1.0.0 --module demo
```

Equivalent explicit flag:

```bash
hyperbricks plugin build myplugin@1.0.0 \
  --module demo \
  --hyperbricks-path "$HOME/GitHub/hyperbricks"
```

Global plugin install against local core:

```bash
hyperbricks plugin install esbuild@2.0.0 \
  --hyperbricks-path "$HOME/GitHub/hyperbricks"
```

Rules:

- Use `HYPERBRICKS_LOCAL_PATH` or `--hyperbricks-path` for local HyperBricks
  core development.
- Use Git tags only for release builds.
- Do not create temporary public tags just to make local plugin builds pass.
- If `--hyperbricks-path` or `HYPERBRICKS_LOCAL_PATH` is set, treat the build as
  local development mode.
- In release mode, require the requested HyperBricks version to exist as a real
  tag or revision.
- Rebuild plugins after runtime API changes or when compatibility is uncertain.

## Troubleshooting

Custom plugin does not appear:

```text
plugins.enabled contains the custom config name.
The config name ends with __<module>@<version>.
manifest.json exists in the expected source folder.
The selected module and build are correct.
```

Runtime cannot find plugin:

```text
directories.plugins points to ./bin/plugins or the intended directory.
The process working directory is stable.
The config name exactly matches the artifact name without `.so` or `.wasm`.
The plugin was built for the same HyperBricks version that is running.
```

Build fails with `unknown revision`:

```text
The requested HyperBricks version does not exist as a Git tag or revision.
Use HYPERBRICKS_LOCAL_PATH or --hyperbricks-path for local core development.
Use real Git tags only for release builds.
```

## Codex Behavior Rules

When editing a HyperBricks project:

1. Preserve YAML source order and semantic child names.
2. Use lowercase YAML `type` values.
3. Use YAML `imports`, not old `@import`.
4. Use YAML `inherit`, not old macro/property expansion.
5. Use YAML resolver mappings, not old marker strings.
6. Do not write runtime-only `@type` or `@order` in source YAML.
7. Do not move source files into subdirectories without adding imports.
8. Keep `package.hyperbricks.yaml` plugin names exact.
9. Never include `.so` in `plugins.enabled`.
10. For custom plugins, include `__<module>@<version>` in the config name.
11. Do not assume plugin binaries are valid; rebuild if version compatibility is
    uncertain.
12. Do not create temporary public Git tags for local plugin testing.
13. Use `HYPERBRICKS_LOCAL_PATH` or `--hyperbricks-path` for unreleased local
    core testing.
14. Run CLI commands from the project root unless the repository clearly
    documents otherwise.
15. When changing deploy or plugin config, check the working directory
    assumption for `./bin/plugins`.

## Validation Commands

Use these when relevant:

```bash
hyperbricks version
hyperbricks start -m <module>
hyperbricks static -m <module>
hyperbricks build --hra -m <module>
hyperbricks plugin list
hyperbricks plugin build <name>@<version>
hyperbricks plugin build <name>@<version> --module <module>
```

For command details:

```bash
hyperbricks <command> --help
```
