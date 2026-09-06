# Goja Render

> **Note: goja_render is intended for testing with trusted project scripts. It is not a security sandbox for scripts supplied by client users.** 

The component is ready for beta use with JavaScript written and reviewed as part of your HyperBricks project. It is not a security sandbox for scripts supplied by visitors, customers, or other untrusted authors.

`goja_render` runs JavaScript on the server and inserts the returned data into a
Go HTML template. It is built into HyperBricks, so you do not need a Node server
or a separate plugin build. The browser receives finished HTML without needing
to run the calculation itself.

This is useful for small, synchronous calculations such as price estimates,
availability messages, configuration summaries, and printable results.

`goja_render` is a practical alternative to a custom HyperBricks plugin for
small, synchronous server-side logic. It avoids building and deploying a
separate plugin when the logic only needs configured values and selected query
parameters. Use a Go plugin when you need persistent state, external services,
filesystem or database access, or heavier processing.

Use a normal Go template when you only need simple formatting. Use browser
JavaScript for interactions that only matter after the page loads.

## Quick start

Create a JavaScript file at `resources/scripts/availability.js`:

```javascript
function main(input) {
  const quantity = Number(input.query.quantity || 1);
  const stock = Number(input.values.stock);

  if (!Number.isInteger(quantity) || quantity < 1) {
    return { message: "Choose a valid quantity." };
  }

  return {
    message: quantity <= stock ? "In stock" : "Not enough stock"
  };
}
```

Add the component to a page or another renderable object:

```yaml
availability:
  - type: goja_render
  - script:
      file:
        base: resources
        path: scripts/availability.js
  - querykeys: [quantity]
  - values:
      stock: 12
  - timeout: 100ms
  - inline: '<p>{{.Data.message}}</p>'
```

Start the module and open the route with `?quantity=2`. HyperBricks passes the
configured `stock` value and the allowed `quantity` query parameter to
`main(input)`. The returned `message` is available to the template as
`.Data.message`.

The main fields are:

| Field | Purpose |
| --- | --- |
| `script` | JavaScript source containing `function main(input)`. Use the file resolver for a project script. |
| `values` | Configuration data available as `input.values`. |
| `querykeys` | Query parameters allowed into `input.query`. None are exposed when this field is omitted. |
| `timeout` | Maximum script runtime. The default is `100ms`; the maximum is `5s`. |
| `inline` | Inline Go HTML template that renders the returned object through `.Data`. |
| `template` | A template file to use instead of `inline`. Choose exactly one of the two. |

For a larger template, load it from the module's template directory:

```yaml
  - template:
      file: availability.html
```

Template output uses Go's contextual HTML escaping, so values returned by the
script are safely escaped for their position in the HTML. The usual `enclose`
field can wrap the completed output.

## Example: calculate a line total

This example combines a quantity from the URL with a unit price from the
HyperBricks configuration. The JavaScript calculates the total on the server,
and the template places the result in the HTML response.

Create `resources/scripts/line-total.js`:

```javascript
function main(input) {
  const quantity = Number(input.query.quantity || 1);
  const unitPrice = Number(input.values.unit_price);

  if (!Number.isInteger(quantity) || quantity < 1) {
    return {
      message: "Choose a whole quantity of 1 or more."
    };
  }

  const total = quantity * unitPrice;

  return {
    message: `${quantity} items cost €${total.toFixed(2)}`
  };
}
```

Add the component to a page:

```yaml
line_total:
  - type: goja_render
  - script:
      file:
        base: resources
        path: scripts/line-total.js
  - querykeys:
      - quantity
  - values:
      unit_price: 19.95
  - inline: '<p>{{.Data.message}}</p>'
```

If the component is on the `/price` route, opening `/price?quantity=3` renders:

```html
<p>3 items cost €59.85</p>
```

Only `quantity` is read from the URL because it is listed under `querykeys`.
The configured `unit_price` is available through `input.values` and cannot be
overridden with another query parameter. The object returned by `main(input)`
is exposed to the template as `.Data`.

## What the script receives and returns

