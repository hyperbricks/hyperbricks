# Hyperbricks Plugins

This document explains the Hyperbricks plugin system: global plugins, custom module-level plugins, naming conventions, directory layout, CLI commands, dashboard plugin management, and local development workflows for testing unreleased Hyperbricks changes.

---

## Goals and design

* Keep compiled plugin binaries in one stable location: `./bin/plugins`.
* Support public/global plugins from the shared plugin index.
* Support custom plugins inside module builds, compiled on the target host.
* Use explicit plugin binary names in config instead of alias layers.
* Provide plugin management through both CLI and dashboard.
* Support dashboard plugin management in both `--deploy-remote` and `--deploy-local`.
* Allow local testing of unreleased Hyperbricks core changes without publishing temporary Git tags.

---

## Plugin types

Hyperbricks supports two plugin types:

1. Global plugins
2. Custom plugins

---

## Global plugins

Global plugins come from the public plugin index and are shared across modules.

### Source location

```text
./plugins/<name>/<version>/manifest.json
```

### Build output

```text
./bin/plugins/<Binary>@<version>.so
```

### Config usage

Enable global plugins in a module using the compiled binary name without the `.so` suffix:

```hcl
plugins {
  enabled = [ EsbuildPlugin@2.0.0 ]
}
```

---

## Custom plugins

Custom plugins are module-level plugins. They are part of a module and ship with that module’s builds.

### Source location, local mode

```text
modules/<module>/plugins/<name>/<version>/manifest.json
```

### Source location, deploy-remote runtime

```text
<deploy.root>/<module>/runtime/<build_id>/plugins/<name>/<version>/manifest.json
```

### Build output

```text
./bin/plugins/<Binary>__<module>@<version>.so
```

### Config usage

Enable custom plugins with the full custom config name, without `.so`:

```hcl
plugins {
  enabled = [ MyPlugin__test-003@1.0.0 ]
}
```

---

## Naming rules

The config name is the plugin binary name without the `.so` suffix.

This exact name must be used in:

* `plugins.enabled`
* `plugin = "..."` usage inside Hyperbricks configs

---

## How the binary name is derived

The `binary` field in `manifest.json` is optional.

If `binary` is present, it overrides the base name that would otherwise be derived from the Go source file.

If `binary` is absent, Hyperbricks derives the base name from the Go source file by:

1. Stripping `.go`
2. Converting the result to CamelCase

Example:

```text
upload_plugin.go -> UploadPlugin
```

With an explicit binary field:

```json
{
  "source": "upload_plugin.go",
  "binary": "Upload"
}
```

The base name becomes:

```text
Upload
```

The binary name affects:

* The compiled output name
* The config name used in `plugins.enabled`
* The `plugin = "..."` value used in Hyperbricks configs

The binary name does not affect source discovery. Source discovery still uses:

```text
plugins/<name>/<version>/manifest.json
modules/<module>/plugins/<name>/<version>/manifest.json
```

---

## Naming examples

### Global plugin

Manifest source:

```text
esbuild_plugin.go
```

Derived base:

```text
EsbuildPlugin
```

Config name:

```text
EsbuildPlugin@1.0.10
```

Binary path:

```text
./bin/plugins/EsbuildPlugin@1.0.10.so
```

### Custom plugin in module `test-003`

Manifest source:

```text
my_plugin.go
```

Derived base:

```text
MyPlugin
```

Config name:

```text
MyPlugin__test-003@1.0.0
```

Binary path:

```text
./bin/plugins/MyPlugin__test-003@1.0.0.so
```

---

## Required config

Custom plugins only appear in the dashboard if they are listed in `plugins.enabled` for that module.

The Custom Plugins view only lists entries whose config name ends with:

```text
__<module>@<version>
```

Global plugin names in the module config remain in the Global Plugins view.

Example `package.hyperbricks`:

```hcl
plugins {
  enabled = [ MyPlugin__test-003@1.0.0 ]
}
```

Rules:

* Do not include `.so` in the config name.
* There is no automatic update of `package.hyperbricks`.
* Update `package.hyperbricks` manually when adding, removing, or renaming plugins.

---

## Directory layout summary

### Global plugins

```text
Source: ./plugins/<name>/<version>/manifest.json
Output: ./bin/plugins/<Binary>@<version>.so
```

