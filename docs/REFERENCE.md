# HyperBricks Component Reference

This reference describes HyperBricks runtime components and their fields.
Field tables are generated from the runtime schema and examples come from
curated executable YAML fixtures.

For YAML syntax, ordering, inheritance, imports, and resolvers, see
[YAML_USAGE.md](YAML_USAGE.md).

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

Component for rendering all your single or multiline snippets.


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
| `inline` | `string` | no | Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>> |
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

Request-time API fragment that forwards to an upstream endpoint and renders the response.

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
| `guard.on_forbidden.hx_redirect` | `string` | no | HTMX redirect target used for HX requests; defaults to redirect when omitted |
| `guard.on_forbidden.redirect` | `string` | no | Full-page redirect target used for non-HTMX requests |
| `guard.on_forbidden.status` | `int` | no | Override HTTP status code for this denied response |
| `guard.on_unauthenticated.hx_redirect` | `string` | no | HTMX redirect target used for HX requests; defaults to redirect when omitted |
| `guard.on_unauthenticated.redirect` | `string` | no | Full-page redirect target used for non-HTMX requests |
| `guard.on_unauthenticated.status` | `int` | no | Override HTTP status code for this denied response |
| `guard.require.authenticated` | `bool` | no | Require an authenticated request before rendering |
| `guard.require.query` | `map` | no | Required query keys, set each key to true to enforce presence |
| `headers` | `map` | no | Optional HTTP headers for API requests |
| `index` | `int` | no | Index number is a sort order option for the api-fragment-render menu section. See MENU and MENU_TEMPLATE for further explanation |
| `inline` | `string` | no | Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>> |
| `jwtclaims` | `map` | no | JWT claims to include when signing the bearer token |
| `jwtsecret` | `string` | no | When not empty it uses jwtsecret for Bearer Token Authentication. When empty it switches if configured to basic auth via http.Request |
| `method` | `string` | yes | HTTP method to use for API calls, GET POST PUT DELETE etc... |
| `password` | `string` | no | Password for basic auth |
| `querykeys` | `list` | no | Set allowed proxy query keys |
| `queryparams` | `map` | no | Set proxy query keys in the configuration |
| `response.hx_location` | `string` | no | allows you to do a client-side redirect that does not do a full page reload |
| `response.hx_push_url` | `string` | no | Pushes a new URL into the history stack |
| `response.hx_redirect` | `string` | no | can be used to do a client-side redirect to a new location |
| `response.hx_refresh` | `string` | no | if set to 'true' the client-side will do a full refresh of the page |
| `response.hx_replace_url` | `string` | no | Replaces the current URL in the location bar |
| `response.hx_reselect` | `string` | no | CSS selector that selects which part of the response is swapped in |
| `response.hx_reswap` | `string` | no | allows you to specify how the response will be swapped |
| `response.hx_retarget` | `string` | no | CSS selector that updates the target of the content update |
| `response.hx_trigger` | `string` | no | allows you to trigger client-side events |
| `response.hx_trigger_after_settle` | `string` | no | allows you to trigger client-side events after the settle step |
| `response.hx_trigger_after_swap` | `string` | no | allows you to trigger client-side events after the swap step |
| `route` | `string` | no | The route (URL-friendly identifier) for the fragment |
| `section` | `string` | no | The section the fragment belongs to |
| `setcookie` | `string` | no | Legacy shorthand for one Set-Cookie response template. Applied on any 2xx upstream response. |
| `setcookies` | `list` | no | Optional list of Set-Cookie response templates. Each entry becomes its own Set-Cookie header on any 2xx upstream response. |
| `template` | `string` | no | Loads contents of a template file in the modules template directory |
| `title` | `string` | no | The title of the fragment |
| `username` | `string` | no | Username for basic auth |
| `values` | `map` | no | Key-value pairs for template rendering |

#### Example

Fixture: `api-fragment-render-@doc.hyperbricks.yaml.test`

