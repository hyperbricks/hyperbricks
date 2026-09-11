# API Render

HyperBricks can fetch API data and render it into HTML through Go templates. There are two API-oriented components:

- `api_render` fetches API data inside another route and renders it as part of a page or fragment.
- `api_fragment_render` owns its own route and returns a dynamic fragment.

Use `api_render` for read-only API content nested in a page or fragment; that parent route owns the rendered-output cache policy. Use `api_fragment_render` for interactive, request-specific, or authenticated fragment responses.

## At A Glance

| YAML type | Runtime type | Route owner | Cache behavior | Typical use |
| --- | --- | --- | --- | --- |
| `api_render` | `<API_RENDER>` | no | no API-response cache; rendered HTML follows the parent route policy | public feeds, public widgets, read-only API content |
| `api_fragment_render` | `<API_FRAGMENT_RENDER>` | yes | always renders and calls its upstream API | forms, authenticated fragments, API-backed page sections |

`api_fragment_render` is forced to `nocache` at runtime; it does not need a configured `nocache` field.

### Cache Ownership

Neither API component caches upstream API responses. Whenever either component executes, it sends a new HTTP request to its configured endpoint. HTTP connection reuse is not response caching.

`api_render` does not own a route and has no `nocache` field. Its parent `hypermedia` or `fragment` route decides whether the complete rendered response may be reused. On a parent-route cache hit, HyperBricks skips the whole render tree, so the nested `api_render` does not execute and makes no API request. Put `nocache: true` on the parent route when every incoming route request must fetch current API data:

```yaml
products:
  - type: hypermedia
  - route: products
  - nocache: true
  - content:
      - type: api_render
      - endpoint: https://api.example.test/products
      - method: GET
      - template:
          file: api/products.html
```

Do not put `nocache` on the nested component; it is not an `api_render` option and does not propagate to its parent:

```yaml
# Unsupported: this does not change route or API caching.
- type: api_render
- nocache: true
```

`api_fragment_render` is different because it is itself a route-owning root component. HyperBricks always bypasses the internal rendered-output cache for that route, so every request executes the component and calls the upstream API. The forced route policy still does not create or configure an API-response cache.

`nocache` controls HyperBricks' internal rendered-route cache. `Cache-Control` controls browsers and HTTP intermediaries. Configure the route's response headers separately when clients must not store its HTML. Upstream caching performed by a proxy, CDN, or API service is outside both component contracts. See [Live-mode HTTP caching](LIVE_MODE_HTTP.md) and [HTTP responses](HTTP_RESPONSES.md).

## API Render

`api_render` is a nested component. It must live inside a route owner such as `hypermedia` or `fragment`.

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

The template receives the parsed upstream response as `.Data`, the upstream HTTP status as `.Status`, and `values` merged into the template root.

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

`api_render` works with `hyperbricks static` because static rendering now starts an internal localhost runtime and requests routes over HTTP. This means nested `api_render` blocks receive normal request context and their rendered HTML is written into the static output file.

Use this for public or cacheable API-backed pages, for example a product list, blog feed, documentation index, or catalog page. The upstream API must be reachable when `hyperbricks static` runs. A non-2xx upstream response from `api_render` is treated as a render error, so static snapshot builds fail instead of freezing a broken API result into HTML.

Explicit targets in `package.hyperbricks.yaml` win over automatic route discovery:

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

See `modules/sampleapis-coffee-static` for a runnable module that fetches the SampleAPIs Coffee endpoint with `api_render` and freezes the result into `rendered/index.html`.

## API Fragment Render

`api_fragment_render` owns a route. It receives the browser request, optionally maps query/form/body data to an upstream API request, renders the upstream response, and returns fragment HTML.

### Example With HTMX

