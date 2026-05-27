# HTMX Fragments and Canonical URLs

HyperBricks can serve full hypermedia pages and smaller HTMX fragments from
root configurations. A common temptation is to give a page and its fragment the
same route and let `HX-Request: true` decide which one should be rendered.

That works as an idea, but it creates an unclear ownership model: one URL now
means two different root configs. It also interacts poorly with route collision
handling, caching, debugging, and static output.

The preferred pattern is: keep the full page route canonical, and request the
fragment from an explicit fragment endpoint or composed fragment source.

## The Problem With Route Collisions

Consider this shape:

```hyperbricks
app_assets <<< app
app_assets.route = assets

assets = <FRAGMENT>
assets.route = assets
```

Both configs want to own `/assets`.

HyperBricks prevents direct route overwrites by making duplicate route keys
unique. The first config keeps `assets`; the next one becomes something like
`assets_1`. That avoids accidental overwrites, but it also means the fragment no
longer lives at the route the author probably expected.

Using `HX-Request: true` to recover from this would make route selection depend
on request headers. That is possible, but it makes the route less explicit:

- normal request to `/assets` returns the full page
- HTMX request to `/assets` returns a fragment
- refresh, copy/paste, cache behavior, and diagnostics all need to account for
  that split

For most applications, this is unnecessary complexity.

## Preferred Pattern

Use one canonical route for the page, and a separate fragment request target for
HTMX.

The browser URL should stay canonical:

```text
/assets
```

The HTMX request can target the fragment explicitly:

```html
<a
  href="/assets"
  hx-get="/fragments/assets"
  hx-target="#content_right"
  hx-push-url="/assets"
>
  Assets
</a>
```

In this model:

- `href` is the non-JavaScript fallback
- `hx-get` requests the fragment response
- `hx-target` chooses where the fragment is swapped
- `hx-push-url` keeps the address bar on the canonical page route

This avoids route collisions while preserving correct browser history and share
URLs.

## Composition Example

When a page is derived from an app shell, keep the page route on the composed
page:

```hyperbricks
app_assets <<< app
app_assets.route = assets
```

Then compose or reuse the fragment content explicitly:

```hyperbricks
app_assets.10.values.content.10.values.content_right <<< assets.10
```

This expresses the ownership clearly:

- `app_assets` owns the `/assets` page route
- `assets.10` owns the reusable content block
- HTMX can request a fragment endpoint for that block
- `hx-push-url="/assets"` keeps the visible URL stable

The important point is that the page route and the fragment source do not need
to collide. The page decides where the fragment belongs, and HTMX decides when
to fetch it.

## When To Use Header-Based Selection

Header-based selection can still be useful when a framework deliberately treats
one URL as both a full-page endpoint and a fragment endpoint:

```text
GET /assets                  -> full page
GET /assets + HX-Request     -> fragment
```

If HyperBricks supports this as a first-class feature, it should be modeled
explicitly rather than inferred from accidental route collisions. For example,
a page config could declare which fragment should answer HTMX requests for the
same canonical route.

Until that exists, prefer explicit fragment targets plus `hx-push-url`.

## Rule Of Thumb

Do not use route collisions to model HTMX fragments.

Use canonical page routes for browser state, explicit fragment targets for HTMX
requests, and composition to reuse the same content in both places.
