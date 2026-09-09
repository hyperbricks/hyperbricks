# Markdown WASM Plugin

This is a Go-authored WebAssembly plugin for the HyperBricks WASM plugin runtime. The CLI builds it with `GOOS=wasip1 GOARCH=wasm`, and the runtime provides WASI imports without mounting a filesystem.

Build it from the repository root:

```bash
hyperbricks plugin build markdown-wasm@1.0.0
```

The command emits:

```text
bin/plugins/MarkdownWasmPlugin@1.0.0.wasm
```

Use the artifact name without the `.wasm` suffix in module config:

```yaml
hyperbricks:
  plugins:
    enabled:
      - MarkdownWasmPlugin@1.0.0
```

Then render it with a normal plugin component:

```yaml
markdown:
  - type: plugin
  - plugin: MarkdownWasmPlugin@1.0.0
  - data:
      content: "# Hello from MarkdownWasmPlugin"
      class: wasm-markdown
```

The plugin exports the v1 WASM ABI:

- `memory`
- `alloc(size: i32) -> ptr: i32`
- `render(input_ptr: i32, input_len: i32) -> packed_ptr_len: i64`

Go/WASI modules also export `_initialize`; HyperBricks calls it before invoking `alloc` and `render`.

The output is JSON:

```json
{"kind":"html","html":"<div>...</div>"}
```
