**Licence:** MIT
**Version:** v1.2.3-beta

**Build time:** 2026-09-10 21:58 UTC


# HyperBricks Component Reference

This reference is generated from the runtime schema and YAML documentation fixtures. It is intentionally compact: field tables come from Go struct tags, while examples come from curated executable YAML fixtures.

Regenerate this reference and the root README with:

```bash
bash scripts/build_docs.sh
```

Schema version: 1

## component

### `<HTML>`


Raw HTML snippet for leaf content or small escaped blocks.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `trimspace` | `bool` | no | TrimSpace filters all leading and trailing white space removed, as defined by Unicode. |
| `value` | `string` | yes | The raw HTML content |

#### Example

Fixture: `html-@doc.hyperbricks.yaml.test`

Component for rendering raw HTML snippets.


```yaml
html:
  - type: html
  - enclose: <div>|</div>
  - value: |
      <p>HTML TEST</p>
```

Expected output:

```html
<div><p>HTML TEST</p></div>
```


### `<PLUGIN>`


Plugin renderer that delegates output to a loaded HyperBricks plugin.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `classes` | `list` | no | Optional CSS classes for the plugin output wrapper |
| `data` | `map` | no | Plugin-specific data passed to the renderer |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `plugin` | `string` | no | Name of the plugin to render |

#### Example

Fixture: `plugin-plugin.hyperbricks.yaml.test`

Name of the plugin to render


```yaml
plugin:
  - type: plugin
  - plugin: example
```

Expected output:

```html
Plugin example
```


### `<TEMPLATE>`


Template-backed component that binds scalar values and value-mounted bricks into generated HTML.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `enclose` | `string` | no | Enclosing property for the template rendered output |
| `inline` | `string` | no | Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `querykeys` | `list` | no | Set allowed proxy query keys |
| `queryparams` | `map` | no | Set proxy query keys in the configuration |
| `template` | `string` | no | Loads contents of a template file in the modules template directory |
| `values` | `map` | no | Key-value pairs for template rendering |

#### Example

Fixture: `template-@doc.hyperbricks.yaml.test`

`<TEMPLATE>` can be used nested in `<FRAGMENT>` or `<HYPERMEDIA>` types. It uses Go's standard html/template library.


```yaml
fragment:
  - type: fragment
  - content:
      - type: tree
      - featured_video:
          - type: template
          - inline: |
              <iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>
          - values:
              height: "400"
              src: https://www.youtube.com/watch?v=Wlh6yFSJEms
              width: "300"
      - fallback_video:
          - type: template
          - inline: |
              <iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>
          - values:
              height: "400"
              src: https://www.youtube.com/embed/tgbNymZ7vqY
              width: "300"
      - enclose: <div class="youtube_video">|</div>
myComponent:
  - type: template
  - inline: |
      <iframe width="{{.width}}" height="{{.height}}" src="{{.src}}"></iframe>
  - values:
      height: "400"
      src: https://www.youtube.com/embed/tgbNymZ7vqY
      width: "300"
```

Expected output:

```html
<div class="youtube_video">
        <iframe width="300" height="400" src="https://www.youtube.com/watch?v=Wlh6yFSJEms"></iframe>

        <iframe width="300" height="400" src="https://www.youtube.com/embed/tgbNymZ7vqY"></iframe>
            </div>
```


### `<TEXT>`


Plain text leaf node.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `value` | `string` | yes | The paragraph content |

#### Example

Fixture: `text-@doc.hyperbricks.yaml.test`


```yaml
text:
  - type: text
  - enclose: <span>|</span>
  - value: SOME VALUE
```

Expected output:

```html
<span>SOME VALUE</span>
```


## composite

### `<API_FRAGMENT_RENDER>`


