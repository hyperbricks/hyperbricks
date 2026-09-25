# Migration Guide

Use this guide when updating an older configuration to the current component contracts. Apply only the changes that affect your project. The examples describe configuration changes; they do not assign every change to a particular release interval.

## Use YAML component definitions

Since `v1.2.0-beta`, HyperBricks uses YAML source files. Earlier internal configuration formats are no longer supported.

Write component definitions as ordered sequences and keep package settings as ordinary mappings. See [YAML Usage: Component Source Shape](YAML_USAGE.md#component-source-shape).

## Replace HTMX-specific response fields

Move removed `response.hx_*` fields to literal HTTP headers under `response.headers`.

Before:

```yaml
response:
  hx_retarget: "#status"
  hx_reswap: outerHTML
  hx_trigger: statusUpdated
```

After:

```yaml
response:
  headers:
    HX-Retarget: "#status"
    HX-Reswap: outerHTML
    HX-Trigger: statusUpdated
```

Removed fields are rejected. Update inherited base definitions too. Browser attributes such as `hx-get`, `hx-target`, and `hx-swap` remain in your templates.

For API components, top-level `headers` configures the upstream request; `response.headers` configures the browser response. Keep API credentials in upstream request settings.

See [HTTP Responses: Migrate Existing Configuration](HTTP_RESPONSES.md#migrate-existing-configuration) for the complete field mapping and examples.

## Configure guard denial responses explicitly

Replace the old `redirect`, `hx_redirect`, and flat denial `status` settings with `default` and optional `variants`. For an ordinary browser redirect:

```yaml
on_unauthenticated:
  default:
    status: 303
    headers:
      Location: /login
```

Add a request-header variant when another client needs a different denial response. The first matching variant replaces the whole default response, including its status and headers. HyperBricks does not automatically convert a browser redirect to `HX-Redirect`.

Keep the authentication and authorization rules intact. See [HTTP Responses: Guard Denials](HTTP_RESPONSES.md#htmx-example-guard-denials) for a complete before-and-after example.

## Review HTMX 4 event timing

HTMX 4 removed `HX-Trigger-After-Swap` and `HX-Trigger-After-Settle`. Use `HX-Trigger` for application events. If your handler needs the updated DOM, listen for `htmx:after:swap`; if it needs settling to finish, use `htmx:after:settle`.

These are browser-library changes. `HX-Trigger` does not preserve the old headers' timing, and HyperBricks's generic HTTP headers do not enforce it. See [HTTP Responses: Fragment And API Fragment Headers](HTTP_RESPONSES.md#htmx-example-fragment-and-api-fragment-headers).

## Select API credentials explicitly

API components no longer forward the browser's `token` cookie implicitly. Add a named cookie only when the configured endpoint should receive it:

```yaml
forwardtoken: token
```

Leave `forwardtoken` omitted for public data or service-authenticated calls. It accepts a cookie-name string, not a Boolean. Choose one authentication source per component; remove competing credentials.

Credential-bearing requests require HTTPS. The local plain-HTTP exception applies only to literal loopback addresses in development or debug mode. Cross-origin API redirects are rejected. See [API Render: Authentication](API_RENDER.md#authentication) and [Migration from implicit forwarding](API_RENDER.md#migration-from-implicit-forwarding).

## Update developer dashboard links

The developer Dashboard now lives at `/__hyperbricks/dashboard`, alongside
`/__hyperbricks/errors` and `/__hyperbricks/spaces`. Update bookmarks and custom
developer-navigation links that used `/dashboard`. There is no redirect alias:
`/dashboard` is available for an application-owned route.

Dashboard enablement and developer Basic Auth are unchanged. The deployment
interfaces show **Open dashboard** only for a running Development process. The
action is disabled when the module dashboard is not enabled or unavailable.
Live builds do not expose this action.

## Check the updated routes

Request the affected pages, fragments, and actions. Check successful and rejected requests, guard denials, response headers, and rendered feedback. If you changed HTMX behavior, also check direct access, reload, Back, and Forward.

For a static project, rebuild the export. For native plugins, rebuild against the runtime you will run and restart the server. Use [Troubleshooting](TROUBLESHOOTING.md) to locate any configuration or render errors.
