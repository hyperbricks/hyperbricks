# HyperBricks basics: Project Desk

Start here if you know ordinary HTML, CSS, JavaScript, and HTTP, and want to
understand how to put them together in HyperBricks. Project Desk is a small
working application: an overview, two project pages, a refreshable panel, and a
server-rendered estimate. Every page has a real URL and works without JavaScript.

The basic project runs without a database, credentials, Docker, npm installation,
or a custom plugin build. Its sample projects live in YAML. Nothing you enter is
saved. [Optional lessons](docs/advanced.md) add a local API, route guards, and a
custom Go plugin after the foundations make sense.

## Run it

Use a HyperBricks runtime built from the same source checkout as this module.
This example uses native `esbuild` and beta `goja_render`; an older installed
CLI may not include them. The source checkout currently still reports
`v1.2.2-beta`, so the version label alone does not prove feature compatibility.

From the HyperBricks repository root, with the Go version required by `go.mod`:

```sh
go build -o ./bin/hyperbricks ./cmd/hyperbricks
mkdir -p bin/plugins modules/hyperbricks-basics/rendered
./bin/hyperbricks start -m hyperbricks-basics
```

Open [Project Desk](http://localhost:8104/). Stop the server with Ctrl+C. For
a different port, add `--port 8105`. Once a compatible released CLI is installed,
use `hyperbricks` in place of `./bin/hyperbricks` in the commands below.

Keep the terminal open. It reports loaded routes, missing files, and render
errors. Development watch is enabled for YAML, templates, and resources; save a
change, allow the rebuild to finish, and refresh the browser. Restart after
changing the package configuration.

## A 30-minute first journey

Thirty minutes is a learning target, not a measured completion promise. The aim
is to make one visible change, add a route, and understand what you would deploy.

| Time | Try this | What you should see |
| --- | --- | --- |
| 0–5 min | Start the module; open Projects and Atlas, reload, then use Back. | Real page URLs, one shared shell, and working history. |
| 5–10 min | Change Atlas's `summary` in `hyperbricks/partials/views.hyperbricks.yaml`. | The same new text in its card and detail page. |
| 10–20 min | Add the page and fragment below, then add its navigation item. | A direct page URL and a smaller fragment response share the same content. |
| 20–25 min | Visit `/estimate?quantity=3`, then try `quantity=0` and two `quantity` values. | €59.85 for three items; a helpful message for invalid input. |
| 25–30 min | Open the public handbook and compare the delivery choices below. | A clear distinction between static content and a running application. |

## Know where things live

| File or directory | What it owns |
| --- | --- |
| `package.hyperbricks.yaml` | Port, development behavior, and directories. This is an ordinary YAML mapping. |
| `hyperbricks/app.hyperbricks.yaml` | Full-page and fragment routes, composed from shared views. |
| `hyperbricks/partials/site.hyperbricks.yaml` | The common page shell and asset attachments. |
| `hyperbricks/partials/navigation.hyperbricks.yaml` | Navigation labels, canonical URLs, and fragment URLs. |
| `hyperbricks/partials/views.hyperbricks.yaml` | Sample project data and reusable content components. |
| `hyperbricks/partials/assets.hyperbricks.yaml` | Native esbuild configuration for browser CSS and JavaScript. |
| `templates/` | HTML and small presentation expressions. |
| `resources/` | Browser source files, the server calculation, and the pinned HTMX source. |
| `static/` | Public assets, including generated bundles. The public URL begins with `/static/`. |
| `rendered/` | Generated static output; keep teaching sources elsewhere. |
| `lessons/handbook/` | A separate configuration that loads only the static handbook homepage. |
| `lessons/advanced/`, `lessons/plugin/`, `plugins/`, `tools/` | Optional advanced lessons and local support tools. |

The runtime scans top-level `*.hyperbricks.yaml` files in the configured
`hyperbricks/` directory. Nested files need explicit `imports`; that is why the
partials are imported and optional lessons are not loaded by the basic config.

In component files, `- type:`, `- values:`, and named children form an ordered
object. Ordinary mappings hold data. Navigation uses a mapping with keys such as
`01_overview` so Go templates iterate in a predictable order. This also avoids
the current template renderer's limitation with direct lists of objects under
`values`. The project list uses a mapping keyed by project name; a tags list is
just a list of strings. See [YAML usage](../../docs/YAML_USAGE.md).

## Add a page and its fragment

Create `hyperbricks/about.hyperbricks.yaml`:

```yaml
imports:
  - partials/site.hyperbricks.yaml

about_view:
  - type: template
  - inline: |
      <section data-page-title="About" class="page-section">
        <p class="eyebrow">Make it yours</p>
        <h1>{{.heading}}</h1>
        <p class="lede">A small project with room to grow.</p>
      </section>
  - values:
      heading: About this workspace

about_page:
  - inherit: basics_page
  - route: about
  - title: About · Project Desk
  - body:
      - values:
          navigation:
            - inherit: basics_navigation
            - values:
                active: /about
          content:
            - inherit: about_view

about_fragment:
  - type: fragment
  - route: fragments/about
  - content:
      - inherit: about_view
```

Then add one data item under `basics_navigation.values.items` in
`hyperbricks/partials/navigation.hyperbricks.yaml`, aligned with `03_estimate`:

```yaml
        04_about:
          label: About
          url: /about
          fragment: /fragments/about
```

Open `/about` directly. It should include the complete page. Open
`/fragments/about`; it should contain only the section. Navigate to About from
another page, reload, and use Back and Forward. Finally disable JavaScript and
follow the link again: `href` still opens the full page.

An inherited component inside a `values` field should be restated with its own
`inherit` when overriding it, as with `navigation` above. This gives that value
its component type and template as well as the local override.

## How the pieces work together

**Pages and fragments share a view.** `projects_page` renders `projects_view`
inside `basics_page`. `projects_fragment` renders that same view directly.
Initial content is rendered on the server, so the page never depends on an
extra browser request just to display its main content.

**Links enhance normal navigation.** The navigation template gives each link an
ordinary `href`, an `hx-get` fragment address, an `hx-target`, and a canonical
`hx-push-url`. Only the content inside `#main-content` is replaced. The small
browser entrypoint updates the title, active navigation item, and keyboard focus
after a swap. HTMX 4 uses `htmx:after:settle`; its settle task identifies the
updated target. `history: "reload"` makes Back and Forward reload the canonical
page with fresh request-bound estimate results. HTMX 4 no longer stores local
history snapshots, so no `hx-history` attribute or history-cache setting is needed.

Every request declares its own target and swap mode. Main-content links use
`innerHTML`, preserving the focusable `<main>` element; the status refresh uses
`outerHTML` to replace its complete panel. These elements do not depend on parent
attribute inheritance.

**A panel can refresh independently.** The overview includes `basics_status`
during the initial render. Its refresh link requests `/fragments/status` and
replaces `#status-panel`; its plain `href` reloads the overview when JavaScript
is disabled. The timestamp uses Sprig's `now` and `date` functions. It is a
render time, not a claim about the health of another service.

**Templates handle presentation.** The project template uses
`{{.project.tags | sortAlpha | join ", "}}` to display tags. Values are HTML
escaped by the normal template renderer; no `safe` helper is needed for user
text. Components mounted into template values are rendered by HyperBricks.

**The estimate calculates on the server.** The trusted script in
`resources/scripts/line-total.js` receives the configured price in cents and
only the allowed `quantity` query parameter. It validates whole numbers from 1
to 100 and rejects repeated quantities. It returns a plain object that
`templates/estimate.html` reads through `.Data`. The form deliberately uses an
ordinary GET to `/estimate`, keeping its quantity in a bookmarkable URL.

Try `/estimate?quantity=3&unit_price_cents=1`: the total is still €59.85. Query
values cannot override the configured price. Each execution has a fresh Goja
runtime and copied inputs, and these routes use `Cache-Control: no-store`.
`goja_render` is beta and intended for trusted project scripts. Use a Go plugin
or API-backed component for persistent state, external services, or database
access. See [Goja Render](../../docs/GOJA_RENDER.md).

**Asset source and public output have different jobs.** The CSS and JavaScript
entries in `resources/` are bundled by native esbuild into `static/`. The
component emits the correct fingerprinted URL, which is included in the shared
head. Edit the resource files; esbuild generates a new bundle when they change.
No separate Node build process or esbuild plugin is needed. See
[esbuild](../../docs/ESBUILD.md).

## Choose what to deliver

The handbook is standalone public content. It contains only in-page anchors
and public external links, with no estimate form or application navigation.
Use the separate handbook configuration to export it. That configuration loads
only one homepage and reuses the handbook template and stylesheet. From the
repository root, stage it in a new directory:

```sh
python3 modules/hyperbricks-basics/tools/stage_source.py /tmp/project-desk-handbook --handbook
cd /tmp/project-desk-handbook
/absolute/path/to/hyperbricks/bin/hyperbricks static -m hyperbricks-basics --force --zip
```

Replace the absolute binary path with the source-matched runtime built above.
The staging helper selects `lessons/handbook/package.hyperbricks.yaml` as the
copied module's default configuration. The page and assets are written to
`modules/hyperbricks-basics/rendered/` inside that new project, and a zip is
written to the export directory shown by the command. Serve the generated
directory to preview it.

Keep the static lesson separate from the application: the current static
renderer discovers routes as well as configured targets, so a `static.routes`
entry alone does not limit an application export to one page. A static host
serves generated files; it cannot run the estimate, optional API actions, route
guards, or Go plugins. The original running application still exposes the same
handbook content at `/handbook`.

For the complete application, build a runtime archive from a clean copy of the
declared module sources, then run it with a compatible HyperBricks runtime.
The CLI's archive builder does not apply Git ignore rules. The source allowlist
and staging helper supplied with this module keep generated output and local
drafts out of that copy. From the repository root:

```sh
python3 modules/hyperbricks-basics/tools/stage_source.py /tmp/project-desk-release
cd /tmp/project-desk-release
/absolute/path/to/hyperbricks/bin/hyperbricks build --hra -m hyperbricks-basics
/absolute/path/to/hyperbricks/bin/hyperbricks start --deploy -m hyperbricks-basics --port 8105
```

Replace `/absolute/path/to/hyperbricks/bin/hyperbricks` with the source-matched
binary you built above. The staging destination must be new; the helper refuses
to overwrite an existing directory. The archive goes into `deploy/`. Inspect its
contents and open the local deployment before choosing a real hosting target.
Use `build --zip` for the alternative runtime archive format.

See [source staging](SOURCE_FILES.txt) and
[CLI archive commands](../../docs/HYPERBRICKS_CLI.md#build-archives).

## Continue learning

- [Optional API, guards, and plugin lessons](docs/advanced.md) add real local
  request flows with explicit prerequisites.
- [General HyperBricks skill](../../SKILLS/hyperbricks/SKILL.md) gives agents a
  concise workflow and a task-based Source Of Truth.
- [CLI reference](../../docs/HYPERBRICKS_CLI.md),
  [routing](../../docs/ROUTING.md), and
  [component reference](../../docs/REFERENCE.md) cover the complete options.

## Check the example

From the matching HyperBricks repository root, run the reusable checks with
Python 3.9+, Go, and your source-matched binary:

```sh
python3 scripts/test_hyperbricks_basics.py --binary /absolute/path/to/hyperbricks
```

The check creates temporary projects and local servers. It exercises pages,
fragments, assets, request validation, concurrent estimates, development reload,
the exact add-page exercise above, API outcomes, guards, and both delivery
formats. It stops its servers afterward and keeps diagnostics if a check fails.
Add `--with-plugin` to also build and check the native Go plugin against this
checkout; that requires a compatible native plugin platform and Go toolchain.

The browser walkthrough has also been checked with real HTMX navigation,
Back/Forward, reload, status refresh, and JavaScript disabled, plus desktop and
mobile layouts. When adapting the example, repeat the interactions you change;
HTTP checks alone do not prove browser behavior.

## Credits and reproducibility

HyperBricks: [website](https://hyperbricks.org/) and
[repository](https://github.com/hyperbricks/hyperbricks).
Native bundling uses [esbuild](https://esbuild.github.io/), by Evan Wallace,
under its [MIT license](https://github.com/evanw/esbuild/blob/main/LICENSE.md).
Server-side JavaScript uses [Goja](https://github.com/dop251/goja).

HTMX **4.0.0** is included locally as `resources/vendor/htmx-4.0.0.js`, copied
unchanged from the npm package's `dist/htmx.esm.js`. Its SHA-256 is
`077b8017a057e3e6dd6834d20012f75c6387bde189bbd26172c6b3a853125cbc`.
The original [0BSD license](static/vendor/HTMX-LICENSE.txt) is included.
See the [HTMX website](https://htmx.org/) and
[source repository](https://github.com/bigskysoftware/htmx).
To update it deliberately, obtain the named npm release, replace the source and
license together, record the version and hash here, and rerun navigation checks.
The basic app and both interactive lessons use this same source. See the pinned
[HTMX 4 migration guide](https://raw.githubusercontent.com/bigskysoftware/htmx/v4.0.0/dist/skills/htmx-upgrade-from-htmx2.md)
and [HTMX 4 guidance](https://raw.githubusercontent.com/bigskysoftware/htmx/v4.0.0/dist/skills/htmx-guidance.md).
