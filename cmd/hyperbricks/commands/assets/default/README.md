# HyperBricks Starter

A three-page starting project demonstrating section menus, templates and HTMX 4 fragment updates.

## Requirements

- Use the HyperBricks release that generated this starter, or a compatible newer release.

## Run

From the project root:

```sh
hyperbricks start -m __MODULE_SHELL__ --port 8080
```

Open http://localhost:8080/. Native esbuild bundles the local HTMX 4.0.0 distribution and CSS. No npm installation, external API or plugin is required.

## Export a static ZIP

From the repository root, render the three pages and the status fragment and package their assets:

```sh
hyperbricks static -m __MODULE_SHELL__ --force --zip --out __EXPORT_SHELL__
```

`--force` replaces the generated files in `modules/__MODULE_NAME__/rendered/`. The command prints the path to a timestamped ZIP in `exports/__MODULE_NAME__/`.

Extract the ZIP into an empty folder. It contains `index.html`, `templates.html`, `fragments.html`, `hello-status.html`, and `static/` assets. HyperBricks is no longer needed to serve the export. CSS and HTMX are bundled locally; this demo uses system fonts.

The status fragment is a snapshot taken at export time. HTMX still requests and displays that HTML, and menu navigation still works. Any future server-side calculations or data changes would require a running HyperBricks server or a new export.

## Serve the static export

Run one of the following options from the extracted folder containing `index.html`. Open [http://localhost:8080/](http://localhost:8080/) and stop the server with Ctrl+C. Serve this folder at the website root, because links and asset paths begin with `/`. Opening the HTML files directly with `file://` will not support HTMX navigation.

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

Accept the package installation prompt if shown. `cleanUrls` lets `/fragments` return `fragments.html`, matching the demo's links. Do not add `--single` or `-s`: this is a multi-page site, and each route must return its own HTML.

See the [serve documentation](https://github.com/vercel/serve) and [clean URL configuration](https://github.com/vercel/serve-handler#cleanurls-booleanarray).

### Python 3 alternative

Plain `python3 -m http.server` does not resolve `/fragments` to `fragments.html`. For this export, run the following small server from the extracted folder. It adds that lookup while preserving normal asset and index-file serving:

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

After starting either server, visit `/templates` and `/fragments` directly, reload, then follow the menu and use Back/Forward. On Fragments, select Load status fragment twice. The server must resolve `/hello-status` to `hello-status.html` as well as resolving the page URLs.

## Pages and menu

- `/`: Overview of routing and the update process.
- `/templates`: inline templates, file/environment values and nested trees.
- `/fragments`: a reusable template card and an interactive fragment response.

Navigation follows the section-menu pattern in `hyperbricks-patterns-yaml/hyperbricks/50-menu-htmx-demo.hyperbricks.yaml`. Pages share `section: scaffold_navigation` and use indexes 10, 20 and 30. The `menu` component generates links with `sort: index` and a distinct active template.

HTMX requests canonical page URLs, selects `#scaffold-content > *`, and refreshes `#scaffold-navigation` with `hx-select-oob`. Menu navigation pushes the URL into browser history. Without JavaScript the same links perform ordinary full-page navigation. Each route works directly and on reload.

## Fragment interaction

The Load status fragment link requests `/hello-status` with `hx-get`. `hx-target="#hello-status"` and `hx-swap="outerHTML"` replace only the response panel. The returned fragment retains the same ID, so subsequent requests work. The link remains available outside that panel. Without JavaScript its ordinary `href` opens the HTML fragment directly.

The response region uses `aria-live="polite"`. Request timeout is 10 seconds, and the status-fragment request does not update browser history. Filesystem path output is omitted.

## Files

- `hyperbricks/hello-world.hyperbricks.yaml`: page composition and fragment route.
- `hyperbricks/partials/assets.hyperbricks.yaml`: native esbuild configuration.
- `templates/`: reusable card and status response.
- `resources/css/app.css`: responsive layout and styling.
- `resources/js/app.js`: HTMX settings.
- `resources/vendor/htmx-4.0.0.js`: pinned library; see VENDOR.md and the attribution below.

## Verify

Visit all three pages through the menu, check the active link and title, then use Back/Forward and reload. On Fragments, select Load status fragment twice, and confirm the result appears without changing the URL. Reload to return to the initial state. Check narrow widths, keyboard focus, and the direct `/hello-status` route.

## Attribution

This demo uses [HTMX 4.0.0](https://four.htmx.org/) by [Big Sky Software](https://github.com/bigskysoftware/htmx). HTMX is distributed under the [Zero-Clause BSD license](https://github.com/bigskysoftware/htmx/blob/v4.0.0/LICENSE). The library is bundled locally with an ES-module export added for esbuild; its source and checksum are recorded in [VENDOR.md](VENDOR.md).
