# Hyperbricks Plugins

This document explains the current plugin system, including global vs custom
plugins, naming conventions, directory layout, CLI commands, and the dashboard
plugin manager.

---

## Goals and design

- Keep plugin binaries in a stable, global location: `./bin/plugins`.
- Allow custom plugins to live inside module builds, compiled on the target host.
- Avoid alias layers by using explicit binary names in config.
- Provide a plugin manager UI in both `--deploy-remote` and `--deploy-local`.

---

## Plugin types

### Global plugins

Global plugins come from the public index and are shared across all modules.

- Source root: `./plugins/<name>/<version>/manifest.json`
- Output: `./bin/plugins/<Binary>@<version>.so`
- Enabled in a module via `plugins.enabled` using the binary name without `.so`.

### Custom plugins (module-level)

Custom plugins are part of a module and ship with builds.

- Source root (local): `modules/<module>/plugins/<name>/<version>/manifest.json`
- Source root (deploy-remote runtime): `<deploy.root>/<module>/runtime/<build_id>/plugins/<name>/<version>/manifest.json`
- Output: `./bin/plugins/<Binary>__<module>@<version>.so`
- Enabled via `plugins.enabled` using the full custom config name without `.so`.

---

## Naming rules

The config name is the binary name without the `.so` suffix. This name must be
used in:

- `plugins.enabled`
- `plugin = "..."` usage inside Hyperbricks configs

### How the binary name is derived

- If the manifest includes `binary`, that value is used (without `.so`).
- Otherwise the name is derived from the Go source file by converting to
  CamelCase and stripping `.go`.

### Examples

Global plugin:

- Manifest source: `esbuild_plugin.go`
- Derived base: `EsbuildPlugin`
- Config name: `EsbuildPlugin@1.0.10`
- Binary path: `./bin/plugins/EsbuildPlugin@1.0.10.so`

Custom plugin in module `test-003`:

- Manifest source: `my_plugin.go`
- Derived base: `MyPlugin`
- Config name: `MyPlugin__test-003@1.0.0`
- Binary path: `./bin/plugins/MyPlugin__test-003@1.0.0.so`

---

## Required config

Custom plugins only appear in the dashboard if they are listed in
`plugins.enabled` for that module. The custom plugins view only lists entries
whose config name ends with `__<module>@<version>`; global plugin names in the
module config stay in the Global Plugins view.

Example `package.hyperbricks`:

```hcl
plugins {
  enabled = [ MyPlugin__test-003@1.0.0 ]
}
```

Notes:

- Do not include `.so` in the config name.
- There is no auto-update of `package.hyperbricks`; update it manually when
  adding or renaming plugins.

---

## Directory layout summary

Global:

- Sources: `./plugins/<name>/<version>/manifest.json`
- Output: `./bin/plugins/<Binary>@<version>.so`

Custom (local):

- Sources: `modules/<module>/plugins/<name>/<version>/manifest.json`
- Output: `./bin/plugins/<Binary>__<module>@<version>.so`

Custom (deploy-remote runtime):

- Sources: `<deploy.root>/<module>/runtime/<build_id>/plugins/<name>/<version>/manifest.json`
- Output: `./bin/plugins/<Binary>__<module>@<version>.so`

---

## Manifest format

`manifest.json` example:

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

Fields:

- `plugin`: Repository identifier (used for index display).
- `source`: Go file to compile.
- `version`: Plugin version.
- `binary`: Optional explicit binary base name.
- `compatible_hyperbricks`: Semver constraints.
- `description`: Human-readable description.

---

## CLI commands

Global plugins:

- `hyperbricks plugin list`
- `hyperbricks plugin install <name>@<version>`
- `hyperbricks plugin build <name>@<version>`
- `hyperbricks plugin remove <name>@<version>`

Custom plugins:

- `hyperbricks plugin build <name>@<version> --module <module>`
- `hyperbricks plugin remove <name>@<version> --module <module>`

Build output always lands in `./bin/plugins` using the naming rules above.

---

## Dashboard plugin manager

Available for both `--deploy-remote` and `--deploy-local`:

- Global plugins tab: lists index and installed binaries.
- Custom plugins tab: lists plugins from `plugins.enabled` for the selected module
  (and build in remote mode).
- Each row shows a `Copy` button for the config name (without `.so`).
- Actions include compile/rebuild and remove; they run as background tasks with
  status/logs via task polling.

---

## API endpoints

Deploy-remote:

- `GET /deploy/plugins/global/index`
- `GET /deploy/plugins/global`
- `POST /deploy/plugins/global/install`
- `POST /deploy/plugins/global/rebuild`
- `POST /deploy/plugins/global/remove`
- `GET /deploy/plugins/custom?module=<m>&build_id=<id>`
- `POST /deploy/plugins/custom/compile`
- `POST /deploy/plugins/custom/remove`
- `GET /deploy/plugins/tasks/<task_id>`
- `GET /deploy/plugins/tasks/<task_id>/logs`

Deploy-local (no HMAC):

- `GET /local/plugins/global/index`
- `GET /local/plugins/global`
- `POST /local/plugins/global/install`
- `POST /local/plugins/global/rebuild`
- `POST /local/plugins/global/remove`
- `GET /local/plugins/custom?module=<m>`
- `POST /local/plugins/custom/compile`
- `POST /local/plugins/custom/remove`
- `GET /local/plugins/tasks/<task_id>`
- `GET /local/plugins/tasks/<task_id>/logs`

---

## Working directory

Plugin lookup uses `directories.plugins` as-is. The default is `./bin/plugins`
which is CWD-relative, so deploy services must run with a stable
WorkingDirectory.

---

## Troubleshooting

- Plugin does not appear in Custom Plugins view:
  - Confirm it is listed in `plugins.enabled` for that module.
  - Confirm `manifest.json` exists in the module plugin source folder.
- Build fails:
  - Ensure Go is installed on the host.
  - Check task logs from the dashboard or `/plugins/tasks/<task_id>/logs`.
- Plugin loads but runtime cannot find it:
  - Make sure `directories.plugins` points to `./bin/plugins`.
  - Verify the config name matches the compiled binary name (without `.so`).
