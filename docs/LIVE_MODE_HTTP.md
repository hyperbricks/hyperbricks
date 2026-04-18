# Live Mode HTTP Settings

This note explains a small but important part of HyperBricks runtime behavior: the HTTP server settings used in `live` mode.

It is written for newcomers who want the practical picture first.

## Two different concerns

In HyperBricks, these are different things:

- `live.cache` controls how long rendered page output may stay cached.
- `server.read_timeout`, `server.write_timeout`, `server.idle_timeout`, and `server.keep_alives_enabled` control how the Go HTTP server handles network connections.

That distinction matters because caching and connection handling solve different problems.

## What the settings mean

### `read_timeout`

How long the server allows itself to read the incoming request.

This protects the server from clients that send data very slowly.

### `write_timeout`

How long the server allows itself to send the response.

This helps avoid a response hanging forever on a slow or broken connection.

### `idle_timeout`

How long an already-open keep-alive connection may sit idle before the server closes it.

### Keep-alives

Keep-alive means one TCP connection can be reused for multiple HTTP requests.

That is usually a good default for real traffic, because it reduces connection setup work and plays nicely with browsers, proxies, and load balancers.

In HyperBricks config, that policy is controlled with:

```hyperbricks
server {
    keep_alives_enabled = true
}
```

## How live mode works now

`live` mode now uses the configured server transport settings instead of silently replacing them with a separate hard-coded profile.

- `server.read_timeout`
- `server.write_timeout`
- `server.idle_timeout`
- `server.keep_alives_enabled`

That means the values in `package.hyperbricks` are the values the live server actually uses.

## Where `nocache` must be set

If you want live mode to skip cache headers and skip storing the response in the live cache, set `nocache = true` on the routed root object itself.

That usually means the top-level `<HYPERMEDIA>` or `<FRAGMENT>` that owns the route.

Setting `nocache` only inside a nested template or child component is not enough, because the live cache decision is made from the root route config.

Example:

```hyperbricks
page = <HYPERMEDIA>
page.route = cacheTest
page.nocache = true
```

```hyperbricks
fragment = <FRAGMENT>
fragment.route = cacheTest
fragment.nocache = true
```

## Defaults when omitted

If you do not set these values in `package.hyperbricks`, HyperBricks uses these runtime defaults:

- `read_timeout = 5s`
- `write_timeout = 10s`
- `idle_timeout = 20s`
- `keep_alives_enabled = true`

This applies even when the `server` block does not explicitly list them.

## The practical scaling lesson

When people first think about "scaling", they often think only about handling many requests at once.

In practice, scaling also means controlling how long slow, stuck, or idle clients are allowed to occupy server resources.

Timeouts and keep-alive policy are part of that.

## Default guidance

For most internet-facing deployments, these are sensible defaults:

- keep `keep_alives_enabled = true`
- use finite read, write, and idle timeouts
- treat `keep_alives_enabled = false` as a special-case setting, not the normal production default

If you later want benchmark-style one-request-per-connection behavior, make that an explicit choice in config.

## Example config

```hyperbricks
hyperbricks {
    mode = live

    server {
        read_timeout = 5s
        write_timeout = 10s
        idle_timeout = 20s
        keep_alives_enabled = true
    }
}
```

## Starter profiles

These are not strict rules. They are practical starting points for common use cases.

### 1. Small public site

Use this for a brochure site, simple blog, or small HTMX app with normal page traffic.

```hyperbricks
hyperbricks {
    mode = live

    live {
        cache = 30s
    }

    server {
        read_timeout = 5s
        write_timeout = 10s
        idle_timeout = 20s
        keep_alives_enabled = true
    }
}
```

### 2. Balanced production app

Use this when the site is public, sits behind a reverse proxy, and serves a steady mix of full pages and fragments.

```hyperbricks
hyperbricks {
    mode = live

    live {
        cache = 15s
    }

    server {
        read_timeout = 10s
        write_timeout = 15s
        idle_timeout = 30s
        keep_alives_enabled = true
    }
}
```

### 3. Heavy pages or slower clients

Use this when pages are larger, some clients are slower, or the app serves more expensive responses and needs slightly looser network deadlines.

```hyperbricks
hyperbricks {
    mode = live

    live {
        cache = 10s
    }

    server {
        read_timeout = 15s
        write_timeout = 30s
        idle_timeout = 60s
        keep_alives_enabled = true
    }
}
```

## When to disable keep-alives

Set `keep_alives_enabled = false` only when you explicitly want that behavior and understand the tradeoff.

Example:

```hyperbricks
hyperbricks {
    mode = live

    server {
        read_timeout = 5s
        write_timeout = 10s
        idle_timeout = 20s
        keep_alives_enabled = false
    }
}
```

That usually means more connection churn and less efficient normal browser or proxy traffic.

## Beginner takeaway

`live.cache` is about rendered output reuse.

`server.*` is about connection safety and behavior.

In live mode, HyperBricks now uses the server settings you configure, so those values are worth choosing deliberately.
