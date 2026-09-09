# MyPlugin WASM

`myplugin_wasm` is a small Go/WASM plugin example for HyperBricks. It renders a simple content card from plugin `data` and is meant to show the full path from source code to a usable `.wasm` plugin artifact.

The example uses the v1 WASM plugin ABI:

```text
memory
alloc(size: i32) -> ptr: i32
render(input_ptr: i32, input_len: i32) -> packed_ptr_len: i64
```

Because this plugin is written in Go, the CLI builds it as a WASI module:

```text
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared
```

HyperBricks provides the required WASI imports with wazero, but does not mount a filesystem or expose network/process APIs to the plugin.

## Source Layout

```text
plugins/myplugin_wasm/1.0.0/
  go.mod
  manifest.json
  myplugin_wasm.go
  readme.md
```

The manifest marks the plugin as a WASM plugin:

```json
{
  "plugin": "github.com/hyperbricks/plugins/myplugin_wasm",
  "source": "myplugin_wasm.go",
  "runtime": "wasm",
  "binary": "MyPluginWasmPlugin",
  "version": "1.0.0"
}
```

The `binary` field controls the config name. With this manifest, the global artifact name is:

```text
MyPluginWasmPlugin@1.0.0
```

and the generated artifact path is:

```text
bin/plugins/MyPluginWasmPlugin@1.0.0.wasm
```

## Build As A Global Plugin

Run this from the HyperBricks repository root:

```bash
hyperbricks plugin build myplugin_wasm@1.0.0
```

During local development from source, this equivalent command is useful:

```bash
go run ./cmd/hyperbricks plugin build myplugin_wasm@1.0.0
```

Expected output:

```text
Building: myplugin_wasm Version: 1.0.0
Building plugin: myplugin_wasm
Using Go WASM toolchain: ...
Build successful: .../bin/plugins/MyPluginWasmPlugin@1.0.0.wasm
Config name: MyPluginWasmPlugin@1.0.0
```

## Enable A Global Plugin In A Module

Add the config name to the module's `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  plugins:
    enabled:
      - MyPluginWasmPlugin@1.0.0
```

Then use the same name in a normal `<PLUGIN>` component:

```yaml
page:
  - type: hypermedia
  - route: myplugin-wasm
  - title: MyPlugin WASM
  - main:
      - type: tree
      - demo_card:
          - type: plugin
          - plugin: MyPluginWasmPlugin@1.0.0
          - data:
              title: Hello WASM plugin
              message: This card was rendered by a Go/WASM plugin.
              eyebrow: Plugin example
              cta_label: Open docs
              cta_href: /docs/plugins
              accent: "#2563eb"
              class: demo-wasm-card
```

Render or start the module:

```bash
hyperbricks static -m demo --force
hyperbricks start -m demo
```

## Build As A Module-Local Plugin

Use a module-local plugin when the plugin belongs to one SaaS tenant/module or when you do not want the source to live in the global `plugins/` folder.

Create this folder:

```text
modules/demo/plugins/myplugin_wasm/1.0.0/
```

Copy these files into it:

```text
go.mod
manifest.json
myplugin_wasm.go
readme.md
```

Build with the `--module` flag:

```bash
hyperbricks plugin build myplugin_wasm@1.0.0 --module demo
```

The module-local artifact name includes the module suffix:

```text
MyPluginWasmPlugin__demo@1.0.0
```

The generated artifact path is:

```text
bin/plugins/MyPluginWasmPlugin__demo@1.0.0.wasm
```

Enable that module-local config name in `modules/demo/package.hyperbricks.yaml`:

```yaml
hyperbricks:
  plugins:
    enabled:
      - MyPluginWasmPlugin__demo@1.0.0
```

Use that exact config name in the component:

```yaml
demo_card:
  - type: plugin
  - plugin: MyPluginWasmPlugin__demo@1.0.0
  - data:
      title: Local module plugin
      message: This artifact was built from modules/demo/plugins.
      eyebrow: Module plugin
      accent: "#0f766e"
```

## Deploy Notes

For a global plugin deployment:

1. Build the global artifact before starting or packaging the module.
2. Ensure `bin/plugins/MyPluginWasmPlugin@1.0.0.wasm` exists on the deployed runtime host.
3. Keep `plugins.enabled` and the component `plugin` field set to `MyPluginWasmPlugin@1.0.0`.

For a module-local plugin deployment:

1. Keep the source under `modules/<module>/plugins/myplugin_wasm/1.0.0/`.
2. Build it with `hyperbricks plugin build myplugin_wasm@1.0.0 --module <module>`.
3. Enable and render the suffixed config name: `MyPluginWasmPlugin__<module>@1.0.0`.
4. Build or deploy the module after the `.wasm` artifact exists.

Do not include `.wasm` in `plugins.enabled` or in the component `plugin` field.

## Data Fields

| Field | Required | Description |
| --- | ---: | --- |
| `title` | no | Card title. |
| `message` | no | Body text. |
| `eyebrow` | no | Small label above the title. |
| `cta_label` | no | Link text. Link is omitted when this is empty. |
| `cta_href` | no | Link target. Allows `http://`, `https://`, `mailto:`, `/...`, and `#...`. |
| `accent` | no | Basic CSS color value. Unsafe characters fall back to blue. |
| `class` | no | Wrapper class. Defaults to `myplugin-wasm-card`. |

## How The Plugin Works

1. HyperBricks normalizes the `<PLUGIN>` component into JSON.
2. The host calls `alloc` in the WASM module and writes that JSON into guest memory.
3. The host calls `render(input_ptr, input_len)`.
4. The plugin decodes `data`, renders escaped HTML, encodes `{"kind":"html","html":"..."}`, and returns a packed pointer/length.
5. HyperBricks reads the JSON and sends the HTML through the existing plugin renderer contract.
