# Composite Route Guard

Status: working reference  
Date: 2026-04-30

## Purpose

This document explains the shared `guard { ... }` mechanism for route-owning
HyperBricks composites.

The feature is optional.

If `guard` is absent, or `guard.enabled = false`, the route behaves exactly as
it did before.

## Supported route owners

The shared route guard applies only to route-owning root composites:

- `<HYPERMEDIA>`
- `<FRAGMENT>`
- `<API_FRAGMENT_RENDER>`

It does not apply to embedded non-route components such as:

- `<API_RENDER>`
- `<TEMPLATE>`
- `<TREE>`
- `<PLUGIN>`

## What problem it solves

Some routes should not execute unless the request is allowed first.

Examples:

- a full page should not render for anonymous users
- a fragment route should not execute its child plugins unless the user is
  allowed
- an API fragment route should not make an upstream HTTP request unless the
  request is allowed first

That is what `guard` is for.

It is a pre-render route barrier.

## Core rule

`guard` is evaluated before rendering starts.

That means:

- for `<HYPERMEDIA>`, before shell/page rendering
- for `<FRAGMENT>`, before fragment tree/template rendering
- for `<API_FRAGMENT_RENDER>`, before any upstream API/proxy call

If the guard denies the request:

- no child items render
- no plugins execute
- no templates render
- no upstream API request is made for `<API_FRAGMENT_RENDER>`

## Contract

The route guard is declared as:

```hyperbricks
guard {
    enabled = true
    auth { ... }
    require { ... }
    authorize { ... }
    on_unauthenticated { ... }
    on_forbidden { ... }
}
```

### `enabled`

Turns the guard on or off.

```hyperbricks
guard.enabled = true
```

Rules:

- omitted => no guard behavior
- `false` => no guard behavior
- `true` => evaluate guard before route render

### `auth`

Controls how HyperBricks resolves the request token.

```hyperbricks
guard.auth.cookie = token
guard.auth.header = Authorization
guard.auth.scheme = Bearer
```

Fields:

- `cookie`
- `header`
- `scheme`

Behavior:

- if `auth.cookie` is set, HyperBricks tries that cookie first
- otherwise or additionally it can inspect a header
- default header behavior is compatible with `Authorization: Bearer <token>`

### `require`

Declares conditions that must be true before the route may continue.

```hyperbricks
guard.require.authenticated = true
guard.require.query.project = true
```

Fields:

- `authenticated`
- `query.<key> = true`

Behavior:

- `authenticated = true` requires a resolved token
- `query.<key> = true` requires a non-empty query string value

### `authorize`

Performs an optional upstream authorization check before the route renders.

```hyperbricks
guard.authorize.endpoint = http://127.0.0.1:13000/projects?slug=eq.$project&select=id&limit=1
guard.authorize.method = GET
guard.authorize.headers.Accept = application/vnd.pgrst.object+json
guard.authorize.body = {"project_slug":"$project"}
```

Fields:

- `endpoint`
- `method`
- `headers`
- `body`

Behavior:

- if `authorize.endpoint` is missing, no upstream authorization request is made
- request placeholders such as `$project` are interpolated from query/form
  values
- if the outgoing authorization request does not already define
  `Authorization`, HyperBricks forwards the resolved token as
  `Bearer <token>`

### `on_unauthenticated`

Defines what happens when authentication is missing or invalid.

```hyperbricks
guard.on_unauthenticated.redirect = /login
guard.on_unauthenticated.hx_redirect = /login
guard.on_unauthenticated.status = 401
```

Fields:

- `redirect`
- `hx_redirect`
- `status`

### `on_forbidden`

Defines what happens when the request is authenticated but not allowed.

```hyperbricks
guard.on_forbidden.redirect = /forbidden
guard.on_forbidden.hx_redirect = /forbidden
guard.on_forbidden.status = 403
```

Fields:

- `redirect`
- `hx_redirect`
- `status`

## Evaluation order

HyperBricks evaluates the route guard in this order:

1. Resolve the token from cookie/header settings.
2. If `require.authenticated = true` and no token is resolved, deny as
   unauthenticated.
3. Check required query keys.
4. If `authorize.endpoint` exists, perform the upstream authorization request.
5. Interpret the upstream response status.

## Status semantics

The current guard semantics are:

- `2xx` => allow render
- `401` => unauthenticated
- `403` => forbidden
- `406` => forbidden
- anything else => authorization failure (`502` style behavior)

The `406` rule matters for systems like PostgREST that may return
`406 Not Acceptable` for singular-object auth checks when no visible row
exists.

For route guard purposes, that is treated as access denial.

## Response behavior

When a guard denies a request:

- full-page style requests use `Location` when `redirect` is set
- HTMX requests use `HX-Redirect` when `hx_redirect` is set

Default behavior:

- full-page redirect typically returns `303`
- HTMX deny typically returns `401` or `403`

Guarded routes are treated as non-cacheable.

Response characteristics:

- `Cache-Control: no-store`
- `Vary: Cookie, Authorization, HX-Request`

## Why this is a route feature

