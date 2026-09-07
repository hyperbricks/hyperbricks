# MENU + HTMX Demo

## Summary

This pattern shows how to keep normal page links while making them feel faster with HTMX.

- each item points to a canonical page route
- the same route is used as the HTMX fetch source
- HTMX selects only the content panel from the full page response
- normal browser navigation still works through `href`

## Files

- Config: `hyperbricks/50-menu-htmx-demo.hyperbricks.yaml`
- Shell template: `templates/patterns/menu-htmx-shell.html`
- Content templates:
  `templates/patterns/menu-htmx-intro.html`
  `templates/patterns/menu-htmx-doc1.html`
  `templates/patterns/menu-htmx-doc2.html`
  `templates/patterns/menu-htmx-doc3.html`

## Routes

- `/menu-demo`
- `/menu-demo/doc-1`
- `/menu-demo/doc-2`
- `/menu-demo/doc-3`

## Pattern rule

Use this when:

- the menu items already have real page routes
- you want HTMX-enhanced in-page transitions
- you want `href` to remain the progressive-enhancement fallback

The core trick is in the `<MENU>.item` template:

- `href` stays pointed at the canonical page
- `hx-get` fetches that same canonical page
- `hx-select="#menu-demo-panel > *"` extracts only the content panel
- `hx-target="#menu-demo-panel"` swaps in place
- `hx-push-url` updates the browser URL to the canonical destination

The link explicitly uses `hx-swap="innerHTML"` to preserve the panel.
Its `hx-select-oob` selection replaces the
sibling sidebar after the main content swap, keeping the menu's active state
in sync. Both selections are declared on each request link, so this flow does
not rely on inherited attributes.

## Use a fragment instead when

Use explicit fragment endpoints instead of this pattern when the menu needs:

- much smaller payloads than the page route returns
- request-time behavior that is different from the full page route
- a separate partial contract for authenticated or API-driven content
