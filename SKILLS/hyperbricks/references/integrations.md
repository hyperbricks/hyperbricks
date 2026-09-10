# Server logic and integrations

Use this reference when presentation needs a calculation, upstream data, or protected actions. Core manuals: `docs/GOJA_RENDER.md`, `docs/API_RENDER.md`, `docs/ROUTE_GUARD.md`. Read [plugins](plugins.md) for host capabilities and custom server workflows.

## Choose the owner

Keep text formatting, defaults, and simple presentation transformations in the template. Use `goja_render` for small synchronous calculations with configured values and selected query inputs. Use an API component when an existing service owns the operation. Use a native Go plugin for storage, host integrations, transactions, or custom server workflows that belong inside HyperBricks.

`goja_render` is beta and intended for trusted project scripts. It is a practical alternative to compiling a custom plugin for small calculations. It is not an untrusted-code sandbox or a persistent session store. It has no Node.js, filesystem, or network APIs.

## A small calculation

Put this component below a page or fragment, or in a template value slot:

```yaml
line_total:
  - type: goja_render
  - script:
      file: {base: resources, path: scripts/line-total.js}
  - querykeys: [quantity]
  - values:
      unit_price: 19.95
  - timeout: 100ms
  - inline: '<p>{{ .Data.message }}</p>'
```

Create `resources/scripts/line-total.js`:

```js
function main(input) {
  const raw = input.query.quantity;
  if (Array.isArray(raw)) {
    return { message: "Choose one quantity." };
  }
  const quantity = raw === undefined ? 1 : Number(raw);
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000) {
    return { message: "Choose a whole quantity from 1 to 1000." };
  }
  const total = quantity * Number(input.values.unit_price);
  return { message: `${quantity} items cost €${total.toFixed(2)}` };
}
```

With `?quantity=3`, this renders `3 items cost €59.85`. This is a teaching calculation; production financial rules belong to the application's domain model. Verify a valid value, a rejected value, and repeated query keys.

Each execution gets a fresh JavaScript runtime and copied input; the compiled script and parsed template are reused. Globals do not persist between renders. Configured `values` are plain data, not rendered child components. Return a plain JSON-compatible object from synchronous `main(input)`. Use exactly one of `inline` and `template`. Omitted Goja `querykeys` exposes no query input, unlike the default template/API allowlist. Routes containing Goja disable response caching. The configured timeout defaults to 100ms and must be positive, up to 5s.

## Read from an API

Neither API component caches upstream API responses. Whenever one executes, it
makes a fresh HTTP request. `api_render` is nested under a page or fragment and
has no `route` or `nocache` field. Its rendered HTML follows the parent route's
cache policy: a parent cache hit skips the entire child render, including the API
call. Put `nocache: true` on the parent route when every request must fetch current
API data.

`api_fragment_render` is a route-owning root component and always bypasses the
rendered-output cache. Every route invocation therefore executes the component
and calls its upstream. This is distinct from browser or proxy caching, which is
controlled through HTTP response headers. For example, a fragment against an
application-owned service can look like:

```yaml
project_status:
  - type: api_fragment_render
  - route: fragments/project-status
  - endpoint:
      config: myconf.api.status_endpoint
  - method: GET
  - querykeys: [project]
  - inline: |
      {{ if and (ge .Status 200) (lt .Status 300) }}
        <p>{{ .Data.message }}</p>
      {{ else }}
        <p role="alert">The project status could not be loaded.</p>
      {{ end }}
```

Set `myconf.api.status_endpoint` in the package to the actual reachable service URL. This recipe expects JSON such as `{"message":"Ready"}` on success; it is not a supplied backend. The integrated dashboard provides a separate local API lesson with a documented service and explicit demo-state limits.

`.Data` contains the parsed response; `.Status` is the upstream HTTP status; configured `values` are available at the template root. Explicit `querykeys` limits incoming URL query forwarding; `queryparams` supplies static outgoing query values. Templates and APIs default to `id`, `name`, `order` when query keys are omitted. API form and JSON body input have their own mapping rules, so a query allowlist is not validation of a submitted form.

## Forms and refreshes

Use explicit action routes when an API or plugin owns writes. A typical sequence is: submit form → perform/validate the action → render feedback → refresh the related read panel after success. Keep HTML in templates and backend validation at the operation owner. For API body mapping and authentication, read the API manual before forwarding fields or credentials.

Inspect both upstream `.Status` and the browser's HTTP response: do not assume an upstream error is automatically forwarded as the same browser status. A fixed `response.headers.HX-Trigger` is response metadata, not a success condition. Emit or handle the refresh event only when the operation actually succeeded. Exercise validation, conflict, unavailable service, and success in the browser. Use the integrated optional lesson as the verified complete flow when available.

## Protect the routes that do the work

Guards belong on `hypermedia`, `fragment`, and `api_fragment_render`. They run before child rendering and upstream API calls. Protect the full page and any independently callable fragment or action; hiding its navigation is insufficient.

A guard has separate concerns:

- `auth` locates a token in a configured cookie/header.
- `require.authenticated: true` requires token presence; it does not validate a signature or check that a session is still valid.
- `require.query` requires named query values to be non-empty.
- `authorize.endpoint`, when configured, calls the authorization owner to decide whether this request may continue.
- `on_unauthenticated` and `on_forbidden` each define a `default` HTTP response and optional ordered `variants` selected by `when.request_headers`.

The authorization endpoint allows `2xx`, treats `401` as unauthenticated and `403`/`406` as forbidden, and rejects other outcomes. It can receive the resolved bearer token. The storage/API/plugin operation must also enforce the user's permissions; a route guard does not replace data-level authorization.

Each response contains optional `status` and a `headers` map of literal strings. All configured request headers in a variant must match; names are case-insensitive and values are exact and case-sensitive. The first match replaces the default response completely. Without a match, the default is used. For HTMX redirects, explicitly configure a variant matching `HX-Request: "true"` with `response.headers.HX-Redirect`; the core has no implicit HTMX redirect rule. See `docs/ROUTE_GUARD.md` for complete examples and `docs/HTTP_RESPONSES.md` for the migration from removed `response.hx_*`, `redirect`, and `hx_redirect` fields.

Route owners also use `response.status` and `response.headers` for the browser response. In API components, top-level `headers` remains the upstream request headers. An API template's `.Status` remains the upstream status, independently of `response.status`. Only the root route determines response metadata; a nested fragment cannot change the enclosing page's status or headers.

Guarded responses are non-cacheable. Sample fixed tokens or an in-memory service are useful for trying the flow, but do not constitute production identity or durable storage. Keep those prerequisites explicit in optional lesson docs.
