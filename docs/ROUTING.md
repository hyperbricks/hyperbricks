# Routing

This document explains how HyperBricks resolves routes and clean URLs for
dynamic rendering and static output.

Route guard behavior is documented in [ROUTE_GUARD.md](ROUTE_GUARD.md).
Configure browser status and headers with the route's `response` block; see
[HTTP responses](HTTP_RESPONSES.md).

## Route Owners

Routes are owned by root components:

- `hypermedia`
- `fragment`
- `api_fragment_render`

Example page route:

```yaml
page:
  - type: hypermedia
  - route: help
  - title: Help
  - main:
      - type: tree
      - content:
          - type: html
          - value: <h1>Help</h1>
```

Example fragment route:

```yaml
status:
  - type: fragment
  - route: fragments/status
  - body:
      - type: html
      - value: <div id="status">Ready</div>
```

## Routing Configuration

Routing config lives in `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  server:
    routing:
      clean_urls: true
      index_files:
        - index.html
        - index.htm
      extensions:
        - html
        - htm
```

Defaults are used when routing config is omitted:

| Field | Default |
| --- | --- |
| `clean_urls` | `true` |
| `index_files` | `index.html`, `index.htm` |
| `extensions` | `html`, `htm` |

Defaults are also applied for empty list values.

## Clean URLs

Clean URLs are internal rewrites, not redirects. The browser URL stays the same.

Examples:

```text
/       can resolve to index or index.html
/help   can resolve to help or help.html
```

If you want canonical redirects such as `/help.html` to `/help`, add them at a
reverse proxy such as Caddy, Nginx, or Cloudflare.

## Dynamic Route Resolution

When `hyperbricks start` serves a request, it resolves routes in this order:

1. For `/`, try the route `index`.
2. For `/`, try each configured `index_files` value.
3. Try an exact route match.
4. If `clean_urls` is `false`, stop here.
5. If the request has an allowed extension, try the extension-less route.
6. If the request has no extension, try appending each allowed extension.

This allows clean browser URLs while still supporting explicit `.html` routes.

## Static File Resolution

When serving static files, for example after `hyperbricks static`:

1. If the URL ends with `/`, try configured `index_files`.
2. If `clean_urls` is `true` and the URL has no extension, try configured
   `extensions`.
3. Otherwise serve the file as requested.

Go's static file server can still redirect `/index.html` to `/`. HyperBricks
serves `/` directly to avoid redirect loops.

## Practical Examples

### `index` As `/`

```yaml
page:
  - type: hypermedia
  - route: index
```

Requests:

```text
/       -> index
/index  -> index
```

### `index.html` As `/`

```yaml
page:
  - type: hypermedia
  - route: index.html
```

Requests:

```text
/             -> index.html
/index        -> index.html
/index.html   -> index.html
```

### `help.html` As `/help`

```yaml
page:
  - type: hypermedia
  - route: help.html
```

Requests:

```text
/help        -> help.html
/help.html   -> help.html
```

### Strict Routing

```yaml
hyperbricks:
  server:
    routing:
      clean_urls: false
```

Routes resolve like this:

```text
/             -> index or configured index file
/help         -> only if route help exists
/help.html    -> only if route help.html exists
```

## Live Mode Cache Headers

In live mode, cacheable dynamic route responses include cache metadata headers:

```text
X-Hyperbricks-Rendered-At
X-Hyperbricks-Cache-Expires-At
```

These are response metadata. They do not affect route matching. Development mode
does not add these headers.

## Notes

- Route resolution is internal and does not change the browser URL.
- Index routing for `/` is always active, even when `clean_urls` is `false`.
- Guarded routes are resolved first, then guard evaluation runs before render.
