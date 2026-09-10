# Route Guard

Route guard is an optional pre-render authorization barrier for route-owning components.

If `guard` is absent, or `guard.enabled` is `false`, the route renders normally.

## Supported Route Owners

Route guard applies only to root route owners:

- `hypermedia`
- `fragment`
- `api_fragment_render`

It does not apply to embedded components such as `tree`, `template`, `api_render`, or `plugin`.

## What It Does

Some routes should not execute unless the request is allowed first.

Examples:

- a page should not render for anonymous users
- a fragment should not execute its children unless the user is allowed
- an API fragment should not call an upstream API unless the request is allowed

Route guard is evaluated before rendering starts.

The access check runs on the server and does not depend on HTMX. After a denial, the configured response determines how the browser is notified.

If the guard denies the request:

- no child items render
- no plugins execute
- no templates render
- no upstream request is made for `api_fragment_render`

## Guard Shape

This example supports ordinary browser navigation and [HTMX 4](https://four.htmx.org/) requests. The `HX-*` headers configure the HTMX response variants; they do not affect the authorization decision.

```yaml
guard:
  enabled: true
  auth:
    cookie: token
    header: Authorization
    scheme: Bearer
  require:
    authenticated: true
    query:
      project: true
  authorize:
    endpoint: http://127.0.0.1:13000/projects?slug=eq.$project&select=id&limit=1
    method: GET
    headers:
      Accept: application/vnd.pgrst.object+json
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
  on_forbidden:
    default:
      status: 303
      headers:
        Location: /forbidden
    variants:
      - when:
          request_headers:
            HX-Request: "true"
        response:
          status: 403
          headers:
            HX-Redirect: /forbidden
```

## Fields

### `enabled`

Turns guard behavior on or off.

```yaml
guard:
  enabled: true
```

Rules:

- omitted: no guard behavior
- `false`: no guard behavior
- `true`: evaluate guard before route render

### `auth`

Controls how HyperBricks resolves the request token.

```yaml
guard:
  enabled: true
  auth:
    cookie: token
    header: Authorization
    scheme: Bearer
```

Fields:

| Field | Meaning |
| --- | --- |
| `cookie` | Cookie name used to resolve the token. |
| `header` | Header name used to resolve the token. |
| `scheme` | Optional header scheme. `Bearer` is the default for `Authorization`. |

If `auth.cookie` is set, HyperBricks tries that cookie first. Header auth can also be used.

### `require`

Declares conditions that must be true before the route may continue.

```yaml
guard:
  enabled: true
  require:
    authenticated: true
    query:
      project: true
```

Rules:

- `authenticated: true` requires a resolved token.
- `query.<key>: true` requires a non-empty query string value.

### `authorize`

Performs an optional upstream authorization check before the route renders.

```yaml
guard:
  enabled: true
  authorize:
    endpoint: http://127.0.0.1:13000/project_memberships?project=eq.$project&role=eq.owner&limit=1
    method: GET
    headers:
      Accept: application/vnd.pgrst.object+json
    body: '{"project":"$project"}'
```

Rules:

- If `authorize.endpoint` is omitted, no upstream authorization request is made.
- Placeholders such as `$project` are interpolated from request query/form values.
- If the outgoing authorization request has no `Authorization` header, HyperBricks forwards the resolved token as `Bearer <token>`.

### `on_unauthenticated`

Defines what happens when authentication is missing or invalid.

Example with a normal redirect and an HTMX response variant:

```yaml
guard:
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

### `on_forbidden`

Defines what happens when the request is authenticated but not allowed.

Example with a normal redirect and an HTMX response variant:

```yaml
guard:
  on_forbidden:
    default:
      status: 303
      headers:
        Location: /forbidden
    variants:
      - when:
          request_headers:
            HX-Request: "true"
        response:
          status: 403
          headers:
            HX-Redirect: /forbidden
```

## Evaluation Order

HyperBricks evaluates route guard in this order:

1. Resolve the token from cookie/header settings.
2. If `require.authenticated` is `true` and no token is resolved, deny as unauthenticated.
3. Check required query keys.
4. If `authorize.endpoint` exists, perform the upstream authorization request.
5. Interpret the upstream response status.

## Authorization Status Semantics

| Upstream status | Meaning |
| --- | --- |
| `2xx` | allow render |
| `401` | unauthenticated |
| `403` | forbidden |
| `406` | forbidden |
| anything else | authorization failure |

The `406` rule is useful for systems such as PostgREST, where a singular-object authorization check can return `406 Not Acceptable` when no visible row exists.

## Response Behavior

Both denial actions use the same HTTP response shape: `default` plus optional ordered `variants`. HyperBricks evaluates variants only after denying access. Client headers select the denial response; they never grant access.

- Every entry in `when.request_headers` must match for that variant to apply.
- Header names are case-insensitive; values match exactly and case-sensitively. `HX-Request: "True"` does not match `HX-Request: "true"`.
- A missing request header does not match. Use non-empty selector maps.
- The first matching variant replaces the entire default response, including its status and headers. Headers are not merged with the default.
- If no variant matches, HyperBricks uses `default`.
- Without a configured status, unauthenticated denials use `401` and forbidden denials use `403`. Redirects need an explicit redirect status, such as `303`, and `headers.Location`.

The examples above explicitly choose `303` with `Location` for a normal browser request and `401`/`403` with `HX-Redirect` for an HTMX request. There is no built-in HTMX request detection or automatic redirect-header conversion. A project using another client can configure that client's HTTP conventions in the same shape. The removed `redirect` and `hx_redirect` shorthands are invalid; see [HTTP response migration](HTTP_RESPONSES.md#migrate-existing-configuration).

Guarded routes bypass the internal response cache and use `Cache-Control: no-store`. `Vary` includes the configured authentication inputs and request header selectors, so response selection is reflected in HTTP cache metadata. For the complete guard example above that includes:

```text
Cache-Control: no-store
Vary: Cookie, Authorization, HX-Request
```

`HX-Request` is included because the example configures it, not because the runtime treats it specially. Authentication and authorization settings retain their existing meanings.

## Protected Page

Example with HTMX-aware denial responses:

```yaml
dashboard:
  - type: hypermedia
  - route: dashboard
  - title: Dashboard
  - guard:
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
      on_forbidden:
        default:
          status: 303
          headers:
            Location: /forbidden
        variants:
          - when:
              request_headers:
                HX-Request: "true"
            response:
              status: 403
              headers:
                HX-Redirect: /forbidden
  - main:
      - type: tree
      - content:
          - type: html
          - value: <h1>Private dashboard</h1>
```

Anonymous requests are redirected before the page renders.

## Owner-Only Page

Example with an upstream authorization check and HTMX-aware denial responses:

```yaml
builder:
  - type: hypermedia
  - route: builder
  - title: Builder
  - guard:
      enabled: true
      auth:
        cookie: token
      require:
        authenticated: true
        query:
          project: true
      authorize:
        endpoint: http://127.0.0.1:13000/project_memberships?select=project_id,project:projects!inner(slug)&project.slug=eq.$project&role=eq.owner&limit=1
        method: GET
        headers:
          Accept: application/vnd.pgrst.object+json
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
      on_forbidden:
        default:
          status: 303
          headers:
            Location: /forbidden
        variants:
          - when:
              request_headers:
                HX-Request: "true"
            response:
              status: 403
              headers:
                HX-Redirect: /forbidden
  - main:
      - type: tree
      - content:
          - type: html
          - value: <h1>Project builder</h1>
```

The request must be authenticated, include `?project=<slug>`, and pass the upstream authorization check before rendering starts.

## Protected Fragment

Example with HTMX-aware denial responses:

```yaml
project_members:
  - type: fragment
  - route: project/members
  - guard:
      enabled: true
      auth:
        cookie: token
      require:
        authenticated: true
        query:
          project: true
      authorize:
        endpoint: http://127.0.0.1:13000/project_memberships?select=project_id,project:projects!inner(slug)&project.slug=eq.$project&role=eq.owner&limit=1
        method: GET
        headers:
          Accept: application/vnd.pgrst.object+json
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
      on_forbidden:
        default:
          status: 303
          headers:
            Location: /forbidden
        variants:
          - when:
              request_headers:
                HX-Request: "true"
            response:
              status: 403
              headers:
                HX-Redirect: /forbidden
  - panel:
      - type: html
      - value: <div>Members panel</div>
```

If the route is denied, `panel` does not render.

## Protected API Fragment

Example with HTMX-aware denial responses:

```yaml
project_status:
  - type: api_fragment_render
  - route: project/status
  - method: GET
  - endpoint: http://127.0.0.1:13000/rpc/project_status?project=$project
  - guard:
      enabled: true
      auth:
        cookie: token
      require:
        authenticated: true
        query:
          project: true
      authorize:
        endpoint: http://127.0.0.1:13000/projects?slug=eq.$project&select=id&limit=1
        method: GET
        headers:
          Accept: application/vnd.pgrst.object+json
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
      on_forbidden:
        default:
          status: 303
          headers:
            Location: /forbidden
        variants:
          - when:
              request_headers:
                HX-Request: "true"
            response:
              status: 403
              headers:
                HX-Redirect: /forbidden
  - inline: |
      <div class="status">
        {{ .Data }}
      </div>
```

If guard denies the request, HyperBricks does not call the upstream `endpoint`.

## Good Uses

Use route guard when:

- the route should not render for anonymous users
- the route requires a project or tenant query context
- the route should check role or membership before execution
- the route should not perform upstream work unless the request is allowed

## Non-Goals

Route guard is not:

- a replacement for backend authorization
- a replacement for database row-level security
- a substitute for validating business operations inside plugins or services
- a child item sequencing mechanism

Guard blocks the route early. It does not remove the need for authorization in the service or action that owns the protected operation.
