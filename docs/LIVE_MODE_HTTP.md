# Live Mode HTTP Settings

Live mode has two separate concerns:

- `live.cache` controls rendered output reuse.
- `server.*` controls HTTP connection behavior.

Those settings solve different problems. Cache settings decide whether a route
can reuse rendered content. Server settings decide how long clients may hold
network resources.

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

`idle_timeout` is how long an open keep-alive connection may sit unused before
the server closes it.

`keep_alives_enabled` controls whether one TCP connection can be reused for
multiple requests. Keep-alives should normally stay enabled for browser traffic,
reverse proxies, and load balancers.

## Defaults

When omitted, HyperBricks uses these defaults:

- `read_timeout: 5s`
- `write_timeout: 10s`
- `idle_timeout: 20s`
- `keep_alives_enabled: true`

These defaults apply even when the `server` block does not explicitly list the
settings.

## Output Cache

`live.cache` controls how long rendered output may be stored and reused in live
mode.

```yaml
hyperbricks:
  mode: live
  live:
    cache: 15s
```

When a route is cacheable, HyperBricks can add live cache metadata headers and
serve repeated requests from the cache until the entry expires.

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

Fragment:

```yaml
account_status:
  - type: fragment
  - route: fragments/account-status
  - nocache: true
  - response:
      hx_target: "#account-status"
      hx_reswap: outerHTML
  - body:
      - type: html
      - value: <div id="account-status">Updated</div>
```

The `nocache` field belongs on the route owner because the live cache decision
is made before child components render. Setting it only inside a nested template
or tree item is not enough.

Routes with `guard` are also treated as non-cacheable, because authorization is
request-specific. See [Route Guard](ROUTE_GUARD.md).

`api_fragment_render` routes are always dynamic.

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

Disable keep-alives only when you explicitly want one-request-per-connection
behavior and understand the cost.

```yaml
hyperbricks:
  mode: live
  server:
    read_timeout: 5s
    write_timeout: 10s
    idle_timeout: 20s
    keep_alives_enabled: false
```

This usually increases connection churn and is not the normal production
default.

## Takeaway

Use `live.cache` for rendered output reuse.

Use `server.*` for connection safety and transport behavior.

Set `nocache: true` on the routed component when the response depends on the
current request.
