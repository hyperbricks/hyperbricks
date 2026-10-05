# Catalog Store

A small, runnable store for learning HyperBricks v1.2.9-beta. It has a searchable
catalog, category filters, product details, an in-memory cart, and a **mock**
checkout. No payment is collected and no order is sent.

The storefront is rendered by HyperBricks. A small Python API supplies catalog
data and cart state so you can see how `api_render` composes pages and how
`api_fragment_render` updates part of a page. HTMX handles browser requests and
swaps. There is no npm install, database, Docker container, or Go plugin build.

![Catalog Store on desktop](docs/screenshots/catalog.png)

## Run it

You need a `hyperbricks` binary with `start --with-processes`, and Python 3.9
or newer. From the project root (the directory containing `modules/`), start
both the API and site with one command:

```sh
hyperbricks start -m catalog-store --with-processes
```

Open [http://127.0.0.1:4320/catalog](http://127.0.0.1:4320/catalog). The first
catalog request can take a moment: the API asks HyperBricks to render the 24
`<IMAGE>` components, then caches their generated `/static/images/` URLs for
this run. The API listens only on `127.0.0.1:4319`; HyperBricks uses port 4320.
The managed API starts first and must answer `/health` before HyperBricks begins
serving. Ctrl+C or `q` stops the site and its API. The two empty runtime
directories are included, so `hyperbricks init` is not needed.

If you are developing HyperBricks from its source checkout and have not
installed the binary, run `go install ./cmd/hyperbricks` there first. Run
`hyperbricks version` to check which version is on your path.

The public store runs without a developer login. Its dashboard is disabled by
default. If you enable Dashboard in `package.hyperbricks.yaml`, it opens without
login when both developer credentials are absent, with a startup warning. To
require login, set both values before startup; there is no built-in account:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
```

A partial account blocks access with `503`. Restart after changing package
settings or these variables. Dashboard is available in development and debug
mode, and excluded from live mode, production runtimes, and static output.
Frontend editing is explicitly disabled in this module. See
[developer access settings](../../docs/SPACES.md#development-configuration).

If the default ports are occupied, set both the API's listening port and the
URL used by `api_render`. The site port can be overridden independently:

```sh
CATALOG_API_PORT=14319 CATALOG_API_URL=http://127.0.0.1:14319 \
  hyperbricks start -m catalog-store --port 14320 --with-processes
```

Open `http://127.0.0.1:14320/catalog` in this case. HyperBricks passes the
effective site port to [`demo-api/server.sh`](demo-api/server.sh), so product
image generation uses the correct address. `CATALOG_API_PORT` also configures
the managed service readiness check. Both API variables must describe the same
port.

You can still run the processes separately. From the project root, open two
terminals:

```sh
python3 modules/catalog-store/demo-api/server.py
```

```sh
hyperbricks start -m catalog-store
```

Without `--with-processes`, HyperBricks leaves the API under your control; stop
it in its own terminal. To use different ports this way, pass `--port` and
`--site-url` to `server.py`, then set `PORT` and `CATALOG_API_URL` for
HyperBricks.

If the API is not running, the catalog shows a retry message. Check
`http://127.0.0.1:4319/health` and start the API, then use **Retry** or refresh.

## Try it

1. Search for “lamp,” select a category, and move between result pages. These
   controls ask HyperBricks for HTML fragments; they do not reload the document.
2. Open a product. Try an unknown ID such as
   `/catalog/detail?id=does-not-exist`: both the page and its fragment return
   HTTP 404 rather than a soft 404 page.
3. Add an item, change its quantity, and open the cart. The API keeps each
   browser session's cart in memory. Restarting the API clears it.
4. Continue to checkout and place a mock order. The form validates the entered
   fields, then clears the cart. It does not ask for payment details.

## Where to look

| File | What it does |
| --- | --- |
| [`hyperbricks/catalog.hyperbricks.yaml`](hyperbricks/catalog.hyperbricks.yaml) | Routes for pages, fragments, cart actions, and response statuses. |
| [`hyperbricks/partials/api.hyperbricks.yaml`](hyperbricks/partials/api.hyperbricks.yaml) | Reusable `api_render` definitions and allowed query keys. |
| [`hyperbricks/partials/media.hyperbricks.yaml`](hyperbricks/partials/media.hyperbricks.yaml) | The `<IMAGE>` component instances that generate static product images. |
| [`hyperbricks/partials/site.hyperbricks.yaml`](hyperbricks/partials/site.hyperbricks.yaml) | Shared page shell, font preloads, and native esbuild CSS/JS bundles. |
| [`templates/`](templates/) | Server-rendered catalog, product, cart, checkout, and fragment markup. |
| [`resources/js/catalog.js`](resources/js/catalog.js) | Small HTMX companion for detail-page state and notification timing. |
| [`package.hyperbricks.yaml`](package.hyperbricks.yaml) | Optional managed API, readiness URL, and site port configuration. |
| [`demo-api/server.sh`](demo-api/server.sh) | Starts the API in the foreground with the effective site port; HyperBricks stops it on quit. |
| [`demo-api/server.py`](demo-api/server.py) | Local JSON API and in-memory demo cart. |
| [`demo-api/catalog.json`](demo-api/catalog.json) | The 24 example products. |

`response_status` on the mounted detail `api_render` maps an upstream 404 to
the page response. The same policy is used on the detail fragment. The feed
fragment maps an upstream 503. These policies are optional; the other routes
use their normal HyperBricks response behavior.

The Python service lives in `demo-api/` because this example teaches an
**external HTTP API**. Turning it into a Go plugin would add a build step and
would no longer demonstrate the same `api_render` boundary. For a real store,
replace the demo API with your own service and persistent cart/order storage.

## Change the catalog

Edit [`demo-api/catalog.json`](demo-api/catalog.json) for names, categories,
descriptions, and prices. Product images are under
[`resources/images/products/`](resources/images/products/). The sample uses
IDs `item-01` through `item-24`; the image component bank in
[`media.hyperbricks.yaml`](hyperbricks/partials/media.hyperbricks.yaml) uses the
same IDs. When adding or removing products, update both places. Restart the API
and HyperBricks after changing these files; development watching is off here.

The frontend fonts and HTMX source are served locally. Their provenance and
licenses are in [VENDOR.md](VENDOR.md). The product images were generated for
this example.

## Quick checks

```sh
hyperbricks doctor -m modules/catalog-store
curl -fsS http://127.0.0.1:4319/health
curl -fsS http://127.0.0.1:4320/catalog >/dev/null
```

The API is a local teaching fixture, not a production commerce backend. Cart
state disappears when it stops, and checkout confirmation is only a demo.
