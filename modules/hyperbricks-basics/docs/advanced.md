# Optional lessons: APIs, guards, and plugins

Try these after the [basic walkthrough](../README.md). They extend Project Desk with a local project edit screen, protected settings, and a small Go plugin. The normal module does not load these routes or require their dependencies.

Run commands from the project root containing `modules/`. Use the compatible HyperBricks binary described in the main README. All example URLs below use the default module port, `8104`.

## Read and edit a project through an API

Prerequisite: Go. The supplied fixture uses only Go's standard library; it needs no database, Docker, external API, or credentials.

In one terminal, start the local teaching API:

```sh
go run modules/hyperbricks-basics/tools/fixture-api/main.go -port 8099
```

In another terminal, start the optional module configuration:

```sh
hyperbricks start -m hyperbricks-basics --config package.advanced.hyperbricks.yaml
```

Open [the API lesson](http://localhost:8104/advanced). The fixture listens only on `127.0.0.1`. It stores one shared sample project in memory; restarting it resets the name and version. The basic project pages keep their own local sample data. Press Ctrl+C in each terminal when finished.

If `8099` is occupied, start the fixture with another `-port` and point HyperBricks to it before startup:

```sh
HYPERBRICKS_BASICS_API_URL=http://127.0.0.1:8199 \
  hyperbricks start -m hyperbricks-basics --config package.advanced.hyperbricks.yaml
```

Keep the API URL without a trailing slash. This setting is resolved when the module loads, rather than from visitor input.

### Follow the request

The [lesson YAML](../lessons/advanced/app.hyperbricks.yaml) wires these parts:

| Part | Responsibility |
| --- | --- |
| `/advanced` | Full page with the initial project panel already rendered. |
| `advanced_project_panel` | Nested `api_render` reads `GET /api/project`. |
| `/fragments/advanced-project` | Reuses that panel when it needs refreshing. |
| `/actions/advanced-project-save` | `api_fragment_render` forwards the form to `POST /api/project`. |
| [Project panel template](../lessons/advanced/templates/project-panel.html) | Shows the name and version; submits both fields. |
| [Result template](../lessons/advanced/templates/save-result.html) | Shows `.Status` and `.Data.message` from the API. |
| [Feedback script](../lessons/advanced/feedback.js) | Emits `basics-project-updated` only after a successful save. |

The panel listens for that event with `hx-trigger`. It refreshes once when the save succeeds, with no polling. Both page and fragment are non-cacheable because this lesson reads changing data.

The feedback script listens for HTMX 4's `htmx:after:swap` and reads the request target from `event.detail.ctx.target`. It checks only the save-feedback panel's API-status marker, so rendering another panel cannot trigger a save refresh. The POST forms declare their own target and swap mode; HTMX submits their fields and the clicked role button as form data. Reloading the project is a separate GET and intentionally submits no unsaved form fields.

The API owns validation and the version check. YAML forwards selected input; templates present the result. `querykeys: []` blocks incoming query forwarding. Submitted form fields are a separate input source, so the API must still validate them. See [API Render](../../../docs/API_RENDER.md).

### Try success, validation, and conflict

1. Change the name and save. The API returns **200**, and the panel refreshes with the new name and version.
2. Clear the name and save. The API returns **422** and the project stays unchanged. The form intentionally permits an empty submission to show this.
3. Open `/advanced` in two tabs. Save in the first tab, then save in the second. Its old version receives **409**. Select **Reload project** before trying again; the failed save has not overwritten the other tab's change.

The fixture checks and updates the version while holding one lock. This keeps the teaching example consistent even when two saves arrive together. It is memory-only sample state, not durable storage.

The current runtime exposes upstream status through `.Status`, but the rendered `api_fragment_render` response itself is HTTP **200**, including upstream 422 and 409 results. The lesson therefore displays the **API status** explicitly and uses it to decide whether to refresh. Do not treat the outer HTTP 200 as proof that a save succeeded. The fixture itself returns real 200/422/409 responses.

There is also a current body-placeholder limitation: a name ending in a double quote, such as `Garden "team"`, can produce malformed upstream JSON and an API 400 response. The lesson shows that failure without changing the project. Use ordinary names for the main exercise; test your actual input shapes before adopting body placeholders in an application.

For a small exercise, change the allowed name length in the fixture and template, then repeat the validation case. Put the rule in the API first; the form limit only helps the person typing.

## Protect the settings route

With the same fixture and configuration running, open [demo roles](http://localhost:8104/advanced/login). Try these paths:

| Demo state | Opening `/advanced/settings` |
| --- | --- |
| No cookie, or an unknown token | Redirects to `/advanced/login`. |
| Viewer | Redirects to `/advanced/forbidden`. |
| Owner | Renders owner settings. |

The role buttons call `/actions/demo-login`. On a successful API response, HyperBricks sets a `token` cookie with `HttpOnly`, `SameSite=Lax`, and `Path=/`. The **Clear the demo session** button expires that cookie. Use the provided links after choosing or clearing a role to see the guard's result.

The page guard calls the fixture's `/auth/owner` endpoint before rendering its children. The settings API independently checks the token before returning data. A caller who bypasses the page and requests `/api/settings` directly still receives 401 or 403 unless the owner demo token is present. This shows why the route guard and the service's authorization check have separate jobs.

**These public, fixed role tokens are a teaching stub.** Anyone can select owner; there is no real login, session store, password, or identity validation. In a real project, connect the same guard pattern to your actual authentication and authorization service, use HTTPS cookies, and validate access in the service that owns the data. See [Route Guard](../../../docs/ROUTE_GUARD.md).

For a small exercise, change the forbidden page's explanation while keeping the API's access decision unchanged. Template text is presentation; hiding a button or changing a message does not grant access.

## One Go plugin, two explicit actions

This lesson does not need the fixture. It needs Go, a platform that supports native Go plugins, and a plugin built against the same runtime and compatible toolchain used to run the module. For local runtime development, use the matching HyperBricks source checkout. From the project root:

```sh
HYPERBRICKS_LOCAL_PATH=/absolute/path/to/hyperbricks \
  hyperbricks plugin build project-desk@1.0.0 --module hyperbricks-basics

hyperbricks start -m hyperbricks-basics \
  --config lessons/plugin/package.hyperbricks.yaml
```

Use your actual checkout path. `plugin build` reads `HYPERBRICKS_LOCAL_PATH`; the current command does not accept `--hyperbricks-path`. The build updates the plugin's Go dependency files and writes `bin/plugins/ProjectDeskPlugin__hyperbricks-basics@1.0.0.so`. Review those dependency changes before committing, especially a machine-specific local replacement. For an installed published release, leave `HYPERBRICKS_LOCAL_PATH` unset and run `hyperbricks plugin build project-desk@1.0.0 --module hyperbricks-basics` without the environment prefix. The plugin must match that installed runtime. See [Plugins](../../../docs/PLUGINS.md).

Stop any earlier server using port 8104, or supply `--port` to run this lesson on another port. Open [the plugin lesson](http://localhost:8104/advanced/workflow).

The [route file](../lessons/plugin/hyperbricks/app.hyperbricks.yaml) declares two actions, both using one [small plugin](../plugins/project-desk/1.0.0/project_desk_plugin.go):

| Route | Configured action | Result |
| --- | --- | --- |
| `/actions/advanced-name-preview` | `preview` | Validates the submitted name and displays a review step. |
| `/actions/advanced-name-confirm` | `confirm` | Validates again and displays the reviewed name. |

The client submits the name; YAML selects the action. The plugin returns a direct `<TEMPLATE>` configuration with a template name and plain `values`. Its displayed word count is returned as a string, matching the template component's scalar-value handling. HyperBricks renders [the result template](../lessons/plugin/templates/review-result.html) with its normal HTML escaping. An additional tree wrapper is unnecessary.

This is a stateless review exercise: confirmation does **not** save a project, and its submitted name is not proof that an earlier preview occurred. A real workflow must enforce required transitions and persistence in its owning service. The example is deliberately small so the plugin contract is visible. Use `goja_render` for small trusted synchronous calculations when its inputs are enough; use a plugin when you need Go libraries, request handling, external access, or a larger workflow.

For a small exercise, add a word-count message to the result template. Keep the count in plugin values and the wording in HTML.

## Check and package your work

Run the fixture's validation, stale-save, concurrent-save, and access checks:

```sh
go test -race modules/hyperbricks-basics/tools/fixture-api/main.go \
  modules/hyperbricks-basics/tools/fixture-api/main_test.go
```

After building the plugin against your checkout, run `go test ./...` from its `plugins/project-desk/1.0.0/` directory. It checks the direct template contract, both actions, input validation, and that request input cannot select the action.

These optional screens need a running server; exporting HTML does not preserve API edits, login, or plugin requests. Follow the main README's clean-source packaging workflow. Use its separate handbook profile for a public static site; do not run static export against either optional configuration. The fixture is a local teaching dependency and should not be published as a production backend.
