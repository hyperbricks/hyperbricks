# Native esbuild demo

This module contains a working estimate calculator, imported TypeScript and CSS, source maps, and development watch configuration.

## Development

From the repository root, using a HyperBricks build containing the native component:

```sh
hyperbricks start -m esbuild-demo
```

Open <http://localhost:8097/>. Adjust quantities or VAT and check the estimate. The calculation is browser-side TypeScript bundled by embedded esbuild, not server-side JavaScript execution. No plugin, Tailwind, Node runtime, or npm install is needed to run this module.

- `hyperbricks/page.hyperbricks.yaml` declares independent JS and CSS builds and inherits them into the page head.
- `resources/js/main.ts` imports the calculation from `calculate.ts`.
- `resources/css/site.css` imports `tokens.css` for the calculator's color tokens.
- Both entries use `fingerprint: true`. The first uncached page request creates `static/js/app.<hash>.js`, `static/css/site.<hash>.css` and their source maps. Later renders reuse valid builds, including after restart when the private persistent manifest and its recorded files validate.
- Editing a resource triggers the existing development reload; refreshing the page rebuilds on demand. Generated static assets do not trigger a reload loop.
- Set a component's `cache: false` to rebuild it on every render. The page has `nocache: true` so HTML caching cannot bypass component rendering in this demo.
- Changed generated content produces a new asset URL. Previous versions remain available; there is no automatic cleanup. Set `fingerprint: false` for fixed names.

The `static` and `rendered` directories are generated and ignored. Source maps include source text; turn them off before publishing if that is undesirable.

See [ESBUILD.md](../../docs/ESBUILD.md) for options, migration, and limitations.

## Optional Developer Access

The calculator works without developer credentials. Spaces also opens without
login when both developer credentials are absent. To require a developer login
and use render diagnostics while Dashboard is disabled, choose your own password
and export these values in the same terminal before starting the module from the
repository root:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
hyperbricks start -m esbuild-demo
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

[Render diagnostics](http://localhost:8097/__hyperbricks/render-diagnostics) require
credentials while Dashboard is disabled. [Spaces](http://localhost:8097/__hyperbricks/spaces)
uses the same login when configured and opens without login when both values are
absent. Spaces and its writes default to enabled in development and debug mode;
set `hyperbricks.development.frontend_editing.spaces.write: false` for read-only
access. To also open the
[Dashboard](http://localhost:8097/__hyperbricks/dashboard), set
`hyperbricks.development.dashboard.enabled: true` in
`package.hyperbricks.yaml` and restart. See
[development configuration](../../docs/SPACES.md#development-configuration) for
the shared access settings, host restrictions, and network setup.

## Static export

```sh
hyperbricks static -m esbuild-demo --force --zip
```

The deployable site is in `modules/esbuild-demo/rendered/`, with the zip in the usual export directory. Deploy only the contents of `rendered/`. No running HyperBricks process or resources directory is needed at the destination. Source-map `sources` paths identify the original filenames; `sourcesContent` embeds the original CSS and TypeScript inside each map, so those filenames need not be public files.
