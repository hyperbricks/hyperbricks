## Hyperbricks CLI

```ini
$>hyperbricks ↵

Usage:
  hyperbricks [command]

Available Commands:
  build       Build a Hypermedia Runtime Archive
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Create package.hyperbricks and required directories
  plugin      Hyperbricks plugin manager
  select      Select a hyperbricks module
  start       Start server
  static      Render static content
  version     Show version

Flags:
  -h, --help              help for hyperbricks
      --non-interactive   Disable keyboard input

Use "hyperbricks [command] --help" for more information about a command.
22-01-2026 14:50        INFO
 _   _                       ____       _      _        
| | | |_   _ _ __   ___ _ __| __ ) _ __(_) ___| | _____ 
| |_| | | | | '_ \ / _ \ '__|  _ \| '__| |/ __| |/ / __|
|  _  | |_| | |_) |  __/ |  | |_) | |  | | (__|   <\__ \
|_| |_|\__, | .__/ \___|_|  |____/|_|  |_|\___|_|\_\___/
       |___/|_|                                        

v0.7.6-alpha
```

### Commands

```bash
hyperbricks [command]
```

| Command      | Description                                |
| ------------ | ------------------------------------------ |
| `build`      | Build a runtime archive (`.hra`/`.zip`)     |
| `help`       | Help on any command                        |
| `init`       | Create config and folders for a new module |
| `select`     | Select the active module                   |
| `plugin`     | Plugin management commands                 |
| `start`      | Start the server                           |
| `static`     | Render static HTML output                  |
| `version`    | Show version info                          |

Tip: run `hyperbricks <command> --help` to discover flags and subcommands.

### 1. Install

Requirements:

* Go 1.23.2 or higher

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```


### 2. Initialize a New Project

```bash
hyperbricks init -m someproject
```

This creates a folder `someproject` in the `modules` directory with this structure:

```ini
someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks
```

Always run Hyperbricks CLI from the **project root** (parent of `modules/`).


### 3. Start the Project

```bash
hyperbricks start -m someproject
```

Visit: [http://localhost:8080](http://localhost:8080)

To see CLI options:

```bash
hyperbricks start --help
```


### 4. Render Static Output

```bash
hyperbricks static -m someproject
```

### 5. Build Deploy Archives (`.hra` / `.zip`)

Create a deployable archive for a module:

```bash
hyperbricks build --hra -m someproject
```

Common flags:

* `--zip` — build a `.zip` instead of `.hra`
* `--out <dir>` — output directory (default: `deploy/`)
* `--force` — rebuild even when the source hash is unchanged
* `--replace[=<build_id>]` — replace the current build (or a specific build)
* `--push` — build then push to the default deploy target
* `--target <name>` — pick a deploy target when using `--push`

### 6. Deploy Runtime / Dashboards

Run a module from the deploy folder:

```bash
hyperbricks start --deploy -m someproject
```

Deploy services:

```bash
hyperbricks start --deploy-remote   # Deploy API (remote runtime hub)
hyperbricks start --deploy-local    # Local build dashboard
hyperbricks start --deploy-init-config local|remote # Creates deploy config file
```

Deploy configuration lives in `deploy.hyperbricks` at the project root.

### 7. Docker Deploy (optional)

Spin up the deploy API + SSH using the included Docker setup:

```bash
docker compose -f docker/docker-compose.yml up --build
```

Defaults:
* Deploy API: `http://localhost:9090`
* SSH: `localhost:2222`

See `docs/DOCKER.md` for required keys and configuration.


## Hyperbricks Plugin

Hyperbricks supports plugins compiled as Go `.so` files. The CLI offers tools to discover, install, build, and manage plugins compatible with your current version of Hyperbricks.


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
| `plugin build`   | Build/rebuild a plugin from local source              |
| `plugin remove`  | Remove a compiled plugin from the local system        |

Notes:
* Use `--module <module>` with `plugin build` / `plugin remove` for **custom** (module) plugins.


