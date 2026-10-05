# Caching: A Practical Guide

A cache lets HyperBricks reuse a response it has already rendered. The first
request does the rendering work; later requests can reuse that output until its
lifetime expires. You choose which pages or fragments may reuse output, how
long it remains valid, and whether to keep the response body in memory or on disk.

This guide walks through those choices with a small route whose timestamp makes
caching visible. You will see fresh rendering, enable memory caching, observe
expiry, switch to disk, and clear the cache yourself. The detailed rules remain
in the [output-cache reference](LIVE_MODE_HTTP.md#output-cache).

## Before You Begin

Use HyperBricks **v1.3.0-beta or newer**, which supports route cache storage and
expiry. If HyperBricks is not installed, start with the [Quickstart](QUICKSTART.md).
The terminal examples below use macOS or Linux and `curl`.

Run commands from your project root, the directory containing `modules/`. Create
a separate learning module so you can change its settings freely:

```sh
hyperbricks version
hyperbricks init -m cache-demo
```

This creates `modules/cache-demo/` with a working starter. Keep its generated
files. We will add one route and edit a few package settings. If that module
already exists, use it only if it is your learning module, or choose a new name
and substitute it in the paths and commands below.

## 1. Watch a Route Render Fresh

Caching applies in **live mode**. Edit the existing
`modules/cache-demo/package.hyperbricks.yaml` and set these fields, preserving
the rest of the package:

```yaml
hyperbricks:
  mode: live
  live:
    cache: 10m
```

Merge these values into the existing `hyperbricks` and `live` mappings; do not
add a second `hyperbricks` key. The package lifetime supplies the default for
cacheable routes. Development and debug modes always render fresh, even when
route cache settings are present.

Create `modules/cache-demo/hyperbricks/cache-demo.hyperbricks.yaml`:

```yaml
cache_demo:
  - type: fragment
  - route: cache-demo
  - nocache: true
  - content_type: text/plain; charset=utf-8
  - response:
      headers:
        Cache-Control: no-store
  - template:
      inline: 'Rendered at {{ (now).UnixNano }}'
```

This fragment returns a timestamp generated during rendering. `nocache: true`
makes each request render again. The example uses plain text so the timestamp
is easy to inspect; the same route cache settings work on full pages of type
`hypermedia`.

Check the module, then start it:

```sh
hyperbricks doctor -m cache-demo --strict
hyperbricks start -m cache-demo --port 8189
```

Leave this terminal running. In a second terminal, make two requests:

```sh
curl -i http://127.0.0.1:8189/cache-demo
curl -i http://127.0.0.1:8189/cache-demo
```

The body timestamps should differ. We send `Cache-Control: no-store` to prevent
browser/proxy reuse while learning. That header controls HTTP clients; it does
not switch off HyperBricks' internal cache. `nocache` controls internal reuse.
See [internal caching and HTTP caching](LIVE_MODE_HTTP.md#internal-caching-and-http-caching).

## 2. Reuse the Response in Memory

Stop the server with **Ctrl+C**. In the route file, replace the whole
`- nocache: true` line with:

```yaml
  - cache: 30s
```

Leave the rest of the route unchanged, including `Cache-Control: no-store`.
Removing `nocache: true` matters: it takes priority over a positive cache
lifetime. Start the server again with the same command:

```sh
hyperbricks start -m cache-demo --port 8189
```

Now make the two requests again. Within thirty seconds, they should return the
**same body timestamp**. HyperBricks renders the first request and keeps its
response body in memory for reuse. The route's `30s` overrides the package's
`10m` default.

You should also see these response headers:

| Header | What to look for |
| --- | --- |
| `X-Hyperbricks-Rendered-At` | Stays the same while the stored response is reused. |
| `X-Hyperbricks-Cache-Expires-At` | Shows when that cached response expires. |
| `ETag` | Identifies the response content for conditional HTTP requests. |

A request ID may change between requests; compare the body and cache headers.

The expanded form of the same route setting is:

```yaml
  - cache:
      storage: mem
      expire: 30s
```

Choose one form. If you omit `cache` entirely, an eligible route uses memory
storage and the package lifetime. See the [configuration table and inheritance
rules](LIVE_MODE_HTTP.md#output-cache) for the other forms.

## 3. See What Expiry Actually Does

After a successful cached request, wait longer than the route lifetime:

```sh
sleep 31
curl -i http://127.0.0.1:8189/cache-demo
curl -i http://127.0.0.1:8189/cache-demo
```

The first request after expiry should show a new body timestamp. The following
request should reuse that new response. A cache hit does not restart the clock:
the lifetime is fixed from the start of rendering.

Expiry allows the **next request** to render fresh output. It does not schedule
background rendering every thirty seconds. A periodic cleanup removes expired
stored entries even when nobody requests them again; expired entries cannot be
served while waiting for that cleanup.

Choose a lifetime by asking: **how old may this response be before the user
needs a new version?** A public article can usually tolerate more reuse than a
stock indicator. Changing a database or upstream API does not automatically
invalidate a cached page; expiry or a deliberate purge lets the next request
pick up that change.

See [storage, expiry, and cleanup](LIVE_MODE_HTTP.md#storage-expiry-and-cleanup)
for the precise lifecycle.

## 4. Move the Response Body to Disk

Memory is a good starting choice for frequently requested output. Disk storage
is useful when you want response bodies stored in files instead of retained in
the application's memory cache. Disk still uses memory for its index, active
requests, and operating-system file caching; it adds filesystem work to a hit.

Stop the server. Replace the route's cache setting with:

```yaml
  - cache:
      storage: disk
      expire: 5m
```

The longer lifetime gives you time to inspect the files. Restart the server,
request the route, then look in the module:

```sh
hyperbricks start -m cache-demo --port 8189
```

In the second terminal, from the same project root:

```sh
curl -i http://127.0.0.1:8189/cache-demo
ls -la modules/cache-demo/.cache/responses/runtime-*/
```

The first disk write creates a private runtime directory containing the cached
body. Another request within five minutes should reuse the response. The
directory layout is:

```text
modules/cache-demo/.cache/responses/<runtime-id>/
```

**Keep the server running while inspecting the files.** A normal shutdown
removes that process's cache directory, and a restart begins with an empty cache.
Disk response caching stores disposable runtime output; it does not preserve
responses between server sessions. Files can also disappear after expiry,
purge, or eviction when a disk limit is reached.

The default disk limits are **256 MiB of response bodies** and **10,000 entries**,
with cleanup every **minute**. To change them, merge a `disk_cache` mapping into
the package's existing `hyperbricks.live` mapping:

```yaml
hyperbricks:
  live:
    cache: 10m
    disk_cache:
      max_bytes: 268435456
      max_entries: 10000
      cleanup_interval: 1m
```

These limits apply to disk entries per running instance. A body larger than the
byte limit is served fresh without being stored in memory as a fallback. The
limits do not impose a memory-cache budget. For custom directories, eviction,
and filesystem failure behavior, see [storage, expiry, and cleanup](LIVE_MODE_HTTP.md#storage-expiry-and-cleanup).

After changing package settings, restart the server. Live mode does not watch
configuration files; cache purge clears stored responses but does not reload
configuration.

## 5. Clear a Cached Response Before It Expires

Keep the server running. From the second terminal, purge the route:

```sh
hyperbricks cache purge -m cache-demo --route cache-demo
curl -i http://127.0.0.1:8189/cache-demo
```

The next request should show a new body timestamp even though the five-minute
lifetime had not elapsed. Purging removes the stored response; that next request
renders and stores a replacement according to the route's policy.

To clear every memory and disk response in this running instance:

```sh
hyperbricks cache purge -m cache-demo --all
```

Run purge on the same host and as the same operating-system user as the server.
If several processes run the same module, the command reports their instance
identifiers so you can select one with `--instance`. Purge clears HyperBricks'
internal cache; browser and proxy caches have their own lifetimes.

See [purging memory and disk](LIVE_MODE_HTTP.md#purging-memory-and-disk) for
instance selection and deployed-module paths.

## 6. Choose a Policy for Your Application

Apply the setting to the page or fragment that owns the route. You can keep
different policies in the same module:

| Need | Route setting | Result in live mode |
| --- | --- | --- |
| Use the package default | Omit `cache` | Memory storage with `live.cache`. |
| Reuse a short-lived public fragment | `cache: 30s` | Memory storage for thirty seconds. |
| Keep a public response body on disk | `cache: {storage: disk, expire: 5m}` | Disk storage for five minutes. |
| Recalculate on every request | `nocache: true` | Always render fresh. |

Use fresh rendering when each request must observe current account, permission,
or application state, and for endpoints that perform actions. Query parameters,
cookies, and authorization inputs separate cache variants, but separation alone
does not keep underlying data current. If output depends on additional headers,
declare the corresponding `Vary` values. See [request variants](LIVE_MODE_HTTP.md#request-variants)
and [no-cache routes](LIVE_MODE_HTTP.md#no-cache-routes).

An `api_fragment_render` route always calls its upstream API and bypasses this
rendered-response cache. A nested `api_render` executes when its parent renders;
reusing the parent response also reuses that rendered API content. See
[API cache ownership](API_RENDER.md#cache-ownership).

Remember the mode and override rules:

| Setting | Effect |
| --- | --- |
| `mode: development` or `mode: debug` | Every request renders fresh; route cache configuration is still validated. |
| `mode: live` | Eligible routes can use memory or disk caching. |
| Package `live.cache: 0s` | Disables internal output caching globally, including routes with positive lifetimes. |
| Route `nocache: true` or `cache: 0s` | Disables internal output caching for that route. |

The Boolean `cache: true` on an `esbuild` component controls asset builds. Page
and fragment response caching uses durations or the storage/expiry mapping
shown here.

## 7. Take the Configuration to Deployment

HyperBricks automatically excludes `.cache` directories from deployment
archives and runtime snapshots. Each destination process creates its own cache
files when requested. Git ignore rules and deployment exclusions are separate;
you do not need to add a deployment exclusion for the default cache directory.

If you choose a custom directory inside the module, declare it in the default
`package.hyperbricks.yaml` used by the build so it is excluded too. Merely
starting with an alternate package profile does not change the build's directory
settings. Keep cache files outside public static and rendered-output directories.
See [response-cache files and deployment](DEPLOY.md#response-cache-files-and-deployment)
for writable volumes, custom paths, and deployed purge commands.

For an upgrade, check the [route-cache migration note](MIGRATION.md#route-cache-durations-now-take-effect):
before v1.3.0-beta, a scalar route duration such as `cache: 30s` was accepted but
the package lifetime was used. That route duration now takes effect.

## Keep Exploring

The repository includes a [response-cache test module](../modules/response-cache-test/README.md)
with runnable examples for memory and disk, expiry, purge, runtime modes,
custom directories, and storage limits. Its automated checks use temporary
module copies; inspect files from a manually running module when you want to
watch its cache on disk.

Use your application's response sizes, rendering work, and freshness requirements
when deciding which backend and lifetime to deploy. Start with the observable
checks above: reuse within the lifetime, fresh output after expiry or purge,
and the expected storage behavior while the server is running.

For a symptom while following this guide, start here:

| What you see | First check |
| --- | --- |
| A new timestamp on every request | Live mode, a positive package lifetime, and removal of `nocache: true`. |
| No disk files | `storage: disk`, a successful request, and a server that is still running. |
| Files vanish after stopping | Expected: normal shutdown removes the runtime cache directory. |
| Updated data appears late | The route lifetime and whether you need to purge or render fresh. |
| No running instance found during purge | The selected module path and whether its live server is running. |

The [output-cache reference](LIVE_MODE_HTTP.md#output-cache) and
[Troubleshooting](TROUBLESHOOTING.md) cover the detailed behavior and diagnostics.
