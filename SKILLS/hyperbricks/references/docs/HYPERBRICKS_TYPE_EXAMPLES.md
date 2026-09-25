<!-- Generated from docs/HYPERBRICKS_TYPE_EXAMPLES.md. Do not edit directly. -->

# HyperBricks Type Examples

Use these examples to configure all 20 native component types. They draw on the [patterns module](https://github.com/hyperbricks/hyperbricks/tree/v1.2.5-beta/modules/hyperbricks-patterns-yaml) and [Component reference](REFERENCE.md).

Each example shows selected fields and links to the full reference for defaults, constraints, and other options. A field missing from an example may still be supported or required.

## Conventions

- **Required:** a field marked required in the reference/schema.
- **Conditional:** needed for the selected source, output, or routing behavior.
- **Optional:** enriches the example; omit it when it is not needed.
- Every non-inheriting brick requires `type`; inheriting bricks may omit it.
- Children use named ordered sequences; ordinary property maps are not children.
- `enclose` uses `prefix|suffix`, where `|` is replaced by rendered output.
- Prefer unquoted scalars where YAML permits. Use literal blocks (`|`) for inline source and HTML values.

See [YAML usage](YAML_USAGE.md) for the native syntax and scalar rules.
The comments inside each snippet explain requirements and the selected behavior.

## Shared Prerequisites

These are examples to adapt, not an installed module. No server startup or
static export is needed to use this document. Paths use the project's configured
directories; referenced assets must already exist or be supplied separately.

The generic Hypermedia page uses `templates/page.html`:

```html
<main>
  <h1>{{.title}}</h1>
  {{.body}}
</main>
```

Supply browser source assets at `resources/js/main.js` and
`resources/css/main.css`. Other examples reference
`resources/content/page.md`, `resources/data/items.json`, and
`resources/images/example.jpg`; this document does not create those files.

The JSON file and item-list API examples expect this data shape:

```json
{
  "items": [
    {"title": "Coastal walking guide"},
    {"title": "Weekend market dates"}
  ]
}
```

Replace API endpoints and the plugin name with your own services and enabled
plugin contract.

## Fast Scaffold Starters

The interactive `hyperbricks scaffold` wizard uses a small core YAML library based
on the patterns and examples in this document. The library keeps the native source
shape visible and supplies only the values needed for a useful first result. It is
not a replacement for the complete field reference.

The available source choices are:

| Starter | Native type | Additional source | Route/title prompt |
| --- | --- | --- | --- |
| `hypermedia` | `hypermedia` | Inline Template with Markdown body | Both |
| `fragment` | `fragment` | Inline HTML child | Route |
| `tree` | `tree` | Inline HTML and text children | None |
| `head` | `head` | Metadata mapping | None |
| `api_fragment_render` | `api_fragment_render` | Inline response template | Route |
| `template (inline)` | `template` | Inline HTML template | None |
| `template (file)` | `template` | Generic HTML file in `templates/` | None |
| `text` | `text` | Inline scalar text | None |
| `html` | `html` | Multiline inline HTML | None |
| `markdown (inline)` | `markdown` | Inline Markdown block | None |
| `markdown (file)` | `markdown` | Generic Markdown file in `resources/` | None |
| `menu` | `menu` | Inline item and active links | None |
| `esbuild` | `esbuild` | Generic JavaScript entry in `resources/` | None |
| `css` | `css` | Inline CSS | None |
| `js` | `js` | Inline browser JavaScript | None |
| `styles` | `styles` | Generic CSS file in `resources/` | None |
| `image` | `image` | Generic PNG in `resources/` | None |
| `images` | `images` | Generic PNG directory in `resources/` | None |
| `json_render` | `json_render` | Generic JSON object in `resources/` | None |
| `goja_render` | `goja_render` | Inline script and response template | None |
| `api_render` | `api_render` | Inline response template | None |

File-backed starters use unique names so repeated wizard runs do not silently
overwrite one another. The wizard preserves an existing file when its generated
asset path is already present. API starters contain an explicit placeholder endpoint
and must be pointed at a real service before use. Plugin configuration is intentionally
project-specific and is authored with [Authoring](AUTHOR.md) after the plugin is
known and enabled.

The library is checked in as [the scaffold template library](https://github.com/hyperbricks/hyperbricks/blob/v1.2.5-beta/cmd/hyperbricks/commands/assets/scaffold/library.yaml).
Its generated assets live beside it under the same scaffold asset directory. This
keeps starter maintenance separate from the runtime component registry while still
making the examples native YAML. A preview confirms the exact YAML and files that
will be written; it does not run esbuild, Goja, an upstream API, or a browser.

The snippets form one related example set. The generic page inherits
`example_esbuild_css` and `example_esbuild_js` from the Resources section;
the inherited page depends on `example_page`. Keep these bricks in the same
loaded import scope, or replace the references with existing project bricks.
The menu discovers pages in the `example_pages` section.

## Composite

### Hypermedia

[Full field reference: `hypermedia`](REFERENCE.md#hypermedia)

#### Generic page

```yaml
# Hypermedia: no additional schema-required fields beyond type.
# route is conditional: required to expose this example as a browser page.
# This generic page pattern intentionally includes metadata, esbuild, and Template.
example_page:
  - type: hypermedia
  - route: example # /example; index is the route value for /.
  - title: Page # Optional page title; also used by section menus.
  - htmltag: <html lang="en"> # Optional document language.
  - section: example_pages # Optional grouping discovered by example_menu.
  - index: 10 # Optional menu sort position; this is not a route.
  - bodytag: <body class="site-page">|</body>
  - head:
      - type: head
      - title: Page
      - meta: # Optional metadata map; og: keys emit property instead of name.
          description: A short page description.
          keywords: example, page
          viewport: width=device-width, initial-scale=1
          og:title: Page
          og:description: A short page description.
          og:type: website
      # Charset needs literal HTML; meta.charset would emit a name/content tag.
      - charset:
          - type: html
          - value: |
              <meta charset="utf-8">
      # inherit references named bricks in this document's import scope.
      - styles:
          - inherit: example_esbuild_css
      - scripts:
          - inherit: example_esbuild_js
  - content:
      - type: template
      - template: # Conditional: page.html must exist in the templates directory.
          file: page.html
      # These keys match {{.title}} and {{.body}} in page.html.
      - values:
          title: Page
          body:
            - type: markdown
            - content: |
                ## Welcome

                Explore our latest stories, practical guides, and upcoming events.
```

#### Inherited page

```yaml
# Hypermedia variant: reuse the complete page shape instead of repeating assets.
# inherit must resolve within the same loaded import scope; type is inherited.
# Override only page-specific fields and existing Template value bindings.
example_page_inherited:
  - inherit: example_page
  - route: journal # A distinct route for the new page.
  - title: Coastal journal
  - index: 20
  - head:
      - title: Coastal journal
      - meta:
          description: Walking guides and weekend ideas along the coast.
          og:title: Coastal journal
          og:description: Walking guides and weekend ideas along the coast.
  - content:
      - values:
          title: Coastal journal
          body:
            - type: markdown
            - content: |
                ## Discover the coast

                Find your next favourite walking route or plan a relaxed weekend away.
```

### Fragment

[Full field reference: `fragment`](REFERENCE.md#fragment)

```yaml
# Fragment: no additional schema-required fields; children provide its output.
# A routed fragment returns replaceable HTML without a document wrapper.
example_fragment:
  - type: fragment
  - route: fragments/example # Conditional for an independently requested URL.
  - nocache: true # Optional; bypass rendered-output caching for fresh status.
  - response: # Optional browser response settings.
      headers:
        Cache-Control: no-store
  - content:
      - type: html
      - value: |
          <p id="status">
            Your booking request has been received.
          </p>
```

### Tree

[Full field reference: `tree`](REFERENCE.md#tree)

```yaml
# Tree: no additional schema-required fields; children render in source order.
# Use a tree to group content, not to own a browser route.
example_tree:
  - type: tree
  - enclose: <section class="updates">|</section> # Optional wrapper.
  - heading:
      - type: html
      - value: |
          <h2>
            Latest updates
          </h2>
  - copy:
      - type: text
      - value: New stories are published every week.
```

### Head

[Full field reference: `head`](REFERENCE.md#head)

```yaml
# Head: no additional schema-required fields; choose metadata/assets as needed.
# Mount this helper inside a Hypermedia head. A null meta value suppresses
# that entry when overriding inherited metadata.
example_head:
  - type: head
  - title: Coastal journal
  - meta:
      description: Walking guides and weekend ideas along the coast.
      viewport: width=device-width, initial-scale=1
  - css: # Optional already-served assets; this does not compile source CSS.
      - /static/css/main.css
  - js: # Optional already-served scripts; use esbuild children for bundling.
      - /static/js/main.js
```

### API Fragment Render

[Full field reference: `api_fragment_render`](REFERENCE.md#api_fragment_render)

#### GET response

```yaml
# API fragment render: required endpoint and method.
# route is conditional for exposing an independently requested browser URL.
# Always calls the upstream API; unlike nested api_render it owns a route.
example_api_fragment_render:
  - type: api_fragment_render
  - route: fragments/items
  - endpoint: https://api.example.com/items # Required; replace the placeholder.
  - method: GET # Required upstream HTTP method.
  - headers: # Optional upstream headers, not browser response headers.
      Accept: application/json
  - querykeys: [] # Optional; forward no browser query values to this API.
  - inline: | # Conditional: choose inline or a referenced template source.
      <ul>
        {{range .Data.items}}
          <li>
            {{.title}}
          </li>
        {{end}}
      </ul>
  - response: # Optional headers sent to the browser after rendering.
      headers:
        HX-Trigger: items-loaded
```

#### POST response

```yaml
# API fragment POST variant: raw JSON body, not a nested YAML object.
# This fixed payload demonstrates the shape; real forms supply their own inputs.
# Placeholder endpoint response: {"message": "Your newsletter subscription is confirmed."}
example_api_fragment_post:
  - type: api_fragment_render
  - route: actions/subscribe # Conditional browser action route.
  - endpoint: https://api.example.com/subscriptions # Required upstream endpoint.
  - method: POST # Required; upstream request method, not a route restriction.
  - headers:
      Content-Type: application/json # Conditional for this JSON payload.
      Accept: application/json
  - querykeys: []
  - body: | # Optional generally; supplies the request payload in this example.
      {"email": "reader@example.com", "newsletter": "coastal-journal"}
  - inline: |
      <p role="status">
        {{.Data.message}}
      </p>
```

## Component

### Template

[Full field reference: `template`](REFERENCE.md#template)

#### Inline template

```yaml
# Template: choose inline or template.file for meaningful output.
# No individual source field is schema-required; the selected file must exist.
# Scalar data and renderable children both mount under values.
example_template:
  - type: template
  - inline: |
      <article>
        <h2>
          {{.title}}
        </h2>
        {{.body}}
      </article>
  - querykeys: [] # Optional; this template exposes no request query parameters.
  - values: # Optional; supply only keys that the template actually uses.
      title: Latest updates
      body:
        - type: markdown
        - content: |
            Our spring collection is here. Discover new ideas for your next project.
```

#### File template

```yaml
# Template file variant: same bindings, HTML stored in an existing template asset.
# Use the page.html shape documented at the top; omit inline in this form.
example_template_file:
  - type: template
  - template:
      file: page.html # Conditional existing templates-relative asset.
  - values:
      title: Your coastal weekend
      body:
        - type: markdown
        - content: |
            Browse our walking guides and discover somewhere new this weekend.
```

#### Request query values

```yaml
# Template query variant: request values are .Params, not .Data or .values.
# Visiting a page that mounts this brick with ?q=coastal exposes .Params.q.
example_template_query:
  - type: template
  - querykeys: [q] # Conditional allowlist for this request-driven binding.
  - inline: |
      <p>
        {{if .Params.q}}
          You searched for {{.Params.q}}.
        {{else}}
          Search for a destination or walking route.
        {{end}}
      </p>
```

### Text

[Full field reference: `text`](REFERENCE.md#text)

```yaml
# Text: value is required and emitted unchanged; it is not HTML-escaped.
# Use trusted configuration text. Put visitor input in a Go HTML template
# binding when it needs contextual escaping.
example_text:
  - type: text
  - value: Your next adventure starts here. # Required text.
  - enclose: <p>|</p> # Optional wrapper.
```

### HTML

[Full field reference: `html`](REFERENCE.md#html)

```yaml
# HTML: value is required and renders literal markup.
example_html:
  - type: html
  - value: | # Required HTML source.
      <p>
        Discover walking routes, local favourites, and places worth visiting.
      </p>
  - trimspace: true # Optional; strip outer whitespace from the HTML value.
```

### Markdown

[Full field reference: `markdown`](REFERENCE.md#markdown)

#### Inline content

```yaml
# Markdown: conditional requirement -- declare exactly one of content or file.
# content may explicitly be empty. Markdown renders sanitized HTML.
example_markdown:
  - type: markdown
  - content: | # Conditional inline source; do not also declare file.
      # A weekend by the coast

      Take a break from the everyday and explore quiet beaches, local markets,
      and walking trails along the shoreline.

      ## Plan your visit

      - Walk the coastal trail before breakfast.
      - Pick up fresh produce at the Saturday market.
      - Finish the day with dinner overlooking the harbour.
  - class: prose # Optional; wraps the Markdown HTML in a div with this class.
```

#### File content

```yaml
# Markdown file form: clean resources-relative .md or .markdown reference.
# The file must exist; absolute paths, remote URLs, and query-selected files
# are not supported. Unlike image/JSON paths, do not use a path resolver here.
example_markdown_file:
  - type: markdown
  - file: content/page.md # Conditional file source; omit content.
  - max_bytes: 1048576 # Optional; default 1 MiB, supported range 1..20971520.
```

### Plugin

[Full field reference: `plugin`](REFERENCE.md#plugin)

```yaml
# Plugin: no schema-required property beyond type, but meaningful execution
# needs the exact name of an enabled, loaded plugin. data follows its contract;
# the example name and message field are placeholders, not a built-in plugin.
example_plugin:
  - type: plugin
  - plugin: ExamplePlugin__demo@1.0.0 # Conditional; replace with your plugin name.
  - data:
      message: Thank you for subscribing. Your first newsletter arrives on Friday.
  - classes: [newsletter-confirmation] # Optional plugin output wrapper classes.
```

## Menu

### Menu

[Full field reference: `menu`](REFERENCE.md#menu)

```yaml
# Menu: section, item, and active are required.
# Matching route owners must exist; example_page supplies one in example_pages.
# The templates use .Route and .Title from each discovered page configuration.
example_menu:
  - type: menu
  - section: example_pages # Required matching group.
  - sort: index # Optional; supported sorts: title, route, index.
  - order: asc # Optional; asc or desc.
  - item: | # Required regular-item template.
      <a href="{{if eq .Route "index"}}/{{else}}/{{.Route}}{{end}}">
        {{.Title}}
      </a>
  - active: | # Required current-route template.
      <a href="{{if eq .Route "index"}}/{{else}}/{{.Route}}{{end}}" aria-current="page">
        {{.Title}}
      </a>
  - enclose: <nav aria-label="Main navigation">|</nav> # Optional wrapper.
```

## Resources

### Esbuild

[Full field reference: `esbuild`](REFERENCE.md#esbuild)

#### JavaScript bundle

```yaml
# Esbuild: entry and outfile are required. The entry must exist; outfile must
# resolve inside static. Mount in a page head to render its asset tag.
# The embedded Go API is the default; no external esbuild executable is required.
example_esbuild_js:
  - type: esbuild
  - entry: # Required source reference.
      path: {base: resources, path: js/main.js}
  - outfile: # Required generated output reference.
      path: {base: static, path: js/main.js}
  - cache: true # Optional; reuse valid builds, independently of page caching.
  - minify: true # Optional; compact generated output.
  - fingerprint: true # Optional; use content-versioned output filenames.
  - enclose: <script src="|" defer></script>
```

#### CSS bundle

```yaml
example_esbuild_css:
  - type: esbuild
  - entry: # Required CSS source reference.
      path: {base: resources, path: css/main.css}
  - outfile: # Required generated CSS output reference.
      path: {base: static, path: css/main.css}
  - cache: true
  - minify: true
  - fingerprint: true
  - enclose: <link rel="stylesheet" href="|">
```

### CSS

[Full field reference: `css`](REFERENCE.md#css)

#### Inline CSS

```yaml
# CSS: choose inline, link, or file for useful output; none is schema-required.
# file takes precedence over link/inline and must exist. Inline/file output
# renders a style tag; link renders a stylesheet link. esbuild bundles source.
example_css:
  - type: css
  - inline: |
      body {
        margin: 0;
        font-family: system-ui, sans-serif;
      }
  - attributes: # Optional style-tag attributes.
      media: screen
```

#### Linked CSS

```yaml
# CSS link variant: URL of an already-served stylesheet, not a filesystem path.
example_css_link:
  - type: css
  - link: /static/css/main.css # Conditional source for the link form.
  - attributes:
      media: screen
```

#### File CSS

```yaml
# CSS file variant: existing local source is read and emitted inside a style tag.
example_css_file:
  - type: css
  - file: # Conditional source for this form; omit inline/link.
      path: {base: resources, path: css/main.css}
```

### JavaScript

[Full field reference: `js`](REFERENCE.md#js)

#### Inline JavaScript

```yaml
# JavaScript: js is canonical; javascript is an alias.
# Choose inline, link, or file; file takes precedence and must exist.
# Output is a script tag. Use esbuild for browser dependency bundling.
example_js:
  - type: js
  - inline: |
      document.documentElement.dataset.assetsReady = "true";
```

#### Linked JavaScript

```yaml
# JavaScript link variant: URL of an already-served script.
example_js_link:
  - type: js
  - link: /static/js/main.js # Conditional source for the external script form.
  - attributes:
      defer: true # Optional; defer execution of the external script.
```

#### File JavaScript

```yaml
# JavaScript file variant: existing local source is read into an inline script tag.
example_js_file:
  - type: js
  - file: # Conditional source for this form; omit inline/link.
      path: {base: resources, path: js/main.js}
```

### Styles

[Full field reference: `styles`](REFERENCE.md#styles)

```yaml
# Styles: file is required and must exist; reads local CSS into a style tag.
# This performs no bundling and is separate from CSS's inline/link choices.
example_styles:
  - type: styles
  - file: # Required; use a path resolver for the configured resources root.
      path: {base: resources, path: css/main.css}
```

### Image

[Full field reference: `image`](REFERENCE.md#image)

```yaml
# Image: src is required and must reference an existing local JPEG/PNG/GIF.
# Remote URLs and SVG processing are unsupported. Generated images are served
# under /static/images/. Omit height to preserve aspect ratio from width.
example_image:
  - type: image
  - src: # Required local source path.
      path: {base: resources, path: images/example.jpg}
  - alt: A coastal walking trail overlooking the sea. # Describe informative images.
  - width: 640 # Optional integer pixels; omit both dimensions to keep source size.
  - quality: 90 # Optional JPEG quality 1..100; does not affect PNG/GIF.
  - loading: lazy # Optional; use eager for an important first-viewport image.
  - class: article-image # Optional image class.
```

### Images

[Full field reference: `images`](REFERENCE.md#images)

```yaml
# Images: directory is required and must exist. Renders JPEG/PNG/GIF files in
# filename order, without descending into subdirectories; other types are skipped.
# alt is shared by every image; use individual image bricks for distinct captions/alt.
example_images:
  - type: images
  - directory: # Required local input directory.
      path: {base: resources, path: images}
  - alt: Views along the coastal walking trail.
  - width: 640 # Optional; omitted height preserves each image's aspect ratio.
  - loading: lazy
  - attributes:
      decoding: async # Optional browser decoding hint.
```

## Data

### JSON Render

[Full field reference: `json_render`](REFERENCE.md#json_render)

```yaml
# JSON render: file is required and must contain valid local JSON.
# Choose inline or template.file for output. JSON becomes .Data, while optional
# values remain ordinary template bindings such as .title. json is an alias.
example_json_render:
  - type: json_render
  - file: # Required existing local JSON source; shape documented above.
      path: {base: resources, path: data/items.json}
  - values: # Optional presentation values alongside the loaded data.
      title: Latest guides
  - inline: |
      <h2>
        {{.title}}
      </h2>
      <ul>
        {{range .Data.items}}
          <li>
            {{.title}}
          </li>
        {{end}}
      </ul>
```

### Goja Render

[Full field reference: `goja_render`](REFERENCE.md#goja_render)

```yaml
# Goja render: script is required and must declare main(input).
# Choose inline OR template.file; those sources are mutually exclusive.
# Each execution uses a fresh runtime. values are plain input data, not rendered
# children; return data from main to expose it as .Data in the output template.
example_goja_render:
  - type: goja_render
  - values:
      message: Your reservation is confirmed. We look forward to welcoming you.
  - timeout: 100ms # Optional default; positive deadline no greater than 5s.
  - querykeys: [] # Optional; scripts receive no query parameters by default.
  - script: | # Required trusted server-side JavaScript.
      function main(input) {
        return {message: input.values.message};
      }
  - inline: |
      <p>
        {{.Data.message}}
      </p>
```

### API Render

[Full field reference: `api_render`](REFERENCE.md#api_render)

```yaml
# API render: endpoint and method are required; choose an output template source.
# This is nested content, not a route owner. Mount it in a page/tree/template.
# .Data contains the parsed response and .Status contains the upstream status.
# It calls the API when executed; parent rendered-output caching can skip it.
# Set nocache on the parent page/fragment when every request needs fresh data.
example_api_render:
  - type: api_render
  - endpoint: https://api.example.com/items # Required; replace the placeholder.
  - method: GET # Required upstream HTTP method.
  - headers: # Optional upstream request headers.
      Accept: application/json
  - querykeys: [] # Optional; expose no browser query values to the upstream API.
  - queryparams: # Optional static upstream query values.
      limit: 3
  - inline: |
      <ul>
        {{range .Data.items}}
          <li>
            {{.title}}
          </li>
        {{end}}
      </ul>
```
