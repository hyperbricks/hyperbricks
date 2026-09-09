# Live Mode HTTP Settings

Live mode has three separate concerns:

- `live.cache` controls rendered output reuse.
- `server.*` controls HTTP connection behavior.
- `rate_limit.*` controls the process-level request limiter.

Those settings solve different problems. Cache settings decide whether a route can reuse rendered content. Server settings decide how long clients may hold network resources. Rate limiting controls how many requests the process accepts. These settings work independently of the browser library used by the application.

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

`live.cache` controls how long rendered output may be reused in live mode. It is one process-wide duration, shared by cacheable routes; there is no per-route duration override. The default is `10m`.

```yaml
hyperbricks:
  mode: live
  live:
    cache: 15s
```

When a route is cacheable, HyperBricks can add live cache metadata headers and serve repeated requests from the cache until the entry expires.

Use a valid duration such as `15s`, `10m`, or `1h`. An invalid duration string currently logs a parse error and falls back to `24h`; check startup logs after changing this value. Use route-level `nocache: true` to bypass caching explicitly.

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

`nocache` and `Cache-Control` control different layers. `nocache: true` bypasses HyperBricks' internal rendered-output cache. The HTTP `Cache-Control` header tells browsers and shared HTTP caches how to handle the response.

Setting `response.headers.Cache-Control: no-store` by itself **does not disable HyperBricks' internal cache**. Likewise, `nocache: true` alone does not add an HTTP `Cache-Control` policy. For private content that must stay fresh at both layers, configure both:

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

Conversely, a public page may deliberately use the internal cache while sending `Cache-Control: no-store` to clients. Repeated requests still reuse the server's rendered output for `live.cache`.

### Request Variants

The internal cache separates requests by their query parameters, `Authorization`, `Cookie`, request body, and HTTP method. GET and HEAD share the same method variant. Additional request headers affect the key only when declared in `Vary`; this includes `Host`, `Accept-Language`, and `HX-Request` when the rendered content depends on them.

This separation prevents one variant from reusing another variant's stored output. It does not make a response fresh when the underlying database, session, permissions, or API data changes. Use `nocache: true` for those requirements. State-changing POST/PUT/DELETE routes also need `nocache: true`; an ordinary cacheable route can reuse identical non-GET requests.

Internal query/authentication/cookie separation does not automatically declare the corresponding HTTP caching policy. Configure the appropriate `Vary` and `Cache-Control` headers for clients and proxies. See [HTTP responses](HTTP_RESPONSES.md).

### Expiry, Updates, and Memory

Entries live in memory in each HyperBricks process. On a request for an expired entry, HyperBricks renders fresh content and replaces that entry. It does not refresh entries in the background. The cache currently has no size limit or periodic removal of expired entries; an unused expired variant can remain in memory until the cache is cleared or the process stops.

Choose `live.cache` according to how long public content may remain stale. After a data change, an already cached page can continue serving its previous output until expiry. A full configuration reinitialization clears the internal cache; restarting or deploying a new process also starts with an empty cache. There is no public per-route invalidation API. With multiple processes, each has its own cache and expiry timing.

Use `nocache: true` for routes with many distinct search queries, session cookies, large request bodies, or rapidly changing data when storing all those variants offers little reuse. Monitor process memory for applications with many cacheable variants. Expiry limits reuse time, not memory usage.

Invalid HTTP/guard configuration and handled responses bypass caching. Ordinary component render errors are recorded separately: a nonempty partial render can still be cached with its error count. Do not assume every diagnostic or non-200 status automatically disables caching; set the route policy explicitly and check `X-Hyperbricks-Render-Error-Count` during verification.

### Verifying a Mixed Configuration

Request each route twice with the same inputs. A cacheable response includes `X-Hyperbricks-Rendered-At`, `X-Hyperbricks-Cache-Expires-At`, and an `ETag`; a cache hit retains the rendering timestamps. An uncached response has no HyperBricks cache metadata. Repeat with changed query/header values to confirm the intended variants, and retry after expiry to confirm a fresh render.

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