A `<FRAGMENT>` dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience.


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
| `guard.on_forbidden.hx_redirect` | `string` | no | HTMX redirect target used for HX requests; defaults to redirect when omitted |
| `guard.on_forbidden.redirect` | `string` | no | Full-page redirect target used for non-HTMX requests |
| `guard.on_forbidden.status` | `int` | no | Override HTTP status code for this denied response |
| `guard.on_unauthenticated.hx_redirect` | `string` | no | HTMX redirect target used for HX requests; defaults to redirect when omitted |
| `guard.on_unauthenticated.redirect` | `string` | no | Full-page redirect target used for non-HTMX requests |
| `guard.on_unauthenticated.status` | `int` | no | Override HTTP status code for this denied response |
| `guard.require.authenticated` | `bool` | no | Require an authenticated request before rendering |
| `guard.require.query` | `map` | no | Required query keys, set each key to true to enforce presence |
| `index` | `int` | no | Index number is a sort order option for the fragment menu section. See MENU and MENU_TEMPLATE for further explanation |
| `nocache` | `bool` | no | Explicitly disable cache |
| `response.hx_location` | `string` | no | allows you to do a client-side redirect that does not do a full page reload |
| `response.hx_push_url` | `string` | no | Pushes a new URL into the history stack |
| `response.hx_redirect` | `string` | no | can be used to do a client-side redirect to a new location |
| `response.hx_refresh` | `string` | no | if set to 'true' the client-side will do a full refresh of the page |
| `response.hx_replace_url` | `string` | no | Replaces the current URL in the location bar |
| `response.hx_reselect` | `string` | no | CSS selector that selects which part of the response is swapped in |
| `response.hx_reswap` | `string` | no | allows you to specify how the response will be swapped |
| `response.hx_retarget` | `string` | no | CSS selector that updates the target of the content update |
| `response.hx_trigger` | `string` | no | allows you to trigger client-side events |
| `response.hx_trigger_after_settle` | `string` | no | allows you to trigger client-side events after the settle step |
| `response.hx_trigger_after_swap` | `string` | no | allows you to trigger client-side events after the swap step |
| `route` | `string` | no | The route (URL-friendly identifier) for the fragment |
| `section` | `string` | no | The section the fragment belongs to |
| `static` | `string` | no | Static file path associated with the fragment |
| `template.enclose` | `string` | no | Enclosing property for the template rendered output |
| `template.inline` | `string` | no | Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>> |
| `template.querykeys` | `list` | no | Set allowed proxy query keys |
| `template.queryparams` | `map` | no | Set proxy query keys in the configuration |
| `template.template` | `string` | no | Loads contents of a template file in the modules template directory |
| `template.values` | `map` | no | Key-value pairs for template rendering |
| `title` | `string` | no | The title of the fragment |

#### Example

Fixture: `fragment-@doc.hyperbricks.yaml.test`

A FRAGMENT dynamically renders a part of an HTML page, allowing updates without a full page reload and improving performance and user experience.


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
            - src: hyperbricks-test-files/assets/cute_cat.jpg
            - width: "800"
          text:
            - type: text
            - value: some text
  - response:
      hx_trigger: myEvent
```

Expected output:

```html
<h2>SOME HEADER</h2>
<p>some text</p>
<img src="static/images/cute_cat_w800_h800.jpg" width="800" height="800" />
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

</script><meta name="generator" content="hyperbricks runtime"><link rel="icon" type="image/x-icon" href="resources/favicon.svg">
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
| `guard.on_forbidden.hx_redirect` | `string` | no | HTMX redirect target used for HX requests; defaults to redirect when omitted |
| `guard.on_forbidden.redirect` | `string` | no | Full-page redirect target used for non-HTMX requests |
| `guard.on_forbidden.status` | `int` | no | Override HTTP status code for this denied response |
| `guard.on_unauthenticated.hx_redirect` | `string` | no | HTMX redirect target used for HX requests; defaults to redirect when omitted |
| `guard.on_unauthenticated.redirect` | `string` | no | Full-page redirect target used for non-HTMX requests |
| `guard.on_unauthenticated.status` | `int` | no | Override HTTP status code for this denied response |
| `guard.require.authenticated` | `bool` | no | Require an authenticated request before rendering |
| `guard.require.query` | `map` | no | Required query keys, set each key to true to enforce presence |
| `head` | `map` | no | Configurations for the head section of the hypermedia |
| `headers` | `map` | no | HTTP response headers to include when serving this hypermedia |
| `htmltag` | `string` | no | The opening HTML tag with attributes |
| `index` | `int` | no | Index number is a sort order option for the hypermedia defined in the section field. See `<MENU>` for further explanation and field options |
| `nocache` | `bool` | no | Explicitly disable cache |
| `route` | `string` | no | The route (URL-friendly identifier) for the hypermedia |
| `section` | `string` | no | The section the hypermedia belongs to. This can be used with the component `<MENU>` for example. |
| `static` | `string` | no | Static file path associated with the hypermedia, for rendering out the hypermedia to static files. |
| `template.enclose` | `string` | no | Enclosing property for the template rendered output |
| `template.inline` | `string` | no | Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>> |
| `template.querykeys` | `list` | no | Set allowed proxy query keys |
| `template.queryparams` | `map` | no | Set proxy query keys in the configuration |
| `template.template` | `string` | no | Loads contents of a template file in the modules template directory |
| `template.values` | `map` | no | Key-value pairs for template rendering |
| `title` | `string` | no | The title of the hypermedia site |

#### Example

Fixture: `hypermedia-@doc.hyperbricks.yaml.test`

HYPERMEDIA type is the main initiator of a htmx document. Its location is defined by the route property. Use `<FRAGMENT>` to utilize hx-[method] (GET,POST etc) requests.


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

</style><meta name="generator" content="hyperbricks runtime"></head><body><p>SOME CONTENT</p></body></html>
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

Remote API fetcher that renders the upstream response through a template.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `body` | `string` | no | Raw request body. Use a scalar string value; nested objects are not parsed for this field. |
| `debug` | `bool` | no | Debug the response data |
| `debugpanel` | `bool` | no | Render a frontend debug panel when frontend_errors is enabled in modules package.hyperbricks.yaml |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `endpoint` | `string` | yes | The API endpoint |
| `headers` | `map` | no | Optional HTTP headers for API requests |
| `inline` | `string` | no | Use inline to define the template in a multiline block <<[ /* Template goes here */ ]>> |
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

