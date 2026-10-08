<!-- Generated from docs/PACKAGE_CONFIGURATION.md. Do not edit directly. -->

# Package Configuration

`package.hyperbricks.yaml` configures a module's runtime: its mode, developer tools, server, caching, plugins, logging, and directory locations. Place it in the module root, alongside directories such as `hyperbricks/`, `templates/`, and `resources/`.

Package settings use ordinary YAML mappings. Component files use ordered component trees; their syntax is covered in [YAML Usage](YAML_USAGE.md). For a first working application, follow the [Quickstart](QUICKSTART.md).

## Start With a Small Package

For an existing module at `modules/demo/`, a small development configuration is:

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    watch_dirs: [hyperbricks, templates, resources]
  server:
    port: 8080
```

Save it as `modules/demo/package.hyperbricks.yaml` and start the module from the project root:

```sh
hyperbricks start -m demo
```

Omitted settings use runtime defaults. In this example, Dashboard is disabled; Spaces and its writes are enabled, with no login when both developer credentials are absent. Spaces still enforces its host and write-origin checks. See [Developer Tools](#developer-tools) to change those settings.

Restart after changing package settings or credential environment variables. Source watching does not reload the package file. See the [CLI module selection rules](HYPERBRICKS_CLI.md#start-module-argument) for module paths and alternative package files selected with `--config`.

## Run With a Different Configuration

Use `--config` to start the same module with another package file. For example, create `modules/demo/profiles/preview.hyperbricks.yaml` with:

```yaml
hyperbricks:
  mode: debug
  server:
    port: 8081
```

From the project root, run:

```sh
hyperbricks start -m demo --config profiles/preview.hyperbricks.yaml
```

This serves the existing demo module on port `8081` in debug mode, using its normal source, template, and asset directories. To check the same selected configuration before starting it:

```sh
hyperbricks doctor -m demo --config profiles/preview.hyperbricks.yaml
```

The selected file supplies the complete package configuration. Omitted settings use runtime defaults; HyperBricks does not merge it over `package.hyperbricks.yaml`. Import the default package explicitly when you want to retain its custom directory, plugin, or application settings.

The `--config` path is relative to the selected module directory. Absolute paths and paths that escape through `..` are rejected. A file in `profiles/` still uses the selected module as its `module` path base; its source and asset directories do not move into `profiles/`.

For example, this command uses port `8082` in place of the selected file's `8081`:

```sh
hyperbricks start -m demo --config profiles/preview.hyperbricks.yaml --port 8082
```

The current CLI treats `--port 8080` as its default value and leaves a different package port in effect. To select `8080` in that case, set `server.port: 8080` under `hyperbricks` in the selected file.

Omit `--config` to use `package.hyperbricks.yaml` again. Restart the process after editing whichever package file it uses.

## Optional Package Imports

A single package file remains fully supported. Larger packages can import mapping fragments outside the component-source directory:

```yaml
imports:
  - config/runtime.hyperbricks.yaml
  - config/development.hyperbricks.yaml

hyperbricks:
  mode: development
```

Each fragment uses the same `hyperbricks`, `vars`, and `myconf` mappings as the entry package. Imports are literal relative filenames, resolved relative to the containing file. Nested imports are supported; paths must remain inside the selected module, including after resolving symlinks. Globs, remote URLs, and resolver expressions in import filenames are not supported.

Imports apply in declaration order, with each document's own values applied after its imports. Mappings merge recursively; later scalars and whole lists replace earlier ones. `[]` clears an inherited list. Recognized resolver expressions are replaced as a unit. Explicit null replaces an inherited value and undergoes the normal field validation; it does not delete a key. Duplicate keys within any file and circular imports are errors. A shared import reached through two branches is applied at each declared position.

Variables resolve after the complete merge, so an entry-file variable override can supply an imported setting. Import locations do not change existing path bases such as `{base: module}`. The selected `--config` file remains the entry point; a profile may explicitly import `../package.hyperbricks.yaml`.

Restart after editing either the entry or an imported file. Editor validation uses pending imported-file contents and reports source ownership. Runtime archives preserve configuration fragments at their module-relative paths, and imported changes participate in build hashes. A build rejects an import filtered out by archive exclusions instead of creating a package with missing inputs; keep configuration fragments in an included directory such as `config/`.

## Top-Level Values and Resolvers

| Key | Purpose |
| --- | --- |
| `imports` | Optional ordered list of package configuration fragments. |
| `hyperbricks` | Runtime settings described in this document. |
| `myconf` | Application-owned configuration that components can read through the `config` resolver. |
| `vars` | Inputs for value resolvers. They are not copied into the materialized configuration. |

Configuration values support the [same value resolvers](YAML_USAGE.md#value-resolvers) as component source:

```yaml
vars:
  site_title: My demo

myconf:
  demo:
    title: {var: site_title}

hyperbricks:
  mode: development
  directories:
    templates:
      path: {base: module, path: templates}
