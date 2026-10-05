# After Hours — Swup navigation demo

A text-only guide to four fictional evening venues. HyperBricks renders six English guide pages plus German and Dutch café Spaces; Swup 4.10.0 animates navigation between them.

## Run

First, [install the HyperBricks CLI](../../docs/HYPERBRICKS_CLI.md#install).

From the repository root:

```sh
hyperbricks start -m navigation-demo-swup
```

Open http://localhost:8125/. The library is vendored locally and native esbuild bundles JavaScript and CSS. No npm install or separate server is required.

With both `HB_DEVELOPER_USER` and `HB_DEVELOPER_PASSWORD` absent, Dashboard,
Errors, Spaces, and contextual editing open without login in development and
debug mode. Startup warns about this access. Spaces writes are enabled; set
`hyperbricks.development.frontend_editing.spaces.write: false` for read-only access,
or `hyperbricks.development.frontend_editing.spaces.enabled: false` to disable
Spaces and contextual editing.

### Optional developer login

To require login, set both variables in the terminal that starts HyperBricks:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
hyperbricks start -m navigation-demo-swup
```

Replace the password placeholder with your own password. The package reads these
variables for the shared developer login; there is no default account. If only
one resolves, developer access is blocked with `503`. Restart after changing the
variables. Public pages and static exports do not require this login.

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

### LAN access to Spaces

Localhost and loopback addresses work by default. For another server hostname or
IP, add an entry under the existing `hyperbricks.development.frontend_editing.spaces`
configuration and restart:

```yaml
allowed_hosts:
  - editor.example.test
  - 192.0.2.10
```

List the server address from the browser URL without a scheme, port, or path.
This checks the server address, not which client computers may connect, and is
enforced with or without login. Without credentials and with writes enabled,
anyone who can reach an allowed address can edit. A rejected host returns `403`;
write requests also need an origin matching the backend connection's scheme,
host, and port. An HTTPS proxy forwarding to HTTP HyperBricks rejects saves with
`403`; forwarded headers do not change this check. Configure both credentials
when access needs to be restricted, and use a private encrypted tunnel or network
that preserves the browser origin to protect Basic Auth over a network. See
[LAN access and allowed hosts](../../docs/SPACES.md#lan-access-and-allowed-hosts).

`hyperbricks.development.frontend_editing.spaces.public_origin` is optional for
ordinary editing. It supplies the public
website origin for absolute sharing-image URLs; it does not grant access. See
[Spaces configuration](../../docs/SPACES.md#development-configuration).

## Export a static ZIP

From the repository root, render all eight pages and package their assets:

```sh
hyperbricks static -m navigation-demo-swup --force --zip --out exports/navigation-demo-swup
```

`--force` replaces the generated files in `modules/navigation-demo-swup/rendered/`. The command prints the path to a timestamped ZIP in `exports/navigation-demo-swup/`.

Extract the ZIP into an empty folder. It contains `index.html`, seven other HTML pages, and `static/` assets. HyperBricks is no longer needed to run this export. The Google font needs an internet connection; the bundled CSS, JavaScript and Swup work locally.

## Serve the static export

Run one of the following options from the extracted folder containing `index.html`. Open [http://localhost:8080/](http://localhost:8080/) and stop the server with Ctrl+C. Serve this folder at the website root, because links and asset paths begin with `/`. Opening the HTML files directly with `file://` will not support Swup navigation.

### Node.js: serve (recommended)

With Node.js and npm installed, create `serve.json` in the extracted folder:

```json
{
  "cleanUrls": true
}
```

Then run:

```sh
npx serve . --listen tcp://127.0.0.1:8080
```

Accept the package installation prompt if shown. `cleanUrls` lets `/last-bite` return `last-bite.html`, matching the demo's links. Do not add `--single` or `-s`: this is a multi-page site, and each route must return its own HTML.

See the [serve documentation](https://github.com/vercel/serve) and [clean URL configuration](https://github.com/vercel/serve-handler#cleanurls-booleanarray).

### Python 3 alternative

Plain `python3 -m http.server` does not resolve `/last-bite` to `last-bite.html`. For this export, run the following small server from the extracted folder. It adds that lookup while preserving normal asset and index-file serving:

```sh
python3 - <<'PYTHON'
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

class CleanURLHandler(SimpleHTTPRequestHandler):
    def translate_path(self, path):
        resolved = super().translate_path(path)
        if not Path(resolved).exists() and Path(resolved + ".html").is_file():
            return resolved + ".html"
        return resolved

print("Serving at http://localhost:8080/", flush=True)
ThreadingHTTPServer(("127.0.0.1", 8080), CleanURLHandler).serve_forever()
PYTHON
```

This uses Python's built-in [http.server](https://docs.python.org/3/library/http.server.html) for local previews.

After starting either server, visit `/last-bite` directly, reload it, then follow the menu and use Back/Forward. All eight routes should load their own content and retain the animated transitions when reduced motion is off.

## Pages

- `/` — Explore
- `/night-owl-cafe`
- `/side-b-records`
- `/skyline-cinema`
- `/last-bite`
- `/how-it-works` — developer walkthrough of rendering, menus, transitions, and source files

## Café translations with Spaces

Night Owl Café is also an editable Spaces example:

- `/night-owl-cafe` — English source page.
- `/night-owl-cafe/de` — German Space inheriting `night_owl_cafe_page`.
- `/night-owl-cafe/nl` — Dutch Space inheriting `night_owl_cafe_page`.

The café page links all three languages. The neighbourhood guide remains English. The German and Dutch instances share the English source's template, styles, JavaScript, and editing rules; they override translated text, document language, title, and description metadata. They use a separate section to keep the main guide menu at four venues.

Open [Manage Spaces](http://localhost:8125/__hyperbricks/spaces) to edit either translation. The How it works page links to each Space’s editing form and its contextual `?edit=true` page. Editor links and contextual edit mode use full-page navigation so the editor loads and exits cleanly. The development dashboard is at `/__hyperbricks/dashboard`. Local development writes are enabled in the package configuration. In development mode, save, wait for the source watcher to reload, then refresh the public page. In debug mode or with watching disabled, reload or restart the runtime after saving, then refresh the page. The editor is not included in static exports.

If both developer credentials are configured, use the same login for the
Dashboard's Overview and Errors views, Spaces, and contextual editing. A missing
or incorrect browser login receives `401`; a partial configured account receives
`503`. With both credentials absent these sections open without login. Spaces
still enforces the [LAN host settings](#lan-access-to-spaces). The editor also
works in debug mode and is unavailable in live mode, production, and static
output.

Editable fields live on the café source in `hyperbricks/app.hyperbricks.yaml`, under `body.values.content.editable`. The instances and their managed import index live in `hyperbricks/spaces/night_owl_cafe_page/`. See the [Spaces walkthrough](../../docs/SPACES.md#run-the-night-owl-cafe-example) for the runnable example and creation commands.

## How it is built

Each page declares `section: guide_navigation` and an `index`. The `menu` component in `hyperbricks/partials/navigation.hyperbricks.yaml` generates the navigation using `sort: index`, including `aria-current="page"` for the active route. There is no JavaScript menu registry.

The developer page uses a separate `developer_navigation` section at the top-right of the header.

Swup replaces `#swup`, `#guide-navigation`, and `#developer-navigation` from the complete server response. The header shell and footer stay in place. Shared templates and esbuild assets follow the existing module pattern. Venue content is supplied through template values in `hyperbricks/app.hyperbricks.yaml`.

CSS supplies a short fade and an 8px entrance with a small stagger on the directory rows. The OS reduced-motion preference disables animated visits, including when the preference changes while the page is open. A page-view hook focuses the new main region without scrolling and announces the page title. Swup handles document titles and browser history; the page-view hook updates the document language from the new main region; default nonanimated history visits retain native scroll restoration. Cache is disabled so development edits appear on the next visit.

Every link works without JavaScript. Direct URLs and reloads return full HTML. External footer links use ordinary browser navigation. The public guide has no forms, storage, API actions, image assets, or frontend-rendered pages. The built-in development editor persists Space edits to YAML.

## Verify

Open each route directly; follow the section menu and Next stop links; check the title and active menu after each visit. Check Back/Forward, reload, narrow screens, keyboard navigation, reduced motion, and operation without JavaScript. See VENDOR.md for the library source and license.
