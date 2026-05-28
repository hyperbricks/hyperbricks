# Plugins

HyperBricks plugins are Go shared objects loaded by the runtime. They let a
module delegate rendering to compiled Go code while keeping the route and
component structure in YAML.

There are two plugin source types:

- Global plugins from `./plugins`.
- Custom module plugins from `modules/<module>/plugins`.

Both are compiled into `./bin/plugins` and enabled from
`package.hyperbricks.yaml`.

## Enable Plugins

Enable compiled plugin binaries without the `.so` suffix:

```yaml
hyperbricks:
  plugins:
    enabled:
      - ExamplePlugin@1.0.0
      - CustomWidget__demo@1.0.0
```

The runtime loads each enabled plugin from the configured plugin directory. By
default that directory is `./bin/plugins`.

```yaml
hyperbricks:
  directories:
    plugins: ./bin/plugins
```

## Use A Plugin Component

Use the `plugin` component inside a page, fragment, template value, or tree.

```yaml
page:
  - type: hypermedia
  - route: plugin-demo
  - title: Plugin demo
  - main:
      - type: tree
      - widget:
          - type: plugin
          - plugin: ExamplePlugin@1.0.0
          - data:
              title: Rendered by a plugin
              variant: compact
```

The `plugin` field must match the enabled binary name exactly, without `.so`.
The `data` map is plugin-specific input.

## Global Plugins

Global plugins come from the public plugin index and are shared across modules.

Source layout:

```text
plugins/<name>/<version>/manifest.json
```

Build output:

```text
bin/plugins/<Binary>@<version>.so
```

Config name:

```text
<Binary>@<version>
```

Example:

```text
Source: plugins/example/1.0.0/manifest.json
Output: bin/plugins/ExamplePlugin@1.0.0.so
Config: ExamplePlugin@1.0.0
```

## Custom Module Plugins

Custom plugins belong to one module and include the module name in the compiled
binary.

Source layout:

```text
modules/<module>/plugins/<name>/<version>/manifest.json
```

Build output:

```text
bin/plugins/<Binary>__<module>@<version>.so
```

Config name:

```text
<Binary>__<module>@<version>
```

Example for module `demo`:

```text
Source: modules/demo/plugins/widget/1.0.0/manifest.json
Output: bin/plugins/CustomWidget__demo@1.0.0.so
Config: CustomWidget__demo@1.0.0
```

## Manifest

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

| Field | Required | Purpose |
| --- | ---: | --- |
| `plugin` | yes | Repository or plugin identifier |
| `source` | yes | Go source file to compile |
| `version` | yes | Plugin version |
| `binary` | no | Explicit binary base name |
| `compatible_hyperbricks` | yes | Compatible runtime versions |
| `description` | yes | Human-readable description |

When `binary` is omitted, HyperBricks derives the binary base name from the Go
source file by removing `.go` and converting the name to CamelCase.

```text
example_plugin.go -> ExamplePlugin
```

## CLI

List compatible global plugins:

```bash
hyperbricks plugin list
```

Install and build a global plugin:

```bash
hyperbricks plugin install example@1.0.0
```

Build a plugin from local source:

```bash
hyperbricks plugin build example@1.0.0
```

Build a custom module plugin:

```bash
hyperbricks plugin build widget@1.0.0 --module demo
```

Remove a compiled plugin:

```bash
hyperbricks plugin remove example@1.0.0
```

Update a global plugin to the latest compatible version:

```bash
hyperbricks plugin update example
```

## Local Runtime Development

When testing plugin compatibility against a local HyperBricks checkout, build
the plugin with a local runtime path:

```bash
hyperbricks plugin build example@1.0.0 \
  --hyperbricks-path /path/to/hyperbricks
```

This avoids publishing temporary runtime versions just to test plugin changes.

## Rules

- Use the compiled binary name in `plugins.enabled`.
- Use the same compiled binary name in the YAML `plugin` component.
- Do not include `.so` in configuration.
- Keep global and custom plugin names distinct.
- Rebuild plugins after runtime API changes.
- Add module plugins to `package.hyperbricks.yaml`; HyperBricks does not edit
  that file automatically.
