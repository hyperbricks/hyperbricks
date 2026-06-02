# HTMX Canonical Page + Fragment Demo

## Summary

This example shows an important rule for HTMX pages: the browser URL and the fragment URL do not have to be the same thing.

It demonstrates:

- distinct canonical page routes for each view
- one rooted fragment route for reusable section content
- explicit fragment endpoints for HTMX panel swaps
- `hx-push-url` preserving the canonical page URL
- a plugin-rendered panel integrated into the same flow

## Composer reference shape

Composer uses:

```yaml
app_status:
  - inherit: app_section
  - route: status
  - template_10:
      - values:
          content_right:
            - inherit: status.template_10
```

and the reusable section source:

```yaml
status:
  - type: fragment
  - route: fragments/status
  - template_10:
      - type: tree
```

It also exposes explicit request-time partial endpoints in YAML source, such as:

- `project/status/summary`
- `project/status/settings`
- `project/status/invites`

## Pattern demo in this module

Files:

- Config: `hyperbricks/20-htmx-canonical-fragment-demo.hyperbricks.yaml`
- Landing page: `hyperbricks/10-template-config-plugin.hyperbricks.yaml`
- Shell template: `templates/patterns/layout-shell.html`
- Section template: `templates/patterns/status-demo.html`
- Panel fragments:
  `templates/patterns/status-overview.html`
  `templates/patterns/status-summary.html`
  `templates/patterns/status-settings.html`

Routes:

- Canonical pages:
  - `/status-demo`
  - `/status-demo/summary`
  - `/status-demo/settings`
  - `/status-demo/plugin`
- Reusable section fragment: `/fragments/status-demo`
- Explicit panel fragments:
  - `/fragments/status-demo-summary`
  - `/fragments/status-demo-settings`
  - `/fragments/status-demo-plugin`

## Rule

Do not let the full page and the fragment fight over the same route.

Use:

- a canonical page route for each real view the user can land on
- rooted fragment route for reusable section content
- explicit fragment endpoints for HTMX requests

That keeps ownership clear and matches the preferred Composer pattern.
