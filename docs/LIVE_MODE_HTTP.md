# Live Mode HTTP Settings

Configure live mode in `package.hyperbricks.yaml`:

- `live.cache` controls reuse of rendered output.
- `server.*` controls HTTP connections and CPU parallelism.
- `rate_limit.*` controls the process-level request limiter.

Caching determines whether a route can reuse its output. Server settings limit how long clients can hold network resources. Rate limiting controls how many requests the process accepts. These settings work independently of the application's browser library.

## Connection Settings

The live server honors these values from `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  mode: live
  live:
    cache: 30s
  server:
    read_timeout: 5s
    write_timeout: 10s
    idle_timeout: 20s
    keep_alives_enabled: true
```

`read_timeout` is how long the server allows itself to read a request.

`write_timeout` is how long the server allows itself to write a response.

`idle_timeout` is how long an open keep-alive connection may sit unused before the server closes it.

`keep_alives_enabled` controls whether one TCP connection can be reused for multiple requests. Keep-alives should normally stay enabled for browser traffic, reverse proxies, and load balancers.

## Defaults

When omitted, HyperBricks uses these defaults:

- `read_timeout: 5s`
- `write_timeout: 10s`
- `idle_timeout: 20s`
- `keep_alives_enabled: true`

These defaults apply even when the `server` block does not explicitly list the settings.

## Request Limiter

The request limiter is enabled by default and uses a token bucket. It is independent from output caching and runs before route rendering.

```yaml
hyperbricks:
  rate_limit:
    enabled: true
    requests_per_second: 100
    burst: 500
```

Set `enabled: false` when a trusted reverse proxy owns rate limiting, or for a controlled renderer benchmark. Setting `requests_per_second` to zero is not a disable switch; an enabled zero-rate limiter rejects requests.

## Output Cache

For a guided first setup with observable results, follow [Caching: a practical guide](CACHING.md). This section defines the full configuration and runtime behavior.

`live.cache` supplies the default lifetime for reusable rendered output in live mode. The default is `10m`. Pages (`hypermedia`) and ordinary `fragment` routes can override that lifetime and choose memory or disk storage. Development and debug modes always render fresh, while still validating route cache configuration.

```yaml
hyperbricks:
  mode: live
  live:
    cache: 15s
```

When a route is cacheable, HyperBricks can add live cache metadata headers and serve repeated requests from the cache until the entry expires.

A scalar route duration selects memory storage and overrides the package lifetime:

```yaml
products:
  - type: hypermedia
  - route: products
  - cache: 30s
```

The expanded form selects either backend:

```yaml
products:
  - type: hypermedia
  - route: products
  - cache:
      storage: disk
      expire: 1h

stock:
  - type: fragment
  - route: stock
  - cache:
      storage: mem
      expire: 30s
```

These snippets show route policy only; add the page or fragment's content alongside it.

| Route setting | Result |
| --- | --- |
| `cache` omitted | Memory storage with the package lifetime. |
| `cache: 30s` | Memory storage for 30 seconds. |
| `cache: 0s` or `cache: {expire: 0s}` | No output caching for this route. |
| `cache: {storage: disk}` | Disk storage with the package lifetime. |
| `cache: {storage: disk, expire: 1h}` | Disk storage for one hour. |
| `nocache: true` | No output caching, regardless of `cache`. |

Normal mapping inheritance applies. A child overriding only `expire` keeps its parent's storage. A scalar duration replaces an inherited mapping and selects memory. An empty mapping does not erase inherited values. Unknown fields, invalid storage values, empty/null values and invalid or negative route durations are configuration errors. The mapping accepts only `storage` and `expire`; the separate Boolean `esbuild.cache` retains its build-cache meaning.

