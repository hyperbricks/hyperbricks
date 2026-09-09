# Streaming Demo

One request delivers four progress updates over about three seconds. HTMX replaces the progress panel as each update arrives. A note outside the panel stays in place. The task is simulated; the HTTP stream is real.

HyperBricks serves both the page and `POST /demo/events`. The event route contains one native Go plugin that returns a `shared.HandledResponse.Stream` callback. The ordinary HTTP server sends the callback's output as it arrives. The primary demo uses one server, with no proxy or external API.

## Build And Run

Run these commands from this HyperBricks repository root with Go 1.26.1 or newer. Build the executable and plugin from the same checkout and Go toolchain; a previously installed HyperBricks binary may not have this streaming contract. The browser dependencies are included locally, so no npm installation is needed.

```sh
go build -o ./bin/hyperbricks-streaming-demo ./cmd/hyperbricks
HYPERBRICKS_LOCAL_PATH="$PWD" ./bin/hyperbricks-streaming-demo plugin build streaming-demo@1.0.0 --module streaming-demo
./bin/hyperbricks-streaming-demo start -m streaming-demo --port 18110 --non-interactive
```

The plugin build writes `bin/plugins/StreamingDemoPlugin__streaming-demo@1.0.0.so`. The module already enables that exact name. `HYPERBRICKS_LOCAL_PATH` makes the plugin build use the same local source as the executable; it is required while this API is unreleased. Rebuild both artifacts together after changing the shared plugin contract.

Open [the streaming demo](http://127.0.0.1:18110/). That HyperBricks listener owns the page, assets and stream. Stop it with Ctrl+C when finished. If the earlier two-server prototype is still running, stop those processes before starting this single-server version on its port.

## Try It

1. Type a note in **Keep your place**.
2. Click **Start demo**. The panel shows **Started**, **Preparing**, **Processing** and **Finished**. The received-updates list records their arrival times.
3. Check that your note stays and the browser URL does not change.
4. Start again and click **Stop** before it finishes. The last update stays visible, the connection closes, and no further steps arrive.
5. Click **Start demo** again. Each request starts its own sequence.

In browser developer tools, the Network panel shows one `POST /demo/events` per run. Its response has `Content-Type: text/event-stream`; updates arrive before the request finishes. There is no polling and no browser timer generating the progress. The optional JavaScript records actual received messages and controls the Start/Stop buttons.

## Where To Look

| File | Role |
| --- | --- |
| [Route configuration](hyperbricks/10-streaming-demo.hyperbricks.yaml) | Full page and the `demo/events` fragment containing one plugin. |
| [Native plugin](plugins/streaming-demo/1.0.0/streaming_demo_plugin.go) | Deferred SSE producer, one flush per message, cancellation and error handling. |
| [Plugin tests](plugins/streaming-demo/1.0.0/streaming_demo_plugin_test.go) | Deferred execution, callback context, early flush, completion, cancellation, deadlines, write/flush failures and independent requests. |
| [Plugin manifest](plugins/streaming-demo/1.0.0/manifest.json) | Native build and registration name. |
| [Page template](templates/demo.html) | Request button, fragment target, note field and received-updates list. |
| [Browser helper](static/js/demo.js) | Start/Stop state and arrival times. HTMX performs the requests and swaps. |
| [Module configuration](package.hyperbricks.yaml) | Enabled plugin, directories, listener and server timeouts. |
| [Optional test server](tools/demo-server/main.go) | Original independent SSE producer and page proxy, retained for comparison. |

## HTTP And Plugin Contract

The page uses an ordinary request with an explicit target:

```html
<button hx-post="/demo/events" hx-target="#stream-panel" hx-swap="innerHTML">
  Start demo
</button>
```

The plugin requires `POST`; other methods receive `405` and `Allow: POST`. `Render` prepares status and headers and returns a callback without producing body bytes. HyperBricks runs that callback after rendering, with the live HTTP request context, an `io.Writer` and an error-returning flush function. The plugin does not access a response writer from the render context.

The demo sends no request body. For plugins that accept input, read and validate it during `Render`, then capture the needed values in the callback. HyperBricks settles a remaining body before starting the stream: at most 256 KiB and five seconds, subject to a shorter configured read timeout. Invalid, incomplete or excess leftover data is rejected before the producer runs. Leave request body closure to the HTTP server so the preflight can verify EOF.

Each unnamed SSE message contains one HTML update. HTMX's SSE extension swaps that HTML into the chosen panel. A blank line ends each message. The callback flushes each complete message and returns after the fourth update. The server sends `Cache-Control: no-store`, bypasses rendered-output caching and emits no `Content-Length` for the stream.

Each callback starts its own four-second context deadline. The first update is immediate, followed by three one-second waits. A wait stops when the request is cancelled; write and flush errors return immediately. Start, step, completion and cancellation logs include a distinct stream ID. Concurrent requests share only the ID counter, not their sequence or cancellation state.

The module's `write_timeout: 15s` remains active. The plugin deadline does not extend the server timeout. The finite example does not claim durability, reconnection, production proxy behavior or a load test. It does not stream page components automatically or proxy an upstream API stream. A protected version can place a normal route guard on the fragment before the plugin runs.

See [Native streaming responses](../../docs/PLUGINS.md#native-streaming-responses) for the core contract, including single-owner responses and the native/WASM boundary.

## Verify The Plugin

From the repository root, after building the plugin:

```sh
go -C modules/streaming-demo/plugins/streaming-demo/1.0.0 test -race . -count=1
```

The plugin's module points at this checkout. The build CLI may rewrite that local replacement to the checkout's absolute path. The committed relative path keeps the bundled example portable inside the repository.

## Optional Independent Baseline

The original `tools/demo-server` remains available to compare the browser interaction with a standalone Go HTTP producer. It owns `/demo/events` itself and proxies the page and assets to HyperBricks. This optional baseline needs two servers; it is not used by the native demo above. Its page content describes the primary native setup, while this comparison supplies the SSE response separately.

Stop the primary demo first. In one terminal run:

```sh
./bin/hyperbricks-streaming-demo start -m streaming-demo --port 18111 --non-interactive
```

In another terminal run:

```sh
go run ./modules/streaming-demo/tools/demo-server -port 18110 -upstream http://127.0.0.1:18111
```

Open the same [demo URL](http://127.0.0.1:18110/). Stop both processes with Ctrl+C when finished. The baseline server listens on loopback only. Its separate tests are available with:

```sh
go test -race ./modules/streaming-demo/tools/demo-server -count=1
```

## Basis And Credits

The interaction follows **Update an Element → Stream an Update** from the [official HTMX 4 SSE example](https://four.htmx.org/extensions/hx-sse). The official [multipart example](https://four.htmx.org/extensions/hx-multipart) shows another transport; this demo uses SSE only.

`static/vendor/htmx-4.0.0.min.js` and `static/vendor/hx-sse-4.0.0.min.js` are unmodified copies from `htmx.org@4.0.0`. The included [HTMX license](static/vendor/HTMX-LICENSE.txt) retains the upstream attribution.
