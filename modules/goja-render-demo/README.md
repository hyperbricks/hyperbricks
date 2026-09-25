# Native Goja Render Demo

This module runs server-side JavaScript through the native `goja_render`
component. It includes an availability check at `/` and a materials calculator at
`/materialenlijst`. No plugin or Node runtime is needed.

## Run It

From the repository root:

```sh
hyperbricks start -m goja-render-demo
```

Open the [availability check](http://localhost:8095/) or the
[materials calculator](http://localhost:8095/materialenlijst). Submit either
form to pass its allowed query values to a script under `resources/scripts/`.
The corresponding definitions in `hyperbricks/` render the returned values.
See [Server Scripts](../../docs/GOJA_RENDER.md) for the component contract.

## Optional Developer Access

The public pages work without developer credentials. To use the protected
development tools, choose your own password and export these values in the same
terminal before starting the module:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
hyperbricks start -m goja-render-demo
```

Replace the password placeholder before running the commands. By default, the
package reads these variables through `development.dashboard.credentials`; use
the same values for the browser's login prompt. There is no default account.
Missing values lock the developer tools while the public pages stay available.
Restart the process after changing credentials or package settings.

Developer credentials can also be set directly in `package.hyperbricks.yaml`.
Replace only `credentials` under `hyperbricks.development.dashboard`, leaving
`enabled` and the other settings unchanged:

```yaml
credentials:
  user: developer
  password: choose-a-long-password
```

With direct values, the environment exports above are not needed. Choose your
own password, restart the server, and use these values to log in. The password
is stored as plain text; do not commit real credentials to a shared repository.

[Render diagnostics](http://localhost:8095/__hyperbricks/render-diagnostics) and
the read-only [Spaces view](http://localhost:8095/__hyperbricks/spaces) share this
login even though the dashboard is disabled. To also open the
[Dashboard](http://localhost:8095/__hyperbricks/dashboard), set
`hyperbricks.development.dashboard.enabled: true` in
`package.hyperbricks.yaml` and restart. See
[development configuration](../../docs/SPACES.md#development-configuration) for
the shared access settings.
