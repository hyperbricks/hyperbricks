# Config-Driven Section Rail

## Summary

This pattern shows how to build a left rail from data instead of hand-writing every link.

- navigation is defined as structured config, not handwritten markup
- the template derives canonical routes and fragment routes from each config row
- each row can optionally define subsection anchors
- the same config also determines the HTMX swap target

## Files

- Config: `hyperbricks/60-config-driven-section-rail.hyperbricks`
- Shell template: `templates/patterns/section-rail-shell.html`

## Routes

- Page routes:
  - `/section-rail-demo`
  - `/rail-builder`
  - `/rail-assets`
  - `/rail-status`
- Fragment routes:
  - `/fragments/rail-builder`
  - `/fragments/rail-assets`
  - `/fragments/rail-status`

## Config shape

Each rail item uses the same shape:

- `target`
- `fragment`
- `label`
- optional `sub_section`
- optional `sub_labels`

That is the important pattern. The template then derives:

- `href="/<fragment>"`
- `hx-get="/fragments/<fragment>"`
- `hx-target="#<target>"`

## When to use it

Use this when:

- one shell owns several section panels
- the rail should be data-driven
- subsection anchors should stay coupled to the section definition

## Avoid it when

Avoid this pattern when each menu item needs unrelated routing logic or radically different UI behavior. In that case, a plain custom template may be clearer than forcing everything through one config schema.
