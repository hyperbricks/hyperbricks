# Troubleshooting

Start with the developer interface or server log and check the affected route. An HTTP `200` response can still contain component errors.

## Find the reported error

In development mode, use the developer interface's **Errors** section:

1. Set `hyperbricks.development.dashboard.enabled: true` and configure
   `hyperbricks.development.dashboard.credentials.user` and `.password` in
   `package.hyperbricks.yaml`, preferably through environment resolvers.
2. Set the referenced environment variables and restart the server. There is no
   default developer account.
3. Open `/__hyperbricks/dashboard` on your running server, complete the browser's Basic Auth
   challenge, and select **Errors**. You can also open
   `/__hyperbricks/errors` directly with the same login.
4. Select the affected route or request and read its diagnostics. Filter by
   severity when you need to separate errors from warnings.

Fix the reported source and request the affected route again. The **Diagnostic JSON** link opens the underlying diagnostic data. The Errors view is unavailable in live mode, production deployment, or static output.

If the developer interface is disabled, use the server log:

In development or debug mode, open the render diagnostics URL shown in the log.
It requires the module's `development.dashboard.credentials` even when the
Dashboard itself is disabled. It identifies the request and, where available,
the source file, component path, key, type, and error message. Fix that source
and request the route again.

The response header `X-Hyperbricks-Render-Error-Count` reports the number of collected diagnostics, including warnings and notices. Use `X-Hyperbricks-Request-ID` to match a response to its diagnostics. Without a request ID, `/__hyperbricks/render-diagnostics` lists current records containing diagnostics. A successful retry clears the earlier error for that request context; configuration reload and restart clear the store.

The Errors view also shows unchecked routes. An empty error list does not prove that every route or input has been tested.

The endpoint is disabled in live mode. A `503` response means the module's
developer credentials are absent or did not resolve; a `401` response means the
browser did not supply the configured login or supplied the wrong one. If the
server cannot start, read the terminal error instead. See
[HyperBricks CLI: Render diagnostics](HYPERBRICKS_CLI.md#render-diagnostics).

## A route returns 404

Check that you selected the right module with `-m` and that the route belongs to `hypermedia`, `fragment`, or `api_fragment_render`. Other component types do not create URLs by themselves.

`*.hyperbricks.yaml` files directly inside the configured `hyperbricks/` directory load automatically. Files in subdirectories need file-level `imports` in a loaded source file. Check startup diagnostics for invalid definitions. See [Routing](ROUTING.md) and [YAML Usage: Imports](YAML_USAGE.md#imports).

## Components render in the wrong order

Check the sequence of child components in the YAML. Each component entry starts with `-`; the renderer follows the configured child order. A mapping under `values` holds data and does not define the order of rendered children.

Keep the sequence syntax and check where inherited definitions or nested components supply the content. See [YAML Usage: Ordering](YAML_USAGE.md#ordering).

## A template value is empty

Check the configured value and the name used in the template. Ordinary values use `.name`; allowed query input uses `.Params.name`; API and Goja results use `.Data`.

Query input must be allowed by `querykeys`. Check resolver diagnostics if the value comes from a variable, environment setting, configuration, or file. See [YAML Usage: Template Syntax](YAML_USAGE.md#template-syntax) and [API Render](API_RENDER.md).

## An edit does not appear

Check that the file is loaded and development watching is enabled for its directory. Changes to `package.hyperbricks.yaml` require a server restart.

In live mode, the containing route may reuse cached output. Use `nocache: true` on that route when each request needs current data. Static output is a snapshot: rebuild it after source changes. See [Live Mode HTTP Settings](LIVE_MODE_HTTP.md) and [HyperBricks CLI: Static Rendering](HYPERBRICKS_CLI.md#static-rendering).

## An API fragment calls the API every time

This is the `api_fragment_render` contract. It bypasses the internal rendered-output cache and calls its upstream endpoint on every request.

A nested `api_render` also calls its endpoint whenever it executes, but a cache hit on its containing page or fragment skips that execution. Configure caching on the route owner. See [API Render: Cache Ownership](API_RENDER.md#cache-ownership).

## CSS or JavaScript is missing

Check the asset build error, source entry, output path, and emitted URL. Native `esbuild` output must stay in the configured static directory. The page must render the component's output, usually in its head, to include the script or stylesheet tag.

If source imports use npm packages, install the project's browser dependencies first. Native `esbuild` itself needs no plugin. See [Native esbuild](ESBUILD.md).

## A native plugin cannot load

Check that its configured name matches the enabled artifact and compiled filename, including its version and module suffix. Check the configured plugin directory.

The plugin and host must match in platform, Go toolchain, runtime source, and shared dependencies. After rebuilding a plugin, restart the server. Use `HYPERBRICKS_LOCAL_PATH` only when targeting a local HyperBricks checkout. See [Plugins: Local Runtime Development](PLUGINS.md#local-runtime-development).

## Static export fails

Read the first render error. Check that APIs and asset dependencies are available during the build. A non-2xx response from a nested `api_render` fails static rendering.

Export route discovery also includes routes found in the loaded source. `static.routes` adds targets; it does not restrict discovery to those targets. Use a source directory containing only static-ready routes when isolating an export. See [HyperBricks CLI: Static Rendering](HYPERBRICKS_CLI.md#static-rendering) and [API Render: Static Snapshots](API_RENDER.md#static-snapshots).

## An older configuration is rejected

Check [Migration Guide](MIGRATION.md) for removed response fields and changed API authentication settings. Update reusable definitions as well as the routes that inherit them.