Route-owning API fragment that always bypasses rendered-output caching and makes a fresh upstream request when invoked.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `body` | `string` | no | Raw request body. Use a scalar string value; nested objects are not parsed for this field. |
| `debug` | `bool` | no | Debug the response data |
| `debugpanel` | `bool` | no | Render a frontend debug panel when frontend_errors is enabled in modules package.hyperbricks.yaml |
| `enclose` | `string` | no | Wrapping property for the fragment rendered output |
| `endpoint` | `string` | yes | The API endpoint |
| `guard.auth.cookie` | `string` | no | Cookie name used to resolve the request token |
| `guard.auth.header` | `string` | no | Header name used to resolve the request token, defaults to Authorization |
| `guard.auth.scheme` | `string` | no | Optional header scheme, defaults to Bearer for Authorization headers |
| `guard.authorize.body` | `string` | no | Optional request body with $key placeholder interpolation from the incoming request |
| `guard.authorize.endpoint` | `string` | no | Optional authorization endpoint called before rendering |
| `guard.authorize.headers` | `map` | no | Optional headers sent to the authorization endpoint |
| `guard.authorize.method` | `string` | no | HTTP method for the authorization endpoint |
| `guard.enabled` | `bool` | no | Enable route guarding before the route is rendered |
| `guard.on_forbidden.default.headers` | `map` | no | HTTP response headers sent to the browser |
| `guard.on_forbidden.default.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `guard.on_forbidden.variants` | `list` | no | Ordered alternatives with when.request_headers, response.status and response.headers. All header values must match exactly; names are case-insensitive. The first match replaces the default response completely |
| `guard.on_unauthenticated.default.headers` | `map` | no | HTTP response headers sent to the browser |
| `guard.on_unauthenticated.default.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `guard.on_unauthenticated.variants` | `list` | no | Ordered alternatives with when.request_headers, response.status and response.headers. All header values must match exactly; names are case-insensitive. The first match replaces the default response completely |
| `guard.require.authenticated` | `bool` | no | Require an authenticated request before rendering |
| `guard.require.query` | `map` | no | Required query keys, set each key to true to enforce presence |
| `headers` | `map` | no | Optional HTTP headers for API requests |
| `index` | `int` | no | Index number is a sort order option for the api-fragment-render menu section. See MENU and MENU_TEMPLATE for further explanation |
| `inline` | `string` | no | Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `jwtclaims` | `map` | no | JWT claims to include when signing the bearer token |
| `jwtsecret` | `string` | no | When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request |
| `method` | `string` | yes | HTTP method to use for API calls, GET POST PUT DELETE etc... |
| `password` | `string` | no | Password for basic auth |
| `querykeys` | `list` | no | Set allowed proxy query keys |
| `queryparams` | `map` | no | Set proxy query keys in the configuration |
| `response.headers` | `map` | no | HTTP response headers sent to the browser |
| `response.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `route` | `string` | no | The route (URL-friendly identifier) for the fragment |
| `section` | `string` | no | The section the fragment belongs to |
| `setcookie` | `string` | no | Single Set-Cookie response template shorthand. Applied on any 2xx upstream response. |
| `setcookies` | `list` | no | Optional list of Set-Cookie response templates. Each entry becomes its own Set-Cookie header on any 2xx upstream response. |
| `template` | `string` | no | Loads contents of a template file in the modules template directory |
| `title` | `string` | no | The title of the fragment |
| `username` | `string` | no | Username for basic auth |
| `values` | `map` | no | Key-value pairs for template rendering |

#### Example

Fixture: `api-fragment-render-@doc.hyperbricks.yaml.test`

Expose a route that calls an upstream API and renders a fragment response. API fragment routes always bypass rendered-output caching and call the upstream whenever invoked.


```yaml
api_fragment:
  - type: api_fragment_render
  - endpoint: https://example.com/fragment
  - method: GET
  - route: api-fragment
