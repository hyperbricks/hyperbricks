<!-- Generated from docs/QUICKSTART.md. Do not edit directly. -->

# Quickstart

Create a small module with HTML templates, JavaScript, and CSS. HyperBricks renders a full page and an HTML fragment. The built-in `esbuild` component bundles the browser assets.

The example uses [HTMX 4](https://four.htmx.org/) to update part of the page.

## Install, Start And See The Result

Requires Go 1.26.1 or newer, a browser, and internet access to install HyperBricks,
download the starter, and fetch HTMX. Shell commands below use Bash-compatible
syntax on macOS, Linux, or WSL. No Node.js or npm is needed for this example.

**1. Install and check HyperBricks**

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
hyperbricks version
```

The second command should print the installed version. If your shell reports
`command not found`, follow [Go binary directory and PATH](TROUBLESHOOTING.md#hyperbricks-is-not-found)
before continuing. When following documentation for a specific release, use
that release's version and documentation together; `@latest` selects the latest
published release, not an unreleased checkout.

**2. Create a project and install the starter**

Use a new directory for this example. All subsequent commands run from this
project root, which will contain `modules/`:

```bash
mkdir my-hyperbricks-site
cd my-hyperbricks-site
hyperbricks init-starter get hello-world -m demo
```

This installs the published `hello-world` starter as `modules/demo`. For a pinned
Git snapshot, see [installing an official starter](HYPERBRICKS_CLI.md#init-starter-install-an-official-starter).

**3. Start it**

```bash
hyperbricks start -m demo
```

Keep this terminal running and open [localhost:8080](http://localhost:8080/).
You should see **HELLO WORLD!**. If the port is occupied, use
`hyperbricks start -m demo --port 8081` and open `http://localhost:8081/`;
use that port for the later browser links too.

Open `modules/demo/hyperbricks/hello-world.hyperbricks.yaml` in your editor,
change `HELLO WORLD!` to your own text, save, and refresh. The starter watches
source changes in development mode. If the result does not change, see
[an edit does not appear](TROUBLESHOOTING.md#an-edit-does-not-appear).

Using the VS Code extension? Open the project directory and a
`*.hyperbricks.yaml` file. **HyperBricks: Show Output** identifies the runtime it
uses. If that differs from your terminal or cannot start, set
`hyperbricks.executable` to the intended binary's absolute path; see
[VS Code executable recovery](TROUBLESHOOTING.md#vs-code-cannot-find-hyperbricks).

## Open The Dashboard

Dashboard lists your routes and links to **Errors**. It is enabled by default
in development and debug mode. Open
[http://localhost:8080/__hyperbricks/dashboard](http://localhost:8080/__hyperbricks/dashboard),
using your selected port. The **Dashboard** startup log shows the complete URL.
Dashboard is unavailable in live mode, production runtimes and static output.

If an existing module explicitly sets `development.dashboard.enabled: false`
(or legacy `development.dashboard: false`), stop it with **Ctrl+C** and run:

```bash
hyperbricks settings -m demo
```

Use the arrow keys and **Enter** to open **Development → Dashboard → Enabled**.
Choose `true` and press **Enter** to stage the change. Press **r** to review,
**Enter** to save, and **q** to leave the editor. Restart the module:

```bash
hyperbricks start -m demo
```

If you previously used `--port 8081`, include it again when restarting.

With both developer credentials absent, Dashboard opens without a login. See
[developer access, credentials, Errors, and Spaces](#developer-interface) for
the access rules and [the settings editor](PACKAGE_CONFIGURATION.md#editing-settings-interactively)
for source ownership and saving. Package settings take effect after a restart.

### Render Status Indicator

In development/debug mode, a small render-status badge appears at the bottom right
of full HTML pages when Dashboard is enabled. It is on by default.

Green **● ok** means the renders observed by this page have succeeded.
Red **● errors** means a page or fragment render reported an error. Click green to
open Dashboard or red to open the selected error. Clicks reuse the same developer browser tab.
Configured Dashboard credentials still require login there.

The badge follows the Dashboard light/dark preference on the same origin, or your
system theme when no preference is saved. An `ok` badge hides after five seconds.
A new render error makes it visible again; after recovery it shows `ok` for another
five seconds. Further successful requests do not restart this timer.

**Hide the indicator:** add or update this setting under the existing
`hyperbricks.development` mapping in `modules/demo/package.hyperbricks.yaml`:

```yaml
hyperbricks:
  development:
    status_indicator: false
```

Keep the module's other settings. Stop the server with **Ctrl+C**, then restart
with `hyperbricks start -m demo` (and your chosen `--port`, if any). Dashboard
remains available. Set `status_indicator: true` to show the badge again.
Disabling Dashboard also disables the indicator.

Same-origin `fetch` and XMLHttpRequest responses update the indicator using
HyperBricks response headers. A successful retry of the same URL and method clears
that request's error; unrelated successes do not hide other failures. A page reload
starts a new observation. Green is not a whole-application health check: JavaScript
errors, network failures and requests without render-status headers are not assessed.

The button is omitted from fragment bodies, live mode, production runtimes and
static exports. It shows no error details on the public page. It requires JavaScript;
a restrictive Content Security Policy must permit the same-origin helper script
and stylesheet. HyperBricks does not relax your policy.

## Extend The Same Module

The running starter is the base for the rest of this page. Next, replace its
single text component with a page template, bundled assets, and a fragment.
Stop the server with **Ctrl+C** while adding the files below, so it does not try
to load an incomplete example. Restart after connecting them.

Create the asset directories:

```bash
mkdir -p \
  modules/demo/resources/js \
  modules/demo/resources/css \
  modules/demo/resources/vendor
```

Keep the generated `package.hyperbricks.yaml`. Replace the contents of the starter's `hyperbricks/hello-world.hyperbricks.yaml` with the configuration below. This replaces the starter route instead of creating a second route for `/`.

The package file controls module settings such as the server port, runtime mode,
and directory locations. See [Package Configuration](PACKAGE_CONFIGURATION.md)
for its structure and defaults.

The files you will work with are:

```text
modules/demo/
  package.hyperbricks.yaml
  hyperbricks/
    hello-world.hyperbricks.yaml
  templates/
    page.html
    card.html
  resources/
    js/app.js
    css/app.css
    vendor/htmx-4.0.0.js
  static/                       Generated browser bundles
  rendered/                     Generated static site
```

HyperBricks automatically loads `*.hyperbricks.yaml` files directly inside the module's configured `hyperbricks/` directory. Files in subdirectories need a file-level `imports` entry in a loaded source file. For example, `hyperbricks/partials/site.hyperbricks.yaml` needs an import such as `partials/site.hyperbricks.yaml` from a file directly inside `hyperbricks/`. Import paths are relative to the importing file. See [YAML Usage: Imports](YAML_USAGE.md#imports).

## Add The Templates

Create `modules/demo/templates/page.html`:

```html
<main>
  <h1>{{.title}}</h1>
  <p>HyperBricks renders the HTML. HTMX updates the card below.</p>
  <button hx-get="/hello-fragment" hx-target="#target" hx-swap="innerHTML">
    Load fragment
  </button>
  <p id="update-status" role="status">No updates yet.</p>
  <div id="target">{{.card}}</div>
</main>
```

Create `modules/demo/templates/card.html`:

```html
<section class="card">
  <h2>{{.title}}</h2>
  <p>{{.body}}</p>
</section>
```

The same card template supplies the initial page content and the fragment.
YAML `values` provide `.title` and `.body`; the template owns the HTML markup.
A nested component can also supply a value: the configuration below renders
`base_card` first and places its HTML in the page template's `{{.card}}` slot.
Ordinary text values are escaped by Go's `html/template` for their HTML context.

For file lookup, see [the `template.file` resolver](YAML_USAGE.md#templatefile).
For conditionals, loops, functions, and request input, see
[template syntax](YAML_USAGE.md#template-syntax); the
[template component reference](REFERENCE.md#template) lists the supported fields.

## Add JavaScript And CSS

Download a pinned copy of HTMX into the module:

```bash
curl -fL \
  https://raw.githubusercontent.com/bigskysoftware/htmx/v4.0.0/dist/htmx.js \
  -o modules/demo/resources/vendor/htmx-4.0.0.js
```

Create `modules/demo/resources/js/app.js`:

```javascript
import "../vendor/htmx-4.0.0.js";

const target = document.getElementById("target");
const status = document.getElementById("update-status");
let updates = 0;

document.addEventListener("htmx:after:request", (event) => {
  const { ctx } = event.detail;
  if (ctx.target !== target) return;

  const diagnostics = Number(ctx.response.headers.get("X-Hyperbricks-Render-Error-Count"));
  if (!ctx.response.raw.ok || !ctx.text.trim() || diagnostics > 0) {
    event.preventDefault(); // Keep the current card; do not swap this response.
    status.textContent = diagnostics > 0
      ? "Card not updated: the response contains diagnostics. Check Errors or the server log."
      : "Card could not be loaded. Check Errors or the server log, then try again.";
  }
});

document.addEventListener("htmx:after:swap", (event) => {
  if (event.detail.ctx.target !== target) return;
  updates += 1;
  status.textContent = `Card updated ${updates} time${updates === 1 ? "" : "s"}.`;
});

document.addEventListener("htmx:error", (event) => {
  if (event.detail.ctx?.target !== target) return;
  status.textContent = "Card could not be updated. Check the connection and server, then try again.";
});
```

The import includes HTMX in your application bundle. There is no separate CDN
script in the page. The handlers apply only to the card's target. In HTMX 4,
`htmx:after:request` runs before the swap; cancelling it keeps the current card
when the response has an unsuccessful HTTP status, is empty, or contains render
diagnostics. Only `htmx:after:swap` increments the counter. Connection failures
also show a retry message.

HTTP 200 alone does not guarantee a complete render. The diagnostic count also
includes warnings and notices, so this example keeps the previous card whenever
that count is positive and asks you to inspect it. Static hosts may omit this
header; the HTTP-status and empty-response checks still apply. See
[finding the reported error](TROUBLESHOOTING.md#find-the-reported-error) for
diagnostic details and the [HTMX 4 event reference](https://four.htmx.org/reference/events)
for the request lifecycle.

Create `modules/demo/resources/css/app.css`:

```css
body {
  margin: 2rem;
  font-family: system-ui, sans-serif;
  color: #243b32;
  background: #f6f7f3;
}

main {
  max-width: 42rem;
  margin: auto;
}

button {
  padding: 0.6rem 1rem;
  cursor: pointer;
}

.card {
  padding: 1rem;
  background: white;
  border: 1px solid #ccd5ce;
}

#update-status {
  color: #52655a;
}
```

## Connect The Routes, Templates And Assets

Components use YAML sequences: each `-` adds an entry, preserving the configured order of component children. Data fields such as `values` use ordinary mappings. See [YAML Usage: Ordered objects and ordinary mappings](YAML_USAGE.md#ordered-objects-and-ordinary-mappings).

Replace `modules/demo/hyperbricks/hello-world.hyperbricks.yaml` with:

```yaml
app_styles:
  - type: esbuild
  - entry:
      path: {base: resources, path: css/app.css}
  - outfile:
      path: {base: static, path: css/app.css}
  - cache: true
  - fingerprint: true
  - enclose: <link rel="stylesheet" href="|">

app_scripts:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/app.js}
  - outfile:
      path: {base: static, path: js/app.js}
  - cache: true
  - fingerprint: true
  - enclose: <script src="|" defer></script>

base_card:
  - type: template
  - template:
      file: card.html
  - values:
      title: Welcome
      body: This card was rendered with the page.

page:
  - type: hypermedia
  - route: index
  - title: HyperBricks Quickstart
  - head:
      - type: head
      - meta:
          viewport: width=device-width, initial-scale=1
          description: A HyperBricks page with templates and bundled assets.
      - charset:
          - type: html
          - value: '<meta charset="utf-8">'
      - styles:
          - inherit: app_styles
      - scripts:
          - inherit: app_scripts
  - body:
      - type: template
      - template:
          file: page.html
      - values:
          title: Hello HyperBricks
          card:
            - inherit: base_card

hello_fragment:
  - type: fragment
  - route: hello-fragment
  - panel:
      - inherit: base_card
      - values:
          title: Hi from a fragment
          body: This card arrived without reloading the page.
```

- `hypermedia` serves a full page at `/` and `/index` with the default routing settings.
- `template.file` loads a file from the module's templates directory.
- `inherit` reuses a component. The fragment overrides the card's text while keeping its template.
- The two `esbuild` components bundle JavaScript and CSS into `static/` and emit their script and stylesheet tags. `fingerprint` adds a content hash to asset URLs; `cache` reuses build results when the source files have not changed.
- The `fragment` route returns only the card HTML. The `hx-*` attributes in `page.html` tell HTMX to request it and replace the contents of `#target`. HyperBricks does not require HTMX to render this route.

The flow is `resources/` → `esbuild` → `static/` → browser. The first page request
renders the head, which builds the bundles and emits their public URLs. Edit
`resources/`, not the generated files in `static/`. Asset build reuse, page
caching, and browser caching are separate; see [build and cache lifecycle](ESBUILD.md#build-and-cache-lifecycle)
and [fingerprinted output](ESBUILD.md#versioned-output).

For fragments, the flow is browser click → `GET /hello-fragment` → HyperBricks
renders `card.html` with the fragment's values → HTMX replaces `#target` → your
JavaScript updates the counter. The fragment has its own URL and contains no
page shell. To go further, see [explicit page and fragment routes](HTMX_FRAGMENTS_AND_CANONICAL_URLS.md#pattern-1-canonical-page-and-explicit-fragment-route),
[inheritance and overrides](YAML_USAGE.md#inheritance), and
[route resolution](ROUTING.md).

## Run And Try It

From the same project root, start the completed example:

```bash
hyperbricks start -m demo
```

Keep any `--port` override you used earlier when restarting.
Open [localhost:8080](http://localhost:8080/). Expect **Hello HyperBricks**, a
**Welcome** card, styled content, and **No updates yet.** The first page render
builds the assets, including HTMX. No separate asset build command is needed.

Click **Load fragment**. The card title becomes **Hi from a fragment** and the
status becomes **Card updated 1 time.** Click again: the counter increases
without a full page reload. Refresh the page to restore the initial card and
reset the counter. Opening `/hello-fragment` directly shows only the card HTML.

If the fragment request fails, the existing card stays visible and the status
explains what to check. Fix the reported problem and click **Load fragment**
again: a successful retry updates the card and resumes the counter. Failed
attempts do not count as updates.

To make an independent change, edit the fragment's `values.body` in YAML and
click **Load fragment** again after saving. Change `templates/card.html` to
alter markup for both cards, or `resources/css/app.css` to change their styling.
Save and refresh to see template and style changes. Keep browser behavior in
`app.js`, styling in `app.css`, and route composition in YAML.

If the page is incomplete or an interaction fails, start with the
[Developer Interface](#developer-interface) below. For missing bundles or an
unchanged page, see [asset troubleshooting](TROUBLESHOOTING.md#css-or-javascript-is-missing)
and [development watching](ESBUILD.md#development-watch).

## Developer Interface

Return to [Dashboard](http://localhost:8080/__hyperbricks/dashboard) to inspect
routes and follow their links; use the port selected when starting your module.
If it is not enabled yet, follow [Open The Dashboard](#open-the-dashboard).
**Errors** shows problems from requests that actually ran, with source locations
where available. After fixing a template or YAML
error, save and request the affected page or fragment again; a successful retry
clears that request's earlier error. An unchecked route has not been tested.
See [finding the reported error](TROUBLESHOOTING.md#find-the-reported-error).

The settings editor writes `hyperbricks.development.dashboard.enabled` in
`modules/demo/package.hyperbricks.yaml`. The corresponding YAML is below, with
optional environment references for requiring a developer login. If editing the file directly, keep the other
package settings and update the existing keys; do not add a second
`hyperbricks` or `development` key:

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

When both credentials are configured, the same module-owned login protects the
Dashboard's Overview and Errors views, render diagnostics, Spaces, contextual
editing, and configured frontend-editor plugins. There is no default account.

In development and debug mode, leaving both credentials empty allows the enabled
Dashboard, Errors, their diagnostics, Spaces, and contextual editing to open
without login and produces a startup warning. If only one credential resolves,
developer access remains blocked with `503`. Frontend-editor plugins still
require credentials and run only in development mode. Public application routes
remain available in either case.

The `hello-world` starter explicitly disables `frontend_editing`. To try Spaces,
run `hyperbricks settings -m demo` again and set
**Development → Frontend editing → Enabled** to `true`
(`hyperbricks.development.frontend_editing.enabled`). Review, save, and restart
the server. The example above has no editable fields yet: define a
[Space source and its editable fields](SPACES.md#source-schema) before using
Spaces to edit its content. Enabling the interface alone does not make every
YAML value editable.

When frontend editing is enabled, Spaces and its writes default to enabled. Under
`hyperbricks.development.frontend_editing.spaces`, set `write: false` for
read-only access or `enabled: false` to disable Spaces and contextual editing.
Spaces accepts localhost and loopback by default; other server addresses need
an explicit `allowed_hosts` entry in that same section, even when login is configured. See
[Spaces configuration](SPACES.md#development-configuration) for LAN setup and
the separate `public_origin` setting. Developer interfaces are unavailable in
live mode, production runtimes, and static output.

To require a developer login, set both `HB_DEVELOPER_USER` and
`HB_DEVELOPER_PASSWORD` in the terminal before starting the server. Restart after
changing these variables or the package configuration. For access beyond your
own machine, configure both credentials before exposing the server.

## Export Or Run On A Server

Choose according to what must happen when someone visits the site:

| Need | Delivery |
| --- | --- |
| Pages and fragments can be rendered ahead of time, as in this example. | Export HTML and assets to a static host. Browser JavaScript still runs; HyperBricks does not run there. |
| Requests need current API data, server logic, authentication, or user-specific output. | Build a runtime archive and run it with HyperBricks on a server. |

If you used `start --port 8081`, that override did not change the package.
The `static --serve` command and runtime archive use `hyperbricks.server.port` from
`modules/demo/package.hyperbricks.yaml`. Before continuing, stop the development
server and set that existing field to your selected free port if needed. Save
the package before exporting or building; no development restart is needed.
See [server settings](PACKAGE_CONFIGURATION.md#server-and-request-limits) for the configuration.

### Render Static Output

Stop the development server with **Ctrl+C**, then run:

```bash
hyperbricks static -m demo --serve
```

If asked to delete `modules/demo/rendered`, confirm only when it contains output
you are ready to replace. The command renders a fresh snapshot and serves it;
open the URL printed in the terminal and check both the page and **Load fragment**.
The preview URL can differ from the development server's localhost URL.
The development interfaces are not part of the static site. Stop this preview
with **Ctrl+C** when finished. See [the static rendering lifecycle](HYPERBRICKS_CLI.md#rendering-runtime-and-static-file-server).

For an agent or terminal without interactive input, use
`hyperbricks static -m demo --serve --force --non-interactive`. This replaces the
generated output without asking. See [non-interactive mode](HYPERBRICKS_CLI.md#non-interactive-mode).

### Zip Archive

Create a ZIP for a static host:

```bash
hyperbricks static -m demo --zip --force
```

`--force` replaces the generated `modules/demo/rendered/` directory without a
prompt. Keep source files outside that directory. The archive is written under
`exports/demo/`; use the exact filename printed by the command.

Extract the ZIP into a new `site/` directory so that `site/index.html` exists.
From the directory containing `site/`, preview those files:

```bash
hyperbricks serve ./site
```

Open the URL printed in the terminal and check the page, CSS, JavaScript, and
**Load fragment**. This command serves the extracted files without rebuilding
the module. It uses port `8080` by default, independently of package settings;
add `--port 8081` or `--port 0` if that port is occupied. Stop it with **Ctrl+C**.

For publication, the host must resolve `/hello-fragment` to the exported
fragment HTML, as this preview does. A single-page-app fallback would return
the full page for that request and break the interaction. See
[serving the export](HYPERBRICKS_CLI.md#serving-the-export), and repeat the checks
at the public URL before considering the site published. Exported API results
are snapshots; they do not call the API again when a visitor requests the file.

### Runtime Archive

For a server application, build and run an archive from the project root:

```bash
hyperbricks build --hra -m demo
hyperbricks deploy run -m demo --mode live
```

Stop other servers using the same port first. Open the URL printed in the
terminal and check the page and fragment again. This verifies the local archive;
a public deployment additionally needs a configured host and access settings.
Follow [running a deployment build](DEPLOY.md#run-a-deployment-build) and
[service configuration](DEPLOY.md#service-examples) for the server setup.
Development interfaces are unavailable in live mode.

## Adjust package settings

From the project root, run `hyperbricks settings -m demo` for an interactive settings menu with descriptions, effective values, and source locations. Review pending changes before saving, then restart the application. One package file is enough; imports are optional when configuration grows. See [Package Configuration](PACKAGE_CONFIGURATION.md#editing-settings-interactively).

Native esbuild output cleanup is automatic for startup/restart and static export. No cleanup hook is required. Use the component's `cache_keep` option when older fingerprinted generations must remain available; see [Native esbuild](ESBUILD.md#versioned-output).

Initialization records the module name, module version, and installed runtime
version in the package. To maintain those values later, see
[refreshing or bumping package metadata](HYPERBRICKS_CLI.md#refresh-or-bump-package-metadata).
A build records its provenance in the archive; it does not rewrite the source
package's metadata.

## Next Steps

- [Package Configuration](PACKAGE_CONFIGURATION.md): configure runtime modes, developer access, caching, and module directories.
- [Caching: a practical guide](CACHING.md): watch a route render fresh, enable memory or disk caching, observe expiry, and purge stored responses.
- [Development hooks and managed services](DEVELOPMENT_HOOKS.md): start a local API with your module, wait for readiness, and stop both with one command. Try the [beginner demo](https://github.com/hyperbricks/hyperbricks/blob/v1.3.1-beta/modules/development-hooks-demo/README.md).
- [Set up a coding agent](AGENT_SETUP.md): use the shared HyperBricks skill with Codex, Claude Code, or Gemini CLI, without a required plugin.
- [YAML usage](YAML_USAGE.md): YAML syntax, resolvers, imports, inheritance.
- [Component reference](REFERENCE.md): component fields and executable examples.
- [ROUTING.md](ROUTING.md): route resolution and clean URLs.
- [Route guards](ROUTE_GUARD.md): pre-render route authorization.
- [Troubleshooting](TROUBLESHOOTING.md): find and resolve configuration and render errors.
- [Migration Guide](MIGRATION.md): update older response and API authentication settings.
