# Plugins

HyperBricks plugins let a module delegate rendering to compiled code while
keeping the route and component structure in YAML.

There are two runtime artifact formats:

- Native Go plugins: `.so`
- WebAssembly plugins: `.wasm`

There are two plugin source types:

- Global plugins from `./plugins`.
- Custom module plugins from `modules/<module>/plugins`.

Both artifact formats are installed into `./bin/plugins` and enabled from
`package.hyperbricks.yaml`.

## Enable Plugins

Enable compiled plugin artifacts without the `.so` or `.wasm` suffix:

```yaml
hyperbricks:
  plugins:
    enabled:
      - ExamplePlugin@1.0.0
      - CustomWidget__demo@1.0.0
```

The runtime loads each enabled plugin from the configured plugin directory. By
default that directory is `./bin/plugins`. A plugin name may resolve to either a
`.so` or `.wasm` artifact. If both artifacts exist for the same enabled plugin
name, startup rejects the plugin instead of guessing.

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

The `plugin` field must match the enabled artifact name exactly, without `.so`
or `.wasm`. The `data` map is plugin-specific input.

## WASM Plugins

WASM plugins use the same `<PLUGIN>` component contract as native plugins. The
render pipeline does not change: HyperBricks still handles routing, nesting,
wrapping, response handling, and errors.

Artifact layout:

```text
bin/plugins/MarkdownWasmPlugin@1.0.0.wasm
```

Config name:

```text
MarkdownWasmPlugin@1.0.0
```

The v1 WASM ABI is:

```text
exports:
  memory
  alloc(size: i32) -> ptr: i32
  render(input_ptr: i32, input_len: i32) -> packed_ptr_len: i64
```

The packed return value stores the output pointer in the high 32 bits and the
output length in the low 32 bits.

The host writes normalized plugin JSON into guest memory before calling
`render`. The WASM plugin returns JSON:

```json
{
  "kind": "html",
  "html": "<div>Hello</div>",
  "errors": []
}
```

Only `kind: "html"` is supported in the first WASM runtime pass. The adapter
returns the HTML string through the existing plugin renderer contract.

WASM plugins may be plain no-import modules or Go/WASI modules. If a module
imports `wasi_snapshot_preview1`, HyperBricks provides wazero's WASI imports
without mounting a filesystem or configuring environment variables. WASM does
not provide network or process-spawning APIs. Each render call creates a fresh
module instance from the compiled module and runs with a host-side timeout and
memory-page limit.

## Global Plugins

Global plugins come from the public plugin index and are shared across modules.

Source layout:

```text
plugins/<name>/<version>/manifest.json
```

Build output:

```text
bin/plugins/<Binary>@<version>.so
bin/plugins/<Binary>@<version>.wasm
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
bin/plugins/<Binary>__<module>@<version>.wasm
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
  "runtime": "native",
  "version": "1.0.0",
  "binary": "ExamplePlugin",
  "compatible_hyperbricks": [">=1.1.0-beta"],
  "description": "Example plugin"
}
```

| Field | Required | Purpose |
| --- | ---: | --- |
| `plugin` | yes | Repository or plugin identifier |
| `source` | yes | Source file to compile |
| `runtime` | no | `native`/`go` for Go `.so` plugins, or `wasm` for WebAssembly plugins. Defaults to `native`. |
| `version` | yes | Plugin version |
| `binary` | no | Explicit binary base name |
| `compatible_hyperbricks` | yes | Compatible runtime versions |
| `description` | yes | Human-readable description |

When `binary` is omitted, HyperBricks derives the binary base name from the
source file by removing its extension and converting the name to CamelCase.

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

Build the bundled WASM markdown testcase:

```bash
hyperbricks plugin build markdown-wasm@1.0.0
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
- Do not include `.so` or `.wasm` in configuration.
- Keep global and custom plugin names distinct.
- Rebuild plugins after runtime API changes.
- Add module plugins to `package.hyperbricks.yaml`; HyperBricks does not edit
  that file automatically.
