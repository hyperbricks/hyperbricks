## Hyperbricks Configuration Reference

This section explains the configuration options for your `package.hyperbricks.yaml` file.

---

### Module Paths

The runtime supplies the active module directory to package config path resolvers.
Use explicit YAML path objects when a field should point inside the current module.

---

### Global Configuration Objects

You can define global config blocks (e.g., for custom use):

```yaml
myconf:
  some: value
```

Only the `hyperbricks` object is processed by the runtime. Other objects are allowed for organizational or user-defined purposes.

---

### Main Configuration Block

#### `hyperbricks:`

This is the primary configuration block.

---

### Mode Settings

```yaml
hyperbricks:
  mode: development
```

Available modes:

* `development` – Ideal for local dev with live reload.
* `live` – Optimized for production deployment.
* `debug` – Extra verbose logging (used for diagnostics).

---

### Debugging

```yaml
hyperbricks:
  debug:
    level: debugging
```

Controls Go-level debug verbosity.

---

### Development Mode

```yaml
hyperbricks:
  development:
    watch: true
    reload: true
    frontend_errors: false
    dashboard: false
```

These settings are active only in `development` mode.

---

### Live Mode

```yaml
hyperbricks:
  live:
    cache: 10s
```

* Sets cache duration for rendered pages.
* Supports Go-style durations like `300ms`, `2h45m`, etc.

---

### Server Settings

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

If these settings are omitted, HyperBricks uses the same defaults shown above: `read_timeout = 5s`, `write_timeout = 10s`, `idle_timeout = 20s`, and `keep_alives_enabled = true`.

Adjust timeout values based on traffic level. Keep-alives are enabled by default for normal web traffic; only disable them when you explicitly want one-request-per-connection behavior.

---

### System Settings

```yaml
hyperbricks:
  system:
    metrics_watch_interval: 10s
```

Interval for system ticker to gather and report metrics.

---

### Rate Limiting

```yaml
hyperbricks:
  rate_limit:
    requests_per_second: 100
    burst: 500
```

Control traffic with configurable request and burst limits. Adjust for your traffic level.

---

### Plugins

```yaml
hyperbricks:
  plugins:
    enabled:
      - MyPlugin@1.0.0
      - OtherPlugin@1.0.0
```

Enable plugins by listing their exact config names without the `.so` suffix. Use `hyperbricks plugin help` and `hyperbricks plugin list` for details.
See plugin section on how to create and/or install plugins for Hyperbricks.

---

### Directory Settings

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

| Key           | Purpose                                                 |
| ------------- | ------------------------------------------------------- |
| `render`      | Where static output is generated (`hyperbricks static`) |
| `static`      | Public assets served as-is, like minified JS/CSS        |
| `resources`   | Raw, unprocessed files like JS sources or markdown      |
| `plugins`     | Path to `.so` plugin files                              |
| `templates`   | Used with `<TEMPLATE>` and template engines             |
| `hyperbricks` | Directory to scan for `.hyperbricks` files              |
| `logs`        | (Optional) Enable file-based logging                    |

---

## Module Directory Structure Guide

A Hyperbricks module follows a specific folder structure. Each folder serves a unique role in how your module is configured, rendered, and served.

```
someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks.yaml
```

---

### Folder Breakdown

#### `hyperbricks/`

Contains the core `.hyperbricks` configuration files.

* Files are loaded automatically in alphanumeric order from the root of this folder.
* Subdirectories are not auto-loaded — you must explicitly include them using `@import`.

---

#### `rendered/`

Default output folder used by the `hyperbricks static` command.

* Stores pre-rendered routes and other static outputs.
* Contents are typically generated — not edited by hand.

---

#### `resources/`

Raw asset and source data folder.

Use this to store:

* JavaScript source files
* Tailwind or other build configurations
* Uncompiled markdown documents
* Unprocessed images or data files

---

#### `static/`

Public asset directory available at runtime.

* Served relative to the root domain.
* Example: `static/css/styles.min.css` becomes `https://mydomain.com/static/css/styles.min.css`

Use it for precompiled CSS, JS, fonts, or images.

---

#### `templates/`

Stores template files used during rendering.

* Used for `<TEMPLATE>` blocks and any other templated output components.
* Often embedded into config via `hypermedia` markers.

---

#### `package.hyperbricks.yaml`

Module entry point.

* Defines the module's main configuration.
* Links together scripts, templates, resources, and routes.

---

### Path Markers in Configurations

Use these markers in your configuration files for cleaner, portable paths:

| Marker        | Refers To                  |
| ------------- | -------------------------- |
| `MODULE`      | Current module directory   |
| `MODULE_ROOT` | Root folder of all modules |
| `RESOURCES`   | `resources/` directory     |
| `HYPERBRICKS` | `hyperbricks/` directory   |
| `TEMPLATES`   | `templates/` directory     |
| `STATIC`      | `static/` directory        |
| `ROOT`        | Root of the entire project |

---

## Hypermedia Markers: Cached Content Embedding

The `hypermedia` marker lets you preload files into memory and assign them as templates or raw text blocks. These are injected into the configuration and made instantly available — no runtime file access required.

### Why use it?

* Fast rendering
* Self-contained config
* Immutable state after load

### Structure

```
hypermedia.<key> = <TYPE>
hypermedia.<key>.<field> = {{<TYPE>:<file>}}
```

| Type       | Field      | Loaded From                |
| ---------- | ---------- | -------------------------- |
| `TEMPLATE` | `template` | `templates/` in the module |
| `TEXT`     | `value`    | Module's root directory    |

