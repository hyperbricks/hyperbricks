---
name: hyperBricks-basic-cli-skills
description: Work on HyperBricks routes, fragments, templates, plugins, and deploys
default_prompt: "Use Linear context to triage or update relevant issues for this task, with clear next actions."
metadata:
  short-description: Manage HyperBricks isseus with CLI
---

Save as:

```text
.codex/skills/hyperbricks/SKILL.md
```

````markdown
# HyperBricks Skill for Codex

Use this skill when working on HyperBricks projects, `.hyperbricks` configuration files, HyperBricks CLI workflows, package configuration, routes, fragments, templates, plugins, static builds, deploy archives, or plugin development.

HyperBricks is a Go-based system for building and serving hypermedia web applications from modular `.hyperbricks` configuration files. It supports full pages, HTMX fragments, static rendering, dynamic rendering, Go templates, plugin-based components, Tailwind CLI integration, and optional JS/TS bundling through plugins.

## Core mental model

A HyperBricks project is configured through small `.hyperbricks` files. Each file defines components and their nesting.

There are two component categories:

1. Standard components: leaf nodes that render their own content.
2. Composite components: structural nodes that contain other components.

Common standard components:

```text
<HTML>
<TEXT>
<IMAGE>
<CSS>
<JS>
<JSON>
<MENU>
<PLUGIN>
````

Common composite components:

```text
<HYPERMEDIA>
<FRAGMENT>
<TREE>
<TEMPLATE>
<API_RENDER>
<API_FRAGMENT_RENDER>
```

Use standard components for direct output. Use composite components for routing, nesting, reuse, data rendering, and HTMX partials.

## Root components and routes

Root components initiate frontend output and usually own routes.

Root types:

```text
<HYPERMEDIA>
<FRAGMENT>
<API_FRAGMENT_RENDER>
```

Use `<HYPERMEDIA>` for full pages.

Example:

```hyperbricks
hypermedia = <HYPERMEDIA>
hypermedia.route = index
hypermedia.title = Welcome!

hypermedia.10 = <HTML>
hypermedia.10.value = <p>Hello from HyperBricks.</p>
```

This creates:

```text
/index
```

Use `<FRAGMENT>` for HTMX partial responses.

Example:

```hyperbricks
myfragment = <FRAGMENT>
myfragment.10 = <HTML>
myfragment.10.value = <p>Fragment content 1</p>
myfragment.20 = <HTML>
myfragment.20.value = <p>Fragment content 2</p>
```

Root composites may declare a `guard { ... }` block to deny a request before rendering starts.

## Composition components

Use these for larger projects:

```text
<API_RENDER>      Fetch and render public API data.
<TREE>            Nest components hierarchically.
<TEMPLATE>        Use reusable Go template logic with Sprig extensions.
```

Prefer composition over duplication. Extract repeated structures into `<TREE>` or `<TEMPLATE>`. Use `@macro` only when repetition becomes significant.

## Imports

HyperBricks loads `.hyperbricks` files from the module’s `hyperbricks/` directory.

Files in subdirectories are not auto-loaded. Import them explicitly:

```hyperbricks
@import "plugins/esbuild.hyperbricks"
@import "page/menu.hyperbricks"
```

Best practice:

```text
hyperbricks/
├── pages/
├── fragments/
├── plugins/
├── menus/
└── theme/
```

Use imports for plugins, themes, menus, reusable fragments, and page sections.

## Macros

Use `@macro` for repeated route definitions, menus, mappings, or generated config blocks.

Example:

```hyperbricks
@macro as (index, title, route, doc) {
1|Introduction|introduction_fragment|introduction
2|Quickstart|quickstart_fragment|quickstart
} = <<<[
    {{{.route}}} < docs_fragment
    {{{.route}}} {
        index = {{{.index}}}
        route = {{{.route}}}
        title = {{{.title}}}

        10.data.source = {{RESOURCES}}/docs/{{{.doc}}}.md
    }
]>>>
```

Rules:

* Use macros for repeated config only.
* Keep macro tables compact.
* Prefer explicit config when there are only one or two cases.

## Project structure

A standard module looks like:

```text
someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks.yaml
```

Directory purposes:

```text
hyperbricks/   Core `.hyperbricks` config files.
rendered/      Static output from `hyperbricks static`.
resources/     Raw assets, JS sources, Tailwind config, markdown, data.
static/        Public files served directly.
templates/     Go templates used by `<TEMPLATE>`.
package.hyperbricks.yaml Module entrypoint and runtime config.
```

Run the HyperBricks CLI from the project root, usually the parent of `modules/`.

## Path markers

Use these markers for portable paths:

```text
MODULE       Current module directory
MODULE_ROOT  Root folder of all modules
RESOURCES    resources/ directory
HYPERBRICKS  hyperbricks/ directory
TEMPLATES    templates/ directory
STATIC       static/ directory
ROOT         Root of the whole project
```

Example:

```hyperbricks
10.data.source = {{RESOURCES}}/docs/introduction.md
```

## Hypermedia cached file markers

Use the `hypermedia` marker to preload files into memory.

Templates:

```hyperbricks
hypermedia.10 = TEMPLATE
hypermedia.10.template = {{TEMPLATE:sometemplate.html}}
```

Text:

```hyperbricks
hypermedia.10 = TEXT
hypermedia.10.value = {{TEXT:sometext.md}}
```

Use this for fast rendering and self-contained loaded state.

## CLI commands

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

## Install HyperBricks

Requirement:

```text
Go 1.23.2 or higher
```

Install:

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

## Initialize a project

```bash
hyperbricks init -m someproject
```

Creates:

```text
modules/someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks.yaml
```

## Initialize from starter

List starters:

```bash
hyperbricks init-starter list
```

Install latest compatible starter:

```bash
hyperbricks init-starter get hello-world -m someproject
```

Install specific version:

```bash
hyperbricks init-starter get hello-world@1.0.0 -m someproject
```

The target module directory must be missing or empty.

## Start server

```bash
hyperbricks start -m someproject
```

Default local URL:

```text
http://localhost:8080
```

## Static render

```bash
hyperbricks static -m someproject
```

Output goes to the module’s `rendered/` directory unless configured otherwise.

## Build deploy archive

Build `.hra`:

```bash
hyperbricks build --hra -m someproject
```

Build `.zip`:

```bash
hyperbricks build --zip -m someproject
```

Common flags:

```text
--out <dir>              Output directory, default `deploy/`.
--force                  Rebuild even when source hash is unchanged.
--replace[=<build_id>]   Replace current or specified build.
--push                   Build and push to default deploy target.
--target <name>          Select deploy target when using `--push`.
```

## Deploy runtime

Run module from deploy folder:

```bash
hyperbricks start --deploy -m someproject
```

Deploy services:

```bash
hyperbricks start --deploy-remote
hyperbricks start --deploy-local
hyperbricks start --deploy-init-config local
hyperbricks start --deploy-init-config remote
```

Deploy config lives at:

```text
deploy.hyperbricks
```

## Docker deploy

Optional Docker setup:

```bash
docker compose -f docker/docker-compose.yml up --build
```

Defaults:

```text
Deploy API: http://localhost:9090
SSH:        localhost:2222
```

## package.hyperbricks.yaml

The runtime supplies the active module directory to package config path resolvers.

The main runtime config is inside:

```yaml
hyperbricks: {}
```

Only the `hyperbricks` object is processed by the runtime. Other objects may be used for organization.

## Runtime mode

Available modes:

```yaml
hyperbricks:
  mode: development
