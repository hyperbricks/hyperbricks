# Plugin vs API Route Split

## Summary

This pattern explains one of the first practical decisions a builder developer has to make:

- use `<API_FRAGMENT_RENDER>` when the route mostly forwards user input to a backend and renders the returned data
- use `<PLUGIN>` when the route has to think first: validate, normalize, branch, or build a small workflow result before rendering

The point is not that plugins are “better.” The point is that they solve a different kind of problem.

## Files

- Config: `hyperbricks/80-plugin-vs-api-route-split.hyperbricks.yaml`
- Page template: `templates/patterns/route-split-demo.html`
- API result template: `templates/patterns/route-split-api-result.html`
- Plugin result template: `templates/patterns/route-split-plugin-result.html`
- Plugin: `plugins/route-split-demo/1.0.0/route_split_demo_plugin.go`
- Mock PostgREST responses:
  `templates/patterns/mock-postgrest-rename-success.json`
  `templates/patterns/mock-postgrest-rename-conflict.json`

## Routes

- Page:
  - `/plugin-vs-api-route-split`
- API-side actions:
  - `/patterns/route-split/rename-success`
  - `/patterns/route-split/rename-conflict`
- Mock API fragments:
  - `/fragments/mock-postgrest-rename-success`
  - `/fragments/mock-postgrest-rename-conflict`
- Plugin-side action:
  - `/patterns/route-split/import`
- Refresh probe:
  - `/fragments/route-split-refresh-probe`

## Pattern rule

Start with this simple question:

- is this route mostly passing data through to a backend owner?
- or does this route need local decision-making before a result exists?

Use `<API_FRAGMENT_RENDER>` when:

- the route is mostly request mapping
- the real owner of the action is an upstream API or PostgREST RPC
- the returned data can be rendered directly by a template

Use `<PLUGIN>` when:

- the route has validation rules beyond simple field mapping
- the request needs preprocessing or orchestration
- the result is better described as a computed plan, a workflow outcome, or a derived response

## Why the API side is mocked

The API branch still uses the real `endpoint`, `body`, `template`, and `response.hx_trigger` behavior, but the upstream JSON is served by local fragment routes. That keeps the demo easy to run and inspect without needing a live PostgREST service first.

## What the page is trying to teach

The left side is intentionally boring:

- browser fields go in
- HyperBricks remaps them
- backend-style JSON comes back
- a template shows the result

The right side is intentionally not boring:

- input must be checked
- tags are normalized
- a plan is created
- the template shows that computed plan

That difference is the boundary.