### Examples

```ini
hypermedia.10 = TEMPLATE
hypermedia.10.template = {{TEMPLATE:sometemplate.html}}

hypermedia.10 = TEXT
hypermedia.10.value = {{TEXT:sometext.md}}
```

---

## Quickstart

### 1. Install Hyperbricks

Requirements:

* Go 1.23.2 or higher

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

---

### 2. Initialize a New Project

```bash
hyperbricks init -m someproject
```

This creates a folder `someproject` in the `modules` directory with this structure:

```
someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks.yaml
```

Always run Hyperbricks CLI from the **project root** (parent of `modules/`).

---

### 3. Start the Project

```bash
hyperbricks start -m someproject
```

Visit: [http://localhost:8080](http://localhost:8080)

To see CLI options:

```bash
hyperbricks start --help
```

---

### 4. Render Static Output

```bash
hyperbricks static -m someproject
```

---

### Other Commands

```bash
hyperbricks [command]
```

| Command      | Description                                |
| ------------ | ------------------------------------------ |
| `completion` | Generate shell autocompletion              |
| `help`       | Help on any command                        |
| `init`       | Create config and folders for a new module |
| `select`     | Select the active module                   |
| `plugin`     | plugin management commands                 |
| `start`      | Start the server                           |
| `static`     | Render static HTML output                  |
| `version`    | Show version info                          |

Use `hyperbricks [command] --help` for detailed options.

## Hyperbricks Plugin System

Hyperbricks supports plugins compiled as Go `.so` files. The CLI offers tools to discover, install, build, and manage plugins compatible with your current version of Hyperbricks.

---

### Overview

To access plugin management commands:

```bash
hyperbricks plugin [subcommand]
```

Subcommands:

| Command          | Description                                           |
| ---------------- | ----------------------------------------------------- |
| `plugin list`    | List compatible plugins and their installation status |
| `plugin install` | Download and compile a plugin by name and version     |
| `plugin build`   | Rebuild a plugin from local source                    |
| `plugin remove`  | Remove a compiled plugin from the local system        |
| `plugin update`  | Update a plugin to the latest compatible version      |

---

## `plugin list`

Displays available plugins that are compatible with your installed Hyperbricks version.

```bash
hyperbricks plugin list
```

Output includes:

* Plugin name
* Compatible version
* All available versions
* Hyperbricks version constraints
* Installation status (e.g. installed, incompatible, not found)

Example output:
```
Name         Plugin Version  Available Versions  Compatible Hyperbricks  Installed
----         --------------  ------------------  ----------------------  ---------
esbuild      1.0.0           1.0.0               >=1.1.0-beta            yes
loremipsum   1.0.0           1.0.0               >=1.1.0-beta            yes
markdown     1.0.0           1.0.0               >=1.1.0-beta            yes
myplugin     1.0.0           1.0.0               >=1.1.0-beta            no
tailwindcss  1.0.0           1.0.0               >=1.1.0-beta            yes
```

To enable plugins, they must be compiled for the currently installed version of Hyperbricks.
This can be done automatically using:
 hyperbricks plugin install <name>@<plugin_version> 

* To preload the plugin, add the config name to your package.hyperbricks.yaml
* under the `plugins.enabled` array.
* Plugin binaries are compiled as `<Binary>@<plugin_version>.so`, but runtime config omits `.so`.
```yaml
hyperbricks:
  plugins:
    enabled:
      - EsbuildPlugin@1.0.0
      - LoremIpsumPlugin@1.0.0
      - MarkdownPlugin@1.0.0
      - TailwindcssPlugin@1.0.0
```
---

## `plugin install <name>[@<version>]`

Installs and builds a plugin from the remote Hyperbricks plugin index.

```bash
hyperbricks plugin install markdown@1.0.0
```

If no version is specified, the latest compatible version is used.

### What it does:

1. **Fetches metadata** from the remote plugin index.
2. **Performs a sparse Git checkout** of the plugin's source files.
3. **Patches the plugin’s `go.mod`** to match your current Hyperbricks version.
4. **Runs `go mod tidy` and `go build`** to compile the plugin into `./bin/plugins`.

---

## `plugin build <name>@<version>`

Builds a plugin from local source already downloaded to `./plugins/<name>/<version>`.

```bash
hyperbricks plugin build markdown@1.0.0
```

Useful when:

* You manually edited a plugin
* You cloned or fetched plugin sources yourself

---

## `plugin remove <name>@<version>`

Removes a compiled `.so` plugin binary from `./bin/plugins`.

```bash
hyperbricks plugin remove markdown@1.0.0
```

This does not delete the source folder from `./plugins`.

---

## `plugin update <name>`

(Planned) Checks for and installs the latest compatible version of a given plugin.

```bash
hyperbricks plugin update markdown
```

*Note: This command currently shows placeholder output. Update logic is not yet implemented.*

---

## Using Installed Plugins

To enable a plugin in your module, add its config name without `.so` to the `plugins.enabled` list in `package.hyperbricks.yaml`.

Example:

```yaml
hyperbricks:
  plugins:
    enabled:
      - Markdown@1.0.0
```

> Plugin binary filenames follow the format `<CamelCaseName>@<version>.so`; config names omit the `.so` suffix.

---

## Plugin Compatibility

* A plugin is only valid if compiled for the same version of Hyperbricks as you're running.
* The CLI inspects `.so` binaries to validate their embedded version.
* Incompatible or outdated plugins are flagged in yellow or red when using `plugin list`.
