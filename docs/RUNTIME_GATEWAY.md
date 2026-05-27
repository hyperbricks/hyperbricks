# Runtime Gateway

The HyperBricks runtime gateway is a generic host-based proxy hook. It lets a
HyperBricks server intercept configured subdomains before normal route rendering
and ask a trusted resolver where the request should go.

## Use Cases

The gateway is useful when an integration wants normal browser URLs for isolated
runtime views, for example:

```text
project-a.runtime.example.test
project-a.live.example.test
project-a--build-123.runtime.example.test
feature-x.staging.example.test
```

HyperBricks only decides whether the request is under a configured gateway
domain. The resolver decides what the host means.

## Configuration

Single domain:

```bash
hyperbricks start -m my-module --port 8080 \
  --runtime-gateway \
  --runtime-domain runtime.local \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

Multiple domains:

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
  --runtime-host-suffix -runtime.hyperbricks.eu,-live.hyperbricks.eu \
  --runtime-resolver http://127.0.0.1:8080/resolve-runtime
```

Equivalent package configuration:

```hyperbricks
hyperbricks {
  server {
    runtime_gateway {
      enabled = true
      domain = runtime.local
      domains = live.local,runtime.local
      host_suffix = -runtime.hyperbricks.eu
      host_suffixes = -live.hyperbricks.eu,-staging.hyperbricks.eu
      resolver = http://127.0.0.1:8080/resolve-runtime
    }
  }
}
```

`domain` is the original single-domain setting and remains supported.
`domains` is an additional list for integrations that need more than one gateway
suffix. Comma-separated values are accepted.
`host_suffix` and `host_suffixes` support flat host names where the runtime key
is part of the left-hand label instead of a dotted subdomain.

## Matching Rules

Given a configured domain `runtime.local`, HyperBricks matches subhosts below
that domain:

```text
site.runtime.local              matches
site--current.runtime.local     matches
runtime.local                   does not match
control.local                   does not match
site.other.local                does not match
```

The gateway does not require `--` in the left-hand host label:
`site.live.local` and `site--build.runtime.local` can both be valid if the
resolver accepts them.

Given a configured host suffix `-runtime.hyperbricks.eu`, HyperBricks matches
flat hosts ending in that suffix:

```text
site-runtime.hyperbricks.eu       matches
b-123-runtime.hyperbricks.eu      matches
runtime.hyperbricks.eu            does not match
site.runtime.hyperbricks.eu       does not match
site-other.hyperbricks.eu         does not match
```

Use `domain` / `domains` for dotted subhosts such as `site.runtime.local`. Use
`host_suffix` / `host_suffixes` for flat hosts such as
`site-runtime.hyperbricks.eu`.

## Resolver Contract

For a matching request, HyperBricks sends a JSON request to the configured
resolver:

```json
{
  "host": "site.runtime.local",
  "method": "GET",
  "path": "/about",
  "raw_query": "tab=runtime"
}
```

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

The target must be loopback or private network address. Public targets are
rejected by HyperBricks before proxying.

If the resolver rejects the request, it should return:

```json
{
  "allowed": false,
  "status": 403,
  "message": "forbidden"
}
```

## Cookies And Handoff Tokens

Resolvers may return `set_cookies` values. HyperBricks forwards them to the
browser. If the original URL contained `runtime_token`, HyperBricks sets the
cookies and redirects once to the same URL without the token.

This supports a common flow:

1. An authenticated application creates a short-lived token.
2. The browser is redirected to the virtual runtime host with that token.
3. The resolver validates the token and returns a scoped cookie.
4. HyperBricks removes the token from the URL.

## Security Notes

- Treat the resolver as the policy boundary.
- Only configure domains that are dedicated to gateway traffic.
- Do not put normal app hosts under a gateway domain unless the resolver is
  supposed to own them.
- The resolver should authorize every request using cookies, headers, or
  short-lived handoff tokens.
- HyperBricks rejects resolver targets that are not loopback or private network
  addresses.
