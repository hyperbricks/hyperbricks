# Plugins

Use plugins to run compiled code inside a module's component tree. YAML defines the routes and component structure; the plugin handles its rendering task.

HyperBricks supports two formats:

- **Native Go plugins:** `.so` files.
- **WebAssembly plugins:** `.wasm` files.

Global plugin sources live in `./plugins`. Module-specific sources live in `modules/<module>/plugins`. Both formats install into `./bin/plugins`. Enable them in `package.hyperbricks.yaml`.

Spaces and Markdown are built in on supporting runtimes. Configure development editing under `hyperbricks.development.frontend_editing.spaces`; see [Spaces](SPACES.md). Use `type: markdown` to render content or resource files; see [Markdown](MARKDOWN.md).

## Platform Support

**Native Go plugins (`.so`) are not supported on Windows.** They cannot be built or loaded when HyperBricks runs directly on Windows. Go's plugin system supports Linux, FreeBSD, and macOS only; see the [official Go plugin documentation](https://pkg.go.dev/plugin#hdr-Warnings).

To use native plugins from a Windows machine, run both HyperBricks and the plugin build in a Linux environment, such as WSL2 or a Linux container. See [building plugins in Docker](DOCKER.md#build-plugins). Build native plugins for the runtime's operating system and architecture, using the same Go toolchain and matching shared dependencies; a plugin built for macOS cannot be loaded by a Linux runtime.

This Go plugin restriction does not apply to [WebAssembly plugins](#wasm-plugins), which use HyperBricks' separate WASM runtime. WASM plugins have their own execution limits and do not provide all native Go plugin capabilities, including streaming callbacks, network access, or process spawning.

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

## Returning Component Configuration

A native plugin can return runtime component configuration for HyperBricks to
render. This is useful when the plugin validates, normalizes, or calculates
values while a regular HyperBricks template remains responsible for the HTML.
The returned map uses runtime metadata such as `"@type": "<TEMPLATE>"`; YAML
source continues to use declarations such as `type: template`. A `<TREE>` wrapper
is optional when the plugin needs to compose several returned components.

The [template configuration plugin pattern](../modules/hyperbricks-patterns-yaml/docs/pages/template-config-plugin.md)
contains a complete input, output, template, and safety example.

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

## Shared (Global) Plugins

Global plugins come from the public plugin index and are shared across modules.
The local and remote deployment interfaces label this host-wide list **Shared
Plugins**; module-specific plugins appear under **Modules → Custom plugins**.

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

For a compatible installed published HyperBricks release, use the commands below with `HYPERBRICKS_LOCAL_PATH` **unset**. If it was exported during development, run `unset HYPERBRICKS_LOCAL_PATH` before returning to release builds.

Both `plugin install` and `plugin build` compile plugin source. That alone does not require a local HyperBricks checkout. `HYPERBRICKS_LOCAL_PATH` is a **development-only override** that selects local HyperBricks source instead of the release dependency. See [Local runtime development](#local-runtime-development).

List compatible global plugins:

```bash
hyperbricks plugin list
```

Install and build a global plugin:

```bash
hyperbricks plugin install example@1.0.0
```

Build a plugin from its existing source files:

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

Install the highest published version of a global plugin by omitting the version:

```bash
hyperbricks plugin install example
```

Check its compatibility before adopting it: this selects the highest published semantic version, not necessarily the latest compatible version. To change versions, use `install <name>@<version>` and update the module's configured plugin names.

## Local Runtime Development

Use `HYPERBRICKS_LOCAL_PATH` only when building plugins for a HyperBricks runtime built from a local source checkout. From that checkout's root:

```bash
HYPERBRICKS_LOCAL_PATH="$PWD" go run ./cmd/hyperbricks plugin build widget@1.0.0 --module demo
go run ./cmd/hyperbricks start -m demo
```

You can also install the local CLI before building:

```bash
go install ./cmd/hyperbricks
HYPERBRICKS_LOCAL_PATH="$PWD" hyperbricks plugin build widget@1.0.0 --module demo
hyperbricks start -m demo
```

Installing with `go install ./cmd/hyperbricks` still produces a local-source runtime. Ensure the `hyperbricks` command on `PATH` is that installation. Native plugins and their host must match in source, toolchain, platform, and shared dependencies.

The override works with both `plugin build` and `plugin install`. It points at the **HyperBricks checkout**, not the plugin directory. `plugin install` also accepts `--hyperbricks-path /absolute/path/to/hyperbricks` as a command-specific alternative. `plugin build` does not accept that flag; use the environment variable. No local path is needed for normal builds targeting an installed published release.

### Build automatically for an opted-in development session

A module can declare a `development.hooks.before_start` task that invokes the
existing plugin CLI. Run `hyperbricks start --with-processes` to enable that
preparation. The hook must succeed before component/plugin initialization, so a
failed rebuild stops the session even when an older artifact exists.

Use `HB_EXECUTABLE` in the script to select the running CLI and align its working
directory with the builder's `./bin/plugins` output and the runtime's
`directories.plugins`. The
[development-hooks-plugin-demo](../modules/development-hooks-plugin-demo/README.md)
is a runnable example: it embeds a fresh build identifier and verifies the exact
loaded artifact through an HTTP route. See
[Development hooks](DEVELOPMENT_HOOKS.md#build-a-development-plugin) for the
lifecycle contract. Native compatibility requirements still apply; production
builds remain part of build/package preparation.

### How The CLI Selects The Dependency

The native plugin builder uses two paths. With a local override, it adds a Go module `replace` directive pointing to the HyperBricks checkout. Without one, it selects the CLI's embedded HyperBricks version and removes the unversioned local replacement. The embedded version is not an exact Git revision, so a development runtime can contain newer code than its version label suggests. Installing that checkout with `go install ./cmd/hyperbricks` does not turn it into a published-release build.

See the [native plugin builder](../cmd/hyperbricks/commands/plugin-commands.go), Go's [module replacement contract](https://go.dev/ref/mod#go-mod-file-replace), and the official [native plugin compatibility requirements](https://pkg.go.dev/plugin#hdr-Warnings). The local override selects source; it does not by itself guarantee matching toolchains, build settings, or shared dependencies.

## Repository Maintainer Workflow

The CLI commands above are the application-developer workflow for individual plugins. HyperBricks contributors can use the aggregate [plugin build and smoke scripts](../scripts/plugins/README.md) to rebuild the repository's plugin-backed demos and fixtures against this checkout. `./tests.sh --with-plugins` invokes that workflow as part of the full test suite.

On a clean checkout, first install the pinned shared Markdown and Tailwind CSS plugins through the local CLI with `HYPERBRICKS_LOCAL_PATH` pointing at the checkout, as shown in the [shared plugin setup](../scripts/plugins/README.md#install-the-shared-plugin-sources). Their sources are downloaded into `./plugins`; they are not bundled in the HyperBricks repository. Module-specific custom plugin sources are bundled under `modules/<module>/plugins`. The aggregate build wrapper rebuilds these existing sources and does not install missing shared plugins.

## Rules

- Use the compiled binary name in `plugins.enabled`.
- Use the same compiled binary name in the YAML `plugin` component.
- Do not include `.so` or `.wasm` in configuration.
- Keep global and custom plugin names distinct.
- Rebuild plugins after runtime API changes.
- Add module plugins to `package.hyperbricks.yaml`; HyperBricks does not edit that file automatically.