This example configures response headers for [HTMX 4](https://four.htmx.org/). It assumes HTMX is loaded on the page and requests this route.

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

`response.headers` contains literal HTTP headers returned to the browser. HTMX uses the `HX-*` headers in this example; HyperBricks does not add or interpret them. Any valid HTTP header name can be configured here.

`response.status` optionally sets the browser HTTP status, which defaults to `200`. It is independent of the upstream `.Status` available to the template. For example, an upstream `409` may render feedback inside a browser response with status `200`. A fixed `response.headers.HX-Trigger` is sent for every rendered response, so it does not indicate whether the upstream write succeeded.

Top-level `headers` still configures the **upstream request**. It is separate from `response.headers`, which configures the **browser response**. See [HTTP responses](HTTP_RESPONSES.md) for header precedence, status behavior, and the migration from `response.hx_*` fields.

A typical HTMX flow is:

1. The browser triggers an `hx-get` or `hx-post` request.
2. HyperBricks resolves the `api_fragment_render` route.
3. If a `guard` is configured, it runs before any upstream API call.
4. HyperBricks forwards the allowed request data to the upstream API.
5. The upstream response is rendered through `inline` or `template`.
6. HyperBricks returns fragment HTML plus the configured response headers.
7. HTMX processes the response and updates the target in the page.

If the rendered body contains `hx-swap-oob` elements, HTMX applies those out-of-band swaps after the normal target swap.

## Request Mapping

Both API components support the same core API request fields. `api_render` uses them while rendering inside an owning route; `api_fragment_render` uses them for its own route.

| Field | Purpose |
| --- | --- |
| `endpoint` | Upstream API URL |
| `method` | HTTP method |
| `headers` | Extra upstream request headers; `headers.Authorization` selects explicit service authentication |
| `forwardtoken` | Exact incoming cookie name to use as Bearer; omitted or empty disables forwarding |
| `body` | Raw upstream request body with `$key` placeholders |
| `querykeys` | Incoming URL query keys to forward to the upstream URL; does not filter body placeholders |
| `queryparams` | Static upstream query parameters to append, including when a key already exists |
| `username` / `password` | Basic authentication when both values are non-empty |
| `jwtsecret` / `jwtclaims` | Generate a bearer token as the sole upstream authentication source |
| `template` | Template file resolver or template name |
| `inline` | Inline Go template source |
| `values` | Extra template data |
| `debug` | Log request/response metadata without header values, URL paths/queries, or payloads |
| `debugpanel` | Enable the frontend error panel when configured globally |

### Upstream URL query

`querykeys` controls automatic forwarding from the browser URL to the upstream
URL. Names are case-sensitive. Omitted `querykeys` uses `id`, `name`, and `order`;
`querykeys: []` forwards none. Repeated values of an allowed key are preserved.

The upstream URL combines these sources in order:

1. Query parameters already present in `endpoint`.
2. Browser query parameters allowed by `querykeys`.
3. Configured `queryparams`.

Values are appended, not overwritten. For example, an endpoint containing
`?id=service`, a browser request containing `?id=browser`, and
`queryparams: {id: configured}` produce three values:
`id=service&id=browser&id=configured`. The receiving API decides how to handle
repeated keys; avoid collisions when it expects one value.

`queryparams` does not supply values for `$key` body placeholders.

### Configured body placeholders

`body` is a configured string, separate from the outgoing URL query. On the
normal HTTP runtime path, placeholder values come from the following input:

| Source | Behavior |
| --- | --- |
| Browser URL query | All query keys are present in the parsed request data, including keys excluded from upstream URL forwarding by `querykeys`. |
| URL-encoded form body | Single-value fields become strings. Repeated values remain lists. If a form field also occurs in the URL query, the list contains form values first, followed by query values. |
| JSON object body | A key absent from the parsed request data is available as `$key`. On collision, the existing value remains `$key` and the JSON value uses `$body_key`. Avoid input names starting with `body_`, which can collide with these aliases. |

The configured `$key` placeholders select which values are inserted into the
body. `querykeys: []` does not block those substitutions. For example, a submitted
`email` field can supply `$email` while no browser query parameters are forwarded
to the upstream URL. Validate the submitted values in the API that performs the
operation.

The current components differ when the browser request has no body:

| Component | Bodyless browser request |
| --- | --- |
| `api_render` | Sends its configured upstream `body` unchanged, including literal `$key` placeholders. |
| `api_fragment_render` | Applies placeholders using the available parsed query data. |

This concerns the incoming browser body, not the configured upstream method or
body. A browser GET can invoke an upstream POST with a configured `body`.

Placeholder names use letters, digits, and underscores, such as `$id` or
`$body_name`. They do not support nested-property or array-index expressions.
Missing placeholders remain literal text; an empty string value inserts empty
text. Request bodies are attempted as JSON objects regardless of Content-Type;
invalid JSON and non-object JSON contribute no placeholder fields. This mapping
step is not JSON request validation.

Use simple, single-value fields in the JSON string positions shown below. The
current mapper is textual substitution, not a JSON serializer: repeated values,
arrays, objects, and null use Go's display formatting rather than JSON encoding.
It does not automatically produce a URL-encoded or multipart upstream form.

The following HTMX example forwards login data to an API and configures an `HX-Trigger` response header. The `login-updated` event signals that a response was rendered, not that authentication succeeded; inspect the upstream result before treating the login as successful.

This loopback HTTP example requires `development` or `debug` mode. Use an HTTPS
endpoint when deploying it in live mode. If preparing the request body fails,
the component returns an error without calling the upstream API.

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

Both API components require an explicit credential source. Omitted `forwardtoken`
means no browser cookie is forwarded, including the formerly implicit `token`
cookie. The incoming browser `Authorization` header is never copied automatically.

```yaml
profile:
  - type: api_fragment_render
  - route: fragments/profile
  - endpoint: https://accounts.example.test/me
  - method: GET
  - forwardtoken: account_session
  - inline: '<p>{{.Data.name}}</p>'
```

`forwardtoken: account_session` selects the incoming browser cookie with exactly
that name and sends its non-empty value as `Authorization: Bearer <value>`.
A missing or empty cookie produces no derived Authorization header. Multiple
cookies with that name are ambiguous and cause an error before the upstream call.
Use a [route guard](ROUTE_GUARD.md) when authentication is mandatory.

| Configuration | Behavior |
| --- | --- |
| `forwardtoken` omitted or `""` | Browser credential forwarding disabled |
| `forwardtoken: account_session` | Select exactly that incoming cookie |
| Boolean, number, list, object, or null | Component configuration error |
| Invalid or whitespace-only cookie name | Component configuration error |

The field is a string, with no Boolean compatibility form. The API components
check its original value before weak decoding; YAML coercion for other
components is unchanged. Use omission to disable it.

Choose at most one authentication source per API component:

- `forwardtoken`: a named browser cookie;
- `headers.Authorization`: an explicit service credential;
- `username` **and** `password`: Basic Auth;
- `jwtsecret`, optionally with `jwtclaims`: a signed JWT.

Conflicting sources and incomplete Basic credentials are errors. Header names are
case-insensitive; duplicate differently-cased headers are rejected. JWT claims
require a signing secret. `jwtclaims.exp` retains its seconds-offset meaning.
Tokens issued elsewhere remain opaque to the renderer: the receiving API must
validate their authority, audience, expiry, scope, and revocation.

### Transport and redirects

Credential-bearing API requests require HTTPS. Non-empty custom request headers
and configured request bodies are conservatively treated as potentially
sensitive; the header check excludes `Accept` and `Content-Type`. Plain HTTP is permitted for those requests only to literal loopback
addresses (`127.0.0.1`, `::1`, or another loopback IP) in `development` or `debug`
mode. `localhost` and private-network hostnames do not receive that exception.
Use HTTPS for deployed integrations and do not disable certificate verification.

Every API redirect must stay within the initial origin: the same scheme,
hostname, and effective port. Cross-origin redirects are rejected before a
request reaches the destination. This also protects custom headers such as
`X-API-Key`, which are not reliably covered by Go's default Authorization rules.
Endpoint and redirect URLs cannot contain user information. Redirect loops are
bounded, and outgoing requests inherit cancellation from the render context.

The API components use no cookie jar. An upstream `Set-Cookie` is not replayed on
redirects, shared with another component, or copied to the browser. Configure the
actual API endpoint directly when a service expects a different redirect origin.
These rules describe API components; a route guard's authorization call has its
own separate configuration and behavior.

Endpoint paths and query values are not inspected for credentials. Do not put
secrets there; use HTTPS explicitly and appropriate headers or request bodies.

### Migration from implicit forwarding

Review each API component that previously depended on the browser's `token`
cookie. Add `forwardtoken: token` only when that endpoint is an intended recipient.
Leave the field omitted for public data and service-authenticated components.
Remove competing authentication settings instead of depending on their previous
assignment order. Move deployed credential-bearing HTTP endpoints to HTTPS.

For Go callers constructing `ApiFragmentRenderConfig` directly, `SetCookies` is
now `[]interface{}` so it can contain strings and structured maps. Existing string
entries keep their constrained compatibility path; use `int` for `max_age` and
`bool` for `secure`/`http_only` in a structured map.

## Cookies

`api_fragment_render` can deliberately issue browser cookies from successful API
responses. `.Data.token` reads a field in the current upstream JSON response;
the configured cookie name determines what the browser stores.

```yaml
login:
  - type: api_fragment_render
  - route: auth/login
  - endpoint: https://identity.example.test/login
  - method: POST
  - headers:
      Content-Type: application/json
  - body: '{"email":"$email","password":"$password"}'
  - setcookies:
      - name: __Host-account_session
        value: '{{.Data.token}}'
        path: /
        http_only: true
        secure: true
        same_site: lax
  - inline: '<p>Login response received.</p>'
```

For `{"token":"abc123"}`, this issues `__Host-account_session=abc123`. A subsequent
API component selects it with `forwardtoken: __Host-account_session`. The browser
must first receive and store the cookie; sibling components in the same render
cannot read a cookie being issued by that response.

Structured cookie entries separate dynamic values from fixed cookie attributes.
The value uses a strict, header-specific template path. It is never HTML-escaped,
so valid opaque values containing `+` or `&` retain their exact bytes. `Data` and
`Status` are reserved template keys and cannot be replaced through `values`.
Missing, null, non-string, empty dynamic values and invalid cookie-value bytes
are errors. Response data cannot inject cookie attributes.

| Structured field | Type and behavior |
| --- | --- |
| `name` | Required literal string; valid cookie name |
| `value` | Required string; literal text or direct field expressions such as `{{.Data.token}}` or `{{.Data.session.token}}` |
| `path` | Optional nonempty literal string; use `/` for a site-wide cookie |
| `domain` | Optional nonempty literal string; omit for host-only scope |
| `http_only`, `secure` | YAML booleans; omitted means false |
| `same_site` | `lax`, `strict`, or `none`; `none` requires `secure: true` |
| `max_age` | Integer seconds; omitted means session cookie, positive sets lifetime, `0` deletes |
| `expires` | Literal HTTP-date string, such as `Wed, 09 Jun 2027 10:18:14 GMT` |

Quote template values. Cookie templates allow direct field interpolation only;
functions, pipelines, conditionals, loops, and template definitions are rejected.
Use the upstream status and an explicit response contract to decide whether a
credential is issued. Do not use an empty templated token as a logout signal.

Cookies are emitted only after the final upstream status is `2xx`, response
processing and the fragment template succeed, and **every** cookie passes
validation. If any entry fails, no cookies from that component are added.
The HTTP runtime stages those cookies until rendering finishes. A later route
source error or a plugin-owned response discards them before headers are sent.
An invalid upstream JSON body remains an error instead of being retried against
an already-consumed stream. A bodyless `204` remains a valid success.

The upstream status controls cookie eligibility independently of the configured
browser `response.status`. Component errors use the existing render diagnostics;
do not treat browser HTTP 200 alone as proof that a login succeeded.

Logout explicitly deletes the same name and scope:

```yaml
logout:
  - type: api_fragment_render
  - route: auth/logout
  - endpoint: https://identity.example.test/logout
  - method: POST
  - forwardtoken: __Host-account_session
  - setcookies:
      - name: __Host-account_session
        value: ''
        path: /
        http_only: true
        secure: true
        same_site: lax
        max_age: 0
  - inline: '<p>Logout response received.</p>'
```

An omitted `max_age` creates a session cookie; explicit `max_age: 0` deletes it.
An empty value produced by a dynamic expression cannot silently become logout.
The deletion must retain the original name, Path, and Domain scope.

Legacy `setcookie` and string entries in `setcookies` remain supported through a
constrained compatibility path:

```yaml
- setcookie: 'account_session={{.Data.token}}; Path=/; HttpOnly; Secure; SameSite=Lax'
```

Only the value can contain a supported dynamic expression. Cookie names and
attributes must remain literal. Arbitrary templates that generate entire headers
or attributes are rejected. Prefer the structured form for new configuration.

Validation rejects malformed syntax, control characters, unknown attributes,
invalid cookie names/values, invalid SameSite settings, and unsafe prefix
combinations. `SameSite=None` requires `Secure`; `__Host-` requires `Secure`,
`Path=/`, and no Domain; `__Secure-` requires `Secure`. Explicit domains must match
the incoming host and cannot be public suffixes. Accepted cookies are serialized
through Go's cookie representation as separate `Set-Cookie` headers. Attributes
unknown to the runtime require explicit support rather than being silently passed
through.

Prefer host-only `Secure`, `HttpOnly` cookies and an appropriate SameSite policy
for credentials. Ordinary preference cookies do not automatically receive
credential-specific flags. HTTP cookie scope governs browser delivery;
`forwardtoken` governs the later server-to-API transfer.

## Guards

`api_fragment_render` can declare a `guard` block. The guard runs before the upstream API call. A denied request never reaches the upstream endpoint.

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
  - endpoint: https://accounts.example.test/api/profile
  - method: GET
  - forwardtoken: token
  - template:
      file: fragments/profile.html
```

See [Route Guard](ROUTE_GUARD.md) for the full guard contract.

## Security Notes

- Keep `querykeys` narrow and validate submitted data at the operation owner.
- Review every non-empty `forwardtoken` as permission to disclose that named
  credential to the configured endpoint.
- Protect personalized HTML with route caching settings and suitable HTTP
  `Cache-Control` headers; forwarding selection does not change caching policy.
- API debug output contains metadata only. Templates can still intentionally
  render `.Data`; never display credential fields in page templates.
- Token issuers and receiving services enforce token audience, expiry, scope,
  signature verification, and revocation.

The runnable [API security test module](../modules/api-security-test/README.md)
contains a controlled upstream service, configuration examples, the research
article, and automated HTTP assertions for both allowed and rejected behavior.
For all available component fields, see [Reference](REFERENCE.md).
