# Single Plugin, Many Actions

## Summary

This pattern shows how to build a multi-step flow without scattering the logic everywhere.

- each route has a clear name
- all routes point to the same plugin
- `data.action` selects the workflow branch
- the plugin owns the state machine instead of spreading logic across many small plugins
- the plugin returns a `<TEMPLATE>` handoff with a values map, following the module's template-config pattern

In plain language: the URL structure stays easy to read, but one piece of code still owns the “what should happen next?” decisions.

## Files

- Config: `hyperbricks/70-single-plugin-many-actions.hyperbricks.yaml`
- Shell template: `templates/patterns/single-plugin-actions-shell.html`
- Stage template: `templates/patterns/workflow-actions-stage.html`
- Plugin: `plugins/workflow-actions-demo/1.0.0/workflow_actions_demo_plugin.go`

## Routes

- Page:
  - `/single-plugin-actions-demo`
- Plugin action routes:
  - `/workflow-actions-demo/landing`
  - `/workflow-actions-demo/lookup`
  - `/workflow-actions-demo/signup`
  - `/workflow-actions-demo/complete`

## Pattern rule

Use this when:

- one workflow has several related endpoints
- route names should stay explicit
- plugin ownership should stay cohesive

The important part is that HyperBricks still names each route separately, but the plugin stays the single owner of the workflow logic. The plugin does not own the HTML markup directly. It prepares stage values and hands them to a preloaded template, which keeps the rendering pattern aligned with the rest of this module.

## Avoid it when

Avoid this pattern when the actions are unrelated in ownership or lifecycle. In that case, separate plugins or `API_FRAGMENT_RENDER` endpoints are usually clearer.
