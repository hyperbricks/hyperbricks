# Plugins

HyperBricks plugins let a module delegate rendering to compiled code while keeping the route and component structure in YAML.

There are two runtime artifact formats:

- Native Go plugins: `.so`
- WebAssembly plugins: `.wasm`

There are two plugin source types:

- Global plugins from `./plugins`.
- Custom module plugins from `modules/<module>/plugins`.

Both artifact formats are installed into `./bin/plugins` and enabled from `package.hyperbricks.yaml`.

## Enable Plugins

Enable compiled plugin artifacts without the `.so` or `.wasm` suffix:

```yaml
hyperbricks:
  plugins:
    enabled:
      - ExamplePlugin@1.0.0
      - CustomWidget__demo@1.0.0
```

The runtime loads each enabled plugin from the configured plugin directory. By default that directory is `./bin/plugins`. A plugin name may resolve to either a `.so` or `.wasm` artifact. If both artifacts exist for the same enabled plugin name, startup rejects the plugin instead of guessing.

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

The `plugin` field must match the enabled artifact name exactly, without `.so` or `.wasm`. The `data` map is plugin-specific input.

## Native Streaming Responses

A native Go plugin can return `shared.HandledResponse` with a `Stream` callback to produce its HTTP body incrementally. Use this for a plugin-owned response, such as a finite stream of SSE HTML updates. A regular rendered string remains a buffered response; this contract does not automatically stream nested page components or proxy an upstream API stream. The current WASM ABI returns HTML and does not support a Go stream callback.

The callback type is:

```go
func(context.Context, io.Writer, func() error) error
```

Prepare status, content type, headers and cookies in `Render`, then return the callback. Do not start writing during `Render` or retain its context for later use. HyperBricks evaluates the route guard before rendering and calls the stream only after rendering finishes. The callback receives the live HTTP request context, a body writer and a flush function.

The server sends and flushes the response headers before invoking the callback. Perform validation that may require a different HTTP status during `Render`. For `HEAD` requests and statuses without a response body (`204`, `205`, `304`), the callback is not invoked, although `Render` can still run. Keep work needed only to produce the stream inside the callback.

Read any request data you need during `Render` and capture client-specific values in that request's callback. The registered plugin instance is shared between requests; a shared field representing the current user or current response would not be request-scoped. Shared configuration and connection pools can stay on the plugin instance. Keep mutable customer data local to the request. Capturing a map or pointer in a callback does not copy its contents; ensure that mutable client data belongs to that request rather than shared plugin state.

Before streaming begins, HyperBricks consumes up to 256 KiB of any unread request body, with a maximum wait of five seconds. An existing shorter server read timeout still applies. Excess unread data receives `413`, incomplete or malformed data receives `400`, and a read timeout receives `408`; the producer does not run. This limit applies to leftover data, not the size of a body the plugin has already fully read. Consume larger uploads during `Render`, with the application's own size and validation rules, before returning a streaming response. Leave request body closure to the HTTP server so it can verify that reading reached EOF. This ensures that a disconnected HTTP/1 client can be detected while the producer is idle.

For example, a plugin can return a single SSE message like this:

```go
return shared.HandledResponse{
    Status:      http.StatusOK,
    ContentType: "text/event-stream",
    Headers:     map[string]string{"Cache-Control": "no-store"},
    NoCache:     true,
    Stream: func(requestContext context.Context, output io.Writer, flush func() error) error {
        ctx, cancel := context.WithTimeout(requestContext, 4*time.Second)
        defer cancel()
        if err := ctx.Err(); err != nil {
            return err
        }
        if _, err := fmt.Fprint(output, "data: <p>Ready</p>\n\n"); err != nil {
            return err
        }
        return flush()
    },
}, nil
```

This Go example uses `context`, `fmt`, `io`, `net/http`, `time` and the HyperBricks `shared` package. For several updates, write and flush each complete message, and make waits select on `ctx.Done()` as well as their timer. Stop on cancellation or a write/flush error and return the error to the server. Do not retain the writer or flush function after the callback returns, and do not write from background goroutines. Cancellation is cooperative: the server cannot stop arbitrary plugin code that ignores its context or blocks outside these calls.

The writer sends the bytes supplied by the plugin; it does not HTML-escape them. When producing HTML containing user input, render it with Go `html/template` or escape the values appropriately. The example above sends only constant HTML.

`Body` and `Stream` are mutually exclusive. One response has one owner: if multiple rendered plugins return handled responses, HyperBricks rejects the conflict before running a stream. Only the server writes status and headers; the callback writes body bytes. It must not fetch a response writer from the render context or try to change metadata after streaming starts.

Stream responses bypass the rendered-output cache. The server enforces `Cache-Control: no-store` and removes `Content-Length`, `ETag` and HyperBricks cache metadata, including values supplied by the plugin. Set `nocache: true` on dynamic plugin routes, especially ones that sometimes return ordinary HTML: route cache lookup happens before plugin rendering, so an earlier cached HTML response could otherwise hide a later stream. Flushing publishes each completed chunk without waiting for the rest of the stream. The configured `hyperbricks.server.write_timeout` continues to apply; streaming does not silently extend it. Set an appropriate server timeout and an application deadline for the stream. Once the response has started, a callback error ends it and is recorded by the server; it cannot be replaced with a new error status or an HTML error page. Intermediary proxies may need their own buffering settings for early delivery.

The [native streaming demo](../modules/streaming-demo/README.md) uses HTMX in the browser to display the streamed HTML updates. It provides a complete module, build instructions and cancellation tests. Its `fragment` route contains one `plugin` component. The streaming response contract is independent of HTMX; no additional streaming YAML component is required.

## WASM Plugins

WASM plugins use the same `<PLUGIN>` component contract as native plugins. The render pipeline does not change: HyperBricks still handles routing, nesting, wrapping, response handling, and errors.

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

The packed return value stores the output pointer in the high 32 bits and the output length in the low 32 bits.

The host writes normalized plugin JSON into guest memory before calling `render`. The WASM plugin returns JSON:

```json
{
  "kind": "html",
  "html": "<div>Hello</div>",
  "errors": []
}
```

Only `kind: "html"` is supported in the first WASM runtime pass. The adapter returns the HTML string through the existing plugin renderer contract.

WASM plugins may be plain no-import modules or Go/WASI modules. If a module imports `wasi_snapshot_preview1`, HyperBricks provides wazero's WASI imports without mounting a filesystem or configuring environment variables. WASM does not provide network or process-spawning APIs. Each render call creates a fresh module instance from the compiled module and runs with a host-side timeout and memory-page limit.

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

Custom plugins belong to one module and include the module name in the compiled binary.

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

When `binary` is omitted, HyperBricks derives the binary base name from the source file by removing its extension and converting the name to CamelCase.

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

When testing plugin compatibility against a local HyperBricks checkout, build the plugin with a local runtime path:

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
- Add module plugins to `package.hyperbricks.yaml`; HyperBricks does not edit that file automatically.