Fetch a remote API endpoint and render the response through a template or inline template.


```yaml
api_render:
  - type: api_render
  - endpoint: https://example.com/api
  - method: GET
```


### `<JSON_RENDER>`

Local JSON renderer that loads a file and feeds it into a template.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `debug` | `bool` | no | Debug the response data |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `file` | `string` | yes | Path to the local JSON file |
| `inline` | `string` | no | Use inline to define the template in a multiline block <<[ /* Template code goes here */ ]>> |
| `template` | `string` | no | Loads contents of a template file in the modules template directory |
| `values` | `map` | no | Key-value pairs for template rendering |

#### Example

Fixture: `json-@doc.hyperbricks.yaml.test`

Debug the response data


```yaml
local_json_test:
  - type: json_render
  - debug: "false"
  - file: hyperbricks-test-files/assets/quotes.json
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
| `inline` | `string` | no | Use inline to define css in a multiline block <<[ /* css goes here */ ]>> |
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
<pre><code class="language-hyperbricks">
css = `<CSS>`
css.file = hyperbricks-test-files/assets/styles.css
css.attributes {
    media = screen
}
css.enclose = <style media="print">|</style>
</code></pre>
</div>


```yaml
css:
  - type: css
  - attributes:
      media: screen
  - enclose: <style media="print">|</style>
  - file: hyperbricks-test-files/assets/styles.css
```

Expected output:

```html
<style media="print">
  body {
      background-color: red;
  }
</style>
```


### `<IMAGE>`

Single image renderer with optional optimization and HTML output.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `alt` | `string` | no | Alternative text for the image |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `class` | `string` | no | CSS class for styling the image |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `height` | `int` | no | The height of the image (can be a number or percentage) |
| `id` | `string` | no | Id of image |
| `loading` | `string` | no | Lazy loading strategy (e.g., 'lazy', 'eager') |
| `quality` | `int` | no | Image quality for optimization |
| `src` | `string` | yes | The source URL of the image |
| `title` | `string` | no | The title attribute of the image |
| `width` | `int` | no | The width of the image (can be a number or percentage) |

#### Example

Fixture: `image-@doc.hyperbricks.yaml.test`


```yaml
image:
  - type: image
  - alt: cat but cute
  - attributes:
      usemap: '#catmap'
  - class: class-a class-b class-c
  - id: '#cat'
  - quality: "90"
  - src: hyperbricks-test-files/assets/cute_cat.jpg
  - title: Some Cute Cat!
  - width: "100"
```

Expected output:

```html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" alt="cat but cute" title="Some Cute Cat!" class="class-a class-b class-c" id="#cat" usemap="#catmap" />
```


### `<IMAGES>`

Multiple image renderer for a directory of images.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `alt` | `string` | no | Alternative text for the image |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `class` | `string` | no | CSS class for styling the image |
| `directory` | `string` | yes | The directory path containing the images |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `height` | `int` | no | The height of the images (can be a number or percentage) |
| `id` | `string` | no | Id of images with a index added to it |
| `loading` | `string` | no | Lazy loading strategy (e.g., 'lazy', 'eager') |
| `quality` | `int` | no | Image quality for optimization |
| `title` | `string` | no | The title attribute of the image |
| `width` | `int` | no | The width of the images (can be a number or percentage) |

#### Example

Fixture: `images-@doc.hyperbricks.yaml.test`

Id of images with a index added to it


```yaml
image:
  - enclose: <div id="#gallery">|</div>
images:
  - type: images
  - attributes:
      decoding: async
  - directory: hyperbricks-test-files/assets/
  - id: '#img_'
  - loading: lazy
  - width: "100"
```

Expected output:

```html
<img src="static/images/cute_cat_w100_h100.jpg" width="100" height="100" id="#img_0" loading="lazy" decoding="async" />
<img src="static/images/same_cute_cat_w100_h100.jpg" width="100" height="100" id="#img_1" loading="lazy" decoding="async" />
```


### `<JS>`

JavaScript leaf that can emit inline script or link to a script file.

| Field | Kind | Required | Description |
| --- | --- | --- | --- |
| `attributes` | `map` | no | Extra attributes like id, data-role, data-action |
| `enclose` | `string` | no | Wrap rendered output using prefix\|suffix syntax |
| `file` | `string` | no | File overrides link and inline, it loads contents of a file and renders it in a script tag. |
| `inline` | `string` | no | Use inline to define JavaScript in a multiline block <<[ /* JavaScript goes here */ ]>> |
| `link` | `string` | no | Use link for a script tag with a src attribute |

#### Example

Fixture: `javascript-@doc.hyperbricks.yaml.test`

Extra attributes like id, data-role, data-action, type


```yaml
js:
  - type: javascript
  - attributes:
      type: text/javascript
  - file: hyperbricks-test-files/assets/script.js
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
  - file: hyperbricks-test-files/assets/styles.css
```

Expected output:

```html
<style>
body {
    background-color: red;
}
</style>
```
