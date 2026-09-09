# API Fragment Write Demo

## Summary

This pattern shows a common builder flow in plain terms:

- a user submits a form
- HyperBricks reshapes that input for the backend
- the backend answers with data
- a template turns that data into visible feedback
- the route can also tell other parts of the page to refresh

The advanced HyperBricks term for that route shape is `<API_FRAGMENT_RENDER>`, but the important idea is simpler: this is a good pattern when the route mostly forwards data and renders the backend answer.

## Files

- Config: `hyperbricks/40-api-fragment-write-demo.hyperbricks.yaml`
- Page template: `templates/patterns/api-fragment-write-demo.html`
- Result template: `templates/patterns/api-file-write-result.html`
- Mock responses: `templates/patterns/mock-postgrest-file-save-success.json` `templates/patterns/mock-postgrest-file-save-conflict.json`

## Routes

- Page:
  - `/api-fragment-write-demo`
- Write actions:
  - `/patterns/assets/file-save-success`
  - `/patterns/assets/file-save-conflict`
- Mock upstream fragments:
  - `/fragments/mock-postgrest-file-save-success`
  - `/fragments/mock-postgrest-file-save-conflict`
- Secondary refresh fragment:
  - `/fragments/api-fragment-write-refresh-probe`

## Pattern rule

Keep these responsibilities separate:

- the page owns the form and where the result should appear
- the route owns the “translate these fields for the backend” step
- the backend owns the actual save logic
- the result template owns the user-facing feedback

## Why the mock endpoints are fragments

This demo keeps everything inside `hyperbricks-patterns`, so the upstream API is simulated by two local fragment routes that return JSON payloads:

- success-shaped array payload
- error-shaped object payload

That lets you learn the pattern without needing a real PostgREST server running first.

## Limitation of the mock

The local mock fragments return HTTP `200`, so this demo primarily shows response-shape handling.

Real PostgREST errors may return `4xx` or `5xx`. In that case `<API_FRAGMENT_RENDER>` also exposes the upstream status as `.Status` inside the result template.