HyperBricks calls the script as `main(input)`. The input contains two objects:

| Input | Contents |
| --- | --- |
| `input.values` | The data configured under `values` in YAML. |
| `input.query` | Only the URL parameters named under `querykeys`. |

A query parameter normally arrives as a string. If the same parameter appears
more than once, it arrives as an array of strings. Convert numbers explicitly
with `Number(...)` and validate all query input before using it.

`main(input)` must return a plain, JSON-compatible object:

```javascript
function main(input) {
  return {
    title: "Estimate",
    total: 42.50,
    available: true
  };
}
```

The template reads these values as `.Data.title`, `.Data.total`, and
`.Data.available`. Arrays and nested plain objects are supported inside the
returned object. Functions, `undefined`, `BigInt`, circular data, promises, and
non-finite numbers such as `NaN` are not supported.

### How requests stay separate

Each time a `goja_render` component renders, HyperBricks creates a fresh
[Goja](https://github.com/dop251/goja) JavaScript runtime for that execution.
The script and template are prepared when the module loads, but the
[JavaScript runtime itself is new](../pkg/gojaruntime/program.go) for every
render. Globals, modified prototypes, and other JavaScript state disappear when
the render finishes. Two clients making requests at the same time therefore do
not share JavaScript variables or objects.

Before calling `main(input)`, the
[component prepares the input](../pkg/component/goja_render.go) by converting
the configured `values` and that request's allowed query parameters to JSON.
This gives the script its own data instead of references to shared Go objects.
The returned value crosses the same boundary as plain JSON-compatible data, and
the temporary runtime is then discarded. HyperBricks also
[marks routes containing `goja_render` as `no-store`](../cmd/hyperbricks/initialize_goja.go),
so a completed response is not cached and served to another client.

Scripts are loaded and checked when the module loads. Changes take effect after
the normal development reload or a restart. Keep request-specific work inside
`main(input)`. For information that must survive a request, such as a session,
cart, or counter, use persistent storage through a Go component or plugin.

## Errors and beta limits

Syntax errors, a missing `main` function, invalid return data, timeouts, and
template errors are reported as normal HyperBricks component errors. A failed
render does not produce partial component HTML, and the next request starts with
a fresh runtime.

The current beta has these boundaries:

- Scripts are synchronous. Promises and asynchronous JavaScript are not supported.
- The default timeout is `100ms`, configurable up to `5s`.
- Script source, input data, and serialized result data are each limited to 1 MiB.
- Routes containing `goja_render` use `Cache-Control: no-store`; page and script
  result caching cannot currently be enabled for those routes.
- Filesystem, network, process, environment, and Node.js APIs are not provided.
- The timeout and fresh runtime improve request isolation, but they do not make
  Goja a security or memory sandbox. Only run project scripts you trust.

Request values are passed as data and are never evaluated as JavaScript source.
Project data enters the script through the configuration and allowed query values
in `input`; the script does not receive direct access to HyperBricks internals.

## Performance

HyperBricks compiles the JavaScript and parses the template when the module
loads. Each render reuses those prepared resources and creates a fresh Goja
runtime with new request data. The JavaScript source is therefore not compiled
again for every request.

In the repository's focused benchmark on an Apple M3, a small calculation and
template produced these approximate results:

| Rendering path | Time per render | Memory per render |
| --- | ---: | ---: |
| Native Go and template | 0.73 µs | 1 KB |
| Isolated Goja and template | 14.5 µs | 27 KB |

The Goja version is around 20 times slower than the very small native Go
baseline, but its total measured time is about `0.015 ms` per render. This
[benchmark](../pkg/component/goja_render_benchmark_test.go) measures one
component in isolation; it does not represent complete HTTP latency or maximum
server capacity.

This cost is reasonable for small calculations and formatting logic. Complex
scripts, several `goja_render` components on one page, or very high request
volume increase the cost. For those workloads, use a Go plugin and measure the
complete route under realistic traffic.

`goja_render` is powered by the [Goja JavaScript runtime](https://github.com/dop251/goja).