```


### `<FRAGMENT>`


A `<FRAGMENT>` dynamically renders part of an HTML page, allowing updates without a full page reload.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `beautify` | `bool` | no | Override server.beautify for this object when rendered directly |
| `cache` | `string` | no | Cache expire string |
| `content_type` | `string` | no | content type header definition |
| `enclose` | `string` | no | Wrapping property for the fragment rendered output |
| `guard.auth.cookie` | `string` | no | Cookie name used to resolve the request token |
| `guard.auth.header` | `string` | no | Header name used to resolve the request token, defaults to Authorization |
| `guard.auth.scheme` | `string` | no | Optional header scheme, defaults to Bearer for Authorization headers |
| `guard.authorize.body` | `string` | no | Optional request body with $key placeholder interpolation from the incoming request |
| `guard.authorize.endpoint` | `string` | no | Optional authorization endpoint called before rendering |
| `guard.authorize.headers` | `map` | no | Optional headers sent to the authorization endpoint |
| `guard.authorize.method` | `string` | no | HTTP method for the authorization endpoint |
| `guard.enabled` | `bool` | no | Enable route guarding before the route is rendered |
| `guard.on_forbidden.default.headers` | `map` | no | HTTP response headers sent to the browser |
| `guard.on_forbidden.default.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `guard.on_forbidden.variants` | `list` | no | Ordered alternatives with when.request_headers, response.status and response.headers. All header values must match exactly; names are case-insensitive. The first match replaces the default response completely |
| `guard.on_unauthenticated.default.headers` | `map` | no | HTTP response headers sent to the browser |
| `guard.on_unauthenticated.default.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `guard.on_unauthenticated.variants` | `list` | no | Ordered alternatives with when.request_headers, response.status and response.headers. All header values must match exactly; names are case-insensitive. The first match replaces the default response completely |
| `guard.require.authenticated` | `bool` | no | Require an authenticated request before rendering |
| `guard.require.query` | `map` | no | Required query keys, set each key to true to enforce presence |
| `index` | `int` | no | Index number is a sort order option for the fragment menu section. See MENU and MENU_TEMPLATE for further explanation |
| `nocache` | `bool` | no | Explicitly disable cache |
| `response.headers` | `map` | no | HTTP response headers sent to the browser |
| `response.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `route` | `string` | no | The route (URL-friendly identifier) for the fragment |
| `section` | `string` | no | The section the fragment belongs to |
| `static` | `string` | no | Static file path associated with the fragment |
| `template.enclose` | `string` | no | Enclosing property for the template rendered output |
| `template.inline` | `string` | no | Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `template.querykeys` | `list` | no | Set allowed proxy query keys |
| `template.queryparams` | `map` | no | Set proxy query keys in the configuration |
| `template.template` | `string` | no | Loads contents of a template file in the modules template directory |
| `template.values` | `map` | no | Key-value pairs for template rendering |
| `title` | `string` | no | The title of the fragment |

#### Example

Fixture: `fragment-@doc.hyperbricks.yaml.test`

A FRAGMENT renders partial HTML without a full document wrapper. A browser library can load that HTML into an existing page.

