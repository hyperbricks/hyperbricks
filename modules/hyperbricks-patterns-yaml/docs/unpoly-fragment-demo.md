# Unpoly Fragment Demo

Open `/unpoly-demo` in the running patterns module. This example uses one
shared panel template for a complete page and a fragment response. It loads
Unpoly explicitly in its own page head and needs no API or plugin.

The page pins Unpoly `3.14.3` using the CDN URLs published in its
[installation guide](https://unpoly.com/install). The browser needs access to
that CDN; no npm install or build step is needed for this example. The module's
other pages retain their existing browser dependencies.

## Routes And Source

| Route | Result |
| --- | --- |
| `/unpoly-demo` | Full page with the initial panel. |
| `/unpoly-demo/loaded` | Full page with the loaded panel; ordinary link fallback. |
| `/fragments/unpoly-demo` | Only the loaded panel HTML, with `X-Demo-Frontend: unpoly`. |

All three are defined in
`hyperbricks/85-unpoly-fragment-demo.hyperbricks.yaml`.
`unpoly_demo_panel` owns the shared HTML; `unpoly_demo_loaded_panel` changes
only its values. The page and fragment each inherit that loaded panel.

## Browser Contract

```html
<a href="/unpoly-demo/loaded" up-href="/fragments/unpoly-demo"
   up-follow up-target="#unpoly-panel" up-history="false"
   up-cache="false" up-fallback="false">Load fragment</a>
```

Unpoly fetches `up-href` and replaces the selected element with the matching
element in the response. The response therefore includes
`<section id="unpoly-panel">`. Disabling history keeps the full-page URL;
disabling fallback makes a missing target fail visibly. These attributes follow
the [documented `up-follow` contract](https://unpoly.com/up-follow).

Without JavaScript, the link opens `/unpoly-demo/loaded`, which renders the same
loaded panel in a complete page. HyperBricks uses separate route owners for the
page and fragment, and makes no client-specific routing decision.

## Verify The Integration

1. Open `/unpoly-demo` and enter a value in the input outside the panel.
2. Click **Load fragment**. The heading changes from **Initial panel** to
   **Fragment loaded**, the input value survives, and the URL remains
   `/unpoly-demo`.
3. Inspect the fragment request: status `200`,
   `X-Demo-Frontend: unpoly`, and HTML containing the matching panel. No `HX-*`
   response header is configured. Open `/unpoly-demo/loaded` directly to check
   the complete-page fallback.

This example demonstrates fragment replacement with Unpoly. It does not claim
an adapter for Unpoly overlays, authentication, or client-side routing; those
flows need their own application configuration.