```

Modes:

```text
development   Local development with watch/reload.
live          Production-oriented mode.
debug         Verbose diagnostics.
```

## Development config

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    reload: true
    frontend_errors: false
    dashboard: false
```

## Live config

```yaml
hyperbricks:
  mode: live
  live:
    cache: 10s
```

Go-style durations are valid:

```text
300ms
10s
2h45m
```

## Server config

Defaults:

```yaml
hyperbricks:
  server:
    port: 8080
    beautify: true
    read_timeout: 5s
    write_timeout: 10s
    idle_timeout: 20s
    keep_alives_enabled: true
```

Keep-alives should usually stay enabled.

## Rate limiting

```yaml
hyperbricks:
  rate_limit:
    requests_per_second: 100
    burst: 500
```

## Directory config

```yaml
hyperbricks:
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
    plugins: ./bin/plugins/
    templates:
      path:
        base: module
        path: templates
    hyperbricks:
      path:
        base: module
        path: hyperbricks
```

## Runtime gateway

Runtime gateway is disabled by default.

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
  --runtime-host-suffix -runtime.hyperbricks.eu,-live.hyperbricks.eu \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

Config example:

```yaml
hyperbricks:
  server:
    runtime_gateway:
      enabled: true
      domain: runtime.local
      host_suffix: -runtime.hyperbricks.eu
      resolver: http://127.0.0.1:8080/resolve-runtime
```

Rules:

* Resolver must be an internal trusted endpoint.
* Startup fails if gateway is enabled without a domain or host suffix and resolver.
* CLI flags override config.
* Runtime hosts must resolve to the HyperBricks server.

Example local hosts entry:

```text
127.0.0.1 test-001--current.runtime.local
```

## Plugin model

HyperBricks plugins are Go `.so` files.

Two plugin types exist:

```text
Global plugins   Shared public plugins from plugin index.
Custom plugins   Module-level plugins shipped with a module.
```

Compiled binaries live in:

```text
./bin/plugins
```