```

Package configuration seeds the `module` runtime variable for `var`; other variable names must come from its `vars` mapping. Path resolver bases are a separate mechanism: names such as `resources`, `templates`, and `static` remain available as `base` values. Component source receives the full runtime-variable set listed under [Vars](YAML_USAGE.md#vars) after package directories have been configured.

## Runtime Modes

Set `hyperbricks.mode` to `development`, `debug`, or `live`. The default is `live`.

| Mode | Relevant behavior |
| --- | --- |
| `development` | Developer interfaces can be enabled. Source watching runs when configured. Route output caching is bypassed. |
| `debug` | Dashboard, Errors, Spaces, and contextual editing can be available. Source watching does not run; reload or restart after source changes. Route output caching is bypassed. |
| `live` | Developer interfaces are unavailable. Route output caching follows the configured policies. |

Production runtimes and static output exclude developer interfaces. Invalid mode values are reported and fall back to live mode during ordinary startup; strict configuration validation rejects them. See [Doctor](HYPERBRICKS_CLI.md#doctor) for configuration checks.

## Developer Tools

The fields below are relative to the top-level `hyperbricks` mapping. For example, `development.dashboard.enabled` belongs under `hyperbricks.development.dashboard`.

| Field | Purpose |
| --- | --- |
| `development.watch` | Enable source-directory watching in development mode. Defaults to `false`. |
| `development.watch_dirs` | Source directories to watch. The small package example lists the module's source, templates, and resources. |
| `development.reload` | Retained reload setting; this flag alone does not enable browser refresh. Source watching is controlled by `watch`. |
| `development.dashboard.enabled` | Enable Dashboard Overview and Errors in development/debug mode. Defaults to `false`. The old Boolean `development.dashboard` form is invalid. |
| `development.dashboard.credentials` | Shared `user` and `password` for developer interfaces. Values may use environment resolvers; there is no default account. |
| `development.frontend_editing.enabled` | Master switch for Spaces and configured frontend editors. Defaults to `true`. |
| `development.frontend_editing.spaces.enabled` | Show Spaces and contextual editing in development/debug mode. Defaults to `true`; the master switch must also be enabled. |
| `development.frontend_editing.spaces.write` | Permit Spaces file changes and uploads. Defaults to `true`; set `false` for read-only access. |
| `development.frontend_editing.spaces.allowed_hosts` | Additional server hostnames or IP addresses allowed to serve Spaces, without schemes or ports. Localhost and loopback are always allowed. This checks the server address, not connecting client IPs. |
| `development.frontend_editing.spaces.public_origin` | Optional public HTTP(S) website origin for absolute sharing-image URLs. Sharing-image upload/selection requires it together with a `sharing_image` policy. It does not grant editor access. |
| `development.frontend_errors` | Permit frontend error panels when the component enables `debugpanel`. Panels remain development-only and require a request authenticated with the shared developer credentials. |

Credentials control login independently of interface enablement and write permission:

| Resolved credentials | Enabled Dashboard, Errors, Spaces, and contextual editing |
| --- | --- |
| Both absent | Open without login, with a startup warning. Spaces host and write-origin checks still apply. |
| Both configured | Require Basic Auth; a missing or incorrect browser login returns `401`. |
| Only one configured | Block access with `503`. Configure both values or remove both, then restart. |

Spaces uses this policy even when Dashboard is disabled. Render diagnostics require credentials when Dashboard is disabled. Frontend-editor plugins and frontend error panels still require credentials and remain development-only.

Use [Spaces configuration](SPACES.md#development-configuration) for full examples, [LAN access](SPACES.md#lan-access-and-allowed-hosts) for host and proxy requirements, and [Persistence And Watching](SPACES.md#persistence-and-watching) for save/reload behavior. Development hooks and managed services have their own [configuration guide](DEVELOPMENT_HOOKS.md#configure-the-module).

## Server and Request Limits

All field paths in this and the following tables are relative to `hyperbricks`.

| Field | Purpose |
| --- | --- |
| `server.port` | HTTP server port. Defaults to `8080`. See [alternate configuration examples](#run-with-a-different-configuration) for CLI overrides and the current `--port 8080` limitation. |
| `server.gomaxprocs` | Process-wide Go execution parallelism: `auto` (default) or an integer from `1` through the machine's logical CPU count. Invalid values fail startup. See [CPU parallelism](LIVE_MODE_HTTP.md#cpu-parallelism). |
| `server.beautify` | Beautify rendered HTML when supported. |
| `server.self_closing_tags` | Render XHTML-style self-closing tags when enabled. |
| `server.read_timeout`, `server.write_timeout`, `server.idle_timeout` | HTTP server timeout durations. |
| `server.keep_alives_enabled` | Enable or disable HTTP keep-alive connections. |
| `server.routing` | Clean URL and extension routing settings. See [Routing](ROUTING.md). |
| `server.runtime_gateway` | Runtime host gateway settings. See [Runtime Gateway](RUNTIME_GATEWAY.md). |
| `rate_limit.enabled` | Enable the request rate limiter. Defaults to `true`; disable only when another layer owns rate limiting or for controlled measurements. |
| `rate_limit.requests_per_second`, `rate_limit.burst` | Token-bucket request rate and burst settings used when the limiter is enabled. |

For connection defaults and tuning examples, see [Live Mode HTTP Settings](LIVE_MODE_HTTP.md#connection-settings).

## Caching, Plugins, and Logging

| Field | Purpose |
| --- | --- |
| `live.cache` | Default live-mode route cache duration. Defaults to `10m`; accepts Go durations such as `10s`, `5m`, or `2h`. |
| `plugins.enabled` | Plugin config names to preload, without `.so` or `.wasm`. See [Plugins](PLUGINS.md). |
| `plugins.config` | Optional plugin-specific configuration map. |
| `logger.level` | Runtime logging threshold. |
| `logger.path` | Optional file output for logs. |

Routes can select memory or disk storage and override expiry. See [Caching: a practical guide](CACHING.md) to get started and [Output Cache](LIVE_MODE_HTTP.md#output-cache) for package defaults, disk storage settings, and route policy precedence. Static export settings are documented in the [CLI static package configuration](HYPERBRICKS_CLI.md#package-configuration).

## Directories

The default application layout is:

```text
modules/<name>/
  hyperbricks/
  rendered/
  resources/
  static/
  templates/
  package.hyperbricks.yaml