### plugin list

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
Name         Plugin Version  Available Versions  Compatible Hyperbricks  Installed-         --------------  ------------------  ----------------------  ---------
esbuild      1.0.0           1.0.0               >=0.5.0-alpha           yes
loremipsum   1.0.0           1.0.0               >=0.5.0-alpha           yes
markdown     1.0.0           1.0.0               >=0.5.0-alpha           yes
myplugin     1.0.0           1.0.0               >=0.5.0-alpha           no
tailwindcss  1.0.0           1.0.0               >=0.5.0-alpha           yes
```

To enable plugins, they must be compiled for the currently installed version of Hyperbricks.
This can be done automatically using:
```
hyperbricks plugin install <name>@<plugin_version>
```

* To preload the plugin, add the binary name to your package.hyperbricks (exclude .so)
* under the `plugins.enabled` array.
* Binary names are CamelCase and include the version: `<Binary>@<version>.so`.

```
plugins {
  enabled = [ EsbuildPlugin@1.0.0, LoremIpsumPlugin@1.0.0, MarkdownPlugin@1.0.0, TailwindcssPlugin@1.0.0 ]
}
```

### plugin install

Installs and builds a plugin from the remote Hyperbricks plugin index.

```bash
hyperbricks plugin install markdown@1.0.0
```

If no version is specified, the latest compatible version is used.

1. **Fetches metadata** from the remote plugin index.
2. **Performs a sparse Git checkout** of the plugin's source files.
3. **Patches the plugin’s `go.mod`** to match your current Hyperbricks version.
4. **Runs `go mod tidy` and `go build`** to compile the plugin into `./bin/plugins`.


### plugin build

Builds a plugin from local source already downloaded to `./plugins/<name>/<version>`.

```bash
hyperbricks plugin build markdown@1.0.0
```

Useful when:

* You manually edited a plugin
* You cloned or fetched plugin sources yourself


### plugin remove

Removes a compiled `.so` plugin binary from `./bin/plugins`.

```bash
hyperbricks plugin remove markdown@1.0.0
```

This does not delete the source folder from `./plugins`.

### Using Installed Plugins

To enable a plugin in your module, add its **config name** (binary name **without** `.so`) to `plugins.enabled` in `package.hyperbricks`.

Example:

```ini
plugins {
    enabled = [ MarkdownPlugin@1.0.0 ]
}
```

Custom plugins include the module suffix:

```ini
plugins {
    enabled = [ MyPlugin__test-003@1.0.0 ]
}
```

> Plugin binary filenames on disk follow `<CamelCaseName>@<version>.so` (config name is the same without `.so`).


### Plugin Compatibility

* A plugin is only valid if compiled for the same version of Hyperbricks as you're running.
* The CLI inspects `.so` binaries to validate their embedded version.
* Incompatible or outdated plugins are flagged in yellow or red when using `plugin list`.


## package.hyperbricks

### package.hyperbricks Configuration Reference

This section explains the configuration options for your `package.hyperbricks` file.

### Module Declaration

```hyperbricks
# Set the current module path
$module = modules/default
```

Use the `$module` variable to reference the module directory throughout your configuration.


### Global Configuration Objects

You can define global config blocks (e.g., for custom use):

```hyperbricks
myconf {
    some = value
}
```

Only the `hyperbricks` object is processed by the runtime. Other objects are allowed for organizational or user-defined purposes.


### Main Configuration Block

#### `hyperbricks { ... }`

This is the primary configuration block.


### Mode Settings

```hyperbricks
mode = development
```

Available modes:

* `development` – Ideal for local dev with live reload.
* `live` – Optimized for production deployment.
* `debug` – Extra verbose logging (used for diagnostics).


### Debugging

```hyperbricks
debug {
    level = debugging
}
```

Controls Go-level debug verbosity.


### Development Mode

```hyperbricks
development {
    watch = true
    reload = true
    frontend_errors = false
    dashboard = false
}
```

These settings are active only in `development` mode.


### Live Mode

```hyperbricks
live {
    cache = 10s
}
```

* Sets cache duration for rendered pages.
* Supports Go-style durations like `300ms`, `2h45m`, etc.


### Server Settings

```hyperbricks
server {
    port = 8080
    beautify = true
    read_timeout = 5s
    write_timeout = 10s
    idle_timeout = 20s
}
```

Adjust timeout values based on traffic level. High-traffic blogs may prefer higher values (see commented examples).


### System Settings

```hyperbricks
system {
    metrics_watch_interval = 10s
}
```

Interval for system ticker to gather and report metrics.


### Rate Limiting

```hyperbricks
rate_limit {
    requests_per_second = 100
    burst = 500
}
```

Control traffic with configurable request and burst limits. Adjust for your traffic level.


### Plugins

```hyperbricks
plugins {
    # Example:
    # enabled = [ MyPlugin@1.0.0, MyOtherPlugin@2.1.0 ]
}
```

Enable plugins by listing their config names (binary name without `.so`). Use `hyperbricks plugin help` and `hyperbricks plugin list` for details.
See plugin section on how to create and/or install plugins for Hyperbricks.


### Directory Settings

```hyperbricks
directories {
    render      = {{VAR:module}}/rendered
    static      = {{VAR:module}}/static
    resources   = {{VAR:module}}/resources
    plugins     = ./bin/plugins/
    templates   = {{VAR:module}}/templates
    hyperbricks = {{VAR:module}}/hyperbricks
    # logs = {{VAR:module}}/logs
}
```

| Key           | Purpose                                                 |
| ------------- | ------------------------------------------------------- |
| `render`      | Where static output is generated (`hyperbricks static`) |
| `static`      | Public assets served as-is, like minified JS/CSS        |
| `resources`   | Raw, unprocessed files like JS sources or markdown      |
| `plugins`     | Path to compiled plugin binaries                        |
| `templates`   | Used with `<TEMPLATE>` and template engines             |
| `hyperbricks` | Directory to scan for `.hyperbricks` files              |
| `logs`        | (Optional) Enable file-based logging                    |


## Module Directory Structure Guide

A Hyperbricks module follows a specific folder structure. Each folder serves a unique role in how your module is configured, rendered, and served.

```ini
someproject/
├── hyperbricks/
├── rendered/
├── resources/
├── static/
├── templates/
└── package.hyperbricks
```


### Folder Breakdown

#### `hyperbricks/`

Contains the core `.hyperbricks` configuration files.

* Files are loaded automatically in alphanumeric order from the root of this folder.
* Subdirectories are not auto-loaded — you must explicitly include them using `@import`.


#### `rendered/`

Default output folder used by the `hyperbricks static` command.

* Stores pre-rendered routes and other static outputs.
* Contents are typically generated — not edited by hand.


#### `resources/`

Raw asset and source data folder.

Use this to store:

* JavaScript source files
* Tailwind or other build configurations
* Uncompiled markdown documents
* Unprocessed images or data files


#### `static/`

Public asset directory available at runtime.

* Served relative to the root domain.
* Example: `static/css/styles.min.css` becomes `https://mydomain.com/static/css/styles.min.css`

Use it for precompiled CSS, JS, fonts, or images.


#### `templates/`

Stores template files used during rendering.

* Used for `<TEMPLATE>` blocks and any other templated output components.
* Often embedded into config via `hypermedia` markers.


#### `package.hyperbricks`

Module entry point.

* Defines the module's main configuration.
* Links together scripts, templates, resources, and routes.


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


## Hypermedia Markers: Cached Content Embedding

The `hypermedia` marker lets you preload files into memory and assign them as templates or raw text blocks. These are injected into the configuration and made instantly available — no runtime file access required.

### Why use it?

* Fast rendering
* Self-contained config
* Immutable state after load

### Structure

```hyperbricks
hypermedia.<key> = <TYPE>
hypermedia.<key>.<field> = {{<TYPE>:<file>}}
```

| Type       | Field      | Loaded From                |
| ---------- | ---------- | -------------------------- |
| `TEMPLATE` | `template` | `templates/` in the module |
| `TEXT`     | `value`    | Module's root directory    |

### Examples

```hyperbricks
hypermedia.10 = TEMPLATE
hypermedia.10.template = {{TEMPLATE:sometemplate.html}}

hypermedia.10 = TEXT
hypermedia.10.value = {{TEXT:sometext.md}}
```
