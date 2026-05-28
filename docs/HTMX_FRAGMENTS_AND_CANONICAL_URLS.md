# HTMX Fragments And Canonical URLs

HyperBricks can serve full pages and HTMX fragments from route-owning
components. Keep those route owners explicit.

The recommended pattern is:

- use one canonical route for the full page
- use a separate route for the HTMX fragment
- use `hx-push-url` when the browser URL should remain or become canonical
- reuse content through composition or inheritance instead of route collisions

## Why Route Collisions Are A Problem

A full page and a fragment should not both claim the same route.

```yaml
assets_page:
  - type: hypermedia
  - route: assets
  - title: Assets

assets_fragment:
  - type: fragment
  - route: assets
```

Both objects want to own `/assets`. That creates an unclear model:

- normal requests might expect a full page
- HTMX requests might expect a fragment
- cache behavior becomes request-header dependent
- static output becomes ambiguous
- diagnostics and route ownership become harder to reason about

HyperBricks should not rely on accidental duplicate routes to choose output.

## Preferred Pattern

Give the page and fragment separate routes.

```yaml
assets_page:
  - type: hypermedia
  - route: assets
  - title: Assets
  - main:
      - type: tree
      - content:
          - inherit: assets_content

assets_fragment:
  - type: fragment
  - route: fragments/assets
  - response:
      hx_target: "#content"
      hx_reswap: innerHTML
  - content:
      - inherit: assets_content

assets_content:
  - type: tree
  - heading:
      - type: html
      - value: <h1>Assets</h1>
  - copy:
      - type: text
      - value: Shared page and fragment content.
```

The page owns `/assets`. The fragment owns `/fragments/assets`. The reusable
content lives in `assets_content`.

## HTMX Link

Use normal `href` for the canonical fallback and `hx-get` for the fragment:

```html
<a
  href="/assets"
  hx-get="/fragments/assets"
  hx-target="#content"
  hx-push-url="/assets"
>
  Assets
</a>
```

This gives the browser and crawler a stable URL while HTMX can update part of
the page.

## Fragment Response Headers

Fragment response headers belong on the fragment route owner.

```yaml
assets_fragment:
  - type: fragment
  - route: fragments/assets
  - response:
      hx_target: "#content"
      hx_reswap: innerHTML
      hx_push_url: /assets
  - content:
      - inherit: assets_content
```

Use response fields when the fragment itself should tell HTMX how to apply the
response. Use attributes in HTML links or buttons when the trigger should own
the behavior.

## When Same-URL Fragment Selection Is Appropriate

Some frameworks intentionally render a full page for a normal request and a
fragment for the same URL when `HX-Request` is present.

HyperBricks should model that as an explicit feature if it is needed. Until
then, prefer separate fragment routes and shared content.

## Rule

Do not use route collisions to model HTMX fragments.

Use canonical page routes for browser state, explicit fragment routes for HTMX
requests, and reusable content nodes for shared rendering.
