# API Render

HyperBricks can fetch API data and render it into HTML through Go templates.
There are two API-oriented components:

- `api_render` fetches API data inside another route and renders it as part of a
  page or fragment.
- `api_fragment_render` owns its own route and returns a dynamic fragment.

Use `api_render` for public, cacheable, nested content. Use
`api_fragment_render` for interactive, request-specific, authenticated, or HTMX
fragment flows.

## At A Glance

| YAML type | Runtime type | Route owner | Cache behavior | Typical use |
| --- | --- | --- | --- | --- |
| `api_render` | `<API_RENDER>` | no | cacheable by parent route | public feeds, public widgets, read-only API content |
| `api_fragment_render` | `<API_FRAGMENT_RENDER>` | yes | always dynamic | forms, authenticated fragments, HTMX islands |

`api_fragment_render` is forced to `nocache` at runtime.

## API Render

`api_render` is a nested component. It must live inside a route owner such as
`hypermedia` or `fragment`.

```yaml
page:
  - type: hypermedia
  - route: public-feed
  - title: Public feed
  - main:
      - type: tree
      - feed:
          - type: api_render
          - endpoint: https://api.example.test/articles
          - method: GET
          - querykeys:
              - category
          - template:
              file: api/articles.html
          - values:
              heading: Latest articles
```

The template receives the parsed upstream response as `.Data`, the upstream HTTP
status as `.Status`, and `values` merged into the template root.

```html
<section>
  <h2>{{.heading}}</h2>
  {{range .Data.items}}
    <article>
      <h3>{{.title}}</h3>
      <p>{{.summary}}</p>
    </article>
  {{end}}
</section>
```

### Static Snapshots

`api_render` works with `hyperbricks static` because static rendering now starts
an internal localhost runtime and requests routes over HTTP. This means nested
`api_render` blocks receive normal request context and their rendered HTML is
written into the static output file.

Use this for public or cacheable API-backed pages, for example a product list,
blog feed, documentation index, or catalog page. The upstream API must be
reachable when `hyperbricks static` runs. A non-2xx upstream response from
`api_render` is treated as a render error, so static snapshot builds fail
instead of freezing a broken API result into HTML.

Explicit targets in `package.hyperbricks.yaml` win over automatic route
discovery:

```yaml
hyperbricks:
  static:
    routes:
      - path: /products
        output: products.html
```

Configured variants can snapshot the same route with different queries:

```yaml
hyperbricks:
  static:
    variants:
      - path: /products
        query:
          category: shoes
        output: products/shoes.html
```

See `modules/sampleapis-coffee-static` for a runnable module that fetches the
SampleAPIs Coffee endpoint with `api_render` and freezes the result into
`rendered/index.html`.

## API Fragment Render

`api_fragment_render` owns a route. It receives the browser request, optionally
maps query/form/body data to an upstream API request, renders the upstream
response, and returns fragment HTML.

```yaml
profile_fragment:
  - type: api_fragment_render
  - route: fragments/profile
  - endpoint: http://127.0.0.1:9000/api/profile
  - method: GET
  - querykeys:
      - user_id
  - response:
      headers:
        HX-Retarget: "#profile"
        HX-Reswap: outerHTML
  - template:
      file: fragments/profile.html
```

`response.headers` contains literal HTTP headers returned to the browser.
HTMX uses the `HX-*` headers in this example; HyperBricks does not add or
interpret them. Any valid HTTP header name can be configured here.

`response.status` optionally sets the browser HTTP status, which defaults to
`200`. It is independent of the upstream `.Status` available to the template.
For example, an upstream `409` may render feedback inside a browser response
with status `200`. A fixed `response.headers.HX-Trigger` is sent for every
rendered response, so it does not indicate whether the upstream write succeeded.

Top-level `headers` still configures the **upstream request**. It is separate
from `response.headers`, which configures the **browser response**. See
[HTTP responses](HTTP_RESPONSES.md) for header precedence, status behavior, and
the migration from `response.hx_*` fields.

A typical HTMX flow is:

1. The browser triggers an `hx-get` or `hx-post` request.
2. HyperBricks resolves the `api_fragment_render` route.
3. If a `guard` is configured, it runs before any upstream API call.
4. HyperBricks forwards the allowed request data to the upstream API.
5. The upstream response is rendered through `inline` or `template`.
6. HyperBricks returns fragment HTML plus any configured HTMX response headers.

If the rendered body contains `hx-swap-oob` elements, HTMX applies those
out-of-band swaps after the normal target swap.

