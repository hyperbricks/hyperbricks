# Route Guard

Route guard is an optional pre-render authorization barrier for route-owning
components.

If `guard` is absent, or `guard.enabled` is `false`, the route renders normally.

## Supported Route Owners

Route guard applies only to root route owners:

- `hypermedia`
- `fragment`
- `api_fragment_render`

It does not apply to embedded components such as `tree`, `template`,
`api_render`, or `plugin`.

## What It Does

Some routes should not execute unless the request is allowed first.

Examples:

- a page should not render for anonymous users
- a fragment should not execute its children unless the user is allowed
- an API fragment should not call an upstream API unless the request is allowed

Route guard is evaluated before rendering starts.

If the guard denies the request:

- no child items render
- no plugins execute
- no templates render
- no upstream request is made for `api_fragment_render`

## Guard Shape

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
    redirect: /login
    hx_redirect: /login
    status: 401
  on_forbidden:
    redirect: /forbidden
    hx_redirect: /forbidden
    status: 403
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

If `auth.cookie` is set, HyperBricks tries that cookie first. Header auth can
also be used.

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
- Placeholders such as `$project` are interpolated from request query/form
  values.
- If the outgoing authorization request has no `Authorization` header,
  HyperBricks forwards the resolved token as `Bearer <token>`.

### `on_unauthenticated`

Defines what happens when authentication is missing or invalid.

```yaml
guard:
  on_unauthenticated:
    redirect: /login
    hx_redirect: /login
    status: 401
```

### `on_forbidden`

Defines what happens when the request is authenticated but not allowed.

```yaml
guard:
  on_forbidden:
    redirect: /forbidden
    hx_redirect: /forbidden
    status: 403
```

## Evaluation Order

HyperBricks evaluates route guard in this order:

1. Resolve the token from cookie/header settings.
2. If `require.authenticated` is `true` and no token is resolved, deny as
   unauthenticated.
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

The `406` rule is useful for systems such as PostgREST, where a singular-object
authorization check can return `406 Not Acceptable` when no visible row exists.

## Response Behavior

When a guard denies a request:

- full page requests can use `Location` when `redirect` is set
- HTMX requests can use `HX-Redirect` when `hx_redirect` is set

Guarded routes are treated as non-cacheable.

Typical response headers:

```text
Cache-Control: no-store
Vary: Cookie, Authorization, HX-Request
```

## Protected Page

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
        redirect: /login
      on_forbidden:
        redirect: /forbidden
  - main:
      - type: tree
      - content:
          - type: html
          - value: <h1>Private dashboard</h1>
```

Anonymous requests are redirected before the page renders.

## Owner-Only Page

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
        redirect: /login
      on_forbidden:
        redirect: /forbidden
  - main:
      - type: tree
      - content:
          - type: html
          - value: <h1>Project builder</h1>
```

The request must be authenticated, include `?project=<slug>`, and pass the
upstream authorization check before rendering starts.

## Protected Fragment

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
        hx_redirect: /login
        status: 401
      on_forbidden:
        hx_redirect: /forbidden
        status: 403
  - panel:
      - type: html
      - value: <div>Members panel</div>
```

If the route is denied, `panel` does not render.

## Protected API Fragment

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
        hx_redirect: /login
        status: 401
      on_forbidden:
        hx_redirect: /forbidden
        status: 403
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

Guard blocks the route early. It does not remove the need for authorization in
the service or action that owns the protected operation.