### Custom plugins, local mode

```text
Source: modules/<module>/plugins/<name>/<version>/manifest.json
Output: ./bin/plugins/<Binary>__<module>@<version>.so
```

### Custom plugins, deploy-remote runtime

```text
Source: <deploy.root>/<module>/runtime/<build_id>/plugins/<name>/<version>/manifest.json
Output: ./bin/plugins/<Binary>__<module>@<version>.so
```

---

## Manifest format

Example `manifest.json`:

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

### Fields

| Field                    | Required | Description                                                 |
| ------------------------ | -------: | ----------------------------------------------------------- |
| `plugin`                 |      yes | Repository identifier used for index display.               |
| `source`                 |      yes | Go source file to compile.                                  |
| `version`                |      yes | Plugin version.                                             |
| `binary`                 |       no | Explicit binary base name. Overrides source-derived naming. |
| `compatible_hyperbricks` |      yes | Semver constraints for compatible Hyperbricks versions.     |
| `description`            |      yes | Human-readable plugin description.                          |

---

## CLI commands

### Global plugins

```bash
hyperbricks plugin list
hyperbricks plugin install <name>@<version>
hyperbricks plugin build <name>@<version>
hyperbricks plugin remove <name>@<version>
```

If no version is supplied to `install`, Hyperbricks selects the highest available plugin version from the plugin index.

Example:

```bash
hyperbricks plugin install esbuild@2.0.0
```

### Custom plugins

```bash
hyperbricks plugin build <name>@<version> --module <module>
hyperbricks plugin remove <name>@<version> --module <module>
```

Example:

```bash
hyperbricks plugin build myplugin@1.0.0 --module test-003
```

Build output always lands in:

```text
./bin/plugins
```

using the naming rules above.

---

## Building against a Hyperbricks version

During plugin builds, Hyperbricks patches the plugin module so the plugin is compiled against the active Hyperbricks version.

By default, the version comes from the installed Hyperbricks binary.

You can override it explicitly:

```bash
hyperbricks plugin install esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

or:

```bash
hyperbricks plugin build esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

In release mode, this version must exist as a real Git revision or tag for:

```text
github.com/hyperbricks/hyperbricks
```

If the tag does not exist, `go mod tidy` fails with an error like:

```text
unknown revision v1.0.1-beta
```

---

## Local Hyperbricks core development

When developing Hyperbricks core and plugins at the same time, do not create temporary Git tags only to test plugin compatibility.

Use a local Hyperbricks checkout override instead.

This avoids:

* Public test tags
* Go module proxy/index noise
* `pkg.go.dev` indexing for temporary versions
* Repeated prerelease tag cleanup
* Per-plugin `replace` edits
* Per-project `go.work` maintenance

---

## Local development override

The plugin builder supports a local Hyperbricks checkout path.

This can be supplied with an environment variable:

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
```

or explicitly with a CLI flag:

```bash
hyperbricks plugin install esbuild@2.0.0 --hyperbricks-path "$HOME/Github/hyperbricks"
```

For custom plugins:

```bash
hyperbricks plugin build myplugin@1.0.0 \
  --module test-003 \
  --hyperbricks-path "$HOME/Github/hyperbricks"
```

When a local Hyperbricks path is active, the plugin builder should apply this module override inside the plugin module:

```go
require github.com/hyperbricks/hyperbricks v0.0.0

replace github.com/hyperbricks/hyperbricks => /absolute/path/to/hyperbricks
```

This works even when plugins are cloned or built from temporary directories, because the replacement path is absolute.

---

## Why `go work` is not enough

`go work` is useful for local monorepo development, but it is not a complete solution for the Hyperbricks plugin installer.

The installer may clone or build plugins outside the workspace, for example in a temporary directory:

```text
/var/folders/.../.hyperbricks-plugin-*
```

A Go workspace only applies when:

* the current directory is inside the workspace tree, or
* `GOWORK` explicitly points to a `go.work` file

Even with `GOWORK`, the cloned plugin module must be listed in the workspace `use (...)` block. Temporary plugin clones are not stable enough for this to scale.

For plugin builds across many projects and machines, use a central local override instead:

```text
HYPERBRICKS_LOCAL_PATH=/absolute/path/to/hyperbricks
```

---

## Development vs release mode

Use two explicit modes.

### Development mode

Development mode builds plugins against a local Hyperbricks checkout.

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
hyperbricks plugin install esbuild@2.0.0
```

