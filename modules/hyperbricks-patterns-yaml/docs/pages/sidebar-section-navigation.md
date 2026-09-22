# Build sidebar navigation with section links

Build a sidebar from configured navigation entries. Each link loads a fragment into the content panel and updates the browser URL to the corresponding full page. An entry can also list subsections: selecting one loads its panel, then scrolls to the selected heading.

## Configure the navigation

The `rail_items` mapping supplies navigation values to the shared template. For example, the Assets entry defines two subsection links:

```yaml
rail_items:
  - "1_assets":
      target: right_column
      fragment: rail-assets
      label: Assets
      sub_section:
        - "#rail-assets-uploads"
        - "#rail-assets-files"
      sub_labels:
        - Uploads
        - Files
```

| Field | Purpose |
| --- | --- |
| `target` | ID of the element whose contents HTMX replaces, without `#`. |
| `fragment` | Name used to derive both the page URL and fragment URL. |
| `label` | Sidebar label for the entry. |
| `sub_section` | Optional list of anchors, including `#`, that match IDs in the panel HTML. |
| `sub_labels` | Labels for those anchors, in the same order. Supply both subsection lists together with matching lengths. |

A `hypermedia` component passes this mapping to its content template through `values.items`:

```yaml
  - content:
      - values:
          items:
            - inherit: rail_items
```

The mapping supplies data. The template generates the links; this example does not use the `menu` component or generate navigation from `hypermedia` route metadata.

## How a link updates the panel

For the Assets **Files** subsection, the template produces a link with these attributes:

```html
<a href="/rail-assets#rail-assets-files"
   hx-get="/fragments/rail-assets"
   hx-trigger="click"
   hx-target="#right_column"
   hx-swap="innerHTML"
   hx-push-url="/rail-assets#rail-assets-files"
   data-outline-link>Files</a>
```

1. HTMX requests `/fragments/rail-assets` and replaces the contents of `#right_column`, preserving that panel element.
2. `hx-push-url` adds `/rail-assets#rail-assets-files` to browser history.
3. The browser helper waits for `htmx:after:settle`, then scrolls the panel to the element with `id="rail-assets-files"`.

`href` and `hx-push-url` include the full page path and anchor. Without JavaScript, the link opens the complete Assets page at that section. The helper also handles anchored direct access and history restoration.

The template derives URLs from `fragment`, but the matching `hypermedia` and `fragment` routes must be declared separately in YAML. Each full page and its fragment reuse the same panel content. See [Routing](../../../../docs/ROUTING.md) for route ownership.

## Add another entry

Add its navigation values to `rail_items`, define the matching full-page and fragment routes, and give any subsection headings the configured IDs. The full page must pass `rail_items` into `values.items` and render its panel inside the shared layout.

The scrolling helper in this demo specifically uses `#right_column`. If you change the target ID, update the helper as well as the navigation configuration and layout.

## Files and routes

- Configuration: `hyperbricks/60-config-driven-section-rail.hyperbricks.yaml`
- Shared layout and link generation: `templates/patterns/section-rail-shell.html`
- Panel templates: `templates/patterns/section-rail-builder.html`, `templates/patterns/section-rail-assets.html`, and `templates/patterns/section-rail-status.html`
- Subsection scrolling: `resources/js/patterns-ui.js`

Open `/section-rail-demo` for the landing page. The section pages are `/rail-builder`, `/rail-assets`, and `/rail-status`; their fragment URLs have the `/fragments/` prefix.

## Check the behavior

Select **Files** under **Assets**. Check that the Assets panel loads, the panel scrolls to Files, and the URL becomes `/rail-assets#rail-assets-files`. Reload, then check Back and Forward. Disable JavaScript and confirm that the same link opens the complete Assets page at Files.
