# SampleAPIs Coffee Static Demo

This module demonstrates the static snapshot path with a real nested `api_render`.

The route in `hyperbricks/coffee-static.hyperbricks.yaml` fetches `https://api.sampleapis.com/coffee/hot`, renders the JSON through `templates/coffee-cards.html`, and writes the final HTML to `rendered/index.html` when static rendering runs.

Static rendering this module requires internet access. The generated HTML can change when SampleAPIs changes its coffee data.

## Run It Dynamically

Run from the repository root:

```bash
go run ./cmd/hyperbricks start -m sampleapis-coffee-static
```

Open `http://localhost:8080/index.html`.

## Optional Developer Access

The public coffee page and static export work without developer credentials.
Spaces also opens without login when both developer credentials are absent. To
require a developer login and use render diagnostics while Dashboard is disabled,
choose your own password and export these values in the same terminal before
starting the module from the repository root:

```bash
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
go run ./cmd/hyperbricks start -m sampleapis-coffee-static
```

Replace the password placeholder before running the commands. By default, the
package reads these variables through
`hyperbricks.development.dashboard.credentials`; use
the same values for the browser's login prompt. There is no default account.
With both values absent, Spaces and enabled Dashboard views open without login
and startup warns. A partial account blocks access with `503`; a complete account
requires login. Restart the process after changing credentials or package settings.

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

[Render diagnostics](http://localhost:8080/__hyperbricks/render-diagnostics) require
credentials while Dashboard is disabled. [Spaces](http://localhost:8080/__hyperbricks/spaces)
uses the same login when configured and opens without login when both values are
absent. Spaces and its writes default to enabled in development and debug mode;
set `hyperbricks.development.frontend_editing.spaces.write: false` for read-only
access. To also open the
[Dashboard](http://localhost:8080/__hyperbricks/dashboard), set
`hyperbricks.development.dashboard.enabled: true` in
`package.hyperbricks.yaml` and restart. These tools belong to the running
development server and are not part of a static export. See
[development configuration](../../docs/SPACES.md#development-configuration) for
the shared access settings, host restrictions, and network setup.

## Render Static HTML

```bash
go run ./cmd/hyperbricks static -m sampleapis-coffee-static --force
```

Expected output:

- `modules/sampleapis-coffee-static/rendered/index.html`
- `modules/sampleapis-coffee-static/rendered/static/coffee.css`

## Render And Then Serve The Static Result

```bash
go run ./cmd/hyperbricks static -m sampleapis-coffee-static --serve
```

`--serve` rebuilds the snapshot, which calls the coffee API during rendering,
and then serves `modules/sampleapis-coffee-static/rendered`.

To serve an existing snapshot without fetching the API again, use a standalone
file server instead:

```bash
python3 -m http.server 8080 --directory modules/sampleapis-coffee-static/rendered
```

## What To Copy Into Another Module

- Put API-rendered page routes in `hyperbricks/*.hyperbricks.yaml`.
- Put HTML templates in `templates/`.
- Put CSS, images, or JavaScript in `static/`.
- Add explicit snapshot targets under `hyperbricks.static.routes` in `package.hyperbricks.yaml` when you want clear output filenames.

The key pattern is:

```yaml
coffee_menu:
  - type: api_render
  - endpoint: https://api.sampleapis.com/coffee/hot
  - method: GET
  - template:
      file: coffee-cards.html
```