```

| Directory key | Default module directory | Purpose |
| --- | --- | --- |
| `hyperbricks` | `hyperbricks/` | Runtime source files. Top-level `*.hyperbricks.yaml` files load automatically. |
| `templates` | `templates/` | Go `html/template` files used by `template.file` and other template providers. |
| `resources` | `resources/` | Source assets or data read through file and path resolvers. |
| `static` | `static/` | Public files served directly by the runtime. |
| `render` | `rendered/` | Static output written by `hyperbricks static`. |

Configure these locations under `hyperbricks.directories`. Use a path resolver with `base: module` for module-owned directories. Bare relative paths resolve from the invocation directory; the CLI does not change the working directory when selecting a module.

```yaml
hyperbricks:
  directories:
    hyperbricks:
      path: {base: module, path: hyperbricks}
    templates:
      path: {base: module, path: templates}
    resources:
      path: {base: module, path: resources}
    static:
      path: {base: module, path: static}
    render:
      path: {base: module, path: rendered}
```

`/static/somefile.ext` serves `somefile.ext` from the configured `hyperbricks.directories.static` directory, regardless of its name or `base`. A custom path does not require an additional directory named `static`.

HyperBricks does not load subdirectories below `hyperbricks/` automatically. Add a root source file and use [imports](YAML_USAGE.md#imports) to load shared files.

## Expanded Package Example

This example combines optional developer login, live cache duration, explicit HTTP settings, and request limits. The values are a configuration example; omitted settings use their runtime defaults.

```yaml
hyperbricks:
  mode: development
  development:
    watch: true
    watch_dirs: [hyperbricks, templates, resources]
    reload: false
    frontend_errors: false
    dashboard:
      enabled: false
      credentials:
        user:
          env: HB_DEVELOPER_USER
        password:
          env: HB_DEVELOPER_PASSWORD
  live:
    cache: 10m
  server:
    port: 8080
    gomaxprocs: auto
    read_timeout: 5s
    write_timeout: 10s
    idle_timeout: 20s
    keep_alives_enabled: true
  rate_limit:
    enabled: true
    requests_per_second: 100
    burst: 500
```

To require developer login, set both referenced environment variables before starting the module. With both absent, Spaces opens without login and startup warns; Dashboard remains disabled in this example. Set `development.dashboard.enabled: true` under `hyperbricks` to expose its Overview and Errors views too.

Run `hyperbricks doctor -m demo` to check the module configuration and read any warnings. See [Migration](MIGRATION.md) when updating a package written for an older HyperBricks version.

## Editing settings interactively

Run `hyperbricks settings -m demo` to inspect effective settings, built-in defaults, descriptions, and the source file defining each value. Use `--config package.preview.hyperbricks.yaml` to select another entry. Navigate with Enter to open a section or edit a setting, and Esc to return to the parent. A breadcrumb shows your location, and each section remembers its selected item. Search with `/` across all sections; review/save remains available at every level. The editor uses the same package composition and validation as runtime loading.

A normal edit changes the winning definition in its owning file. Choose **override in entry** to keep an imported definition unchanged and write an explicit override in the selected package. Lists replace whole lists; overriding a list item copies the effective list into the entry first. Removing a definition exposes an inherited value or built-in default. Values supplied by resolvers retain their expressions unless explicitly edited.

Changes remain pending until reviewed and saved. Saving checks every loaded source's bytes and canonical path before writing, then revalidates the complete package. If an import changes, moves, disappears, or resolves through a different symlink while the editor is open, saving stops without recreating the missing import. Reload can retain pending edits only where source identity and ownership still match. Otherwise review the pending edits, then explicitly discard/reload and reapply them to the new source.

Writes are atomic per file and preserve permissions. A multi-file save is not a filesystem transaction: if a later write fails, the editor reports which files were saved and keeps the remaining edits pending. Settings changes take effect on a full application restart. Opening, validating, and saving settings never runs lifecycle tasks or asset builds.

A single `package.hyperbricks.yaml` remains the default. Imports are optional; the settings command does not split files or introduce a second configuration system.
