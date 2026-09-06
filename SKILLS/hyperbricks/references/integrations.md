# Server logic and integrations

Use this reference when presentation needs a calculation, upstream data, or
protected actions. Core manuals: `docs/GOJA_RENDER.md`, `docs/API_RENDER.md`,
`docs/ROUTE_GUARD.md`. Read [plugins](plugins.md) for host capabilities and custom
server workflows.

## Choose the owner

Keep text formatting, defaults, and simple presentation transformations in the
template. Use `goja_render` for small synchronous calculations with configured
values and selected query inputs. Use an API component when an existing service
owns the operation. Use a native Go plugin for storage, host integrations,
transactions, or custom server workflows that belong inside HyperBricks.

`goja_render` is beta and intended for trusted project scripts. It is a practical
alternative to compiling a custom plugin for small calculations. It is not an
untrusted-code sandbox or a persistent session store. It has no Node.js,
filesystem, or network APIs.

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

With `?quantity=3`, this renders `3 items cost €59.85`. This is a teaching
calculation; production financial rules belong to the application's domain
model. Verify a valid value, a rejected value, and repeated query keys.

Each execution gets a fresh JavaScript runtime and copied input; the compiled
script and parsed template are reused. Globals do not persist between renders.
Configured `values` are plain data, not rendered child components. Return a
plain JSON-compatible object from synchronous `main(input)`. Use exactly one of
`inline` and `template`. Omitted Goja `querykeys` exposes no query input, unlike
the default template/API allowlist. Routes containing Goja disable response
caching. The configured timeout defaults to 100ms and must be positive, up to 5s.

## Read from an API

`api_render` is nested under a page/fragment; its response can be cached with the
parent. `api_fragment_render` owns its route and is always dynamic. For example,
a fragment against an application-owned service can look like:

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

Set `myconf.api.status_endpoint` in the package to the actual reachable service
URL. This recipe expects JSON such as `{"message":"Ready"}` on success; it is
not a supplied backend. The integrated dashboard provides a separate local API
lesson with a documented service and explicit demo-state limits.

`.Data` contains the parsed response; `.Status` is the upstream HTTP status;
configured `values` are available at the template root. Explicit `querykeys`
limits incoming URL query forwarding; `queryparams` supplies static outgoing
query values. Templates and APIs default to `id`, `name`, `order` when query keys
are omitted. API form and JSON body input have their own mapping rules, so a
query allowlist is not validation of a submitted form.

## Forms and refreshes

Use explicit action routes when an API or plugin owns writes. A typical sequence
is: submit form → perform/validate the action → render feedback → refresh the
related read panel after success. Keep HTML in templates and backend validation
at the operation owner. For API body mapping and authentication, read the API
manual before forwarding fields or credentials.

Inspect both upstream `.Status` and the browser's HTTP response: do not assume
an upstream error is automatically forwarded as the same browser status. A fixed
`response.hx_trigger` is response metadata, not a success condition. Emit or
handle the refresh event only when the operation actually succeeded. Exercise
validation, conflict, unavailable service, and success in the browser. Use the
integrated optional lesson as the verified complete flow when available.

## Protect the routes that do the work

Guards belong on `hypermedia`, `fragment`, and `api_fragment_render`. They run
before child rendering and upstream API calls. Protect the full page and any
independently callable fragment or action; hiding its navigation is insufficient.

A guard has separate concerns:

- `auth` locates a token in a configured cookie/header.
- `require.authenticated: true` requires token presence; it does not validate a
  signature or check that a session is still valid.
- `require.query` requires named query values to be non-empty.
- `authorize.endpoint`, when configured, calls the authorization owner to decide
  whether this request may continue.
- `on_unauthenticated` and `on_forbidden` define status and normal/HTMX redirects.

The authorization endpoint allows `2xx`, treats `401` as unauthenticated and
`403`/`406` as forbidden, and rejects other outcomes. It can receive the resolved
bearer token. The storage/API/plugin operation must also enforce the user's
permissions; a route guard does not replace data-level authorization.

Guarded responses are non-cacheable. Sample fixed tokens or an in-memory service
are useful for trying the flow, but do not constitute production identity or
durable storage. Keep those prerequisites explicit in optional lesson docs.
