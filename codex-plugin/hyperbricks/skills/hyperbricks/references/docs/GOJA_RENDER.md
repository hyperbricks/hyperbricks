<!-- Generated from docs/GOJA_RENDER.md. Do not edit directly. -->

# Goja Render

Use `goja_render` to run JavaScript on the server and pass its result to a Go HTML template. HyperBricks includes the component; you do not need a Node.js server or a separate plugin build. The browser receives the rendered HTML.

The component is ready for beta use with scripts written and reviewed as part of your project. **It is intended for testing with trusted project scripts. It is not a security sandbox for code supplied by visitors, customers, or other untrusted authors.**

Use it for small, synchronous calculations, such as price estimates, availability messages, configuration summaries, and printable results. It can replace a custom plugin when the calculation only needs configured values and selected query parameters.

Use a Go plugin for persistent state, external services, filesystem or database access, or heavier processing. Use a Go template for simple formatting. Use browser JavaScript for interactions after the page loads.

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

Create `hyperbricks/availability.hyperbricks.yaml` in your module with the component and a page that renders it:

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

availability_page:
  - type: hypermedia
  - route: availability
  - title: Availability
  - content:
      - inherit: availability
```

Start the module and open `/availability?quantity=2` on your running server. HyperBricks passes the configured `stock` value and the allowed `quantity` query parameter to `main(input)`. The returned `message` is available to the template as `.Data.message`.

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

Template output uses Go's contextual HTML escaping, so values returned by the script are safely escaped for their position in the HTML. The usual `enclose` field can wrap the completed output.

## Example: calculate a line total

This example combines a quantity from the URL with a unit price from the HyperBricks configuration. The JavaScript calculates the total on the server, and the template places the result in the HTML response.

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

Only `quantity` is read from the URL because it is listed under `querykeys`. The configured `unit_price` is available through `input.values` and cannot be overridden with another query parameter. The object returned by `main(input)` is exposed to the template as `.Data`.

## What the script receives and returns

HyperBricks calls the script as `main(input)`. The input contains two objects:

| Input | Contents |
| --- | --- |
| `input.values` | The data configured under `values` in YAML. |
| `input.query` | Only the URL parameters named under `querykeys`. |

A query parameter normally arrives as a string. If the same parameter appears more than once, it arrives as an array of strings. Convert numbers explicitly with `Number(...)` and validate all query input before using it.

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

The template reads these values as `.Data.title`, `.Data.total`, and `.Data.available`. Arrays and nested plain objects are supported inside the returned object. Functions, `undefined`, `BigInt`, circular data, promises, and non-finite numbers such as `NaN` are not supported.

### How requests stay separate

HyperBricks prepares the script and template when the module loads. Each render creates a fresh [Goja JavaScript runtime](https://github.com/dop251/goja).

Globals, modified prototypes, and other JavaScript state disappear after the render. Concurrent requests do not share JavaScript variables or objects. See the [runtime implementation](https://github.com/hyperbricks/hyperbricks/blob/v1.3.0-beta/pkg/gojaruntime/program.go).

Before calling `main(input)`, the [component](https://github.com/hyperbricks/hyperbricks/blob/v1.3.0-beta/pkg/component/goja_render.go) converts configured `values` and allowed request query parameters to JSON. The script receives its own data, with no references to shared Go objects. It returns plain JSON-compatible data. HyperBricks then discards the temporary runtime.

HyperBricks loads and checks scripts when the module loads. Changes take effect after the normal development reload or a restart. Keep request-specific work inside `main(input)`. For information that must survive a request, such as a session, cart, or counter, use persistent storage through a Go component or plugin.

### Rendered-output caching and HTTP caching

Routes containing `goja_render` automatically bypass HyperBricks' internal
rendered-output cache. During module loading, [Goja preparation](https://github.com/hyperbricks/hyperbricks/blob/v1.3.0-beta/cmd/hyperbricks/initialize_goja.go)
sets the owning route's `nocache` to `true`, even if the configuration specified
`false`. Each request reaching that route renders again. The compiled script
and parsed template are still reused; execution state and results are not.

Browser and proxy caching is controlled separately by HTTP response headers.
To prevent them from storing the response, add this to the owning `hypermedia`
or `fragment` route:

```yaml
- response:
    headers:
      Cache-Control: no-store
```

The existing automatic header behavior differs by route type:

| Route owner | HTTP `Cache-Control` without an explicit `response.headers` policy |
| --- | --- |
| `hypermedia` | Goja preparation adds top-level `headers.Cache-Control: no-store`, which the HTTP server emits. |
| `fragment` | The HTTP server does not emit top-level `headers` for fragments, so Goja does not automatically supply this response header. Use `response.headers`. |

Explicit `response.headers.Cache-Control` takes precedence for both route types.
For example, an explicit HTTP caching policy can allow browser caching while
HyperBricks still renders every request it receives. Set `no-store` explicitly
when the response should not be stored by browsers or proxies.

## Errors and beta limits

HyperBricks reports syntax errors, a missing `main` function, invalid return data, timeouts, and template errors through its normal component diagnostics. A failed render does not produce partial component HTML, and the next request starts with a fresh runtime.

The current beta has these boundaries:

- Scripts are synchronous. Promises and asynchronous JavaScript are not supported.
- The default timeout is `100ms`, configurable up to `5s`.
- Script source, input data, and serialized result data are each limited to 1 MiB.
- Routes containing `goja_render` bypass HyperBricks' internal rendered-output cache. Configure browser and proxy caching separately through the route's `response.headers`.
- Filesystem, network, process, environment, and Node.js APIs are not provided.
- The timeout and fresh runtime improve request isolation, but they do not make Goja a security or memory sandbox. Only run project scripts you trust.

HyperBricks passes request values as data and never evaluates them as JavaScript source. Project data enters the script through the configuration and allowed query values in `input`; the script does not receive direct access to HyperBricks internals.

## Performance

HyperBricks compiles the JavaScript and parses the template when the module loads. Each render reuses those prepared resources and creates a fresh Goja runtime with new request data. The JavaScript source is therefore not compiled again for every request.

In the repository's focused benchmark on an Apple M3, a small calculation and template produced these approximate results:

| Rendering path | Time per render | Memory per render |
| --- | ---: | ---: |
| Native Go and template | 0.73 µs | 1 KB |
| Isolated Goja and template | 14.5 µs | 27 KB |

The Goja version is around 20 times slower than the very small native Go baseline, but its total measured time is about `0.015 ms` per render. This [benchmark](https://github.com/hyperbricks/hyperbricks/blob/v1.3.0-beta/pkg/component/goja_render_benchmark_test.go) measures one component in isolation; it does not represent complete HTTP latency or maximum server capacity.

This cost is reasonable for small calculations and formatting logic. Complex scripts, several `goja_render` components on one page, or very high request volume increase the cost. For those workloads, use a Go plugin and measure the complete route under realistic traffic.

`goja_render` is powered by the [Goja JavaScript runtime](https://github.com/dop251/goja).
