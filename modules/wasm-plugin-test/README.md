# WASM plugin fixture

A manual compatibility fixture for rendering WebAssembly plugins through
ordinary HyperBricks `plugin` components.

The index page combines Markdown rendered by `MarkdownWasmPlugin@1.0.0` and a
content card rendered by `MyPluginWasmPlugin@1.0.0`. Both receive their input
through YAML `data` fields and return HTML to the surrounding page.

The two example plugin sources are included in this repository. Build their
`.wasm` artifacts into `bin/plugins/` before starting the module, from the
repository root with Go available:

```sh
go run ./cmd/hyperbricks plugin build markdown-wasm@1.0.0
go run ./cmd/hyperbricks plugin build myplugin_wasm@1.0.0
go run ./cmd/hyperbricks start -m wasm-plugin-test --port 8080
```

Open <http://localhost:8080/>. This is a focused plugin verification fixture,
not a complete application; the card's `/docs/plugins` link is sample data,
not a route supplied by this module. See the [WASM plugin guide](../../docs/PLUGINS.md#wasm-plugins)
for the runtime contract and limitations.
