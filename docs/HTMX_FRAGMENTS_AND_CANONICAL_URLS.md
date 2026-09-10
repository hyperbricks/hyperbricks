# HTMX Fragments And Canonical URLs

HyperBricks serves full HTML pages with `hypermedia` and HTML fragments with `fragment`. This guide shows how to use those routes with [HTMX 4](https://four.htmx.org/), while keeping a full page URL available for direct visits, reloads, and links without JavaScript.

Pages, fragments, and shared content are general HyperBricks features. The `hx-*` attributes and `HX-*` response headers below configure the HTMX integration.

The routing rule is:

- give each URL one route-owning component
- keep the browser URL on a canonical full-page route
- use either an explicit fragment route or `hx-select` on the canonical response
- reuse content through composition or inheritance instead of duplicating views

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

Both objects declare `/assets`, but HyperBricks does not use request headers to
choose between them. Route preprocessing keeps the registry unique by assigning
a suffix such as `_1` to a later duplicate. The result is then closer to
`/assets` and `/assets_1` than two representations of one route.

That automatic conflict recovery is not an application routing pattern:

- the generated suffix is not an intentional canonical URL
- which declaration keeps the requested route depends on processing order
- `HX-Request` does not select the page or fragment representation
- links, static output, and diagnostics no longer express the intended routes

Declare the two URLs explicitly when the application needs both responses.

## Pattern 1: Canonical Page And Explicit Fragment Route

Give the page and fragment separate routes.

```yaml
assets_page:
  - type: hypermedia
  - route: assets
  - title: Assets
  - main:
      - type: template
      - inline: '<main id="content">{{.content}}</main>'
      - values:
          content:
            - inherit: assets_content

assets_fragment:
  - type: fragment
  - route: fragments/assets
  - response:
      headers:
        HX-Retarget: "#content"
        HX-Reswap: innerHTML
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

The page owns `/assets`. The fragment owns `/fragments/assets`. The reusable content lives in `assets_content`. The page wraps it in `#content`, which is the target for fragment updates.

## HTMX Link

Use normal `href` for the canonical fallback and `hx-get` for the fragment. Load HTMX in the page and place this link in a layout containing `#content`:

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

This gives the browser and crawler a stable URL while HTMX can update part of the page.

Use an explicit fragment route when the fragment is independently useful,
significantly cheaper to render than the full page, reused from several pages,
or represents a distinct request-time operation. The fragment URL is an
implementation endpoint; `href` and `hx-push-url` continue to identify the
canonical page.

## Fragment Response Headers

Set these HTMX response headers under the fragment route's general `response.headers` mapping. HyperBricks sends the configured headers; HTMX interprets them in the browser.

```yaml
assets_fragment:
  - type: fragment
  - route: fragments/assets
  - response:
      headers:
        HX-Retarget: "#content"
        HX-Reswap: innerHTML
        HX-Push-Url: /assets
  - content:
      - inherit: assets_content
```

Use response headers when the fragment itself should tell HTMX how to apply the response. Use attributes in HTML links or buttons when the trigger should own the behavior.

## Pattern 2: Request The Canonical Page And Select Its Content

HTMX can request the canonical full-page route and select only the part needed
for the swap. This is useful for ordinary page navigation when rendering a
separate fragment would add another route without avoiding meaningful work.

The page still owns `/assets`; there is no second component claiming that URL:

```html
<a
  href="/assets"
  hx-get="/assets"
  hx-select="#content > *"
  hx-target="#content"
  hx-swap="innerHTML"
  hx-push-url="true"
>
  Assets
</a>
```

HyperBricks returns the complete `/assets` document. HTMX extracts the children
of `#content` from that response and inserts them into the existing `#content`.
Without JavaScript, the same `href` performs ordinary full-page navigation.
Direct access, reload, and browser-history restoration therefore keep a complete
page route available.

When the response also contains navigation whose active state has changed, use
`hx-select-oob` to replace it from the same canonical response:

```html
<a
  href="/assets"
  hx-get="/assets"
  hx-select="#content > *"
  hx-select-oob="#site-navigation:outerHTML"
  hx-target="#content"
  hx-swap="innerHTML"
  hx-push-url="true"
>
  Assets
</a>
```

This pattern transfers and renders the full page even though the browser swaps
only selected regions. Prefer an explicit fragment route when response size or
rendering cost makes that wasteful, or when the partial response has its own
request contract.

## Do Not Confuse Selection With Route Collisions

Some frameworks intentionally render a full page for a normal request and a fragment for the same URL when `HX-Request` is present.

HyperBricks does not automatically select a page or fragment based on `HX-Request`.
Configuring `Vary: HX-Request` only separates cache entries; it does not select
different content. Defining both a `hypermedia` and a `fragment` with the same
source route invokes duplicate-route recovery; it does not create same-URL
content negotiation.

Canonical-page selection is different: one `hypermedia` component owns the URL,
returns one complete document, and HTMX performs `hx-select` in the browser.

## Rule

Give every URL one route owner and every browser destination a canonical page.

For enhanced navigation, either request an explicit fragment and push its
canonical page URL, or request the canonical page and select the required region
from its response. Use shared components and templates so full-page and partial
updates render the same application state.
