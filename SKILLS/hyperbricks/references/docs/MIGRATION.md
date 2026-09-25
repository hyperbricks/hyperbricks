<!-- Generated from docs/MIGRATION.md. Do not edit directly. -->

# Migration Guide

Use this guide when updating an older configuration to the current component contracts. Apply only the changes that affect your project. The examples describe configuration changes; they do not assign every change to a particular release interval.

## Upgrade from v1.2.4-beta to v1.2.5-beta

### Replace the dashboard Boolean and configure developer access

In `package.hyperbricks.yaml`, replace the old Boolean:

```yaml
hyperbricks:
  development:
    dashboard: true
```

with an explicit mapping:

```yaml
hyperbricks:
  development:
    dashboard:
      enabled: true
      credentials:
        user:
          env: HB_DEVELOPER_USER
        password:
          env: HB_DEVELOPER_PASSWORD
```

Set both environment variables before starting the module. For a previously
disabled dashboard, use `enabled: false`; `dashboard: false` is also rejected.
The shared credentials protect Dashboard, Errors, and Spaces. Missing or empty
credentials leave enabled developer interfaces locked with HTTP 503; incorrect
or absent browser credentials receive an HTTP 401 Basic Auth challenge once the
account is configured. These are module credentials, separate from deployment
service credentials. Use HTTPS or a private encrypted connection for non-loopback
access because Basic Auth does not encrypt credentials.

Restart the module after changing its settings, and update the
[dashboard URL](#update-developer-dashboard-links). See
[Spaces configuration](SPACES.md#development-configuration) and
[developer access settings](DEPLOY.md#developer-access-settings) for the separate
interface enablement controls.

### Update deployment commands and configuration

The old commands and flags have no compatibility aliases. Update scripts and
service definitions using this mapping:

| Previous command | Current command |
| --- | --- |
| `hyperbricks deploy-daemon` or `hyperbricks start --deploy-remote` | `hyperbricks deploy remote` |
| `hyperbricks start --deploy-local` | `hyperbricks deploy local` |
| `hyperbricks start --deploy -m demo` | `hyperbricks deploy run -m demo` |
| `hyperbricks start --deploy-init-config local` or `remote` | `hyperbricks deploy init` |

Move archive-selection options such as `--build` and `--deploy-dir` to
`deploy run`. `start` now serves source modules only. Select a service's
deployment configuration with `--config`, then `HB_DEPLOY_CONFIG`, or the default
`deploy.hyperbricks.yaml` in the invocation directory. Service startup no longer
creates a missing configuration; `deploy init` creates one and refuses to
overwrite an existing file.

Update the existing deployment YAML as well; these old fields are rejected:

| Previous field | Required change |
| --- | --- |
| `deploy.hmac_secret` | Configure `deploy.remote.hmac_secret` on the receiver and `deploy.client.targets.<name>.hmac_secret` on each applicable client target. |
| `deploy.remote.api_enabled` | Remove it; `deploy remote` selects the service. |
| `deploy.remote.api_bind` / `api_port` | Rename to `deploy.remote.bind` / `port`. |
| `credentials.pass` | Rename to `credentials.password` in its owning role or target. |

Local runtime settings now belong to `deploy.local`; they are not inherited
from `deploy.remote`. Configure `credentials.user` and `credentials.password`
separately under `deploy.local`, `deploy.remote`, and each client target you use.
There are no default credentials: a service without both values starts locked
with HTTP 503, and an incomplete client target cannot push or sync. Remote
operations still require HMAC signing in addition to the new Basic Auth login.
For keyed signing, retain the matching `key_id` and scoped server secret; the
client's signing secret is now selected explicitly from its target configuration.

Use the [complete deployment configuration](DEPLOY.md#complete-configuration-example)
and [authentication rules](DEPLOY.md#authentication) to migrate only the roles
you run. Keep deployment logins separate from the module developer login above.

### Review saved deployment modes before restarting builds

For existing build indexes, a valid `runtime_mode` takes precedence. Without a
valid mode, `production: true` becomes Live; `production: false` **or an omitted
production field now becomes Development**, instead of inheriting the package's
mode. Review saved builds and explicitly select Live in the deployment interface
where production behavior is intended before starting or restarting them.

For a direct archive launch, choose explicitly with
`hyperbricks deploy run -m demo --mode live` (or `--mode development`).
`--production` remains a Live-mode alias and cannot be combined with
`--mode development`. See [legacy build-index migration](DEPLOY.md#legacy-build-index-migration)
and [running a deployment build](DEPLOY.md#run-a-deployment-build).

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

The URL change does not replace the dashboard configuration and authentication
migration [above](#replace-the-dashboard-boolean-and-configure-developer-access).
The deployment interfaces show **Open dashboard** only for a running Development
process. The action is disabled when the module dashboard is not enabled or
unavailable. Live builds do not expose this action.

## Check the updated routes

Request the affected pages, fragments, and actions. Check successful and rejected requests, guard denials, response headers, and rendered feedback. If you changed HTMX behavior, also check direct access, reload, Back, and Forward.

For a static project, rebuild the export. For native plugins, rebuild against the runtime you will run and restart the server. Use [Troubleshooting](TROUBLESHOOTING.md) to locate any configuration or render errors.
