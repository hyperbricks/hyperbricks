# HTMX Canonical Page + Fragment Demo

## Summary

This example shows an important rule for HTMX pages: the browser URL and the fragment URL do not have to be the same thing.

It demonstrates:

- distinct canonical page routes for each view
- one rooted fragment route for reusable section content
- explicit fragment endpoints for HTMX panel swaps
- `hx-push-url` preserving the canonical page URL
- a plugin-rendered panel integrated into the same flow

The shared browser script uses HTMX 4. Each link declares `hx-swap="innerHTML"` to preserve `#status-demo-panel`. The URL diagnostic observes `htmx:after:history:update` and `htmx:after:swap`; the last-request diagnostic uses `event.detail.ctx.request.action` before the request and after the swap. Updating it after the swap preserves the actual request URL when a history restore replaces the diagnostic itself. It shows **No HTMX request yet** on initial page access and the canonical page URL for an HTMX history request.

Check direct page access, panel navigation, reload, Back, and Forward. History restoration requests the canonical full page, so each pushed URL must keep its complete-page route.

## Configuration in this module

The canonical page inserts the shared section into the page shell:

```yaml
app_status_demo:
  - inherit: status_demo_shell
  - route: status-demo
  - content:
      - values:
          content_right:
            - inherit: status_demo_base_view.content
```

The reusable section has its own fragment route:

```yaml
status_demo:
  - type: fragment
  - route: fragments/status-demo
  - nocache: "true"
  - "10_10":
      - inherit: status_demo_base_view.content
```

The summary, settings, and plugin links fetch separate panel fragments and push their corresponding full-page URLs. Their canonical pages select the same panel through inherited template values.

## Pattern demo in this module

Files:

- Config: `hyperbricks/20-htmx-canonical-fragment-demo.hyperbricks.yaml`
- Landing page: `hyperbricks/10-template-config-plugin.hyperbricks.yaml`
- Shell template: `templates/patterns/layout-shell.html`
- Section template: `templates/patterns/status-demo.html`
- Panel fragments: `templates/patterns/status-overview.html` `templates/patterns/status-summary.html` `templates/patterns/status-settings.html`

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

The complete page owns direct access and history restoration; the fragment owns the partial response.
