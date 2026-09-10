# After Hours — Swup navigation demo

A text-only guide to four fictional evening venues. HyperBricks renders six complete pages; Swup 4.10.0 animates navigation between them.

## Run

First, [install the HyperBricks CLI](../../docs/HYPERBRICKS_CLI.md#install).

From the repository root:

```sh
hyperbricks start -m navigation-demo-swup
```

Open http://localhost:8125/. The library is vendored locally and native esbuild bundles JavaScript and CSS. No npm install or separate server is required.

## Export a static ZIP

From the repository root, render all six pages and package their assets:

```sh
hyperbricks static -m navigation-demo-swup --force --zip --out exports/navigation-demo-swup
```

`--force` replaces the generated files in `modules/navigation-demo-swup/rendered/`. The command prints the path to a timestamped ZIP in `exports/navigation-demo-swup/`.

Extract the ZIP into an empty folder. It contains `index.html`, five other HTML pages, and `static/` assets. HyperBricks is no longer needed to run this export. The Google font needs an internet connection; the bundled CSS, JavaScript and Swup work locally.

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

After starting either server, visit `/last-bite` directly, reload it, then follow the menu and use Back/Forward. All six routes should load their own content and retain the animated transitions when reduced motion is off.

## Pages

- `/` — Explore
- `/night-owl-cafe`
- `/side-b-records`
- `/skyline-cinema`
- `/last-bite`
- `/how-it-works` — developer walkthrough of rendering, menus, transitions, and source files

## How it is built

Each page declares `section: guide_navigation` and an `index`. The `menu` component in `hyperbricks/partials/navigation.hyperbricks.yaml` generates the navigation using `sort: index`, including `aria-current="page"` for the active route. There is no JavaScript menu registry.

The developer page uses a separate `developer_navigation` section at the top-right of the header.

Swup replaces `#swup`, `#guide-navigation`, and `#developer-navigation` from the complete server response. The header shell and footer stay in place. Shared templates and esbuild assets follow the existing module pattern. Venue content is supplied through template values in `hyperbricks/app.hyperbricks.yaml`.

CSS supplies a short fade and an 8px entrance with a small stagger on the directory rows. The OS reduced-motion preference disables animated visits, including when the preference changes while the page is open. A page-view hook focuses the new main region without scrolling and announces the page title. Swup handles document titles and browser history; default nonanimated history visits retain native scroll restoration. Cache is disabled so development edits appear on the next visit.

Every link works without JavaScript. Direct URLs and reloads return full HTML. External footer links use ordinary browser navigation. There are no forms, storage, API actions, image assets, or frontend-rendered pages.

## Verify

Open each route directly; follow the section menu and Next stop links; check the title and active menu after each visit. Check Back/Forward, reload, narrow screens, keyboard navigation, reduced motion, and operation without JavaScript. See VENDOR.md for the library source and license.
