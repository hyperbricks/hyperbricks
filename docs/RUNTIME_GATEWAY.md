# Runtime Gateway

Use the runtime gateway to send requests for configured hosts to separate runtime views. Before normal route rendering, HyperBricks asks a trusted resolver where to forward the request.

HyperBricks checks whether the host matches a gateway domain or host suffix. The resolver interprets the host, checks access, and selects the target.

## Use Cases

The gateway is useful when an integration wants normal browser URLs for isolated runtime views:

```text
project-a.runtime.example.test
project-a.live.example.test
project-a--build-123.runtime.example.test
feature-x.staging.example.test
project-a-runtime.example.test
```

## CLI Configuration

Single dotted domain:

```bash
hyperbricks start -m my-module --port 8080 \
  --runtime-gateway \
  --runtime-domain runtime.local \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

Multiple dotted domains:

```bash
hyperbricks start -m my-module --port 8080 \
  --runtime-gateway \
  --runtime-domain live.local,runtime.local \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

Flat host suffixes:

```bash
hyperbricks start -m my-module --port 8080 \
  --runtime-gateway \
  --runtime-host-suffix -runtime.example.test,-live.example.test \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

## Package Configuration

You can also configure the gateway in `package.hyperbricks.yaml`:

```yaml
hyperbricks:
  server:
    runtime_gateway:
      enabled: true
      domain: runtime.local
      domains:
        - live.local
        - runtime.local
      host_suffix: -runtime.example.test
      host_suffixes:
        - -live.example.test
        - -staging.example.test
      resolver: http://127.0.0.1:8080/resolve-runtime
```

`domain` is the single-domain setting.

`domains` adds multiple dotted gateway domains.

`host_suffix` is the single flat-host suffix setting.

`host_suffixes` adds multiple flat-host suffixes.

Comma-separated values are accepted by the CLI and package configuration.

## Matching Rules

Given `runtime.local`, HyperBricks matches subhosts below that domain:

```text
site.runtime.local              matches
site--current.runtime.local     matches
runtime.local                   does not match
control.local                   does not match
site.other.local                does not match
```

HyperBricks does not interpret the left-hand host label. Your resolver can use it for project names, build IDs, variants, or another convention.

Given `-runtime.example.test`, HyperBricks matches flat hosts ending in that suffix:

```text
site-runtime.example.test       matches
b-123-runtime.example.test      matches
runtime.example.test            does not match
site.runtime.example.test       does not match
site-other.example.test         does not match
```

Use `domain` or `domains` for dotted subhosts such as `site.runtime.local`. Use `host_suffix` or `host_suffixes` for flat hosts such as `site-runtime.example.test`.

## Resolver Request

For a matching request, HyperBricks sends JSON to the configured resolver:

```json
{
  "host": "site.runtime.local",
  "method": "GET",
  "path": "/about",
  "raw_query": "tab=runtime"
}
```

## Resolver Allow Response

The resolver returns a private target:

```json
{
  "allowed": true,
  "target": "http://127.0.0.1:18200",
  "project": "site",
  "variant": "current",
  "cache_ttl_seconds": 5
}
```

The target must be a loopback or private network address. HyperBricks rejects public targets before proxying.

## Resolver Deny Response

When a request is not allowed, the resolver can deny it:

```json
{
  "allowed": false,
  "status": 403,
  "message": "forbidden"
}
```

## Cookies And Handoff Tokens

Resolvers may return `set_cookies` values. HyperBricks forwards those cookies to the browser.

If the resolver returns a non-empty `set_cookies` list and the original URL contains `runtime_token`, HyperBricks forwards those cookies and redirects once to the same URL without the token.

That supports this flow:

1. An authenticated application creates a short-lived handoff token.
2. The application redirects the browser to the runtime host with that token.
3. The resolver validates the token and returns scoped cookies.
4. HyperBricks removes the token from the URL.

## Security Notes

- Treat the resolver as the policy boundary.
- Configure only domains that are dedicated to gateway traffic.
- Do not put normal app hosts under a gateway domain unless the resolver should own those hosts.
- Authorize every request in the resolver using cookies, headers, or short-lived handoff tokens.
- Return only loopback or private network targets.