Before v1.3.0-beta, scalar route durations were accepted but ignored. They now take effect. Remove the route field to keep using the package lifetime; see [Migration](MIGRATION.md#route-cache-durations-now-take-effect).


Set the duration to exactly `0s` to disable the internal HyperBricks rendered-output cache for the complete live process:

```yaml
hyperbricks:
  mode: live
  live:
    cache: 0s
```

With `0s`, every route passes through rendering instead of looking up or storing an internal cache entry. The response consequently has no HyperBricks cache metadata. This is a process-wide switch and wins over positive route lifetimes for both storage backends. Use route-level `nocache: true` or `cache: 0s` when only selected routes must bypass the internal cache.

Use zero or a positive duration such as `15s`, `10m`, or `1h`. A negative duration such as `-1s` is invalid and runtime validation stops startup; the deployment package editor rejects it before saving. An invalid duration string currently logs a parse error and falls back to `24h` during ordinary startup, while the deployment package editor's strict validation rejects it. Check startup logs after changing this value.

The cached response retains its configured status and headers. Configure request-header variation with a literal `Vary` response header:

```yaml
status:
  - type: fragment
  - route: fragments/status
  - response:
      headers:
        Vary: Accept-Language
  - content:
      - type: html
      - value: '<section id="status">Ready</section>'
```

The named request headers become part of the internal cache key as well as the HTTP `Vary` contract. `Vary: "*"` bypasses internal caching. On `hypermedia`, the existing top-level `headers.Vary` is also supported; `response.headers` takes precedence for the same header. This declares cache separation only; it does not choose different rendered content by itself. There is no implicit `HX-Request` cache variant. See [HTTP responses](HTTP_RESPONSES.md).

## No-Cache Routes

Set `nocache: true` on the route owner when a route must stay dynamic.

Full page:

```yaml
page:
  - type: hypermedia
  - route: account
  - title: Account
  - nocache: true
  - main:
      - type: html
      - value: <main>Account content</main>
```

Fragment with [HTMX 4](https://four.htmx.org/) response headers:

```yaml
account_status:
  - type: fragment
  - route: fragments/account-status
  - nocache: true
  - response:
      headers:
        HX-Retarget: "#account-status"
        HX-Reswap: outerHTML
  - body:
      - type: html
      - value: <div id="account-status">Updated</div>
```

In this HTMX example, `HX-Retarget` and `HX-Reswap` tell HTMX where and how to insert the HTML. They do not control caching. The same `nocache: true` setting applies to fragments used by any browser client.

The `nocache` field belongs on the route owner because the live cache decision is made before child components render. Setting it only inside a nested template or tree item is not enough.

Routes with an enabled `guard` are also treated as non-cacheable, because authorization is request-specific. See [Route Guard](ROUTE_GUARD.md).

`api_fragment_render` routes are always dynamic.

## Mixing Cached and Dynamic Routes

A single live application can serve cached public pages, fresh account fragments, API actions, and streams. Choose the policy on each route owner:

| Use case | Route configuration | Internal rendered-output cache |
| --- | --- | --- |
| Public page whose content may stay unchanged for the configured duration | Leave `nocache` omitted or `false` | Reuses output until expiry. |
| Public page varying by language, host, or another request header | Declare the relevant `response.headers.Vary` | Stores separate variants for those header values. |
| Account page, frequently updated data, or state-changing action | `nocache: true` | Renders on every request. |
| Route with an enabled `guard` | Configure the guard normally | Rechecks the guard and renders permitted requests each time. |
| `api_fragment_render` route | Configure the API fragment normally | Calls the upstream for every request, including retries after failures. |
| Plugin returning a buffered or streaming `HandledResponse` | Use the handled-response plugin contract | Never stores the handled response or stream producer. Streams also send `Cache-Control: no-store`. |
| Response whose representation cannot be described by a finite set of headers | `response.headers.Vary: "*"` | Bypasses the internal cache. |

Embedding an `api_render` or ordinary plugin inside a page does **not** make the owning page dynamic. Set `nocache: true` on that page or fragment if its child data must be fetched again on every request. A cached parent skips rendering its children entirely.

### Internal Caching and HTTP Caching

`live.cache: 0s`, `nocache`, and `Cache-Control` control different scopes or layers. `live.cache: 0s` bypasses HyperBricks' internal rendered-output cache process-wide. `nocache: true` bypasses that same internal cache for one route owner. The HTTP `Cache-Control` header tells browsers and shared HTTP caches how to handle the response.

Setting `response.headers.Cache-Control: no-store` by itself **does not disable HyperBricks' internal cache**. Conversely, neither `live.cache: 0s` nor `nocache: true` adds an HTTP `Cache-Control` policy. Browser caches, reverse proxies, and CDNs therefore remain separate and must be configured through response headers and their own policies. For private content that must stay fresh at both layers, configure both:

```yaml
account_status:
  - type: fragment
  - route: fragments/account-status
  - nocache: true
  - response:
      headers:
        Cache-Control: private, no-store
  - body:
      - type: html
      - value: <div id="account-status">Current account status</div>
```

Conversely, a public page may deliberately use the internal cache while sending `Cache-Control: no-store` to clients. Repeated requests still reuse the server's rendered output for the route's effective cache lifetime.

### Request Variants

The internal cache separates requests by their query parameters, `Authorization`, `Cookie`, request body, and HTTP method. GET and HEAD share the same method variant. Additional request headers affect the key only when declared in `Vary`; this includes `Host`, `Accept-Language`, and `HX-Request` when the rendered content depends on them.

This separation prevents one variant from reusing another variant's stored output. It does not make a response fresh when the underlying database, session, permissions, or API data changes. Use `nocache: true` for those requirements. State-changing POST/PUT/DELETE routes also need `nocache: true`; an ordinary cacheable route can reuse identical non-GET requests.

Internal query/authentication/cookie separation does not automatically declare the corresponding HTTP caching policy. Configure the appropriate `Vary` and `Cache-Control` headers for clients and proxies. See [HTTP responses](HTTP_RESPONSES.md).

### Storage, Expiry, and Cleanup

Both backends use the same request variants, response status, headers, cookies and ETags. Memory storage keeps response bodies in the process. Disk storage keeps bodies in private files and retains a small metadata index in memory. Disk hits verify the file and stream it without loading the full body into a string. Rendering a cache miss still needs normal rendering memory. Disk trades filesystem work for lower retained body memory; it is not expected to beat memory-cache latency.

Expiry is fixed from the start of rendering. A hit does not extend it, and the next request after expiry renders fresh output. There is no background refresh or stale-while-revalidate behavior. A periodic sweep removes expired entries from both backends, including variants nobody requests again. Removing memory entries makes their storage eligible for Go garbage collection; it does not guarantee an immediate reduction in operating-system memory figures.

Configure the disk directory and maintenance limits in the package:

```yaml
hyperbricks:
  mode: live
  directories:
    cache: '{{MODULE}}/.cache'
  live:
    cache: 10m
    disk_cache:
      max_bytes: 268435456
      max_entries: 10000
      cleanup_interval: 1m
```

These are the defaults: **256 MiB of disk response bodies**, **10,000 disk entries**, and cleanup every **minute**. The cleanup interval applies to expired memory entries too. All three values must be positive. A size or entry limit first evicts expired disk entries, then the oldest entries; entries larger than the byte limit are served fresh. The body-byte limit applies per runtime, not to filesystem overhead, the small metadata index, already-open files held by in-flight responses, memory-cache bodies or all processes combined. Memory caching has no byte limit; avoid caching routes with many distinct, rarely reused variants.

The default layout is `<module>/.cache/responses/<runtime-id>/`. The directory is created on the first disk write. A custom `directories.cache` must be dedicated to private runtime data, outside source, resources, templates, plugins, static and rendered-output directories. Use `{{MODULE}}` for module-relative paths; bare relative paths follow the existing invocation-directory convention. A dedicated absolute path can be used for a writable container volume. Do not expose it through Caddy or another file server.

`.cache` directories are excluded by default from deployment packages and runtime snapshots. A configured cache directory inside the module is excluded too. These are built-in packaging rules, independent of `.gitignore`; see [Deployment cache exclusions](DEPLOY.md#response-cache-files-and-deployment). Cache directories are also excluded from source watching. Archive directory settings come from the module’s default `package.hyperbricks.yaml`. When starting with a different configuration profile, keep its cache under `.cache`, outside the module, or at the same cache location declared in the default package.

Each runtime starts with an empty response cache. Disk entries are disposable and are not reused across restarts. A configuration reload invalidates both backends. Normal shutdown removes owned files; startup on first disk use and periodic maintenance reclaim abandoned runtime directories only when their recorded local process is demonstrably absent. Files belonging to active or unknown owners are left alone. Multiple processes maintain independent caches and limits.

Missing, damaged or unwritable disk entries cause fresh rendering with a diagnostic, without retaining the body in a fallback memory cache. Invalidation rejects cache writes from renders started before a purge or reload, so older work cannot repopulate the new cache. An already-running response may finish.

Invalid HTTP/guard configuration and handled responses bypass caching. Render failures, rejected diagnostics, and HTTP statuses of `500` or higher also bypass caching and receive `Cache-Control: no-store`, so the next request can retry. Warnings and notices alone do not disable caching. Other non-200 statuses can still be cached; set the route policy explicitly and check `X-Hyperbricks-Render-Error-Count` during verification.

### Purging Memory and Disk

Run cache administration on the same host and as the same operating-system user as the running module:

```sh
hyperbricks cache purge --module my-site --all
hyperbricks cache purge --module my-site --route products
```

`--all` invalidates every cached response in that instance. `--route` invalidates all request variants for the selected route, in either backend. The next request renders fresh and caches according to the existing policy. Purge does not change configuration or eagerly render pages. Exactly one of `--all` or `--route` is required; unknown routes produce an error.

The command communicates with the running live instance through a private local Unix socket on macOS/Linux. It has no public HTTP endpoint and does not depend on the development dashboard. With several instances using the same module directory, the command lists their identifiers and requires `--instance <id>`; purge each intended instance separately. For deployed builds, pass the running build's extracted module directory with `--module`, rather than the source directory. A stopped process has no memory cache to purge; the command reports that no running instance was found.

The private connection lives in the operating-system user's cache directory. If local control cannot start, the server logs that purge control is unavailable and continues serving. Review startup logs before relying on automated purge commands. This operation affects HyperBricks' internal cache only; browser, proxy and CDN caches remain independent.

### Verifying a Mixed Configuration

Request each route twice with the same inputs. With a positive cache duration, a cacheable response includes `X-Hyperbricks-Rendered-At`, `X-Hyperbricks-Cache-Expires-At`, and an `ETag`; a cache hit retains the rendering timestamps. An uncached response has no HyperBricks cache metadata. With `live.cache: 0s`, both requests must render independently and neither response should contain HyperBricks cache metadata. Repeat with changed query/header values to confirm the intended variants, and retry after expiry to confirm a fresh render.

For a guarded route, verify that revoking access is enforced on the next request with the same token. For an API action or stream, verify fresh upstream work or a new producer on every request. Browser refresh alone is not proof of a server-side render when the route remains cacheable.

The runtime regression matrix is in [`server_live_cache_matrix_test.go`](../cmd/hyperbricks/server_live_cache_matrix_test.go). Run it from the repository root:

```sh
go test ./cmd/hyperbricks -run 'TestServeContent_LiveCache' -count=1
```

## Starter Profiles

Small public site:

```yaml
hyperbricks:
  mode: live
  live:
    cache: 30s
  server:
    read_timeout: 5s
    write_timeout: 10s
    idle_timeout: 20s
    keep_alives_enabled: true
```

Balanced production app:

```yaml
hyperbricks:
  mode: live
  live:
    cache: 15s
  server:
    read_timeout: 10s
    write_timeout: 15s
    idle_timeout: 30s
    keep_alives_enabled: true
```

Heavy pages or slower clients:

```yaml
hyperbricks:
  mode: live
  live:
    cache: 10s
  server:
    read_timeout: 15s
    write_timeout: 30s
    idle_timeout: 60s
    keep_alives_enabled: true
```

## Disabling Keep-Alives

Disable keep-alives only when you explicitly want one-request-per-connection behavior and understand the cost.

```yaml
hyperbricks:
  mode: live
  server:
    read_timeout: 5s
    write_timeout: 10s
    idle_timeout: 20s
    keep_alives_enabled: false
```

This usually increases connection churn and is not the normal production default.

## CPU parallelism

Configure process-wide Go execution parallelism in the selected module's `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  server:
    gomaxprocs: auto
```

Omitted or `auto` uses Go's CPU-aware default via `runtime.SetDefaultGOMAXPROCS()`. Go can periodically adjust this default to available CPU resources and supported container CPU limits. This replaces the former hard-coded value of four. Automatic mode does not create a HyperBricks polling loop or try to adjust concurrency based on request traffic.

For reproducible CPU allocation or intentionally reduced parallelism, use a fixed integer:

```yaml
hyperbricks:
  server:
    gomaxprocs: 2
```

Fixed values must be between **1 and `runtime.NumCPU()`**, inclusive. Zero, negative values, fractions, booleans, unknown strings and values above the detected logical CPU count cause startup to fail before the application starts serving requests. Numeric strings are accepted so environment resolvers can supply the value. An explicitly fixed setting disables Go's automatic updates and can exceed a container's CPU quota; use `auto` for container-aware adjustment.

Package configuration owns this setting: both automatic and fixed modes override a `GOMAXPROCS` environment variable. To configure it through the environment, use the YAML resolver explicitly:

```yaml
hyperbricks:
  server:
    gomaxprocs: {env: {name: HB_GOMAXPROCS, default: auto}}
```

The limit controls how many OS threads can execute Go code simultaneously. It does not reserve CPU cores, cap the number of goroutines, enforce a memory limit or replace request rate limiting. The setting applies to the entire application process, including static rendering; it is not a per-route or per-request option.

Changing the package value requires restarting the process. In `auto` mode Go's resource-aware updates remain dynamic after startup. Existing fixed-four deployments should set `gomaxprocs: 4` explicitly if the host has at least four logical CPUs and they need to preserve that behavior.
