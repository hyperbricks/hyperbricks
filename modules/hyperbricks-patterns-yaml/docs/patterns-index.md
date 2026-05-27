# HyperBricks Patterns Documentation

This page is the documentation landing page for the `hyperbricks-patterns` module.

Use it in two ways:

- open a demo route and click around
- read the short explanation first, then jump into the live demo

## Recommended order

If you are new to the module, this order has the gentlest learning curve:

1. [HTMX canonical page + fragment demo](/status-demo)
2. [MENU + HTMX demo](/menu-demo)
3. [Config-driven section rail](/section-rail-demo)
4. [API fragment write demo](/api-fragment-write-demo)
5. [Single plugin, many actions](/single-plugin-actions-demo)
6. [Plugin vs API route split](/plugin-vs-api-route-split)
7. [Guarded page demo](/guarded-demo)

## Pattern index

### 1. HTMX canonical page + fragment demo

- Live demo: [open](/status-demo)
- Notes: keep the browser URL for real pages, and use separate fragment routes for partial HTMX updates.

### 2. MENU + HTMX demo

- Live demo: [open](/menu-demo)
- Notes: keep real page links in the menu, then layer HTMX on top for faster in-page transitions.

### 3. Config-driven section rail

- Live demo: [open](/section-rail-demo)
- Notes: define rail behavior in data, then let the template derive links, targets, and subsection anchors.

### 4. API fragment write demo

- Live demo: [open](/api-fragment-write-demo)
- Notes: good for “submit form -> backend answer -> render feedback” flows.

### 5. Single plugin, many actions

- Live demo: [open](/single-plugin-actions-demo)
- Notes: good when one multi-step workflow should stay owned by one plugin, while routes remain explicit.

### 6. Plugin vs API route split

- Live demo: [open](/plugin-vs-api-route-split)
- Notes: use this to learn when a route should mostly forward data to a backend and when it should run richer local decision-making first.

### 7. Guarded page demo

- Live demo: [open](/guarded-demo)
- Notes: shows public login, protected page, forbidden page, and a native route guard working together.

## Quick glossary

### Canonical page route

The real page URL a user can bookmark, reload, or open directly.

### Fragment route

A route meant for partial page updates, often used by HTMX.

### Plugin

A Go-based renderer or handler that can validate input, branch, compute values, and hand the result back into the HyperBricks pipeline.

### `API_FRAGMENT_RENDER`

A route shape that is useful when the page mostly forwards data to another service and renders the returned response.
