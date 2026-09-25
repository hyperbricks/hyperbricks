# HTTP Responses

Set `response` on a `hypermedia`, `fragment`, or `api_fragment_render` route to choose the browser response status and headers. These settings work with any browser JavaScript library:

```yaml
status_fragment:
  - type: fragment
  - route: fragments/status
  - response:
      status: 200
      headers:
        Cache-Control: no-store
        X-App-View: status
  - content:
      - type: html
      - value: '<section id="status">Ready</section>'
```

`response.status` defaults to `200` for a rendered route. If you set it, use a value between `200` and `599`.

`response.headers` maps valid header names to literal strings. Quote values such as `"true"` and `"60"` so YAML does not read them as booleans or numbers. Header names are case-insensitive. HyperBricks rejects duplicate names with different capitalization and invalid names or values.

HyperBricks renders the HTML and applies the configured HTTP response. The application chooses any library-specific headers; the browser library interprets them. For example, a project using HTMX can configure `HX-Redirect`, while a normal browser redirect uses a `3xx` status and `Location`. HyperBricks does not automatically convert one into the other.

## Browser And Upstream Responses

| Setting | Destination |
| --- | --- |
| Any route owner's `response.status` | HTTP status returned to the browser. |
| Any route owner's `response.headers` | Headers returned to the browser. |
| `hypermedia.headers` | Existing browser headers; remains supported. |
| `api_render.headers` or `api_fragment_render.headers` | Headers sent to the upstream API. |
| `guard.authorize.headers` | Headers sent to the authorization endpoint. |

In an API template, `.Status` remains the upstream API status. It is independent of `response.status`. A configured response header does not establish that an upstream write succeeded. For example, in an HTMX integration, a static `HX-Trigger` header is sent whenever the route produces its response. Let the template or application logic inspect the upstream result before triggering a success-only refresh. See [API Render](API_RENDER.md).

## Ownership And Precedence

Only the root route owner supplies configured browser response metadata. Embedding a fragment in a full page reuses its rendered content; that fragment's `response` block does not change the page's status or headers. This applies to both rendering paths.

For ordinary rendered responses:

- `response.headers` overrides the same header in `hypermedia.headers`.
- An explicit route `content_type` takes precedence over a configured `Content-Type` header. Without either, the runtime supplies its normal type.
- Existing cookie fields remain supported. Use `setcookies` on an API fragment to send multiple cookies, and keep its upstream-success condition in mind.
- A plugin that explicitly handles the HTTP response retains ownership of its status and headers; route response defaults do not overwrite it.

HyperBricks caches the configured headers and status with the rendered content. Cache hits return the same metadata. A configured `Vary` adds those request headers to the internal cache key. `Vary: "*"` bypasses the cache.

Use `nocache: true` to disable the internal route cache. Setting `Cache-Control` alone does not replace it. Routes with an enabled guard and API fragment routes bypass the internal cache.

Guard responses use `Cache-Control: no-store`. Their `Vary` includes configured authentication inputs and request-header selectors. HyperBricks adds `HX-Request` variation only when configuration uses that header.

## Migrate Existing Configuration

This is a breaking configuration migration. Removed fields are rejected rather than silently translated or ignored. Migrate reusable base definitions as well as the routes that inherit them. Browser markup such as `hx-get`, `hx-target`, and `hx-swap` continues to belong to the application; it is unchanged.

### HTMX Example: Fragment And API Fragment Headers

This example keeps the HTMX integration while replacing the removed HTMX-specific fields with general HTTP headers. The resulting example uses [HTMX 4](https://four.htmx.org/).

Before:

```yaml
status:
  - type: fragment
  - route: fragments/status
  - response:
      hx_retarget: "#status"
      hx_reswap: outerHTML
      hx_trigger: statusUpdated
  - content:
      - type: html
      - value: '<section id="status">Ready</section>'
```

After:

```yaml
status:
  - type: fragment
  - route: fragments/status
  - response:
      headers:
        HX-Retarget: "#status"
        HX-Reswap: outerHTML
        HX-Trigger: statusUpdated
  - content:
      - type: html
      - value: '<section id="status">Ready</section>'
```

The old fields map to the following literal response headers. HyperBricks accepts arbitrary valid header names; check which headers your browser library supports:

| Removed field under `response` | Key under `response.headers` |
| --- | --- |
| `hx_location` | `HX-Location` |
| `hx_push_url` | `HX-Push-Url` |
| `hx_redirect` | `HX-Redirect` |
| `hx_refresh` | `HX-Refresh` |
| `hx_replace_url` | `HX-Replace-Url` |
| `hx_reswap` | `HX-Reswap` |
| `hx_retarget` | `HX-Retarget` |
| `hx_reselect` | `HX-Reselect` |
| `hx_trigger` | `HX-Trigger` |
| `hx_trigger_after_settle` | `HX-Trigger-After-Settle` (removed in HTMX 4) |
| `hx_trigger_after_swap` | `HX-Trigger-After-Swap` (removed in HTMX 4) |

HTMX 4 removed `HX-Trigger-After-Swap` and `HX-Trigger-After-Settle`. Use `response.headers.HX-Trigger` for application events. If an action needs the updated DOM, register an explicit `htmx:after:swap` browser listener; use `htmx:after:settle` when it must wait for settling. `HX-Trigger` does not preserve the old headers' timing. The generic HTTP core can still emit those header names for clients that support them. See the [HTMX 4 upgrade guide](https://raw.githubusercontent.com/bigskysoftware/htmx/v4.0.0/dist/skills/htmx-upgrade-from-htmx2.md).

Some older examples used `hx_target`, which was not the renderer's actual retarget field. Correct those examples to `response.headers.HX-Retarget` too.

Keep API request headers at the top-level `headers` field. Moving them under `response.headers` would send credentials or request metadata to the browser instead of the API.

### HTMX Example: Guard Denials

The guard checks access on the server, independently of the browser library. This example configures a normal browser redirect by default and an HTMX response when the request contains `HX-Request: "true"`.

Before:

```yaml
guard:
  enabled: true
  auth:
    cookie: token
  require:
    authenticated: true
  on_unauthenticated:
    redirect: /login
    hx_redirect: /login
    status: 401
```

After:

```yaml
guard:
  enabled: true
  auth:
    cookie: token
  require:
    authenticated: true
  on_unauthenticated:
    default:
      status: 303
      headers:
        Location: /login
    variants:
      - when:
          request_headers:
            HX-Request: "true"
        response:
          status: 401
          headers:
            HX-Redirect: /login
```

Apply the same shape to `on_forbidden`, typically with `403` in its HTMX variant and `/forbidden` as the destination. Keep `auth`, `require`, and `authorize` settings intact. A plain denial can use only `default: {status: 401}` or `default: {status: 403}` without a redirect.

A variant matches only when every header in `when.request_headers` matches. Names are case-insensitive; values require an exact, case-sensitive match. Missing headers do not match.

The first matching variant replaces the whole default response. Without a match, HyperBricks uses the default. For example, the HTMX variant above has no `Location` header. Selection happens after access is denied and cannot grant access. See [Route guards](ROUTE_GUARD.md) for the full rules.

## Another Browser Client

The [Unpoly fragment example](../modules/hyperbricks-patterns-yaml/docs/pages/unpoly-fragment-demo.md) reuses one template in a full page and a fragment. Its page loads Unpoly explicitly and uses `up-follow` and `up-target` to replace that fragment. The route uses the same HTTP response contract, without an HTMX browser dependency or a core-specific adapter.