Enable plugins in `package.hyperbricks.yaml` using the compiled binary name without `.so`.

Example:

```yaml
hyperbricks:
  plugins:
    enabled:
      - MarkdownPlugin@1.0.0
```

## Plugin CLI

Plugin commands:

```bash
hyperbricks plugin list
hyperbricks plugin install <name>@<version>
hyperbricks plugin build <name>@<version>
hyperbricks plugin remove <name>@<version>
```

If version is omitted for `install`, the latest compatible version is selected.

Example:

```bash
hyperbricks plugin install markdown@1.0.0
```

## Global plugins

Source:

```text
./plugins/<name>/<version>/manifest.json
```

Output:

```text
./bin/plugins/<Binary>@<version>.so
```

Config usage:

```yaml
hyperbricks:
  plugins:
    enabled:
      - EsbuildPlugin@2.0.0
```

## Custom plugins

Source in local mode:

```text
modules/<module>/plugins/<name>/<version>/manifest.json
```

Source in deploy-remote runtime:

```text
<deploy.root>/<module>/runtime/<build_id>/plugins/<name>/<version>/manifest.json
```

Output:

```text
./bin/plugins/<Binary>__<module>@<version>.so
```

Config usage:

```yaml
hyperbricks:
  plugins:
    enabled:
      - MyPlugin__test-003@1.0.0
```

Build custom plugin:

```bash
hyperbricks plugin build myplugin@1.0.0 --module test-003
```

Remove custom plugin:

```bash
hyperbricks plugin remove myplugin@1.0.0 --module test-003
```

## Plugin naming rules

The config name is the plugin binary name without `.so`.

Use the exact config name in:

```text
plugins.enabled
plugin = "..."
```

Do not include `.so`.

Global plugin pattern:

```text
<Binary>@<version>
```

Custom plugin pattern:

```text
<Binary>__<module>@<version>
```

Examples:

```text
EsbuildPlugin@1.0.10
MyPlugin__test-003@1.0.0
```

## Plugin manifest

Example:

```json
{
  "plugin": "github.com/hyperbricks/plugins/myplugin",
  "source": "my_plugin.go",
  "version": "1.0.0",
  "binary": "MyPlugin",
  "compatible_hyperbricks": [ ">=0.5.0-alpha" ],
  "description": "Basic Plugin example"
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

If `binary` is present, it overrides the derived binary name.

If `binary` is absent, derive the base name from the Go source file:

```text
upload_plugin.go -> UploadPlugin
```

## Plugin compatibility

A plugin must be compiled for the same HyperBricks version as the running binary.

HyperBricks checks compiled plugins using embedded Go module metadata, comparable to:

```bash
go version -m <plugin.so>
```

If incompatible, rebuild the plugin against the active HyperBricks version.

## Building plugins against a HyperBricks version

Explicit release version:

```bash
hyperbricks plugin install esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

```bash
hyperbricks plugin build esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

In release mode, the version must exist as a real Git revision or tag for:

```text
github.com/hyperbricks/hyperbricks
```

Otherwise `go mod tidy` may fail with:

```text
unknown revision v1.0.1-beta
```

Replace that section with:

```markdown
## Local HyperBricks core development

When answering any **local plugin build** question, first decide whether “local” means only a module-local plugin source, or also a local HyperBricks core checkout.

If the user says “local plugin”, “local build”, “develop plugin locally”, or similar, include the local core workflow unless they clearly mean a released/installed HyperBricks version.

Use a local checkout override instead of temporary Git tags.

Preferred local-core workflow:

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
hyperbricks plugin build myplugin@1.0.0 --module test-003
```

Equivalent explicit flag:

```bash
hyperbricks plugin build myplugin@1.0.0 \
  --module test-003 \
  --hyperbricks-path "$HOME/Github/hyperbricks"
```

Global plugin install against local core:

```bash
hyperbricks plugin install esbuild@2.0.0 \
  --hyperbricks-path "$HOME/Github/hyperbricks"
```

Effective module override:

```go
require github.com/hyperbricks/hyperbricks v0.0.0

replace github.com/hyperbricks/hyperbricks => /absolute/path/to/hyperbricks
```

Rules:

- Use `HYPERBRICKS_LOCAL_PATH` or `--hyperbricks-path` for local HyperBricks core development.
- Use Git tags only for release builds.
- Do not create temporary public tags just to make local plugin builds pass.
- Do not answer local plugin build questions with only `hyperbricks plugin build <name>@<version> --module <module>` unless the user clearly wants the installed/released HyperBricks binary.
```

## Plugin build mode rules

Development mode:

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
hyperbricks plugin install esbuild@2.0.0
```

Release mode:

```bash
unset HYPERBRICKS_LOCAL_PATH
hyperbricks plugin install esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

Rules:

* If `--hyperbricks-path` is set, use local development mode.
* If `HYPERBRICKS_LOCAL_PATH` is set, use local development mode.
* If both `--hyperbricks-path` and `--hyperbricks-version` are set, fail unless explicitly allowed.
* In release mode, remove existing `replace github.com/hyperbricks/hyperbricks`.
* In release mode, require the requested version to exist as a real tag or revision.
* Normalize versions so both `1.0.1-beta` and `v1.0.1-beta` work.
* Never create temporary public test tags only to make plugin builds pass.

## Dashboard plugin manager

Available in:

```text
--deploy-remote
--deploy-local
```

Global Plugins tab shows:

```text
Available plugins
Compatible versions
Installed binaries
Compatibility status
Install/Rebuild/Remove actions
```

Custom Plugins tab shows module-level plugins from `plugins.enabled`.

Custom plugins appear only when:

```text
The plugin is listed in plugins.enabled.
The config name ends with __<module>@<version>.
manifest.json exists in the module plugin source folder.
The selected module/build is correct.
```

## Plugin API endpoints

Deploy-remote:

```text
GET  /deploy/plugins/global/index
GET  /deploy/plugins/global
POST /deploy/plugins/global/install
POST /deploy/plugins/global/rebuild
POST /deploy/plugins/global/remove
GET  /deploy/plugins/custom?module=<m>&build_id=<id>
POST /deploy/plugins/custom/compile
POST /deploy/plugins/custom/remove
GET  /deploy/plugins/tasks/<task_id>
GET  /deploy/plugins/tasks/<task_id>/logs
```

Deploy-local:

```text
GET  /local/plugins/global/index
GET  /local/plugins/global
POST /local/plugins/global/install
POST /local/plugins/global/rebuild
POST /local/plugins/global/remove
GET  /local/plugins/custom?module=<m>
POST /local/plugins/custom/compile
POST /local/plugins/custom/remove
GET  /local/plugins/tasks/<task_id>
GET  /local/plugins/tasks/<task_id>/logs
```

Deploy-local endpoints do not use HMAC.

## Working directory rule

Plugin lookup uses:

```text
directories.plugins
```

Default:

```text
./bin/plugins
```

This path is relative to the current working directory.

Deploy services must run with a stable working directory. Otherwise plugins may compile successfully but fail to load at runtime.

## Troubleshooting

### Custom plugin does not appear

Check:

```text
plugins.enabled contains the custom config name.
The config name ends with __<module>@<version>.
manifest.json exists in the expected source folder.
The selected module and build are correct.
```

### Build fails with unknown revision

Cause:

```text
The requested HyperBricks version does not exist as a Git tag or revision.
```

Fix for local development:

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
hyperbricks plugin install <name>@<version>
```

Fix for release:

```bash
git tag v1.0.1-beta
git push origin v1.0.1-beta
unset HYPERBRICKS_LOCAL_PATH
hyperbricks plugin install <name>@<version> --hyperbricks-version v1.0.1-beta
```

### Runtime cannot find plugin

Check:

```text
directories.plugins points to ./bin/plugins or intended directory.
Process working directory is stable.
Config name exactly matches binary name without .so.
Plugin was built for the same HyperBricks version that is running.
```

### Plugin marked incompatible

Rebuild against the active HyperBricks version:

```bash
hyperbricks plugin build <name>@<version>
```

Or, for local core development:

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
hyperbricks plugin build <name>@<version>
```

## Recommended workflows

Local core plus plugin development:

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
hyperbricks plugin install esbuild@2.0.0
hyperbricks plugin build myplugin@1.0.0 --module test-003
```

Release:

```bash
unset HYPERBRICKS_LOCAL_PATH
git tag v1.0.1-beta
git push origin v1.0.1-beta
hyperbricks plugin install esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

## Codex behavior rules

When editing a HyperBricks project:

1. Preserve existing `.hyperbricks` naming and numeric ordering.
2. Prefer small modular files and `@import`.
3. Do not move files into subdirectories without adding imports.
4. Use `<HYPERMEDIA>` for full-page routes.
5. Use `<FRAGMENT>` for HTMX partials.
6. Use `<TREE>` or `<TEMPLATE>` for repeated structure.
7. Use `@macro` only when repetition is material.
8. Keep `package.hyperbricks.yaml` plugin names exact.
9. Never include `.so` in `plugins.enabled`.
10. For custom plugins, include `__<module>@<version>` in the config name.
11. Do not assume plugin binaries are valid; rebuild if version compatibility is uncertain.
12. Do not create temporary public Git tags for local plugin testing.
13. Use `HYPERBRICKS_LOCAL_PATH` or `--hyperbricks-path` for unreleased local core testing.
14. Run CLI commands from the project root unless the repository clearly documents otherwise.
15. When changing deploy or plugin config, check the working directory assumption for `./bin/plugins`.

## Validation commands

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
