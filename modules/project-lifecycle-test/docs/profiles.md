# Project lifecycle fixture profiles

These profiles are test inputs selected by [`scripts/test_project_lifecycle.py`](../../../scripts/test_project_lifecycle.py). They are documented here so a failing integration check can be reproduced directly. They are not a progressive tutorial.

All routes use the default module port `8104` when no `--port` override is supplied.

## Default rendering profile

Configuration: `package.hyperbricks.yaml`

The default profile verifies:

- Full pages at `/`, `/projects`, `/projects/atlas`, `/projects/beacon`, `/estimate`, and `/static-preview`
- Matching fragments below `/fragments/`
- A refreshable status fragment
- Native CSS and JavaScript builds
- A request-specific estimate implemented with `goja_render`
- Development reload and production rendering

The test copies `fixtures/about.hyperbricks.yaml` into the running temporary module and appends `fixtures/navigation-about.hyperbricks.yaml` to the navigation source. The resulting `/about` page and `/fragments/about` route prove that the watcher loads new source without making README prose part of the test contract.

## API and guard profile

Configuration: `package.api.hyperbricks.yaml`

Start the fixture API:

```sh
go run modules/project-lifecycle-test/tools/fixture-api/main.go -port 8099
```

Start the profile:

```sh
HYPERBRICKS_LIFECYCLE_API_URL=http://127.0.0.1:8099 \
  hyperbricks start -m project-lifecycle-test \
  --config package.api.hyperbricks.yaml
```

The fixture API binds to loopback and keeps one project in memory. Restarting it resets the state. The profile exercises these contracts:

| Route | Contract |
| --- | --- |
| `/advanced` | A complete page whose nested `api_render` performs `GET /api/project` |
| `/fragments/advanced-project` | A non-cacheable fragment that reuses the API-backed project panel |
| `/actions/advanced-project-save` | An `api_fragment_render` action that forwards selected form values to `POST /api/project` |
| `/actions/demo-login` | Sets the returned demo token in an `HttpOnly`, `SameSite=Lax` cookie |
| `/actions/demo-logout` | Expires the demo cookie |
| `/advanced/settings` | Uses the fixture authorization endpoint before rendering protected API data |

The write checks cover an accepted update, an empty-name validation result, and a stale-version conflict. `.Status` exposes upstream 200, 422, and 409 outcomes to the result template even though the rendered fragment response currently uses HTTP 200.

The fixed `demo-viewer` and `demo-owner` tokens are deterministic test data. They are not an authentication example for production applications.

## Native plugin profile

Configuration: `package.plugin.hyperbricks.yaml`

Build the plugin against local HyperBricks source:

```sh
HYPERBRICKS_LOCAL_PATH="$PWD" \
  go run ./cmd/hyperbricks plugin build lifecycle-test@1.0.0 \
  --module project-lifecycle-test
```

Start the profile:

```sh
hyperbricks start -m project-lifecycle-test \
  --config package.plugin.hyperbricks.yaml
```

The routes `/actions/advanced-name-preview` and `/actions/advanced-name-confirm` invoke the same plugin. The route configuration supplies `data.action`, so submitted form or query values cannot select the operation. The plugin returns a runtime `<TEMPLATE>` configuration and lets the normal HyperBricks renderer produce and escape the HTML.

The plugin is stateless. The confirmation result does not persist data.

## Static export profile

Configuration: `package.static.hyperbricks.yaml`

`tools/stage_source.py --static-profile` copies this configuration over the staged module's default package file. Its HyperBricks directory points only to `profiles/static/hyperbricks`, which exposes `index.html` and no application actions.

This separation verifies an important export property: `static.routes` adds targets but is not an allowlist for every discovered route. Selecting a source directory containing only static-ready pages prevents dynamic application routes from entering the export.

The integration check verifies the rendered HTML, local asset targets, and generated zip. It also verifies the complete dynamic profile separately as an `.hra` runtime archive.
