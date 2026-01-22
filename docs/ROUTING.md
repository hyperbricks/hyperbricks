# Routing

This document explains how Hyperbricks resolves routes and how to configure
clean URLs (like `/help`) for `.html` content (like `help.html`).

The routing config lives in your module's `package.hyperbricks` under
`hyperbricks.server.routing`.

## Defaults (when missing)

If `server.routing` is missing, Hyperbricks uses these defaults:

- `clean_urls = true`
- `index_files = [ index.html, index.htm ]`
- `extensions = [ html, htm ]`

Defaults are applied even if you only set some fields. Empty lists are
replaced with defaults.

## What "clean URLs" means here

Clean URLs are implemented as internal rewrites, not redirects.
The browser URL stays the same:

- `/` can serve `index.html`
- `/help` can serve `help.html`

If you want canonical redirects (for example `/help.html` -> `/help`),
you should add them at a reverse proxy (Caddy, Nginx, Cloudflare, etc.).

## Resolution order (dynamic rendering)

When `hyperbricks start` serves a request, it resolves routes like this:

1) Root request `/`
   - `index` (if a route exists)
   - then any file in `index_files` (default: `index.html`, `index.htm`)
2) Exact match (route exists exactly as requested)
3) If `clean_urls = false`: stop here
4) If request has an allowed extension (for example `.html`), try the
   extension-less route
5) If request has no extension, try adding each extension from `extensions`

This keeps URLs clean while still letting you define explicit `.html` routes.

## Resolution order (static file server)

When serving static files (for example after `hyperbricks static`):

1) If the URL ends with `/`, try `index_files`
2) If `clean_urls = true` and the URL has no extension, try `extensions`
3) Otherwise serve the file as-is

Note: `/index.html` still redirects to `/` (Go's file server behavior), and
`/` serves the index file directly to avoid redirect loops.

## Configuration reference

Example block:

```
hyperbricks {
  server {
    routing {
      clean_urls = true
      index_files = [ index.html, index.htm ]
      extensions = [ html, htm ]
    }
  }
}
```

Fields:

- `clean_urls` (bool)
  - `true`: allow `/help` to resolve to `help.html`
  - `false`: only exact routes (except `/` which still uses `index_files`)
- `index_files` (list)
  - Files to try for `/` or `/path/` requests
  - You can include values like `index.html` or `index.htm`
- `extensions` (list)
  - Extensions to try for clean URLs
  - Use `html`, `htm`, etc (leading dots are accepted but trimmed)

## Example configurations

### 1) Default behavior (clean URLs on)

```
hyperbricks {
  server {
    routing {
      clean_urls = true
      index_files = [ index.html, index.htm ]
      extensions = [ html, htm ]
    }
  }
}
```

Routes resolve like this:

```
/            -> index.html (or index.htm)
/help        -> help.html (or help.htm)
/help.html   -> help.html (exact match), or help (if only help exists)
```

### 2) Strict routing (clean URLs off)

```
hyperbricks {
  server {
    routing {
      clean_urls = false
    }
  }
}
```

Routes resolve like this:

```
/            -> index.html (or index.htm)
/help        -> only if "help" exists
/help.html   -> only if "help.html" exists
```

### 3) Custom index files

```
hyperbricks {
  server {
    routing {
      index_files = [ home.html, index.html ]
    }
  }
}
```

Routes resolve like this:

```
/            -> home.html (if it exists), else index.html
```

### 4) Custom extensions

```
hyperbricks {
  server {
    routing {
      extensions = [ html, xhtml ]
    }
  }
}
```

Routes resolve like this:

```
/help        -> help.html, or help.xhtml
```

## Practical examples

### Example A: `index.html` as `/`

```
page = <HYPERMEDIA>
page.route = index.html
```

Requests:

```
/      -> index.html
/index -> index.html
```

### Example B: `help.html` as `/help`

```
page = <HYPERMEDIA>
page.route = help.html
```

Requests:

```
/help      -> help.html
/help.html -> help.html
```

### Example C: `help` route with `.html` access

```
page = <HYPERMEDIA>
page.route = help
```

Requests:

```
/help      -> help
/help.html -> help
```

## Notes

- Route resolution is internal and does not change the browser URL.
- If you want redirects (canonical URLs), add them at your reverse proxy.
- Index routing for `/` is always active, even when `clean_urls = false`.
