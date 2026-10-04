<!-- Generated from docs/QUICKSTART.md. Do not edit directly. -->

# Quickstart

Create a small module with HTML templates, JavaScript, and CSS. HyperBricks renders a full page and an HTML fragment. The built-in `esbuild` component bundles the browser assets.

The example uses [HTMX 4](https://four.htmx.org/) to update part of the page.

## Install And Create A Module

Requires Go 1.26.1 or newer and internet access to install HyperBricks, download the starter, and fetch HTMX.

**1. Install HyperBricks**

Install the latest release:

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

Make sure your Go binary directory (`GOBIN`, or `$(go env GOPATH)/bin` by default) is on your `PATH`.

**2. Create the module**

From your project root, the directory that will contain `modules/`:

```bash
hyperbricks init-starter get hello-world -m demo
```

The command copies the published `modules/hello-world` starter from the HyperBricks repository. It already runs with `hyperbricks start -m demo` and displays a simple Hello World page. Continue below to replace it with a page that uses templates, bundled assets, and an HTML fragment. To install a pinned Git snapshot, see [`init-starter` in the CLI guide](HYPERBRICKS_CLI.md#init-starter-install-an-official-starter).

Initialization also reconciles the new module's source metadata. Its
`package.hyperbricks.yaml` identifies the destination module, starts its module
version at `1.0.0`, and records the version of the HyperBricks binary that
installed it:

```yaml
hyperbricks:
  metadata:
    module: demo
    moduleversion: "1.0.0"
    hyperbricks: <exact installed version>
```

The `<exact installed version>` placeholder represents the exact version written
by the binary you run. Later, refresh the identity and runtime version without
changing the scaffold, or bump the module version at the same time:

```bash
hyperbricks init -m demo --update-metadata
hyperbricks init -m demo --bump-version          # patch
hyperbricks init -m demo --bump-version=minor
hyperbricks init -m demo --bump-version=major
```

These metadata-only commands also accept a relative or absolute module path,
just like `start -m` and `build -m`. A regular build leaves this source package
unchanged and writes `format`, `format_version`, `commit`, `built_at`, and the
exact building HyperBricks version only into the resulting deployment archive.
The build's `source_hash` is recorded separately in its build index.

**3. Create the asset directories**

```bash
mkdir -p \
  modules/demo/resources/js \
  modules/demo/resources/css \
  modules/demo/resources/vendor
```

Keep the generated `package.hyperbricks.yaml`. Replace the contents of the starter's `hyperbricks/hello-world.hyperbricks.yaml` with the configuration below. This replaces the starter route instead of creating a second route for `/`.

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

The same card template supplies the initial page content and the fragment. YAML values provide its text; the template owns the HTML markup.

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

let updates = 0;
document.addEventListener("htmx:after:swap", () => {
  updates += 1;
  document.getElementById("update-status").textContent =
    `Card updated ${updates} time${updates === 1 ? "" : "s"}.`;
});
```

The import includes HTMX in your application bundle. There is no separate CDN script in the page. The small event handler shows where your own JavaScript belongs: it updates the status after HTMX replaces the card.

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

Edit files under `resources/` and `templates/`; the files under `static/` are build output. See [JavaScript and CSS](ESBUILD.md) for more esbuild options.

## Developer Interface

Open `modules/demo/package.hyperbricks.yaml` and enable the module's developer
dashboard. Keep credentials in environment variables rather than committing a
password:

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

The same module-owned login protects the Dashboard's Overview and Errors views,
render diagnostics, Spaces, contextual editing, and configured frontend-editor
plugins. There is no
default account; unresolved credentials lock those interfaces while public
application routes remain available.


## Run And Try It

From the project root:

```bash
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
hyperbricks start -m demo
```

Open [localhost:8080](http://localhost:8080/). The first page render builds the assets, including the imported HTMX source. No npm installation or separate asset build command is needed for this example.

Click **Load fragment**. The card changes and your JavaScript increments the status counter. Click again: the counter increases without reloading the page. A full reload restores the initial card and resets the counter.

With the dashboard enabled, open [localhost:8080/__hyperbricks/dashboard](http://localhost:8080/__hyperbricks/dashboard) to inspect the routes and follow their links. If a page reports an error, select **Errors** to read its diagnostics. See [Troubleshooting](TROUBLESHOOTING.md).


Try changing the card text in YAML, the markup in `templates/card.html`, or the styles in `resources/css/app.css`. With development watching enabled, save and reload the page to see the change. Keep browser behavior in `app.js`, styling in `app.css`, and route composition in YAML.

## Render Static Output

If you want to host the static site on Cloudflare, GitHub Pages or other static hosting service, you can export the site to a static version and serve it with the `--serve` option.

```bash
hyperbricks static -m demo --serve
```

When asked `Delete directory "modules/demo/rendered"? (y/n)` type yes, it will just remove the last static export. You can omit that with the `--force` option.

The generated site is written to `modules/demo/rendered/`. Static rendering requests routes through an internal runtime before writing the HTML and assets. On a separate static host, the fragment URL `/hello-fragment` must resolve to its exported HTML file; configure clean-URL handling to match your runtime routes.

## Zip Archive

To create a ZIP archive of your site, use the `--zip` option. The default output directory is `./exports/<module>/`, so this example writes its archive to `./exports/demo/`.



```bash
hyperbricks static -m demo --zip --force
```

The `--force ` option automatically overrides the exported files in `rendered/`.
Type `hyperbricks static --help` for all static options.

## Next Steps

- [Development hooks and managed services](DEVELOPMENT_HOOKS.md): start a local API with your module, wait for readiness, and stop both with one command. Try the [beginner demo](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/modules/development-hooks-demo/README.md).
- [General HyperBricks skill](https://github.com/hyperbricks/hyperbricks/blob/v1.2.9-beta/SKILLS/hyperbricks/SKILL.md): give an agent the project conventions, CLI workflow, and task-based Source Of Truth.
- [YAML usage](YAML_USAGE.md): YAML syntax, resolvers, imports, inheritance.
- [Component reference](REFERENCE.md): component fields and executable examples.
- [ROUTING.md](ROUTING.md): route resolution and clean URLs.
- [Route guards](ROUTE_GUARD.md): pre-render route authorization.
- [Troubleshooting](TROUBLESHOOTING.md): find and resolve configuration and render errors.
- [Migration Guide](MIGRATION.md): update older response and API authentication settings.
