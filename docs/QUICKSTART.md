# Quickstart

This guide creates a small HyperBricks module with a page route, an HTMX
fragment route, and a reusable template.

For YAML syntax details, see [YAML_USAGE.md](YAML_USAGE.md). For component
fields, see [REFERENCE.md](REFERENCE.md).

## Install

Requires Go 1.26.1 or newer.

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

## Create A Module

Run this from the project root, the directory that will contain `modules/`:

```bash
hyperbricks init -m demo
```

The module layout is:

```text
modules/
  demo/
    hyperbricks/
    rendered/
    resources/
    static/
    templates/
    package.hyperbricks.yaml
```

Always run HyperBricks commands from the project root.

## Add A Page Route

Create or replace:

```text
modules/demo/hyperbricks/index.hyperbricks.yaml
```

```yaml
page:
  - type: hypermedia
  - route: index
  - title: HyperBricks Quickstart
  - head:
      - type: head
      - js:
          - https://unpkg.com/htmx.org@2.0.4
      - inline_styles:
          - type: css
          - inline: |
              body {
                font-family: system-ui, sans-serif;
                margin: 2rem;
              }
              main {
                max-width: 48rem;
              }
              button {
                cursor: pointer;
              }
              #target {
                margin-top: 1rem;
                padding: 1rem;
                border: 1px solid #ddd;
              }
  - main:
      - type: tree
      - enclose: <main>|</main>
      - intro:
          - type: html
          - value: |
              <h1>Hello HyperBricks</h1>
              <p>This page is rendered from YAML.</p>
              <button
                hx-get="/hello-fragment"
                hx-target="#target"
                hx-swap="innerHTML"
              >
                Load fragment
              </button>
              <div id="target">(fragment loads here)</div>
```

`page` is the object name. `type: hypermedia` makes it a full page route.
`route: index` makes it available as `/index`, and also as `/` with the default
routing settings.

## Add A Fragment Route

Create:

```text
modules/demo/hyperbricks/hello-fragment.hyperbricks.yaml
```

```yaml
hello_fragment:
  - type: fragment
  - route: hello-fragment
  - response:
      hx_trigger: helloLoaded
      hx_reswap: innerHTML
  - panel:
      - type: html
      - value: |
          <section>
            <h2>Hi from a fragment</h2>
            <p>This HTML was returned without a full page reload.</p>
          </section>
```

`type: fragment` creates an HTMX-friendly partial route. The `response` block
can set HTMX response headers.

## Run The Dev Server

```bash
hyperbricks start -m demo
```

Open:

```text
http://localhost:8080/
```

Click `Load fragment`. The browser calls `/hello-fragment` and swaps the
response into `#target`.

## Use A Template File

Create:

```text
modules/demo/templates/card.html
```

```html
<section class="card">
  <h2>{{.title}}</h2>
  <p>{{.body}}</p>
</section>
```

Then add a template child to `page.main`:

```yaml
      - reusable_card:
          - type: template
          - template:
              file: card.html
          - values:
              title: Template file
              body: This block is loaded from modules/demo/templates/card.html.
```

`template.file` preloads the file from the module templates directory and keeps
the template name in the runtime config.

## Reuse With Inheritance

Define a reusable object at the top level:

```yaml
base_card:
  - type: template
  - template:
      file: card.html
  - values:
      title: Base card
      body: Base body
```

Then use it inside `page.main` and override only the values that change:

```yaml
      - welcome_card:
          - inherit: base_card
          - values:
              title: Welcome
              body: This card reuses base_card and overrides its values.
      - next_card:
          - inherit: base_card
          - values:
              title: Next step
              body: The template source stays shared.
```

`inherit` deep-copies the referenced object before applying local fields.
Inherited objects keep their structure, while local values override or extend
the copy.

## Render Static Output

```bash
hyperbricks static -m demo
```

Static output is written to the module render directory, by default:

```text
modules/demo/rendered/
```

Static rendering requests your routes through an internal localhost runtime
before writing files. Nested `api_render` blocks run during this snapshot, so
public API-backed pages can be exported as plain HTML. For a runnable example,
see `modules/sampleapis-coffee-static`.

## Next Steps

- [YAML_USAGE.md](YAML_USAGE.md): YAML syntax, resolvers, imports, inheritance.
- [REFERENCE.md](REFERENCE.md): component fields and executable examples.
- [ROUTING.md](ROUTING.md): route resolution and clean URLs.
- [ROUTE_GUARD.md](ROUTE_GUARD.md): pre-render route authorization.
