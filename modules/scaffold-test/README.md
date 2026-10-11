# HyperBricks Starter

A three-page starting project demonstrating section menus, templates and HTMX 4 fragment updates.

## Requirements

- Use the HyperBricks release that generated this starter, or a compatible newer release.

## Run

From the project root:

```sh
hyperbricks start -m 'scaffold-test' --port 8080
```

Open http://localhost:8080/. Native esbuild bundles the local HTMX 4.0.0 distribution and CSS. No npm installation, external API or plugin is required.

## Export a static ZIP

From the repository root, render the three pages and the status fragment and package their assets:

```sh
hyperbricks static -m 'scaffold-test' --force --zip --out 'exports/scaffold-test'
```

`--force` replaces the generated files in `modules/scaffold-test/rendered/`. The command prints the path to a timestamped ZIP in `exports/scaffold-test/`.

Extract the ZIP into an empty folder. It contains `index.html`, `templates.html`, `fragments.html`, `hello-status.html`, and `static/` assets. The export can be hosted as ordinary static files without an application runtime. CSS and HTMX are bundled locally; this demo uses system fonts.

The status fragment is a snapshot taken at export time. HTMX still requests and displays that HTML, and menu navigation still works. Any future server-side calculations or data changes would require a running HyperBricks server or a new export.

## Serve the static export

From the extracted folder containing `index.html`, run:

```sh
hyperbricks serve .
```

Open the URL printed in the terminal, normally [http://127.0.0.1:8080/](http://127.0.0.1:8080/). If the port is occupied, add `--port 8081` or use `--port 0` to select an available port. Stop the server with **Ctrl+C**.

The command serves existing files without loading a module, rendering, or building assets. It requires no package configuration or additional server installation. Clean URLs let `/fragments` return `fragments.html`, matching the demo's links. Each route returns its own file; missing files return `404` without a single-page-app fallback or directory listing.

Serve this extracted folder at the website root, because links and asset paths begin with `/`. Opening the HTML files directly with `file://` will not support HTMX navigation. For public hosting, configure a static host with the same clean-URL behavior; `hyperbricks serve` is for local previews.

After starting the server, visit `/templates` and `/fragments` directly, reload, then follow the menu and use Back/Forward. On Fragments, select Load status fragment twice. The server resolves `/hello-status` to `hello-status.html` as well as resolving the page URLs.

## Pages and menu

- `/`: Overview of routing and the update process.
- `/templates`: inline templates, file/environment values and nested trees.
- `/fragments`: a reusable template card and an interactive fragment response.

Navigation follows the [section-menu pattern in the HyperBricks repository](https://github.com/hyperbricks/hyperbricks/blob/main/modules/hyperbricks-patterns-yaml/hyperbricks/50-menu-htmx-demo.hyperbricks.yaml). Pages share `section: scaffold_navigation` and use indexes 10, 20 and 30. The `menu` component generates links with `sort: index` and a distinct active template.

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
