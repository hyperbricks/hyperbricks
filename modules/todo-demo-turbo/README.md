# Little List — Hotwire Turbo

A small todo app served by HyperBricks, using **Hotwire Turbo 8.0.23** for HTML fragment updates and **localStorage** for persistence.

## Run

From the HyperBricks repository root:

```sh
go install ./cmd/hyperbricks
mkdir -p modules/todo-demo-turbo/rendered
hyperbricks start -m todo-demo-turbo
```

Open [localhost:8122](http://localhost:8122/). Native esbuild builds the JavaScript and CSS automatically. The browserlibrary is included locally; no npm install, CDN connection, plugin build or extra server is needed.

## Try it

1. Add a task, mark it complete, switch between All / Active / Completed, and delete it or clear completed tasks.
2. Type a note in the scratchpad. Change your list: the note stays because only the task panel updates.
3. Reload the page: tasks and your scratchpad note return from localStorage. Notes are saved automatically on every edit.
4. Open [How it works](http://localhost:8122/how-it-works) for the component overview.

## What this demonstrates

| HyperBricks feature | Where to look |
| --- | --- |
| Full pages with `hypermedia` and shared `inherit` definitions | [app.hyperbricks.yaml](hyperbricks/app.hyperbricks.yaml), [site.hyperbricks.yaml](hyperbricks/partials/site.hyperbricks.yaml) |
| A separate `fragment` route and general response headers | [app.hyperbricks.yaml](hyperbricks/app.hyperbricks.yaml): `/fragments/tasks` |
| Templates, query allowlisting, Sprig validation, filtering and progress counts | [views.hyperbricks.yaml](hyperbricks/partials/views.hyperbricks.yaml), [tasks.html](templates/tasks.html) |
| A `menu` built from page sections, with active navigation | [navigation.hyperbricks.yaml](hyperbricks/partials/navigation.hyperbricks.yaml) |
| `tree`, `text` and `html` for ordered composition | [views.hyperbricks.yaml](hyperbricks/partials/views.hyperbricks.yaml) |
| `head` and native `esbuild` for JavaScript/CSS | [assets.hyperbricks.yaml](hyperbricks/partials/assets.hyperbricks.yaml) |
| Browser storage and todo operations | [app.js](resources/js/app.js) |
| Scratchpad autosave and restore | [scratchpad.js](resources/js/scratchpad.js) |
| The Hotwire Turbo integration | [integration.js](resources/js/integration.js) |

Setting `src` on `<turbo-frame id="todo-panel">` loads a response containing the matching frame. Turbo performs the frame replacement; request failures and timeouts leave saved data unchanged.

The browser computes the candidate list and sends it to HyperBricks. The server template renders all task rows and progress HTML; application JavaScript does not generate task HTML or simulate server responses. Saving happens after a successful server-rendered update. The two page links use normal navigation.

## Storage and request data

- Storage keys: `todo-demo-turbo:tasks:v1` and `todo-demo-turbo:scratchpad:v1`. Each demo has a different key. Storage is scoped to the browser profile and origin, not to a logged-in account; tabs on the same origin share it.
- HyperBricks does not persist task data. The render-only GET request carries the list as query data. This avoids a plugin or database, but task text can appear in request logs. This is a local demo, not a private multi-user task service.
- Limit: 20 tasks and 120 characters per title. Templates validate structure, IDs and bounded input, and HTML-escape titles. No `safe` conversion is applied to task data.
- The fragment disables HyperBricks caching and sends `Cache-Control: no-store`. Filters always request fresh HTML.
- If the request fails, the candidate change is not saved. Use **Refresh list** after connectivity returns. If localStorage is unavailable, successful changes stay in memory until reload and the page reports this.
- Concurrent editing in different tabs uses last-write-wins localStorage; there is no shared account or collaborative conflict resolution.
- No Goja, Go plugin or core changes are used.

## Compare the three versions

- [HTMX](../todo-demo-htmx/README.md) — port 8121
- [Hotwire Turbo](../todo-demo-turbo/README.md) — port 8122
- [Unpoly](../todo-demo-unpoly/README.md) — port 8123

These are independent modules with the same UI and application logic. Differences are the library, its integration file, and the Turbo frame wrapper. They prove this todo/fragment workflow, not every feature of each library.

## Credits

Built with [HyperBricks](https://hyperbricks.org/) ([repository](https://github.com/hyperbricks/hyperbricks)). Browserlibrary sources, pinned versions and checksums are in [VENDOR.md](VENDOR.md); its license is included in [static/vendor/LICENSE.txt](static/vendor/LICENSE.txt). Native bundling uses [esbuild](https://esbuild.github.io/), under the [MIT license](https://github.com/evanw/esbuild/blob/main/LICENSE.md).