This example configures a response header for [HTMX 4](https://four.htmx.org/): `HX-Trigger` tells HTMX to dispatch the `myEvent` event. HyperBricks sends the configured header; HTMX handles it in the browser.


```yaml
fragment:
  - type: fragment
  - profile_card:
      - type: template
      - inline: |
          <h2>{{.header}}</h2>
          <p>{{.text}}</p>
          {{.image}}
      - values:
          header: SOME HEADER
          image:
            - type: image
            - src: hyperbricks-yaml-test-files/assets/cute_cat.jpg
            - width: "800"
          text:
            - type: text
            - value: some text
  - response:
      headers:
        HX-Trigger: myEvent
```

Expected output:

```html
<h2>SOME HEADER</h2>
<p>some text</p>
<img src="/static/images/cute_cat_c8c4b21c311ec78f82af3515e383d5a4_w800_h800.jpg" width="800" height="800" alt="" />
```


### `<HEAD>`


Document head helper that assembles title, meta, CSS, and JavaScript.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `css` | `list` | no | CSS files to include |
| `favicon` | `string` | no | Path to the favicon for the hypermedia document |
| `js` | `list` | no | JavaScript files to include |
| `meta` | `map` | no | Metadata for the head section |
| `title` | `string` | no | The title of the hypermedia document |

#### Example

Fixture: `head-assets.hyperbricks.yaml.test`

HEAD keeps reserved css/js fields as properties while non-colliding named children remain ordered components.


```yaml
page:
  - type: hypermedia
  - route: head-assets

  - head:
      - type: head
      - title: YAML Head Fixture
      - favicon:
          path:
            base: resources
            path: favicon.svg
      - meta:
          description: Head properties stay properties.
      - css:
          - path:
              base: resources
              path: css/base.css
      - js:
          - path:
              base: resources
              path: js/app.js

      - inline_styles:
          - type: css
          - inline: |
              .fixture {
                color: green;
              }

      - inline_script:
          - type: javascript
          - inline: |
              console.log("yaml head fixture");
```

Expected output:

```html
<!DOCTYPE html><html><head><style>
.fixture {
  color: green;
}

</style><script>
console.log("yaml head fixture");

</script><meta name="generator" content="HyperBricks"><link rel="icon" type="image/x-icon" href="resources/favicon.svg">
<title>YAML Head Fixture</title>
<meta name="description" content="Head properties stay properties.">
<link rel="stylesheet" href="resources/css/base.css">
<script src="resources/js/app.js"></script>
</head><body></body></html>
```


### `<HYPERMEDIA>`


Route-owning page shell that renders the main HyperBricks document.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `beautify` | `bool` | no | Override server.beautify for this object when rendered directly |
| `bodytag` | `string` | no | Special body enclose with use of \|. Please note that this will not work when a `<HYPERMEDIA>`.template is configured. In that case, you have to add the bodytag in the template. |
| `cache` | `string` | no | Cache expire string |
| `content_type` | `string` | no | content type header definition |
| `cookies` | `list` | no | Set-Cookie values to include when serving this hypermedia |
| `doctype` | `string` | no | Alternative Doctype for the HTML document |
| `enclose` | `string` | no | Enclosure of the property for the hypermedia |
| `favicon` | `string` | no | Path to the favicon for the hypermedia |
| `guard.auth.cookie` | `string` | no | Cookie name used to resolve the request token |
| `guard.auth.header` | `string` | no | Header name used to resolve the request token, defaults to Authorization |
| `guard.auth.scheme` | `string` | no | Optional header scheme, defaults to Bearer for Authorization headers |
| `guard.authorize.body` | `string` | no | Optional request body with $key placeholder interpolation from the incoming request |
| `guard.authorize.endpoint` | `string` | no | Optional authorization endpoint called before rendering |
| `guard.authorize.headers` | `map` | no | Optional headers sent to the authorization endpoint |
| `guard.authorize.method` | `string` | no | HTTP method for the authorization endpoint |
| `guard.enabled` | `bool` | no | Enable route guarding before the route is rendered |
| `guard.on_forbidden.default.headers` | `map` | no | HTTP response headers sent to the browser |
| `guard.on_forbidden.default.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `guard.on_forbidden.variants` | `list` | no | Ordered alternatives with when.request_headers, response.status and response.headers. All header values must match exactly; names are case-insensitive. The first match replaces the default response completely |
| `guard.on_unauthenticated.default.headers` | `map` | no | HTTP response headers sent to the browser |
| `guard.on_unauthenticated.default.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `guard.on_unauthenticated.variants` | `list` | no | Ordered alternatives with when.request_headers, response.status and response.headers. All header values must match exactly; names are case-insensitive. The first match replaces the default response completely |
| `guard.require.authenticated` | `bool` | no | Require an authenticated request before rendering |
| `guard.require.query` | `map` | no | Required query keys, set each key to true to enforce presence |
| `head` | `map` | no | Configurations for the head section of the hypermedia |
| `headers` | `map` | no | HTTP response headers to include when serving this hypermedia |
| `htmltag` | `string` | no | The opening HTML tag with attributes |
| `index` | `int` | no | Index number is a sort order option for the hypermedia defined in the section field. See `<MENU>` for further explanation and field options |
| `nocache` | `bool` | no | Explicitly disable cache |
| `response.headers` | `map` | no | HTTP response headers sent to the browser |
| `response.status` | `int` | no | Browser HTTP status (200–599); omit to retain the route or guard default |
| `route` | `string` | no | The route (URL-friendly identifier) for the hypermedia |
| `section` | `string` | no | The section the hypermedia belongs to. This can be used with the component `<MENU>` for example. |
| `static` | `string` | no | Static file path associated with the hypermedia, for rendering out the hypermedia to static files. |
| `template.enclose` | `string` | no | Enclosing property for the template rendered output |
| `template.inline` | `string` | no | Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `template.querykeys` | `list` | no | Set allowed proxy query keys |
| `template.queryparams` | `map` | no | Set proxy query keys in the configuration |
| `template.template` | `string` | no | Loads contents of a template file in the modules template directory |
| `template.values` | `map` | no | Key-value pairs for template rendering |
| `title` | `string` | no | The title of the hypermedia site |

#### Example

Fixture: `hypermedia-@doc.hyperbricks.yaml.test`

HYPERMEDIA renders a complete HTML document. The route property defines its URL. Use `fragment` to return partial HTML without the document wrapper.


```yaml
css:
  - type: html
  - value: |
      <style>
          body {
              padding:20px;
          }
      </style>
hypermedia:
  - type: hypermedia
  - main_content:
      - type: tree
      - intro:
          - type: html
          - value: <p>SOME CONTENT</p>
  - head:
      - type: head
      - inline_head_style:
          - type: html
          - value: |
              <style>
                  body {
                      padding:20px;
                  }
              </style>
      - page_styles:
          - type: css
          - inline: |
              .content {
                  color:green;
              }
```

Expected output:

```html
<!DOCTYPE html><html><head>
<style>
    body {
        padding:20px;
    }
</style>
<style>
.content {
    color:green;
}

</style><meta name="generator" content="HyperBricks"></head><body><p>SOME CONTENT</p></body></html>
```


### `<TREE>`


Ordered container that renders nested child items in key order.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `enclose` | `string` | no | Wrapping property for the tree |

#### Example

Fixture: `tree-@doc.hyperbricks.yaml.test`

TREE description


```yaml
fragment:
  - type: fragment
  - content_groups:
      - type: tree
      - primary_group:
          - type: tree
          - primary_intro:
              - type: html
              - value: <p>SOME NESTED HTML --- 10-1</p>
          - primary_detail:
              - type: html
              - value: <p>SOME NESTED HTML --- 10-2</p>
      - secondary_group:
          - type: tree
          - secondary_intro:
              - type: html
              - value: <p>SOME NESTED HTML --- 20-1</p>
          - secondary_detail:
              - type: html
              - value: <p>SOME NESTED HTML --- 20-2</p>
```

Expected output:

```html
<p>SOME NESTED HTML --- 10-1</p><p>SOME NESTED HTML --- 10-2</p><p>SOME NESTED HTML --- 20-1</p><p>SOME NESTED HTML --- 20-2</p>
```


## data

### `<API_RENDER>`


Nested API fetcher with no upstream-response cache or nocache field. It makes a fresh upstream request whenever its parent route renders; the parent owns rendered-output caching.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `body` | `string` | no | Raw request body. Use a scalar string value; nested objects are not parsed for this field. |
| `debug` | `bool` | no | Debug the response data |
| `debugpanel` | `bool` | no | Render a frontend debug panel when frontend_errors is enabled in modules package.hyperbricks.yaml |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `endpoint` | `string` | yes | The API endpoint |
| `headers` | `map` | no | Optional HTTP headers for API requests |
| `inline` | `string` | no | Inline Go template source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `jwtclaims` | `map` | no | JWT claims to include when signing the bearer token |
| `jwtsecret` | `string` | no | When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request |
| `method` | `string` | yes | HTTP method to use for API calls, GET POST PUT DELETE etc... |
| `password` | `string` | no | Password for basic auth |
| `querykeys` | `list` | no | Set allowed proxy query keys |
| `queryparams` | `map` | no | Set proxy query keys in the configuration |
| `template` | `string` | no | Loads contents of a template file in the modules template directory |
| `username` | `string` | no | Username for basic auth |
| `values` | `map` | no | Key-value pairs for template rendering |

#### Example

Fixture: `api-render-@doc.hyperbricks.yaml.test`

Fetch an upstream API whenever this nested component executes and render the response through a template. The parent route owns rendered-output caching; api_render has no nocache field or upstream-response cache.


```yaml
api_render:
  - type: api_render
  - endpoint: https://example.com/api
  - method: GET
```


### `<GOJA_RENDER>`


Trusted server-side JavaScript with request-local state and template output.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `inline` | `string` | no | Inline Go HTML template. Mutually exclusive with template. |
| `querykeys` | `list` | no | Explicitly allowed query keys. No query parameters are exposed by default. |
| `script` | `string` | yes | JavaScript declaring main(input). Use the file resolver to load source from resources. |
| `template` | `string` | no | Preloaded Go template file. The script result is available as .Data. |
| `timeout` | `string` | no | Script deadline, default 100ms. Must be positive and at most 5s. |
| `values` | `map` | no | Plain input data, copied into each script execution. Nested components are not rendered. |

#### Example

Fixture: `goja-render-@doc.hyperbricks.yaml.test`

Run trusted server-side JavaScript in a fresh runtime per request. The returned object is available as .Data in the Go HTML template. Use the existing file resolver under script to load source from resources; see GOJA_RENDER.md for the file-based example and execution limits.


```yaml
greeting:
  - type: goja_render
  - script: |
      function main(input) {
        return { message: input.values.message };
      }
  - values:
      message: Hello from the server
  - inline: '<p>{{.Data.message}}</p>'
```

Expected output:

```html
<p>Hello from the server</p>
```


### `<JSON_RENDER>`

Aliases: `<JSON>`


Local JSON renderer that loads a file and feeds it into a template.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `debug` | `bool` | no | Debug the response data |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `file` | `string` | yes | Path to the local JSON file |
| `inline` | `string` | no | Inline Go template source used to render the loaded JSON. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `template` | `string` | no | Loads contents of a template file in the modules template directory |
| `values` | `map` | no | Key-value pairs for template rendering |

#### Example

Fixture: `json-@doc.hyperbricks.yaml.test`

Debug the response data


```yaml
local_json_test:
  - type: json_render
  - debug: "false"
  - file: hyperbricks-yaml-test-files/assets/quotes.json
  - inline: |-
      <h1>{{.someproperty}}</h1>
      <ul>
          {{range .Data.quotes}}
              <li><strong>{{.author}}:</strong> {{.quote}}</li>
          {{end}}
      </ul>
  - values:
      someproperty: Quotes!
```

Expected output:

```html
<h1>Quotes!</h1>
<ul>
    <li><strong>Rumi:</strong> Your heart is the size of an ocean. Go find yourself in its hidden depths.</li>
    <li><strong>Abdul Kalam:</strong> The Bay of Bengal is hit frequently by cyclones. The months of November and May, in particular, are dangerous in this regard.</li>
    <li><strong>Abdul Kalam:</strong> Thinking is the capital, Enterprise is the way, Hard Work is the solution.</li>
    <li><strong>Bill Gates:</strong> If You Can&#39;T Make It Good, At Least Make It Look Good.</li>
    <li><strong>Rumi:</strong> Heart be brave. If you cannot be brave, just go. Love&#39;s glory is not a small thing.</li>
</ul>
```


## menu

### `<MENU>`


Menu renderer that sorts and formats page links by section.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `active` | `string` | yes | Template for the active menu item. |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Template to enclose the menu items. |
| `item` | `string` | yes | Template for regular menu items. |
| `order` | `string` | no | The order of items in the menu ('asc' or 'desc'). |
| `section` | `string` | yes | The section of the menu to display. |
| `sort` | `string` | no | The field to sort menu items by ('title', 'route', or 'index'). |

#### Example

Fixture: `menu-items.hyperbricks.yaml.test`

MENU keeps route metadata separate from the selected menu component; the {!{main_menu}} scope renders the menu.


```yaml
home:
  - type: hypermedia
  - route: index
  - title: Home
  - section: main
  - index: 10

about:
  - type: hypermedia
  - route: about
  - title: About
  - section: main
  - index: 20

main_menu:
  - type: menu
  - section: main
  - sort: index
  - order: asc
  - item: '<a href="/{{.Route}}">{{.Title}}</a>'
  - active: '<strong>{{.Title}}</strong>'
  - enclose: '<nav>|</nav>'
```

Expected output:

```html
<nav><strong>Home</strong>
<a href="/about">About</a></nav>
```


## resources

### `<CSS>`


Stylesheet leaf that can emit inline CSS or link to a stylesheet.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `file` | `string` | no | file overrides link and inline, it loads contents of a file and renders it in a style tag. |
| `inline` | `string` | no | Inline CSS source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `link` | `string` | no | Use link for a link tag |

#### Example

Fixture: `css-@doc.hyperbricks.yaml.test`

### Inline css example
<div id="python" class="tab-content">
<pre><code class="language-html">
<p>oki</p>
</code></pre>
</div>

<div id="javascript" class="tab-content" style="display:none;">
<pre><code class="language-yaml">
css:
  - type: css
  - file: hyperbricks-yaml-test-files/assets/styles.css
  - attributes:
      media: screen
  - enclose: <style media="print">|</style>
</code></pre>
</div>


```yaml
css:
  - type: css
  - attributes:
      media: screen
  - enclose: <style media="print">|</style>
  - file: hyperbricks-yaml-test-files/assets/styles.css
```

Expected output:

```html
<style media="print">
  body {
      background-color: red;
  }
</style>
```


### `<ESBUILD>`


Native JavaScript, TypeScript, and CSS bundling with lazy cached or per-render builds.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `binary` | `string` | no | Optional external esbuild executable; empty uses the embedded Go API. |
| `cache` | `bool` | no | True reuses valid builds; false rebuilds on every component render. Default false. Independent of page caching. |
| `debug` | `bool` | no | Log effective build options, engine, and cache diagnostics. |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `entry` | `string` | yes | Source filename. Use path with an explicit resources base. |
| `external` | `list` | no | Import or asset URL patterns to leave unbundled, e.g. /static/vendor/*. |
| `fingerprint` | `bool` | no | Emit content-versioned JS/CSS filenames in the configured output directory. Default false. Old assets are retained. |
| `loader` | `map` | no | Extension loader overrides, e.g. .woff2: file or .png: dataurl. |
| `mangle` | `bool` | no | Advanced: mangle JavaScript properties using .*; may break external property contracts. Default false; not allowed for CSS-only entries. |
| `minify` | `bool` | no | Minify whitespace and syntax. Default false. |
| `minify_identifiers` | `bool` | no | Minify identifiers independently of whitespace/syntax. Default false. YAML also accepts the legacy minifyident alias. |
| `outfile` | `string` | yes | Output filename inside the configured static directory. Use an explicit static path base. |
| `sourcemap` | `bool` | no | Emit a linked source map. Default false. |
| `target` | `list` | no | Optional browser/language targets, e.g. chrome110, safari16, es2020. |

#### Example

Fixture: `esbuild-@doc.hyperbricks.yaml.test`

Build browser JavaScript, TypeScript, or CSS with embedded esbuild. Use path, not file, because the engine needs filenames rather than file contents. With cache true a render restores a validated persistent build or compiles, and later renders reuse valid output. With cache false every component render builds. Fingerprint true emits app.<hash>.js inside the configured directory and retains old assets. This example verifies configuration materialization; the esbuild-demo module and component integration tests exercise compilation and publication. See ESBUILD.md for CSS, migration, watching, and the complete option list.


```yaml
scripts:
  - type: esbuild
  - entry:
      path: {base: resources, path: js/main.js}
  - outfile:
      path: {base: static, path: js/app.js}
  - minify: true
  - cache: true
  - fingerprint: true
  - enclose: '<script src="|" defer></script>'
```


### `<IMAGE>`


Single image renderer with optional optimization and HTML output.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `alt` | `string` | no | Alternative text, automatically HTML-escaped. An empty value renders an empty alt attribute for decorative images; supply meaningful text for informative images. |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `class` | `string` | no | CSS class for styling the image |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `height` | `int` | no | Output height in integer pixels; omit or use 0 to preserve aspect ratio from width. Setting both dimensions resizes to that exact size. |
| `id` | `string` | no | Id of image |
| `loading` | `string` | no | Lazy loading strategy (e.g., 'lazy', 'eager') |
| `quality` | `int` | no | JPEG encoding quality from 1 to 100; omit or use 0 for 90. Does not affect PNG or GIF encoding. |
| `src` | `string` | yes | Local filesystem path to a JPEG, PNG, or GIF image, relative to the working directory unless absolute. Use a path resolver for module resources. Remote URLs and SVG processing are not supported. |
| `title` | `string` | no | The title attribute of the image |
| `width` | `int` | no | Output width in integer pixels; omit or use 0 to preserve aspect ratio from height. Omit both dimensions to keep the source size. |

#### Example

Fixture: `image-@doc.hyperbricks.yaml.test`

Process a local JPEG, PNG, or GIF. The generated /static/images/ URL works at nested routes and uses a fingerprint of the source bytes and resize settings. Width and height are integer pixels. See [Image usage](IMAGES.md) for module path resolvers, responsive CSS, gallery behavior, and migration notes.


```yaml
image:
  - type: image
  - alt: cat but cute
  - attributes:
      usemap: '#catmap'
  - class: class-a class-b class-c
  - id: '#cat'
  - quality: "90"
  - src: hyperbricks-yaml-test-files/assets/cute_cat.jpg
  - title: Some Cute Cat!
  - width: "100"
```

Expected output:

```html
<img src="/static/images/cute_cat_cf6e86a5019b7eff32b5cae8e570c67d_w100_h100.jpg" width="100" height="100" alt="cat but cute" title="Some Cute Cat!" class="class-a class-b class-c" id="#cat" usemap="#catmap" />
```


### `<IMAGES>`


Multiple image renderer for a directory of images.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `alt` | `string` | no | Alternative text, automatically HTML-escaped. An empty value renders an empty alt attribute for decorative images; supply meaningful text for informative images. |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `class` | `string` | no | CSS class for styling the image |
| `directory` | `string` | yes | Local filesystem directory containing JPEG, PNG, or GIF images. Reads files in filename order without descending into subdirectories; other extensions are skipped. |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `height` | `int` | no | Output height in integer pixels; omit or use 0 to preserve aspect ratio from width. Setting both dimensions resizes to that exact size. |
| `id` | `string` | no | Id of images with a index added to it |
| `loading` | `string` | no | Lazy loading strategy (e.g., 'lazy', 'eager') |
| `quality` | `int` | no | JPEG encoding quality from 1 to 100; omit or use 0 for 90. Does not affect PNG or GIF encoding. |
| `title` | `string` | no | The title attribute of the image |
| `width` | `int` | no | Output width in integer pixels; omit or use 0 to preserve aspect ratio from height. Omit both dimensions to keep each source size. |

#### Example

Fixture: `images-@doc.hyperbricks.yaml.test`

Process a directory in filename order. Each generated URL includes the source content and resize settings. A configured id gets an index suffix; no id is added when omitted. Invalid images produce render errors. See [Image usage](IMAGES.md) for file formats and gallery accessibility.


```yaml
images:
  - type: images
  - attributes:
      decoding: async
  - directory: hyperbricks-yaml-test-files/assets/
  - id: '#img_'
  - loading: lazy
  - width: "100"
```

Expected output:

```html
<img src="/static/images/cute_cat_cf6e86a5019b7eff32b5cae8e570c67d_w100_h100.jpg" width="100" height="100" alt="" id="#img_0" loading="lazy" decoding="async" />
<img src="/static/images/same_cute_cat_cf6e86a5019b7eff32b5cae8e570c67d_w100_h100.jpg" width="100" height="100" alt="" id="#img_1" loading="lazy" decoding="async" />
```


### `<JS>`

Aliases: `<JAVASCRIPT>`


JavaScript leaf that can emit inline script or link to a script file.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `file` | `string` | no | File overrides link and inline, it loads contents of a file and renders it in a script tag. |
| `inline` | `string` | no | Inline JavaScript source. Use a normal YAML string, or a YAML block scalar when the source spans multiple lines. |
| `link` | `string` | no | Use link for a script tag with a src attribute |

#### Example

Fixture: `javascript-@doc.hyperbricks.yaml.test`

Extra attributes like id, data-role, data-action, type


```yaml
js:
  - type: javascript
  - attributes:
      type: text/javascript
  - file: hyperbricks-yaml-test-files/assets/script.js
```

Expected output:

```html
<script type="text/javascript">
    console.log("Hello World!")
</script>
```


### `<STYLES>`


Stylesheet file renderer for project style assets.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `file` | `string` | yes | Path to the CSS file |

#### Example

Fixture: `styles-file.hyperbricks.yaml.test`

Path to the CSS file


```yaml
style:
  - type: styles
  - file: hyperbricks-yaml-test-files/assets/styles.css
```

Expected output:

```html
<style>
body {
    background-color: red;
}
</style>
```
