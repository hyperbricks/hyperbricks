# MENU + HTMX Demo

Generate a menu from `hypermedia` configuration and use HTMX to update the content panel and sidebar without reloading the whole document. Each destination keeps a full-page URL that users can open directly, bookmark, or reload.

## How it works

The `menu` component generates navigation from the `section`, `index`, `route`, and `title` fields of configured `hypermedia` components. In this example, it selects components in `section: menu_htmx_demo_pages` and sorts them by `index`. Its `item` template adds HTMX attributes to links for other pages; its `active` template marks the current page with `aria-current="page"`.

See [HyperBricks Component Reference: menu](../../../../docs/REFERENCE.md#menu) for the component fields and options.

When a user selects another page:

1. HTMX requests the full page at the link's `hx-get` URL.
2. `hx-select="#menu-demo-panel > *"` selects the children of the response's content panel.
3. `hx-target="#menu-demo-panel"` and `hx-swap="innerHTML"` replace the current panel's contents, preserving the panel element.
4. `hx-select-oob="#menu-demo-sidebar-shell:outerHTML"` also replaces the sidebar with the destination page's sidebar, including its active menu item.
5. `hx-push-url` adds the destination URL to browser history.

The server returns a complete HTML document for every destination. HTMX selects the parts to update in the browser; this pattern does not reduce the response to a fragment.

## Link example

This illustrates the generated link to Document 1:

```html
<a href="/menu-demo/doc-1"
   hx-get="/menu-demo/doc-1"
   hx-select="#menu-demo-panel > *"
   hx-select-oob="#menu-demo-sidebar-shell:outerHTML"
   hx-target="#menu-demo-panel"
   hx-swap="innerHTML"
   hx-push-url="/menu-demo/doc-1">DOCUMENT 1</a>
```

Without JavaScript, `href` opens the complete page. Each request link declares its own selection, target, and swap attributes, so the update does not depend on inherited attributes.

## Files and routes

- Configuration: `hyperbricks/50-menu-htmx-demo.hyperbricks.yaml`
- Shared layout: `templates/patterns/menu-htmx-shell.html`
- Content: `templates/patterns/menu-htmx-intro.html`, `templates/patterns/menu-htmx-doc1.html`, `templates/patterns/menu-htmx-doc2.html`, and `templates/patterns/menu-htmx-doc3.html`

Open `/menu-demo`, `/menu-demo/doc-1`, `/menu-demo/doc-2`, or `/menu-demo/doc-3` in the running module.

## When to use separate fragments

Use this pattern when the same page response can serve both direct navigation and panel updates. Use separate fragment routes when you need smaller responses or different rendering or access rules for a partial update.

## Check the behavior

Select another menu item and check that the content, active menu item, and browser URL change together. Reload that URL to verify direct page access, then check Back and Forward. Disable JavaScript and confirm that the links still open complete pages.