## Request Mapping

Both API components support the same core API request fields. `api_render` uses
them while rendering inside an owning route; `api_fragment_render` uses them for
its own route.

| Field | Purpose |
| --- | --- |
| `endpoint` | Upstream API URL |
| `method` | HTTP method |
| `headers` | Extra upstream request headers |
| `body` | Raw upstream request body with `$key` placeholders |
| `querykeys` | Allow-list of incoming query keys to forward |
| `queryparams` | Static query parameters to append |
| `username` / `password` | Basic authentication |
| `jwtsecret` / `jwtclaims` | Generate a bearer token for upstream authentication |
| `template` | Template file resolver or template name |
| `inline` | Inline Go template source |
| `values` | Extra template data |
| `debug` | Add debug output in development flows |
| `debugpanel` | Enable the frontend error panel when configured globally |

Incoming data is merged before placeholders are applied:

| Source | Behavior |
| --- | --- |
| Query params | Only keys listed in `querykeys` are forwarded. If `querykeys` is omitted, HyperBricks uses the default allow-list `id`, `name`, and `order`. An explicit empty list forwards none. |
| Form data | Single-value fields become strings. Multi-value fields remain lists. |
| JSON body | Object keys merge into the request data. If a JSON key collides with an existing key, the JSON value is also available as `body_<key>`. |
| `queryparams` | Static values are appended to the outgoing upstream query. |

`body` placeholders use `$key` names resolved from that merged request data.
Prefer simple placeholder names such as `$id`, `$name`, or `$email`; they are
matched as word-like tokens.

```yaml
login:
  - type: api_fragment_render
  - route: auth/login
  - endpoint: http://127.0.0.1:9000/rpc/login
  - method: POST
  - headers:
      Content-Type: application/json
  - body: |
      {"email":"$email","password":"$password"}
  - response:
      headers:
        HX-Trigger: login-updated
  - inline: |
      <div id="login-result">{{.Data.message}}</div>
```

Template context contains:

| Key | Meaning |
| --- | --- |
| `.Data` | Parsed upstream response. JSON becomes maps/lists; plain text remains string-like data. |
| `.Status` | Upstream HTTP status code. |
| Values from `values` | Extra values merged into the template root, for example `{{.heading}}`. |

## Authentication

Upstream authorization is applied in this order:

1. When `jwtsecret` is set, HyperBricks signs `jwtclaims` and sends a bearer
   token.
2. Otherwise, when the incoming request has a `token` cookie, HyperBricks sends
   it as a bearer token.
3. Otherwise, when `username` and `password` are set, HyperBricks uses Basic
   Auth.
4. Otherwise, no upstream auth header is added.

`jwtclaims.exp` is treated as a seconds offset from now.

## Cookies

`api_fragment_render` can set response cookies when the upstream API returns a
successful `2xx` response.

```yaml
logout:
  - type: api_fragment_render
  - route: auth/logout
  - endpoint: http://127.0.0.1:9000/rpc/logout
  - method: POST
  - setcookies:
      - "token=; Path=/; HttpOnly; Max-Age=0; SameSite=Lax"
      - "session=; Path=/; HttpOnly; Max-Age=0; SameSite=Lax"
  - inline: |
      <div id="auth-status">Signed out</div>
```

Use `setcookies` when one route must emit multiple `Set-Cookie` headers.
`setcookie` is still accepted as a shorthand for one cookie template.

## Guards

`api_fragment_render` can declare a `guard` block. The guard runs before the
upstream API call. A denied request never reaches the upstream endpoint.

```yaml
secure_profile:
  - type: api_fragment_render
  - route: fragments/secure-profile
  - guard:
      enabled: true
      auth:
        cookie: token
      require:
        authenticated: true
      authorize:
        endpoint: http://127.0.0.1:9000/auth/authorize
        method: POST
  - endpoint: http://127.0.0.1:9000/api/profile
  - method: GET
  - template:
      file: fragments/profile.html
```

See [Route Guard](ROUTE_GUARD.md) for the full guard contract.

## Security Notes

- Keep `querykeys` narrow. Do not forward arbitrary browser query parameters to
  upstream APIs.
- Use loopback or private upstream URLs for internal services.
- Prefer `HttpOnly`, `Secure`, `SameSite`, and `Path` on cookies.
- Do not reflect untrusted input into headers or cookies without validation.
- Keep debug output disabled in live deployments.

For all available component fields, see [Reference](REFERENCE.md).