Effective module behavior:

```go
require github.com/hyperbricks/hyperbricks v0.0.0
replace github.com/hyperbricks/hyperbricks => /absolute/path/to/hyperbricks
```

Use this for:

* Local Hyperbricks core changes
* Plugin compatibility testing
* Multi-plugin test builds
* Multi-project local development

### Release mode

Release mode builds plugins against a real Hyperbricks version tag.

```bash
unset HYPERBRICKS_LOCAL_PATH
hyperbricks plugin install esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

Effective module behavior:

```go
require github.com/hyperbricks/hyperbricks v1.0.1-beta
```

No local `replace` should be present in release mode.

Use this for:

* Published releases
* Production builds
* CI builds that should verify real module resolution

---

## Recommended safety rules

The plugin builder should enforce these rules:

* If `--hyperbricks-path` is set, use local development mode.
* If `HYPERBRICKS_LOCAL_PATH` is set, use local development mode.
* If both `--hyperbricks-path` and `--hyperbricks-version` are set, fail unless explicitly allowed.
* In release mode, remove any existing `replace github.com/hyperbricks/hyperbricks` entry.
* In release mode, require the requested version to exist as a real tag or revision.
* Normalize version input so both `1.0.1-beta` and `v1.0.1-beta` are handled correctly.
* Never publish temporary test tags only to make plugin builds pass.

---

## Dashboard plugin manager

The dashboard plugin manager is available for both `--deploy-remote` and `--deploy-local`.

### Global Plugins tab

The Global Plugins tab lists:

* Available plugins from the public index
* Compatible versions
* Installed binaries
* Compatibility status against the current Hyperbricks version

Actions include:

* Install
* Rebuild
* Remove

### Custom Plugins tab

The Custom Plugins tab lists module-level plugins from `plugins.enabled` for the selected module.

In remote mode, the selected build is also used.

Actions include:

* Compile
* Rebuild
* Remove

Each row exposes a `Copy` button for the config name without `.so`.

Actions run as background tasks. Status and logs are read through task polling endpoints.

---

## API endpoints

### Deploy-remote

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

### Deploy-local

Deploy-local endpoints do not use HMAC.

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

---

## Working directory

Plugin lookup uses `directories.plugins` as-is.

The default is:

```text
./bin/plugins
```

This path is relative to the current working directory.

Deploy services must therefore run with a stable `WorkingDirectory`.

If the service starts from a different directory, Hyperbricks may compile plugins successfully but fail to load them at runtime.

---

## Troubleshooting

### Plugin does not appear in Custom Plugins view

Check:

* The plugin is listed in `plugins.enabled` for that module.
* The config name ends with `__<module>@<version>`.
* `manifest.json` exists in the module plugin source folder.
* The selected module and build are correct in the dashboard.

### Build fails with `unknown revision`

Example:

```text
unknown revision v1.0.1-beta
```

Cause:

```text
The requested Hyperbricks version does not exist as a Git tag or revision.
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

### Plugin builds but runtime cannot find it

Check:

* `directories.plugins` points to `./bin/plugins` or the intended plugin directory.
* The process has a stable working directory.
* The config name exactly matches the compiled binary name without `.so`.
* The plugin was built for the same Hyperbricks version that is running.

### Plugin is marked incompatible

Hyperbricks inspects the compiled plugin binary with:

```bash
go version -m <plugin.so>
```

If the embedded `github.com/hyperbricks/hyperbricks` version differs from the running Hyperbricks version, the plugin may be shown as incompatible.

Rebuild the plugin against the current Hyperbricks version or use the local development override when testing unreleased core changes.

---

## Recommended workflow

### Local core + plugin development

```bash
export HYPERBRICKS_LOCAL_PATH="$HOME/Github/hyperbricks"
hyperbricks plugin install esbuild@2.0.0
hyperbricks plugin build myplugin@1.0.0 --module test-003
```

### Release

```bash
unset HYPERBRICKS_LOCAL_PATH
git tag v1.0.1-beta
git push origin v1.0.1-beta
hyperbricks plugin install esbuild@2.0.0 --hyperbricks-version v1.0.1-beta
```

### Rule

Use local override for tests. Use Git tags only for releases.