`guard` is not a child component or plugin sequencing feature.

It exists at the route owner because route access must be decided before the
route work starts.

This is especially important for:

- fragment routes with multiple child items
- routes that use plugins
- routes that proxy to upstream APIs

## Example: protected page with `<HYPERMEDIA>`

```hyperbricks
dashboard = <HYPERMEDIA>
dashboard.route = dashboard
dashboard.title = Dashboard
dashboard.guard {
    enabled = true
    auth.cookie = token
    require.authenticated = true
    on_unauthenticated.redirect = /login
    on_forbidden.redirect = /forbidden
}
dashboard.template.inline = <<[
<!DOCTYPE html>
<html>
  <body>
    <h1>Private dashboard</h1>
  </body>
</html>
]>>
```

Meaning:

- anonymous requests are redirected before the page renders
- authenticated requests may continue

## Example: owner-only project page with `<HYPERMEDIA>`

```hyperbricks
builder = <HYPERMEDIA>
builder.route = builder
builder.guard {
    enabled = true
    auth.cookie = token
    require.authenticated = true
    require.query.project = true
    authorize.endpoint = http://127.0.0.1:13000/project_memberships?select=project_id,project:projects!inner(slug)&project.slug=eq.$project&role=eq.owner&limit=1
    authorize.method = GET
    authorize.headers.Accept = application/vnd.pgrst.object+json
    on_unauthenticated.redirect = /login
    on_forbidden.redirect = /forbidden
}
```

Meaning:

- user must be logged in
- `?project=<slug>` must be present
- upstream authorization must confirm owner membership
- if not, the page route is denied before render

## Example: protected fragment route with `<FRAGMENT>`

```hyperbricks
project_members = <FRAGMENT>
project_members.route = project/members
project_members.guard {
    enabled = true
    auth.cookie = token
    require.authenticated = true
    require.query.project = true
    authorize.endpoint = http://127.0.0.1:13000/project_memberships?select=project_id,project:projects!inner(slug)&project.slug=eq.$project&role=eq.owner&limit=1
    authorize.method = GET
    authorize.headers.Accept = application/vnd.pgrst.object+json
    on_unauthenticated.redirect = /login
    on_forbidden.redirect = /forbidden
}
project_members.10 = <HTML>
project_members.10.value = <div>Members panel</div>
```

Meaning:

- the route is denied before fragment content renders
- child items do not execute when the route is denied

This is the correct way to protect a rooted fragment route.

## Example: protected API fragment route with `<API_FRAGMENT_RENDER>`

```hyperbricks
project_status = <API_FRAGMENT_RENDER>
project_status.route = project/status
project_status.method = GET
project_status.endpoint = http://127.0.0.1:13000/rpc/project_status?project=$project
project_status.guard {
    enabled = true
    auth.cookie = token
    require.authenticated = true
    require.query.project = true
    authorize.endpoint = http://127.0.0.1:13000/projects?slug=eq.$project&select=id&limit=1
    authorize.method = GET
    authorize.headers.Accept = application/vnd.pgrst.object+json
    on_unauthenticated.redirect = /login
    on_forbidden.redirect = /forbidden
}
project_status.inline = <<[
<div class="status">
  {{ .Data }}
</div>
]>>
```

Meaning:

- if guard denies, HyperBricks does not call the upstream `endpoint`
- if guard allows, the API fragment continues normally

## HTMX behavior example

When an HTMX request hits a guarded route:

- denied requests can use `HX-Redirect`
- the browser can redirect without a full-page render first

Example:

```hyperbricks
guard.on_unauthenticated.redirect = /login
guard.on_unauthenticated.hx_redirect = /login
guard.on_unauthenticated.status = 401
```

This is useful for fragment and API fragment routes triggered by HTMX.

## Good uses

Use route guard when:

- the route should not render for anonymous users
- the route requires a project or tenant query context
- the route should check role/membership before route execution
- the route should not perform upstream work unless the request is allowed

## Bad uses

Do not use route guard as:

- a replacement for backend authorization
- a replacement for database row-level security
- a substitute for validating business operations inside plugins or services
- a child-item sequencing mechanism

Guard blocks the route early.

It does not remove the need for real authorization in the action owner.

## Relationship to other features

`<HYPERMEDIA>.guard` and `<FRAGMENT>.guard` protect whether the route may
execute.

`<API_FRAGMENT_RENDER>` performs dynamic request-time API work after the route
is allowed.

That means:

- route guard protects the door
- fragment/API logic performs work inside the route after that decision

## Notes on current implementation

- the feature is optional
- it is enforced in the HTTP serving path before render
- it currently treats guarded routes as `nocache`
- it supports `<HYPERMEDIA>`, `<FRAGMENT>`, and `<API_FRAGMENT_RENDER>`

## Related docs

- [API_RENDER.md](API_RENDER.md)
- [HTMX_FRAGMENTS_AND_CANONICAL_URLS.md](HTMX_FRAGMENTS_AND_CANONICAL_URLS.md)
- [ROUTING.md](ROUTING.md)
- [REFERENCE.md](REFERENCE.md)
